package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"analytics-service/internal/clients"
	"analytics-service/internal/config"
	"analytics-service/internal/events"
	"analytics-service/internal/fdw"
	"analytics-service/internal/handlers"
	"analytics-service/internal/middleware"
	"analytics-service/internal/repository"
	"analytics-service/internal/services"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
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

	// Initialize FDW (Foreign Data Wrapper) for cross-database queries
	// This is critical for analytics - without FDW, the service cannot query orders/customers/products
	fdwConfig := &fdw.FDWConfig{
		DBHost:     cfg.DBHost,
		DBPort:     fmt.Sprintf("%d", cfg.DBPort),
		DBUser:     cfg.DBUser,
		DBPassword: cfg.DBPassword,
		// FDWServerHost: CRITICAL - this must be "localhost" when all databases are on
		// the same PostgreSQL instance. FDW connections happen FROM PostgreSQL itself,
		// not from this application. Using the K8s service hostname would fail because
		// PostgreSQL cannot resolve K8s DNS names.
		FDWServerHost:   getEnv("FDW_SERVER_HOST", "localhost"),
		ProductsDB:      getEnv("FDW_PRODUCTS_DB", "products_db"),
		OrdersDB:        getEnv("FDW_ORDERS_DB", "orders_db"),
		CustomersDB:     getEnv("FDW_CUSTOMERS_DB", "customers_db"),
		ProductsTables:  []string{"products"},
		OrdersTables:    []string{"orders", "order_items"},
		CustomersTables: []string{"customers"},
		Enabled:         getEnv("FDW_ENABLED", "true") == "true",
		MaxRetries:      5,
		RetryInterval:   10 * time.Second,
	}

	fdwManager := fdw.NewFDWManager(db, fdwConfig, logger)

	// Initialize FDW with timeout
	fdwCtx, fdwCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer fdwCancel()

	if err := fdwManager.Initialize(fdwCtx); err != nil {
		// FDW initialization failed - this is critical
		// Log error but don't exit - allow health checks to report unhealthy
		logger.WithError(err).Error("FDW initialization failed - analytics queries will not work")
	} else {
		logger.Info("FDW initialized successfully - cross-database queries ready")
	}

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

		// Check FDW health (critical for analytics queries)
		fdwHealthy, fdwErr := fdwManager.IsHealthy(c.Request.Context())
		if !fdwHealthy {
			errMsg := "fdw not initialized"
			if fdwErr != nil {
				errMsg = fdwErr.Error()
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not ready",
				"error":     "fdw health check failed",
				"fdw_error": errMsg,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready", "fdw": "healthy"})
	})

	// FDW status endpoint for debugging
	router.GET("/fdw/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, fdwManager.Status())
	})

	// FDW reinitialize endpoint (for manual recovery)
	router.POST("/fdw/reinitialize", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()

		if err := fdwManager.Reinitialize(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "reinitialized"})
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

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
