package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"tenant-service/internal/repository"
)

// MembershipService handles user-tenant membership business logic
type MembershipService struct {
	membershipRepo *repository.MembershipRepository
}

// NewMembershipService creates a new membership service
func NewMembershipService(membershipRepo *repository.MembershipRepository) *MembershipService {
	return &MembershipService{
		membershipRepo: membershipRepo,
	}
}

// ============================================================================
// Tenant Context Operations
// ============================================================================

// UserTenantContext represents the context of a user accessing a tenant
type UserTenantContext struct {
	UserID     uuid.UUID `json:"user_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantSlug string    `json:"tenant_slug"`
	TenantName string    `json:"tenant_name"`
	Role       string    `json:"role"`
	IsOwner    bool      `json:"is_owner"`
	IsDefault  bool      `json:"is_default"`
}

// GetUserTenantContext retrieves the full context for a user accessing a tenant by slug
func (s *MembershipService) GetUserTenantContext(ctx context.Context, userID uuid.UUID, tenantSlug string) (*UserTenantContext, error) {
	// Check access and get tenant
	hasAccess, tenant, err := s.membershipRepo.HasAccessBySlug(ctx, userID, tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to check access: %w", err)
	}
	if !hasAccess {
		return nil, fmt.Errorf("access denied to tenant: %s", tenantSlug)
	}

	// Get membership details
	membership, err := s.membershipRepo.GetMembership(ctx, userID, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get membership: %w", err)
	}

	// Update last accessed
	if err := s.membershipRepo.UpdateLastAccessed(ctx, userID, tenant.ID); err != nil {
		log.Printf("Warning: failed to update last accessed: %v", err)
	}

	return &UserTenantContext{
		UserID:     userID,
		TenantID:   tenant.ID,
		TenantSlug: tenant.Slug,
		TenantName: tenant.DisplayName,
		Role:       membership.Role,
		IsOwner:    membership.Role == models.MembershipRoleOwner,
		IsDefault:  membership.IsDefault,
	}, nil
}

// VerifyTenantAccess checks if a user can access a tenant
func (s *MembershipService) VerifyTenantAccess(ctx context.Context, userID uuid.UUID, tenantSlug string) (bool, error) {
	hasAccess, _, err := s.membershipRepo.HasAccessBySlug(ctx, userID, tenantSlug)
	return hasAccess, err
}

// ============================================================================
// User's Tenants Operations
// ============================================================================

// UserTenantSummary represents a summary of a tenant for the user
type UserTenantSummary struct {
	TenantID       uuid.UUID  `json:"tenant_id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"display_name"`
	LogoURL        string     `json:"logo_url,omitempty"`
	Role           string     `json:"role"`
	IsDefault      bool       `json:"is_default"`
	IsOwner        bool       `json:"is_owner"`
	Status         string     `json:"status"`
	PrimaryColor   string     `json:"primary_color,omitempty"`
	BusinessModel  string     `json:"business_model,omitempty"` // ONLINE_STORE or MARKETPLACE
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
}

// ResolveUserID maps a Keycloak user ID to the local user ID
// This is needed because auth-bff uses Keycloak IDs but memberships use local user IDs
// Returns the original ID if no mapping is found (for backward compatibility)
func (s *MembershipService) ResolveUserID(ctx context.Context, keycloakID uuid.UUID) (uuid.UUID, error) {
	user, err := s.membershipRepo.GetUserByKeycloakID(ctx, keycloakID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to resolve user ID: %w", err)
	}
	if user != nil {
		// Found user by keycloak_id, return local user ID
		log.Printf("[MembershipService] Resolved Keycloak ID %s to local user ID %s", keycloakID, user.ID)
		return user.ID, nil
	}
	// No mapping found - use the provided ID directly (may be a new user or direct local ID)
	return keycloakID, nil
}

// GetUserTenants retrieves all tenants the user has access to
func (s *MembershipService) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]UserTenantSummary, error) {
	memberships, err := s.membershipRepo.GetUserMemberships(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memberships: %w", err)
	}

	summaries := make([]UserTenantSummary, 0, len(memberships))
	for _, m := range memberships {
		if m.Tenant == nil {
			continue
		}
		summaries = append(summaries, UserTenantSummary{
			TenantID:       m.TenantID,
			Slug:           m.Tenant.Slug,
			Name:           m.Tenant.Name,
			DisplayName:    m.Tenant.DisplayName,
			LogoURL:        m.Tenant.LogoURL,
			Role:           m.Role,
			IsDefault:      m.IsDefault,
			IsOwner:        m.Role == models.MembershipRoleOwner,
			Status:         m.Tenant.Status,
			PrimaryColor:   m.Tenant.PrimaryColor,
			BusinessModel:  m.Tenant.BusinessModel,
			LastAccessedAt: m.LastAccessedAt,
		})
	}

	return summaries, nil
}

