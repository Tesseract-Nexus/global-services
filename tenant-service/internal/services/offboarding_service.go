package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	natsClient "github.com/tesseract-hub/domains/common/services/tenant-service/internal/nats"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"gorm.io/gorm"
)

// OffboardingService handles tenant deletion and offboarding logic
type OffboardingService struct {
	db             *gorm.DB
	membershipSvc  *MembershipService
	natsClient     *natsClient.Client
	keycloakClient *auth.KeycloakAdminClient
}

// NewOffboardingService creates a new offboarding service
func NewOffboardingService(
	db *gorm.DB,
	membershipSvc *MembershipService,
	natsClient *natsClient.Client,
	keycloakClient *auth.KeycloakAdminClient,
) *OffboardingService {
	return &OffboardingService{
		db:             db,
		membershipSvc:  membershipSvc,
		natsClient:     natsClient,
		keycloakClient: keycloakClient,
	}
}

// DeleteTenantRequest represents the request to delete a tenant
type DeleteTenantRequest struct {
	TenantID         uuid.UUID
	UserID           uuid.UUID
	ConfirmationText string
	Reason           string
}

// DeleteTenantResponse represents the response after deleting a tenant
type DeleteTenantResponse struct {
	Message          string    `json:"message"`
	TenantID         string    `json:"tenant_id"`
	ArchivedRecordID string    `json:"archived_record_id"`
	ArchivedAt       time.Time `json:"archived_at"`
}

