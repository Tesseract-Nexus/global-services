package reconciler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"tenant-router-service/internal/config"
	"tenant-router-service/internal/k8s"
	"tenant-router-service/internal/keycloak"
	"tenant-router-service/internal/models"
	"tenant-router-service/internal/repository"
)

// ReconcileResult represents the result of a reconciliation
type ReconcileResult struct {
	Requeue      bool          // Whether to requeue the item
	RequeueAfter time.Duration // Duration to wait before requeuing
}

// Condition represents a status condition (Kubebuilder pattern)
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"` // True, False, Unknown
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// Conditions constants
const (
	ConditionCertificateReady        = "CertificateReady"
	ConditionGatewayConfigured       = "GatewayConfigured"
	ConditionAdminVSConfigured       = "AdminVSConfigured"
	ConditionStorefrontConfigured    = "StorefrontVSConfigured"
	ConditionStorefrontWwwConfigured = "StorefrontWwwVSConfigured"
	ConditionAPIVSConfigured         = "APIVSConfigured"
	ConditionAuthPolicyConfigured    = "AuthPolicyConfigured"
	ConditionReady                   = "Ready"

	StatusTrue    = "True"
	StatusFalse   = "False"
	StatusUnknown = "Unknown"

	ReasonProvisioning   = "Provisioning"
	ReasonProvisioned    = "Provisioned"
	ReasonFailed         = "Failed"
	ReasonResourceExists = "ResourceExists"
)

// WorkItem represents an item in the work queue
type WorkItem struct {
	Key       string // slug as unique key
	Event     interface{} // TenantCreatedEvent or TenantDeletedEvent
	Operation string // "create" or "delete"
	AddedAt   time.Time
	Attempts  int
}

