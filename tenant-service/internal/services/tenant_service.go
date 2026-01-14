package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/clients"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	natsClient "github.com/tesseract-hub/domains/common/services/tenant-service/internal/nats"
	"gorm.io/gorm"
)

// TenantService handles tenant-related business logic
type TenantService struct {
	db            *gorm.DB
	membershipSvc *MembershipService
	vendorClient  *clients.VendorClient
	natsClient    *natsClient.Client
}

// NewTenantService creates a new tenant service
func NewTenantService(
	db *gorm.DB,
	membershipSvc *MembershipService,
	vendorClient *clients.VendorClient,
	natsClient *natsClient.Client,
) *TenantService {
	return &TenantService{
		db:            db,
		membershipSvc: membershipSvc,
		vendorClient:  vendorClient,
		natsClient:    natsClient,
	}
}

// CreateTenantForUserRequest represents the request to create a tenant for an existing user
type CreateTenantForUserRequest struct {
	UserID         uuid.UUID
	Name           string
	Slug           string
	Industry       string
	PrimaryColor   string
	SecondaryColor string
}

// CreateTenantForUserResponse represents the response after creating a tenant
type CreateTenantForUserResponse struct {
	Tenant     *TenantInfo     `json:"tenant"`
	Membership *MembershipInfo `json:"membership"`
	AdminURL   string          `json:"admin_url"`
}

// TenantInfo contains basic tenant information
type TenantInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Industry       string `json:"industry"`
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
	Status         string `json:"status"`
}

