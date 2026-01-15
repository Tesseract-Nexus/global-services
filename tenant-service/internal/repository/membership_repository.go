package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"gorm.io/gorm"
)

// MembershipRepository handles user-tenant membership database operations
type MembershipRepository struct {
	db                 *gorm.DB
	tenantRouterClient TenantRouterClientInterface
	vendorClient       VendorClientInterface
}

// TenantRouterClientInterface defines the interface for tenant-router-service client
type TenantRouterClientInterface interface {
	IsSlugRecentlyDeleted(ctx context.Context, slug string) (bool, int, string, error)
}

// VendorClientInterface defines the interface for vendor-service client
// Used to resolve storefronts when tenant slug lookup fails
type VendorClientInterface interface {
	GetStorefrontBySlug(ctx context.Context, slug string) (StorefrontInfo, error)
}

// StorefrontInfo is an interface for storefront data needed for tenant resolution
type StorefrontInfo interface {
	GetTenantID() string
}

// NewMembershipRepository creates a new membership repository
func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

// SetTenantRouterClient sets the tenant router client for checking recently deleted slugs
func (r *MembershipRepository) SetTenantRouterClient(client TenantRouterClientInterface) {
	r.tenantRouterClient = client
}

// SetVendorClient sets the vendor client for storefront slug resolution
func (r *MembershipRepository) SetVendorClient(client VendorClientInterface) {
	r.vendorClient = client
}

// ============================================================================
// Tenant Operations
// ============================================================================

// GetTenantBySlug retrieves a tenant by its URL slug
// Falls back to storefront slug lookup if tenant slug not found
// This handles the case where storefront slug differs from tenant slug
func (r *MembershipRepository) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&tenant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Try storefront slug lookup as fallback
			if r.vendorClient != nil {
				storefront, sfErr := r.vendorClient.GetStorefrontBySlug(ctx, slug)
				if sfErr == nil && storefront != nil && storefront.GetTenantID() != "" {
					// Parse tenant ID and fetch tenant
					tenantID, parseErr := uuid.Parse(storefront.GetTenantID())
					if parseErr == nil {
						if lookupErr := r.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; lookupErr == nil {
							return &tenant, nil
						}
					}
				}
			}
			return nil, fmt.Errorf("tenant not found: %s", slug)
		}
		return nil, fmt.Errorf("failed to get tenant by slug: %w", err)
	}
	return &tenant, nil
}

// GetTenantByID retrieves a tenant by its ID
func (r *MembershipRepository) GetTenantByID(ctx context.Context, tenantID uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := r.db.WithContext(ctx).First(&tenant, "id = ?", tenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found: %s", tenantID)
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &tenant, nil
}

// UpdateTenant updates a tenant's details
func (r *MembershipRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	tenant.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(tenant).Error; err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}
	return nil
}

// GenerateUniqueSlug generates a unique URL-friendly slug from a tenant name
func (r *MembershipRepository) GenerateUniqueSlug(ctx context.Context, name string) (string, error) {
	// Convert to lowercase and replace non-alphanumeric chars with hyphens
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	baseSlug := strings.ToLower(reg.ReplaceAllString(name, "-"))
	// Trim leading/trailing hyphens
	baseSlug = strings.Trim(baseSlug, "-")
	// Limit length
	if len(baseSlug) > 45 {
		baseSlug = baseSlug[:45]
	}
	// Ensure minimum length
	if len(baseSlug) < 3 {
		baseSlug = "store-" + baseSlug
	}

	slug := baseSlug
	counter := 0

	// Check for uniqueness and append number if needed
	for {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
			return "", fmt.Errorf("failed to check slug uniqueness: %w", err)
		}
		if count == 0 {
			break
		}
		counter++
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
	}

	return slug, nil
}

// IsSlugAvailable checks if a slug is available for use
func (r *MembershipRepository) IsSlugAvailable(ctx context.Context, slug string, excludeTenantID *uuid.UUID) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug)
	if excludeTenantID != nil {
		query = query.Where("id != ?", *excludeTenantID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check slug availability: %w", err)
	}
	return count == 0, nil
}