// TenantReconciler reconciles tenant routing configuration
// Follows Kubebuilder reconciler pattern with work queue and rate limiting
type TenantReconciler struct {
	k8sClient      *k8s.Client
	keycloakClient *keycloak.Client
	repo           repository.TenantHostRepository
	config         *config.Config

	// Work queue for processing events
	workQueue  chan *WorkItem
	inProgress map[string]bool
	mu         sync.Mutex

	// Rate limiter for K8s API calls (Kubebuilder best practice)
	rateLimiter *rate.Limiter

	// Metrics
	metrics *ReconcilerMetrics

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ReconcilerMetrics tracks reconciler performance (internal)
type ReconcilerMetrics struct {
	mu                   sync.RWMutex
	ReconcileTotal       int64
	ReconcileSuccessful  int64
	ReconcileFailed      int64
	ReconcileDuration    time.Duration
	LastReconcileTime    time.Time
	CurrentQueueDepth    int
	RetryCount           int64
}

// MetricsSnapshot is a read-only snapshot of metrics (safe to copy)
type MetricsSnapshot struct {
	ReconcileTotal       int64         `json:"reconcile_total"`
	ReconcileSuccessful  int64         `json:"reconcile_successful"`
	ReconcileFailed      int64         `json:"reconcile_failed"`
	ReconcileDuration    time.Duration `json:"reconcile_duration"`
	LastReconcileTime    time.Time     `json:"last_reconcile_time"`
	CurrentQueueDepth    int           `json:"current_queue_depth"`
	RetryCount           int64         `json:"retry_count"`
}

// NewTenantReconciler creates a new reconciler
func NewTenantReconciler(k8sClient *k8s.Client, keycloakClient *keycloak.Client, repo repository.TenantHostRepository, cfg *config.Config) *TenantReconciler {
	ctx, cancel := context.WithCancel(context.Background())

	return &TenantReconciler{
		k8sClient:      k8sClient,
		keycloakClient: keycloakClient,
		repo:           repo,
		config:         cfg,
		workQueue:   make(chan *WorkItem, 100), // Buffer for 100 items
		inProgress:  make(map[string]bool),
		rateLimiter: rate.NewLimiter(rate.Limit(10), 20), // 10 req/s, burst 20
		metrics:     &ReconcilerMetrics{},
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins processing the work queue with specified number of workers
func (r *TenantReconciler) Start(workers int) {
	log.Printf("[Reconciler] Starting with %d workers", workers)

	for i := 0; i < workers; i++ {
		r.wg.Add(1)
		go r.worker(i)
	}

	// Run startup reconciliation in background after workers are ready
	go r.ReconcileOnStartup()
}

// ReconcileOnStartup scans the database for incomplete or pending records and enqueues them
// This ensures any records that were not fully provisioned (e.g., due to service restart)
// are picked up and completed. Also handles deletion of records marked for removal.
func (r *TenantReconciler) ReconcileOnStartup() {
	// Wait a bit for workers to be ready
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	log.Println("[Reconciler] Starting startup reconciliation...")

	// 1. Find incomplete records (pending or partially provisioned)
	incompleteRecords, err := r.repo.ListIncomplete(ctx)
	if err != nil {
		log.Printf("[Reconciler] Failed to list incomplete records: %v", err)
	} else {
		log.Printf("[Reconciler] Found %d incomplete records to reconcile", len(incompleteRecords))
		for _, record := range incompleteRecords {
			// Check if VirtualServices already exist in K8s
			adminVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-admin-vs", record.Slug))
			storefrontVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-storefront-vs", record.Slug))
			apiVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-api-vs", record.Slug))

			// If VS already exist but DB says not, update DB
			if adminVSExists && !record.AdminVSPatched {
				log.Printf("[Reconciler] VirtualService %s-admin-vs already exists, updating database", record.Slug)
				r.repo.UpdateProvisioningState(ctx, record.Slug, "admin_vs_patched", true, "devtest")
				record.AdminVSPatched = true
			}
			if storefrontVSExists && !record.StorefrontVSPatched {
				log.Printf("[Reconciler] VirtualService %s-storefront-vs already exists, updating database", record.Slug)
				r.repo.UpdateProvisioningState(ctx, record.Slug, "storefront_vs_patched", true, "devtest")
				record.StorefrontVSPatched = true
			}
			if apiVSExists && !record.APIVSPatched {
				log.Printf("[Reconciler] VirtualService %s-api-vs already exists, updating database", record.Slug)
				r.repo.UpdateProvisioningState(ctx, record.Slug, "api_vs_patched", true, "devtest")
				record.APIVSPatched = true
			}

			// If still incomplete after DB sync, enqueue for reconciliation
			if !record.CertificateCreated || !record.GatewayPatched || !record.AdminVSPatched || !record.StorefrontVSPatched || !record.APIVSPatched {
				event := &models.TenantCreatedEvent{
					TenantID:       record.TenantID,
					Slug:           record.Slug,
					AdminHost:      record.AdminHost,
					StorefrontHost: record.StorefrontHost,
				}
				if err := r.EnqueueCreate(event); err != nil {
					log.Printf("[Reconciler] Failed to enqueue incomplete record %s: %v", record.Slug, err)
				} else {
					log.Printf("[Reconciler] Enqueued incomplete record %s for reconciliation", record.Slug)
				}
			} else {
				// All resources exist, mark as provisioned
				if record.Status != models.HostStatusProvisioned {
					log.Printf("[Reconciler] Marking %s as provisioned (all resources exist)", record.Slug)
					r.repo.MarkProvisioned(ctx, record.Slug)
				}
			}
		}
	}

	// 2. Find records marked for deletion
	deletingRecords, err := r.repo.ListDeleting(ctx)
	if err != nil {
		log.Printf("[Reconciler] Failed to list deleting records: %v", err)
	} else {
		log.Printf("[Reconciler] Found %d records pending deletion", len(deletingRecords))
		for _, record := range deletingRecords {
			event := &models.TenantDeletedEvent{
				TenantID:       record.TenantID,
				Slug:           record.Slug,
				AdminHost:      record.AdminHost,
				StorefrontHost: record.StorefrontHost,
			}
			if err := r.EnqueueDelete(event); err != nil {
				log.Printf("[Reconciler] Failed to enqueue deletion for %s: %v", record.Slug, err)
			} else {
				log.Printf("[Reconciler] Enqueued deletion for %s", record.Slug)
			}
		}
	}

	log.Println("[Reconciler] Startup reconciliation completed")
}

// EnqueueSync manually enqueues a tenant for reconciliation (for API endpoint)
func (r *TenantReconciler) EnqueueSync(ctx context.Context, slug string) error {
	record, err := r.repo.GetBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("failed to get record: %w", err)
	}
	if record == nil {
		return fmt.Errorf("tenant %s not found", slug)
	}

	event := &models.TenantCreatedEvent{
		TenantID:       record.TenantID,
		Slug:           record.Slug,
		AdminHost:      record.AdminHost,
		StorefrontHost: record.StorefrontHost,
	}
	return r.EnqueueCreate(event)
}

// Stop gracefully shuts down the reconciler
func (r *TenantReconciler) Stop() {
	log.Println("[Reconciler] Stopping...")
	r.cancel()
	close(r.workQueue)
	r.wg.Wait()
	log.Println("[Reconciler] Stopped")
}

// EnqueueCreate adds a create operation to the work queue
func (r *TenantReconciler) EnqueueCreate(event *models.TenantCreatedEvent) error {
	item := &WorkItem{
		Key:       event.Slug,
		Event:     event,
		Operation: "create",
		AddedAt:   time.Now(),
		Attempts:  0,
	}

	select {
	case r.workQueue <- item:
		r.metrics.mu.Lock()
		r.metrics.CurrentQueueDepth++
		r.metrics.mu.Unlock()
		log.Printf("[Reconciler] Enqueued create for %s", event.Slug)
		return nil
	case <-r.ctx.Done():
		return fmt.Errorf("reconciler is shutting down")
	default:
		return fmt.Errorf("work queue is full")
	}
}

// EnqueueDelete adds a delete operation to the work queue
func (r *TenantReconciler) EnqueueDelete(event *models.TenantDeletedEvent) error {
	item := &WorkItem{
		Key:       event.Slug,
		Event:     event,
		Operation: "delete",
		AddedAt:   time.Now(),
		Attempts:  0,
	}

	select {
	case r.workQueue <- item:
		r.metrics.mu.Lock()
		r.metrics.CurrentQueueDepth++
		r.metrics.mu.Unlock()
		log.Printf("[Reconciler] Enqueued delete for %s", event.Slug)
		return nil
	case <-r.ctx.Done():
		return fmt.Errorf("reconciler is shutting down")
	default:
		return fmt.Errorf("work queue is full")
	}
}

// worker processes items from the work queue
func (r *TenantReconciler) worker(id int) {
	defer r.wg.Done()
	log.Printf("[Reconciler] Worker %d started", id)

	for {
		select {
		case <-r.ctx.Done():
			log.Printf("[Reconciler] Worker %d stopping", id)
			return
		case item, ok := <-r.workQueue:
			if !ok {
				log.Printf("[Reconciler] Worker %d: queue closed", id)
				return
			}
			r.processItem(id, item)
		}
	}
}

// processItem handles a single work item with retry logic
func (r *TenantReconciler) processItem(workerID int, item *WorkItem) {
	r.metrics.mu.Lock()
	r.metrics.CurrentQueueDepth--
	r.metrics.ReconcileTotal++
	r.metrics.mu.Unlock()

	// Check if already being processed (idempotency)
	r.mu.Lock()
	if r.inProgress[item.Key] {
		r.mu.Unlock()
		log.Printf("[Reconciler] Worker %d: %s already in progress, skipping", workerID, item.Key)
		return
	}
	r.inProgress[item.Key] = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.inProgress, item.Key)
		r.mu.Unlock()
	}()

	// Rate limit K8s API calls
	if err := r.rateLimiter.Wait(r.ctx); err != nil {
		log.Printf("[Reconciler] Rate limit wait cancelled: %v", err)
		return
	}

	startTime := time.Now()
	var result ReconcileResult
	var err error

	switch item.Operation {
	case "create":
		event := item.Event.(*models.TenantCreatedEvent)
		result, err = r.reconcileCreate(r.ctx, event)
	case "delete":
		event := item.Event.(*models.TenantDeletedEvent)
		result, err = r.reconcileDelete(r.ctx, event)
	}

	duration := time.Since(startTime)

	r.metrics.mu.Lock()
	r.metrics.ReconcileDuration = duration
	r.metrics.LastReconcileTime = time.Now()
	r.metrics.mu.Unlock()

	if err != nil {
		r.metrics.mu.Lock()
		r.metrics.ReconcileFailed++
		r.metrics.mu.Unlock()
		log.Printf("[Reconciler] Worker %d: failed to reconcile %s: %v", workerID, item.Key, err)

		// Requeue with exponential backoff
		if result.Requeue && item.Attempts < 5 {
			item.Attempts++
			backoff := r.calculateBackoff(item.Attempts)
			log.Printf("[Reconciler] Requeuing %s after %v (attempt %d)", item.Key, backoff, item.Attempts)

			r.metrics.mu.Lock()
			r.metrics.RetryCount++
			r.metrics.mu.Unlock()

			go func() {
				select {
				case <-time.After(backoff):
					r.workQueue <- item
				case <-r.ctx.Done():
				}
			}()
		}
		return
	}

	r.metrics.mu.Lock()
	r.metrics.ReconcileSuccessful++
	r.metrics.mu.Unlock()

	log.Printf("[Reconciler] Worker %d: successfully reconciled %s in %v", workerID, item.Key, duration)
}

