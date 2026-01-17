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

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tenant-service/internal/background"
	"tenant-service/internal/clients"
	"tenant-service/internal/config"
	"tenant-service/internal/handlers"
	"tenant-service/internal/middleware"
	"tenant-service/internal/models"
	natsClient "tenant-service/internal/nats"
	"tenant-service/internal/redis"
	"tenant-service/internal/repository"
	"tenant-service/internal/services"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/Tesseract-Nexus/go-shared/metrics"
	sharedMiddleware "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// vendorClientAdapter adapts clients.VendorClient to repository.VendorClientInterface
// This is needed because the interface returns StorefrontInfo, but the client returns *StorefrontData
type vendorClientAdapter struct {
	client *clients.VendorClient
}

func (a *vendorClientAdapter) GetStorefrontBySlug(ctx context.Context, slug string) (repository.StorefrontInfo, error) {
	return a.client.GetStorefrontBySlug(ctx, slug)
}

func main() {
	// Load configuration
	cfg := config.New()

	// Initialize database connection
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate models
	if err := autoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Redis connection
	var redisClient *redis.Client
	redisClient, err = redis.NewClient(cfg.Redis)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
		log.Println("Draft persistence will use PostgreSQL only (no Redis caching)")
	} else {
		log.Println("Connected to Redis successfully")
	}

	// Initialize NATS connection for event publishing
	var nc *natsClient.Client
	nc, err = natsClient.NewClient(nil) // Uses NATS_URL env var or default
	if err != nil {
		log.Printf("Warning: Failed to connect to NATS: %v", err)
		log.Println("Event publishing will be disabled")
	} else {
		log.Println("Connected to NATS successfully")
		defer nc.Close()

		// Subscribe to session.completed events for SSE broadcasting
		// This enables real-time verification status updates to connected clients
		sseHub := handlers.GetSSEHub()
		if err := nc.SubscribeSessionCompleted(func(event *natsClient.SessionCompletedEvent) {
			log.Printf("[SSE] Broadcasting session.completed event for session %s", event.SessionID)
			sseHub.BroadcastSessionCompleted(event.SessionID, event.Email)
		}); err != nil {
			log.Printf("Warning: Failed to subscribe to session.completed events for SSE: %v", err)
		}
	}

	// Initialize metrics
	metricsCollector := initMetrics(db)

	// Initialize repositories
	onboardingRepo := repository.NewOnboardingRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	membershipRepo := repository.NewMembershipRepository(db)

	// Initialize tenant router client for recently deleted slug checking
	tenantRouterServiceURL := getEnv("TENANT_ROUTER_SERVICE_URL", "http://tenant-router-service.devtest.svc.cluster.local:8080")
	tenantRouterClient := clients.NewTenantRouterClient(tenantRouterServiceURL)
	membershipRepo.SetTenantRouterClient(tenantRouterClient)
	log.Printf("Initialized tenant-router-service client: %s", tenantRouterServiceURL)

	// Initialize clients
	verificationServiceURL := getEnv("VERIFICATION_SERVICE_URL", "http://localhost:8088")
	// Load verification API key from GCP Secret Manager (production) or env var (dev)
	verificationServiceAPIKey := secrets.GetSecretOrEnv("VERIFICATION_API_KEY_SECRET_NAME", "VERIFICATION_SERVICE_API_KEY", "tesseract_verification_dev_key_2025")

	verificationClient := clients.NewVerificationClient(
		verificationServiceURL,
		verificationServiceAPIKey,
	)
	// Use verification-service for sending emails (instead of deprecated notification-service)
	notificationClient := clients.NewNotificationClient(verificationServiceURL, verificationServiceAPIKey)

	// Initialize services
	paymentSvc := services.NewPaymentService()
	verificationSvc := services.NewVerificationService(verificationClient, notificationClient, redisClient, cfg.Verification)
	// Wire up NATS client and onboarding repo for event-driven verification emails
	if nc != nil {
		verificationSvc.SetNATSClient(nc)
		log.Println("Verification service: NATS event publishing enabled")
	}
	verificationSvc.SetOnboardingRepo(onboardingRepo)
	templateSvc := services.NewTemplateService(templateRepo)
	notificationSvc := services.NewNotificationService()
	membershipSvc := services.NewMembershipService(membershipRepo)
	onboardingSvc := services.NewOnboardingService(
		onboardingRepo,
		taskRepo,
		verificationSvc,
		paymentSvc,
		membershipSvc,
		nc,
		db,
	)

	// Initialize draft service (with optional Redis)
	var draftSvc *services.DraftService
	if redisClient != nil {
		draftSvc = services.NewDraftService(db, redisClient, cfg.Draft, notificationSvc)
		log.Println("Draft service initialized with Redis caching")
	}

	// Initialize vendor client for tenant creation
	vendorServiceURL := getEnv("VENDOR_SERVICE_URL", "http://localhost:8087")
	vendorClient := clients.NewVendorClient(vendorServiceURL)

	// Wire vendor client to membership repo for storefront slug resolution
	// This allows tenant lookup to fall back to storefront slug when tenant slug not found
	membershipRepo.SetVendorClient(&vendorClientAdapter{vendorClient})
	log.Printf("Membership repository: vendor-service client wired for storefront slug resolution")

	// Initialize tenant service (for quick tenant creation)
	tenantSvc := services.NewTenantService(db, membershipSvc, vendorClient, nc)

	// Initialize Keycloak admin client for offboarding cleanup
	var keycloakClient *auth.KeycloakAdminClient
	keycloakBaseURL := getEnv("KEYCLOAK_BASE_URL", "https://devtest-internal-idp.tesserix.app")
	keycloakRealm := getEnv("KEYCLOAK_REALM", "tesserix-internal")
	keycloakAdminClientID := getEnv("KEYCLOAK_ADMIN_CLIENT_ID", "admin-cli")
	keycloakAdminSecret := secrets.GetSecretOrEnv("KEYCLOAK_ADMIN_CLIENT_SECRET_NAME", "KEYCLOAK_ADMIN_CLIENT_SECRET", "")
	if keycloakAdminSecret != "" {
		keycloakClient = auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
			BaseURL:      keycloakBaseURL,
			Realm:        keycloakRealm,
			ClientID:     keycloakAdminClientID,
			ClientSecret: keycloakAdminSecret,
			Timeout:      30 * time.Second,
		})
		log.Printf("Keycloak admin client initialized for offboarding cleanup (realm: %s)", keycloakRealm)
	} else {
		log.Printf("Warning: KEYCLOAK_ADMIN_CLIENT_SECRET not set - Keycloak cleanup during tenant deletion will be skipped")
	}

	// Initialize offboarding service (for tenant deletion)
	offboardingSvc := services.NewOffboardingService(db, membershipSvc, nc, keycloakClient)

	// Initialize tenant auth service for multi-tenant credential isolation
	// This enables the same email to have different passwords per tenant
	var tenantAuthSvc *services.TenantAuthService
	if keycloakClient != nil {
		// Load the token exchange client secret (separate from admin client)
		// This is the marketplace-dashboard client secret for token exchange
		keycloakClientSecret := secrets.GetSecretOrEnv("KEYCLOAK_CLIENT_SECRET_NAME", "KEYCLOAK_CLIENT_SECRET", "")
		if keycloakClientSecret == "" {
			log.Printf("Warning: KEYCLOAK_CLIENT_SECRET not set - using admin client secret for token exchange")
			keycloakClientSecret = keycloakAdminSecret
		} else {
			log.Printf("✓ Secret %s loaded from GCP Secret Manager for token exchange", getEnv("KEYCLOAK_CLIENT_SECRET_NAME", "KEYCLOAK_CLIENT_SECRET"))
		}

		tenantAuthSvc = services.NewTenantAuthService(db, keycloakClient, &services.KeycloakAuthConfig{
			ClientID:     getEnv("KEYCLOAK_CLIENT_ID", "marketplace-dashboard"),
			ClientSecret: keycloakClientSecret,
		})
		log.Println("TenantAuthService initialized for multi-tenant credential isolation")
	} else {
		// Initialize without Keycloak (credential validation only, no token issuance)
		tenantAuthSvc = services.NewTenantAuthService(db, nil, nil)
		log.Println("TenantAuthService initialized (without Keycloak token issuance)")
	}

	// Initialize staff client for staff tenant lookup during login
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service.marketplace.svc.cluster.local:8082" // Default for k8s
	}
	staffClient := clients.NewStaffClient(staffServiceURL)

	// Wire staff client to auth service for staff credential validation
	tenantAuthSvc.SetStaffClient(staffClient)
	log.Println("Staff client wired to TenantAuthService for staff credential validation")

	// Wire notification client to auth service for sending welcome/verification emails
	tenantAuthSvc.SetNotificationClient(notificationClient)
	log.Println("Notification client wired to TenantAuthService for customer registration emails")

	// Wire verification client to auth service for customer email verification
	tenantAuthSvc.SetVerificationClient(verificationClient)
	log.Println("Verification client wired to TenantAuthService for customer email verification")

	// Initialize customer deactivation service for self-service account deactivation
	var customerDeactivationSvc *services.CustomerDeactivationService
	if keycloakClient != nil {
		customerDeactivationSvc = services.NewCustomerDeactivationService(
			db,
			membershipRepo,
			notificationClient,
			keycloakClient,
			&services.KeycloakAuthConfig{
				ClientID:     getEnv("KEYCLOAK_CLIENT_ID", "marketplace-dashboard"),
				ClientSecret: secrets.GetSecretOrEnv("KEYCLOAK_CLIENT_SECRET_NAME", "KEYCLOAK_CLIENT_SECRET", keycloakAdminSecret),
			},
		)
		log.Println("CustomerDeactivationService initialized for self-service account deactivation")
	} else {
		customerDeactivationSvc = services.NewCustomerDeactivationService(db, membershipRepo, notificationClient, nil, nil)
		log.Println("CustomerDeactivationService initialized (without Keycloak password verification)")
	}

	// Initialize password reset service for self-service password recovery
	var passwordResetSvc *services.PasswordResetService
	if keycloakClient != nil {
		passwordResetSvc = services.NewPasswordResetService(db, keycloakClient, notificationClient)
		log.Println("PasswordResetService initialized for self-service password recovery")
	} else {
		log.Println("Warning: PasswordResetService not initialized (Keycloak client not available)")
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandlerWithNATS(db, nc)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingSvc, templateSvc)
	templateHandler := handlers.NewTemplateHandler(templateSvc)
	verificationHandler := handlers.NewVerificationHandler(verificationSvc, onboardingSvc)
	membershipHandler := handlers.NewMembershipHandlerWithStaff(membershipSvc, staffClient, tenantSvc)
	tenantHandler := handlers.NewTenantHandler(tenantSvc, offboardingSvc)
	authHandler := handlers.NewAuthHandler(tenantAuthSvc, staffClient)
	authHandler.SetDeactivationService(customerDeactivationSvc)
	log.Println("CustomerDeactivationService wired to AuthHandler for account deactivation endpoints")

	// Wire password reset service to auth handler
	if passwordResetSvc != nil {
		authHandler.SetPasswordResetService(passwordResetSvc)
		log.Println("PasswordResetService wired to AuthHandler for password reset endpoints")
	}

	// Initialize draft handler (optional)
	var draftHandler *handlers.DraftHandler
	if draftSvc != nil {
		draftHandler = handlers.NewDraftHandler(draftSvc)
		// Wire draft service to verification handler for cleanup after verification
		verificationHandler.SetDraftService(draftSvc)
	}

	// Start background jobs (if Redis is available)
	var bgRunner *background.Runner
	if draftSvc != nil {
		bgRunner = background.NewRunner(draftSvc, cfg.Draft)
		// Wire deactivation service for account purge background job
		if customerDeactivationSvc != nil {
			bgRunner.SetDeactivationService(customerDeactivationSvc)
			log.Println("CustomerDeactivationService wired to background runner for account purge job")
		}
		bgRunner.Start()
	}

	// Initialize test handler (for E2E testing - only enabled in dev/test)
	var testHandler *handlers.TestHandler
	if handlers.IsTestMode() {
		testHandler = handlers.NewTestHandler(verificationSvc, onboardingSvc, redisClient)
		log.Println("Test handler initialized (dev/test mode) - test endpoints are available")
	}

	// Setup router
	router := setupRouter(
		healthHandler,
		onboardingHandler,
		templateHandler,
		verificationHandler,
		membershipHandler,
		tenantHandler,
		authHandler,
		draftHandler,
		testHandler,
		metricsCollector,
	)

	// Setup server
	port := getEnv("PORT", "8086")
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting tenant-service on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Stop background jobs first
	if bgRunner != nil {
		bgRunner.Stop()
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close Redis connection
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}

	log.Println("Server exited")
}