// SlugValidationResult contains the result of slug validation with suggestions
type SlugValidationResult struct {
	Slug            string   `json:"slug"`
	Available       bool     `json:"available"`
	Message         string   `json:"message,omitempty"`
	Suggestions     []string `json:"suggestions,omitempty"`
	RecentlyDeleted bool     `json:"recently_deleted,omitempty"` // True if slug was recently deleted
	DaysRemaining   int      `json:"days_remaining,omitempty"`   // Days until slug is available again
}

// ValidateSlugWithSuggestions checks if a slug is available and provides alternatives if not
// If sessionID is provided, it excludes that session's own reservation from the check
func (r *MembershipRepository) ValidateSlugWithSuggestions(ctx context.Context, requestedSlug string, sessionID *uuid.UUID) (*SlugValidationResult, error) {
	// Normalize the slug
	normalizedSlug := normalizeSlug(requestedSlug)

	// Validate slug format
	if len(normalizedSlug) < 3 {
		return &SlugValidationResult{
			Slug:      requestedSlug,
			Available: false,
			Message:   "Slug must be at least 3 characters long",
		}, nil
	}

	if len(normalizedSlug) > 50 {
		return &SlugValidationResult{
			Slug:      requestedSlug,
			Available: false,
			Message:   "Slug must be 50 characters or less",
		}, nil
	}

	// Check if it's a reserved slug (database lookup with caching)
	isReserved, reservedInfo, err := r.IsReservedSlug(ctx, normalizedSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to check reserved slug: %w", err)
	}
	if isReserved {
		suggestions, err := r.generateSlugSuggestions(ctx, normalizedSlug, 5)
		if err != nil {
			return nil, err
		}
		reason := "This name is reserved and cannot be used"
		if reservedInfo != nil && reservedInfo.Reason != "" {
			reason = fmt.Sprintf("This name is reserved: %s", reservedInfo.Reason)
		}
		return &SlugValidationResult{
			Slug:        normalizedSlug,
			Available:   false,
			Message:     reason,
			Suggestions: suggestions,
		}, nil
	}

	// Check if slug is already reserved/taken by another session
	// Pass sessionID to exclude current session's own reservation
	isReservedByOther, existingReservation, err := r.IsSlugReservedOrTaken(ctx, normalizedSlug, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check slug reservation: %w", err)
	}
	if isReservedByOther {
		suggestions, err := r.generateSlugSuggestions(ctx, normalizedSlug, 5)
		if err != nil {
			return nil, err
		}
		message := "This name is already taken. Try one of these alternatives:"
		if existingReservation != nil && existingReservation.Status == models.SlugReservationPending {
			message = "This name is currently being held by another user. Try one of these alternatives:"
		}
		return &SlugValidationResult{
			Slug:        normalizedSlug,
			Available:   false,
			Message:     message,
			Suggestions: suggestions,
		}, nil
	}

	// Also check existing tenants table (in case reservation wasn't created)
	available, err := r.IsSlugAvailable(ctx, normalizedSlug, sessionID)
	if err != nil {
		return nil, err
	}

	if available {
		// Check if slug was recently deleted (tenant-router-service tracks routing resources)
		// This handles the case where a tenant was deleted but DNS/certs need time to cleanup
		if r.tenantRouterClient != nil {
			isRecentlyDeleted, daysRemaining, message, err := r.tenantRouterClient.IsSlugRecentlyDeleted(ctx, normalizedSlug)
			if err == nil && isRecentlyDeleted {
				suggestions, _ := r.generateSlugSuggestions(ctx, normalizedSlug, 5)
				return &SlugValidationResult{
					Slug:            normalizedSlug,
					Available:       false,
					Message:         message,
					Suggestions:     suggestions,
					RecentlyDeleted: true,
					DaysRemaining:   daysRemaining,
				}, nil
			}
		}

		return &SlugValidationResult{
			Slug:      normalizedSlug,
			Available: true,
			Message:   "This name is available!",
		}, nil
	}

	// Not available - generate suggestions
	suggestions, err := r.generateSlugSuggestions(ctx, normalizedSlug, 5)
	if err != nil {
		return nil, err
	}

	return &SlugValidationResult{
		Slug:        normalizedSlug,
		Available:   false,
		Message:     "This name is already taken. Try one of these alternatives:",
		Suggestions: suggestions,
	}, nil
}

