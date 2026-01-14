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
	"github.com/google/uuid"
	"notification-hub/internal/config"
	"notification-hub/internal/handlers"
	"notification-hub/internal/middleware"
	"notification-hub/internal/models"
	natsc "notification-hub/internal/nats"
	"notification-hub/internal/repository"
	"notification-hub/internal/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set Gin mode
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to database
	db, err := initDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	// First, try to drop any old constraints/indexes that might conflict
	// This handles cases where the schema changed
	// Only attempt if table exists
	var tableExists bool
	db.Raw("SELECT EXISTS(SELECT FROM information_schema.tables WHERE table_name = 'notification_preferences')").Scan(&tableExists)
	if tableExists {
		// Drop old unique constraint that GORM might try to drop during AutoMigrate
		_ = db.Exec("ALTER TABLE notification_preferences DROP CONSTRAINT IF EXISTS uni_notification_preferences_user_id").Error
		_ = db.Exec("DROP INDEX IF EXISTS uni_notification_preferences_user_id").Error
		_ = db.Exec("DROP INDEX IF EXISTS idx_notification_preferences_user_id").Error
		log.Println("Cleaned up old constraints/indexes")
	}

	// First migrate Notification table
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		log.Fatalf("Failed to auto-migrate Notification: %v", err)
	}

	// Then migrate NotificationPreference with error handling
	if err := db.AutoMigrate(&models.NotificationPreference{}); err != nil {
		// If migration fails due to constraint issue, drop and recreate the table
		log.Printf("Warning: AutoMigrate for NotificationPreference failed: %v", err)
		log.Println("Dropping notification_preferences table to fix schema...")
		if dropErr := db.Exec("DROP TABLE IF EXISTS notification_preferences CASCADE").Error; dropErr != nil {
			log.Fatalf("Failed to drop notification_preferences: %v", dropErr)
		}
		// Retry migration
		if retryErr := db.AutoMigrate(&models.NotificationPreference{}); retryErr != nil {
			log.Fatalf("Failed to auto-migrate NotificationPreference after reset: %v", retryErr)
		}
		log.Println("Successfully recreated notification_preferences table")
	}
	log.Println("Database migration completed")

	// Initialize repositories
	notifRepo := repository.NewNotificationRepository(db)
	prefRepo := repository.NewPreferenceRepository(db)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize SSE hub
	sseHub := handlers.NewSSEHub()

	// Connect to NATS with retry
	var natsClient *natsc.Client
	var natsSubscriber *natsc.Subscriber

	// Initialize health handler early (NATS client may be nil initially)
	healthHandler := handlers.NewHealthHandler(db, nil)

	natsClient, err = natsc.NewClient(&cfg.NATS)
	if err != nil {
		log.Printf("Warning: Failed to connect to NATS: %v", err)
		log.Println("Will retry NATS connection in background...")

		// Start background NATS connection retry
		go func() {
			retryInterval := 10 * time.Second
			maxRetries := 30 // Try for ~5 minutes

			for i := 0; i < maxRetries; i++ {
				time.Sleep(retryInterval)
				log.Printf("Retrying NATS connection (attempt %d/%d)...", i+1, maxRetries)

				client, err := natsc.NewClient(&cfg.NATS)
				if err != nil {
					log.Printf("NATS retry failed: %v", err)
					continue
				}

				natsClient = client
				log.Println("Successfully connected to NATS on retry")

				// Create subscriber
				userResolver := &CombinedUserResolver{
					sseHub: sseHub,
					wsHub:  wsHub,
				}
				natsSubscriber = natsc.NewSubscriber(natsClient, wsHub, notifRepo, userResolver)
				if err := natsSubscriber.Start(context.Background()); err != nil {
					log.Printf("Warning: Failed to start NATS subscriber: %v", err)
				} else {
					log.Println("NATS subscriber started successfully")
				}

				// Update health handler with new NATS client
				healthHandler.SetNATSClient(natsClient)
				break
			}
		}()
	} else {
		// Update health handler with NATS client
		healthHandler.SetNATSClient(natsClient)

		// Create NATS subscriber with SSE broadcast support
		userResolver := &CombinedUserResolver{
			sseHub: sseHub,
			wsHub:  wsHub,
		}
		natsSubscriber = natsc.NewSubscriber(natsClient, wsHub, notifRepo, userResolver)
		if err := natsSubscriber.Start(context.Background()); err != nil {
			log.Printf("Warning: Failed to start NATS subscriber: %v", err)
		}
	}

	// Initialize other handlers
	notifHandler := handlers.NewNotificationHandler(notifRepo, wsHub, sseHub)
	prefHandler := handlers.NewPreferenceHandler(prefRepo)
	wsHandler := handlers.NewWebSocketHandler(wsHub, notifRepo, &cfg.WebSocket)
	sseHandler := handlers.NewSSEHandler(sseHub, notifRepo)
	debugHandler := handlers.NewDebugHandler(notifRepo, cfg.App.Environment)

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.App.Environment == "production" {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("notification-hub"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("notification-hub"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "notification_hub")
	log.Println("✓ Prometheus metrics initialized")

	// Set up router
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("notification-hub"))

	// Health endpoints (no auth required)
	router.GET("/health", healthHandler.Health)
	router.GET("/livez", healthHandler.Livez)
	router.GET("/readyz", healthHandler.Readyz)
	router.GET("/metrics", gosharedmw.Handler())

	// API routes
	api := router.Group("/api/v1")
	{
		// Authenticated routes
		notifications := api.Group("/notifications")
		notifications.Use(middleware.TenantAuth())
		{
			// List and get
			notifications.GET("", notifHandler.List)
			notifications.GET("/:id", notifHandler.Get)

			// Mark read/unread
			notifications.PATCH("/:id/read", notifHandler.MarkRead)
			notifications.PATCH("/:id/unread", notifHandler.MarkUnread)
			notifications.POST("/mark-read", notifHandler.MarkBatchRead)
			notifications.POST("/mark-all-read", notifHandler.MarkAllRead)

			// Unread count
			notifications.GET("/unread-count", notifHandler.GetUnreadCount)

			// Delete
			notifications.DELETE("/:id", notifHandler.Delete)
			notifications.DELETE("", notifHandler.DeleteAll)

			// Preferences
			notifications.GET("/preferences", prefHandler.Get)
			notifications.PUT("/preferences", prefHandler.Update)
			notifications.POST("/preferences/reset", prefHandler.Reset)

			// Real-time streams
			notifications.GET("/ws", wsHandler.Handle)
			notifications.GET("/ws/status", wsHandler.GetStatus)
			notifications.GET("/stream", sseHandler.Stream)

			// Debug endpoints (non-production only)
			notifications.POST("/debug/seed", debugHandler.SeedNotifications)
			notifications.DELETE("/debug/clear", debugHandler.ClearNotifications)
		}
	}

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	log.Printf("Notification Hub started on port %d", cfg.Server.Port)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop NATS subscriber
	if natsSubscriber != nil {
		natsSubscriber.Stop()
	}

	// Close NATS connection
	if natsClient != nil {
		natsClient.Close()
	}

	// Shutdown WebSocket hub
	wsHub.Shutdown()

	// Shutdown tracer provider
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		} else {
			log.Println("✓ Tracer provider shut down")
		}
	}

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Notification hub stopped")
}

func initDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)

	gormLogger := logger.Default.LogMode(logger.Silent)
	if os.Getenv("DB_LOG_LEVEL") == "info" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// CombinedUserResolver resolves users by checking connected WebSocket and SSE clients
type CombinedUserResolver struct {
	sseHub *handlers.SSEHub
	wsHub  *websocket.Hub
}

// GetConnectedUsers returns all connected users for a tenant from both WebSocket and SSE hubs
func (r *CombinedUserResolver) GetConnectedUsers(tenantID string) []uuid.UUID {
	// Get connected users from WebSocket hub
	wsUsers := r.wsHub.GetConnectedUserIDs(tenantID)

	// Get connected users from SSE hub
	sseUsers := r.sseHub.GetConnectedUserIDs(tenantID)

	// Combine and deduplicate users
	userMap := make(map[uuid.UUID]bool)
	for _, u := range wsUsers {
		userMap[u] = true
	}
	for _, u := range sseUsers {
		userMap[u] = true
	}

	// Convert back to slice
	users := make([]uuid.UUID, 0, len(userMap))
	for u := range userMap {
		users = append(users, u)
	}

	return users
}
