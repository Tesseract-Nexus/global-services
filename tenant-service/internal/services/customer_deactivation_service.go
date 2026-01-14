package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/models"
	"tenant-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"gorm.io/gorm"
)

// CustomerDeactivationService handles customer self-service account deactivation
type CustomerDeactivationService struct {
	db                 *gorm.DB
	membershipRepo     *repository.MembershipRepository
	notificationClient *clients.NotificationClient
	keycloakClient     *auth.KeycloakAdminClient
	keycloakConfig     *KeycloakAuthConfig
}

// NewCustomerDeactivationService creates a new customer deactivation service
func NewCustomerDeactivationService(
	db *gorm.DB,
	membershipRepo *repository.MembershipRepository,
	notificationClient *clients.NotificationClient,
	keycloakClient *auth.KeycloakAdminClient,
	keycloakConfig *KeycloakAuthConfig,
) *CustomerDeactivationService {
	return &CustomerDeactivationService{
		db:                 db,
		membershipRepo:     membershipRepo,
		notificationClient: notificationClient,
		keycloakClient:     keycloakClient,
		keycloakConfig:     keycloakConfig,
	}
}

// DeactivateCustomerRequest represents a customer self-deactivation request
type DeactivateCustomerRequest struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Reason   string    `json:"reason"` // Optional reason (e.g., "not_using", "privacy_concerns", "other")
}

// DeactivateCustomerResponse represents the deactivation result
type DeactivateCustomerResponse struct {
	Success          bool      `json:"success"`
	DeactivatedAt    time.Time `json:"deactivated_at"`
	ScheduledPurgeAt time.Time `json:"scheduled_purge_at"`
	DaysUntilPurge   int       `json:"days_until_purge"`
	Message          string    `json:"message"`
}

// CheckDeactivatedResponse represents the result of checking deactivation status
type CheckDeactivatedResponse struct {
	IsDeactivated  bool       `json:"is_deactivated"`
	CanReactivate  bool       `json:"can_reactivate"`
	DaysUntilPurge int        `json:"days_until_purge,omitempty"`
	DeactivatedAt  *time.Time `json:"deactivated_at,omitempty"`
	PurgeDate      *time.Time `json:"purge_date,omitempty"`
}

// ReactivateCustomerRequest represents a reactivation request
type ReactivateCustomerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug"`
}

