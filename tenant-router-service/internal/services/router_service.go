package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"

	"tenant-router-service/internal/config"
	"tenant-router-service/internal/k8s"
	"tenant-router-service/internal/models"
	"tenant-router-service/internal/repository"
)

// RouterService handles tenant routing provisioning
type RouterService struct {
	k8sClient *k8s.Client
	repo      repository.TenantHostRepository
	config    *config.Config
	hosts     map[string]*models.TenantHost // In-memory cache for quick lookups
	mu        sync.RWMutex
}

// NewRouterService creates a new router service
func NewRouterService(k8sClient *k8s.Client, repo repository.TenantHostRepository, cfg *config.Config) *RouterService {
	return &RouterService{
		k8sClient: k8sClient,
		repo:      repo,
		config:    cfg,
		hosts:     make(map[string]*models.TenantHost),
	}
}

// slugRegex validates that slug contains only lowercase alphanumeric and hyphens
var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// validateSlug checks if a slug is valid for use in hostnames
func validateSlug(slug string) error {
	if len(slug) < 2 || len(slug) > 63 {
		return fmt.Errorf("slug must be between 2 and 63 characters")
	}
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug must contain only lowercase letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	return nil
}

// ProvisionTenant provisions all routing resources for a new tenant
func (s *RouterService) ProvisionTenant(ctx context.Context, slug, tenantID string, event *models.TenantCreatedEvent) (*models.ProvisionResult, error) {
	log.Printf("[RouterService] Provisioning tenant: %s (ID: %s)", slug, tenantID)

	// Validate slug
	if err := validateSlug(slug); err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	domain := s.config.Domain.BaseDomain
	adminHost := fmt.Sprintf("%s-admin.%s", slug, domain)
	storefrontHost := fmt.Sprintf("%s.%s", slug, domain)
	certName := fmt.Sprintf("%s-tenant-tls", slug)

	// Check if already exists in database
	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing record: %w", err)
	}

	var record *models.TenantHostRecord
	if existing != nil {
		// Record exists - check if we need to resume provisioning
		if existing.Status == models.HostStatusProvisioned {
			log.Printf("[RouterService] Tenant %s already provisioned, skipping", slug)
			return &models.ProvisionResult{
				TenantID:       tenantID,
				Slug:           slug,
				AdminHost:      adminHost,
				StorefrontHost: storefrontHost,
				CertName:       certName,
				Success:        true,
			}, nil
		}
		record = existing
		record.Status = models.HostStatusPending
		if err := s.repo.Update(ctx, record); err != nil {
			log.Printf("[RouterService] Failed to update existing record: %v", err)
		}
	} else {
		// Create new record in database
		record = &models.TenantHostRecord{
			TenantID:       tenantID,
			Slug:           slug,
			AdminHost:      adminHost,
			StorefrontHost: storefrontHost,
			CertName:       certName,
			Status:         models.HostStatusPending,
		}
		// Add metadata from event if available
		if event != nil {
			record.Product = event.Product
			record.BusinessName = event.BusinessName
			record.Email = event.Email
		}
		if err := s.repo.Create(ctx, record); err != nil {
			return nil, fmt.Errorf("failed to create database record: %w", err)
		}
	}

	result := &models.ProvisionResult{
		TenantID:       tenantID,
		Slug:           slug,
		AdminHost:      adminHost,
		StorefrontHost: storefrontHost,
		CertName:       certName,
		Errors:         []string{},
		Success:        true,
	}

	// 1. Create Certificate
	if !record.CertificateCreated {
		log.Printf("[RouterService] Creating Certificate for %s", slug)
		startTime := time.Now()
		if err := s.k8sClient.CreateCertificate(ctx, slug, adminHost, storefrontHost); err != nil {
			log.Printf("[RouterService] Failed to create certificate: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("certificate: %v", err))
			result.Success = false
			s.logActivity(ctx, record.ID, "create_certificate", "Certificate", s.config.Kubernetes.Namespace, false, err.Error(), time.Since(startTime))
		} else {
			s.repo.UpdateProvisioningState(ctx, slug, "certificate_created", true, s.config.Kubernetes.Namespace)
			s.logActivity(ctx, record.ID, "create_certificate", "Certificate", s.config.Kubernetes.Namespace, true, "", time.Since(startTime))
		}
	}

	// 2. Patch Gateway with new server entries
	// Skip if using wildcard certificate (default behavior)
	if !record.GatewayPatched {
		if s.config.Kubernetes.SkipGatewayPatch {
			// Mark as patched since wildcard cert handles this
			log.Printf("[RouterService] Skipping gateway patch for %s (using wildcard cert %s)",
				slug, s.config.Kubernetes.WildcardCertName)
			s.repo.UpdateProvisioningState(ctx, slug, "gateway_patched", true, "wildcard")
			s.logActivity(ctx, record.ID, "skip_gateway", "Gateway", "wildcard", true, "Using wildcard certificate", 0)
		} else {
			log.Printf("[RouterService] Patching Gateway for %s", slug)
			startTime := time.Now()
			gwNamespace, err := s.k8sClient.PatchGatewayServer(ctx, slug, adminHost, storefrontHost, "add")
			if err != nil {
				log.Printf("[RouterService] Failed to patch gateway: %v", err)
				result.Errors = append(result.Errors, fmt.Sprintf("gateway: %v", err))
				result.Success = false
				s.logActivity(ctx, record.ID, "patch_gateway", "Gateway", gwNamespace, false, err.Error(), time.Since(startTime))
			} else {
				s.repo.UpdateProvisioningState(ctx, slug, "gateway_patched", true, gwNamespace)
				s.logActivity(ctx, record.ID, "patch_gateway", "Gateway", gwNamespace, true, "", time.Since(startTime))
			}
		}
	}

	// 3. Patch admin VirtualService
	if !record.AdminVSPatched {
		log.Printf("[RouterService] Patching admin VirtualService for %s", slug)
		startTime := time.Now()
		if err := s.k8sClient.PatchVirtualServiceHosts(ctx, s.config.Kubernetes.AdminVSName, adminHost, "add"); err != nil {
			log.Printf("[RouterService] Failed to patch admin VS: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("admin-vs: %v", err))
			result.Success = false
			s.logActivity(ctx, record.ID, "patch_admin_vs", "VirtualService", "", false, err.Error(), time.Since(startTime))
		} else {
			// Get the namespace where VS was found
			vsLocation, _ := s.k8sClient.FindVirtualServiceByName(ctx, s.config.Kubernetes.AdminVSName)
			ns := ""
			if vsLocation != nil {
				ns = vsLocation.Namespace
			}
			s.repo.UpdateProvisioningState(ctx, slug, "admin_vs_patched", true, ns)
			s.logActivity(ctx, record.ID, "patch_admin_vs", "VirtualService", ns, true, "", time.Since(startTime))
		}
	}

	// 4. Patch storefront VirtualService
	if !record.StorefrontVSPatched {
		log.Printf("[RouterService] Patching storefront VirtualService for %s", slug)
		startTime := time.Now()
		if err := s.k8sClient.PatchVirtualServiceHosts(ctx, s.config.Kubernetes.StorefrontVSName, storefrontHost, "add"); err != nil {
			log.Printf("[RouterService] Failed to patch storefront VS: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("storefront-vs: %v", err))
			result.Success = false
			s.logActivity(ctx, record.ID, "patch_storefront_vs", "VirtualService", "", false, err.Error(), time.Since(startTime))
		} else {
			vsLocation, _ := s.k8sClient.FindVirtualServiceByName(ctx, s.config.Kubernetes.StorefrontVSName)
			ns := ""
			if vsLocation != nil {
				ns = vsLocation.Namespace
			}
			s.repo.UpdateProvisioningState(ctx, slug, "storefront_vs_patched", true, ns)
			s.logActivity(ctx, record.ID, "patch_storefront_vs", "VirtualService", ns, true, "", time.Since(startTime))
		}
	}

	// Update final status in database
	if result.Success {
		if err := s.repo.MarkProvisioned(ctx, slug); err != nil {
			log.Printf("[RouterService] Failed to mark as provisioned: %v", err)
		}
		log.Printf("[RouterService] Successfully provisioned tenant %s", slug)
	} else {
		errMsg := fmt.Sprintf("%v", result.Errors)
		if err := s.repo.MarkFailed(ctx, slug, errMsg); err != nil {
			log.Printf("[RouterService] Failed to mark as failed: %v", err)
		}
		log.Printf("[RouterService] Provisioned tenant %s with errors: %v", slug, result.Errors)
	}

	// Update in-memory cache
	s.mu.Lock()
	s.hosts[slug] = &models.TenantHost{
		TenantID:       tenantID,
		Slug:           slug,
		AdminHost:      adminHost,
		StorefrontHost: storefrontHost,
		CertName:       certName,
		Status:         string(record.Status),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	s.mu.Unlock()

	return result, nil
}

// DeprovisionTenant removes all routing resources for a tenant
func (s *RouterService) DeprovisionTenant(ctx context.Context, slug string) error {
	log.Printf("[RouterService] Deprovisioning tenant: %s", slug)

	// Get record from database
	record, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("failed to get record: %w", err)
	}
	if record == nil {
		log.Printf("[RouterService] No record found for slug %s, nothing to deprovision", slug)
		return nil
	}

	// Mark as deleting
	if err := s.repo.UpdateStatus(ctx, slug, models.HostStatusDeleting); err != nil {
		log.Printf("[RouterService] Failed to update status to deleting: %v", err)
	}

	domain := s.config.Domain.BaseDomain
	adminHost := fmt.Sprintf("%s-admin.%s", slug, domain)
	storefrontHost := fmt.Sprintf("%s.%s", slug, domain)

	var errors []string

	// 1. Remove from admin VirtualService
	startTime := time.Now()
	if err := s.k8sClient.PatchVirtualServiceHosts(ctx, s.config.Kubernetes.AdminVSName, adminHost, "remove"); err != nil {
		log.Printf("[RouterService] Failed to remove from admin VS: %v", err)
		errors = append(errors, err.Error())
		s.logActivity(ctx, record.ID, "remove_admin_vs", "VirtualService", "", false, err.Error(), time.Since(startTime))
	} else {
		s.logActivity(ctx, record.ID, "remove_admin_vs", "VirtualService", "", true, "", time.Since(startTime))
	}

	// 2. Remove from storefront VirtualService
	startTime = time.Now()
	if err := s.k8sClient.PatchVirtualServiceHosts(ctx, s.config.Kubernetes.StorefrontVSName, storefrontHost, "remove"); err != nil {
		log.Printf("[RouterService] Failed to remove from storefront VS: %v", err)
		errors = append(errors, err.Error())
		s.logActivity(ctx, record.ID, "remove_storefront_vs", "VirtualService", "", false, err.Error(), time.Since(startTime))
	} else {
		s.logActivity(ctx, record.ID, "remove_storefront_vs", "VirtualService", "", true, "", time.Since(startTime))
	}

	// 3. Remove from Gateway
	startTime = time.Now()
	if _, err := s.k8sClient.PatchGatewayServer(ctx, slug, adminHost, storefrontHost, "remove"); err != nil {
		log.Printf("[RouterService] Failed to remove from gateway: %v", err)
		errors = append(errors, err.Error())
		s.logActivity(ctx, record.ID, "remove_gateway", "Gateway", "", false, err.Error(), time.Since(startTime))
	} else {
		s.logActivity(ctx, record.ID, "remove_gateway", "Gateway", "", true, "", time.Since(startTime))
	}

	// 4. Delete Certificate
	startTime = time.Now()
	if err := s.k8sClient.DeleteCertificate(ctx, slug); err != nil {
		log.Printf("[RouterService] Failed to delete certificate: %v", err)
		errors = append(errors, err.Error())
		s.logActivity(ctx, record.ID, "delete_certificate", "Certificate", "", false, err.Error(), time.Since(startTime))
	} else {
		s.logActivity(ctx, record.ID, "delete_certificate", "Certificate", "", true, "", time.Since(startTime))
	}

	// Soft delete from database
	if err := s.repo.Delete(ctx, slug); err != nil {
		log.Printf("[RouterService] Failed to delete database record: %v", err)
	}

	// Remove from in-memory cache
	s.mu.Lock()
	delete(s.hosts, slug)
	s.mu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("deprovisioning completed with errors: %v", errors)
	}

	log.Printf("[RouterService] Successfully deprovisioned tenant %s", slug)
	return nil
}

// SyncTenant forces a sync of a tenant's routing configuration
func (s *RouterService) SyncTenant(ctx context.Context, slug, tenantID string) (*models.ProvisionResult, error) {
	log.Printf("[RouterService] Syncing tenant: %s", slug)

	// Simply re-provision (idempotent operations)
	return s.ProvisionTenant(ctx, slug, tenantID, nil)
}

// GetTenantHost returns the host configuration for a tenant from database
func (s *RouterService) GetTenantHost(ctx context.Context, slug string) (*models.TenantHostRecord, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// GetTenantHostFromCache returns the host configuration from in-memory cache (quick lookup)
func (s *RouterService) GetTenantHostFromCache(slug string) (*models.TenantHost, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	host, ok := s.hosts[slug]
	return host, ok
}

// ListTenantHosts returns all configured tenant hosts from database
func (s *RouterService) ListTenantHosts(ctx context.Context, status *models.HostStatus, limit, offset int) ([]models.TenantHostRecord, int64, error) {
	return s.repo.List(ctx, status, limit, offset)
}

// AddTenantHost manually adds a tenant host (for admin API)
func (s *RouterService) AddTenantHost(ctx context.Context, slug, tenantID string) (*models.ProvisionResult, error) {
	return s.ProvisionTenant(ctx, slug, tenantID, nil)
}

// RemoveTenantHost manually removes a tenant host (for admin API)
func (s *RouterService) RemoveTenantHost(ctx context.Context, slug string) error {
	return s.DeprovisionTenant(ctx, slug)
}

// GetCertificateStatus returns the status of a tenant's certificate
func (s *RouterService) GetCertificateStatus(ctx context.Context, slug string) (string, error) {
	return s.k8sClient.GetCertificateStatus(ctx, slug)
}

// GetStats returns statistics about tenant hosts
func (s *RouterService) GetStats(ctx context.Context) (*repository.HostStats, error) {
	return s.repo.GetStats(ctx)
}

// LoadCache loads all active tenant hosts from database into memory cache
func (s *RouterService) LoadCache(ctx context.Context) error {
	log.Println("[RouterService] Loading tenant hosts into cache...")

	status := models.HostStatusProvisioned
	records, _, err := s.repo.List(ctx, &status, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to load hosts from database: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range records {
		s.hosts[record.Slug] = &models.TenantHost{
			TenantID:       record.TenantID,
			Slug:           record.Slug,
			AdminHost:      record.AdminHost,
			StorefrontHost: record.StorefrontHost,
			CertName:       record.CertName,
			Status:         string(record.Status),
			CreatedAt:      record.CreatedAt,
			UpdatedAt:      record.UpdatedAt,
		}
	}

	log.Printf("[RouterService] Loaded %d tenant hosts into cache", len(records))
	return nil
}

// SyncVirtualServiceRoutes syncs a tenant's VirtualService routes with the template
// This is useful when the template is updated with new routes (like swagger-docs)
func (s *RouterService) SyncVirtualServiceRoutes(ctx context.Context, slug string, vsType string) error {
	log.Printf("[RouterService] Syncing VirtualService routes for tenant %s (type: %s)", slug, vsType)

	// Get tenant record
	record, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("failed to get tenant record: %w", err)
	}
	if record == nil {
		return fmt.Errorf("tenant %s not found", slug)
	}

	domain := s.config.Domain.BaseDomain
	var templateVSName, tenantHost string

	switch vsType {
	case "admin":
		templateVSName = s.config.Kubernetes.AdminVSName
		tenantHost = record.AdminHost
	case "storefront":
		templateVSName = s.config.Kubernetes.StorefrontVSName
		tenantHost = record.StorefrontHost
	case "api":
		templateVSName = s.config.Kubernetes.APIVSName
		tenantHost = fmt.Sprintf("%s-api.%s", slug, domain)
	default:
		return fmt.Errorf("invalid VS type: %s (must be admin, storefront, or api)", vsType)
	}

	// Update the VirtualService routes
	err = s.k8sClient.UpdateTenantVirtualService(ctx, slug, templateVSName, tenantHost, record.AdminHost, record.StorefrontHost)
	if err != nil {
		log.Printf("[RouterService] Failed to sync VS routes for %s: %v", slug, err)
		return fmt.Errorf("failed to sync VirtualService routes: %w", err)
	}

	log.Printf("[RouterService] Successfully synced %s VirtualService routes for tenant %s", vsType, slug)
	return nil
}

// SyncAllVirtualServiceRoutes syncs all tenant VirtualServices for a given template
// Returns the number of successfully synced VirtualServices
func (s *RouterService) SyncAllVirtualServiceRoutes(ctx context.Context, vsType string) (int, error) {
	log.Printf("[RouterService] Syncing all %s VirtualService routes", vsType)

	var templateVSName string
	switch vsType {
	case "admin":
		templateVSName = s.config.Kubernetes.AdminVSName
	case "storefront":
		templateVSName = s.config.Kubernetes.StorefrontVSName
	case "api":
		templateVSName = s.config.Kubernetes.APIVSName
	default:
		return 0, fmt.Errorf("invalid VS type: %s (must be admin, storefront, or api)", vsType)
	}

	synced, err := s.k8sClient.SyncTenantVirtualServices(ctx, templateVSName)
	if err != nil {
		return 0, fmt.Errorf("failed to sync VirtualServices: %w", err)
	}

	log.Printf("[RouterService] Synced %d %s VirtualServices", synced, vsType)
	return synced, nil
}

// RetryFailedProvisions attempts to retry failed provisioning
func (s *RouterService) RetryFailedProvisions(ctx context.Context, maxRetries int) (int, error) {
	records, err := s.repo.ListPendingRetry(ctx, maxRetries)
	if err != nil {
		return 0, fmt.Errorf("failed to get failed records: %w", err)
	}

	retried := 0
	for _, record := range records {
		log.Printf("[RouterService] Retrying provisioning for %s (attempt %d)", record.Slug, record.RetryCount+1)
		_, err := s.ProvisionTenant(ctx, record.Slug, record.TenantID, nil)
		if err != nil {
			log.Printf("[RouterService] Retry failed for %s: %v", record.Slug, err)
		} else {
			retried++
		}
	}

	return retried, nil
}

// logActivity logs a provisioning activity to the database
func (s *RouterService) logActivity(ctx context.Context, tenantHostID uuid.UUID, action, resource, namespace string, success bool, errorMsg string, duration time.Duration) {
	activityLog := &models.ProvisioningActivityLog{
		TenantHostID: tenantHostID,
		Action:       action,
		Resource:     resource,
		Namespace:    namespace,
		Success:      success,
		ErrorMessage: errorMsg,
		Duration:     duration.Milliseconds(),
	}

	if err := s.repo.LogActivity(ctx, activityLog); err != nil {
		log.Printf("[RouterService] Failed to log activity: %v", err)
	}
}