// ValidateAndReserveSlug validates a slug and reserves it for the session if available
// This is the main method to use during onboarding to claim a slug
func (r *MembershipRepository) ValidateAndReserveSlug(ctx context.Context, requestedSlug string, sessionID uuid.UUID, reservedBy string) (*SlugValidationResult, error) {
	// First validate - pass sessionID to exclude current session's own reservation
	result, err := r.ValidateSlugWithSuggestions(ctx, requestedSlug, &sessionID)
	if err != nil {
		return nil, err
	}

	// If available, reserve it
	if result.Available {
		_, err := r.ReserveSlug(ctx, result.Slug, sessionID, reservedBy)
		if err != nil {
			// Race condition - someone else grabbed it
			result.Available = false
			result.Message = "This name was just taken. Try one of these alternatives:"
			result.Suggestions, _ = r.generateSlugSuggestions(ctx, result.Slug, 5)
		}
	}

	return result, nil
}

// generateSlugSuggestions generates available slug alternatives
func (r *MembershipRepository) generateSlugSuggestions(ctx context.Context, baseSlug string, count int) ([]string, error) {
	suggestions := make([]string, 0, count)

	// Strategy 1: Append numbers (1, 2, 3...)
	for i := 1; len(suggestions) < count && i <= 99; i++ {
		candidate := fmt.Sprintf("%s-%d", baseSlug, i)
		available, err := r.IsSlugAvailable(ctx, candidate, nil)
		if err != nil {
			return nil, err
		}
		if available {
			suggestions = append(suggestions, candidate)
		}
	}

	// Strategy 2: Append common suffixes
	suffixes := []string{"store", "shop", "hub", "online", "app", "hq"}
	for _, suffix := range suffixes {
		if len(suggestions) >= count {
			break
		}
		candidate := fmt.Sprintf("%s-%s", baseSlug, suffix)
		if len(candidate) <= 50 {
			available, err := r.IsSlugAvailable(ctx, candidate, nil)
			if err != nil {
				return nil, err
			}
			if available {
				suggestions = append(suggestions, candidate)
			}
		}
	}

	// Strategy 3: Append year
	if len(suggestions) < count {
		candidate := fmt.Sprintf("%s-2025", baseSlug)
		if len(candidate) <= 50 {
			available, err := r.IsSlugAvailable(ctx, candidate, nil)
			if err != nil {
				return nil, err
			}
			if available {
				suggestions = append(suggestions, candidate)
			}
		}
	}

	return suggestions, nil
}

// normalizeSlug converts a string to a valid slug format
func normalizeSlug(input string) string {
	// Convert to lowercase
	slug := strings.ToLower(input)
	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Remove consecutive hyphens
	reg = regexp.MustCompile("-+")
	slug = reg.ReplaceAllString(slug, "-")
	return slug
}

// ReservedSlugCache provides in-memory caching for reserved slugs
// This avoids hitting the database for every slug validation
type ReservedSlugCache struct {
	slugs    map[string]*models.ReservedSlug
	loadedAt time.Time
	cacheTTL time.Duration
	mu       sync.RWMutex
}

var reservedSlugCache = &ReservedSlugCache{
	slugs:    make(map[string]*models.ReservedSlug),
	cacheTTL: 5 * time.Minute, // Cache for 5 minutes
}