func setupRouter(
	healthHandler *handlers.HealthHandler,
	onboardingHandler *handlers.OnboardingHandler,
	templateHandler *handlers.TemplateHandler,
	verificationHandler *handlers.VerificationHandler,
	membershipHandler *handlers.MembershipHandler,
	tenantHandler *handlers.TenantHandler,
	authHandler *handlers.AuthHandler,
	draftHandler *handlers.DraftHandler,
	testHandler *handlers.TestHandler,
	metricsCollector *metrics.Metrics,
) *gin.Engine {
	// Set Gin mode
	if getEnv("GIN_MODE", "debug") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:3002",               // Onboarding app (local)
		"http://localhost:4200",               // Admin portal (local)
		"http://localhost:4201",               // Onboarding app alternate (local)
		"https://dev-admin.tesserix.app",      // Admin portal (dev)
		"https://dev-onboarding.tesserix.app", // Onboarding app (dev)
		"https://admin.tesserix.app",          // Admin portal (prod)
		"https://onboarding.tesserix.app",     // Onboarding app (prod)
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-Tenant-ID", "X-User-ID"}
	config.AllowCredentials = true

	// Global middleware
	router.Use(cors.New(config))              // CORS
	router.Use(gin.Recovery())                // Panic recovery
	router.Use(middleware.RequestID())        // Correlation IDs
	router.Use(middleware.StructuredLogger()) // Structured logging
	router.Use(metricsCollector.Middleware()) // Prometheus metrics
	router.Use(middleware.TenantExtraction()) // Tenant context

	// Metrics endpoint (Prometheus scraping)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health endpoints
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Onboarding templates
		templates := v1.Group("/onboarding/templates")
		{
			templates.GET("", templateHandler.ListTemplates)
			templates.POST("", templateHandler.CreateTemplate)
			templates.GET("/:templateId", templateHandler.GetTemplate)
			templates.PUT("/:templateId", templateHandler.UpdateTemplate)
			templates.DELETE("/:templateId", templateHandler.DeleteTemplate)
			templates.POST("/:templateId/set-default", templateHandler.SetDefaultTemplate)
			templates.GET("/by-type/:applicationType", templateHandler.GetTemplatesByApplicationType)
			templates.GET("/default/:applicationType", templateHandler.GetDefaultTemplate)
			templates.GET("/active", templateHandler.GetActiveTemplates)
			templates.POST("/validate-config", templateHandler.ValidateTemplateConfiguration)
		}

		// SSE handler for real-time session events
		sseHandler := handlers.NewSSEHandler()

		// Onboarding sessions
		sessions := v1.Group("/onboarding/sessions")
		{
			sessions.POST("", onboardingHandler.StartOnboarding)
			sessions.GET("/:sessionId", onboardingHandler.GetOnboardingSession)
			sessions.GET("/:sessionId/events", sseHandler.StreamSessionEvents) // SSE endpoint for real-time events
			sessions.POST("/:sessionId/complete", onboardingHandler.CompleteOnboarding)
			sessions.POST("/:sessionId/account-setup", onboardingHandler.CompleteAccountSetup)
			sessions.GET("/:sessionId/progress", onboardingHandler.GetProgress)
			sessions.GET("/:sessionId/tasks", onboardingHandler.GetTasks)
			sessions.PUT("/:sessionId/tasks/:taskId", onboardingHandler.UpdateTaskStatus)

			// Business information
			sessions.POST("/:sessionId/business-information", onboardingHandler.UpdateBusinessInformation)
			sessions.PUT("/:sessionId/business-information", onboardingHandler.UpdateBusinessInformation)

			// Contact information
			sessions.POST("/:sessionId/contact-information", onboardingHandler.UpdateContactInformation)

			// Business addresses
			sessions.POST("/:sessionId/business-addresses", onboardingHandler.UpdateBusinessAddress)

			// Store setup (saves to application_configurations)
			sessions.POST("/:sessionId/store-setup", onboardingHandler.UpdateStoreSetup)
			sessions.PUT("/:sessionId/store-setup", onboardingHandler.UpdateStoreSetup)

			// Verification
			verification := sessions.Group("/:sessionId/verification")
			{
				verification.POST("/email", verificationHandler.StartEmailVerification)
				verification.POST("/phone", verificationHandler.StartPhoneVerification)
				verification.POST("/verify", verificationHandler.VerifyCode)
				verification.POST("/resend", verificationHandler.ResendVerificationCode)
				verification.GET("/status", verificationHandler.GetVerificationStatus)
				verification.GET("/:type/check", verificationHandler.CheckVerification)
			}
		}

		// Validation endpoints
		validation := v1.Group("/validation")
		{
			validation.GET("/subdomain", onboardingHandler.ValidateSubdomain)
			validation.GET("/storefront", onboardingHandler.ValidateStorefront)
			validation.GET("/business-name", onboardingHandler.ValidateBusinessName)
			validation.GET("/slug", membershipHandler.ValidateSlug)
			validation.GET("/slug/generate", membershipHandler.GenerateSlug)
		}

		// Verification endpoints (public - for email verification links)
		verify := v1.Group("/verify")
		{
			verify.GET("/method", verificationHandler.GetVerificationMethod)
			verify.POST("/token", verificationHandler.VerifyByToken)
			verify.GET("/token-info", verificationHandler.GetTokenInfo)
			verify.POST("/token-info", verificationHandler.GetTokenInfoPost)
		}

		// Public invitation acceptance (no auth)
		publicInvitations := v1.Group("/invitations")
		{
			publicInvitations.POST("/accept-public", authHandler.AcceptInvitationPublic)
		}

		// Initialize Istio auth middleware for protected routes
		// During migration, AllowLegacyHeaders=true allows X-User-ID headers as fallback
		logger := logrus.New()
		logger.SetFormatter(&logrus.JSONFormatter{})
		istioAuth := sharedMiddleware.IstioAuth(sharedMiddleware.IstioAuthConfig{
			RequireAuth:        true,
			AllowLegacyHeaders: true, // Allow legacy headers during migration
			Logger:             logger.WithField("component", "istio_auth"),
		})

		// User tenant management endpoints (requires auth)
		users := v1.Group("/users")
		users.Use(istioAuth) // Requires Istio JWT auth
		{
			users.GET("/me/tenants", membershipHandler.GetUserTenants)
			users.GET("/me/tenants/default", membershipHandler.GetUserDefaultTenant)
			users.PUT("/me/tenants/default", membershipHandler.SetUserDefaultTenant)
		}

		// Tenant management endpoints (requires auth)
		tenants := v1.Group("/tenants")
		tenants.Use(istioAuth) // Requires Istio JWT auth
		{
			// Quick tenant creation for existing users
			tenants.POST("/create-for-user", tenantHandler.CreateTenantForUser)
			tenants.GET("/check-slug", tenantHandler.CheckSlugAvailability)

			// Tenant context/access (uses slug or UUID as identifier)
			tenants.GET("/:id/context", membershipHandler.GetTenantContext)
			tenants.GET("/:id/access", membershipHandler.VerifyTenantAccess)

			// Tenant onboarding data (for settings auto-population)
			tenants.GET("/:id/onboarding-data", tenantHandler.GetTenantOnboardingData)

			// Member management (uses tenant ID)
			tenants.POST("/:id/members/invite", membershipHandler.InviteMember)
			tenants.DELETE("/:id/members/:memberId", membershipHandler.RemoveMember)
			tenants.PUT("/:id/members/:memberId/role", membershipHandler.UpdateMemberRole)

			// Tenant deletion (offboarding) - owner only
			tenants.GET("/:id/deletion", tenantHandler.GetTenantDeletionInfo)
			tenants.DELETE("/:id", tenantHandler.DeleteTenant)
		}

		// Invitation endpoints (requires auth)
		invitations := v1.Group("/invitations")
		invitations.Use(istioAuth) // Requires Istio JWT auth
		{
			invitations.POST("/accept", membershipHandler.AcceptInvitation)
		}

		// Multi-tenant authentication endpoints
		// These enable tenant-specific credential validation (same email, different passwords per tenant)
		authRoutes := v1.Group("/auth")
		{
			// Public endpoints (no auth required)
			authRoutes.POST("/validate", authHandler.ValidateCredentials)          // Validate tenant-specific credentials
			authRoutes.POST("/tenants", authHandler.GetUserTenantsForAuth)         // Get user's tenants for login selection
			authRoutes.POST("/account-status", authHandler.CheckAccountStatus)     // Check if account is locked
			authRoutes.POST("/register", authHandler.RegisterCustomer)             // Direct customer registration (storefront)
			authRoutes.POST("/check-deactivated", authHandler.CheckDeactivatedAccount) // Check if account is deactivated
			authRoutes.POST("/reactivate-account", authHandler.ReactivateAccount)  // Reactivate within 90-day window

			// Password reset endpoints (public - no auth required)
			authRoutes.POST("/request-password-reset", authHandler.RequestPasswordReset) // Request password reset email
			authRoutes.POST("/validate-reset-token", authHandler.ValidateResetToken)     // Validate reset token
			authRoutes.POST("/reset-password", authHandler.ResetPassword)                // Reset password with token
		}

		// Protected auth endpoints (require Istio JWT auth)
		// SECURITY: These endpoints modify user credentials and must be authenticated
		protectedAuth := v1.Group("/auth")
		protectedAuth.Use(istioAuth)
		{
			protectedAuth.POST("/change-password", authHandler.ChangePassword)        // Change password for a tenant
			protectedAuth.POST("/set-password", authHandler.SetPassword)              // Set password (after verification)
			protectedAuth.POST("/unlock-account", authHandler.UnlockAccount)          // Admin: unlock locked account
			protectedAuth.POST("/deactivate-account", authHandler.DeactivateAccount)  // Customer self-service deactivation
		}

		// Internal service-to-service endpoints (requires X-Internal-Service header)
		internal := router.Group("/internal")
		{
			internal.GET("/tenants/:id", tenantHandler.GetTenantInfo)
			internal.GET("/tenants/by-slug/:slug", tenantHandler.GetTenantBySlug)
		}

		// Draft persistence endpoints (optional - only if draftHandler is available)
		if draftHandler != nil {
			draft := v1.Group("/onboarding/draft")
			{
				draft.POST("/save", draftHandler.SaveDraft)
				draft.GET("/:sessionId", draftHandler.GetDraft)
				draft.DELETE("/:sessionId", draftHandler.DeleteDraft)
				draft.POST("/heartbeat", draftHandler.ProcessHeartbeat)
				draft.POST("/browser-close", draftHandler.MarkBrowserClosed)
			}
		}

		// Test endpoints (only available in dev/test mode for E2E testing)
		if testHandler != nil {
			test := v1.Group("/test")
			{
				test.POST("/verify-email", testHandler.VerifyEmailForTest)
				test.GET("/verification-token", testHandler.GetVerificationTokenForTest)
				test.GET("/otp", testHandler.GetOTPForTest)
			}
		}
	}

	return router
}

