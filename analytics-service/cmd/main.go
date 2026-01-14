package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/tesseract-hub/analytics-service/internal/clients"
	"github.com/tesseract-hub/analytics-service/internal/config"
	"github.com/tesseract-hub/analytics-service/internal/events"
	"github.com/tesseract-hub/analytics-service/internal/handlers"
	"github.com/tesseract-hub/analytics-service/internal/middleware"
	"github.com/tesseract-hub/analytics-service/internal/repository"
	"github.com/tesseract-hub/analytics-service/internal/services"

	gosharedmw "github.com/tesseract-hub/go-shared/middleware"
	"github.com/tesseract-hub/go-shared/rbac"
)

func main() {
	// Load .env file if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger := logrus.New()
	if cfg.LogFormat == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	switch cfg.LogLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Info("Starting Analytics Service...")

	// Initialize database
	db, err := config.InitDB(cfg)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	logger.Info("Connected to database")

	// Initialize tenant client for slug resolution
	tenantClient := clients.NewTenantClient()
	logger.Info("Initialized tenant client for slug resolution")

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	logger.Info("RBAC middleware initialized")

	// Initialize repository, service, and handlers
	analyticsRepo := repository.NewAnalyticsRepository(db)
	analyticsService := services.NewAnalyticsService(analyticsRepo, logger)
	analyticsHandlers := handlers.NewAnalyticsHandlers(analyticsService, logger)

	// Initialize NATS events publisher (non-blocking)
	go func() {
		if err := events.InitPublisher(logger); err != nil {
			logger.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
		} else {
			logger.Info("NATS events publisher initialized")
		}
	}()

	// Setup router
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Apply middleware
	router.Use(middleware.RecoveryMiddleware(logger))
	router.Use(middleware.LoggerMiddleware(logger))
	router.Use(middleware.CORSMiddleware())

	// Health check endpoints (no tenant required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "analytics-service"})
	})
	router.GET("/ready", func(c *gin.Context) {
		// Check database connection
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database error"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database ping failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// API routes (tenant required - with slug resolution)
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true,
		Logger:             istioAuthLogger,
	})

	// Authentication middleware
	if cfg.Environment == "development" {
		api.Use(middleware.AuthMiddleware())
		api.Use(middleware.TenantMiddlewareWithResolver(tenantClient))
	} else {
		api.Use(istioAuth)
		api.Use(gosharedmw.VendorScopeFilter())
	}
	{
		analytics := api.Group("/analytics")
		{
			// Dashboard endpoints with RBAC
			analytics.GET("/sales", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsRead), analyticsHandlers.GetSalesDashboard)
			analytics.GET("/inventory", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsRead), analyticsHandlers.GetInventoryReport)
			analytics.GET("/customers", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsRead), analyticsHandlers.GetCustomerAnalytics)
			analytics.GET("/financial", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsRead), analyticsHandlers.GetFinancialReport)

			// Export endpoints with RBAC
			analytics.GET("/sales/export", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsExport), analyticsHandlers.ExportSalesReport)
			analytics.GET("/inventory/export", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsExport), analyticsHandlers.ExportInventoryReport)
			analytics.GET("/customers/export", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsExport), analyticsHandlers.ExportCustomerReport)
			analytics.GET("/financial/export", rbacMiddleware.RequirePermission(rbac.PermissionAnalyticsExport), analyticsHandlers.ExportFinancialReport)
		}
	}

	// Start server
	addr := cfg.GetServerAddress()
	logger.WithField("address", addr).Info("Analytics Service listening")
	if err := router.Run(addr); err != nil {
		logger.WithError(err).Fatal("Failed to start server")
	}
}