// IsReservedSlug checks if a slug is reserved using database with caching
func (r *MembershipRepository) IsReservedSlug(ctx context.Context, slug string) (bool, *models.ReservedSlug, error) {
	// Check cache first
	reservedSlugCache.mu.RLock()
	if time.Since(reservedSlugCache.loadedAt) < reservedSlugCache.cacheTTL && len(reservedSlugCache.slugs) > 0 {
		if reserved, exists := reservedSlugCache.slugs[slug]; exists {
			reservedSlugCache.mu.RUnlock()
			return true, reserved, nil
		}
		reservedSlugCache.mu.RUnlock()
		return false, nil, nil
	}
	reservedSlugCache.mu.RUnlock()

	// Cache expired or empty, reload from database
	if err := r.loadReservedSlugsCache(ctx); err != nil {
		// If DB fails, fall back to checking just this slug
		var reserved models.ReservedSlug
		if err := r.db.WithContext(ctx).Where("slug = ? AND is_active = ?", slug, true).First(&reserved).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil, nil
			}
			return false, nil, fmt.Errorf("failed to check reserved slug: %w", err)
		}
		return true, &reserved, nil
	}

	// Check cache again after reload
	reservedSlugCache.mu.RLock()
	defer reservedSlugCache.mu.RUnlock()
	if reserved, exists := reservedSlugCache.slugs[slug]; exists {
		return true, reserved, nil
	}
	return false, nil, nil
}

// loadReservedSlugsCache loads all reserved slugs into memory
func (r *MembershipRepository) loadReservedSlugsCache(ctx context.Context) error {
	reservedSlugCache.mu.Lock()
	defer reservedSlugCache.mu.Unlock()

	// Double-check if another goroutine already loaded
	if time.Since(reservedSlugCache.loadedAt) < reservedSlugCache.cacheTTL && len(reservedSlugCache.slugs) > 0 {
		return nil
	}

	var slugs []models.ReservedSlug
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&slugs).Error; err != nil {
		return fmt.Errorf("failed to load reserved slugs: %w", err)
	}

	// Rebuild cache
	newCache := make(map[string]*models.ReservedSlug, len(slugs))
	for i := range slugs {
		newCache[slugs[i].Slug] = &slugs[i]
	}

	reservedSlugCache.slugs = newCache
	reservedSlugCache.loadedAt = time.Now()
	return nil
}

// GetAllReservedSlugs returns all reserved slugs (for admin API)
func (r *MembershipRepository) GetAllReservedSlugs(ctx context.Context, includeInactive bool) ([]models.ReservedSlug, error) {
	var slugs []models.ReservedSlug
	query := r.db.WithContext(ctx)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	if err := query.Order("category, slug").Find(&slugs).Error; err != nil {
		return nil, fmt.Errorf("failed to get reserved slugs: %w", err)
	}
	return slugs, nil
}

// AddReservedSlug adds a new reserved slug (admin API)
func (r *MembershipRepository) AddReservedSlug(ctx context.Context, slug *models.ReservedSlug) error {
	if err := r.db.WithContext(ctx).Create(slug).Error; err != nil {
		return fmt.Errorf("failed to add reserved slug: %w", err)
	}
	// Invalidate cache
	reservedSlugCache.mu.Lock()
	reservedSlugCache.loadedAt = time.Time{}
	reservedSlugCache.mu.Unlock()
	return nil
}

// RemoveReservedSlug deactivates a reserved slug (admin API)
func (r *MembershipRepository) RemoveReservedSlug(ctx context.Context, slug string) error {
	if err := r.db.WithContext(ctx).Model(&models.ReservedSlug{}).
		Where("slug = ?", slug).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("failed to remove reserved slug: %w", err)
	}
	// Invalidate cache
	reservedSlugCache.mu.Lock()
	reservedSlugCache.loadedAt = time.Time{}
	reservedSlugCache.mu.Unlock()
	return nil
}

// InvalidateReservedSlugCache forces cache refresh on next lookup
func (r *MembershipRepository) InvalidateReservedSlugCache() {
	reservedSlugCache.mu.Lock()
	reservedSlugCache.loadedAt = time.Time{}
	reservedSlugCache.mu.Unlock()
}

// ============================================================================
// Slug Reservation Operations (for onboarding flow)
// ============================================================================

