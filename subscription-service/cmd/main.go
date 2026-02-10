package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"subscription-service/internal/clients"
	"subscription-service/internal/config"
	"subscription-service/internal/handlers"
	"subscription-service/internal/middleware"
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

	// Initialize layers
	// Note: Schema creation and plan seeding is handled by db-schema-bootstrap CronJob
	repo := repository.NewSubscriptionRepository(db)
	tenantClient := clients.NewTenantClient()

	planService := services.NewPlanService(repo)
	subscriptionService := services.NewSubscriptionService(repo, tenantClient)
	webhookService := services.NewWebhookService(repo, tenantClient)

	planHandler := handlers.NewPlanHandler(planService)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService)
	webhookHandler := handlers.NewWebhookHandler(webhookService)
	statsHandler := handlers.NewStatsHandler(subscriptionService)

	// Setup router
	router := setupRouter(planHandler, subscriptionHandler, webhookHandler, statsHandler)

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

func setupRouter(planHandler *handlers.PlanHandler, subscriptionHandler *handlers.SubscriptionHandler, webhookHandler *handlers.WebhookHandler, statsHandler *handlers.StatsHandler) *gin.Engine {
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
			subs.POST("/checkout", subscriptionHandler.CreateCheckoutSession)
			subs.POST("/portal", subscriptionHandler.CreatePortalSession)
			subs.POST("/:tenantId/cancel", subscriptionHandler.CancelSubscription)
			subs.POST("/:tenantId/reactivate", subscriptionHandler.ReactivateSubscription)
			subs.PUT("/:tenantId/plan", subscriptionHandler.ChangePlan)
			subs.GET("/:tenantId/invoices", subscriptionHandler.GetInvoices)
		}

		// Stats
		v1.GET("/stats/overview", statsHandler.GetOverview)
	}

	// Webhooks (public, rate limited)
	webhooks := router.Group("/webhooks")
	webhooks.Use(middleware.RateLimitMiddleware(webhookLimiter, "ip"))
	{
		webhooks.POST("/stripe", webhookHandler.HandleStripeWebhook)
	}

	return router
}