// MembershipInfo contains membership information
type MembershipInfo struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// CreateTenantForUser creates a new tenant for an existing authenticated user
// This is a simplified flow that bypasses onboarding for users who already have an account
func (s *TenantService) CreateTenantForUser(ctx context.Context, req *CreateTenantForUserRequest) (*CreateTenantForUserResponse, error) {
	// Validate slug format
	if !isValidSlug(req.Slug) {
		return nil, fmt.Errorf("invalid slug format: must contain only lowercase letters, numbers, and hyphens")
	}

	// Check if slug is available
	available, reason := s.CheckSlugAvailability(ctx, req.Slug)
	if !available {
		return nil, fmt.Errorf("slug already exists: %s", reason)
	}

	// Set defaults
	primaryColor := req.PrimaryColor
	if primaryColor == "" {
		primaryColor = "#3b82f6"
	}
	secondaryColor := req.SecondaryColor
	if secondaryColor == "" {
		secondaryColor = "#8b5cf6"
	}
	industry := req.Industry
	if industry == "" {
		industry = "other"
	}

	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create tenant
	tenantID := uuid.New()
	tenant := &models.Tenant{
		ID:             tenantID,
		Name:           req.Name,
		Slug:           req.Slug,
		Subdomain:      req.Slug, // Use slug as subdomain
		DisplayName:    req.Name,
		Industry:       industry,
		Status:         "active",
		Mode:           "development",
		DefaultTimezone: "UTC",
		DefaultCurrency: "USD",
		OwnerUserID:    &req.UserID,
		PricingTier:    models.PricingTierFree,
		PrimaryColor:   primaryColor,
		SecondaryColor: secondaryColor,
	}

	if err := tx.WithContext(ctx).Create(tenant).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	log.Printf("[TenantService] Created tenant %s (ID: %s) for user %s", req.Name, tenantID, req.UserID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Create owner membership (outside transaction for better error handling)
	var membershipInfo *MembershipInfo
	if s.membershipSvc != nil {
		membership, err := s.membershipSvc.CreateOwnerMembership(ctx, tenantID, req.UserID)
		if err != nil {
			log.Printf("[TenantService] Warning: Failed to create owner membership: %v", err)
		} else {
			status := "inactive"
			if membership.IsActive {
				status = "active"
			}
			membershipInfo = &MembershipInfo{
				ID:     membership.ID.String(),
				Role:   membership.Role,
				Status: status,
			}
			log.Printf("[TenantService] Created owner membership for user %s in tenant %s", req.UserID, tenantID)
		}

		// Activate slug reservation
		if err := s.membershipSvc.ActivateSlugReservation(ctx, req.Slug, tenantID); err != nil {
			log.Printf("[TenantService] Warning: Failed to activate slug reservation: %v", err)
		}
	}

	// Create default vendor for the tenant with retry logic
	// CRITICAL: A tenant without a vendor is functionally useless
	if s.vendorClient != nil {
		retryCfg := defaultRetryConfig()
		// Generate a placeholder email for the vendor (required field)
		vendorEmail := fmt.Sprintf("owner@%s.tesserix.app", req.Slug)
		businessName := req.Name

		// Retry vendor creation with exponential backoff
		vendorData, vendorErr := retryWithBackoff(ctx, retryCfg, "Vendor creation", func() (*clients.VendorData, error) {
			return s.vendorClient.CreateVendorForTenant(
				ctx,
				tenantID,
				businessName,
				vendorEmail,
				businessName, // Use business name as contact
			)
		})

		if vendorErr != nil {
			// CRITICAL: Vendor creation is essential - fail tenant creation
			log.Printf("[TenantService] CRITICAL: Failed to create vendor for tenant %s after retries: %v", tenantID, vendorErr)
			return nil, fmt.Errorf("failed to create vendor for tenant %s: %w - tenant creation cannot complete without a vendor", tenantID, vendorErr)
		}

		log.Printf("[TenantService] Created vendor %s for tenant %s", vendorData.ID, tenantID)

		// Create default storefront for the vendor with retry logic
		vendorID := vendorData.ID

		storefrontData, sfErr := retryWithBackoff(ctx, retryCfg, "Storefront creation", func() (*clients.StorefrontData, error) {
			return s.vendorClient.CreateStorefront(
				ctx,
				tenantID,
				vendorID,
				businessName, // Use business name as storefront name
				req.Slug,     // Use tenant slug as storefront slug
				true,         // Set as default storefront
			)
		})

		if sfErr != nil {
			// CRITICAL: Storefront creation is essential - fail tenant creation
			log.Printf("[TenantService] CRITICAL: Failed to create storefront for vendor %s after retries: %v", vendorData.ID, sfErr)
			return nil, fmt.Errorf("failed to create storefront for vendor %s: %w - tenant creation cannot complete without a storefront", vendorData.ID, sfErr)
		}

		log.Printf("[TenantService] Created default storefront %s for vendor %s", storefrontData.ID, vendorData.ID)
	} else {
		// No vendor client configured - this is a critical configuration error
		log.Printf("[TenantService] CRITICAL: Vendor client not configured - cannot create vendor for tenant %s", tenantID)
		return nil, fmt.Errorf("vendor client not configured - cannot complete tenant creation without vendor creation capability")
	}

	// Generate admin URL (subdomain-based routing)
	// URL pattern: https://{slug}-admin.{baseDomain}
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}
	adminURL := fmt.Sprintf("https://%s-admin.%s", req.Slug, baseDomain)

	// Publish tenant.created event for routing and other subscribers
	go func() {
		if s.natsClient != nil {
			// Generate host URLs for tenant-router-service
			adminHost := fmt.Sprintf("%s-admin.%s", req.Slug, baseDomain)
			storefrontHost := fmt.Sprintf("%s.%s", req.Slug, baseDomain)

			event := &natsClient.TenantCreatedEvent{
				TenantID:       tenantID.String(),
				Product:        "ecommerce",
				BusinessName:   req.Name,
				Slug:           req.Slug,
				AdminHost:      adminHost,
				StorefrontHost: storefrontHost,
				BaseDomain:     baseDomain,
			}
			if err := s.natsClient.PublishTenantCreated(context.Background(), event); err != nil {
				log.Printf("[TenantService] Failed to publish tenant.created event: %v", err)
			}
		}
	}()

	return &CreateTenantForUserResponse{
		Tenant: &TenantInfo{
			ID:             tenantID.String(),
			Name:           req.Name,
			Slug:           req.Slug,
			Industry:       industry,
			PrimaryColor:   primaryColor,
			SecondaryColor: secondaryColor,
			Status:         "active",
		},
		Membership: membershipInfo,
		AdminURL:   adminURL,
	}, nil
}

