package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/tesseract-hub/audit-service/internal/cache"
	"github.com/tesseract-hub/audit-service/internal/config"
	"github.com/tesseract-hub/audit-service/internal/consumer"
	"github.com/tesseract-hub/audit-service/internal/database"
	"github.com/tesseract-hub/audit-service/internal/handlers"
	"github.com/tesseract-hub/audit-service/internal/middleware"
	auditNats "github.com/tesseract-hub/audit-service/internal/nats"
	"github.com/tesseract-hub/audit-service/internal/repository"
	"github.com/tesseract-hub/audit-service/internal/scheduler"
	"github.com/tesseract-hub/audit-service/internal/services"
	"github.com/tesseract-hub/audit-service/internal/tenant"

	gosharedmw "github.com/tesseract-hub/go-shared/middleware"
	"github.com/tesseract-hub/go-shared/tracing"
)

// @title Audit Service API
// @version 2.0
// @description Production-ready Multi-Tenant Audit Service API for tracking and managing audit logs across multiple products/databases
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8093
// @BasePath /api/v1
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	if cfg.IsProduction() {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Initialize Redis client
	redisClient := initRedis(cfg, logger)
	defer func() {
		if redisClient != nil {
			redisClient.Close()
		}
	}()

	// Initialize tenant registry
	tenantRegistry, err := initTenantRegistry(cfg, redisClient, logger)
	if err != nil {
		log.Fatalf("Failed to initialize tenant registry: %v", err)
	}
	logger.Info("Tenant registry initialized")

	// Initialize multi-database connection manager with fallback database
	var fallbackDBConfig *database.FallbackDBConfig
	if cfg.FallbackDB.Enabled && cfg.FallbackDB.Host != "" {
		fallbackDBConfig = &database.FallbackDBConfig{
			Enabled:      cfg.FallbackDB.Enabled,
			Host:         cfg.FallbackDB.Host,
			Port:         cfg.FallbackDB.Port,
			Database:     cfg.FallbackDB.Database,
			User:         cfg.FallbackDB.User,
			Password:     cfg.FallbackDB.Password,
			SSLMode:      cfg.FallbackDB.SSLMode,
			MaxOpenConns: cfg.FallbackDB.MaxOpenConns,
			MaxIdleConns: cfg.FallbackDB.MaxIdleConns,
		}
		logger.WithFields(logrus.Fields{
			"host":     cfg.FallbackDB.Host,
			"port":     cfg.FallbackDB.Port,
			"database": cfg.FallbackDB.Database,
		}).Info("Fallback database configured")
	}

	dbManager := database.NewManager(database.ManagerConfig{
		Registry:            tenantRegistry,
		Logger:              logger,
		MaxPoolsPerService:  cfg.MaxDBPools(),
		PoolCleanupInterval: 5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		FallbackDB:          fallbackDBConfig,
	})
	defer dbManager.Close()
	logger.Info("Multi-database connection manager initialized")

	// Initialize cache
	auditCache := cache.NewAuditCache(cache.CacheConfig{
		RedisClient:    redisClient,
		Logger:         logger,
		DefaultTTL:     5 * time.Minute,
		SummaryTTL:     1 * time.Minute,
		CriticalTTL:    30 * time.Second,
		LocalCacheSize: 1000,
	})
	logger.Info("Audit cache initialized")

	// Initialize NATS client for real-time event streaming
	var natsClient *auditNats.Client
	var natsPublisher *auditNats.Publisher
	var natsSubscriber *auditNats.Subscriber
	if cfg.NATS.Enabled {
		natsClient, err = auditNats.NewClient(auditNats.Config{
			URL:           cfg.NATS.URL,
			MaxReconnects: cfg.NATS.MaxReconnects,
			ReconnectWait: time.Duration(cfg.NATS.ReconnectWait) * time.Second,
		}, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize NATS, real-time streaming disabled")
		} else {
			natsPublisher = auditNats.NewPublisher(natsClient, logger)
			natsSubscriber = auditNats.NewSubscriber(natsClient, logger)
			logger.Info("NATS client initialized for real-time event streaming")
		}
	} else {
		logger.Info("NATS disabled, using polling for real-time updates")
	}
	defer func() {
		if natsClient != nil {
			natsClient.Close()
		}
	}()

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.IsProduction() {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("audit-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("audit-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "audit_service")
	log.Println("Prometheus metrics initialized")

	// Initialize repository with multi-tenant support
	auditRepo := repository.NewMultiTenantRepository(dbManager, auditCache, logger)

	// Initialize service with NATS publisher for event streaming
	auditService := services.NewAuditService(auditRepo, logger, natsPublisher)

	// Initialize handlers with NATS subscriber for real-time streaming
	auditHandlers := handlers.NewAuditHandlers(auditService, logger, natsSubscriber)

	// Initialize cleanup scheduler for retention management
	cleanupScheduler := scheduler.NewCleanupScheduler(auditRepo, tenantRegistry, cfg.Retention, logger)
	if err := cleanupScheduler.Start(); err != nil {
		logger.WithError(err).Warn("Failed to start cleanup scheduler (continuing without scheduled cleanup)")
	} else {
		logger.Info("Cleanup scheduler started")
	}
	defer cleanupScheduler.Stop()

	// Initialize domain event consumer to receive events from all services
	var domainEventConsumer *consumer.DomainEventConsumer
	if cfg.NATS.Enabled {
		consumerCfg := consumer.ConsumerConfig{
			NATSURL:       cfg.NATS.URL,
			MaxReconnects: cfg.NATS.MaxReconnects,
			ReconnectWait: time.Duration(cfg.NATS.ReconnectWait) * time.Second,
		}
		domainEventConsumer, err = consumer.NewDomainEventConsumer(consumerCfg, auditService, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to create domain event consumer (continuing without event consumption)")
		} else {
			if err := domainEventConsumer.Start(context.Background()); err != nil {
				logger.WithError(err).Warn("Failed to start domain event consumer")
			} else {
				logger.Info("Domain event consumer started - listening for events from all services")
			}
		}
	}
	defer func() {
		if domainEventConsumer != nil {
			domainEventConsumer.Stop()
		}
	}()

	// Create stats handler for monitoring
	statsHandler := &StatsHandler{
		dbManager:        dbManager,
		tenantRegistry:   tenantRegistry,
		cache:            auditCache,
		natsSubscriber:   natsSubscriber,
		cleanupScheduler: cleanupScheduler,
	}

	// Setup router
	router := setupRouter(cfg, auditHandlers, statsHandler, metrics)

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down Audit Service...")

		// Shutdown tracer provider
		if tracerProvider != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tracerProvider.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down tracer provider: %v", err)
			} else {
				log.Println("Tracer provider shut down")
			}
		}

		// Close database connections
		if err := dbManager.Close(); err != nil {
			log.Printf("Error closing database connections: %v", err)
		} else {
			log.Println("Database connections closed")
		}

		log.Println("Audit service stopped")
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting Audit Service on %s", cfg.GetServerAddress())
	if err := router.Run(cfg.GetServerAddress()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initRedis initializes the Redis client
func initRedis(cfg *config.Config, logger *logrus.Logger) *redis.Client {
	if cfg.Redis.URL == "" {
		logger.Warn("Redis URL not configured, caching will use local memory only")
		return nil
	}

	opt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.WithError(err).Warn("Failed to parse Redis URL, using local memory cache only")
		return nil
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.WithError(err).Warn("Failed to connect to Redis, using local memory cache only")
		return nil
	}

	logger.Info("Redis connection established")
	return client
}

