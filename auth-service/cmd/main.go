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

	sharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
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

	// Security handlers will be initialized after security middleware is created
	var securityHandlers *handlers.SecurityHandlers

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

	// Initialize security middleware for rate limiting and account lockout
	// Build security config from environment/config
	securityConfig := middleware.SecurityConfig{
		MaxLoginAttempts: cfg.Security.MaxLoginAttempts,
		LockoutTiers: []middleware.LockoutTier{
			{Tier: 1, Duration: cfg.Security.GetTier1Duration()},
			{Tier: 2, Duration: cfg.Security.GetTier2Duration()},
			{Tier: 3, Duration: cfg.Security.GetTier3Duration()},
			{Tier: 4, Duration: 0}, // Permanent lockout (0 = permanent)
		},
		PermanentLockoutThreshold: cfg.Security.PermanentLockoutThreshold,
		LockoutResetAfter:         cfg.Security.GetLockoutResetDuration(),
		RedisKeyPrefix:            "auth:lockout:",
		RedisKeyPrefixEmail:       "auth:lockout:email:",
	}
	securityMiddleware := middleware.NewSecurityMiddlewareWithConfig(redisClient, logger, securityConfig)
	log.Printf("✓ Security middleware initialized (progressive lockout: %d attempts/tier, permanent after %d attempts)",
		cfg.Security.MaxLoginAttempts, cfg.Security.PermanentLockoutThreshold)

	// Initialize security handlers for admin unlock endpoints
	securityHandlers = handlers.NewSecurityHandlers(securityMiddleware, authRepo, eventsPublisher)

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
		// Public authentication routes with rate limiting
		auth := api.Group("/auth")
		{
			// Login endpoint with combined rate limiting:
			// - IP-based rate limiting (go-shared AuthRateLimit)
			// - Account lockout with exponential backoff (IP + email combination)
			auth.POST("/login",
				sharedmw.AuthRateLimit(),                        // IP-based rate limiting
				securityMiddleware.AccountLockoutMiddleware(),   // Account lockout check
				authHandlers.Login,
			)

			// Refresh token with auth rate limiting
			auth.POST("/refresh",
				sharedmw.AuthRateLimit(),
				authHandlers.RefreshToken,
			)

			// Logout endpoint (lighter rate limiting)
			auth.POST("/logout", authHandlers.Logout)

			// Token validation
			auth.POST("/validate", authHandlers.ValidateToken)

			// Registration with auth rate limiting
			auth.POST("/register",
				sharedmw.AuthRateLimit(),
				passwordHandlers.Register,
			)

			// Email verification
			auth.POST("/verify-email", passwordHandlers.VerifyEmail)

			// Password reset with strict rate limiting (prevents abuse)
			auth.POST("/password/forgot",
				sharedmw.PasswordResetRateLimit(), // Very strict: ~3 requests per hour
				passwordHandlers.ForgotPassword,
			)
			auth.POST("/password/reset",
				sharedmw.PasswordResetRateLimit(),
				passwordHandlers.ResetPassword,
			)

			// Resend verification email (rate limited to prevent spam)
			auth.POST("/resend-verification",
				sharedmw.PasswordResetRateLimit(),
				passwordHandlers.ResendVerification,
			)
		}

		// Protected routes
		protected := api.Group("/auth")
		protected.Use(authMiddleware.AuthRequired())
		{
			// User profile
			protected.GET("/profile", authHandlers.GetProfile)

			// Password change (requires authentication + rate limiting)
			protected.POST("/password/change",
				sharedmw.AuthRateLimit(), // Rate limited to prevent brute force
				passwordHandlers.ChangePassword,
			)

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

				// User lockout management (requires security:manage permission)
				users.GET("/:user_id/lockout-status", securityHandlers.GetLockoutStatus)
				users.POST("/:user_id/unlock", securityHandlers.UnlockAccount)
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

			// Security management
			security := admin.Group("/security")
			{
				security.GET("/locked-accounts", securityHandlers.ListLockedAccounts)
				security.POST("/unlock-by-email", securityHandlers.UnlockAccountByEmail)
				security.GET("/config", securityHandlers.GetSecurityConfig)
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
