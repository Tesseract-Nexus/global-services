package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/config"
	"tenant-service/internal/models"
	"tenant-service/internal/redis"
	"gorm.io/gorm"
)

// DraftService handles draft persistence for onboarding forms
type DraftService struct {
	db              *gorm.DB
	redisClient     *redis.Client
	config          config.DraftConfig
	notificationSvc *NotificationService
}

// NewDraftService creates a new draft service
func NewDraftService(db *gorm.DB, redisClient *redis.Client, cfg config.DraftConfig, notificationSvc *NotificationService) *DraftService {
	return &DraftService{
		db:              db,
		redisClient:     redisClient,
		config:          cfg,
		notificationSvc: notificationSvc,
	}
}

// SaveDraftRequest represents a request to save draft data
type SaveDraftRequest struct {
	SessionID   uuid.UUID              `json:"session_id" validate:"required"`
	FormData    map[string]interface{} `json:"form_data" validate:"required"`
	CurrentStep int                    `json:"current_step"`
	Progress    int                    `json:"progress"`
	Email       string                 `json:"email,omitempty"`
	Phone       string                 `json:"phone,omitempty"`
}

// SaveDraftResponse represents the response after saving a draft
type SaveDraftResponse struct {
	SessionID uuid.UUID `json:"session_id"`
	SavedAt   time.Time `json:"saved_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Message   string    `json:"message"`
}

// SaveDraft saves form data as a draft
func (s *DraftService) SaveDraft(ctx context.Context, req *SaveDraftRequest) (*SaveDraftResponse, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(s.config.ExpiryHours) * time.Hour)
	ttl := time.Duration(s.config.ExpiryHours) * time.Hour

	// Save to Redis for fast access
	draftData := &redis.DraftData{
		SessionID:   req.SessionID.String(),
		FormData:    req.FormData,
		CurrentStep: req.CurrentStep,
		Progress:    req.Progress,
		Email:       req.Email,
		Phone:       req.Phone,
	}

	if err := s.redisClient.SaveDraft(ctx, req.SessionID.String(), draftData, ttl); err != nil {
		log.Printf("Warning: Failed to save draft to Redis: %v", err)
		// Continue to save to PostgreSQL even if Redis fails
	}

	// Save to PostgreSQL for persistence
	formDataJSON, err := models.NewJSONB(req.FormData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize form data: %w", err)
	}

	// Update onboarding session with draft data
	updates := map[string]interface{}{
		"draft_saved_at":      now,
		"draft_expires_at":    expiresAt,
		"draft_form_data":     formDataJSON,
		"current_step":        fmt.Sprintf("step_%d", req.CurrentStep),
		"progress_percentage": req.Progress,
		"status":              "in_progress",
	}

	// Check if this is the first draft save (set initial expiry)
	var session models.OnboardingSession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", req.SessionID).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Only set draft_expires_at on first save
	if session.DraftExpiresAt == nil {
		updates["draft_expires_at"] = expiresAt
	}

	if err := s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", req.SessionID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to save draft to database: %w", err)
	}

	return &SaveDraftResponse{
		SessionID: req.SessionID,
		SavedAt:   now,
		ExpiresAt: expiresAt,
		Message:   "Draft saved successfully",
	}, nil
}

// GetDraftResponse represents the response when retrieving a draft
type GetDraftResponse struct {
	SessionID          uuid.UUID              `json:"session_id"`
	FormData           map[string]interface{} `json:"form_data"`
	CurrentStep        int                    `json:"current_step"`
	Progress           int                    `json:"progress"`
	SavedAt            *time.Time             `json:"saved_at"`
	ExpiresAt          *time.Time             `json:"expires_at"`
	Found              bool                   `json:"found"`
	TimeRemainingHours float64                `json:"time_remaining_hours,omitempty"`
}

// GetDraft retrieves draft data for a session
func (s *DraftService) GetDraft(ctx context.Context, sessionID uuid.UUID) (*GetDraftResponse, error) {
	// Try Redis first for faster access
	draftData, err := s.redisClient.GetDraft(ctx, sessionID.String())
	if err == nil && draftData != nil {
		// Verify the session still exists in PostgreSQL and is not completed
		var session models.OnboardingSession
		if checkErr := s.db.WithContext(ctx).
			Select("id", "status").
			Where("id = ?", sessionID).
			First(&session).Error; checkErr != nil {
			// Session doesn't exist in PostgreSQL, clean up stale Redis cache
			log.Printf("[DraftService] Cleaning up stale Redis cache for session %s (not found in DB)", sessionID)
			if delErr := s.redisClient.DeleteDraft(ctx, sessionID.String()); delErr != nil {
				log.Printf("Warning: Failed to delete stale draft from Redis: %v", delErr)
			}
			return &GetDraftResponse{Found: false}, nil
		}

		// Check if session is completed - clean up Redis and return not found
		if session.Status == "completed" {
			log.Printf("[DraftService] Session %s is completed, cleaning up Redis cache", sessionID)
			if delErr := s.redisClient.DeleteDraft(ctx, sessionID.String()); delErr != nil {
				log.Printf("Warning: Failed to delete completed session draft from Redis: %v", delErr)
			}
			return &GetDraftResponse{Found: false}, nil
		}

		// Session exists and is not completed, return Redis data
		return &GetDraftResponse{
			SessionID:   sessionID,
			FormData:    draftData.FormData,
			CurrentStep: draftData.CurrentStep,
			Progress:    draftData.Progress,
			SavedAt:     &draftData.LastSavedAt,
			Found:       true,
		}, nil
	}

	// Fall back to PostgreSQL
	var session models.OnboardingSession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", sessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &GetDraftResponse{Found: false}, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session has expired (main session expiry, not draft expiry)
	if session.ExpiresAt.Before(time.Now()) {
		log.Printf("[DraftService] Session %s has expired", sessionID)
		return &GetDraftResponse{Found: false}, nil
	}

	// Check if session is already completed - don't return draft for completed sessions
	if session.Status == "completed" {
		log.Printf("[DraftService] Session %s is already completed, not returning draft", sessionID)
		return &GetDraftResponse{Found: false}, nil
	}

	// Parse form data from JSONB (could be nil for new sessions)
	var formData map[string]interface{}
	if len(session.DraftFormData) > 0 {
		if err := json.Unmarshal(session.DraftFormData, &formData); err != nil {
			return nil, fmt.Errorf("failed to parse form data: %w", err)
		}
	}

	// Parse current step - convert step name to step number
	currentStep := 0
	switch session.CurrentStep {
	case "business_information":
		currentStep = 0
	case "contact_information":
		currentStep = 1
	case "business_address":
		currentStep = 2
	case "verification":
		currentStep = 3
	default:
		// Try parsing step_N format as fallback
		fmt.Sscanf(session.CurrentStep, "step_%d", &currentStep)
	}

	// Calculate time remaining based on session expiry (7 days)
	timeRemaining := time.Until(session.ExpiresAt)
	timeRemainingHours := timeRemaining.Hours()

	// For new sessions without draft data, still return Found: true
	// because the session exists and is valid
	return &GetDraftResponse{
		SessionID:          sessionID,
		FormData:           formData,
		CurrentStep:        currentStep,
		Progress:           session.ProgressPercentage,
		SavedAt:            session.DraftSavedAt,
		ExpiresAt:          &session.ExpiresAt,
		Found:              true, // Session exists, return found even without draft data
		TimeRemainingHours: timeRemainingHours,
	}, nil
}

// ProcessHeartbeat updates the heartbeat for a session (user is still active)
func (s *DraftService) ProcessHeartbeat(ctx context.Context, sessionID uuid.UUID) error {
	ttl := time.Duration(s.config.ExpiryHours) * time.Hour

	// Update Redis heartbeat
	if err := s.redisClient.UpdateHeartbeat(ctx, sessionID.String(), ttl); err != nil {
		log.Printf("Warning: Failed to update heartbeat in Redis: %v", err)
	}

	// Clear browser_closed_at since user is active
	return s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", sessionID).
		Update("browser_closed_at", nil).Error
}

// MarkBrowserClosed marks that the browser was closed for a session
func (s *DraftService) MarkBrowserClosed(ctx context.Context, sessionID uuid.UUID) error {
	now := time.Now()

	// Update Redis
	if err := s.redisClient.MarkBrowserClosed(ctx, sessionID.String()); err != nil {
		log.Printf("Warning: Failed to mark browser closed in Redis: %v", err)
	}

	// Update PostgreSQL
	return s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", sessionID).
		Update("browser_closed_at", now).Error
}

// DeleteDraft removes draft data for a session
func (s *DraftService) DeleteDraft(ctx context.Context, sessionID uuid.UUID) error {
	// Delete from Redis
	if err := s.redisClient.DeleteDraft(ctx, sessionID.String()); err != nil {
		log.Printf("Warning: Failed to delete draft from Redis: %v", err)
	}

	// Clear draft fields in PostgreSQL (but keep the session)
	updates := map[string]interface{}{
		"draft_saved_at":    nil,
		"draft_expires_at":  nil,
		"draft_form_data":   nil,
		"browser_closed_at": nil,
		"reminder_count":    0,
		"last_reminder_at":  nil,
	}

	return s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error
}

// CleanupExpiredDrafts removes expired drafts
func (s *DraftService) CleanupExpiredDrafts(ctx context.Context) (int64, error) {
	now := time.Now()

	// Find and mark expired sessions as abandoned
	result := s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("draft_expires_at IS NOT NULL AND draft_expires_at < ?", now).
		Where("status IN ?", []string{"started", "in_progress"}).
		Updates(map[string]interface{}{
			"status":            "abandoned",
			"draft_saved_at":    nil,
			"draft_expires_at":  nil,
			"draft_form_data":   nil,
			"browser_closed_at": nil,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup expired drafts: %w", result.Error)
	}

	log.Printf("Cleaned up %d expired drafts", result.RowsAffected)
	return result.RowsAffected, nil
}

// GetDraftsNeedingReminder returns sessions that need reminder emails
func (s *DraftService) GetDraftsNeedingReminder(ctx context.Context) ([]models.OnboardingSession, error) {
	now := time.Now()
	reminderInterval := time.Duration(s.config.ReminderInterval) * time.Hour

	var sessions []models.OnboardingSession

	// Find sessions where:
	// 1. Draft is saved and not expired
	// 2. Browser was closed
	// 3. Reminder count < max reminders
	// 4. Last reminder was > reminder interval ago (or never sent)
	err := s.db.WithContext(ctx).
		Preload("ContactInformation").
		Where("draft_saved_at IS NOT NULL").
		Where("draft_expires_at > ?", now).
		Where("browser_closed_at IS NOT NULL").
		Where("reminder_count < ?", s.config.MaxReminders).
		Where("last_reminder_at IS NULL OR last_reminder_at < ?", now.Add(-reminderInterval)).
		Where("status IN ?", []string{"started", "in_progress"}).
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get drafts needing reminder: %w", err)
	}

	return sessions, nil
}

// SendDraftReminder sends a reminder email for a draft session
func (s *DraftService) SendDraftReminder(ctx context.Context, session *models.OnboardingSession) error {
	// Get primary contact email
	var email string
	for _, contact := range session.ContactInformation {
		if contact.IsPrimaryContact {
			email = contact.Email
			break
		}
	}

	if email == "" && len(session.ContactInformation) > 0 {
		email = session.ContactInformation[0].Email
	}

	if email == "" {
		log.Printf("No email found for session %s, skipping reminder", session.ID)
		return nil
	}

	// Get name from contact
	var firstName string
	for _, contact := range session.ContactInformation {
		if contact.IsPrimaryContact {
			firstName = contact.FirstName
			break
		}
	}

	// Send reminder email
	if err := s.notificationSvc.SendDraftReminderEmail(ctx, email, firstName, session.ID.String()); err != nil {
		return fmt.Errorf("failed to send reminder email: %w", err)
	}

	// Update reminder count and last reminder time
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", session.ID).
		Updates(map[string]interface{}{
			"reminder_count":   gorm.Expr("reminder_count + 1"),
			"last_reminder_at": now,
		}).Error
}

// ProcessReminders sends reminders for all drafts that need them
func (s *DraftService) ProcessReminders(ctx context.Context) (int, error) {
	sessions, err := s.GetDraftsNeedingReminder(ctx)
	if err != nil {
		return 0, err
	}

	sentCount := 0
	for _, session := range sessions {
		if err := s.SendDraftReminder(ctx, &session); err != nil {
			log.Printf("Failed to send reminder for session %s: %v", session.ID, err)
			continue
		}
		sentCount++
	}

	log.Printf("Sent %d reminder emails", sentCount)
	return sentCount, nil
}
