package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/clients"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/config"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/handlers"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/middleware"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/repository"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/services"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/validators"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetOutput(os.Stdout)

	// Load configuration
	cfg := config.NewConfig()

	// Set log level based on environment
	if cfg.Server.IsProd() {
		logger.SetLevel(logrus.InfoLevel)
		gin.SetMode(gin.ReleaseMode)
	} else {
		logger.SetLevel(logrus.DebugLevel)
		gin.SetMode(gin.DebugMode)
	}

	log := logger.WithField("service", "secret-provisioner")
	log.Info("starting secret-provisioner service")

	// Validate required configuration
	if cfg.GCP.ProjectID == "" {
		log.Fatal("GCP_PROJECT_ID is required")
	}

	// Initialize database
	db, err := initDatabase(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize database")
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.WithError(err).Fatal("failed to run migrations")
	}

	// Initialize GCP Secret Manager client
	ctx := context.Background()
	gcpClient, err := clients.NewGCPSecretManagerClient(ctx, cfg.GCP.ProjectID, log)
	if err != nil {
		log.WithError(err).Fatal("failed to create GCP Secret Manager client")
	}
	defer gcpClient.Close()

	// Initialize repositories
	metadataRepo := repository.NewSecretMetadataRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	// Initialize validator registry
	validatorRegistry := validators.NewRegistry()

	// Initialize service
	provisionerService := services.NewProvisionerService(
		cfg,
		gcpClient,
		metadataRepo,
		auditRepo,
		validatorRegistry,
		log,
	)

	// Initialize naming migration service
	migrationService := services.NewNamingMigrationService(
		cfg,
		gcpClient,
		metadataRepo,
		auditRepo,
		log,
	)

	// Run naming migration on startup to fix any inconsistent secret names
	log.Info("running startup naming migration check")
	if result, err := migrationService.RunMigration(ctx, false); err != nil {
		log.WithError(err).Warn("naming migration check failed, continuing startup")
	} else {
		log.WithFields(logrus.Fields{
			"scanned":  result.SecretsScanned,
			"migrated": result.SecretsMigrated,
			"skipped":  result.SecretsSkipped,
		}).Info("naming migration check completed")
	}

	// Initialize handlers
	secretHandler := handlers.NewSecretHandler(provisionerService, log)
	healthHandler := handlers.NewHealthHandler(db)
	migrationHandler := handlers.NewMigrationHandler(migrationService, log)

	// Setup router
	router := setupRouter(cfg, secretHandler, healthHandler, migrationHandler, log)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.WithField("addr", srv.Addr).Info("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("server forced to shutdown")
	}

	log.Info("server stopped")
}

func initDatabase(cfg *config.Config, log *logrus.Entry) (*gorm.DB, error) {
	// Configure GORM logger
	var gormLog gormlogger.Interface
	if cfg.Server.IsProd() {
		gormLog = gormlogger.Default.LogMode(gormlogger.Silent)
	} else {
		gormLog = gormlogger.Default.LogMode(gormlogger.Info)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	log.Info("database connection established")
	return db, nil
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.SecretMetadata{},
		&models.SecretAuditLog{},
	)
}

func setupRouter(cfg *config.Config, secretHandler *handlers.SecretHandler, healthHandler *handlers.HealthHandler, migrationHandler *handlers.MigrationHandler, log *logrus.Entry) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.RequestLogger(log))

	// Health endpoints (no auth)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1
	v1 := router.Group("/api/v1")
	v1.Use(middleware.VerifyInternalService(cfg.Auth.AllowedServices))
	v1.Use(middleware.TenantID())
	{
		secrets := v1.Group("/secrets")
		{
			secrets.POST("", secretHandler.ProvisionSecrets)
			secrets.GET("/metadata", secretHandler.GetMetadata)
			secrets.GET("/providers", secretHandler.ListProviders)
			secrets.DELETE("/:name", secretHandler.DeleteSecret)
		}

		// Admin endpoints for maintenance operations
		admin := v1.Group("/admin")
		{
			migration := admin.Group("/migration")
			{
				// Check what secrets need migration (dry-run)
				migration.GET("/naming/check", migrationHandler.CheckMigration)
				// Run the naming migration
				migration.POST("/naming", migrationHandler.RunMigration)
			}
		}
	}

	return router
}
