package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/tesseract-hub/feature-flags-service/internal/clients"
	"github.com/tesseract-hub/feature-flags-service/internal/config"
	"github.com/tesseract-hub/feature-flags-service/internal/handlers"
	"github.com/tesseract-hub/feature-flags-service/internal/middleware"

	_ "github.com/tesseract-hub/feature-flags-service/docs"

	gosharedmw "github.com/tesseract-hub/go-shared/middleware"
	"github.com/tesseract-hub/go-shared/tracing"
)

// @title Feature Flags Service API
// @version 1.0.0
// @description Multi-tenant feature flags service powered by Growthbook
// @termsOfService http://swagger.io/terms/

// @contact.name Feature Flags API Support
// @contact.email support@tesseract.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8096
// @BasePath /api/v1

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize Growthbook client
	growthbookClient := clients.NewGrowthbookClient(cfg)

	// Initialize handlers
	featuresHandler := handlers.NewFeaturesHandler(growthbookClient, cfg)
	experimentsHandler := handlers.NewExperimentsHandler(growthbookClient, cfg)

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	var err error
	if cfg.Environment == "production" {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("feature-flags-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("feature-flags-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "feature_flags_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add observability middleware
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("feature-flags-service"))

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.ReadyCheck(growthbookClient))
	router.GET("/metrics", gosharedmw.Handler())

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Public features endpoint (for SDK clients)
	// This proxies to Growthbook with tenant context
	router.GET("/api/features/:clientKey", featuresHandler.GetFeatures)

	// Protected API routes
	api := router.Group("/api/v1")

	// Add auth middleware
	if cfg.Environment == "development" {
		api.Use(middleware.DevelopmentAuthMiddleware())
	} else {
		tenantID := os.Getenv("AZURE_TENANT_ID")
		applicationID := os.Getenv("AZURE_APPLICATION_ID")
		if tenantID != "" && applicationID != "" {
			api.Use(middleware.AzureADAuthMiddleware(tenantID, applicationID))
		} else {
			api.Use(middleware.DevelopmentAuthMiddleware())
		}
	}
	api.Use(gosharedmw.TenantMiddlewareWithOptions(gosharedmw.TenantOptions{
		RequireVendorID: true,
	}))

	// Feature Flags routes
	v1 := api.Group("")
	{
		features := v1.Group("/features")
		{
			// Get all features for tenant
			features.GET("", featuresHandler.ListFeatures)

			// Evaluate a specific feature
			features.POST("/evaluate", featuresHandler.EvaluateFeature)
			features.GET("/evaluate/:key", featuresHandler.EvaluateFeatureByKey)

			// Batch evaluate multiple features
			features.POST("/evaluate/batch", featuresHandler.BatchEvaluate)

			// Feature overrides (for testing)
			features.POST("/override", featuresHandler.SetOverride)
			features.DELETE("/override/:key", featuresHandler.ClearOverride)
		}

		experiments := v1.Group("/experiments")
		{
			// Get experiment results
			experiments.GET("", experimentsHandler.ListExperiments)
			experiments.GET("/:id", experimentsHandler.GetExperiment)
			experiments.POST("/:id/track", experimentsHandler.TrackExperiment)
		}

		// SDK Configuration
		sdk := v1.Group("/sdk")
		{
			sdk.GET("/config", featuresHandler.GetSDKConfig)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8096"
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Feature flags service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down feature-flags-service...")

	// Shutdown tracer provider
	if tracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		} else {
			log.Println("✓ Tracer provider shut down")
		}
	}

	log.Println("Feature flags service stopped")
}