func autoMigrate(db *gorm.DB) error {
	log.Println("Starting database migration...")

	// Enable UUID extension in PostgreSQL
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		log.Printf("Warning: Failed to create uuid-ossp extension: %v", err)
	}

	// Pre-migration: Handle slug column for existing tenants
	// This must run BEFORE AutoMigrate to avoid NOT NULL constraint violation
	if err := preMigrateTenantSlugs(db); err != nil {
		log.Printf("Warning: Pre-migration for tenant slugs: %v", err)
	}

	// Pre-migration: Handle User model schema changes (remove tenant_id and role columns)
	// Users are now global (GitHub-style multi-tenant), relationships via UserTenantMembership
	if err := preMigrateUserModel(db); err != nil {
		log.Printf("Warning: Pre-migration for User model: %v", err)
	}

	// Auto-migrate all models
	modelsToMigrate := []interface{}{
		&models.Tenant{},
		&models.User{},
		&models.UserTenantMembership{},  // Multi-tenant membership support
		&models.TenantActivityLog{},     // Audit trail for tenant activities
		&models.ReservedSlug{},          // Reserved slugs for validation (cached in memory)
		&models.TenantSlugReservation{}, // Tracks claimed slugs during onboarding
		&models.DeletedTenant{},         // Audit table for deleted tenants
		&models.OnboardingTemplate{},
		&models.OnboardingSession{},
		&models.BusinessInformation{},
		&models.ContactInformation{},
		&models.BusinessAddress{},
		&models.VerificationRecord{},
		&models.PaymentInformation{},
		&models.ApplicationConfiguration{},
		&models.OnboardingTask{},
		&models.TaskExecutionLog{},
		&models.DomainReservation{},
		&models.OnboardingNotification{},
		&models.WebhookEvent{},
		// Multi-tenant credential isolation models
		&models.TenantCredential{},   // Per-tenant passwords for enterprise credential isolation
		&models.TenantAuthPolicy{},   // Per-tenant authentication policies
		&models.TenantAuthAuditLog{}, // Authentication audit trail per tenant
		// Customer account deactivation
		&models.DeactivatedMembership{}, // Archive of deactivated customer accounts
		// Password reset tokens
		&models.PasswordResetToken{}, // Secure tokens for password reset flow
	}

	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	log.Println("Database migration completed successfully")

	// Seed default templates if they don't exist
	if err := seedDefaultTemplates(db); err != nil {
		log.Printf("Warning: Failed to seed default templates: %v", err)
	}

	// Seed reserved slugs for URL protection
	if err := seedReservedSlugs(db); err != nil {
		log.Printf("Warning: Failed to seed reserved slugs: %v", err)
	}

	return nil
}

