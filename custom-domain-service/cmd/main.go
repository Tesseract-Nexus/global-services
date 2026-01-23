package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"custom-domain-service/internal/clients"
	"custom-domain-service/internal/config"
	"custom-domain-service/internal/handlers"
	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"
	"custom-domain-service/internal/services"
	"custom-domain-service/internal/workers"

	"github.com/Tesseract-Nexus/go-shared/events"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Initialize logging
	initLogging()

	log.Info().Msg("Starting custom-domain-service")

	// Load configuration
	cfg := config.NewConfig()

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Initialize Redis
	redisClient := initRedis(cfg)

	// Initialize repository
	domainRepo := repository.NewDomainRepository(db)

	// Initialize DNS verifier
	dnsVerifier := services.NewDNSVerifier(cfg)

	// Initialize Kubernetes client
	var k8sClient *clients.KubernetesClient
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		k8sClient, err = clients.NewKubernetesClient(cfg)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Kubernetes client (running outside cluster?)")
		}
	} else {
		log.Info().Msg("Not running in Kubernetes cluster, K8s client disabled")
	}

	// Initialize Keycloak client
	keycloakClient := clients.NewKeycloakClient(cfg)

	// Initialize tenant client
	tenantClient := clients.NewTenantClient(cfg)

	// Initialize Cloudflare client
	var cloudflareClient *clients.CloudflareClient
	if cfg.Cloudflare.Enabled {
		cloudflareClient = clients.NewCloudflareClient(&cfg.Cloudflare)
		log.Info().
			Str("tunnel_id", clients.MaskSensitiveID(cfg.Cloudflare.TunnelID)).
			Msg("Cloudflare Tunnel client initialized")
	} else {
		log.Info().Msg("Cloudflare Tunnel disabled, using cert-manager for custom domains")
	}

	// Initialize NATS event publisher
	var eventPublisher *events.Publisher
	if cfg.NATS.URL != "" {
		publisherCfg := events.DefaultPublisherConfig(cfg.NATS.URL)
		publisherCfg.Name = "custom-domain-service"

		logrusLogger := logrus.New()
		if os.Getenv("GIN_MODE") == "release" {
			logrusLogger.SetLevel(logrus.InfoLevel)
		} else {
			logrusLogger.SetLevel(logrus.DebugLevel)
		}

		var err error
		eventPublisher, err = events.NewPublisher(publisherCfg, logrusLogger)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize NATS publisher, events will not be published")
		} else {
			log.Info().Str("url", cfg.NATS.URL).Msg("NATS event publisher initialized")

			// Ensure the domain events stream exists
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := eventPublisher.EnsureStream(ctx, events.StreamDomains, []string{"domain.>"}); err != nil {
				log.Warn().Err(err).Msg("Failed to ensure domain events stream")
			}
			cancel()
		}
	} else {
		log.Warn().Msg("NATS URL not configured, event publishing disabled")
	}

	// Initialize domain service
	domainService := services.NewDomainService(
		cfg,
		domainRepo,
		dnsVerifier,
		k8sClient,
		keycloakClient,
		tenantClient,
		cloudflareClient,
		redisClient,
		eventPublisher,
	)

	// Initialize handlers
	domainHandlers := handlers.NewDomainHandlers(domainService)
	internalHandlers := handlers.NewInternalHandlers(domainService)

	// Create router
	router := setupRouter(cfg, domainHandlers, internalHandlers)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startWorkers(ctx, cfg, domainRepo, dnsVerifier, domainService, k8sClient)

	// Start server
	go func() {
		log.Info().Str("addr", server.Addr).Msg("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Cancel context to stop workers
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	// Close database connection
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	// Close Redis connection
	if redisClient != nil {
		redisClient.Close()
	}

	// Close NATS event publisher
	if eventPublisher != nil {
		eventPublisher.Close()
	}

	log.Info().Msg("Server exited")
}

func initLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Use JSON logging in production
	if os.Getenv("GIN_MODE") == "release" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	gormLogger := logger.Default
	if os.Getenv("GIN_MODE") == "release" {
		gormLogger = logger.Default.LogMode(logger.Silent)
	} else {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func runMigrations(db *gorm.DB) error {
	log.Info().Msg("Running database migrations")

	// Create extension for UUID generation
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	// Run auto-migrations
	return db.AutoMigrate(
		&models.CustomDomain{},
		&models.DomainActivity{},
		&models.DomainHealth{},
	)
}

func initRedis(cfg *config.Config) *redis.Client {
	opt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse Redis URL, using defaults")
		opt = &redis.Options{
			Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		}
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("Failed to connect to Redis, caching disabled")
		return nil
	}

	log.Info().Msg("Connected to Redis")
	return client
}

func setupRouter(cfg *config.Config, domainHandlers *handlers.DomainHandlers, internalHandlers *handlers.InternalHandlers) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	// CORS middleware - configured for internal service communication
	// When running behind Istio, CORS is typically handled at the gateway level
	// For internal APIs, we restrict origins to known services
	allowedOrigins := []string{
		"https://admin.devtest.tesserix.app",
		"https://onboarding.devtest.tesserix.app",
		"https://admin.tesserix.app",
		"https://onboarding.tesserix.app",
	}
	// Allow additional origins from environment
	if extraOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); extraOrigins != "" {
		for _, origin := range splitAndTrim(extraOrigins, ",") {
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID", "X-User-ID", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health endpoints
	router.GET("/health", internalHandlers.Health)
	router.GET("/ready", internalHandlers.Ready)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Domain routes (authenticated)
		domains := v1.Group("/domains")
		{
			domains.POST("/validate", domainHandlers.ValidateDomain)
			domains.POST("/verify-by-name", domainHandlers.VerifyDomainByName)
			domains.POST("", domainHandlers.CreateDomain)
			domains.GET("", domainHandlers.ListDomains)
			domains.GET("/stats", domainHandlers.GetStats)
			domains.GET("/:id", domainHandlers.GetDomain)
			domains.PATCH("/:id", domainHandlers.UpdateDomain)
			domains.DELETE("/:id", domainHandlers.DeleteDomain)
			domains.POST("/:id/verify", domainHandlers.VerifyDomain)
			domains.GET("/:id/dns", domainHandlers.GetDNSStatus)
			domains.GET("/:id/ssl", domainHandlers.GetSSLStatus)
			domains.GET("/:id/health", domainHandlers.HealthCheck)
			domains.GET("/:id/activities", domainHandlers.GetActivities)

			// NS Delegation routes for automatic SSL certificate management
			domains.GET("/:id/ns-delegation", domainHandlers.GetNSDelegationStatus)
			domains.POST("/:id/ns-delegation/verify", domainHandlers.VerifyNSDelegation)
			domains.POST("/:id/ns-delegation/enable", domainHandlers.EnableNSDelegation)
		}

		// Internal routes (service-to-service)
		internal := v1.Group("/internal")
		{
			internal.GET("/resolve", internalHandlers.ResolveDomain)
			internal.GET("/check", internalHandlers.CheckDomain)
		}
	}

	return router
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}

		log.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Str("client_ip", c.ClientIP()).
			Msg("request")
	}
}

func startWorkers(
	ctx context.Context,
	cfg *config.Config,
	repo *repository.DomainRepository,
	dnsVerifier *services.DNSVerifier,
	domainSvc *services.DomainService,
	k8sClient *clients.KubernetesClient,
) {
	// DNS Verification Worker
	dnsWorker := workers.NewDNSVerificationWorker(cfg, repo, dnsVerifier, domainSvc)
	go dnsWorker.Start(ctx)

	// Certificate Monitor Worker (only if K8s client is available)
	if k8sClient != nil {
		certWorker := workers.NewCertMonitorWorker(cfg, repo, k8sClient)
		go certWorker.Start(ctx)
	}

	// Health Check Worker
	healthWorker := workers.NewHealthCheckWorker(cfg, repo)
	go healthWorker.Start(ctx)

	// Cleanup Worker
	cleanupWorker := workers.NewCleanupWorker(cfg, repo)
	go cleanupWorker.Start(ctx)

	log.Info().Msg("Background workers started")
}

// splitAndTrim splits a string by separator and trims whitespace from each element
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
