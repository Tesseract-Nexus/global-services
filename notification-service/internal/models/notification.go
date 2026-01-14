package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationChannel represents the delivery channel
type NotificationChannel string

const (
	ChannelEmail NotificationChannel = "EMAIL"
	ChannelSMS   NotificationChannel = "SMS"
	ChannelPush  NotificationChannel = "PUSH"
	ChannelInApp NotificationChannel = "IN_APP"
)

// NotificationStatus represents the delivery status
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "PENDING"
	StatusQueued    NotificationStatus = "QUEUED"
	StatusSending   NotificationStatus = "SENDING"
	StatusSent      NotificationStatus = "SENT"
	StatusDelivered NotificationStatus = "DELIVERED"
	StatusFailed    NotificationStatus = "FAILED"
	StatusBounced   NotificationStatus = "BOUNCED"
	StatusCancelled NotificationStatus = "CANCELLED"
)

// NotificationPriority represents message priority
type NotificationPriority string

const (
	PriorityLow      NotificationPriority = "LOW"
	PriorityNormal   NotificationPriority = "NORMAL"
	PriorityHigh     NotificationPriority = "HIGH"
	PriorityCritical NotificationPriority = "CRITICAL"
)

// Notification represents a notification message
type Notification struct {
	ID             uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string               `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	UserID         uuid.UUID            `json:"userId" gorm:"type:uuid;not null;index"`
	Type           string               `json:"type" gorm:"type:varchar(100);not null;index"`
	Title          string               `json:"title" gorm:"type:varchar(500);not null"`
	Message        string               `json:"message" gorm:"type:text"`
	Icon           string               `json:"icon" gorm:"type:varchar(255)"`
	ActionURL      string               `json:"actionUrl" gorm:"type:varchar(2048)"`
	SourceService  string               `json:"sourceService" gorm:"type:varchar(100);not null"`
	SourceEventID  string               `json:"sourceEventId" gorm:"type:varchar(255);index"`
	EntityType     string               `json:"entityType" gorm:"type:varchar(100)"`
	EntityID       *uuid.UUID           `json:"entityId" gorm:"type:uuid"`
	GroupKey       string               `json:"groupKey" gorm:"type:varchar(255);index"`
	GroupCount     int                  `json:"groupCount" gorm:"default:1"`
	IsRead         bool                 `json:"isRead" gorm:"default:false;index"`
	ReadAt         *time.Time           `json:"readAt"`
	IsArchived     bool                 `json:"isArchived" gorm:"default:false"`
	ArchivedAt     *time.Time           `json:"archivedAt"`
	Channel        NotificationChannel  `json:"channel" gorm:"type:varchar(20);not null;index"`
	Status         NotificationStatus   `json:"status" gorm:"type:varchar(20);not null;default:'PENDING';index"`
	Priority       NotificationPriority `json:"priority" gorm:"type:varchar(20);default:'NORMAL'"`

	// Template information
	TemplateID     *uuid.UUID           `json:"templateId" gorm:"type:uuid;index"`
	TemplateName   string               `json:"templateName" gorm:"type:varchar(255)"`

	// Recipient information
	RecipientID    *uuid.UUID           `json:"recipientId" gorm:"type:uuid;index"` // User ID if applicable
	RecipientEmail string               `json:"recipientEmail" gorm:"type:varchar(255);index"`
	RecipientPhone string               `json:"recipientPhone" gorm:"type:varchar(50)"`
	RecipientToken string               `json:"recipientToken" gorm:"type:text"` // FCM token for push

	// Message content
	Subject        string               `json:"subject" gorm:"type:varchar(500)"`
	Body           string               `json:"body" gorm:"type:text"`
	BodyHTML       string               `json:"bodyHtml" gorm:"type:text"`
	Variables      datatypes.JSON       `json:"variables" gorm:"type:jsonb"` // Template variables
	Metadata       datatypes.JSON       `json:"metadata" gorm:"type:jsonb"`  // Additional metadata

	// Delivery tracking
	ScheduledFor   *time.Time           `json:"scheduledFor"` // For scheduled notifications
	SentAt         *time.Time           `json:"sentAt"`
	DeliveredAt    *time.Time           `json:"deliveredAt"`
	FailedAt       *time.Time           `json:"failedAt"`
	ErrorMessage   string               `json:"errorMessage" gorm:"type:text"`
	RetryCount     int                  `json:"retryCount" gorm:"default:0"`
	MaxRetries     int                  `json:"maxRetries" gorm:"default:3"`

	// Provider information
	Provider       string               `json:"provider" gorm:"type:varchar(100)"` // sendgrid, twilio, fcm, etc.
	ProviderID     string               `json:"providerId" gorm:"type:varchar(255)"` // External provider message ID
	ProviderData   datatypes.JSON       `json:"providerData" gorm:"type:jsonb"`

	// Tracking
	OpenedAt       *time.Time           `json:"openedAt"`
	ClickedAt      *time.Time           `json:"clickedAt"`
	UnsubscribedAt *time.Time           `json:"unsubscribedAt"`

	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt       `json:"-" gorm:"index"`
}

// NotificationTemplate represents a reusable notification template
type NotificationTemplate struct {
	ID             uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string               `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Name           string               `json:"name" gorm:"type:varchar(255);not null;uniqueIndex:idx_template_name_tenant"`
	Description    string               `json:"description" gorm:"type:text"`
	Channel        NotificationChannel  `json:"channel" gorm:"type:varchar(20);not null"`
	Category       string               `json:"category" gorm:"type:varchar(100);index"` // order, auth, marketing, etc.

	// Template content
	Subject        string               `json:"subject" gorm:"type:varchar(500)"`
	BodyTemplate   string               `json:"bodyTemplate" gorm:"type:text"`
	HTMLTemplate   string               `json:"htmlTemplate" gorm:"type:text"`

	// Template configuration
	Variables      datatypes.JSON       `json:"variables" gorm:"type:jsonb"` // Available variables with descriptions
	DefaultData    datatypes.JSON       `json:"defaultData" gorm:"type:jsonb"` // Default values for variables

	// Versioning
	Version        int                  `json:"version" gorm:"default:1"`
	IsActive       bool                 `json:"isActive" gorm:"default:true"`
	IsSystem       bool                 `json:"isSystem" gorm:"default:false"` // System templates can't be deleted

	// Metadata
	Tags           datatypes.JSON       `json:"tags" gorm:"type:jsonb"`

	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt       `json:"-" gorm:"index"`
}

