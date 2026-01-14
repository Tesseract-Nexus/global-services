package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationPreference represents user notification preferences
type NotificationPreference struct {
	ID                    uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID              string    `json:"tenantId" gorm:"column:tenant_id;type:varchar(255);not null;uniqueIndex:idx_preferences_tenant_user"`
	UserID                uuid.UUID `json:"userId" gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_preferences_tenant_user"`
	WebSocketEnabled      bool      `json:"websocketEnabled" gorm:"column:websocket_enabled;default:true"`
	SSEEnabled            bool      `json:"sseEnabled" gorm:"column:sse_enabled;default:true"`
	CategoryPreferences   JSONB     `json:"categoryPreferences" gorm:"column:category_preferences;type:jsonb;default:'{}'"`
	SoundEnabled          bool      `json:"soundEnabled" gorm:"column:sound_enabled;default:true"`
	VibrationEnabled      bool      `json:"vibrationEnabled" gorm:"column:vibration_enabled;default:true"`
	QuietHoursEnabled     bool      `json:"quietHoursEnabled" gorm:"column:quiet_hours_enabled;default:false"`
	QuietHoursStart       string    `json:"quietHoursStart,omitempty" gorm:"column:quiet_hours_start;type:time"`
	QuietHoursEnd         string    `json:"quietHoursEnd,omitempty" gorm:"column:quiet_hours_end;type:time"`
	QuietHoursTimezone    string    `json:"quietHoursTimezone,omitempty" gorm:"column:quiet_hours_timezone;type:varchar(50)"`
	GroupSimilar          bool      `json:"groupSimilar" gorm:"column:group_similar;default:true"`
	CreatedAt             time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName returns the table name for the NotificationPreference model
func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

// BeforeCreate sets default values before creating preferences
func (p *NotificationPreference) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CategoryPreferences == nil {
		p.CategoryPreferences = JSONB{}
	}
	return nil
}

// IsCategoryEnabled checks if a notification category is enabled
func (p *NotificationPreference) IsCategoryEnabled(category string) bool {
	if p.CategoryPreferences == nil {
		return true // Default to enabled
	}
	if enabled, ok := p.CategoryPreferences[category].(bool); ok {
		return enabled
	}
	return true // Default to enabled if not set
}

// SetCategoryEnabled sets the enabled state for a notification category
func (p *NotificationPreference) SetCategoryEnabled(category string, enabled bool) {
	if p.CategoryPreferences == nil {
		p.CategoryPreferences = JSONB{}
	}
	p.CategoryPreferences[category] = enabled
}

// GetDefaultPreferences returns default notification preferences for a user
func GetDefaultPreferences(tenantID string, userID uuid.UUID) *NotificationPreference {
	return &NotificationPreference{
		TenantID:            tenantID,
		UserID:              userID,
		WebSocketEnabled:    true,
		SSEEnabled:          true,
		CategoryPreferences: JSONB{},
		SoundEnabled:        true,
		VibrationEnabled:    true,
		QuietHoursEnabled:   false,
		GroupSimilar:        true,
	}
}

// PreferenceResponse is the API response wrapper for preferences
type PreferenceResponse struct {
	Success bool                    `json:"success"`
	Data    *NotificationPreference `json:"data,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

// UpdatePreferencesRequest is the request body for updating preferences
type UpdatePreferencesRequest struct {
	WebSocketEnabled    *bool  `json:"websocketEnabled,omitempty"`
	SSEEnabled          *bool  `json:"sseEnabled,omitempty"`
	CategoryPreferences JSONB  `json:"categoryPreferences,omitempty"`
	SoundEnabled        *bool  `json:"soundEnabled,omitempty"`
	VibrationEnabled    *bool  `json:"vibrationEnabled,omitempty"`
	QuietHoursEnabled   *bool  `json:"quietHoursEnabled,omitempty"`
	QuietHoursStart     string `json:"quietHoursStart,omitempty"`
	QuietHoursEnd       string `json:"quietHoursEnd,omitempty"`
	QuietHoursTimezone  string `json:"quietHoursTimezone,omitempty"`
	GroupSimilar        *bool  `json:"groupSimilar,omitempty"`
}