// IsSlugReservedOrTaken checks if a slug is already reserved or taken by another session/tenant
// Returns: isReserved, existingReservation, error
func (r *MembershipRepository) IsSlugReservedOrTaken(ctx context.Context, slug string, currentSessionID *uuid.UUID) (bool, *models.TenantSlugReservation, error) {
	var reservation models.TenantSlugReservation

	query := r.db.WithContext(ctx).
		Where("slug = ? AND status IN (?, ?)", slug, models.SlugReservationPending, models.SlugReservationActive)

	// Exclude current session's own reservation
	if currentSessionID != nil {
		query = query.Where("(session_id IS NULL OR session_id != ?)", *currentSessionID)
	}

	if err := query.First(&reservation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil, nil // Not reserved
		}
		return false, nil, fmt.Errorf("failed to check slug reservation: %w", err)
	}

	// Check if pending reservation has expired
	if reservation.Status == models.SlugReservationPending &&
		reservation.ExpiresAt != nil &&
		reservation.ExpiresAt.Before(time.Now()) {
		// Expired - release it
		r.releaseExpiredReservation(ctx, &reservation)
		return false, nil, nil
	}

	return true, &reservation, nil
}

// ReserveSlug reserves a slug for an onboarding session (temporary hold)
func (r *MembershipRepository) ReserveSlug(ctx context.Context, slug string, sessionID uuid.UUID, reservedBy string) (*models.TenantSlugReservation, error) {
	expiresAt := time.Now().Add(models.DefaultSlugReservationDuration)

	reservation := &models.TenantSlugReservation{
		Slug:       slug,
		Status:     models.SlugReservationPending,
		SessionID:  &sessionID,
		ReservedBy: reservedBy,
		ExpiresAt:  &expiresAt,
	}

	// Use upsert to handle race conditions
	result := r.db.WithContext(ctx).
		Where("slug = ?", slug).
		Assign(map[string]interface{}{
			"status":      models.SlugReservationPending,
			"session_id":  sessionID,
			"reserved_by": reservedBy,
			"expires_at":  expiresAt,
			"updated_at":  time.Now(),
			"released_at": nil,
		}).
		FirstOrCreate(reservation)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to reserve slug: %w", result.Error)
	}

	return reservation, nil
}

// ActivateSlugReservation converts a pending reservation to active (when tenant is created)
func (r *MembershipRepository) ActivateSlugReservation(ctx context.Context, slug string, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("slug = ?", slug).
		Updates(map[string]interface{}{
			"status":     models.SlugReservationActive,
			"tenant_id":  tenantID,
			"expires_at": nil, // No expiry for active
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to activate slug reservation: %w", result.Error)
	}

	// If no existing reservation, create one
	if result.RowsAffected == 0 {
		reservation := &models.TenantSlugReservation{
			Slug:     slug,
			Status:   models.SlugReservationActive,
			TenantID: &tenantID,
		}
		if err := r.db.WithContext(ctx).Create(reservation).Error; err != nil {
			return fmt.Errorf("failed to create active slug reservation: %w", err)
		}
	}

	return nil
}

// ReleaseSlugReservation releases a slug reservation (when user abandons or session expires)
func (r *MembershipRepository) ReleaseSlugReservation(ctx context.Context, slug string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("slug = ? AND status = ?", slug, models.SlugReservationPending).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": now,
			"updated_at":  now,
		}).Error; err != nil {
		return fmt.Errorf("failed to release slug reservation: %w", err)
	}
	return nil
}

// ReleaseSlugsBySession releases all slug reservations for a session (when session is abandoned)
func (r *MembershipRepository) ReleaseSlugsBySession(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("session_id = ? AND status = ?", sessionID, models.SlugReservationPending).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": now,
			"updated_at":  now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to release session slugs: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ReleaseSlugByTenant releases the slug when a tenant is deleted
func (r *MembershipRepository) ReleaseSlugByTenant(ctx context.Context, tenantID uuid.UUID) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.SlugReservationActive).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": now,
			"updated_at":  now,
		}).Error; err != nil {
		return fmt.Errorf("failed to release tenant slug: %w", err)
	}
	return nil
}

