package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"subscription-service/internal/clients"
	"subscription-service/internal/config"
	"subscription-service/internal/handlers"
	"subscription-service/internal/models"
	"subscription-service/internal/middleware"
	natsconsumer "subscription-service/internal/nats"
	"subscription-service/internal/repository"
	"subscription-service/internal/services"
)

func main() {
	cfg := config.Load()

	// Initialize Stripe
	if cfg.StripeSecretKey != "" {
		stripe.Key = cfg.StripeSecretKey
		log.Println("Stripe API key configured")
	} else {
		log.Println("WARNING: STRIPE_SECRET_KEY not set")
	}

	// Connect to database
	db, err := connectDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate subscription tables
	if err := db.AutoMigrate(
		&models.SubscriptionPlan{},
		&models.TenantSubscription{},
		&models.SubscriptionInvoice{},
		&models.SubscriptionEvent{},
	); err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	log.Println("Database schema migrated")

	// Initialize layers
	repo := repository.NewSubscriptionRepository(db)
	tenantClient := clients.NewTenantClient()

	planService := services.NewPlanService(repo)
	subscriptionService := services.NewSubscriptionService(repo, tenantClient, cfg)
	webhookService := services.NewWebhookService(repo, tenantClient, cfg)
	expirationService := services.NewExpirationService(repo, tenantClient)

	stripeSettingsService, err := services.NewStripeSettingsService(cfg)
	if err != nil {
		log.Printf("WARNING: Failed to create Stripe settings service: %v", err)
	}

	planHandler := handlers.NewPlanHandler(planService)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService)
	webhookHandler := handlers.NewWebhookHandler(webhookService, cfg)
	statsHandler := handlers.NewStatsHandler(subscriptionService)

	var stripeSettingsHandler *handlers.StripeSettingsHandler
	if stripeSettingsService != nil {
		stripeSettingsHandler = handlers.NewStripeSettingsHandler(stripeSettingsService)
	}

	// Setup router
	router := setupRouter(planHandler, subscriptionHandler, webhookHandler, statsHandler, stripeSettingsHandler)

	// Start NATS consumer for tenant.created events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.NatsURL != "" {
		consumer, err := natsconsumer.NewTenantEventConsumer(cfg.NatsURL, subscriptionService)
		if err != nil {
			log.Printf("WARNING: Failed to create NATS consumer: %v", err)
		} else {
			if err := consumer.Start(ctx); err != nil {
				log.Printf("WARNING: Failed to start NATS consumer: %v", err)
			} else {
				log.Println("NATS tenant event consumer started")
				defer consumer.Stop()
			}
		}
	}

	// Start background expiration processor
	go expirationService.StartProcessor(ctx, 1*time.Hour)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	log.Printf("Subscription Service starting on port %s (env: %s)", cfg.Port, cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func connectDatabase(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Connected to database")
	return db, nil
}

func setupRouter(planHandler *handlers.PlanHandler, subscriptionHandler *handlers.SubscriptionHandler, webhookHandler *handlers.WebhookHandler, statsHandler *handlers.StatsHandler, stripeSettingsHandler *handlers.StripeSettingsHandler) *gin.Engine {
	router := gin.Default()

	apiLimiter := middleware.NewRateLimiter(100, time.Minute)
	webhookLimiter := middleware.NewRateLimiter(50, time.Minute)

	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.TenantMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "subscription-service",
		})
	})

	// API v1
	v1 := router.Group("/api/v1")
	v1.Use(middleware.RateLimitMiddleware(apiLimiter, "tenant"))
	{
		// Plans
		plans := v1.Group("/plans")
		{
			plans.GET("", planHandler.ListPlans)
			plans.GET("/:id", planHandler.GetPlan)
			plans.POST("", planHandler.CreatePlan)
			plans.PUT("/:id", planHandler.UpdatePlan)
			plans.DELETE("/:id", planHandler.DeletePlan)
			plans.POST("/sync-stripe", planHandler.SyncToStripe)
		}

		// Subscriptions
		subs := v1.Group("/subscriptions")
		{
			subs.GET("/:tenantId", subscriptionHandler.GetSubscription)
			subs.GET("/:tenantId/status", subscriptionHandler.GetSubscriptionStatus)
			subs.POST("/checkout", subscriptionHandler.CreateCheckoutSession)
			subs.POST("/portal", subscriptionHandler.CreatePortalSession)
			subs.POST("/:tenantId/cancel", subscriptionHandler.CancelSubscription)
			subs.POST("/:tenantId/reactivate", subscriptionHandler.ReactivateSubscription)
			subs.POST("/:tenantId/setup-payment", subscriptionHandler.CreateSetupPayment)
			subs.PUT("/:tenantId/plan", subscriptionHandler.ChangePlan)
			subs.GET("/:tenantId/invoices", subscriptionHandler.GetInvoices)
		}

		// Stats
		v1.GET("/stats/overview", statsHandler.GetOverview)

		// Admin APIs (platform admin)
		admin := v1.Group("/admin")
		{
			admin.GET("/stats/enhanced", statsHandler.GetEnhancedStats)
			admin.GET("/stats/expiring-trials", statsHandler.GetExpiringTrials)
			admin.GET("/invoices", statsHandler.GetAdminInvoices)
			admin.POST("/subscriptions/:tenantId/extend-trial", subscriptionHandler.ExtendTrial)

			if stripeSettingsHandler != nil {
				admin.GET("/settings/stripe", stripeSettingsHandler.GetStripeSettings)
				admin.PUT("/settings/stripe", stripeSettingsHandler.UpdateStripeKeys)
				admin.POST("/settings/stripe/verify", stripeSettingsHandler.VerifyStripeKey)
				admin.POST("/settings/stripe/reload", stripeSettingsHandler.ReloadStripeKeys)
			}
		}
	}

	// Webhooks (public, rate limited)
	webhooks := router.Group("/webhooks")
	webhooks.Use(middleware.RateLimitMiddleware(webhookLimiter, "ip"))
	{
		webhooks.POST("/stripe", webhookHandler.HandleStripeWebhook)
	}

	return router
}
