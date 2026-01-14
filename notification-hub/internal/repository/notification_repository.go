package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"notification-hub/internal/models"
	"gorm.io/gorm"
)

// NotificationFilters holds filtering options for listing notifications
type NotificationFilters struct {
	IsRead     *bool
	Type       string
	Priority   string
	GroupKey   string
	EntityType string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// NotificationRepository defines the interface for notification data access
type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) (*models.Notification, error)
	List(ctx context.Context, tenantID string, userID uuid.UUID, filters NotificationFilters) ([]models.Notification, int64, error)
	MarkAsRead(ctx context.Context, tenantID string, userID uuid.UUID, ids []uuid.UUID) error
	MarkAsUnread(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error
	MarkAllAsRead(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error)
	GetUnreadCount(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error)
	Delete(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error
	DeleteAll(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error)
	ExistsBySourceEventID(ctx context.Context, sourceEventID string) (bool, error)
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

// Create creates a new notification
func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	if err := r.db.WithContext(ctx).Create(notification).Error; err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// GetByID retrieves a notification by ID with tenant and user isolation
func (r *notificationRepository) GetByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		First(&notification).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return &notification, nil
}

// List retrieves notifications with filtering and pagination
func (r *notificationRepository) List(ctx context.Context, tenantID string, userID uuid.UUID, filters NotificationFilters) ([]models.Notification, int64, error) {
	// Include both user-specific notifications AND broadcast notifications (uuid.Nil)
	// Only return in_app notifications (exclude email, push, sms which are handled by notification-service)
	query := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("tenant_id = ? AND (user_id = ? OR user_id = ?)", tenantID, userID, uuid.Nil).
		Where("channel = ?", "in_app").
		Where("is_archived = ?", false)

	// Apply filters
	if filters.IsRead != nil {
		query = query.Where("is_read = ?", *filters.IsRead)
	}
	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}
	if filters.Priority != "" {
		query = query.Where("priority = ?", filters.Priority)
	}
	if filters.GroupKey != "" {
		query = query.Where("group_key = ?", filters.GroupKey)
	}
	if filters.EntityType != "" {
		query = query.Where("entity_type = ?", filters.EntityType)
	}
	if filters.From != nil {
		query = query.Where("created_at >= ?", filters.From)
	}
	if filters.To != nil {
		query = query.Where("created_at <= ?", filters.To)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Apply pagination
	if filters.Limit < 1 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	// Fetch results
	var notifications []models.Notification
	err := query.
		Order("created_at DESC").
		Offset(filters.Offset).
		Limit(filters.Limit).
		Find(&notifications).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list notifications: %w", err)
	}

	return notifications, total, nil
}

// MarkAsRead marks notifications as read (including broadcast notifications)
func (r *notificationRepository) MarkAsRead(ctx context.Context, tenantID string, userID uuid.UUID, ids []uuid.UUID) error {
	now := time.Now()
	// Include both user-specific AND broadcast notifications (uuid.Nil)
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id IN ? AND tenant_id = ? AND (user_id = ? OR user_id = ?)", ids, tenantID, userID, uuid.Nil).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark notifications as read: %w", result.Error)
	}
	return nil
}

// MarkAsUnread marks a notification as unread
func (r *notificationRepository) MarkAsUnread(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		Updates(map[string]interface{}{
			"is_read": false,
			"read_at": nil,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark notification as unread: %w", result.Error)
	}
	return nil
}

// MarkAllAsRead marks all notifications as read for a user (including broadcast notifications)
func (r *notificationRepository) MarkAllAsRead(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error) {
	now := time.Now()
	// Include both user-specific AND broadcast notifications (uuid.Nil)
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("tenant_id = ? AND (user_id = ? OR user_id = ?) AND is_read = ?", tenantID, userID, uuid.Nil, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetUnreadCount returns the count of unread notifications for a user
func (r *notificationRepository) GetUnreadCount(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error) {
	var count int64
	// Include both user-specific AND broadcast notifications (uuid.Nil)
	// Only count in_app notifications (exclude email, push, sms)
	err := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("tenant_id = ? AND (user_id = ? OR user_id = ?) AND channel = ? AND is_read = ? AND is_archived = ?", tenantID, userID, uuid.Nil, "in_app", false, false).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// Delete soft-deletes (archives) a notification (including broadcast notifications)
func (r *notificationRepository) Delete(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error {
	now := time.Now()
	// Include both user-specific AND broadcast notifications (uuid.Nil)
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND tenant_id = ? AND (user_id = ? OR user_id = ?)", id, tenantID, userID, uuid.Nil).
		Updates(map[string]interface{}{
			"is_archived": true,
			"archived_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to delete notification: %w", result.Error)
	}
	return nil
}

// DeleteAll archives all notifications for a user (including broadcast notifications)
func (r *notificationRepository) DeleteAll(ctx context.Context, tenantID string, userID uuid.UUID) (int64, error) {
	now := time.Now()
	// Include both user-specific AND broadcast notifications (uuid.Nil)
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("tenant_id = ? AND (user_id = ? OR user_id = ?) AND is_archived = ?", tenantID, userID, uuid.Nil, false).
		Updates(map[string]interface{}{
			"is_archived": true,
			"archived_at": now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete all notifications: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ExistsBySourceEventID checks if a notification with the given source event ID exists (for deduplication)
func (r *notificationRepository) ExistsBySourceEventID(ctx context.Context, sourceEventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("source_event_id = ?", sourceEventID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check source event ID: %w", err)
	}
	return count > 0, nil
}