// CheckSlugAvailability checks if a slug is available for use
func (s *TenantService) CheckSlugAvailability(ctx context.Context, slug string) (bool, string) {
	// Check format
	if !isValidSlug(slug) {
		return false, "Invalid slug format"
	}

	// Check reserved slugs
	reservedSlugs := []string{
		"admin", "api", "www", "app", "login", "signup", "register",
		"dashboard", "settings", "help", "support", "billing", "account",
		"auth", "oauth", "static", "assets", "public", "private",
	}
	for _, reserved := range reservedSlugs {
		if slug == reserved {
			return false, "This slug is reserved"
		}
	}

	// Check if slug exists in tenants table
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
		return false, "Unable to verify slug availability"
	}
	if count > 0 {
		return false, "Slug is already in use"
	}

	// Check if slug is available (not reserved)
	if s.membershipSvc != nil {
		isAvailable, _ := s.membershipSvc.IsSlugAvailable(ctx, slug)
		if !isAvailable {
			return false, "Slug is currently reserved"
		}
	}

	return true, ""
}

// isValidSlug checks if a slug has valid format
func isValidSlug(slug string) bool {
	if len(slug) < 3 || len(slug) > 63 {
		return false
	}
	// Must start with letter, contain only lowercase letters, numbers, and hyphens
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$`, slug)
	if !matched {
		// Also allow single-segment slugs like "abc"
		matched, _ = regexp.MatchString(`^[a-z][a-z0-9]*$`, slug)
	}
	// No consecutive hyphens
	if strings.Contains(slug, "--") {
		return false
	}
	return matched
}

// TenantBasicInfo represents basic tenant information for internal service calls
type TenantBasicInfo struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	DisplayName  string `json:"displayName,omitempty"`
	Subdomain    string `json:"subdomain,omitempty"`
	BillingEmail string `json:"billingEmail,omitempty"`
}

// GetTenantByID retrieves basic tenant information by ID (for internal service calls)
func (s *TenantService) GetTenantByID(ctx context.Context, tenantID uuid.UUID) (*TenantBasicInfo, error) {
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return nil, err
	}

	return &TenantBasicInfo{
		ID:          tenant.ID.String(),
		Slug:        tenant.Slug,
		Name:        tenant.Name,
		DisplayName: tenant.DisplayName,
		Subdomain:   tenant.Subdomain,
		BillingEmail: tenant.BillingEmail,
	}, nil
}

// GetTenantBySlug retrieves basic tenant information by slug (for internal service calls)
func (s *TenantService) GetTenantBySlug(ctx context.Context, slug string) (*TenantBasicInfo, error) {
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&tenant).Error; err != nil {
		return nil, err
	}

	return &TenantBasicInfo{
		ID:          tenant.ID.String(),
		Slug:        tenant.Slug,
		Name:        tenant.Name,
		DisplayName: tenant.DisplayName,
		Subdomain:   tenant.Subdomain,
		BillingEmail: tenant.BillingEmail,
	}, nil
}

// ============================================================================
// Tenant Onboarding Data (for Settings Pre-population)
// ============================================================================

// TenantOnboardingData represents the sanitized onboarding data for a tenant
// This is used to pre-populate settings pages with data collected during onboarding
// Note: PII fields are included but should be transmitted over HTTPS only
type TenantOnboardingData struct {
	TenantID    string               `json:"tenant_id"`
	TenantSlug  string               `json:"tenant_slug"`
	TenantName  string               `json:"tenant_name"`
	CompletedAt string               `json:"completed_at,omitempty"`
	Business    *OnboardingBusiness  `json:"business,omitempty"`
	Contact     *OnboardingContact   `json:"contact,omitempty"`
	Address     *OnboardingAddress   `json:"address,omitempty"`
	StoreSetup  *OnboardingStoreSetup `json:"store_setup,omitempty"`
}

// OnboardingBusiness represents business information from onboarding
type OnboardingBusiness struct {
	BusinessName        string `json:"business_name"`
	BusinessType        string `json:"business_type"`
	Industry            string `json:"industry"`
	BusinessDescription string `json:"business_description,omitempty"`
	Website             string `json:"website,omitempty"`
	RegistrationNumber  string `json:"registration_number,omitempty"`
	TaxID               string `json:"tax_id,omitempty"`
}

// OnboardingContact represents primary contact information from onboarding
type OnboardingContact struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
	JobTitle  string `json:"job_title,omitempty"`
}

// OnboardingAddress represents business address from onboarding
type OnboardingAddress struct {
	StreetAddress string `json:"street_address"`
	City          string `json:"city"`
	StateProvince string `json:"state_province,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
	Country       string `json:"country"`
}

