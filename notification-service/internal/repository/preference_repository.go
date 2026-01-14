package repository

import (
	"context"

	"github.com/google/uuid"
	"notification-service/internal/models"
	"gorm.io/gorm"
)

// PreferenceRepository handles notification preference database operations
type PreferenceRepository interface {
	GetByUserID(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error)
	Upsert(ctx context.Context, pref *models.NotificationPreference) error
	UpdatePushTokens(ctx context.Context, tenantID string, userID uuid.UUID, tokens []string) error
}

type preferenceRepository struct {
	db *gorm.DB
}

// NewPreferenceRepository creates a new preference repository
func NewPreferenceRepository(db *gorm.DB) PreferenceRepository {
	return &preferenceRepository{db: db}
}

func (r *preferenceRepository) GetByUserID(ctx context.Context, tenantID string, userID uuid.UUID) (*models.NotificationPreference, error) {
	var pref models.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default preferences
			return &models.NotificationPreference{
				TenantID:         tenantID,
				UserID:           userID,
				EmailEnabled:     true,
				SMSEnabled:       true,
				PushEnabled:      true,
				MarketingEnabled: true,
				OrdersEnabled:    true,
				SecurityEnabled:  true,
			}, nil
		}
		return nil, err
	}
	return &pref, nil
}

func (r *preferenceRepository) Upsert(ctx context.Context, pref *models.NotificationPreference) error {
	return r.db.WithContext(ctx).Save(pref).Error
}

func (r *preferenceRepository) UpdatePushTokens(ctx context.Context, tenantID string, userID uuid.UUID, tokens []string) error {
	return r.db.WithContext(ctx).
		Model(&models.NotificationPreference{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("push_tokens", tokens).Error
}