// seedDefaultTemplates creates default onboarding templates if they don't exist
func seedDefaultTemplates(db *gorm.DB) error {
	// Check if ACTIVE ecommerce template already exists
	// IMPORTANT: Must check is_active = true to match GetDefaultTemplate query
	var activeCount int64
	if err := db.Model(&models.OnboardingTemplate{}).Where("application_type = ? AND is_default = ? AND is_active = ?", "ecommerce", true, true).Count(&activeCount).Error; err != nil {
		return fmt.Errorf("failed to check existing templates: %w", err)
	}

	if activeCount > 0 {
		log.Println("Default active templates already exist, skipping seed")
		return nil
	}

	// Check if inactive template exists and reactivate it
	var inactiveCount int64
	if err := db.Model(&models.OnboardingTemplate{}).Where("application_type = ? AND is_default = ? AND is_active = ?", "ecommerce", true, false).Count(&inactiveCount).Error; err != nil {
		log.Printf("Warning: Failed to check inactive templates: %v", err)
	}

	if inactiveCount > 0 {
		log.Println("Found inactive default template, reactivating...")
		if err := db.Model(&models.OnboardingTemplate{}).
			Where("application_type = ? AND is_default = ?", "ecommerce", true).
			Update("is_active", true).Error; err != nil {
			return fmt.Errorf("failed to reactivate template: %w", err)
		}
		log.Println("Default ecommerce template reactivated successfully")
		return nil
	}

	log.Println("Seeding default onboarding templates...")

	// E-commerce template steps
	ecommerceSteps := []map[string]interface{}{
		{
			"id":          "business-registration",
			"name":        "Business Registration",
			"description": "Register your business details",
			"order":       1,
			"required":    true,
			"type":        "form",
		},
		{
			"id":          "contact-details",
			"name":        "Contact Details",
			"description": "Provide contact information",
			"order":       2,
			"required":    true,
			"type":        "form",
		},
		{
			"id":          "business-address",
			"name":        "Business Address",
			"description": "Enter your business address",
			"order":       3,
			"required":    true,
			"type":        "form",
		},
		{
			"id":          "store-setup",
			"name":        "Store Setup",
			"description": "Configure your store settings",
			"order":       4,
			"required":    true,
			"type":        "form",
		},
	}

	stepsJSON, _ := models.NewJSONB(ecommerceSteps)
	configJSON, _ := models.NewJSONB(map[string]interface{}{
		"requires_payment":  true,
		"requires_domain":   true,
		"trial_period_days": 14,
	})
	metadataJSON, _ := models.NewJSONB(map[string]interface{}{})

	ecommerceTemplate := &models.OnboardingTemplate{
		Name:            "E-commerce Store Setup",
		Description:     "Complete setup flow for online stores and e-commerce platforms",
		ApplicationType: "ecommerce",
		Version:         1,
		IsActive:        true,
		IsDefault:       true,
		TemplateConfig:  configJSON,
		Steps:           stepsJSON,
		Metadata:        metadataJSON,
	}

	if err := db.Create(ecommerceTemplate).Error; err != nil {
		return fmt.Errorf("failed to create ecommerce template: %w", err)
	}

	log.Printf("Created default ecommerce template with ID: %s", ecommerceTemplate.ID)
	return nil
}