// GetAllTenants retrieves all tenants in the system (for platform admins/super_admin)
// Returns tenants with "platform_admin" role to indicate admin access
func (s *MembershipService) GetAllTenants(ctx context.Context) ([]UserTenantSummary, error) {
	tenants, err := s.membershipRepo.GetAllTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tenants: %w", err)
	}

	summaries := make([]UserTenantSummary, 0, len(tenants))
	for _, t := range tenants {
		summaries = append(summaries, UserTenantSummary{
			TenantID:      t.ID,
			Slug:          t.Slug,
			Name:          t.Name,
			DisplayName:   t.DisplayName,
			LogoURL:       t.LogoURL,
			Role:          "platform_admin", // Indicate platform-level access
			IsDefault:     false,
			IsOwner:       false,
			Status:        t.Status,
			PrimaryColor:  t.PrimaryColor,
			BusinessModel: t.BusinessModel,
		})
	}

	return summaries, nil
}

// GetUserDefaultTenant retrieves the user's default tenant
func (s *MembershipService) GetUserDefaultTenant(ctx context.Context, userID uuid.UUID) (*UserTenantSummary, error) {
	membership, err := s.membershipRepo.GetUserDefaultMembership(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get default tenant: %w", err)
	}
	if membership == nil || membership.Tenant == nil {
		return nil, nil // No tenants
	}

	return &UserTenantSummary{
		TenantID:       membership.TenantID,
		Slug:           membership.Tenant.Slug,
		Name:           membership.Tenant.Name,
		DisplayName:    membership.Tenant.DisplayName,
		LogoURL:        membership.Tenant.LogoURL,
		Role:           membership.Role,
		IsDefault:      membership.IsDefault,
		IsOwner:        membership.Role == models.MembershipRoleOwner,
		Status:         membership.Tenant.Status,
		PrimaryColor:   membership.Tenant.PrimaryColor,
		BusinessModel:  membership.Tenant.BusinessModel,
		LastAccessedAt: membership.LastAccessedAt,
	}, nil
}

// SetUserDefaultTenant sets the user's default tenant
func (s *MembershipService) SetUserDefaultTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	// Verify user has access to this tenant
	hasAccess, err := s.membershipRepo.HasAccess(ctx, userID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to verify access: %w", err)
	}
	if !hasAccess {
		return fmt.Errorf("user does not have access to this tenant")
	}

	return s.membershipRepo.SetDefaultMembership(ctx, userID, tenantID)
}

// ============================================================================
// Tenant Creation (used during onboarding)
// ============================================================================

// CreateTenantRequest represents a request to create a new tenant
type CreateTenantRequest struct {
	Name            string    `json:"name" validate:"required,min=2,max=255"`
	Slug            string    `json:"slug,omitempty"` // Optional, will be generated if not provided
	DisplayName     string    `json:"display_name,omitempty"`
	BusinessType    string    `json:"business_type,omitempty"`
	Industry        string    `json:"industry,omitempty"`
	LogoURL         string    `json:"logo_url,omitempty"`
	PrimaryColor    string    `json:"primary_color,omitempty"`
	SecondaryColor  string    `json:"secondary_color,omitempty"`
	DefaultTimezone string    `json:"default_timezone,omitempty"`
	DefaultCurrency string    `json:"default_currency,omitempty"`
	OwnerUserID     uuid.UUID `json:"owner_user_id" validate:"required"`
}

// CreateTenantResponse represents the response after creating a tenant
type CreateTenantResponse struct {
	Tenant     *models.Tenant               `json:"tenant"`
	Membership *models.UserTenantMembership `json:"membership"`
}

