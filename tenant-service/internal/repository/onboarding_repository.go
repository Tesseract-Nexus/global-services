package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"gorm.io/gorm"
)

// OnboardingRepository handles onboarding session operations
type OnboardingRepository struct {
	db *gorm.DB
}

// NewOnboardingRepository creates a new onboarding repository
func NewOnboardingRepository(db *gorm.DB) *OnboardingRepository {
	return &OnboardingRepository{
		db: db,
	}
}

// CreateSession creates a new onboarding session with associated data
func (r *OnboardingRepository) CreateSession(ctx context.Context, session *models.OnboardingSession) (*models.OnboardingSession, error) {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	// Create the session
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create onboarding session: %w", err)
	}

	return session, nil
}

// GetSessionByID retrieves an onboarding session by ID with related data
func (r *OnboardingRepository) GetSessionByID(ctx context.Context, id uuid.UUID, includeRelations []string) (*models.OnboardingSession, error) {
	var session models.OnboardingSession

	query := r.db.WithContext(ctx)

	// Add preloads based on includeRelations
	for _, relation := range includeRelations {
		switch relation {
		case "template":
			query = query.Preload("Template")
		case "business_information":
			query = query.Preload("BusinessInformation")
		case "contact_information":
			query = query.Preload("ContactInformation")
		case "business_addresses":
			query = query.Preload("BusinessAddresses")
		case "verification_records":
			query = query.Preload("VerificationRecords")
		case "payment_information":
			query = query.Preload("PaymentInformation")
		case "tasks":
			query = query.Preload("Tasks")
		case "application_configurations":
			query = query.Preload("ApplicationConfigurations")
		}
	}

	if err := query.First(&session, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("onboarding session not found")
		}
		return nil, fmt.Errorf("failed to get onboarding session: %w", err)
	}

	return &session, nil
}

// UpdateSession updates an onboarding session
func (r *OnboardingRepository) UpdateSession(ctx context.Context, session *models.OnboardingSession) (*models.OnboardingSession, error) {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return nil, fmt.Errorf("failed to update onboarding session: %w", err)
	}

	return session, nil
}

// ListSessions lists onboarding sessions with pagination
func (r *OnboardingRepository) ListSessions(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]models.OnboardingSession, int64, error) {
	var sessions []models.OnboardingSession
	var total int64

	query := r.db.WithContext(ctx).Model(&models.OnboardingSession{})

	// Apply filters
	for field, value := range filters {
		switch field {
		case "application_type":
			query = query.Where("application_type = ?", value)
		case "status":
			query = query.Where("status = ?", value)
		case "tenant_id":
			if value != nil {
				query = query.Where("tenant_id = ?", value)
			}
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count onboarding sessions: %w", err)
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list onboarding sessions: %w", err)
	}

	return sessions, total, nil
}

// DeleteSession deletes an onboarding session
func (r *OnboardingRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.OnboardingSession{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete onboarding session: %w", err)
	}

	return nil
}

// GetSessionsByStatus retrieves sessions by status
func (r *OnboardingRepository) GetSessionsByStatus(ctx context.Context, status string) ([]models.OnboardingSession, error) {
	var sessions []models.OnboardingSession

	if err := r.db.WithContext(ctx).Where("status = ?", status).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get sessions by status: %w", err)
	}

	return sessions, nil
}

// GetExpiredSessions retrieves expired sessions
func (r *OnboardingRepository) GetExpiredSessions(ctx context.Context) ([]models.OnboardingSession, error) {
	var sessions []models.OnboardingSession

	if err := r.db.WithContext(ctx).Where("expires_at < ? AND status NOT IN (?)",
		"NOW()", []string{"completed", "failed", "abandoned"}).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get expired sessions: %w", err)
	}

	return sessions, nil
}

// GetCompletedSessionByTenantID retrieves the completed onboarding session for a tenant
// with all related business data (business info, contacts, addresses).
// This is used to pre-populate settings pages with data collected during onboarding.
// Returns nil if no completed session exists for the tenant.
func (r *OnboardingRepository) GetCompletedSessionByTenantID(ctx context.Context, tenantID uuid.UUID) (*models.OnboardingSession, error) {
	var session models.OnboardingSession

	// Query for completed session belonging to this tenant
	// Preload all business-related data needed for settings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, "completed").
		Preload("BusinessInformation").
		Preload("ContactInformation").
		Preload("BusinessAddresses").
		Order("completed_at DESC"). // Get the most recent completed session
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		// No completed session found - this is not an error, just no data
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get onboarding session for tenant: %w", err)
	}

	return &session, nil
}