// calculateBackoff returns exponential backoff duration
func (r *TenantReconciler) calculateBackoff(attempt int) time.Duration {
	// Base: 1s, max: 5m, factor: 2
	base := time.Second
	max := 5 * time.Minute

	backoff := base * time.Duration(1<<uint(attempt-1))
	if backoff > max {
		backoff = max
	}
	return backoff
}

// reconcileCreate handles tenant creation with idempotent operations
func (r *TenantReconciler) reconcileCreate(ctx context.Context, event *models.TenantCreatedEvent) (ReconcileResult, error) {
	log.Printf("[Reconciler] Reconciling create for %s (custom_domain=%v)", event.Slug, event.IsCustomDomain)

	// Use host URLs from event (provided by tenant-service)
	adminHost := event.AdminHost
	storefrontHost := event.StorefrontHost
	storefrontWwwHost := event.StorefrontWwwHost
	apiHost := event.APIHost

	// Fallback to config-based generation if not provided
	domain := r.config.Domain.BaseDomain
	if adminHost == "" || storefrontHost == "" {
		adminHost = fmt.Sprintf("%s-admin.%s", event.Slug, domain)
		storefrontHost = fmt.Sprintf("%s.%s", event.Slug, domain)
	}

	// API host for mobile/external access (use provided or generate based on slug)
	if apiHost == "" {
		apiHost = fmt.Sprintf("%s-api.%s", event.Slug, domain)
	}

	certName := fmt.Sprintf("%s-tenant-tls", event.Slug)

	// Check if already exists in database (idempotency)
	existing, err := r.repo.GetBySlug(ctx, event.Slug)
	if err != nil {
		return ReconcileResult{Requeue: true}, fmt.Errorf("failed to check existing record: %w", err)
	}

	var record *models.TenantHostRecord
	if existing != nil {
		if existing.Status == models.HostStatusProvisioned {
			log.Printf("[Reconciler] %s already provisioned, ensuring state", event.Slug)
			// Verify state is correct (reconcile to desired state)
			return r.ensureState(ctx, existing)
		}
		record = existing
	} else {
		// Create new record
		record = &models.TenantHostRecord{
			TenantID:          event.TenantID,
			Slug:              event.Slug,
			AdminHost:         adminHost,
			StorefrontHost:    storefrontHost,
			StorefrontWwwHost: storefrontWwwHost,
			APIHost:           apiHost,
			BaseDomain:        event.BaseDomain,
			IsCustomDomain:    event.IsCustomDomain,
			CertName:          certName,
			Status:            models.HostStatusPending,
			Product:           event.Product,
			BusinessName:      event.BusinessName,
			Email:             event.Email,
		}
		if err := r.repo.Create(ctx, record); err != nil {
			return ReconcileResult{Requeue: true}, fmt.Errorf("failed to create record: %w", err)
		}
	}

	// Reconcile each resource (idempotent operations)
	conditions := make([]Condition, 0, 6)

	// 1. Certificate
	if !record.CertificateCreated {
		if err := r.reconcileCertificate(ctx, record); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionCertificateReady,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			r.updateConditions(ctx, record, conditions)
			return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		conditions = append(conditions, Condition{
			Type:               ConditionCertificateReady,
			Status:             StatusTrue,
			LastTransitionTime: time.Now(),
			Reason:             ReasonProvisioned,
			Message:            "Certificate created successfully",
		})
	}

	// 2. Gateway - Handle differently for custom domains vs default domains
	// For custom domains: Create a DEDICATED gateway with proper TLS certificate
	// For default domains: Use wildcard certificate (skip gateway patch)
	var dedicatedGatewayName string
	if !record.GatewayPatched {
		if record.IsCustomDomain {
			// Custom domains get their own dedicated gateway for better isolation
			// This avoids SNI conflicts and allows per-domain TLS certificates
			domains := []string{record.StorefrontHost}
			if record.AdminHost != "" && record.AdminHost != record.StorefrontHost {
				domains = append(domains, record.AdminHost)
			}
			if record.StorefrontWwwHost != "" {
				domains = append(domains, record.StorefrontWwwHost)
			}
			if record.APIHost != "" {
				domains = append(domains, record.APIHost)
			}

			certSecretName := fmt.Sprintf("%s-tenant-tls", record.Slug)
			gatewayName, err := r.k8sClient.CreateDedicatedGateway(ctx, record.Slug, domains, certSecretName)
			if err != nil {
				conditions = append(conditions, Condition{
					Type:               ConditionGatewayConfigured,
					Status:             StatusFalse,
					LastTransitionTime: time.Now(),
					Reason:             ReasonFailed,
					Message:            err.Error(),
				})
				r.updateConditions(ctx, record, conditions)
				return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
			}
			dedicatedGatewayName = gatewayName
			r.repo.UpdateProvisioningState(ctx, record.Slug, "gateway_patched", true, r.config.Kubernetes.CustomDomainGatewayNS)
			conditions = append(conditions, Condition{
				Type:               ConditionGatewayConfigured,
				Status:             StatusTrue,
				LastTransitionTime: time.Now(),
				Reason:             ReasonProvisioned,
				Message:            fmt.Sprintf("Created dedicated gateway %s for custom domain", gatewayName),
			})
			log.Printf("[Reconciler] Created dedicated gateway %s for custom domain tenant %s", gatewayName, record.Slug)

			// Also create dedicated AuthorizationPolicy for the custom domain
			if err := r.k8sClient.CreateDedicatedAuthorizationPolicy(ctx, record.Slug, domains); err != nil {
				log.Printf("[Reconciler] Warning: Failed to create dedicated AuthorizationPolicy for %s: %v", record.Slug, err)
				// Don't fail - the gateway is created, just log the warning
			} else {
				log.Printf("[Reconciler] Created dedicated AuthorizationPolicy for custom domain tenant %s", record.Slug)
			}
		} else if r.config.Kubernetes.SkipGatewayPatch {
			// Default domain: Mark as patched since wildcard cert handles this
			log.Printf("[Reconciler] Skipping gateway patch for %s (using wildcard cert %s)",
				record.Slug, r.config.Kubernetes.WildcardCertName)
			r.repo.UpdateProvisioningState(ctx, record.Slug, "gateway_patched", true, "wildcard")
			conditions = append(conditions, Condition{
				Type:               ConditionGatewayConfigured,
				Status:             StatusTrue,
				LastTransitionTime: time.Now(),
				Reason:             ReasonProvisioned,
				Message:            fmt.Sprintf("Using wildcard certificate %s", r.config.Kubernetes.WildcardCertName),
			})
		} else {
			// Patch shared gateway (legacy behavior)
			if err := r.reconcileGateway(ctx, record, "add"); err != nil {
				conditions = append(conditions, Condition{
					Type:               ConditionGatewayConfigured,
					Status:             StatusFalse,
					LastTransitionTime: time.Now(),
					Reason:             ReasonFailed,
					Message:            err.Error(),
				})
				r.updateConditions(ctx, record, conditions)
				return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
			}
			conditions = append(conditions, Condition{
				Type:               ConditionGatewayConfigured,
				Status:             StatusTrue,
				LastTransitionTime: time.Now(),
				Reason:             ReasonProvisioned,
				Message:            "Gateway configured successfully",
			})
		}
	} else if record.IsCustomDomain {
		// Gateway already patched - recover the dedicated gateway name for VirtualService creation
		dedicatedGatewayName = fmt.Sprintf("%s-gateway", record.Slug)
	}

	// 3. Admin VirtualService
	if !record.AdminVSPatched {
		if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.AdminVSName, record.AdminHost, "add", dedicatedGatewayName); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionAdminVSConfigured,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			r.updateConditions(ctx, record, conditions)
			return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		conditions = append(conditions, Condition{
			Type:               ConditionAdminVSConfigured,
			Status:             StatusTrue,
			LastTransitionTime: time.Now(),
			Reason:             ReasonProvisioned,
			Message:            "Admin VirtualService configured successfully",
		})
	}

	// 4. Storefront VirtualService
	if !record.StorefrontVSPatched {
		if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.StorefrontVSName, record.StorefrontHost, "add", dedicatedGatewayName); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionStorefrontConfigured,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			r.updateConditions(ctx, record, conditions)
			return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		conditions = append(conditions, Condition{
			Type:               ConditionStorefrontConfigured,
			Status:             StatusTrue,
			LastTransitionTime: time.Now(),
			Reason:             ReasonProvisioned,
			Message:            "Storefront VirtualService configured successfully",
		})
	}

	// 5. Storefront www VirtualService (only for custom domains with www subdomain)
	// Use "www" suffix to create unique VS name (e.g., custom-store-storefront-www-vs)
	// Custom domain's www subdomain - we don't control this DNS, so proxied setting doesn't matter
	// but we set it to false for consistency with other custom domain resources
	if record.StorefrontWwwHost != "" && !record.StorefrontWwwVSPatched {
		log.Printf("[Reconciler] Creating www VirtualService for %s (host: %s)", record.Slug, record.StorefrontWwwHost)
		if err := r.reconcileVirtualServiceWithSuffix(ctx, record, r.config.Kubernetes.StorefrontVSName, record.StorefrontWwwHost, "add", "www", false, dedicatedGatewayName); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionStorefrontWwwConfigured,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			r.updateConditions(ctx, record, conditions)
			return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		// Mark as patched - we need to add a separate update for this
		r.repo.UpdateProvisioningState(ctx, record.Slug, "storefront_www_vs_patched", true, "")
		conditions = append(conditions, Condition{
			Type:               ConditionStorefrontWwwConfigured,
			Status:             StatusTrue,
			LastTransitionTime: time.Now(),
			Reason:             ReasonProvisioned,
			Message:            "Storefront www VirtualService configured successfully",
		})
	}

	// 6. API VirtualService (for mobile/external API access)
	if !record.APIVSPatched {
		// Use APIHost if set, otherwise generate from slug
		apiHost := record.APIHost
		if apiHost == "" {
			apiHost = fmt.Sprintf("%s-api.%s", record.Slug, r.config.Domain.BaseDomain)
		}
		if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.APIVSName, apiHost, "add", dedicatedGatewayName); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionAPIVSConfigured,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			r.updateConditions(ctx, record, conditions)
			return ReconcileResult{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		conditions = append(conditions, Condition{
			Type:               ConditionAPIVSConfigured,
			Status:             StatusTrue,
			LastTransitionTime: time.Now(),
			Reason:             ReasonProvisioned,
			Message:            "API VirtualService configured successfully",
		})
	}

	// NOTE: For custom domain tenants, we NO LONGER create platform subdomain VirtualServices
	// Custom domains use the dedicated custom-domain-gateway with direct A record access
	// (LoadBalancer IP: 34.151.169.37). There's no need for CNAME targets on platform subdomains.
	// The customer simply points their domain's A record to the LoadBalancer IP, and
	// Let's Encrypt issues a certificate via HTTP-01 challenge.

	// 7. Update shared AuthorizationPolicy for custom domains
	// Custom domain VirtualServices need their hosts added to the AuthorizationPolicy
	// to allow traffic through the custom-ingressgateway
	if record.IsCustomDomain {
		if err := r.reconcileAuthorizationPolicy(ctx, record, "add"); err != nil {
			conditions = append(conditions, Condition{
				Type:               ConditionAuthPolicyConfigured,
				Status:             StatusFalse,
				LastTransitionTime: time.Now(),
				Reason:             ReasonFailed,
				Message:            err.Error(),
			})
			// Log but don't fail - the VirtualServices are created, just RBAC might be incomplete
			log.Printf("[Reconciler] Warning: Failed to update AuthorizationPolicy for %s: %v", record.Slug, err)
		} else {
			conditions = append(conditions, Condition{
				Type:               ConditionAuthPolicyConfigured,
				Status:             StatusTrue,
				LastTransitionTime: time.Now(),
				Reason:             ReasonProvisioned,
				Message:            "AuthorizationPolicy updated successfully",
			})
		}
	}

	// 8. Update Keycloak OIDC client redirect URIs for this tenant's hosts
	if r.keycloakClient != nil && r.config.Keycloak.Enabled {
		hosts := []string{record.AdminHost, record.StorefrontHost}
		if record.StorefrontWwwHost != "" {
			hosts = append(hosts, record.StorefrontWwwHost)
		}
		if record.APIHost != "" {
			hosts = append(hosts, record.APIHost)
		}
		if err := r.keycloakClient.AddTenantRedirectURIs(ctx, hosts); err != nil {
			log.Printf("[Reconciler] Warning: Failed to update Keycloak redirect URIs for %s: %v", record.Slug, err)
			// Don't fail provisioning — Keycloak update is non-critical for routing
		} else {
			log.Printf("[Reconciler] Updated Keycloak redirect URIs for %s", record.Slug)
		}
	}

	// All resources provisioned - mark as ready
	conditions = append(conditions, Condition{
		Type:               ConditionReady,
		Status:             StatusTrue,
		LastTransitionTime: time.Now(),
		Reason:             ReasonProvisioned,
		Message:            "All resources provisioned successfully",
	})
	r.updateConditions(ctx, record, conditions)

	if err := r.repo.MarkProvisioned(ctx, record.Slug); err != nil {
		log.Printf("[Reconciler] Failed to mark as provisioned: %v", err)
	}

	r.logActivity(ctx, record.ID, "reconcile_complete", "all", "", true, "", time.Duration(0))

	return ReconcileResult{}, nil
}