// CleanupExpiredReservations releases all expired pending reservations
func (r *MembershipRepository) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?",
			models.SlugReservationPending, now).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": now,
			"updated_at":  now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup expired reservations: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetSlugReservation gets a specific slug reservation
func (r *MembershipRepository) GetSlugReservation(ctx context.Context, slug string) (*models.TenantSlugReservation, error) {
	var reservation models.TenantSlugReservation
	if err := r.db.WithContext(ctx).
		Where("slug = ? AND status IN (?, ?)", slug, models.SlugReservationPending, models.SlugReservationActive).
		First(&reservation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get slug reservation: %w", err)
	}
	return &reservation, nil
}

// ExtendSlugReservation extends the expiration time for a pending reservation
func (r *MembershipRepository) ExtendSlugReservation(ctx context.Context, slug string, sessionID uuid.UUID) error {
	newExpiry := time.Now().Add(models.DefaultSlugReservationDuration)
	result := r.db.WithContext(ctx).
		Model(&models.TenantSlugReservation{}).
		Where("slug = ? AND session_id = ? AND status = ?", slug, sessionID, models.SlugReservationPending).
		Update("expires_at", newExpiry)

	if result.Error != nil {
		return fmt.Errorf("failed to extend slug reservation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no matching reservation found to extend")
	}
	return nil
}

// releaseExpiredReservation helper to release a single expired reservation
func (r *MembershipRepository) releaseExpiredReservation(ctx context.Context, reservation *models.TenantSlugReservation) {
	now := time.Now()
	r.db.WithContext(ctx).
		Model(reservation).
		Updates(map[string]interface{}{
			"status":      models.SlugReservationReleased,
			"released_at": now,
			"updated_at":  now,
		})
}

// ============================================================================
// Membership Operations
// ============================================================================

// CreateMembership creates a new user-tenant membership
func (r *MembershipRepository) CreateMembership(ctx context.Context, membership *models.UserTenantMembership) error {
	if err := r.db.WithContext(ctx).Create(membership).Error; err != nil {
		return fmt.Errorf("failed to create membership: %w", err)
	}
	return nil
}

// GetMembership retrieves a specific membership by user and tenant
func (r *MembershipRepository) GetMembership(ctx context.Context, userID, tenantID uuid.UUID) (*models.UserTenantMembership, error) {
	var membership models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&membership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found is not an error, just no membership
		}
		return nil, fmt.Errorf("failed to get membership: %w", err)
	}
	return &membership, nil
}

// GetUserMemberships retrieves all active memberships for a user
func (r *MembershipRepository) GetUserMemberships(ctx context.Context, userID uuid.UUID) ([]models.UserTenantMembership, error) {
	var memberships []models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("is_default DESC, last_accessed_at DESC NULLS LAST").
		Find(&memberships).Error; err != nil {
		return nil, fmt.Errorf("failed to get user memberships: %w", err)
	}
	return memberships, nil
}

// GetUserByKeycloakID retrieves a user by their Keycloak ID
// This is used to map auth-bff session (which uses Keycloak ID) to local user ID for membership lookup
func (r *MembershipRepository) GetUserByKeycloakID(ctx context.Context, keycloakID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("keycloak_id = ?", keycloakID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found is not an error - user may not have keycloak_id set yet
		}
		return nil, fmt.Errorf("failed to get user by keycloak_id: %w", err)
	}
	return &user, nil
}

// GetTenantMemberships retrieves all memberships for a tenant
func (r *MembershipRepository) GetTenantMemberships(ctx context.Context, tenantID uuid.UUID) ([]models.UserTenantMembership, error) {
	var memberships []models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("role, created_at").
		Find(&memberships).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant memberships: %w", err)
	}
	return memberships, nil
}

// GetUserDefaultMembership retrieves the user's default tenant membership
func (r *MembershipRepository) GetUserDefaultMembership(ctx context.Context, userID uuid.UUID) (*models.UserTenantMembership, error) {
	var membership models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("user_id = ? AND is_default = ? AND is_active = ?", userID, true, true).
		First(&membership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// No default set, get first active membership
			if err := r.db.WithContext(ctx).
				Preload("Tenant").
				Where("user_id = ? AND is_active = ?", userID, true).
				Order("created_at ASC").
				First(&membership).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, nil
				}
				return nil, fmt.Errorf("failed to get first membership: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get default membership: %w", err)
		}
	}
	return &membership, nil
}