// CreateTenantWithOwner creates a new tenant and assigns the owner membership
func (s *MembershipService) CreateTenantWithOwner(ctx context.Context, req *CreateTenantRequest) (*CreateTenantResponse, error) {
	// Generate slug if not provided
	slug := req.Slug
	if slug == "" {
		var err error
		slug, err = s.membershipRepo.GenerateUniqueSlug(ctx, req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate slug: %w", err)
		}
	} else {
		// Validate provided slug is available
		available, err := s.membershipRepo.IsSlugAvailable(ctx, slug, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check slug availability: %w", err)
		}
		if !available {
			return nil, fmt.Errorf("slug '%s' is already taken", slug)
		}
	}

	// Set defaults
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Name
	}
	timezone := req.DefaultTimezone
	if timezone == "" {
		timezone = "UTC"
	}
	currency := req.DefaultCurrency
	if currency == "" {
		currency = "USD"
	}
	primaryColor := req.PrimaryColor
	if primaryColor == "" {
		primaryColor = "#6366f1"
	}
	secondaryColor := req.SecondaryColor
	if secondaryColor == "" {
		secondaryColor = "#8b5cf6"
	}

	// Create tenant
	tenant := &models.Tenant{
		Name:            req.Name,
		Slug:            slug,
		Subdomain:       slug, // Use slug as subdomain initially
		DisplayName:     displayName,
		LogoURL:         req.LogoURL,
		BusinessType:    req.BusinessType,
		Industry:        req.Industry,
		Status:          "active",
		Mode:            "development",
		PrimaryColor:    primaryColor,
		SecondaryColor:  secondaryColor,
		DefaultTimezone: timezone,
		DefaultCurrency: currency,
		OwnerUserID:     &req.OwnerUserID,
	}

	// Note: In a real implementation, you'd want to use a transaction here
	// For now, we'll create tenant first, then membership

	// The tenant will be created by the caller (onboarding service)
	// We just need to create the membership

	return &CreateTenantResponse{
		Tenant: tenant,
	}, nil
}

// CreateOwnerMembership creates the owner membership for a tenant
func (s *MembershipService) CreateOwnerMembership(ctx context.Context, tenantID, ownerUserID uuid.UUID) (*models.UserTenantMembership, error) {
	now := time.Now()

	// Check if user already has any tenants - only first tenant should be default
	existingTenants, err := s.membershipRepo.GetUserMemberships(ctx, ownerUserID)
	isFirstTenant := err == nil && len(existingTenants) == 0

	membership := &models.UserTenantMembership{
		UserID:     ownerUserID,
		TenantID:   tenantID,
		Role:       models.MembershipRoleOwner,
		IsDefault:  isFirstTenant, // Only first tenant is default
		IsActive:   true,
		AcceptedAt: &now, // Owner doesn't need to accept
	}

	if err := s.membershipRepo.CreateMembership(ctx, membership); err != nil {
		return nil, fmt.Errorf("failed to create owner membership: %w", err)
	}

	return membership, nil
}

// ============================================================================
// Member Management
// ============================================================================

// InviteMemberRequest represents a request to invite a member
type InviteMemberRequest struct {
	TenantID  uuid.UUID `json:"tenant_id" validate:"required"`
	Email     string    `json:"email" validate:"required,email"`
	Role      string    `json:"role" validate:"required,oneof=admin manager member viewer"`
	InvitedBy uuid.UUID `json:"invited_by" validate:"required"`
}