// reconcileDelete handles tenant deletion
func (r *TenantReconciler) reconcileDelete(ctx context.Context, event *models.TenantDeletedEvent) (ReconcileResult, error) {
	log.Printf("[Reconciler] Reconciling delete for %s", event.Slug)

	record, err := r.repo.GetBySlug(ctx, event.Slug)
	if err != nil {
		return ReconcileResult{Requeue: true}, err
	}
	if record == nil {
		log.Printf("[Reconciler] No record found for %s, nothing to delete", event.Slug)
		return ReconcileResult{}, nil
	}

	// Mark as deleting
	r.repo.UpdateStatus(ctx, event.Slug, models.HostStatusDeleting)

	// Use host URLs from event or record
	adminHost := event.AdminHost
	storefrontHost := event.StorefrontHost
	if adminHost == "" {
		adminHost = record.AdminHost
	}
	if storefrontHost == "" {
		storefrontHost = record.StorefrontHost
	}
	apiHost := record.APIHost
	if apiHost == "" {
		apiHost = fmt.Sprintf("%s-api.%s", record.Slug, r.config.Domain.BaseDomain)
	}

	// Remove in reverse order
	// 0. Remove hosts from AuthorizationPolicy (for custom domains)
	if record.IsCustomDomain {
		if err := r.reconcileAuthorizationPolicy(ctx, record, "remove"); err != nil {
			log.Printf("[Reconciler] Failed to remove from AuthorizationPolicy: %v", err)
		}
	}

	// 1. Remove API VirtualService
	if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.APIVSName, apiHost, "remove", ""); err != nil {
		log.Printf("[Reconciler] Failed to remove from API VS: %v", err)
	}

	// 2. Remove from Storefront VirtualService
	if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.StorefrontVSName, storefrontHost, "remove", ""); err != nil {
		log.Printf("[Reconciler] Failed to remove from storefront VS: %v", err)
	}

	// 2.5. Remove www Storefront VirtualService (for custom domains)
	if record.StorefrontWwwHost != "" {
		if err := r.k8sClient.DeleteTenantVirtualServiceWithSuffix(ctx, record.Slug, r.config.Kubernetes.StorefrontVSName, "www"); err != nil {
			log.Printf("[Reconciler] Failed to remove www storefront VS: %v", err)
		}
	}

	// 3. Remove from Admin VirtualService
	if err := r.reconcileVirtualService(ctx, record, r.config.Kubernetes.AdminVSName, adminHost, "remove", ""); err != nil {
		log.Printf("[Reconciler] Failed to remove from admin VS: %v", err)
	}

	// 4. Remove from Gateway (or delete dedicated gateway for custom domains)
	if record.IsCustomDomain {
		// Delete dedicated gateway for custom domains
		if err := r.k8sClient.DeleteDedicatedGateway(ctx, record.Slug); err != nil {
			log.Printf("[Reconciler] Failed to delete dedicated gateway: %v", err)
		}
		// Delete dedicated AuthorizationPolicy for custom domains
		if err := r.k8sClient.DeleteDedicatedAuthorizationPolicy(ctx, record.Slug); err != nil {
			log.Printf("[Reconciler] Failed to delete dedicated AuthorizationPolicy: %v", err)
		}
	} else {
		// Remove from shared gateway for default domains
		if err := r.reconcileGateway(ctx, record, "remove"); err != nil {
			log.Printf("[Reconciler] Failed to remove from gateway: %v", err)
		}
	}

	// 5. Delete Certificate
	if err := r.k8sClient.DeleteCertificate(ctx, event.Slug); err != nil {
		log.Printf("[Reconciler] Failed to delete certificate: %v", err)
	}

	// 6. Remove Keycloak redirect URIs
	if r.keycloakClient != nil && r.config.Keycloak.Enabled {
		hosts := []string{adminHost, storefrontHost}
		if record.StorefrontWwwHost != "" {
			hosts = append(hosts, record.StorefrontWwwHost)
		}
		if apiHost != "" {
			hosts = append(hosts, apiHost)
		}
		if err := r.keycloakClient.RemoveTenantRedirectURIs(ctx, hosts); err != nil {
			log.Printf("[Reconciler] Warning: Failed to remove Keycloak redirect URIs for %s: %v", event.Slug, err)
		}
	}

	// Soft delete from database
	if err := r.repo.Delete(ctx, event.Slug); err != nil {
		log.Printf("[Reconciler] Failed to delete record: %v", err)
	}

	return ReconcileResult{}, nil
}

