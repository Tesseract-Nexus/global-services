package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationPriority represents the priority level of a notification
type NotificationPriority string

const (
	PriorityLow    NotificationPriority = "low"
	PriorityNormal NotificationPriority = "normal"
	PriorityHigh   NotificationPriority = "high"
	PriorityUrgent NotificationPriority = "urgent"
)

// JSONB represents a JSONB field in PostgreSQL
type JSONB map[string]interface{}

// Value returns the JSON-encoded value for database storage
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan reads a JSON-encoded value from the database
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Notification represents an in-app notification
type Notification struct {
	ID            uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string               `json:"tenantId" gorm:"column:tenant_id;type:varchar(255);not null;index:idx_notifications_tenant_user"`
	UserID        uuid.UUID            `json:"userId" gorm:"column:user_id;type:uuid;not null;index:idx_notifications_tenant_user"`
	Channel       string               `json:"channel" gorm:"column:channel;type:varchar(50);not null;default:'in_app'"` // in_app, push, email, sms
	Type          string               `json:"type" gorm:"type:varchar(100);not null;index"`                             // order.created, payment.received
	Title         string               `json:"title" gorm:"type:varchar(500);not null"`
	Message       string               `json:"message,omitempty" gorm:"type:text"`
	Icon          string               `json:"icon,omitempty" gorm:"type:varchar(255)"`
	ActionURL     string               `json:"actionUrl,omitempty" gorm:"column:action_url;type:varchar(2048)"`
	SourceService string               `json:"sourceService" gorm:"column:source_service;type:varchar(100);not null"`
	SourceEventID string               `json:"sourceEventId,omitempty" gorm:"column:source_event_id;type:varchar(255);index"`
	EntityType    string               `json:"entityType,omitempty" gorm:"column:entity_type;type:varchar(100)"`
	EntityID      *uuid.UUID           `json:"entityId,omitempty" gorm:"column:entity_id;type:uuid"`
	Metadata      JSONB                `json:"metadata,omitempty" gorm:"type:jsonb"`
	GroupKey      string               `json:"groupKey,omitempty" gorm:"column:group_key;type:varchar(255);index"`
	GroupCount    int                  `json:"groupCount" gorm:"column:group_count;default:1"`
	IsRead        bool                 `json:"isRead" gorm:"column:is_read;default:false;index:idx_notifications_unread"`
	ReadAt        *time.Time           `json:"readAt,omitempty" gorm:"column:read_at"`
	IsArchived    bool                 `json:"isArchived" gorm:"column:is_archived;default:false"`
	ArchivedAt    *time.Time           `json:"archivedAt,omitempty" gorm:"column:archived_at"`
	Priority      NotificationPriority `json:"priority" gorm:"type:varchar(20);default:'normal'"`
	CreatedAt     time.Time            `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time            `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	ExpiresAt     *time.Time           `json:"expiresAt,omitempty" gorm:"column:expires_at;index"`
}

// TableName returns the table name for the Notification model
func (Notification) TableName() string {
	return "notifications"
}

// BeforeCreate sets default values before creating a notification
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.Priority == "" {
		n.Priority = PriorityNormal
	}
	return nil
}

// MarkAsRead marks the notification as read
func (n *Notification) MarkAsRead() {
	now := time.Now()
	n.IsRead = true
	n.ReadAt = &now
}

// MarkAsUnread marks the notification as unread
func (n *Notification) MarkAsUnread() {
	n.IsRead = false
	n.ReadAt = nil
}

// ToJSON converts the notification to JSON bytes
func (n *Notification) ToJSON() []byte {
	data, _ := json.Marshal(n)
	return data
}

// NotificationResponse is the API response wrapper for a notification
type NotificationResponse struct {
	Success bool          `json:"success"`
	Data    *Notification `json:"data,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// NotificationListResponse is the API response for a list of notifications
type NotificationListResponse struct {
	Success     bool           `json:"success"`
	Data        []Notification `json:"data"`
	Pagination  *Pagination    `json:"pagination,omitempty"`
	UnreadCount int64          `json:"unreadCount"`
}

// Pagination holds pagination info
type Pagination struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

// UnreadCountResponse is the API response for unread count
type UnreadCountResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
}