// UpdateMembership updates a membership
func (r *MembershipRepository) UpdateMembership(ctx context.Context, membership *models.UserTenantMembership) error {
	membership.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(membership).Error; err != nil {
		return fmt.Errorf("failed to update membership: %w", err)
	}
	return nil
}

// SetDefaultMembership sets a membership as the user's default
func (r *MembershipRepository) SetDefaultMembership(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Unset current default
		if err := tx.Model(&models.UserTenantMembership{}).
			Where("user_id = ? AND is_default = ?", userID, true).
			Updates(map[string]interface{}{
				"is_default": false,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to unset current default: %w", err)
		}

		// Set new default
		if err := tx.Model(&models.UserTenantMembership{}).
			Where("user_id = ? AND tenant_id = ?", userID, tenantID).
			Updates(map[string]interface{}{
				"is_default": true,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to set new default: %w", err)
		}

		return nil
	})
}

// UpdateLastAccessed updates the last accessed time for a membership
func (r *MembershipRepository) UpdateLastAccessed(ctx context.Context, userID, tenantID uuid.UUID) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("last_accessed_at", now).Error; err != nil {
		return fmt.Errorf("failed to update last accessed: %w", err)
	}
	return nil
}

// DeactivateMembership soft-deletes a membership by setting is_active to false
func (r *MembershipRepository) DeactivateMembership(ctx context.Context, userID, tenantID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"is_default": false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to deactivate membership: %w", err)
	}
	return nil
}

// HasAccess checks if a user has access to a tenant
// Supports both direct user_id match and keycloak_id lookup for backward compatibility
// with users who were created before the Keycloak ID sync was implemented
func (r *MembershipRepository) HasAccess(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
	// First, try direct user_id match (works for new users where local ID = Keycloak ID)
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
		Where("user_id = ? AND tenant_id = ? AND is_active = ?", userID, tenantID, true).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check access: %w", err)
	}
	if count > 0 {
		return true, nil
	}

	// If no direct match, check if userID is a Keycloak ID that maps to a local user
	// This handles cases where the BFF sends Keycloak ID but membership uses local user ID
	var localUser models.User
	if err := r.db.WithContext(ctx).
		Where("keycloak_id = ?", userID).
		First(&localUser).Error; err == nil {
		// Found a user with this keycloak_id, check membership with their local ID
		if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
			Where("user_id = ? AND tenant_id = ? AND is_active = ?", localUser.ID, tenantID, true).
			Count(&count).Error; err != nil {
			return false, fmt.Errorf("failed to check access by keycloak_id: %w", err)
		}
		return count > 0, nil
	}

	return false, nil
}

// HasAccessBySlug checks if a user has access to a tenant by slug
func (r *MembershipRepository) HasAccessBySlug(ctx context.Context, userID uuid.UUID, slug string) (bool, *models.Tenant, error) {
	tenant, err := r.GetTenantBySlug(ctx, slug)
	if err != nil {
		return false, nil, err
	}

	hasAccess, err := r.HasAccess(ctx, userID, tenant.ID)
	if err != nil {
		return false, nil, err
	}

	return hasAccess, tenant, nil
}

// GetUserRole retrieves the user's role within a tenant
// Supports both direct user_id match and keycloak_id lookup for backward compatibility
func (r *MembershipRepository) GetUserRole(ctx context.Context, userID, tenantID uuid.UUID) (string, error) {
	var membership models.UserTenantMembership

	// First, try direct user_id match
	if err := r.db.WithContext(ctx).
		Select("role").
		Where("user_id = ? AND tenant_id = ? AND is_active = ?", userID, tenantID, true).
		First(&membership).Error; err == nil {
		return membership.Role, nil
	}

	// If no direct match, check if userID is a Keycloak ID that maps to a local user
	var localUser models.User
	if err := r.db.WithContext(ctx).
		Where("keycloak_id = ?", userID).
		First(&localUser).Error; err == nil {
		// Found a user with this keycloak_id, get role with their local ID
		if err := r.db.WithContext(ctx).
			Select("role").
			Where("user_id = ? AND tenant_id = ? AND is_active = ?", localUser.ID, tenantID, true).
			First(&membership).Error; err == nil {
			return membership.Role, nil
		}
	}

	return "", fmt.Errorf("no active membership found")
}