// ensureState verifies and corrects the current state (Kubebuilder pattern)
// This is critical for handling cases where K8s resources were manually deleted
// but the DB still shows "provisioned" status
func (r *TenantReconciler) ensureState(ctx context.Context, record *models.TenantHostRecord) (ReconcileResult, error) {
	log.Printf("[Reconciler] Ensuring state for %s - verifying K8s resources exist", record.Slug)

	// Check if VirtualServices actually exist in K8s
	adminVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-admin-vs", record.Slug))
	storefrontVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-storefront-vs", record.Slug))
	apiVSExists := r.k8sClient.VirtualServiceExists(ctx, fmt.Sprintf("%s-api-vs", record.Slug))

	needsReconcile := false

	// If K8s resource is missing but DB says it's created, reset the DB flag
	if !adminVSExists && record.AdminVSPatched {
		log.Printf("[Reconciler] %s admin VS missing in K8s, resetting flag for re-creation", record.Slug)
		r.repo.UpdateProvisioningState(ctx, record.Slug, "admin_vs_patched", false, "")
		record.AdminVSPatched = false
		needsReconcile = true
	}

	if !storefrontVSExists && record.StorefrontVSPatched {
		log.Printf("[Reconciler] %s storefront VS missing in K8s, resetting flag for re-creation", record.Slug)
		r.repo.UpdateProvisioningState(ctx, record.Slug, "storefront_vs_patched", false, "")
		record.StorefrontVSPatched = false
		needsReconcile = true
	}

	if !apiVSExists && record.APIVSPatched {
		log.Printf("[Reconciler] %s API VS missing in K8s, resetting flag for re-creation", record.Slug)
		r.repo.UpdateProvisioningState(ctx, record.Slug, "api_vs_patched", false, "")
		record.APIVSPatched = false
		needsReconcile = true
	}

	if needsReconcile {
		log.Printf("[Reconciler] %s needs re-reconciliation, resetting status to pending", record.Slug)
		r.repo.UpdateStatus(ctx, record.Slug, models.HostStatusPending)

		// Re-run full reconciliation with the updated record
		event := &models.TenantCreatedEvent{
			TenantID:       record.TenantID,
			Slug:           record.Slug,
			AdminHost:      record.AdminHost,
			StorefrontHost: record.StorefrontHost,
		}
		return r.reconcileCreate(ctx, event)
	}

	log.Printf("[Reconciler] %s state verified - all K8s resources exist", record.Slug)
	return ReconcileResult{}, nil
}

