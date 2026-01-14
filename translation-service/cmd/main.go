package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"translation-service/internal/cache"
	"translation-service/internal/clients"
	"translation-service/internal/config"
	"translation-service/internal/handlers"
	"translation-service/internal/middleware"
	"translation-service/internal/models"
	"translation-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	log := logger.WithField("service", "translation-service")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.App.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set Gin mode
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize OpenTelemetry tracing
	var tracerProvider interface{ Shutdown(context.Context) error }
	if cfg.App.Environment == "production" {
		tp, err := tracing.InitTracer(tracing.ProductionConfig("translation-service"))
		if err != nil {
			log.WithError(err).Warn("Failed to initialize production tracer")
		} else {
			tracerProvider = tp
		}
	} else {
		tp, err := tracing.InitTracer(tracing.DefaultConfig("translation-service"))
		if err != nil {
			log.WithError(err).Warn("Failed to initialize tracer")
		} else {
			tracerProvider = tp
		}
	}

	// Initialize database
	db, err := initDatabase(&cfg.Database, cfg.App.Environment)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	// Initialize repository
	repo := repository.NewTranslationRepository(db)

	// Seed languages
	if err := repo.SeedLanguages(context.Background()); err != nil {
		log.WithError(err).Warn("Failed to seed languages")
	}

	// Initialize Redis cache
	var redisCache *cache.TranslationCache
	if cfg.Translation.CacheEnabled {
		redisCache, err = cache.NewTranslationCache(
			cfg.Redis.Host,
			cfg.Redis.Port,
			cfg.Redis.Password,
			cfg.Redis.DB,
			cfg.Translation.CacheTTL,
			log,
		)
		if err != nil {
			log.WithError(err).Warn("Failed to initialize Redis cache, continuing without cache")
		}
	}

	// Initialize translation providers
	// Provider priority: LibreTranslate (1) -> Bergamot (2) -> Hugging Face (3) -> Google (4)
	var providers []clients.TranslationProvider

	// 1. LibreTranslate (primary provider - open source, self-hosted)
	libreTranslate := clients.NewLibreTranslateClient(
		cfg.Translation.LibreTranslateURL,
		cfg.Translation.LibreTranslateKey,
		log,
	)
	providers = append(providers, libreTranslate)

	// Check LibreTranslate health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := libreTranslate.HealthCheck(ctx); err != nil {
		log.WithError(err).Warn("LibreTranslate health check failed - service may not be available")
	} else {
		log.Info("LibreTranslate connection verified (priority 1)")
	}
	cancel()

	// 2. Bergamot (first fallback - Mozilla's fast translation engine, self-hosted)
	if cfg.Translation.BergamotURL != "" {
		bergamot := clients.NewBergamotClient(
			cfg.Translation.BergamotURL,
			log,
		)
		providers = append(providers, bergamot)
		log.Info("Bergamot enabled as fallback (priority 2)")
	}

	// 3. Hugging Face MT (second fallback - Helsinki-NLP models, self-hosted or API)
	// For self-hosted: HuggingFaceURL points to local huggingface-mt-service
	// For API: HuggingFaceURL points to huggingface.co and requires API key
	huggingFace := clients.NewHuggingFaceClient(
		cfg.Translation.HuggingFaceKey,
		cfg.Translation.HuggingFaceURL,
		log,
	)
	if huggingFace.IsConfigured() {
		providers = append(providers, huggingFace)
		log.Info("Hugging Face MT enabled as fallback (priority 3)")
	} else {
		log.Info("Hugging Face MT not configured - skipping as fallback")
	}

	// 4. Google Translate (final fallback - paid, most language support)
	if cfg.Translation.GoogleTranslateKey != "" {
		googleTranslate := clients.NewGoogleTranslateClient(
			cfg.Translation.GoogleTranslateKey,
			log,
		)
		providers = append(providers, googleTranslate)
		log.Info("Google Translate enabled as final fallback (priority 4)")
	} else {
		log.Warn("Google Translate API key not configured - some languages may not be available")
	}

	// Create the orchestrator with provider chain
	orchestrator := clients.NewTranslationOrchestrator(providers, log)

	// Initialize handler with orchestrator
	handler := handlers.NewTranslationHandler(
		repo,
		redisCache,
		orchestrator,
		libreTranslate, // Keep reference for language detection
		&cfg.Translation,
		log,
	)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(
		cfg.Translation.RateLimit,
		cfg.Translation.RateLimitWindow,
	)

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "translation_service")

	// Setup Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())
	router.Use(metrics.Middleware())

	// Initialize Istio auth middleware for Keycloak JWT validation
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        false, // Translation is mostly public, auth is optional
		AllowLegacyHeaders: true,
		Logger:             istioAuthLogger,
	})

	// Authentication middleware
	if cfg.App.Environment == "development" {
		router.Use(middleware.TenantID())
		router.Use(middleware.UserID())
	} else {
		router.Use(istioAuth)
		router.Use(tracing.GinMiddleware("translation-service"))
	}

	// Health endpoints (no auth required)
	router.GET("/health", handler.Health)
	router.GET("/livez", handler.Livez)
	router.GET("/readyz", handler.Readyz)

	// Metrics endpoint
	router.GET("/metrics", gosharedmw.Handler())

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public endpoints (rate limited)
		v1.POST("/translate", rateLimiter.Middleware(), handler.Translate)
		v1.POST("/translate/batch", rateLimiter.Middleware(), handler.TranslateBatch)
		v1.POST("/detect", rateLimiter.Middleware(), handler.DetectLanguage)
		v1.GET("/languages", handler.GetLanguages)

		// Tenant-specific endpoints
		v1.GET("/stats", middleware.RequireTenantID(), handler.GetStats)
		v1.GET("/preferences", middleware.RequireTenantID(), handler.GetPreference)
		v1.PUT("/preferences", middleware.RequireTenantID(), handler.UpdatePreference)
		v1.DELETE("/cache", middleware.RequireTenantID(), handler.InvalidateCache)

		// User-specific language preference endpoints
		// These persist user's preferred language in the database
		// Default is English (en) if user hasn't set a preference
		users := v1.Group("/users")
		{
			users.GET("/me/language", middleware.RequireTenantID(), handler.GetUserLanguagePreference)
			users.PUT("/me/language", middleware.RequireTenantID(), handler.SetUserLanguagePreference)
			users.DELETE("/me/language", middleware.RequireTenantID(), handler.ResetUserLanguagePreference)
		}
	}

	// Start background cleanup task
	go startCleanupTask(repo, log)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.WithField("addr", addr).Info("Starting translation service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server forced to shutdown")
	}

	// Close Redis connection
	if redisCache != nil {
		if err := redisCache.Close(); err != nil {
			log.WithError(err).Warn("Failed to close Redis connection")
		}
	}

	// Shutdown tracer
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.WithError(err).Warn("Failed to shutdown tracer")
		}
	}

	log.Info("Server exited")
}

func initDatabase(cfg *config.DatabaseConfig, env string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	logLevel := gormLogger.Silent
	if env != "production" {
		logLevel = gormLogger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Language{},
		&models.TranslationCache{},
		&models.TranslationStats{},
		&models.TenantLanguagePreference{},
		&models.UserLanguagePreference{}, // User-level language preferences (multi-tenant)
	)
}

func startCleanupTask(repo repository.TranslationRepository, log *logrus.Entry) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		deleted, err := repo.DeleteExpiredTranslations(ctx)
		cancel()

		if err != nil {
			log.WithError(err).Warn("Failed to clean up expired translations")
		} else if deleted > 0 {
			log.WithField("deleted", deleted).Info("Cleaned up expired translations")
		}
	}
}