// DeleteTenant deletes a tenant and archives all data for audit purposes
func (s *OffboardingService) DeleteTenant(ctx context.Context, req *DeleteTenantRequest) (*DeleteTenantResponse, error) {
	// 1. Get the tenant with memberships
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).
		Preload("Memberships").
		First(&tenant, "id = ?", req.TenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// 2. Verify the user is the tenant owner
	isOwner := false
	var ownerEmail string
	for _, membership := range tenant.Memberships {
		if membership.UserID == req.UserID && membership.Role == models.MembershipRoleOwner {
			isOwner = true
			// Get owner email from tenant_users
			var user models.User
			if err := s.db.WithContext(ctx).First(&user, "id = ?", req.UserID).Error; err == nil {
				ownerEmail = user.Email
			}
			break
		}
	}

	if !isOwner {
		return nil, fmt.Errorf("only the tenant owner can delete the tenant")
	}

	// 3. Verify confirmation text
	expectedConfirmation := fmt.Sprintf("DELETE %s", tenant.Slug)
	if req.ConfirmationText != expectedConfirmation {
		return nil, fmt.Errorf("invalid confirmation text: expected '%s'", expectedConfirmation)
	}

	// 4. Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 5. Create JSON snapshot of tenant data
	tenantJSON, err := json.Marshal(tenant)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to serialize tenant data: %w", err)
	}

	// 6. Create JSON snapshot of memberships
	membershipsJSON, err := json.Marshal(tenant.Memberships)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to serialize memberships data: %w", err)
	}

	// 6b. Fetch and archive vendors for this tenant
	var vendors []map[string]interface{}
	if err := tx.WithContext(ctx).
		Table("vendors").
		Where("tenant_id = ?", req.TenantID.String()).
		Find(&vendors).Error; err != nil {
		log.Printf("[OffboardingService] Warning: Failed to fetch vendors: %v", err)
		vendors = []map[string]interface{}{}
	}
	vendorsJSON, _ := json.Marshal(vendors)
	log.Printf("[OffboardingService] Found %d vendors to archive for tenant %s", len(vendors), tenant.ID)

	// 6c. Fetch and archive storefronts for this tenant's vendors
	var storefronts []map[string]interface{}
	vendorIDs := make([]string, 0)
	for _, v := range vendors {
		if id, ok := v["id"].(string); ok {
			vendorIDs = append(vendorIDs, id)
		}
	}
	if len(vendorIDs) > 0 {
		if err := tx.WithContext(ctx).
			Table("storefronts").
			Where("vendor_id IN ?", vendorIDs).
			Find(&storefronts).Error; err != nil {
			log.Printf("[OffboardingService] Warning: Failed to fetch storefronts: %v", err)
			storefronts = []map[string]interface{}{}
		}
	}
	storefrontsJSON, _ := json.Marshal(storefronts)
	log.Printf("[OffboardingService] Found %d storefronts to archive for tenant %s", len(storefronts), tenant.ID)

	// 7. Create archived tenant record
	deletedTenant := &models.DeletedTenant{
		OriginalTenantID: tenant.ID,
		Slug:             tenant.Slug,
		BusinessName:     tenant.Name,
		OwnerUserID:      req.UserID,
		OwnerEmail:       ownerEmail,
		TenantData:       models.JSONB(tenantJSON),
		MembershipsData:  models.JSONB(membershipsJSON),
		VendorsData:      models.JSONB(vendorsJSON),
		StorefrontsData:  models.JSONB(storefrontsJSON),
		DeletedByUserID:  req.UserID,
		DeletionReason:   req.Reason,
	}

	if err := tx.WithContext(ctx).Create(deletedTenant).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create archived tenant record: %w", err)
	}

	log.Printf("[OffboardingService] Archived tenant %s (ID: %s) to deleted_tenants", tenant.Slug, tenant.ID)

	// 8. Delete user_tenant_memberships
	if err := tx.WithContext(ctx).
		Where("tenant_id = ?", req.TenantID).
		Delete(&models.UserTenantMembership{}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete memberships: %w", err)
	}

	log.Printf("[OffboardingService] Deleted %d memberships for tenant %s", len(tenant.Memberships), tenant.ID)

	// 8b. Delete storefronts (must delete before vendors due to foreign key)
	if len(vendorIDs) > 0 {
		result := tx.WithContext(ctx).
			Exec("DELETE FROM storefronts WHERE vendor_id IN ?", vendorIDs)
		if result.Error != nil {
			log.Printf("[OffboardingService] Warning: Failed to delete storefronts: %v", result.Error)
		} else {
			log.Printf("[OffboardingService] Deleted %d storefronts for tenant %s", result.RowsAffected, tenant.ID)
		}
	}

	// 8c. Delete vendors
	result := tx.WithContext(ctx).
		Exec("DELETE FROM vendors WHERE tenant_id = ?", req.TenantID.String())
	if result.Error != nil {
		log.Printf("[OffboardingService] Warning: Failed to delete vendors: %v", result.Error)
	} else {
		log.Printf("[OffboardingService] Deleted %d vendors for tenant %s", result.RowsAffected, tenant.ID)
	}

	// 9. Release slug reservation
	if err := tx.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("tenant_id = ?", req.TenantID).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": time.Now(),
		}).Error; err != nil {
		log.Printf("[OffboardingService] Warning: Failed to release slug reservation: %v", err)
		// Don't fail the transaction for this
	}

	// 10. Hard delete the tenant (we have the archived copy)
	if err := tx.WithContext(ctx).
		Unscoped(). // Bypass soft delete
		Delete(&models.Tenant{}, "id = ?", req.TenantID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete tenant: %w", err)
	}

	log.Printf("[OffboardingService] Deleted tenant %s (ID: %s)", tenant.Slug, tenant.ID)

	// 11. Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Publish NATS event for K8s resource cleanup (synchronous with retry)
	if s.natsClient != nil {
		baseDomain := os.Getenv("BASE_DOMAIN")
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}

		event := &natsClient.TenantDeletedEvent{
			TenantID:       tenant.ID.String(),
			Slug:           tenant.Slug,
			AdminHost:      fmt.Sprintf("%s-admin.%s", tenant.Slug, baseDomain),
			StorefrontHost: fmt.Sprintf("%s.%s", tenant.Slug, baseDomain),
		}

		// Use a context with timeout for event publishing
		publishCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.natsClient.PublishTenantDeleted(publishCtx, event); err != nil {
			// Log error but don't fail - tenant is deleted, K8s resources will be cleaned up on reconciliation
			log.Printf("[OffboardingService] WARNING: Failed to publish tenant.deleted event: %v - K8s resources will be cleaned up on reconciliation", err)
		} else {
			log.Printf("[OffboardingService] Published tenant.deleted event for %s", tenant.Slug)
		}
	} else {
		log.Printf("[OffboardingService] WARNING: NATS client not initialized, tenant.deleted event not published")
	}

	// 13. Remove Keycloak redirect URIs for the deleted tenant
	if s.keycloakClient != nil {
		baseDomain := os.Getenv("BASE_DOMAIN")
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}

		// Build the redirect URIs to remove
		// NOTE: We only add admin URIs during onboarding (storefront is public and uses separate auth)
		// However, we also try to remove storefront URIs for backwards compatibility with tenants
		// that were created before this change.
		adminWildcard := fmt.Sprintf("https://%s-admin.%s/*", tenant.Slug, baseDomain)
		adminCallback := fmt.Sprintf("https://%s-admin.%s/auth/callback", tenant.Slug, baseDomain)
		// Legacy storefront URIs for backwards compatibility cleanup
		storefrontWildcard := fmt.Sprintf("https://%s.%s/*", tenant.Slug, baseDomain)
		storefrontCallback := fmt.Sprintf("https://%s.%s/auth/callback", tenant.Slug, baseDomain)

		redirectURIsToRemove := []string{
			adminWildcard,
			adminCallback,
			// Include legacy storefront URIs for cleanup of old tenants
			storefrontWildcard,
			storefrontCallback,
		}

		keycloakCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := s.keycloakClient.RemoveClientRedirectURIs(keycloakCtx, "marketplace-dashboard", redirectURIsToRemove); err != nil {
			log.Printf("[OffboardingService] WARNING: Failed to remove Keycloak redirect URIs for tenant %s: %v", tenant.Slug, err)
			// Don't fail the deletion - URIs can be cleaned up manually if needed
		} else {
			log.Printf("[OffboardingService] Removed Keycloak redirect URIs for tenant %s", tenant.Slug)
		}
	}

	return &DeleteTenantResponse{
		Message:          "Tenant deleted successfully",
		TenantID:         tenant.ID.String(),
		ArchivedRecordID: deletedTenant.ID.String(),
		ArchivedAt:       deletedTenant.DeletedAt,
	}, nil
}

