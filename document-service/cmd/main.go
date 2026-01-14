package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"document-service/internal/config"
	"document-service/internal/events"
	"document-service/internal/handlers"
	"document-service/internal/middleware"
	"document-service/internal/models"
	"document-service/internal/providers/aws"
	"document-service/internal/providers/gcp"
	"document-service/internal/providers/local"
	// "document-service/internal/providers/azure" // Temporarily disabled due to API compatibility issues
	"document-service/internal/cache"
	"document-service/internal/repository"
	"document-service/internal/service"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info("Starting Document Service")

	// Connect to database
	db, err := connectDatabase(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}

	// Run migrations
	if err := runMigrations(db, logger); err != nil {
		logger.WithError(err).Fatal("Failed to run database migrations")
	}

	// Create cloud storage provider
	provider, err := createStorageProvider(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create storage provider")
	}

	// Initialize cache (Redis)
	var redisCache cache.Cache
	if cfg.IsCacheEnabled() {
		cacheConfig := cfg.GetCacheConfig()
		redisCache, err = cache.NewRedisCache(cache.RedisConfig{
			Host:     cacheConfig.Host,
			Port:     cacheConfig.Port,
			Password: cacheConfig.Password,
			DB:       cacheConfig.DB,
			PoolSize: cacheConfig.PoolSize,
		}, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to connect to Redis, continuing without cache")
			redisCache = cache.NewNoOpCache()
		}
	} else {
		logger.Info("Cache disabled by configuration")
		redisCache = cache.NewNoOpCache()
	}

	// Initialize repository and service
	repo := repository.NewDocumentRepository(db)
	cacheConfig := cfg.GetCacheConfig()
	documentService := service.NewDocumentServiceWithOptions(provider, repo, cfg, logger, &service.ServiceOptions{
		Cache:           redisCache,
		PresignedURLTTL: time.Duration(cacheConfig.PresignedURLTTL) * time.Second,
		MetadataTTL:     time.Duration(cacheConfig.MetadataTTL) * time.Second,
	})

	// Initialize NATS events publisher (non-blocking)
	go func() {
		if err := events.InitPublisher(logger); err != nil {
			logger.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
		} else {
			logger.Info("NATS events publisher initialized")
		}
	}()

	// Setup HTTP server
	router := setupRouter(cfg, documentService, logger)
	server := &http.Server{
		Addr:         cfg.GetAddr(),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server
	go func() {
		logger.WithField("addr", cfg.GetAddr()).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

// setupLogger configures the application logger
func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	switch cfg.Logging.Level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: cfg.Logging.TimeFormat,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: cfg.Logging.TimeFormat,
		})
	}

	// Set output
	if cfg.Logging.Output == "stdout" {
		logger.SetOutput(os.Stdout)
	} else {
		// In production, you might want to write to a file
		logger.SetOutput(os.Stdout)
	}

	return logger
}

// connectDatabase establishes database connection
func connectDatabase(cfg *config.Config, logger *logrus.Logger) (*gorm.DB, error) {
	// Setup GORM logger
	gormLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		gormlogger.Config{
			SlowThreshold:             time.Second,       // Slow SQL threshold
			LogLevel:                  gormlogger.Silent, // Log level
			IgnoreRecordNotFoundError: true,              // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,             // Disable color
		},
	)

	if cfg.IsDevelopment() {
		gormLogger = gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormlogger.Info,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		)
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.MaxLifetime) * time.Second)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Connected to database successfully")
	return db, nil
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB, logger *logrus.Logger) error {
	logger.Info("Running database migrations")

	// Auto-migrate the schema
	if err := db.AutoMigrate(&models.Document{}); err != nil {
		return fmt.Errorf("failed to migrate Document model: %w", err)
	}

	// Create unique index on path with IF NOT EXISTS to avoid errors on restart
	// GORM's AutoMigrate doesn't support IF NOT EXISTS for unique constraints
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uni_documents_path ON documents(path) WHERE deleted_at IS NULL").Error; err != nil {
		logger.WithError(err).Warn("Failed to create unique index on path (may already exist)")
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// createStorageProvider creates the appropriate cloud storage provider
func createStorageProvider(cfg *config.Config, logger *logrus.Logger) (models.CloudStorageProvider, error) {
	switch cfg.Storage.Provider {
	case models.ProviderAWS:
		return aws.NewS3Provider(&cfg.Storage.AWS, logger)
	case models.ProviderAzure:
		// Azure provider temporarily disabled due to API compatibility issues
		logger.Warn("Azure provider is temporarily disabled due to compatibility issues, falling back to AWS")
		cfg.Storage.Provider = models.ProviderAWS
		return aws.NewS3Provider(&cfg.Storage.AWS, logger)
		// return azure.NewBlobProvider(&cfg.Storage.Azure, logger)
	case models.ProviderGCP:
		return gcp.NewGCSProvider(&cfg.Storage.GCP, logger)
	case models.ProviderLocal:
		logger.Info("Using local filesystem storage provider")
		return local.NewLocalProvider(&cfg.Storage.Local, logger)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Storage.Provider)
	}
}

