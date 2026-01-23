package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tenant-service/internal/clients"
	"tenant-service/internal/models"
)

// TenantReconciliationService handles reconciliation of stuck tenants
// This fixes tenants that got stuck in "creating" status due to failures
// in vendor/storefront creation after the main transaction committed
type TenantReconciliationService struct {
	db           *gorm.DB
	vendorClient *clients.VendorClient
	natsClient   interface {
		PublishTenantCreated(ctx context.Context, event interface{}) error
	}

	// Configuration
	stuckThreshold  time.Duration // How long before a "creating" tenant is considered stuck
	maxRetries      int           // Maximum reconciliation attempts before marking as failed
	retryBackoff    time.Duration // Base backoff between retries
	maxStuckAge     time.Duration // Maximum age before we fail the tenant instead of retrying
}

// TenantReconciliationConfig holds configuration for the reconciliation service
type TenantReconciliationConfig struct {
	StuckThreshold time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
	MaxStuckAge    time.Duration
}

// DefaultReconciliationConfig returns sensible defaults
func DefaultReconciliationConfig() TenantReconciliationConfig {
	return TenantReconciliationConfig{
		StuckThreshold: 5 * time.Minute,   // Consider stuck after 5 minutes
		MaxRetries:     3,                 // Try 3 times before giving up
		RetryBackoff:   30 * time.Second,  // Wait 30 seconds between retries
		MaxStuckAge:    24 * time.Hour,    // After 24 hours, fail instead of retry
	}
}

// ReconciliationResult contains the result of a reconciliation run
type ReconciliationResult struct {
	TenantsChecked      int
	TenantsReconciled   int
	TenantsFailed       int
	TenantsAlreadyOK    int
	Errors              []string
}

// NewTenantReconciliationService creates a new reconciliation service
func NewTenantReconciliationService(db *gorm.DB, cfg TenantReconciliationConfig) *TenantReconciliationService {
	// Initialize vendor client
	vendorServiceURL := os.Getenv("VENDOR_SERVICE_URL")
	if vendorServiceURL == "" {
		vendorServiceURL = "http://vendor-service.devtest.svc.cluster.local:8085"
	}
	vendorClient := clients.NewVendorClient(vendorServiceURL)

	return &TenantReconciliationService{
		db:             db,
		vendorClient:   vendorClient,
		stuckThreshold: cfg.StuckThreshold,
		maxRetries:     cfg.MaxRetries,
		retryBackoff:   cfg.RetryBackoff,
		maxStuckAge:    cfg.MaxStuckAge,
	}
}

// SetNATSClient sets the NATS client for publishing events
func (s *TenantReconciliationService) SetNATSClient(nc interface {
	PublishTenantCreated(ctx context.Context, event interface{}) error
}) {
	s.natsClient = nc
}