// NotificationPreference represents user notification preferences
type NotificationPreference struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string    `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	UserID            uuid.UUID `json:"userId" gorm:"type:uuid;not null;uniqueIndex:idx_user_tenant"`

	// Channel preferences
	EmailEnabled      bool      `json:"emailEnabled" gorm:"default:true"`
	SMSEnabled        bool      `json:"smsEnabled" gorm:"default:true"`
	PushEnabled       bool      `json:"pushEnabled" gorm:"default:true"`

	// Category preferences
	MarketingEnabled  bool      `json:"marketingEnabled" gorm:"default:true"`
	OrdersEnabled     bool      `json:"ordersEnabled" gorm:"default:true"`
	SecurityEnabled   bool      `json:"securityEnabled" gorm:"default:true"`

	// Contact information
	Email             string    `json:"email" gorm:"type:varchar(255)"`
	Phone             string    `json:"phone" gorm:"type:varchar(50)"`

	// Push tokens
	PushTokens        datatypes.JSON `json:"pushTokens" gorm:"type:jsonb"` // Array of FCM tokens

	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// NotificationLog represents audit log for notifications
type NotificationLog struct {
	ID             uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	NotificationID uuid.UUID          `json:"notificationId" gorm:"type:uuid;not null;index"`
	Event          string             `json:"event" gorm:"type:varchar(100);not null"` // queued, sent, delivered, failed, etc.
	Status         NotificationStatus `json:"status" gorm:"type:varchar(20);not null"`
	Message        string             `json:"message" gorm:"type:text"`
	Data           datatypes.JSON     `json:"data" gorm:"type:jsonb"`
	CreatedAt      time.Time          `json:"createdAt"`
}

// NotificationBatch represents a batch of notifications
type NotificationBatch struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string    `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Name           string    `json:"name" gorm:"type:varchar(255);not null"`
	Description    string    `json:"description" gorm:"type:text"`
	TemplateID     uuid.UUID `json:"templateId" gorm:"type:uuid;not null"`
	Channel        NotificationChannel `json:"channel" gorm:"type:varchar(20);not null"`

	// Batch stats
	TotalCount     int       `json:"totalCount" gorm:"default:0"`
	SentCount      int       `json:"sentCount" gorm:"default:0"`
	FailedCount    int       `json:"failedCount" gorm:"default:0"`

	Status         string    `json:"status" gorm:"type:varchar(50);default:'PENDING'"` // PENDING, PROCESSING, COMPLETED, FAILED
	ScheduledFor   *time.Time `json:"scheduledFor"`
	StartedAt      *time.Time `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt"`

	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName specifies table names
func (Notification) TableName() string {
	return "notifications"
}

func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

func (NotificationLog) TableName() string {
	return "notification_logs"
}

func (NotificationBatch) TableName() string {
	return "notification_batches"
}

// Helper methods

// CanRetry checks if notification can be retried
func (n *Notification) CanRetry() bool {
	return n.Status == StatusFailed && n.RetryCount < n.MaxRetries
}

// MarkAsSent marks notification as sent
func (n *Notification) MarkAsSent(providerID string) {
	now := time.Now()
	n.Status = StatusSent
	n.SentAt = &now
	n.ProviderID = providerID
}

// MarkAsDelivered marks notification as delivered
func (n *Notification) MarkAsDelivered() {
	now := time.Now()
	n.Status = StatusDelivered
	n.DeliveredAt = &now
}

// MarkAsFailed marks notification as failed
func (n *Notification) MarkAsFailed(errorMsg string) {
	now := time.Now()
	n.Status = StatusFailed
	n.FailedAt = &now
	n.ErrorMessage = errorMsg
	n.RetryCount++
}

// IsScheduled checks if notification is scheduled for future
func (n *Notification) IsScheduled() bool {
	return n.ScheduledFor != nil && n.ScheduledFor.After(time.Now())
}

// ShouldSendNow checks if notification should be sent now
func (n *Notification) ShouldSendNow() bool {
	if n.Status != StatusPending && n.Status != StatusQueued {
		return false
	}
	if n.ScheduledFor == nil {
		return true
	}
	return n.ScheduledFor.Before(time.Now()) || n.ScheduledFor.Equal(time.Now())
}