// ReactivateCustomerResponse represents the reactivation result
type ReactivateCustomerResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// DeactivateCustomer handles customer self-service account deactivation
func (s *CustomerDeactivationService) DeactivateCustomer(ctx context.Context, req *DeactivateCustomerRequest) (*DeactivateCustomerResponse, error) {
	// Start transaction
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get the user
	var user models.User
	if err := tx.Where("id = ?", req.UserID).First(&user).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Get the membership
	var membership models.UserTenantMembership
	if err := tx.Where("user_id = ? AND tenant_id = ?", req.UserID, req.TenantID).First(&membership).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("membership not found: %w", err)
	}

	// Check if already deactivated
	if !membership.IsActive {
		tx.Rollback()
		return &DeactivateCustomerResponse{
			Success: false,
			Message: "Account is already deactivated",
		}, nil
	}

	// Get tenant for email
	var tenant models.Tenant
	if err := tx.Where("id = ?", req.TenantID).First(&tenant).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Create membership data snapshot
	membershipJSON, _ := json.Marshal(membership)

	// Determine reason
	reason := req.Reason
	if reason == "" {
		reason = models.DeactivationReasonSelfService
	}

	now := time.Now()
	purgeDate := now.AddDate(0, 0, models.DataRetentionDays)

	// Create deactivation archive record
	deactivatedMembership := &models.DeactivatedMembership{
		OriginalMembershipID: membership.ID,
		UserID:               req.UserID,
		TenantID:             req.TenantID,
		Email:                user.Email,
		FirstName:            user.FirstName,
		LastName:             user.LastName,
		MembershipData:       models.MustNewJSONB(json.RawMessage(membershipJSON)),
		DeactivationReason:   reason,
		DeactivatedAt:        now,
		ScheduledPurgeAt:     purgeDate,
	}

	if err := tx.Create(deactivatedMembership).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create deactivation record: %w", err)
	}

	// Set membership to inactive
	if err := tx.Model(&membership).Update("is_active", false).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to deactivate membership: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Send goodbye email asynchronously
	if s.notificationClient != nil {
		go func() {
			sendCtx := context.Background()
			storeName := tenant.Name
			if storeName == "" {
				storeName = tenant.Slug
			}
			storefrontURL := fmt.Sprintf("https://%s.tesserix.app/login", tenant.Slug)

			if err := s.notificationClient.SendGoodbyeEmail(sendCtx, &clients.GoodbyeEmailData{
				Email:           user.Email,
				FirstName:       user.FirstName,
				StoreName:       storeName,
				DeactivatedAt:   now,
				ScheduledPurgeAt: purgeDate,
				ReactivationURL: storefrontURL,
			}); err != nil {
				log.Printf("[CustomerDeactivationService] Warning: Failed to send goodbye email: %v", err)
			} else {
				log.Printf("[CustomerDeactivationService] Goodbye email sent to %s", user.Email)
			}
		}()
	}

	log.Printf("[CustomerDeactivationService] Account deactivated for user %s from tenant %s", user.Email, tenant.Slug)

	return &DeactivateCustomerResponse{
		Success:          true,
		DeactivatedAt:    now,
		ScheduledPurgeAt: purgeDate,
		DaysUntilPurge:   models.DataRetentionDays,
		Message:          fmt.Sprintf("Your account has been deactivated. Your data will be retained for %d days.", models.DataRetentionDays),
	}, nil
}

// CheckDeactivatedAccount checks if an account is in deactivated state
func (s *CustomerDeactivationService) CheckDeactivatedAccount(ctx context.Context, email, tenantSlug string) (*CheckDeactivatedResponse, error) {
	// Get tenant by slug
	tenant, err := s.membershipRepo.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Check for deactivated membership
	var deactivated models.DeactivatedMembership
	err = s.db.WithContext(ctx).
		Where("email = ? AND tenant_id = ? AND is_purged = ?", email, tenant.ID, false).
		Where("reactivated_at IS NULL").
		Order("deactivated_at DESC").
		First(&deactivated).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not deactivated
			return &CheckDeactivatedResponse{
				IsDeactivated: false,
				CanReactivate: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to check deactivation status: %w", err)
	}

	return &CheckDeactivatedResponse{
		IsDeactivated:  true,
		CanReactivate:  deactivated.CanReactivate(),
		DaysUntilPurge: deactivated.DaysUntilPurge(),
		DeactivatedAt:  &deactivated.DeactivatedAt,
		PurgeDate:      &deactivated.ScheduledPurgeAt,
	}, nil
}