// InviteMemberResponse represents the response after inviting a member
type InviteMemberResponse struct {
	InvitationToken string    `json:"invitation_token"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// InviteMember creates an invitation for a new member
func (s *MembershipService) InviteMember(ctx context.Context, req *InviteMemberRequest) (*InviteMemberResponse, error) {
	// Verify inviter has permission to invite (must be owner or admin)
	inviterRole, err := s.membershipRepo.GetUserRole(ctx, req.InvitedBy, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify inviter role: %w", err)
	}
	if inviterRole != models.MembershipRoleOwner && inviterRole != models.MembershipRoleAdmin {
		return nil, fmt.Errorf("only owners and admins can invite members")
	}

	// Generate invitation token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate invitation token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Set expiry (7 days)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	// Create invitation
	_, err = s.membershipRepo.CreateInvitation(ctx, req.TenantID, req.InvitedBy, req.Email, req.Role, token, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	return &InviteMemberResponse{
		InvitationToken: token,
		ExpiresAt:       expiresAt,
	}, nil
}

// AcceptInvitation accepts a member invitation
func (s *MembershipService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*models.UserTenantMembership, error) {
	return s.membershipRepo.AcceptInvitation(ctx, token, userID)
}

// RemoveMember removes a member from a tenant
func (s *MembershipService) RemoveMember(ctx context.Context, tenantID, memberUserID, removedBy uuid.UUID) error {
	// Verify remover has permission (must be owner or admin)
	removerRole, err := s.membershipRepo.GetUserRole(ctx, removedBy, tenantID)
	if err != nil {
		return fmt.Errorf("failed to verify remover role: %w", err)
	}
	if removerRole != models.MembershipRoleOwner && removerRole != models.MembershipRoleAdmin {
		return fmt.Errorf("only owners and admins can remove members")
	}

	// Cannot remove owner
	memberRole, err := s.membershipRepo.GetUserRole(ctx, memberUserID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get member role: %w", err)
	}
	if memberRole == models.MembershipRoleOwner {
		return fmt.Errorf("cannot remove the tenant owner")
	}

	return s.membershipRepo.DeactivateMembership(ctx, memberUserID, tenantID)
}

// UpdateMemberRole updates a member's role
func (s *MembershipService) UpdateMemberRole(ctx context.Context, tenantID, memberUserID, updatedBy uuid.UUID, newRole string) error {
	// Verify updater has permission (must be owner)
	updaterRole, err := s.membershipRepo.GetUserRole(ctx, updatedBy, tenantID)
	if err != nil {
		return fmt.Errorf("failed to verify updater role: %w", err)
	}
	if updaterRole != models.MembershipRoleOwner {
		return fmt.Errorf("only owners can change member roles")
	}

	// Cannot change owner role
	memberRole, err := s.membershipRepo.GetUserRole(ctx, memberUserID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get member role: %w", err)
	}
	if memberRole == models.MembershipRoleOwner {
		return fmt.Errorf("cannot change the owner's role")
	}

	// Get membership and update
	membership, err := s.membershipRepo.GetMembership(ctx, memberUserID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get membership: %w", err)
	}

	membership.Role = newRole
	return s.membershipRepo.UpdateMembership(ctx, membership)
}

// ============================================================================
// Activity Logging
// ============================================================================

// LogTenantActivity logs an activity for audit purposes
func (s *MembershipService) LogTenantActivity(ctx context.Context, tenantID, userID uuid.UUID, action string, resourceType string, resourceID *uuid.UUID, details map[string]interface{}, ipAddress, userAgent string) error {
	detailsJSON, err := models.NewJSONB(details)
	if err != nil {
		log.Printf("Warning: failed to serialize activity details: %v", err)
		detailsJSON = models.JSONB{}
	}

	log := &models.TenantActivityLog{
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      detailsJSON,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	return s.membershipRepo.LogActivity(ctx, log)
}

// ============================================================================
// Slug Operations
// ============================================================================

// ValidateSlug validates a slug format and availability
func (s *MembershipService) ValidateSlug(ctx context.Context, slug string, excludeTenantID *uuid.UUID) (bool, string, error) {
	// Check format
	if len(slug) < 3 {
		return false, "Slug must be at least 3 characters", nil
	}
	if len(slug) > 50 {
		return false, "Slug must be at most 50 characters", nil
	}

	// Check availability
	available, err := s.membershipRepo.IsSlugAvailable(ctx, slug, excludeTenantID)
	if err != nil {
		return false, "", fmt.Errorf("failed to check availability: %w", err)
	}
	if !available {
		return false, "This URL is already taken", nil
	}

	return true, "", nil
}

// GenerateSlug generates a unique slug from a name
func (s *MembershipService) GenerateSlug(ctx context.Context, name string) (string, error) {
	return s.membershipRepo.GenerateUniqueSlug(ctx, name)
}

// IsSlugAvailable checks if a slug is available (simple boolean check)
func (s *MembershipService) IsSlugAvailable(ctx context.Context, slug string) (bool, error) {
	return s.membershipRepo.IsSlugAvailable(ctx, slug, nil)
}

// ValidateSlugWithSuggestions validates a slug and returns suggestions if taken
// If sessionID is provided, it excludes that session's own reservation from the check
func (s *MembershipService) ValidateSlugWithSuggestions(ctx context.Context, slug string, sessionID *uuid.UUID) (*repository.SlugValidationResult, error) {
	return s.membershipRepo.ValidateSlugWithSuggestions(ctx, slug, sessionID)
}

// ValidateAndReserveSlug validates a slug and reserves it for the session if available
// This is the preferred method during onboarding to prevent race conditions
func (s *MembershipService) ValidateAndReserveSlug(ctx context.Context, slug string, sessionID uuid.UUID, reservedBy string) (*repository.SlugValidationResult, error) {
	return s.membershipRepo.ValidateAndReserveSlug(ctx, slug, sessionID, reservedBy)
}

// ActivateSlugReservation converts a pending slug reservation to active (permanent)
// This should be called after the tenant is successfully created
func (s *MembershipService) ActivateSlugReservation(ctx context.Context, slug string, tenantID uuid.UUID) error {
	return s.membershipRepo.ActivateSlugReservation(ctx, slug, tenantID)
}