// reconcileCertificate creates or verifies the certificate
// For custom domains, certificates are created in the custom domain gateway namespace (istio-ingress)
// using HTTP-01 challenge with Let's Encrypt. For default domains, the wildcard cert is used.
func (r *TenantReconciler) reconcileCertificate(ctx context.Context, record *models.TenantHostRecord) error {
	startTime := time.Now()
	var err error
	var namespace string

	if record.IsCustomDomain {
		// Custom domain: create certificate in istio-ingress namespace with HTTP-01 challenge
		// Collect all custom domain hosts for the certificate
		domains := []string{record.StorefrontHost}
		if record.AdminHost != "" && record.AdminHost != record.StorefrontHost {
			domains = append(domains, record.AdminHost)
		}
		if record.StorefrontWwwHost != "" {
			domains = append(domains, record.StorefrontWwwHost)
		}
		if record.APIHost != "" {
			domains = append(domains, record.APIHost)
		}

		err = r.k8sClient.CreateCustomDomainCertificate(ctx, record.Slug, domains)
		namespace = r.config.Kubernetes.CustomDomainGatewayNS
		log.Printf("[Reconciler] Creating custom domain certificate for %s with domains: %v", record.Slug, domains)
	} else {
		// Default domain: use existing certificate creation (or skip if using wildcard)
		err = r.k8sClient.CreateCertificate(ctx, record.Slug, record.AdminHost, record.StorefrontHost)
		namespace = r.config.Kubernetes.Namespace
	}

	if err != nil {
		r.logActivity(ctx, record.ID, "create_certificate", "Certificate", namespace, false, err.Error(), time.Since(startTime))
		return err
	}
	r.repo.UpdateProvisioningState(ctx, record.Slug, "certificate_created", true, namespace)
	r.logActivity(ctx, record.ID, "create_certificate", "Certificate", namespace, true, "", time.Since(startTime))
	return nil
}