// GetPendingSessionByEmail finds a pending onboarding session by primary contact email
// Used for resending verification emails when the token has expired
func (r *OnboardingRepository) GetPendingSessionByEmail(ctx context.Context, email string) (*models.OnboardingSession, error) {
	var session models.OnboardingSession

	// Join with contact_information to find session by email
	// Only return sessions that are pending or in_progress (not completed/failed)
	err := r.db.WithContext(ctx).
		Joins("JOIN contact_information ON contact_information.onboarding_session_id = onboarding_sessions.id").
		Where("LOWER(contact_information.email) = LOWER(?) AND contact_information.is_primary = true", email).
		Where("onboarding_sessions.status IN ?", []string{"pending", "in_progress"}).
		Preload("ContactInformation").
		Preload("BusinessInformation").
		Order("onboarding_sessions.created_at DESC"). // Get the most recent session
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil // No session found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by email: %w", err)
	}

	return &session, nil
}

// UpdateSessionProgress updates session progress and marks draft as updated
func (r *OnboardingRepository) UpdateSessionProgress(ctx context.Context, sessionID uuid.UUID, currentStep string, progressPercentage int) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"current_step":        currentStep,
			"progress_percentage": progressPercentage,
			"status":              "in_progress",
			"draft_saved_at":      now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update session progress: %w", err)
	}

	return nil
}

// BusinessInformationRepository handles business information operations
type BusinessInformationRepository struct {
	db *gorm.DB
}

// NewBusinessInformationRepository creates a new business information repository
func NewBusinessInformationRepository(db *gorm.DB) *BusinessInformationRepository {
	return &BusinessInformationRepository{
		db: db,
	}
}

// CreateOrUpdate creates or updates business information for a session
func (r *BusinessInformationRepository) CreateOrUpdate(ctx context.Context, sessionID uuid.UUID, businessInfo *models.BusinessInformation) (*models.BusinessInformation, error) {
	businessInfo.OnboardingSessionID = sessionID

	// Check if business info already exists for this session
	var existing models.BusinessInformation
	err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new
		if businessInfo.ID == uuid.Nil {
			businessInfo.ID = uuid.New()
		}
		if err := r.db.WithContext(ctx).Create(businessInfo).Error; err != nil {
			return nil, fmt.Errorf("failed to create business information: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to check existing business information: %w", err)
	} else {
		// Update existing
		businessInfo.ID = existing.ID
		if err := r.db.WithContext(ctx).Save(businessInfo).Error; err != nil {
			return nil, fmt.Errorf("failed to update business information: %w", err)
		}
	}

	return businessInfo, nil
}

// GetBySessionID retrieves business information by session ID
func (r *BusinessInformationRepository) GetBySessionID(ctx context.Context, sessionID uuid.UUID) (*models.BusinessInformation, error) {
	var businessInfo models.BusinessInformation

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).First(&businessInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("business information not found for session")
		}
		return nil, fmt.Errorf("failed to get business information: %w", err)
	}

	return &businessInfo, nil
}

// ContactInformationRepository handles contact information operations
type ContactInformationRepository struct {
	db *gorm.DB
}

// NewContactInformationRepository creates a new contact information repository
func NewContactInformationRepository(db *gorm.DB) *ContactInformationRepository {
	return &ContactInformationRepository{
		db: db,
	}
}

// CreateContact creates a new contact for a session
func (r *ContactInformationRepository) CreateContact(ctx context.Context, sessionID uuid.UUID, contact *models.ContactInformation) (*models.ContactInformation, error) {
	contact.OnboardingSessionID = sessionID
	if contact.ID == uuid.Nil {
		contact.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(contact).Error; err != nil {
		return nil, fmt.Errorf("failed to create contact information: %w", err)
	}

	return contact, nil
}

// GetBySessionID retrieves all contacts for a session
func (r *ContactInformationRepository) GetBySessionID(ctx context.Context, sessionID uuid.UUID) ([]models.ContactInformation, error) {
	var contacts []models.ContactInformation

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("failed to get contact information: %w", err)
	}

	return contacts, nil
}

// GetPrimaryContact retrieves the primary contact for a session
func (r *ContactInformationRepository) GetPrimaryContact(ctx context.Context, sessionID uuid.UUID) (*models.ContactInformation, error) {
	var contact models.ContactInformation

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND is_primary_contact = ?", sessionID, true).First(&contact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("primary contact not found for session")
		}
		return nil, fmt.Errorf("failed to get primary contact: %w", err)
	}

	return &contact, nil
}