// ReconcileStuckTenants finds and reconciles tenants stuck in "creating" status
func (s *TenantReconciliationService) ReconcileStuckTenants(ctx context.Context) (*ReconciliationResult, error) {
	result := &ReconciliationResult{}

	// Find tenants stuck in "creating" status for longer than threshold
	stuckSince := time.Now().Add(-s.stuckThreshold)

	var stuckTenants []models.Tenant
	err := s.db.WithContext(ctx).
		Where("status = ?", "creating").
		Where("created_at < ?", stuckSince).
		Find(&stuckTenants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query stuck tenants: %w", err)
	}

	result.TenantsChecked = len(stuckTenants)
	if len(stuckTenants) == 0 {
		log.Println("[TenantReconciliation] No stuck tenants found")
		return result, nil
	}

	log.Printf("[TenantReconciliation] Found %d stuck tenants to reconcile", len(stuckTenants))

	for _, tenant := range stuckTenants {
		tenantAge := time.Since(tenant.CreatedAt)

		// If tenant is too old, fail it instead of retrying
		if tenantAge > s.maxStuckAge {
			log.Printf("[TenantReconciliation] Tenant %s is too old (%v), marking as failed", tenant.ID, tenantAge)
			if err := s.failTenant(ctx, &tenant, "tenant stuck in creating state for too long"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to fail tenant %s: %v", tenant.ID, err))
			} else {
				result.TenantsFailed++
			}
			continue
		}

		// Try to reconcile the tenant
		if err := s.reconcileTenant(ctx, &tenant); err != nil {
			log.Printf("[TenantReconciliation] Failed to reconcile tenant %s: %v", tenant.ID, err)
			result.Errors = append(result.Errors, fmt.Sprintf("tenant %s: %v", tenant.ID, err))

			// Check if we should fail the tenant after max retries
			retryCount := s.getRetryCount(&tenant)
			if retryCount >= s.maxRetries {
				log.Printf("[TenantReconciliation] Tenant %s has exceeded max retries (%d), marking as failed", tenant.ID, s.maxRetries)
				if failErr := s.failTenant(ctx, &tenant, fmt.Sprintf("reconciliation failed after %d retries: %v", retryCount, err)); failErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to fail tenant %s: %v", tenant.ID, failErr))
				} else {
					result.TenantsFailed++
				}
			} else {
				// Increment retry count
				s.incrementRetryCount(ctx, &tenant)
			}
		} else {
			result.TenantsReconciled++
		}
	}

	log.Printf("[TenantReconciliation] Completed: checked=%d, reconciled=%d, failed=%d, errors=%d",
		result.TenantsChecked, result.TenantsReconciled, result.TenantsFailed, len(result.Errors))

	return result, nil
}

// reconcileTenant attempts to complete the onboarding for a stuck tenant
func (s *TenantReconciliationService) reconcileTenant(ctx context.Context, tenant *models.Tenant) error {
	log.Printf("[TenantReconciliation] Reconciling tenant %s (slug: %s)", tenant.ID, tenant.Slug)

	// Get the associated onboarding session
	var session models.OnboardingSession
	err := s.db.WithContext(ctx).
		Preload("BusinessInformation").
		Preload("ContactInformation").
		Where("tenant_id = ?", tenant.ID).
		First(&session).Error
	if err != nil {
		return fmt.Errorf("failed to get onboarding session: %w", err)
	}

	// Find primary contact from ContactInformation array
	var primaryContact *models.ContactInformation
	for i := range session.ContactInformation {
		if session.ContactInformation[i].IsPrimaryContact {
			primaryContact = &session.ContactInformation[i]
			break
		}
	}
	// Fall back to first contact if no primary found
	if primaryContact == nil && len(session.ContactInformation) > 0 {
		primaryContact = &session.ContactInformation[0]
	}

	// Check if vendor exists for this tenant
	vendorExists, existingVendor, err := s.checkVendorExists(ctx, tenant.ID)
	if err != nil {
		log.Printf("[TenantReconciliation] Warning: Error checking vendor existence: %v", err)
		vendorExists = false
	}

	var vendorID uuid.UUID
	if vendorExists && existingVendor != nil {
		log.Printf("[TenantReconciliation] Vendor already exists for tenant %s: %s", tenant.ID, existingVendor.ID)
		vendorID = existingVendor.ID
	} else {
		// Create vendor
		log.Printf("[TenantReconciliation] Creating vendor for tenant %s", tenant.ID)

		businessName := tenant.DisplayName
		if session.BusinessInformation != nil && session.BusinessInformation.BusinessName != "" {
			businessName = session.BusinessInformation.BusinessName
		}

		contactName := "Owner"
		if primaryContact != nil {
			contactName = fmt.Sprintf("%s %s", primaryContact.FirstName, primaryContact.LastName)
		}

		contactEmail := ""
		if primaryContact != nil {
			contactEmail = primaryContact.Email
		}

		vendorData, err := s.vendorClient.CreateVendorForTenant(ctx, tenant.ID, businessName, contactName, contactEmail)
		if err != nil {
			return fmt.Errorf("failed to create vendor: %w", err)
		}
		vendorID = vendorData.ID
		log.Printf("[TenantReconciliation] Created vendor %s for tenant %s", vendorID, tenant.ID)
	}

	// Check if storefront exists
	storefrontExists, err := s.checkStorefrontExists(ctx, tenant.ID, vendorID)
	if err != nil {
		log.Printf("[TenantReconciliation] Warning: Error checking storefront existence: %v", err)
		storefrontExists = false
	}

	if !storefrontExists {
		// Create storefront
		log.Printf("[TenantReconciliation] Creating storefront for vendor %s", vendorID)

		storefrontSlug := tenant.Slug
		if session.BusinessInformation != nil && session.BusinessInformation.StorefrontSlug != "" {
			storefrontSlug = session.BusinessInformation.StorefrontSlug
		}

		storefrontName := tenant.DisplayName + " Store"

		_, err := s.vendorClient.CreateStorefront(ctx, tenant.ID, vendorID, storefrontName, storefrontSlug, true)
		if err != nil {
			return fmt.Errorf("failed to create storefront: %w", err)
		}
		log.Printf("[TenantReconciliation] Created storefront for vendor %s", vendorID)
	} else {
		log.Printf("[TenantReconciliation] Storefront already exists for vendor %s", vendorID)
	}

	// Activate tenant
	if err := s.db.WithContext(ctx).Model(tenant).Update("status", "active").Error; err != nil {
		return fmt.Errorf("failed to activate tenant: %w", err)
	}

	// Update session status to completed if not already
	if err := s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ? AND status != ?", session.ID, "completed").
		Update("status", "completed").Error; err != nil {
		log.Printf("[TenantReconciliation] Warning: Failed to update session status: %v", err)
	}

	log.Printf("[TenantReconciliation] Successfully reconciled tenant %s (now active)", tenant.ID)
	return nil
}