// initTenantRegistry initializes the tenant registry
func initTenantRegistry(cfg *config.Config, redisClient *redis.Client, logger *logrus.Logger) (*tenant.Registry, error) {
	return tenant.NewRegistry(tenant.RegistryConfig{
		RegistryURL:   cfg.Tenant.RegistryURL,
		EncryptionKey: cfg.Tenant.EncryptionKey,
		CacheTTL:      time.Duration(cfg.Tenant.CacheTTL) * time.Second,
		RedisClient:   redisClient,
		Logger:        logger,
	})
}

// StatsHandler handles statistics and monitoring endpoints
type StatsHandler struct {
	dbManager        *database.Manager
	tenantRegistry   *tenant.Registry
	cache            *cache.AuditCache
	natsSubscriber   *auditNats.Subscriber
	cleanupScheduler *scheduler.CleanupScheduler
}

// setupRouter configures the Gin router with middleware and routes
func setupRouter(cfg *config.Config, auditHandlers *handlers.AuditHandlers, statsHandler *StatsHandler, metrics *gosharedmw.Metrics) *gin.Engine {
	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Setup middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.TenantID())
	router.Use(middleware.SetupCORS())

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("audit-service"))

	// Health check endpoints
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "audit-service", "version": "2.0"})
	})
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ready", "service": "audit-service"})
	})
	router.GET("/metrics", gosharedmw.Handler())

	// Internal stats endpoint (for monitoring)
	router.GET("/internal/stats", func(c *gin.Context) {
		stats := gin.H{
			"database_manager": statsHandler.dbManager.GetStats(),
			"tenant_registry":  statsHandler.tenantRegistry.GetStats(),
			"cache":            statsHandler.cache.GetStats(),
		}
		if statsHandler.natsSubscriber != nil {
			stats["nats"] = statsHandler.natsSubscriber.GetStats()
		}
		if statsHandler.cleanupScheduler != nil {
			stats["cleanup_scheduler"] = statsHandler.cleanupScheduler.GetStats()
		}
		c.JSON(200, stats)
	})

	// API routes - require tenant ID for multi-tenant data isolation
	api := router.Group("/api/v1")
	api.Use(middleware.RequireTenantID())
	api.Use(middleware.ValidateTenantUUID())
	{
		auditLogs := api.Group("/audit-logs")
		{
			// Core CRUD operations
			auditLogs.POST("", auditHandlers.CreateAuditLog)
			auditLogs.GET("", auditHandlers.ListAuditLogs)
			auditLogs.GET("/:id", auditHandlers.GetAuditLog)

			// Analytics and reporting
			auditLogs.GET("/summary", auditHandlers.GetSummary)
			auditLogs.GET("/critical", auditHandlers.GetCriticalEvents)
			auditLogs.GET("/failed-auth", auditHandlers.GetFailedAuthAttempts)
			auditLogs.GET("/suspicious-activity", auditHandlers.GetSuspiciousActivity)

			// Resource and user history
			auditLogs.GET("/resource/:resource_type/:resource_id", auditHandlers.GetResourceHistory)
			auditLogs.GET("/user/:user_id", auditHandlers.GetUserActivity)
			auditLogs.GET("/user/:user_id/ip-history", auditHandlers.GetUserIPHistory)

			// Real-time updates
			auditLogs.GET("/recent", auditHandlers.GetRecentLogs)
			auditLogs.GET("/stream", auditHandlers.StreamAuditLogs)

			// Export
			auditLogs.GET("/export", auditHandlers.ExportAuditLogs)

			// Retention settings
			auditLogs.GET("/retention", auditHandlers.GetRetentionSettings)
			auditLogs.PUT("/retention", auditHandlers.SetRetentionSettings)
			auditLogs.POST("/cleanup", auditHandlers.TriggerCleanup)
		}

		// Cache management (internal use)
		cacheGroup := api.Group("/cache")
		{
			cacheGroup.DELETE("/tenant/:tenant_id", func(c *gin.Context) {
				tenantID := c.Param("tenant_id")
				statsHandler.cache.InvalidateTenant(c.Request.Context(), tenantID)
				c.JSON(200, gin.H{"message": "Cache invalidated for tenant", "tenant_id": tenantID})
			})
		}
	}

	return router
}