// reconcileGateway adds or removes gateway server entries
func (r *TenantReconciler) reconcileGateway(ctx context.Context, record *models.TenantHostRecord, operation string) error {
	startTime := time.Now()
	gwNamespace, err := r.k8sClient.PatchGatewayServer(ctx, record.Slug, record.AdminHost, record.StorefrontHost, operation)
	if err != nil {
		r.logActivity(ctx, record.ID, "patch_gateway", "Gateway", gwNamespace, false, err.Error(), time.Since(startTime))
		return err
	}
	if operation == "add" {
		r.repo.UpdateProvisioningState(ctx, record.Slug, "gateway_patched", true, gwNamespace)
	}
	r.logActivity(ctx, record.ID, "patch_gateway", "Gateway", gwNamespace, true, "", time.Since(startTime))
	return nil
}

// reconcileVirtualService creates or deletes a tenant-specific VirtualService
// Uses the multi-tenant isolation pattern - each tenant gets their own VS instead of patching shared ones
// cloudflareProxied controls whether the DNS record should be proxied through Cloudflare:
// - true for default domain tenants (protected by Cloudflare)
// - false for custom domain tenants' platform subdomains (allows external CNAME)
// dedicatedGatewayName is used for custom domains to reference their dedicated gateway
func (r *TenantReconciler) reconcileVirtualService(ctx context.Context, record *models.TenantHostRecord, templateVSName, host, operation, dedicatedGatewayName string) error {
	// Default domain tenants get Cloudflare proxy enabled for protection
	// Custom domain tenants get proxy disabled so external domains can CNAME to them
	cloudflareProxied := !record.IsCustomDomain
	return r.reconcileVirtualServiceWithSuffix(ctx, record, templateVSName, host, operation, "", cloudflareProxied, dedicatedGatewayName)
}

// reconcileVirtualServiceWithSuffix creates or deletes a tenant-specific VirtualService with an optional name suffix
// The suffix is used when creating multiple VirtualServices from the same template (e.g., storefront + www)
// cloudflareProxied controls the external-dns cloudflare-proxied annotation for DNS record creation
// For custom domains, the VirtualService references the dedicated gateway instead of the default gateway
// dedicatedGatewayName specifies the dedicated gateway for custom domains (if empty, uses default custom-domain-gateway)
func (r *TenantReconciler) reconcileVirtualServiceWithSuffix(ctx context.Context, record *models.TenantHostRecord, templateVSName, host, operation, nameSuffix string, cloudflareProxied bool, dedicatedGatewayName string) error {
	startTime := time.Now()

	if operation == "add" {
		var err error

		// For custom domains, use the dedicated custom domain gateway
		// This routes traffic through the LoadBalancer instead of Cloudflare Tunnel
		if record.IsCustomDomain && nameSuffix != "platform" {
			// Custom domain VirtualServices reference the dedicated gateway
			// The "platform" suffix is for platform subdomain fallbacks which still use the default gateway
			err = r.k8sClient.CreateCustomDomainVirtualService(ctx, record.Slug, record.TenantID, templateVSName, host, record.AdminHost, record.StorefrontHost, nameSuffix, dedicatedGatewayName)
			gatewayRef := dedicatedGatewayName
			if gatewayRef == "" {
				gatewayRef = r.config.Kubernetes.CustomDomainGateway
			}
			log.Printf("[Reconciler] Creating custom domain VirtualService for %s (host: %s, gateway: %s)", record.Slug, host, gatewayRef)
		} else {
			// Default domain or platform subdomain - use the default gateway
			// Pass cloudflareProxied to control whether DNS should be proxied through Cloudflare
			err = r.k8sClient.CreateTenantVirtualServiceWithSuffix(ctx, record.Slug, record.TenantID, templateVSName, host, record.AdminHost, record.StorefrontHost, nameSuffix, cloudflareProxied)
		}

		if err != nil {
			r.logActivity(ctx, record.ID, fmt.Sprintf("create_%s_%s", templateVSName, nameSuffix), "VirtualService", "", false, err.Error(), time.Since(startTime))
			return err
		}

		vsLocation, _ := r.k8sClient.FindVirtualServiceByName(ctx, templateVSName)
		ns := ""
		if vsLocation != nil {
			ns = vsLocation.Namespace
		}

		// Only update provisioning state for non-suffixed VS (primary VS)
		if nameSuffix == "" {
			switch templateVSName {
			case r.config.Kubernetes.AdminVSName:
				r.repo.UpdateProvisioningState(ctx, record.Slug, "admin_vs_patched", true, ns)
			case r.config.Kubernetes.StorefrontVSName:
				r.repo.UpdateProvisioningState(ctx, record.Slug, "storefront_vs_patched", true, ns)
			case r.config.Kubernetes.APIVSName:
				r.repo.UpdateProvisioningState(ctx, record.Slug, "api_vs_patched", true, ns)
			}
		}

		logSuffix := ""
		if nameSuffix != "" {
			logSuffix = "_" + nameSuffix
		}
		r.logActivity(ctx, record.ID, fmt.Sprintf("create_%s%s", templateVSName, logSuffix), "VirtualService", ns, true, "", time.Since(startTime))
	} else if operation == "remove" {
		// Delete the tenant's VirtualService
		err := r.k8sClient.DeleteTenantVirtualService(ctx, record.Slug, templateVSName)
		if err != nil {
			r.logActivity(ctx, record.ID, fmt.Sprintf("delete_%s", templateVSName), "VirtualService", "", false, err.Error(), time.Since(startTime))
			return err
		}
		r.logActivity(ctx, record.ID, fmt.Sprintf("delete_%s", templateVSName), "VirtualService", "", true, "", time.Since(startTime))
	}

	return nil
}