// ============================================================================
// Invitation Operations
// ============================================================================

// CreateInvitation creates a membership with pending invitation status
func (r *MembershipRepository) CreateInvitation(ctx context.Context, tenantID, invitedBy uuid.UUID, email, role, token string, expiresAt time.Time) (*models.UserTenantMembership, error) {
	// Note: We create the membership with a placeholder user_id until they accept
	// In production, you might want to handle this differently
	now := time.Now()
	membership := &models.UserTenantMembership{
		UserID:              uuid.Nil, // Will be set when invitation is accepted
		TenantID:            tenantID,
		Role:                role,
		IsActive:            false, // Not active until accepted
		InvitedBy:           &invitedBy,
		InvitedAt:           &now,
		InvitationToken:     token,
		InvitationExpiresAt: &expiresAt,
	}

	if err := r.db.WithContext(ctx).Create(membership).Error; err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	return membership, nil
}

// GetInvitationByToken retrieves an invitation by its token
func (r *MembershipRepository) GetInvitationByToken(ctx context.Context, token string) (*models.UserTenantMembership, error) {
	var membership models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("invitation_token = ?", token).
		First(&membership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	return &membership, nil
}

// AcceptInvitation accepts an invitation and activates the membership
func (r *MembershipRepository) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*models.UserTenantMembership, error) {
	var membership models.UserTenantMembership
	if err := r.db.WithContext(ctx).
		Where("invitation_token = ?", token).
		First(&membership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invitation not found")
		}
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	// Check if expired
	if membership.InvitationExpiresAt != nil && membership.InvitationExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("invitation has expired")
	}

	// Check if already accepted
	if membership.AcceptedAt != nil {
		return nil, fmt.Errorf("invitation already accepted")
	}

	// Update membership
	now := time.Now()
	membership.UserID = userID
	membership.IsActive = true
	membership.AcceptedAt = &now
	membership.InvitationToken = "" // Clear token
	membership.UpdatedAt = now

	if err := r.db.WithContext(ctx).Save(&membership).Error; err != nil {
		return nil, fmt.Errorf("failed to accept invitation: %w", err)
	}

	return &membership, nil
}

// ============================================================================
// Activity Log Operations
// ============================================================================

// LogActivity creates an activity log entry
func (r *MembershipRepository) LogActivity(ctx context.Context, log *models.TenantActivityLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to log activity: %w", err)
	}
	return nil
}

// GetTenantActivityLog retrieves activity logs for a tenant
func (r *MembershipRepository) GetTenantActivityLog(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.TenantActivityLog, error) {
	var logs []models.TenantActivityLog
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get activity log: %w", err)
	}
	return logs, nil
}

// ============================================================================
// Statistics
// ============================================================================

// GetUserTenantCount returns the number of tenants a user has access to
func (r *MembershipRepository) GetUserTenantCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count user tenants: %w", err)
	}
	return count, nil
}

// GetTenantMemberCount returns the number of active members in a tenant
func (r *MembershipRepository) GetTenantMemberCount(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.UserTenantMembership{}).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count tenant members: %w", err)
	}
	return count, nil
}

// ============================================================================
// Platform Admin Operations
// ============================================================================

// GetAllTenants retrieves all tenants in the system (for platform admins)
func (r *MembershipRepository) GetAllTenants(ctx context.Context) ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := r.db.WithContext(ctx).
		Where("status != ?", "deleted").
		Order("created_at DESC").
		Find(&tenants).Error; err != nil {
		return nil, fmt.Errorf("failed to get all tenants: %w", err)
	}
	return tenants, nil
}
