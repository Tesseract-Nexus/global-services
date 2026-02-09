package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"tickets-service/internal/clients"
	"tickets-service/internal/config"
	"tickets-service/internal/events"
	"tickets-service/internal/handlers"
	"tickets-service/internal/middleware"
	"tickets-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// Global logger
var log *logrus.Logger

// @title Tickets Management API
// @version 2.0.0
// @description Enterprise tickets management service with multi-tenant support and automated escalation
// @termsOfService http://swagger.io/terms/

// @contact.name Tickets API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8085
// @BasePath /api/v1

// @securityDefinitions.bearer BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Initialize structured logger
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Warn("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize database
	db, err := config.InitDB(cfg)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(log)
	if err != nil {
		log.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
	} else {
		defer eventsPublisher.Close()
		log.Info("✓ NATS events publisher initialized")
	}

	// Initialize notification and tenant clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Info("Notification client initialized for direct API calls")

	// Initialize repository
	ticketsRepo := repository.NewTicketsRepository(db)

	// Initialize handlers with events publisher for audit logging
	ticketsHandler := handlers.NewTicketsHandler(ticketsRepo, notificationClient, tenantClient, eventsPublisher)
	documentHandler := handlers.NewDocumentHandler(cfg.DocumentServiceURL, cfg.ProductID)

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Info("RBAC middleware initialized")

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)

	// Protected API routes
	api := router.Group("/api/v1")

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// API routes with RBAC
	v1 := api.Group("")
	{
		tickets := v1.Group("/tickets")
		{
			// Basic CRUD operations
			tickets.POST("", rbacMiddleware.RequirePermission(rbac.PermissionTicketsCreate), ticketsHandler.CreateTicket)
			tickets.GET("", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.GetTickets)
			tickets.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.GetTicket)
			tickets.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.UpdateTicket)
			tickets.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.DeleteTicket)

			// Status and assignment operations
			tickets.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionTicketsResolve), ticketsHandler.UpdateTicketStatus)
			tickets.POST("/:id/assign", rbacMiddleware.RequirePermission(rbac.PermissionTicketsAssign), ticketsHandler.AssignTicket)
			tickets.DELETE("/:id/assign/:assigneeId", rbacMiddleware.RequirePermission(rbac.PermissionTicketsAssign), ticketsHandler.UnassignTicket)

			// Comments and attachments
			tickets.POST("/:id/comments", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.AddComment)
			tickets.PUT("/:id/comments/:commentId", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.UpdateComment)
			tickets.DELETE("/:id/comments/:commentId", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.DeleteComment)

			// Attachments (JSON-based)
			tickets.POST("/:id/attachments", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.AddAttachment)
			tickets.DELETE("/:id/attachments/:attachmentId", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.DeleteAttachment)

			// Document service attachment management
			tickets.POST("/attachments/upload", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), documentHandler.UploadTicketAttachment)
			tickets.GET("/:id/attachments", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), documentHandler.GetTicketAttachments)
			tickets.POST("/:id/attachments/presigned-url", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), documentHandler.GenerateTicketAttachmentPresignedURL)

			// Bulk operations
			tickets.POST("/bulk/status", rbacMiddleware.RequirePermission(rbac.PermissionTicketsResolve), ticketsHandler.BulkUpdateStatus)
			tickets.POST("/bulk/assign", rbacMiddleware.RequirePermission(rbac.PermissionTicketsAssign), ticketsHandler.BulkAssign)
			tickets.POST("/bulk/priority", rbacMiddleware.RequirePermission(rbac.PermissionTicketsUpdate), ticketsHandler.BulkUpdatePriority)

			// Analytics and reporting
			tickets.GET("/analytics", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.GetAnalytics)
			tickets.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.GetStats)
			tickets.POST("/export", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.ExportTickets)

			// Escalation and automation
			tickets.POST("/:id/escalate", rbacMiddleware.RequirePermission(rbac.PermissionTicketsEscalate), ticketsHandler.EscalateTicket)
			tickets.POST("/:id/clone", rbacMiddleware.RequirePermission(rbac.PermissionTicketsCreate), ticketsHandler.CloneTicket)

			// Advanced queries
			tickets.POST("/search", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.SearchTickets)
			tickets.GET("/:id/similar", rbacMiddleware.RequirePermission(rbac.PermissionTicketsRead), ticketsHandler.GetSimilarTickets)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	log.WithField("port", port).Info("Tickets service starting")
	if err := router.Run(":" + port); err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}
}
