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
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"notification-service/internal/config"
	"notification-service/internal/handlers"
	"notification-service/internal/middleware"
	"notification-service/internal/models"
	"notification-service/internal/nats"
	"notification-service/internal/repository"
	"notification-service/internal/services"

	"github.com/sirupsen/logrus"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate database schema
	if err := migrateDatabase(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize providers
	emailProvider := initEmailProvider(cfg)
	smsProvider := initSMSProvider(cfg)
	pushProvider := initPushProvider(cfg)

	// Initialize repositories
	notifRepo := repository.NewNotificationRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	prefRepo := repository.NewPreferenceRepository(db)

	// Initialize Redis client for email rate limiting (optional)
	var redisClient *redis.Client
	if cfg.Redis.Host != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v - email rate limiting will use in-memory fallback", err)
			redisClient = nil
		} else {
			log.Println("✓ Redis connected for email rate limiting")
		}
	}

	// Initialize email rate limiter (if enabled)
	var emailRateLimiter *middleware.EmailRateLimiter
	if cfg.EmailRateLimit.Enabled {
		emailRateLimitConfig := middleware.EmailRateLimitConfig{
			TenantHourlyLimit:    cfg.EmailRateLimit.TenantHourlyLimit,
			TenantDailyLimit:     cfg.EmailRateLimit.TenantDailyLimit,
			RecipientHourlyLimit: cfg.EmailRateLimit.RecipientHourlyLimit,
			PasswordResetLimit:   cfg.EmailRateLimit.PasswordResetHourlyMax,
			VerificationLimit:    cfg.EmailRateLimit.VerificationHourlyMax,
			PasswordResetWindow:  time.Hour,
			VerificationWindow:   time.Hour,
			RedisKeyPrefix:       "email:ratelimit:",
		}
		emailRateLimiter = middleware.NewEmailRateLimiterWithConfig(redisClient, logrus.StandardLogger(), emailRateLimitConfig)
		log.Printf("✓ Email rate limiting enabled (tenant: %d/hour, %d/day; recipient: %d/hour)",
			cfg.EmailRateLimit.TenantHourlyLimit, cfg.EmailRateLimit.TenantDailyLimit, cfg.EmailRateLimit.RecipientHourlyLimit)
	}

	// Initialize verify service (optional - for OTP/account verification)
	var verifyService *services.VerifyService
	if cfg.Verify.TwilioVerifyServiceSID != "" {
		var err error
		verifyService, err = services.NewVerifyService(&cfg.Verify)
		if err != nil {
			log.Printf("Warning: Failed to initialize Twilio Verify: %v - OTP features disabled", err)
		} else {
			log.Printf("Twilio Verify initialized (auth mode: api_key)")
		}
	} else {
		log.Println("Warning: Twilio Verify not configured - OTP features disabled")
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	notifHandler := handlers.NewNotificationHandler(
		notifRepo,
		templateRepo,
		prefRepo,
		emailProvider,
		smsProvider,
		pushProvider,
	)
	// Set rate limiter if enabled
	if emailRateLimiter != nil {
		notifHandler.SetRateLimiter(emailRateLimiter)
	}
	templateHandler := handlers.NewTemplateHandler(templateRepo)
	prefHandler := handlers.NewPreferenceHandler(prefRepo)
	var verifyHandler *handlers.VerifyHandler
	if verifyService != nil {
		verifyHandler = handlers.NewVerifyHandler(verifyService, cfg.Verify.DevtestEnabled, cfg.Verify.TestPhoneNumber)
		log.Printf("[VERIFY] DevTest mode: %v (test phone: %s)", cfg.Verify.DevtestEnabled, cfg.Verify.TestPhoneNumber)
	}

	// Initialize NATS subscriber (optional - service works without it)
	var natsSubscriber *nats.Subscriber
	natsClient, err := nats.NewClient(cfg.NATS.URL, cfg.NATS.MaxReconnects, cfg.NATS.ReconnectWait)
	if err != nil {
		log.Printf("Warning: Failed to connect to NATS: %v - event-driven notifications disabled", err)
	} else {
		natsSubscriber = nats.NewSubscriber(
			natsClient,
			notifRepo,
			templateRepo,
			prefRepo,
			emailProvider,
			smsProvider,
			pushProvider,
			cfg.App.AdminEmail,
			cfg.App.SupportEmail,
		)
		if err := natsSubscriber.Start(context.Background()); err != nil {
			log.Printf("Warning: Failed to start NATS subscriber: %v", err)
		}
	}

	// Setup router
	router := setupRouter(cfg, healthHandler, notifHandler, templateHandler, prefHandler, verifyHandler)

	// Start server with graceful shutdown
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Notification Service on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Notification Service...")

	// Stop NATS subscriber
	if natsSubscriber != nil {
		natsSubscriber.Stop()
	}
	if natsClient != nil {
		natsClient.Close()
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Notification Service stopped")
}

// initDatabase initializes the database connection
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	gormConfig := &gorm.Config{}
	if cfg.App.Environment == "production" {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), gormConfig)
	if err != nil {
		return nil, err
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// migrateDatabase runs database migrations
func migrateDatabase(db *gorm.DB) error {
	// Run AutoMigrate for all models
	// GORM AutoMigrate safely handles existing tables:
	// - Creates tables if they don't exist
	// - Adds missing columns to existing tables
	// - Creates missing indexes
	// - Does NOT drop columns, change types, or delete data
	modelsToMigrate := []interface{}{
		&models.Notification{},
		&models.NotificationTemplate{},
		&models.NotificationPreference{},
		&models.NotificationLog{},
		&models.NotificationBatch{},
	}

	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	log.Println("Database migration completed successfully")
	return nil
}

// initEmailProvider initializes the email provider with failover chain
// Priority: Postal (primary) -> AWS SES (secondary) -> SendGrid (tertiary)
//
// Postal is the primary self-hosted option with full template control.
// AWS SES is the secondary managed fallback. SendGrid is used as final fallback.
func initEmailProvider(cfg *config.Config) services.Provider {
	var providers []services.Provider

	// 1. Primary: Postal HTTP API (self-hosted, full template control)
	if cfg.Email.PostalAPIURL != "" && cfg.Email.PostalAPIKey != "" {
		postalHTTPConfig := &services.PostalHTTPConfig{
			APIURL:   cfg.Email.PostalAPIURL,
			APIKey:   cfg.Email.PostalAPIKey,
			From:     cfg.Email.PostalFrom,
			FromName: cfg.Email.PostalFromName,
		}
		postalHTTP := services.NewPostalHTTPProvider(postalHTTPConfig)
		providers = append(providers, postalHTTP)
		log.Printf("Email provider configured: Postal-HTTP (primary) - %s", cfg.Email.PostalAPIURL)
	} else if cfg.Email.PostalHost != "" {
		// Postal SMTP (if HTTP API not configured)
		postalConfig := &services.ProviderConfig{
			PostalHost:     cfg.Email.PostalHost,
			PostalPort:     cfg.Email.PostalPort,
			PostalUsername: cfg.Email.PostalUsername,
			PostalPassword: cfg.Email.PostalPassword,
			PostalFrom:     cfg.Email.PostalFrom,
			PostalFromName: cfg.Email.PostalFromName,
		}
		postal := services.NewPostalProvider(postalConfig)
		providers = append(providers, postal)
		log.Printf("Email provider configured: Postal-SMTP (primary) - %s:%d", cfg.Email.PostalHost, cfg.Email.PostalPort)
	}

	// 2. Secondary: AWS SES (managed, reliable fallback)
	if cfg.Email.SESFrom != "" && (cfg.AWS.AccessKeyID != "" || cfg.AWS.Region != "") {
		sesConfig := &services.ProviderConfig{
			AWSRegion:          cfg.AWS.Region,
			AWSAccessKeyID:     cfg.AWS.AccessKeyID,
			AWSSecretAccessKey: cfg.AWS.SecretAccessKey,
			SESFrom:            cfg.Email.SESFrom,
			SESFromName:        cfg.Email.SESFromName,
		}
		sesProvider, err := services.NewSESProvider(sesConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize AWS SES: %v", err)
		} else {
			providers = append(providers, sesProvider)
			log.Printf("Email provider configured: AWS SES (secondary) - region: %s", cfg.AWS.Region)
		}
	}

	// 3. Tertiary: SendGrid API (final fallback)
	if cfg.Email.SendGridAPIKey != "" {
		sendgridConfig := &services.ProviderConfig{
			SendGridAPIKey: cfg.Email.SendGridAPIKey,
			SendGridFrom:   cfg.Email.SendGridFrom,
		}
		sendgrid := services.NewSendGridProvider(sendgridConfig)
		providers = append(providers, sendgrid)
		log.Printf("Email provider configured: SendGrid (tertiary - fallback)")
	}

	// 4. Quaternary: Mautic (newsletters + marketing automation)
	if cfg.Email.MauticURL != "" {
		mauticConfig := &services.ProviderConfig{
			MauticURL:      cfg.Email.MauticURL,
			MauticUsername: cfg.Email.MauticUsername,
			MauticPassword: cfg.Email.MauticPassword,
			MauticFrom:     cfg.Email.MauticFrom,
			MauticFromName: cfg.Email.MauticFromName,
		}
		mautic := services.NewMauticProvider(mauticConfig)
		providers = append(providers, mautic)
		log.Printf("Email provider configured: Mautic (quaternary) - %s", cfg.Email.MauticURL)
	}

	// 5. Legacy SMTP fallback (if no other providers)
	if len(providers) == 0 && cfg.Email.SMTPHost != "" {
		smtpConfig := &services.ProviderConfig{
			SMTPHost:     cfg.Email.SMTPHost,
			SMTPPort:     cfg.Email.SMTPPort,
			SMTPUsername: cfg.Email.SMTPUsername,
			SMTPPassword: cfg.Email.SMTPPassword,
			SMTPFrom:     cfg.Email.SMTPFrom,
		}
		smtp := services.NewSMTPProvider(smtpConfig)
		providers = append(providers, smtp)
		log.Printf("Email provider configured: Legacy SMTP - %s:%d", cfg.Email.SMTPHost, cfg.Email.SMTPPort)
	}

	if len(providers) == 0 {
		log.Println("Warning: No email provider configured")
		return nil
	}

	// Create failover provider
	failoverConfig := &services.FailoverConfig{
		EnableFailover: cfg.Email.EnableFailover,
		MaxRetries:     1,
		RetryDelay:     2 * time.Second,
	}

	failover := services.NewFailoverEmailProvider(providers, failoverConfig)
	log.Printf("Email failover chain initialized: %s (failover=%v)", failover.GetName(), cfg.Email.EnableFailover)

	return failover
}

// initSMSProvider initializes the SMS provider with failover chain
// Priority: AWS SNS (primary) -> Twilio (fallback)
//
// AWS SNS is the primary SMS service for reliability and cost-effectiveness.
// Twilio is used as fallback when SNS is unavailable.
func initSMSProvider(cfg *config.Config) services.Provider {
	var providers []services.Provider

	// 1. Primary: AWS SNS (managed, reliable, cost-effective)
	if cfg.AWS.Region != "" {
		snsConfig := &services.ProviderConfig{
			AWSRegion:          cfg.AWS.Region,
			AWSAccessKeyID:     cfg.AWS.AccessKeyID,
			AWSSecretAccessKey: cfg.AWS.SecretAccessKey,
			SNSFrom:            cfg.SMS.SNSFrom,
		}
		snsProvider, err := services.NewSNSProvider(snsConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize AWS SNS: %v", err)
		} else {
			providers = append(providers, snsProvider)
			log.Printf("SMS provider configured: AWS SNS (primary) - region: %s", cfg.AWS.Region)
		}
	}

	// 2. Secondary: Twilio (fallback when SNS fails)
	if cfg.SMS.TwilioAccountSID != "" && cfg.SMS.TwilioAuthToken != "" {
		twilioConfig := &services.ProviderConfig{
			TwilioAccountSID: cfg.SMS.TwilioAccountSID,
			TwilioAuthToken:  cfg.SMS.TwilioAuthToken,
			TwilioFrom:       cfg.SMS.TwilioFrom,
		}
		twilio := services.NewTwilioProvider(twilioConfig)
		providers = append(providers, twilio)
		log.Printf("SMS provider configured: Twilio (secondary - fallback)")
	}

	if len(providers) == 0 {
		log.Println("Warning: No SMS provider configured")
		return nil
	}

	// If only one provider, return it directly
	if len(providers) == 1 {
		log.Printf("SMS provider initialized: %s", providers[0].GetName())
		return providers[0]
	}

	// Create failover provider for SMS
	failoverConfig := &services.FailoverConfig{
		EnableFailover: cfg.SMS.EnableFailover,
		MaxRetries:     1,
		RetryDelay:     2 * time.Second,
	}

	failover := services.NewFailoverSMSProvider(providers, failoverConfig)
	log.Printf("SMS failover chain initialized: %s (failover=%v)", failover.GetName(), cfg.SMS.EnableFailover)

	return failover
}

// initPushProvider initializes the push notification provider
func initPushProvider(cfg *config.Config) services.Provider {
	if cfg.Push.FCMCredentials != "" {
		providerConfig := &services.ProviderConfig{
			FCMCredentials: cfg.Push.FCMCredentials,
			FCMProjectID:   cfg.Push.FCMProjectID,
		}
		provider, err := services.NewFCMProvider(providerConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize FCM provider: %v", err)
			return nil
		}
		return provider
	}
	log.Println("Warning: No push notification provider configured")
	return nil
}

// setupRouter configures the Gin router with middleware and routes
func setupRouter(
	cfg *config.Config,
	healthHandler *handlers.HealthHandler,
	notifHandler *handlers.NotificationHandler,
	templateHandler *handlers.TemplateHandler,
	prefHandler *handlers.PreferenceHandler,
	verifyHandler *handlers.VerifyHandler,
) *gin.Engine {
	// Set Gin mode
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Setup middleware
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())

	// Health check endpoints
	router.GET("/health", healthHandler.Health)
	router.GET("/livez", healthHandler.Livez)
	router.GET("/readyz", healthHandler.Readyz)

	// API routes
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	istioAuthLogger := logrus.NewEntry(logrus.StandardLogger()).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true,
		// SkipPaths allows internal service-to-service calls without JWT
		// These endpoints are called by tenant-service during onboarding
		SkipPaths: []string{
			"/api/v1/notifications/send",
		},
		Logger: istioAuthLogger,
	})

	// Authentication middleware
	environment := os.Getenv("ENVIRONMENT")
	if environment == "development" || environment == "" {
		api.Use(middleware.TenantAuth())
	} else {
		api.Use(istioAuth)
		api.Use(gosharedmw.VendorScopeFilter())
	}
	{
		// Notifications
		notifications := api.Group("/notifications")
		{
			notifications.POST("/send", notifHandler.Send)
			notifications.GET("", notifHandler.List)
			notifications.GET("/:id", notifHandler.Get)
			notifications.GET("/:id/status", notifHandler.GetStatus)
			notifications.POST("/:id/cancel", notifHandler.Cancel)
		}

		// Templates
		templates := api.Group("/templates")
		{
			templates.GET("", templateHandler.List)
			templates.GET("/:id", templateHandler.Get)
			templates.POST("", templateHandler.Create)
			templates.PUT("/:id", templateHandler.Update)
			templates.DELETE("/:id", templateHandler.Delete)
			templates.POST("/:id/test", templateHandler.Test)
		}

		// User preferences
		preferences := api.Group("/preferences")
		{
			preferences.GET("/:userId", prefHandler.Get)
			preferences.PUT("/:userId", prefHandler.Update)
			preferences.POST("/:userId/push-token", prefHandler.RegisterPushToken)
		}

		// OTP/Verification endpoints (only if Twilio Verify is configured)
		if verifyHandler != nil {
			verify := api.Group("/verify")
			{
				verify.POST("/send", verifyHandler.SendOTP)      // Send OTP
				verify.POST("/check", verifyHandler.CheckOTP)    // Verify OTP
				verify.POST("/resend", verifyHandler.ResendOTP)  // Resend OTP
				verify.POST("/cancel", verifyHandler.CancelOTP)  // Cancel pending verification
				verify.GET("/auth-mode", verifyHandler.GetAuthMode) // Debug: get auth mode
			}
		}
	}

	// Webhooks (no auth required - validated via provider signatures)
	webhooks := router.Group("/webhooks")
	{
		webhooks.POST("/sendgrid", handleSendGridWebhook)
		webhooks.POST("/twilio", handleTwilioWebhook)
	}

	return router
}

// Webhook handlers for delivery status updates

func handleSendGridWebhook(c *gin.Context) {
	// TODO: Implement SendGrid webhook handling for delivery status updates
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func handleTwilioWebhook(c *gin.Context) {
	// TODO: Implement Twilio webhook handling for SMS delivery status
	c.JSON(http.StatusOK, gin.H{"received": true})
}