// GetTenantForDeletion gets tenant information needed for the deletion UI
func (s *OffboardingService) GetTenantForDeletion(ctx context.Context, tenantID, userID uuid.UUID) (map[string]interface{}, error) {
	// Get the tenant
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).
		Preload("Memberships").
		First(&tenant, "id = ?", tenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Verify the user is the tenant owner
	isOwner := false
	for _, membership := range tenant.Memberships {
		if membership.UserID == userID && membership.Role == models.MembershipRoleOwner {
			isOwner = true
			break
		}
	}

	if !isOwner {
		return nil, fmt.Errorf("only the tenant owner can view deletion information")
	}

	// Count resources that will be affected
	memberCount := len(tenant.Memberships)

	return map[string]interface{}{
		"tenant_id":     tenant.ID.String(),
		"name":          tenant.Name,
		"slug":          tenant.Slug,
		"member_count":  memberCount,
		"created_at":    tenant.CreatedAt,
		"is_owner":      isOwner,
		"confirmation_required": fmt.Sprintf("DELETE %s", tenant.Slug),
	}, nil
}

// ListDeletedTenants lists all archived/deleted tenants (admin function)
func (s *OffboardingService) ListDeletedTenants(ctx context.Context, limit, offset int) ([]models.DeletedTenant, int64, error) {
	var deletedTenants []models.DeletedTenant
	var total int64

	// Count total
	if err := s.db.WithContext(ctx).Model(&models.DeletedTenant{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count deleted tenants: %w", err)
	}

	// Get paginated results
	if err := s.db.WithContext(ctx).
		Order("deleted_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deletedTenants).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list deleted tenants: %w", err)
	}

	return deletedTenants, total, nil
}

// GetDeletedTenant gets a specific archived tenant record
func (s *OffboardingService) GetDeletedTenant(ctx context.Context, id uuid.UUID) (*models.DeletedTenant, error) {
	var deletedTenant models.DeletedTenant
	if err := s.db.WithContext(ctx).First(&deletedTenant, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("deleted tenant record not found")
		}
		return nil, fmt.Errorf("failed to get deleted tenant: %w", err)
	}
	return &deletedTenant, nil
}