// seedReservedSlugs creates default reserved slugs for URL protection
func seedReservedSlugs(db *gorm.DB) error {
	// Check if reserved slugs already exist
	var count int64
	if err := db.Model(&models.ReservedSlug{}).Where("is_active = ?", true).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check existing reserved slugs: %w", err)
	}

	if count > 0 {
		log.Printf("Reserved slugs already exist (%d active), skipping seed", count)
		return nil
	}

	log.Println("Seeding reserved slugs for URL protection...")

	// Define reserved slugs by category
	reservedSlugs := []models.ReservedSlug{
		// System slugs - critical paths
		{Slug: "admin", Reason: "System administration", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "api", Reason: "API endpoints", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "app", Reason: "Application path", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "auth", Reason: "Authentication", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "login", Reason: "Login page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "logout", Reason: "Logout page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "register", Reason: "Registration page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "signup", Reason: "Signup page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "signin", Reason: "Signin page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "dashboard", Reason: "Dashboard path", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "settings", Reason: "Settings path", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "account", Reason: "Account path", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "profile", Reason: "Profile path", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "help", Reason: "Help page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "support", Reason: "Support page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "status", Reason: "Status page", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "health", Reason: "Health check", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "metrics", Reason: "Metrics endpoint", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "docs", Reason: "Documentation", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "api-docs", Reason: "API documentation", Category: "system", IsActive: true, CreatedBy: "system"},
		// Infrastructure slugs
		{Slug: "www", Reason: "Web subdomain", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "cdn", Reason: "CDN subdomain", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "mail", Reason: "Mail subdomain", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "smtp", Reason: "SMTP subdomain", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "ftp", Reason: "FTP subdomain", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "static", Reason: "Static assets", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "assets", Reason: "Assets path", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "images", Reason: "Images path", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "files", Reason: "Files path", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "uploads", Reason: "Uploads path", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		{Slug: "media", Reason: "Media path", Category: "infrastructure", IsActive: true, CreatedBy: "system"},
		// Brand slugs
		{Slug: "tesseract", Reason: "Brand protection", Category: "brand", IsActive: true, CreatedBy: "system"},
		{Slug: "tesserix", Reason: "Brand protection", Category: "brand", IsActive: true, CreatedBy: "system"},
		{Slug: "tesseract-hub", Reason: "Brand protection", Category: "brand", IsActive: true, CreatedBy: "system"},
		{Slug: "marketplace", Reason: "Platform name", Category: "brand", IsActive: true, CreatedBy: "system"},
		// Common reserved terms
		{Slug: "test", Reason: "Reserved for testing", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "demo", Reason: "Reserved for demos", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "staging", Reason: "Reserved for staging", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "dev", Reason: "Reserved for development", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "prod", Reason: "Reserved for production", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "production", Reason: "Reserved for production", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "internal", Reason: "Internal use", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "private", Reason: "Private use", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "public", Reason: "Public use", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "root", Reason: "Root access", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "system", Reason: "System use", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "null", Reason: "Reserved keyword", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "undefined", Reason: "Reserved keyword", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "true", Reason: "Reserved keyword", Category: "system", IsActive: true, CreatedBy: "system"},
		{Slug: "false", Reason: "Reserved keyword", Category: "system", IsActive: true, CreatedBy: "system"},
	}

	// Insert reserved slugs, ignoring duplicates
	var inserted int
	for _, slug := range reservedSlugs {
		result := db.Where("slug = ?", slug.Slug).FirstOrCreate(&slug)
		if result.Error != nil {
			log.Printf("Warning: Failed to create reserved slug '%s': %v", slug.Slug, result.Error)
			continue
		}
		if result.RowsAffected > 0 {
			inserted++
		}
	}

	log.Printf("Seeded %d reserved slugs successfully", inserted)
	return nil
}

func initDatabase() (*gorm.DB, error) {
	// Get database configuration from environment
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := secrets.GetDBPassword() // Use GCP Secret Manager if enabled
	dbname := getEnv("DB_NAME", "tesseract_hub")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func initMetrics(db *gorm.DB) *metrics.Metrics {
	// Initialize metrics with service configuration
	m := metrics.New(metrics.Config{
		ServiceName: "tenant-service",
		Namespace:   "tesseract",
		Subsystem:   "tenant",
	})

	// Register business-specific metrics

	// Onboarding session metrics
	onboardingSessionsTotal := m.RegisterCounter(
		"tesseract_tenant_onboarding_sessions_total",
		"Total number of onboarding sessions created",
		[]string{"application_type", "status"},
	)

	verificationAttemptsTotal := m.RegisterCounter(
		"tesseract_tenant_verification_attempts_total",
		"Total number of verification attempts",
		[]string{"type", "status"},
	)

	verificationCodesGenerated := m.RegisterCounter(
		"tesseract_tenant_verification_codes_generated_total",
		"Total number of verification codes generated",
		[]string{"type"},
	)

	// Active sessions gauge
	activeSessions := m.RegisterGauge(
		"tesseract_tenant_active_sessions",
		"Number of currently active onboarding sessions",
	)

	// Database connection pool metrics
	dbConnectionsOpen := m.RegisterGauge(
		"tesseract_tenant_db_connections_open",
		"Number of open database connections",
	)

	dbConnectionsIdle := m.RegisterGauge(
		"tesseract_tenant_db_connections_idle",
		"Number of idle database connections",
	)

	dbConnectionsInUse := m.RegisterGauge(
		"tesseract_tenant_db_connections_in_use",
		"Number of database connections currently in use",
	)

	// Update database metrics periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			sqlDB, err := db.DB()
			if err != nil {
				log.Printf("Failed to get database instance: %v", err)
				continue
			}

			stats := sqlDB.Stats()
			dbConnectionsOpen.Set(float64(stats.OpenConnections))
			dbConnectionsIdle.Set(float64(stats.Idle))
			dbConnectionsInUse.Set(float64(stats.InUse))

			// Count active sessions
			var count int64
			db.Model(&models.OnboardingSession{}).
				Where("status IN ?", []string{"pending", "in_progress"}).
				Count(&count)
			activeSessions.Set(float64(count))
		}
	}()

	// Cleanup expired slug reservations periodically (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// Run initial cleanup on startup
		cleanupExpiredSlugReservations(db)

		for range ticker.C {
			cleanupExpiredSlugReservations(db)
		}
	}()

	// Track verification metrics
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Namespace: "tesseract",
			Subsystem: "tenant",
			Name:      "verification_rate_limits_hit_total",
			Help:      "Total number of times rate limits were hit during verification",
		},
		func() float64 {
			// This would ideally be tracked from actual rate limit hits
			// For now, return 0 as placeholder
			return 0
		},
	))

	log.Println("Metrics initialized successfully")

	// Log registered metrics for debugging
	log.Printf("Registered business metrics:")
	log.Printf("  - onboarding_sessions_total")
	log.Printf("  - verification_attempts_total")
	log.Printf("  - verification_codes_generated_total")
	log.Printf("  - active_sessions")
	log.Printf("  - db_connection metrics")

	// Store metrics in closure variables to avoid unused variable warnings
	_ = onboardingSessionsTotal
	_ = verificationAttemptsTotal
	_ = verificationCodesGenerated

	return m
}