// ReactivateCustomer reactivates a deactivated account within the retention period
func (s *CustomerDeactivationService) ReactivateCustomer(ctx context.Context, req *ReactivateCustomerRequest) (*ReactivateCustomerResponse, error) {
	// Get tenant by slug
	tenant, err := s.membershipRepo.GetTenantBySlug(ctx, req.TenantSlug)
	if err != nil {
		return &ReactivateCustomerResponse{
			Success:      false,
			ErrorCode:    "TENANT_NOT_FOUND",
			ErrorMessage: "Store not found",
		}, nil
	}

	// Find deactivated membership
	var deactivated models.DeactivatedMembership
	err = s.db.WithContext(ctx).
		Where("email = ? AND tenant_id = ? AND is_purged = ?", req.Email, tenant.ID, false).
		Where("reactivated_at IS NULL").
		Order("deactivated_at DESC").
		First(&deactivated).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ReactivateCustomerResponse{
				Success:      false,
				ErrorCode:    "NOT_DEACTIVATED",
				ErrorMessage: "Account is not in deactivated state",
			}, nil
		}
		return nil, fmt.Errorf("failed to find deactivated account: %w", err)
	}

	// Check if can reactivate
	if !deactivated.CanReactivate() {
		return &ReactivateCustomerResponse{
			Success:      false,
			ErrorCode:    "CANNOT_REACTIVATE",
			ErrorMessage: "Account cannot be reactivated. The retention period has expired.",
		}, nil
	}

	// Verify password via Keycloak
	if s.keycloakClient != nil && s.keycloakConfig != nil {
		_, err := s.keycloakClient.GetTokenWithPassword(
			ctx,
			s.keycloakConfig.ClientID,
			s.keycloakConfig.ClientSecret,
			req.Email,
			req.Password,
		)
		if err != nil {
			log.Printf("[CustomerDeactivationService] Password verification failed for reactivation: %v", err)
			return &ReactivateCustomerResponse{
				Success:      false,
				ErrorCode:    "INVALID_PASSWORD",
				ErrorMessage: "Invalid password",
			}, nil
		}
	}

	// Start transaction for reactivation
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Reactivate membership
	if err := tx.Model(&models.UserTenantMembership{}).
		Where("id = ?", deactivated.OriginalMembershipID).
		Update("is_active", true).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to reactivate membership: %w", err)
	}

	// Update deactivation record
	now := time.Now()
	if err := tx.Model(&deactivated).Updates(map[string]interface{}{
		"reactivated_at":     now,
		"reactivation_count": gorm.Expr("reactivation_count + 1"),
	}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update deactivation record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[CustomerDeactivationService] Account reactivated for %s on tenant %s", req.Email, req.TenantSlug)

	return &ReactivateCustomerResponse{
		Success: true,
		Message: "Your account has been reactivated. Welcome back!",
	}, nil
}

// PurgeExpiredAccounts permanently deletes accounts past the retention period
// This is called by the background job
func (s *CustomerDeactivationService) PurgeExpiredAccounts(ctx context.Context) (int64, error) {
	now := time.Now()
	var purgedCount int64

	// Find accounts ready for purge
	var expiredAccounts []models.DeactivatedMembership
	err := s.db.WithContext(ctx).
		Where("scheduled_purge_at < ? AND is_purged = ? AND reactivated_at IS NULL", now, false).
		Limit(100). // Process in batches
		Find(&expiredAccounts).Error

	if err != nil {
		return 0, fmt.Errorf("failed to find expired accounts: %w", err)
	}

	for _, account := range expiredAccounts {
		if err := s.purgeAccount(ctx, &account); err != nil {
			log.Printf("[CustomerDeactivationService] Failed to purge account %s: %v", account.Email, err)
			continue
		}
		purgedCount++
	}

	if purgedCount > 0 {
		log.Printf("[CustomerDeactivationService] Purged %d expired accounts", purgedCount)
	}

	return purgedCount, nil
}

// purgeAccount permanently deletes a single deactivated account
func (s *CustomerDeactivationService) purgeAccount(ctx context.Context, account *models.DeactivatedMembership) error {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete the membership record
	if err := tx.Where("id = ?", account.OriginalMembershipID).Delete(&models.UserTenantMembership{}).Error; err != nil {
		tx.Rollback()
		log.Printf("[CustomerDeactivationService] Warning: Failed to delete membership %s: %v", account.OriginalMembershipID, err)
		// Continue anyway - membership might already be deleted
	}

	// Check if user has any other active memberships
	var otherMemberships int64
	tx.Model(&models.UserTenantMembership{}).
		Where("user_id = ? AND id != ?", account.UserID, account.OriginalMembershipID).
		Count(&otherMemberships)

	// If no other memberships, consider deleting the user from Keycloak
	// For now, we leave the Keycloak user intact as they may have accounts on other stores
	// that aren't tracked in our system

	// Mark as purged
	now := time.Now()
	if err := tx.Model(account).Updates(map[string]interface{}{
		"is_purged": true,
		"purged_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark as purged: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit purge: %w", err)
	}

	log.Printf("[CustomerDeactivationService] Purged account: %s (tenant: %s)", account.Email, account.TenantID)
	return nil
}