// checkVendorExists checks if a vendor already exists for the tenant
func (s *TenantReconciliationService) checkVendorExists(ctx context.Context, tenantID uuid.UUID) (bool, *clients.VendorData, error) {
	vendors, err := s.vendorClient.GetVendorsForTenant(ctx, tenantID)
	if err != nil {
		return false, nil, err
	}
	if len(vendors) > 0 {
		return true, &vendors[0], nil
	}
	return false, nil, nil
}

// checkStorefrontExists checks if a storefront already exists for the vendor
func (s *TenantReconciliationService) checkStorefrontExists(ctx context.Context, tenantID, vendorID uuid.UUID) (bool, error) {
	storefronts, err := s.vendorClient.GetStorefrontsForVendor(ctx, tenantID, vendorID)
	if err != nil {
		return false, err
	}
	return len(storefronts) > 0, nil
}

// failTenant marks a tenant as failed/inactive
func (s *TenantReconciliationService) failTenant(ctx context.Context, tenant *models.Tenant, reason string) error {
	log.Printf("[TenantReconciliation] Failing tenant %s: %s", tenant.ID, reason)

	// Update tenant status to inactive
	if err := s.db.WithContext(ctx).Model(tenant).Update("status", "inactive").Error; err != nil {
		return fmt.Errorf("failed to update tenant status: %w", err)
	}

	// Update associated session to failed
	if err := s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("tenant_id = ?", tenant.ID).
		Update("status", "failed").Error; err != nil {
		log.Printf("[TenantReconciliation] Warning: Failed to update session status: %v", err)
	}

	// TODO: Send notification to user about failed onboarding
	// TODO: Clean up any partial resources (Keycloak redirect URIs, etc.)

	return nil
}

// getRetryCount gets the current retry count from tenant metadata
func (s *TenantReconciliationService) getRetryCount(tenant *models.Tenant) int {
	// For now, estimate based on time since creation
	// A proper implementation would store this in a metadata field
	age := time.Since(tenant.CreatedAt)
	estimatedRetries := int(age / (s.retryBackoff * 2)) // Rough estimate
	return estimatedRetries
}

// incrementRetryCount increments the retry count in tenant metadata
func (s *TenantReconciliationService) incrementRetryCount(ctx context.Context, tenant *models.Tenant) {
	// Update the updated_at timestamp to track retry attempts
	s.db.WithContext(ctx).Model(tenant).Update("updated_at", time.Now())
}
