package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-service/internal/models"
	"gorm.io/gorm"
)

// NotificationRepository handles notification database operations
type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error)
	List(ctx context.Context, tenantID string, filters NotificationFilters) ([]models.Notification, int64, error)
	Update(ctx context.Context, notification *models.Notification) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.NotificationStatus, providerID string, errorMsg string) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetPending(ctx context.Context, limit int) ([]models.Notification, error)
	GetScheduledReady(ctx context.Context, limit int) ([]models.Notification, error)
	GetByRecipient(ctx context.Context, tenantID string, recipientID uuid.UUID, channel models.NotificationChannel) ([]models.Notification, error)
}

// NotificationFilters for listing notifications
type NotificationFilters struct {
	Channel    string
	Status     string
	TemplateID string
	Recipient  string
	FromDate   *time.Time
	ToDate     *time.Time
	Limit      int
	Offset     int
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *notificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&notification).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &notification, nil
}

func (r *notificationRepository) List(ctx context.Context, tenantID string, filters NotificationFilters) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if filters.Channel != "" {
		query = query.Where("channel = ?", filters.Channel)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.TemplateID != "" {
		query = query.Where("template_id = ?", filters.TemplateID)
	}
	if filters.Recipient != "" {
		query = query.Where("recipient_email = ? OR recipient_phone = ?", filters.Recipient, filters.Recipient)
	}
	if filters.FromDate != nil {
		query = query.Where("created_at >= ?", filters.FromDate)
	}
	if filters.ToDate != nil {
		query = query.Where("created_at <= ?", filters.ToDate)
	}

	// Count total
	if err := query.Model(&models.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	err := query.Order("created_at DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&notifications).Error

	return notifications, total, err
}

func (r *notificationRepository) Update(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Save(notification).Error
}

func (r *notificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.NotificationStatus, providerID string, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if providerID != "" {
		updates["provider_id"] = providerID
	}

	switch status {
	case models.StatusSent:
		now := time.Now()
		updates["sent_at"] = &now
	case models.StatusDelivered:
		now := time.Now()
		updates["delivered_at"] = &now
	case models.StatusFailed:
		now := time.Now()
		updates["failed_at"] = &now
		updates["error_message"] = errorMsg
	}

	return r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *notificationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Notification{}, id).Error
}

func (r *notificationRepository) GetPending(ctx context.Context, limit int) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.WithContext(ctx).
		Where("status = ? AND (scheduled_for IS NULL OR scheduled_for <= ?)", models.StatusPending, time.Now()).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func (r *notificationRepository) GetScheduledReady(ctx context.Context, limit int) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.WithContext(ctx).
		Where("status = ? AND scheduled_for <= ?", models.StatusQueued, time.Now()).
		Order("scheduled_for ASC").
		Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func (r *notificationRepository) GetByRecipient(ctx context.Context, tenantID string, recipientID uuid.UUID, channel models.NotificationChannel) ([]models.Notification, error) {
	var notifications []models.Notification
	query := r.db.WithContext(ctx).Where("tenant_id = ? AND recipient_id = ?", tenantID, recipientID)
	if channel != "" {
		query = query.Where("channel = ?", channel)
	}
	err := query.Order("created_at DESC").Limit(100).Find(&notifications).Error
	return notifications, err
}