// OnboardingStoreSetup represents store configuration from onboarding
type OnboardingStoreSetup struct {
	Subdomain      string `json:"subdomain"`
	Currency       string `json:"currency"`
	Timezone       string `json:"timezone"`
	Language       string `json:"language"`
	LogoURL        string `json:"logo_url,omitempty"`
	PrimaryColor   string `json:"primary_color,omitempty"`
	SecondaryColor string `json:"secondary_color,omitempty"`
}

// GetTenantOnboardingData retrieves onboarding data for a tenant with proper access control
// This endpoint enforces multi-tenant isolation by:
// 1. Verifying the user has an active membership to the tenant
// 2. Only returning data for the specific tenant requested
// 3. Not exposing internal session IDs or other sensitive metadata
func (s *TenantService) GetTenantOnboardingData(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) (*TenantOnboardingData, error) {
	// SECURITY: First verify user has access to this tenant
	// This prevents cross-tenant data access
	hasAccess, err := s.membershipSvc.membershipRepo.HasAccess(ctx, userID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify access: %w", err)
	}
	if !hasAccess {
		return nil, fmt.Errorf("access denied: user does not have access to this tenant")
	}

	// Get full tenant model (not TenantBasicInfo) to access all fields
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Query completed onboarding session for this tenant
	var session models.OnboardingSession
	err = s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, "completed").
		Preload("BusinessInformation").
		Preload("ContactInformation").
		Preload("BusinessAddresses").
		Order("completed_at DESC").
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		// No completed onboarding - return tenant basic info only
		// This is valid for tenants created via quick-create flow
		// Use tenant's actual settings from CompleteAccountSetup, not hardcoded defaults
		currency := tenant.DefaultCurrency
		if currency == "" {
			currency = "USD"
		}
		timezone := tenant.DefaultTimezone
		if timezone == "" {
			timezone = "UTC"
		}
		return &TenantOnboardingData{
			TenantID:   tenant.ID.String(),
			TenantSlug: tenant.Slug,
			TenantName: tenant.DisplayName,
			StoreSetup: &OnboardingStoreSetup{
				Subdomain:      tenant.Subdomain,
				Currency:       currency,
				Timezone:       timezone,
				Language:       "en",
				LogoURL:        tenant.LogoURL,
				PrimaryColor:   tenant.PrimaryColor,
				SecondaryColor: tenant.SecondaryColor,
			},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get onboarding data: %w", err)
	}

	// Build response with sanitized data
	result := &TenantOnboardingData{
		TenantID:   tenant.ID.String(),
		TenantSlug: tenant.Slug,
		TenantName: tenant.DisplayName,
	}

	if session.CompletedAt != nil {
		result.CompletedAt = session.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Map business information
	if session.BusinessInformation != nil {
		bi := session.BusinessInformation
		result.Business = &OnboardingBusiness{
			BusinessName:        bi.BusinessName,
			BusinessType:        bi.BusinessType,
			Industry:            bi.Industry,
			BusinessDescription: bi.BusinessDescription,
			Website:             bi.Website,
			RegistrationNumber:  bi.RegistrationNumber,
			TaxID:               bi.TaxID,
		}
	}

	// Map primary contact (first contact in list)
	if len(session.ContactInformation) > 0 {
		ci := session.ContactInformation[0]
		result.Contact = &OnboardingContact{
			FirstName: ci.FirstName,
			LastName:  ci.LastName,
			Email:     ci.Email,
			Phone:     ci.Phone,
			JobTitle:  ci.JobTitle,
		}
	}

	// Map primary address (first address in list)
	if len(session.BusinessAddresses) > 0 {
		ba := session.BusinessAddresses[0]
		result.Address = &OnboardingAddress{
			StreetAddress: ba.StreetAddress,
			City:          ba.City,
			StateProvince: ba.StateProvince,
			PostalCode:    ba.PostalCode,
			Country:       ba.Country,
		}
	}

	// Map store setup from tenant model (these are set during onboarding completion)
	var tenantModel models.Tenant
	if err := s.db.WithContext(ctx).First(&tenantModel, tenantID).Error; err == nil {
		result.StoreSetup = &OnboardingStoreSetup{
			Subdomain:      tenantModel.Subdomain,
			Currency:       tenantModel.DefaultCurrency,
			Timezone:       tenantModel.DefaultTimezone,
			Language:       "en", // Default - could be added to tenant model
			LogoURL:        tenantModel.LogoURL,
			PrimaryColor:   tenantModel.PrimaryColor,
			SecondaryColor: tenantModel.SecondaryColor,
		}
	}

	return result, nil
}