// setupRouter configures the HTTP router
func setupRouter(cfg *config.Config, documentService models.DocumentService, logger *logrus.Logger) *gin.Engine { //nolint:funlen
	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(gosharedmw.CompressionMiddleware())

	// Add CORS middleware if enabled
	if cfg.Security.EnableCORS {
		router.Use(corsMiddleware(cfg))
	}

	// Add rate limiting if enabled
	if cfg.Security.EnableRateLimit {
		// TODO: Implement rate limiting middleware
	}

	// Create handlers
	documentHandler := handlers.NewDocumentHandler(documentService, cfg, logger)
	healthHandler := handlers.NewHealthHandler(documentService, logger)

	// Health check routes (no auth required)
	health := router.Group("/health")
	{
		health.GET("", healthHandler.Health)
		health.GET("/ready", healthHandler.Ready)
		health.GET("/live", healthHandler.Live)
	}

	// Protected API routes
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	istioAuthLogger := logrus.NewEntry(logrus.StandardLogger()).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true,
		Logger:             istioAuthLogger,
	})

	// Add authentication middleware
	if cfg.Security.EnableAuth {
		if cfg.IsDevelopment() {
			api.Use(middleware.DevelopmentAuthMiddleware())
			api.Use(middleware.TenantMiddleware(logger))
		} else {
			api.Use(istioAuth)
			api.Use(gosharedmw.VendorScopeFilter())
		}
	}

	// Add product middleware for multi-product support
	api.Use(middleware.ProductMiddleware(logger))
	{
		documents := api.Group("/documents")
		{
			documents.POST("/upload", documentHandler.UploadDocument)
			documents.GET("", documentHandler.ListDocuments)
			documents.POST("/presigned-url", documentHandler.GeneratePresignedURL)
			documents.POST("/copy", documentHandler.CopyDocument)
			documents.POST("/move", documentHandler.MoveDocument)

			// Batch operations
			batch := documents.Group("/batch")
			{
				batch.POST("/delete", documentHandler.BatchDeleteDocuments)
			}

			// Document metadata and operations (must be before wildcard routes)
			documents.GET("/:bucket/metadata/*path", documentHandler.GetDocumentMetadata)
			documents.PATCH("/:bucket/metadata/*path", documentHandler.UpdateDocumentMetadata)
			documents.GET("/:bucket/exists/*path", documentHandler.DocumentExists)

			// Document download/delete (wildcard routes - must be last)
			documents.GET("/:bucket/file/*path", documentHandler.DownloadDocument)
			documents.DELETE("/:bucket/file/*path", documentHandler.DeleteDocument)
		}

		storage := api.Group("/storage")
		{
			storage.GET("/usage", documentHandler.GetStorageUsage)
			storage.GET("/config", documentHandler.GetBucketConfig)
		}

		// Public URL endpoint (for marketplace assets)
		documents.GET("/public/*path", documentHandler.GetPublicURL)
	}

	return router
}

// corsMiddleware adds CORS headers
func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if len(cfg.Security.AllowedOrigins) > 0 {
			allowed := false
			for _, allowedOrigin := range cfg.Security.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")

		if len(cfg.Security.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", strings.Join(cfg.Security.AllowedHeaders, ", "))
		} else {
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID, X-User-ID, X-Product-ID")
		}

		if len(cfg.Security.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", strings.Join(cfg.Security.AllowedMethods, ", "))
		} else {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
