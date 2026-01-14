package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"auth-service/internal/clients"
	"auth-service/internal/config"
	"auth-service/internal/events"
	"auth-service/internal/handlers"
	"auth-service/internal/middleware"
	"auth-service/internal/migration"
	"auth-service/internal/repository"
	"auth-service/internal/services"
)

func main() {
	// DEPRECATION NOTICE: This service is deprecated and being replaced by Keycloak.
	// New registrations should use Keycloak via tenant-service.
	// Services should configure AUTH_VALIDATOR_TYPE=hybrid to accept both token types.
	// Migration tools available at: /tools/keycloak-migration/
	log.Println("⚠️  WARNING: auth-service is DEPRECATED. Use Keycloak for new user management.")
	log.Println("⚠️  See /tools/keycloak-migration/ for migration instructions.")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := migration.Run(db); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize Redis
	redisClient := initRedis(cfg)
	if redisClient != nil {
		defer redisClient.Close()
	}

	// Initialize logrus logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if cfg.Server.Mode == "release" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Initialize notification clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Println("✓ Notification client initialized")

	// Initialize NATS event publisher for audit logging
	var eventsPublisher *events.Publisher
	natsURL := os.Getenv("NATS_URL")
	if natsURL != "" {
		var err error
		eventsPublisher, err = events.NewPublisher(logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize NATS publisher, continuing without event publishing")
		} else {
			log.Println("✓ Events publisher initialized (NATS connected)")
		}
	} else {
		log.Println("⚠ NATS_URL not set, event publishing disabled")
	}
	defer func() {
		if eventsPublisher != nil {
			eventsPublisher.Close()
		}
	}()

	// Initialize repositories
	authRepo := repository.NewAuthRepository(db)

	// Initialize services
	jwtService := services.NewJWTService(
		cfg.JWT.Secret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiryHours,
		cfg.JWT.RefreshExpiryDays,
	)
	authService := services.NewAuthService(authRepo, jwtService)
	passwordService := services.NewPasswordService()
	emailService := services.NewEmailService() // Placeholder - needs SMTP config

	// Initialize handlers (pass notification client to password handlers for user registration events)
	authHandlers := handlers.NewAuthHandlers(authService, eventsPublisher)
	passwordHandlers := handlers.NewPasswordHandlers(authService, passwordService, emailService, notificationClient, tenantClient, eventsPublisher)
	rbacHandlers := handlers.NewRBACHandlers(db)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

	// Setup Gin router
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(middleware.RequestLoggingMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.TenantMiddlewareWithResolver(tenantClient)) // Supports both UUID and slug in X-Tenant-ID
	router.Use(gin.Recovery())

	// Health check endpoints
	router.GET("/health", authHandlers.Health)
	router.GET("/ready", authHandlers.Ready)

	// API routes
	api := router.Group("/api/v1")
	{
		// Public authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandlers.Login)
			auth.POST("/refresh", authHandlers.RefreshToken)
			auth.POST("/logout", authHandlers.Logout)
			auth.POST("/validate", authHandlers.ValidateToken)

			// Password-based authentication
			auth.POST("/register", passwordHandlers.Register)
			auth.POST("/verify-email", passwordHandlers.VerifyEmail)
		}

		// Protected routes
		protected := api.Group("/auth")
		protected.Use(authMiddleware.AuthRequired())
		{
			// User profile
			protected.GET("/profile", authHandlers.GetProfile)

			// Permission checking
			protected.GET("/permissions/:permission", authHandlers.CheckPermission)
			protected.POST("/permissions/check", authHandlers.CheckPermissions)

			// Available roles and permissions
			protected.GET("/roles/available", authHandlers.GetAvailableRoles)
			protected.GET("/permissions/available", authHandlers.GetAvailablePermissions)
		}

		// Admin routes (require admin role)
		admin := api.Group("/admin")
		admin.Use(authMiddleware.AdminOnly())
		{
			// User management
			users := admin.Group("/users")
			{
				users.GET("/", authHandlers.ListUsers)
				users.GET("/:user_id", authHandlers.GetUser)
				users.GET("/:user_id/roles", rbacHandlers.GetUserRoles)
				users.POST("/:user_id/roles", authHandlers.AssignRole)
				users.DELETE("/:user_id/roles", authHandlers.RemoveRole)
			}

			// Role management
			roles := admin.Group("/roles")
			{
				roles.GET("/", rbacHandlers.ListRoles)
				roles.GET("/:role_id", rbacHandlers.GetRole)
				roles.POST("/", rbacHandlers.CreateRole)
				roles.PUT("/:role_id", rbacHandlers.UpdateRole)
				roles.DELETE("/:role_id", rbacHandlers.DeleteRole)
				roles.GET("/:role_id/permissions", rbacHandlers.GetRolePermissions)
				roles.POST("/:role_id/permissions", rbacHandlers.AssignPermissionToRole)
				roles.DELETE("/:role_id/permissions", rbacHandlers.RemovePermissionFromRole)
			}

			// Permission management
			permissions := admin.Group("/permissions")
			{
				permissions.GET("/", rbacHandlers.ListPermissions)
			}
		}
	}

	// Start server
	serverAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("🚀 Auth service starting on %s", serverAddr)
	log.Printf("📊 Environment: %s", cfg.Server.Mode)
	log.Printf("🗄️  Database: %s@%s:%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port)
	log.Printf("📦 Redis: %s:%s", cfg.Redis.Host, cfg.Redis.Port)

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initDatabase initializes the PostgreSQL database connection
func initDatabase(cfg *config.Config) (*sql.DB, error) {
	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("✅ Database connected successfully")
	return db, nil
}

// initRedis initializes the Redis client
func initRedis(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️  Redis connection failed: %v", err)
		log.Println("🔄 Continuing without Redis (sessions will be stateless)")
		return nil
	}

	log.Println("✅ Redis connected successfully")
	return rdb
}