// reconcileAuthorizationPolicy adds or removes custom domain hosts from the shared AuthorizationPolicy
// This is necessary for custom domains to pass RBAC checks on the custom-ingressgateway
func (r *TenantReconciler) reconcileAuthorizationPolicy(ctx context.Context, record *models.TenantHostRecord, operation string) error {
	if !record.IsCustomDomain {
		// Only custom domains need AuthorizationPolicy updates
		return nil
	}

	// Collect all hosts that need to be added/removed
	hosts := make([]string, 0)

	// Add storefront host
	if record.StorefrontHost != "" {
		hosts = append(hosts, record.StorefrontHost)
	}

	// Add admin host if different from storefront
	if record.AdminHost != "" && record.AdminHost != record.StorefrontHost {
		hosts = append(hosts, record.AdminHost)
	}

	// Add www subdomain if configured
	if record.StorefrontWwwHost != "" {
		hosts = append(hosts, record.StorefrontWwwHost)
	}

	// Add API host if configured
	if record.APIHost != "" {
		hosts = append(hosts, record.APIHost)
	}

	if len(hosts) == 0 {
		log.Printf("[Reconciler] No hosts to %s for AuthorizationPolicy for tenant %s", operation, record.Slug)
		return nil
	}

	startTime := time.Now()
	var err error

	if operation == "add" {
		err = r.k8sClient.AddHostsToSharedAuthPolicy(ctx, hosts)
	} else if operation == "remove" {
		err = r.k8sClient.RemoveHostsFromSharedAuthPolicy(ctx, hosts)
	}

	if err != nil {
		r.logActivity(ctx, record.ID, fmt.Sprintf("%s_auth_policy", operation), "AuthorizationPolicy", r.config.Kubernetes.CustomDomainGatewayNS, false, err.Error(), time.Since(startTime))
		return err
	}

	r.logActivity(ctx, record.ID, fmt.Sprintf("%s_auth_policy", operation), "AuthorizationPolicy", r.config.Kubernetes.CustomDomainGatewayNS, true, "", time.Since(startTime))
	log.Printf("[Reconciler] %sd hosts in AuthorizationPolicy for tenant %s: %v", operation, record.Slug, hosts)
	return nil
}

// updateConditions logs conditions (could be extended to store in DB)
func (r *TenantReconciler) updateConditions(ctx context.Context, record *models.TenantHostRecord, conditions []Condition) {
	for _, c := range conditions {
		log.Printf("[Reconciler] Condition %s: %s=%s reason=%s", record.Slug, c.Type, c.Status, c.Reason)
	}
}

// logActivity logs a provisioning activity
func (r *TenantReconciler) logActivity(ctx context.Context, tenantHostID uuid.UUID, action, resource, namespace string, success bool, errorMsg string, duration time.Duration) {
	activityLog := &models.ProvisioningActivityLog{
		TenantHostID: tenantHostID,
		Action:       action,
		Resource:     resource,
		Namespace:    namespace,
		Success:      success,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	}
	if err := r.repo.LogActivity(ctx, activityLog); err != nil {
		log.Printf("[Reconciler] Failed to log activity: %v", err)
	}
}

// GetMetrics returns a snapshot of current metrics
func (r *TenantReconciler) GetMetrics() MetricsSnapshot {
	r.metrics.mu.RLock()
	defer r.metrics.mu.RUnlock()
	return MetricsSnapshot{
		ReconcileTotal:      r.metrics.ReconcileTotal,
		ReconcileSuccessful: r.metrics.ReconcileSuccessful,
		ReconcileFailed:     r.metrics.ReconcileFailed,
		ReconcileDuration:   r.metrics.ReconcileDuration,
		LastReconcileTime:   r.metrics.LastReconcileTime,
		CurrentQueueDepth:   r.metrics.CurrentQueueDepth,
		RetryCount:          r.metrics.RetryCount,
	}
}
