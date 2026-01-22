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

	"tenant-router-service/internal/config"
	"tenant-router-service/internal/database"
	"tenant-router-service/internal/handlers"
	"tenant-router-service/internal/k8s"
	"tenant-router-service/internal/models"
	natsClient "tenant-router-service/internal/nats"
	redisClient "tenant-router-service/internal/redis"
	"tenant-router-service/internal/reconciler"
	"tenant-router-service/internal/repository"
	"tenant-router-service/internal/services"
)

func main() {
	log.Println("Starting tenant-router-service (internal mode)...")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database connection
	db, err := database.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run database migrations
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize repository
	tenantHostRepo := repository.NewTenantHostRepository(db)

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Initialize Redis client for caching platform settings (e.g., gateway IP)
	var redis *redisClient.Client
	redis, err = redisClient.NewClient(cfg.Redis)
	if err != nil {
		log.Printf("Warning: Failed to initialize Redis: %v (gateway IP caching disabled)", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	// Initialize router service for VS sync operations
	routerService := services.NewRouterService(k8sClient, tenantHostRepo, cfg)

	// Initialize reconciler (Kubebuilder pattern)
	tenantReconciler := reconciler.NewTenantReconciler(k8sClient, tenantHostRepo, cfg)

	// Start reconciler workers (number of workers can be configured)
	workerCount := 3
	tenantReconciler.Start(workerCount)

	// Initialize NATS subscriber with reconciler
	// NATS is optional - service can run without it (manual tenant provisioning still works)
	var natsSubscriber *natsClient.Subscriber
	natsSubscriber, err = natsClient.NewSubscriber(cfg, tenantReconciler)
	if err != nil {
		log.Printf("Warning: Failed to initialize NATS subscriber: %v (service will run without event-driven provisioning)", err)
	}

	// Start NATS subscriptions (if subscriber was created)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if natsSubscriber != nil {
		if err := natsSubscriber.Start(ctx); err != nil {
			log.Printf("Warning: Failed to start NATS subscriptions: %v (service will run without event-driven provisioning)", err)
			natsSubscriber = nil // Set to nil so health check knows NATS is not active
		}
	}

	// Initialize health handler
	healthHandler := handlers.NewHealthHandler(k8sClient, natsSubscriber, db)

	// Setup Gin router (minimal - internal service)
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check endpoints (required for K8s probes)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// Metrics endpoint for observability
	router.GET("/metrics", func(c *gin.Context) {
		metrics := tenantReconciler.GetMetrics()
		c.JSON(http.StatusOK, gin.H{
			"reconciler": gin.H{
				"total":            metrics.ReconcileTotal,
				"successful":       metrics.ReconcileSuccessful,
				"failed":           metrics.ReconcileFailed,
				"retry_count":      metrics.RetryCount,
				"queue_depth":      metrics.CurrentQueueDepth,
				"last_reconcile":   metrics.LastReconcileTime,
				"last_duration_ms": metrics.ReconcileDuration.Milliseconds(),
			},
			"workers": workerCount,
		})
	})

	// Internal debug endpoint (optional)
	router.GET("/debug/status", func(c *gin.Context) {
		metrics := tenantReconciler.GetMetrics()
		c.JSON(http.StatusOK, gin.H{
			"service":        "tenant-router-service",
			"mode":           "internal",
			"nats_connected": natsSubscriber.IsConnected(),
			"reconciler":     metrics,
		})
	})

	// Start cleanup job (runs every 6 hours)
	cleanupInterval := 6 * time.Hour
	cleanupRetention := 15 * 24 * time.Hour // 15 days
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		// Run once at startup after a short delay
		time.Sleep(5 * time.Minute)
		runCleanup(ctx, tenantHostRepo, cleanupRetention)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup(ctx, tenantHostRepo, cleanupRetention)
			}
		}
	}()

	// Start gateway IP sync job (syncs custom domain gateway IP to Redis)
	// This enables other services (like tenant-service) to fetch the IP without K8s API access
	if redis != nil {
		go func() {
			syncInterval := 5 * time.Minute
			ticker := time.NewTicker(syncInterval)
			defer ticker.Stop()

			// Sync immediately at startup
			syncGatewayIP(ctx, k8sClient, redis)

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					syncGatewayIP(ctx, k8sClient, redis)
				}
			}
		}()
	}

	// API endpoints for tenant host management
	api := router.Group("/api/v1")
	{
		// Register a new tenant host (for manual provisioning or NATS event replay)
		// POST /api/v1/hosts
		// Body: {"slug": "...", "tenant_id": "...", "admin_host": "...", "storefront_host": "...", ...}
		api.POST("/hosts", func(c *gin.Context) {
			var req struct {
				Slug              string `json:"slug" binding:"required"`
				TenantID          string `json:"tenant_id" binding:"required"`
				AdminHost         string `json:"admin_host"`
				StorefrontHost    string `json:"storefront_host"`
				StorefrontWwwHost string `json:"storefront_www_host"` // e.g., "www.customdomain.com" (only for custom domains)
				APIHost           string `json:"api_host"`            // e.g., "api.customdomain.com" or "slug-api.tesserix.app"
				BaseDomain        string `json:"base_domain"`         // e.g., "tesserix.app"
				IsCustomDomain    bool   `json:"is_custom_domain"`    // true if using custom domain
				Product           string `json:"product"`
				BusinessName      string `json:"business_name"`
				Email             string `json:"email"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug and tenant_id are required"})
				return
			}

			// Use config domain if hosts not provided
			domain := cfg.Domain.BaseDomain
			if req.AdminHost == "" {
				req.AdminHost = fmt.Sprintf("%s-admin.%s", req.Slug, domain)
			}
			if req.StorefrontHost == "" {
				req.StorefrontHost = fmt.Sprintf("%s.%s", req.Slug, domain)
			}
			if req.APIHost == "" {
				req.APIHost = fmt.Sprintf("%s-api.%s", req.Slug, domain)
			}
			if req.BaseDomain == "" {
				req.BaseDomain = domain
			}

			// Create the tenant host event and enqueue for reconciliation
			event := &models.TenantCreatedEvent{
				TenantID:          req.TenantID,
				Slug:              req.Slug,
				AdminHost:         req.AdminHost,
				StorefrontHost:    req.StorefrontHost,
				StorefrontWwwHost: req.StorefrontWwwHost,
				APIHost:           req.APIHost,
				BaseDomain:        req.BaseDomain,
				IsCustomDomain:    req.IsCustomDomain,
				Product:           req.Product,
				BusinessName:      req.BusinessName,
				Email:             req.Email,
			}

			if err := tenantReconciler.EnqueueCreate(event); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{
				"message":            fmt.Sprintf("Tenant host %s queued for provisioning", req.Slug),
				"slug":               req.Slug,
				"tenant_id":          req.TenantID,
				"admin_host":         req.AdminHost,
				"storefront_host":    req.StorefrontHost,
				"storefront_www_host": req.StorefrontWwwHost,
				"api_host":           req.APIHost,
				"is_custom_domain":   req.IsCustomDomain,
			})
		})

		// Cleanup endpoint - manually trigger cleanup of old deleted records
		api.POST("/cleanup", func(c *gin.Context) {
			deleted, err := tenantHostRepo.CleanupOldDeletedRecords(c.Request.Context(), cleanupRetention)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message":        "Cleanup completed",
				"records_deleted": deleted,
				"retention_days": 15,
			})
		})

		// Check if a slug is available or recently deleted
		api.GET("/slugs/:slug/availability", func(c *gin.Context) {
			slug := c.Param("slug")
			if slug == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
				return
			}

			// Check if slug exists (active)
			exists, err := tenantHostRepo.ExistsBySlug(c.Request.Context(), slug)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if exists {
				c.JSON(http.StatusOK, gin.H{
					"slug":      slug,
					"available": false,
					"reason":    "slug_in_use",
					"message":   "This slug is currently in use by an active tenant.",
				})
				return
			}

			// Check if slug was recently deleted
			deletedInfo, err := tenantHostRepo.GetDeletedSlugInfo(c.Request.Context(), slug)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if deletedInfo != nil && deletedInfo.DaysRemaining > 0 {
				c.JSON(http.StatusOK, gin.H{
					"slug":            slug,
					"available":       false,
					"reason":          "recently_deleted",
					"message":         fmt.Sprintf("This slug was recently deleted. It will be available again in %d days.", deletedInfo.DaysRemaining),
					"deleted_at":      deletedInfo.DeletedAt,
					"available_after": deletedInfo.AvailableAfter,
					"days_remaining":  deletedInfo.DaysRemaining,
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"slug":      slug,
				"available": true,
				"message":   "This slug is available.",
			})
		})

		// Sync endpoint - manually trigger reconciliation for a tenant
		api.POST("/hosts/:slug/sync", func(c *gin.Context) {
			slug := c.Param("slug")
			if slug == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
				return
			}

			if err := tenantReconciler.EnqueueSync(c.Request.Context(), slug); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{
				"message": fmt.Sprintf("Sync enqueued for tenant %s", slug),
				"slug":    slug,
			})
		})

		// Get tenant host status
		api.GET("/hosts/:slug", func(c *gin.Context) {
			slug := c.Param("slug")
			if slug == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
				return
			}

			record, err := tenantHostRepo.GetBySlug(c.Request.Context(), slug)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if record == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"slug":                 record.Slug,
				"tenant_id":            record.TenantID,
				"admin_host":           record.AdminHost,
				"storefront_host":      record.StorefrontHost,
				"status":               record.Status,
				"certificate_created":  record.CertificateCreated,
				"gateway_patched":      record.GatewayPatched,
				"admin_vs_patched":     record.AdminVSPatched,
				"storefront_vs_patched": record.StorefrontVSPatched,
				"provisioned_at":       record.ProvisionedAt,
				"last_error":           record.LastError,
			})
		})

		// List all tenant hosts
		api.GET("/hosts", func(c *gin.Context) {
			records, total, err := tenantHostRepo.List(c.Request.Context(), nil, 100, 0)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"total":   total,
				"records": records,
			})
		})

		// Sync VirtualService routes for a specific tenant
		// POST /api/v1/hosts/:slug/sync-routes
		// Body: {"vs_type": "api"} // admin, storefront, or api
		api.POST("/hosts/:slug/sync-routes", func(c *gin.Context) {
			slug := c.Param("slug")
			if slug == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
				return
			}

			var req struct {
				VSType string `json:"vs_type" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "vs_type is required (admin, storefront, or api)"})
				return
			}

			if err := routerService.SyncVirtualServiceRoutes(c.Request.Context(), slug, req.VSType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "VirtualService routes synced successfully",
				"slug":    slug,
				"vs_type": req.VSType,
			})
		})

		// Sync all tenant VirtualService routes from template
		// POST /api/v1/sync-vs-routes
		// Body: {"vs_type": "api"} // admin, storefront, or api
		api.POST("/sync-vs-routes", func(c *gin.Context) {
			var req struct {
				VSType string `json:"vs_type" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "vs_type is required (admin, storefront, or api)"})
				return
			}

			synced, err := routerService.SyncAllVirtualServiceRoutes(c.Request.Context(), req.VSType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "VirtualService routes synced successfully",
				"vs_type": req.VSType,
				"synced":  synced,
			})
		})
	}

	// Start server
	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Tenant Router Service starting on %s (internal mode)", serverAddr)
		log.Printf("  Database: %s:%s/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
		log.Printf("  NATS: %s", cfg.NATS.URL)
		log.Printf("  K8s Namespace: %s", cfg.Kubernetes.Namespace)
		log.Printf("  Base Domain: %s", cfg.Domain.BaseDomain)
		log.Printf("  Reconciler Workers: %d", workerCount)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop reconciler first (drain work queue)
	tenantReconciler.Stop()

	// Stop NATS subscriber
	if err := natsSubscriber.Stop(); err != nil {
		log.Printf("Error stopping NATS subscriber: %v", err)
	}

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}

	// Close Redis connection
	if redis != nil {
		if err := redis.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}

	// Close database connection
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	log.Println("Server stopped")
}

// runCleanup runs the cleanup job to remove old soft-deleted records
func runCleanup(ctx context.Context, repo repository.TenantHostRepository, retention time.Duration) {
	log.Printf("[Cleanup] Starting cleanup of records deleted more than %v ago", retention)
	deleted, err := repo.CleanupOldDeletedRecords(ctx, retention)
	if err != nil {
		log.Printf("[Cleanup] Error during cleanup: %v", err)
		return
	}
	log.Printf("[Cleanup] Cleanup completed: %d records permanently deleted", deleted)
}

// syncGatewayIP fetches the custom domain gateway IP from K8s and stores it in Redis
func syncGatewayIP(ctx context.Context, k8sClient *k8s.Client, redis *redisClient.Client) {
	ip, err := k8sClient.GetCustomDomainGatewayIP(ctx)
	if err != nil {
		log.Printf("[GatewaySync] Failed to fetch gateway IP from K8s: %v", err)
		return
	}

	if err := redis.SetCustomDomainGatewayIP(ctx, ip); err != nil {
		log.Printf("[GatewaySync] Failed to store gateway IP in Redis: %v", err)
		return
	}

	log.Printf("[GatewaySync] Custom domain gateway IP synced to Redis: %s", ip)
}