// cleanupExpiredSlugReservations removes expired pending slug reservations
func cleanupExpiredSlugReservations(db *gorm.DB) {
	ctx := context.Background()
	membershipRepo := repository.NewMembershipRepository(db)

	count, err := membershipRepo.CleanupExpiredReservations(ctx)
	if err != nil {
		log.Printf("Failed to cleanup expired slug reservations: %v", err)
		return
	}

	if count > 0 {
		log.Printf("Cleaned up %d expired slug reservations", count)
	}
}

// preMigrateTenantSlugs handles the slug column migration for existing tenants
// This runs BEFORE AutoMigrate to ensure existing tenants have slugs
// before the NOT NULL constraint is applied
func preMigrateTenantSlugs(db *gorm.DB) error {
	// Check if tenants table exists
	if !db.Migrator().HasTable("tenants") {
		log.Println("Tenants table does not exist, skipping pre-migration")
		return nil
	}

	// Check if slug column exists
	if !db.Migrator().HasColumn(&models.Tenant{}, "slug") {
		// Add slug column as nullable first
		log.Println("Adding slug column to tenants table...")
		if err := db.Exec("ALTER TABLE tenants ADD COLUMN IF NOT EXISTS slug VARCHAR(50)").Error; err != nil {
			return fmt.Errorf("failed to add slug column: %w", err)
		}
	}

	// Count tenants with NULL or empty slugs
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM tenants WHERE slug IS NULL OR slug = ''").Scan(&count).Error; err != nil {
		return fmt.Errorf("failed to count tenants without slugs: %w", err)
	}

	if count == 0 {
		log.Println("All tenants have slugs, skipping slug generation")
		return nil
	}

	log.Printf("Found %d tenants without slugs, generating...", count)

	// Generate slugs for tenants that don't have one
	// Uses the tenant name or subdomain to generate a URL-friendly slug
	updateSQL := `
		UPDATE tenants
		SET slug = LOWER(
			REGEXP_REPLACE(
				REGEXP_REPLACE(
					COALESCE(NULLIF(name, ''), subdomain, 'tenant-' || id::text),
					'[^a-zA-Z0-9]+', '-', 'g'
				),
				'^-+|-+$', '', 'g'
			)
		)
		WHERE slug IS NULL OR slug = ''
	`

	if err := db.Exec(updateSQL).Error; err != nil {
		return fmt.Errorf("failed to generate slugs for existing tenants: %w", err)
	}

	// Handle duplicate slugs by appending a number
	duplicateFixSQL := `
		WITH duplicates AS (
			SELECT id, slug, ROW_NUMBER() OVER (PARTITION BY slug ORDER BY created_at) as rn
			FROM tenants
		)
		UPDATE tenants t
		SET slug = t.slug || '-' || d.rn
		FROM duplicates d
		WHERE t.id = d.id AND d.rn > 1
	`

	if err := db.Exec(duplicateFixSQL).Error; err != nil {
		log.Printf("Warning: Failed to fix duplicate slugs: %v", err)
		// Don't fail the migration for this
	}

	// Create unique index if it doesn't exist
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug) WHERE slug IS NOT NULL").Error; err != nil {
		log.Printf("Warning: Failed to create slug index: %v", err)
	}

	log.Println("Tenant slug pre-migration completed")
	return nil
}

