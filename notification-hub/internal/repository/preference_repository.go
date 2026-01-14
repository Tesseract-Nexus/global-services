package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"notification-hub/internal/models"
	"gorm.io/gorm"
)

// PreferenceRepository defines the interface for notification preference data access
type PreferenceRepository interface {
	Get(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error)
	GetOrCreate(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error)
	Update(ctx context.Context, preference *models.NotificationPreference) error
	Delete(ctx context.Context, tenantID string, userID uuid.UUID) error
}

type preferenceRepository struct {
	db *gorm.DB
}

// NewPreferenceRepository creates a new preference repository
func NewPreferenceRepository(db *gorm.DB) PreferenceRepository {
	return &preferenceRepository{db: db}
}

// Get retrieves notification preferences for a user
func (r *preferenceRepository) Get(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error) {
	var preference models.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&preference).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil // Return nil without error if not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get preferences: %w", err)
	}
	return &preference, nil
}

// GetOrCreate retrieves preferences or creates default ones if they don't exist
func (r *preferenceRepository) GetOrCreate(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error) {
	preference, err := r.Get(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	if preference != nil {
		return preference, nil
	}

	// Create default preferences
	preference = models.GetDefaultPreferences(tenantID, userID)
	if err := r.db.WithContext(ctx).Create(preference).Error; err != nil {
		// Handle race condition - another request might have created it
		if existingPref, getErr := r.Get(ctx, tenantID, userID); getErr == nil && existingPref != nil {
			return existingPref, nil
		}
		return nil, fmt.Errorf("failed to create preferences: %w", err)
	}

	return preference, nil
}

// Update updates notification preferences
func (r *preferenceRepository) Update(ctx context.Context, preference *models.NotificationPreference) error {
	result := r.db.WithContext(ctx).Save(preference)
	if result.Error != nil {
		return fmt.Errorf("failed to update preferences: %w", result.Error)
	}
	return nil
}

// Delete removes notification preferences for a user
func (r *preferenceRepository) Delete(ctx context.Context, tenantID string, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.NotificationPreference{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete preferences: %w", result.Error)
	}
	return nil
}
