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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/config"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/events"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/handlers"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/middleware"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/providers"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/repository"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/services"
	"github.com/tesseract-hub/go-shared/metrics"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate models
	if err := autoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repositories
	verificationRepo := repository.NewVerificationRepository(db)
	rateLimitRepo := repository.NewRateLimitRepository(db)

	// Initialize email provider
	emailProvider, err := providers.EmailProviderFactory(
		cfg.Email.Provider,
		cfg.Email.APIKey,
		cfg.Email.FromEmail,
		cfg.Email.FromName,
	)
	if err != nil {
		log.Fatalf("Failed to initialize email provider: %v", err)
	}

	// Initialize services
	verificationService, err := services.NewVerificationService(
		cfg,
		verificationRepo,
		rateLimitRepo,
		emailProvider,
	)
	if err != nil {
		log.Fatalf("Failed to initialize verification service: %v", err)
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	verificationHandler := handlers.NewVerificationHandler(verificationService)

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

	// Initialize metrics
	metricsCollector := initMetrics(db)

	// Setup router
	router := setupRouter(cfg, healthHandler, verificationHandler, metricsCollector)

	// Setup server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting verification-service on port %s", cfg.Server.Port)
		log.Printf("Email provider: %s", emailProvider.GetName())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter(cfg *config.Config, healthHandler *handlers.HealthHandler, verificationHandler *handlers.VerificationHandler, metricsCollector *metrics.Metrics) *gin.Engine {
	// Set Gin mode
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(metricsCollector.Middleware())

	// Health endpoints (no auth required)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes (with API key authentication)
	v1 := router.Group("/api/v1")
	v1.Use(middleware.APIKeyAuth(cfg.Security.APIKey))
	{
		// Verification endpoints
		v1.POST("/verify/send", verificationHandler.SendCode)
		v1.POST("/verify/code", verificationHandler.VerifyCode)
		v1.POST("/verify/resend", verificationHandler.ResendCode)
		v1.GET("/verify/status", verificationHandler.GetStatus)

		// Email endpoints
		v1.POST("/email/send", verificationHandler.SendEmail)
	}

	return router
}

func autoMigrate(db *gorm.DB) error {
	log.Println("Starting database migration...")

	// Enable UUID extension in PostgreSQL
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		log.Printf("Warning: Failed to create uuid-ossp extension: %v", err)
	}

	// Auto-migrate all models
	modelsToMigrate := []interface{}{
		&models.VerificationCode{},
		&models.VerificationAttempt{},
		&models.RateLimit{},
	}

	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	log.Println("Database migration completed successfully")
	return nil
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	// Connect to database
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Connected to database successfully")
	return db, nil
}

func initMetrics(db *gorm.DB) *metrics.Metrics {
	// Initialize base metrics collector
	m := metrics.New(metrics.Config{
		ServiceName: "verification-service",
		Namespace:   "tesseract",
		Subsystem:   "verification",
	})

	// Register verification-specific business metrics
	codesGenerated := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "codes_generated_total",
			Help:      "Total number of verification codes generated",
		},
		[]string{"type"}, // email, phone
	)

	attemptsTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "attempts_total",
			Help:      "Total number of verification attempts",
		},
		[]string{"type", "result"}, // type: email/phone, result: success/failure
	)

	rateLimitsHit := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "rate_limits_hit_total",
			Help:      "Total number of rate limit violations",
		},
		[]string{"type"}, // send, verify
	)

	activeVerifications := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "active_verifications",
			Help:      "Number of currently active (pending) verifications",
		},
	)

	// Database connection pool metrics
	dbConnectionsOpen := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "db_connections_open",
			Help:      "Number of open database connections",
		},
	)

	dbConnectionsInUse := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "db_connections_in_use",
			Help:      "Number of database connections currently in use",
		},
	)

	dbConnectionsIdle := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tesseract",
			Subsystem: "verification",
			Name:      "db_connections_idle",
			Help:      "Number of idle database connections",
		},
	)

	// Start goroutine to update database and active verification metrics periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Update database connection pool metrics
			sqlDB, err := db.DB()
			if err != nil {
				log.Printf("Failed to get database instance for metrics: %v", err)
				continue
			}

			stats := sqlDB.Stats()
			dbConnectionsOpen.Set(float64(stats.OpenConnections))
			dbConnectionsInUse.Set(float64(stats.InUse))
			dbConnectionsIdle.Set(float64(stats.Idle))

			// Update active verifications count
			var count int64
			db.Model(&models.VerificationCode{}).
				Where("is_used = ? AND verified_at IS NULL AND expires_at > ?", false, time.Now()).
				Count(&count)
			activeVerifications.Set(float64(count))
		}
	}()

	// Log metrics initialization
	log.Println("Metrics initialized successfully")
	log.Printf("Registered metrics: codes_generated_total, attempts_total, rate_limits_hit_total, active_verifications")
	log.Printf("Database metrics: db_connections_open, db_connections_in_use, db_connections_idle")

	// Suppress unused variable warnings (these will be used by handlers in the future)
	_ = codesGenerated
	_ = attemptsTotal
	_ = rateLimitsHit

	return m
}
