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
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"search-service/internal/clients"
	"search-service/internal/config"
	"search-service/internal/handlers"
	"search-service/internal/middleware"
	"search-service/internal/services"

	_ "search-service/docs"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

// @title Search Service API
// @version 1.0.0
// @description Multi-tenant search service powered by Typesense
// @termsOfService http://swagger.io/terms/

// @contact.name Search API Support
// @contact.email support@tesseract.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8095
// @BasePath /api/v1

// @securityDefinitions.bearer BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize Typesense client
	typesenseClient, err := clients.NewTypesenseClient(cfg)
	if err != nil {
		log.Fatal("Failed to initialize Typesense client:", err)
	}

	// Initialize Redis client
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Failed to parse Redis URL:", err)
	}
	// Set Redis password from GCP Secret Manager
	redisOptions.Password = secrets.GetRedisPassword()
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	// Initialize sync service
	syncService := services.NewSyncService(typesenseClient, redisClient, cfg)

	// Initialize handlers
	searchHandler := handlers.NewSearchHandler(typesenseClient)
	indexHandler := handlers.NewIndexHandler(typesenseClient, syncService)
	adminHandler := handlers.NewAdminHandler(typesenseClient)

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.Environment == "production" {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("search-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("search-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "search_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add observability middleware
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("search-service"))

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.ReadyCheck(typesenseClient))
	router.GET("/metrics", gosharedmw.Handler())

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Protected API routes
	api := router.Group("/api/v1")

	// Add auth middleware - use IstioAuth for Keycloak JWT validation
	if cfg.Environment == "development" {
		api.Use(middleware.DevelopmentAuthMiddleware())
	} else {
		// Use Istio headers from Keycloak JWT validation
		api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
			RequireAuth:        true,
			AllowLegacyHeaders: true,
		}))
		api.Use(gosharedmw.VendorScopeFilter())
	}
	api.Use(gosharedmw.TenantMiddlewareWithOptions(gosharedmw.TenantOptions{
		RequireVendorID: true,
	}))

	// Search routes
	v1 := api.Group("")
	{
		search := v1.Group("/search")
		{
			// Global search across all collections
			search.POST("", searchHandler.GlobalSearch)
			search.GET("", searchHandler.GlobalSearchGet)

			// Collection-specific search
			search.POST("/products", searchHandler.SearchProducts)
			search.GET("/products", searchHandler.SearchProductsGet)
			search.POST("/customers", searchHandler.SearchCustomers)
			search.GET("/customers", searchHandler.SearchCustomersGet)
			search.POST("/orders", searchHandler.SearchOrders)
			search.GET("/orders", searchHandler.SearchOrdersGet)
			search.POST("/categories", searchHandler.SearchCategories)
			search.GET("/categories", searchHandler.SearchCategoriesGet)

			// Multi-search (parallel queries)
			search.POST("/multi", searchHandler.MultiSearch)

			// Autocomplete / suggestions
			search.GET("/suggest", searchHandler.Suggest)
		}

		// Indexing routes (for backend services)
		index := v1.Group("/index")
		{
			// Document management
			index.POST("/documents/:collection", indexHandler.IndexDocument)
			index.POST("/documents/:collection/batch", indexHandler.BatchIndex)
			index.PUT("/documents/:collection/:id", indexHandler.UpdateDocument)
			index.DELETE("/documents/:collection/:id", indexHandler.DeleteDocument)
			index.DELETE("/documents/:collection", indexHandler.DeleteByQuery)

			// Sync endpoints (for full reindex)
			index.POST("/sync/products", indexHandler.SyncProducts)
			index.POST("/sync/customers", indexHandler.SyncCustomers)
			index.POST("/sync/orders", indexHandler.SyncOrders)
			index.POST("/sync/categories", indexHandler.SyncCategories)
			index.POST("/sync/all", indexHandler.SyncAll)
		}

		// Admin routes (for ops)
		admin := v1.Group("/admin")
		{
			admin.GET("/collections", adminHandler.ListCollections)
			admin.POST("/collections/:name", adminHandler.CreateCollection)
			admin.DELETE("/collections/:name", adminHandler.DeleteCollection)
			admin.GET("/collections/:name/stats", adminHandler.GetCollectionStats)
			admin.POST("/collections/:name/reindex", adminHandler.ReindexCollection)

			// Cache management
			admin.GET("/cache/stats", searchHandler.GetCacheStats)
			admin.POST("/cache/clear", searchHandler.ClearCache)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Search service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down search-service...")

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

	log.Println("Search service stopped")
}
