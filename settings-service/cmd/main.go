package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"settings-service/internal/cache"
	"settings-service/internal/clients/frankfurter"
	"settings-service/internal/config"
	"settings-service/internal/events"
	"settings-service/internal/handlers"
	"settings-service/internal/health"
	"settings-service/internal/middleware"
	"settings-service/internal/models"
	"settings-service/internal/repository"
	"settings-service/internal/services"
	"settings-service/internal/workers"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Settings Management API
// @version 2.0
// @description Comprehensive settings management service for Tesseract Hub applications
// @termsOfService http://swagger.io/terms/
// @contact.name Tesseract Hub Team
// @contact.email dev@tesseract-hub.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8085
// @BasePath /
// @schemes http https
func main() {
	// Check if running health check
	if len(os.Args) > 1 && os.Args[1] == "health" {
		// Perform a simple liveness check
		resp, err := http.Get("http://localhost:8085/livez")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.NewConfig()

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize database
	db, err := initializeDatabase(cfg.Database)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Run database migrations
	if err := runMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize NATS events publisher (non-blocking)
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)
	go func() {
		if err := events.InitPublisher(eventLogger); err != nil {
			log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
		} else {
			log.Println("✓ NATS events publisher initialized")
		}
	}()

	// Initialize Redis client (graceful degradation if unavailable)
	var redisClient *redis.Client
	if cfg.Redis.URL != "" {
		opt, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			log.Printf("WARNING: Failed to parse Redis URL: %v (caching disabled)", err)
		} else {
			redisClient = redis.NewClient(opt)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("WARNING: Failed to connect to Redis: %v (caching disabled)", err)
				redisClient = nil
			} else {
				log.Println("✓ Redis connection established")
			}
		}
	}

	// Initialize dependencies
	settingsRepo := repository.NewSettingsRepository(db)
	settingsService := services.NewSettingsService(settingsRepo)
	settingsHandler := handlers.NewSettingsHandler(settingsService)

	// Initialize tenant dependencies (for audit config)
	tenantRepo := repository.NewTenantRepository(db)
	tenantHandler := handlers.NewTenantHandler(tenantRepo)

	// Initialize storefront theme dependencies
	storefrontThemeRepo := repository.NewStorefrontThemeRepository(db)
	storefrontThemeService := services.NewStorefrontThemeService(storefrontThemeRepo)
	storefrontThemeHandler := handlers.NewStorefrontThemeHandler(storefrontThemeService)

	// Initialize currency dependencies with Redis caching
	frankfurterClient := frankfurter.NewDefaultClient()
	exchangeRateRepo := repository.NewExchangeRateRepository(db)
	currencyCache := cache.NewCurrencyCache(redisClient) // Redis caching enabled
	currencyService := services.NewCurrencyService(frankfurterClient, exchangeRateRepo, currencyCache)
	rateUpdater := workers.NewRateUpdater(currencyService, workers.DefaultUpdateInterval)
	currencyHandler := handlers.NewCurrencyHandler(currencyService, rateUpdater)

	// Start the rate updater
	rateUpdater.Start()

	// Initialize health checker
	healthChecker := health.NewHealthChecker(db, cfg.App.Version)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Initialize Gin router
	router := setupRouter(settingsHandler, storefrontThemeHandler, currencyHandler, tenantHandler, healthChecker, rbacMiddleware, cfg, eventLogger, redisClient)

	// Mark service as ready
	healthChecker.SetReady(true)

	// Start server
	serverAddr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("🚀 Settings Service starting on %s", serverAddr)
	log.Printf("📚 API Documentation available at http://%s/swagger/index.html", serverAddr)
	log.Printf("🏥 Health endpoints: /health, /livez, /readyz")
	log.Printf("📊 Metrics available at http://%s/metrics", serverAddr)
	log.Printf("💱 Currency API available at http://%s/api/v1/currency", serverAddr)

	// Graceful shutdown handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		rateUpdater.Stop()
		os.Exit(0)
	}()

	if err := router.Run(serverAddr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// initializeDatabase establishes database connection
func initializeDatabase(dbConfig config.DatabaseConfig) (*gorm.DB, error) {
	dsn := dbConfig.DSN()
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database for ping
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Test database connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("✅ Database connection established successfully")
	return db, nil
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB) error {
	log.Println("🔄 Running database migrations...")

	// Run AutoMigrate for all models
	if err := db.AutoMigrate(
		// Core settings models
		&models.Settings{},
		&models.SettingsPreset{},
		&models.SettingsHistory{},
		&models.SettingsValidation{},
		// Storefront theme models
		&models.StorefrontThemeSettings{},
		&models.StorefrontThemeHistory{},
		// Currency models
		&models.ExchangeRate{},
	); err != nil {
		log.Printf("⚠️  AutoMigrate warning: %v", err)
		// Don't fail - the table may already exist with slightly different schema
	}

	log.Println("✅ Database migrations completed successfully")
	return nil
}

// setupRouter configures the Gin router with middleware and routes
func setupRouter(settingsHandler *handlers.SettingsHandler, storefrontThemeHandler *handlers.StorefrontThemeHandler, currencyHandler *handlers.CurrencyHandler, tenantHandler *handlers.TenantHandler, healthChecker *health.HealthChecker, rbacMiddleware *rbac.Middleware, cfg *config.Config, logger *logrus.Logger, redisClient *redis.Client) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recovery())

	// Security headers middleware
	router.Use(gosharedmw.SecurityHeaders())

	// Rate limiting middleware (uses Redis for distributed rate limiting)
	// Custom config to exclude storefront-theme endpoints from rate limiting
	// These are high-frequency endpoints used by admin dashboard for theme customization
	excludedPaths := []string{
		"/health",
		"/ready",
		"/metrics",
		"/livez",
		"/readyz",
		"/api/v1/storefront-theme",        // Exclude all storefront-theme endpoints
		"/api/v1/public/storefront-theme", // Public theme endpoints
	}

	if redisClient != nil {
		rateLimitConfig := gosharedmw.RedisRateLimitConfig{
			RequestsPerSecond: 100,
			BurstSize:         200,
			WindowDuration:    time.Second, // 1 second
			KeyPrefix:         "ratelimit:settings:",
			ExcludedPaths:     excludedPaths,
			ByTenant:          true,
			ByIP:              true,
			ByUser:            false,
		}
		router.Use(gosharedmw.RedisRateLimitMiddlewareWithConfig(redisClient, rateLimitConfig))
		log.Println("✓ Redis-based rate limiting enabled (storefront-theme excluded)")
	} else {
		// Use custom in-memory rate limit config with same exclusions as Redis config
		inMemoryConfig := gosharedmw.RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         20,
			ExcludePaths:      excludedPaths,
			CleanupInterval:   5 * time.Minute,
			TTL:               10 * time.Minute,
		}
		limiter := gosharedmw.NewRateLimiter(inMemoryConfig)
		router.Use(limiter.Middleware())
		log.Println("✓ In-memory rate limiting enabled (storefront-theme excluded, Redis unavailable)")
	}

	router.Use(middleware.SetupCORS())
	router.Use(health.MetricsMiddleware()) // Prometheus metrics middleware

	// Health and observability endpoints (no auth required)
	router.GET("/health", healthChecker.HealthHandler)
	router.GET("/livez", healthChecker.LivezHandler)
	router.GET("/readyz", healthChecker.ReadyzHandler)
	router.GET("/metrics", health.MetricsHandler())

	// ========================================
	// Internal service-to-service API routes (no user auth required)
	// These endpoints are called by other internal services via X-Internal-Service header
	// Protected at network level by Kubernetes network policies and Istio mTLS
	// ========================================
	internalV1 := router.Group("/api/v1")
	internalV1.Use(middleware.InternalServiceMiddleware()) // Only allows internal services
	{
		// Tenant audit config - used by audit-service to get tenant database config
		internalV1.GET("/tenants/:id/audit-config", tenantHandler.GetAuditConfig)
		internalV1.GET("/tenants/audit-enabled", tenantHandler.ListAuditEnabledTenants)
	}

	// ========================================
	// Public API routes (no auth required)
	// These are read-only endpoints for public storefronts
	// ========================================
	publicV1 := router.Group("/api/v1/public")
	publicV1.Use(middleware.TenantMiddleware()) // Still need tenant context
	{
		// Public storefront theme endpoint - allows storefronts to read theme settings
		publicV1.GET("/storefront-theme/:storefrontId", storefrontThemeHandler.GetStorefrontTheme)
		// Public theme presets
		publicV1.GET("/storefront-theme/presets", storefrontThemeHandler.GetThemePresets)
		// Public settings context endpoint - allows storefronts to read marketing/localization settings
		// Uses tenantId from query parameter instead of X-Tenant-ID header
		publicV1.GET("/settings/context", settingsHandler.GetPublicSettingsByContext)
	}

	// Initialize Istio auth middleware for Keycloak JWT validation
	// During migration, AllowLegacyHeaders enables fallback to X-* headers from auth-bff
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true, // Allow X-User-ID, X-Tenant-ID during migration
		Logger:             istioAuthLogger,
	})

	// API v1 routes with RBAC
	v1 := router.Group("/api/v1")

	// Authentication middleware
	// In development: use legacy header extraction for local testing
	// In production: use IstioAuth which reads x-jwt-claim-* headers from Istio
	if cfg.Server.Mode == gin.ReleaseMode {
		v1.Use(istioAuth)
	} else {
		// Development mode: use header extraction middleware
		v1.Use(middleware.TenantMiddleware())
		v1.Use(middleware.AuthMiddleware())
	}
	{
		// Settings endpoints with RBAC
		settings := v1.Group("/settings")
		{
			settings.POST("/", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), settingsHandler.CreateSettings)
			settings.GET("/", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.ListSettings)
			settings.GET("/context", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.GetSettingsByContext)
			settings.GET("/inherited", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.GetInheritedSettings)
			settings.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.GetSettings)
			settings.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), settingsHandler.UpdateSettings)
			settings.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), settingsHandler.DeleteSettings)
			settings.GET("/:id/history", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.GetSettingsHistory)
			settings.POST("/:settingsId/apply-preset/:presetId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), settingsHandler.ApplyPreset)
		}

		// Presets endpoints with RBAC
		presets := v1.Group("/presets")
		{
			presets.GET("/", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), settingsHandler.ListPresets)
		}

		// Storefront Theme endpoints with RBAC
		storefrontTheme := v1.Group("/storefront-theme")
		{
			storefrontTheme.GET("/presets", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), storefrontThemeHandler.GetThemePresets)
			storefrontTheme.GET("/:tenantId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), storefrontThemeHandler.GetStorefrontTheme)
			storefrontTheme.POST("/:tenantId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.CreateOrUpdateStorefrontTheme)
			storefrontTheme.PATCH("/:tenantId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.UpdateStorefrontTheme)
			storefrontTheme.DELETE("/:tenantId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.DeleteStorefrontTheme)
			storefrontTheme.POST("/:tenantId/apply-preset/:presetId", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.ApplyThemePreset)
			storefrontTheme.POST("/:tenantId/clone", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.CloneTheme)
			// History endpoints
			storefrontTheme.GET("/:tenantId/history", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), storefrontThemeHandler.GetThemeHistory)
			storefrontTheme.GET("/:tenantId/history/:version", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), storefrontThemeHandler.GetThemeHistoryVersion)
			storefrontTheme.POST("/:tenantId/restore/:version", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), storefrontThemeHandler.RestoreThemeVersion)
		}

		// Currency endpoints with RBAC
		currency := v1.Group("/currency")
		{
			currency.GET("/convert", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.Convert)
			currency.POST("/bulk-convert", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.BulkConvert)
			currency.GET("/rates", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.GetRates)
			currency.GET("/rate", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.GetRate)
			currency.GET("/supported", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.GetSupportedCurrencies)
			currency.POST("/refresh", rbacMiddleware.RequirePermission(rbac.PermissionSettingsUpdate), currencyHandler.RefreshRates)
			currency.GET("/status", rbacMiddleware.RequirePermission(rbac.PermissionSettingsRead), currencyHandler.GetUpdaterStatus)
		}
		// Note: Tenant audit config endpoints are registered above in the internal service group
		// to allow service-to-service calls without user authentication
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}