// preMigrateUserModel handles schema changes for tenant_users table
// This must run BEFORE AutoMigrate to handle existing tenant_id and role columns
// Since User model is now global (GitHub-style multi-tenant), these columns are no longer needed
func preMigrateUserModel(db *gorm.DB) error {
	// Check if tenant_users table exists
	if !db.Migrator().HasTable("tenant_users") {
		log.Println("tenant_users table does not exist, skipping User model pre-migration")
		return nil
	}

	// Check if tenant_id column exists and drop it (no longer needed for global users)
	if db.Migrator().HasColumn(&models.User{}, "tenant_id") {
		log.Println("Dropping tenant_id column from tenant_users (users are now global)...")

		// First, ensure all existing users have memberships created for their tenants
		// This preserves the user-tenant relationship via UserTenantMembership
		migrateLegacyMembershipsSQL := `
			INSERT INTO user_tenant_memberships (id, tenant_id, user_id, role, status, created_at, updated_at)
			SELECT
				uuid_generate_v4(),
				tenant_id,
				id,
				COALESCE(role, 'owner'),
				'active',
				NOW(),
				NOW()
			FROM tenant_users
			WHERE tenant_id IS NOT NULL
			ON CONFLICT DO NOTHING
		`
		if err := db.Exec(migrateLegacyMembershipsSQL).Error; err != nil {
			log.Printf("Warning: Failed to migrate legacy user-tenant relationships: %v", err)
			// Continue anyway - the table might not have data or membership might already exist
		} else {
			log.Println("Migrated existing user-tenant relationships to memberships")
		}

		// Now drop the tenant_id column
		if err := db.Exec("ALTER TABLE tenant_users DROP COLUMN IF EXISTS tenant_id").Error; err != nil {
			log.Printf("Warning: Failed to drop tenant_id column: %v", err)
		} else {
			log.Println("Dropped tenant_id column from tenant_users")
		}
	}

	// Check if role column exists and drop it (role is now in UserTenantMembership)
	if db.Migrator().HasColumn(&models.User{}, "role") {
		log.Println("Dropping role column from tenant_users (role is now in memberships)...")
		if err := db.Exec("ALTER TABLE tenant_users DROP COLUMN IF EXISTS role").Error; err != nil {
			log.Printf("Warning: Failed to drop role column: %v", err)
		} else {
			log.Println("Dropped role column from tenant_users")
		}
	}

	// Drop the old unique constraint on tenant_id+email if it exists
	// (replaced by simple email uniqueness since users are global)
	dropConstraintSQL := `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.table_constraints
				WHERE constraint_name = 'uni_tenant_users_tenant_id_email'
				AND table_name = 'tenant_users'
			) THEN
				ALTER TABLE tenant_users DROP CONSTRAINT uni_tenant_users_tenant_id_email;
			END IF;
		END $$;
	`
	if err := db.Exec(dropConstraintSQL).Error; err != nil {
		log.Printf("Warning: Failed to drop old tenant_id+email constraint: %v", err)
	}

	log.Println("User model pre-migration completed")
	return nil
}
