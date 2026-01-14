package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/datatypes"
)

// ==========================================
// CORE SETTINGS MODELS
// ==========================================

type SettingsContext struct {
	TenantID      uuid.UUID `json:"tenantId" gorm:"type:uuid;not null"`
	ApplicationID uuid.UUID `json:"applicationId" gorm:"type:uuid;not null"`
	UserID        *uuid.UUID `json:"userId,omitempty" gorm:"type:uuid"`
	Scope         string    `json:"scope" gorm:"type:varchar(50);not null"` // global, tenant, application, user
}

type BrandingSettings struct {
	CompanyName string                 `json:"companyName"`
	LogoURL     *string                `json:"logoUrl,omitempty"`
	FaviconURL  *string                `json:"faviconUrl,omitempty"`
	BrandColors map[string]interface{} `json:"brandColors"`
	Fonts       map[string]interface{} `json:"fonts"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type ThemeSettings struct {
	ColorMode     string                 `json:"colorMode"`     // light, dark, auto
	ColorScheme   string                 `json:"colorScheme"`   // default, blue, green, etc.
	CustomColors  map[string]interface{} `json:"customColors,omitempty"`
	BorderRadius  float64                `json:"borderRadius"`  // in rem
	FontScale     float64                `json:"fontScale"`     // multiplier
	CustomCSS     *string                `json:"customCss,omitempty"`
}

type LayoutSettings struct {
	Sidebar    map[string]interface{} `json:"sidebar"`
	Navigation map[string]interface{} `json:"navigation"`
	Header     map[string]interface{} `json:"header"`
	Page       map[string]interface{} `json:"page"`
	Footer     map[string]interface{} `json:"footer"`
}

type AnimationSettings struct {
	GlobalSpeed     string                 `json:"globalSpeed"` // slow, normal, fast, disabled
	Transitions     map[string]interface{} `json:"transitions"`
	Effects         map[string]interface{} `json:"effects"`
	PageTransitions map[string]interface{} `json:"pageTransitions"`
	ReducedMotion   bool                   `json:"reducedMotion"`
}

type LocalizationSettings struct {
	Language     string                 `json:"language"`     // ISO 639-1
	Region       string                 `json:"region"`       // ISO 3166-1
	Currency     map[string]interface{} `json:"currency"`     // code, symbol, position
	Timezone     string                 `json:"timezone"`     // IANA timezone
	DateFormat   string                 `json:"dateFormat"`   // format string
	TimeFormat   string                 `json:"timeFormat"`   // 12h, 24h
	NumberFormat map[string]interface{} `json:"numberFormat"` // decimal, thousands, precision
	RTL          bool                   `json:"rtl"`          // right-to-left
}

type FeatureSettings struct {
	Flags        map[string]interface{} `json:"flags"`
	Modules      map[string]interface{} `json:"modules"`
	Integrations map[string]interface{} `json:"integrations"`
}

type UserPreferences struct {
	Dashboard     map[string]interface{} `json:"dashboard"`
	Notifications map[string]interface{} `json:"notifications"`
	Privacy       map[string]interface{} `json:"privacy"`
	Accessibility map[string]interface{} `json:"accessibility"`
}

type ApplicationSettings struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Environment  string                 `json:"environment"` // development, staging, production
	APIEndpoints map[string]interface{} `json:"apiEndpoints"`
	Security     map[string]interface{} `json:"security"`
	Performance  map[string]interface{} `json:"performance"`
}

// ==========================================
// MAIN SETTINGS MODEL
// ==========================================

type Settings struct {
	ID              uuid.UUID             `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID        uuid.UUID             `json:"tenantId" gorm:"type:uuid;not null;index"`
	ApplicationID   uuid.UUID             `json:"applicationId" gorm:"type:uuid;not null;index"`
	UserID          *uuid.UUID            `json:"userId,omitempty" gorm:"type:uuid;index"`
	Scope           string                `json:"scope" gorm:"type:varchar(50);not null;index"`
	Branding        datatypes.JSON        `json:"branding" gorm:"type:jsonb"`
	Theme           datatypes.JSON        `json:"theme" gorm:"type:jsonb"`
	Layout          datatypes.JSON        `json:"layout" gorm:"type:jsonb"`
	Animations      datatypes.JSON        `json:"animations" gorm:"type:jsonb"`
	Localization    datatypes.JSON        `json:"localization" gorm:"type:jsonb"`
	Ecommerce       datatypes.JSON        `json:"ecommerce" gorm:"type:jsonb"`
	Security        datatypes.JSON        `json:"security" gorm:"type:jsonb"`
	Notifications   datatypes.JSON        `json:"notifications" gorm:"type:jsonb"`
	Marketing       datatypes.JSON        `json:"marketing" gorm:"type:jsonb"`
	Integrations    datatypes.JSON        `json:"integrations" gorm:"type:jsonb"`
	Performance     datatypes.JSON        `json:"performance" gorm:"type:jsonb"`
	Compliance      datatypes.JSON        `json:"compliance" gorm:"type:jsonb"`
	Features        datatypes.JSON        `json:"features" gorm:"type:jsonb"`
	UserPreferences datatypes.JSON        `json:"userPreferences" gorm:"type:jsonb"`
	Application     datatypes.JSON        `json:"application" gorm:"type:jsonb"`
	Version         int                   `json:"version" gorm:"default:1"`
	CreatedAt       time.Time             `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time             `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt        `json:"deletedAt,omitempty" gorm:"index"`
}

// ==========================================
// SETTINGS PRESET MODEL
// ==========================================

type SettingsPreset struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string         `json:"name" gorm:"type:varchar(255);not null"`
	Description *string        `json:"description,omitempty" gorm:"type:text"`
	Category    string         `json:"category" gorm:"type:varchar(50);not null"` // theme, layout, complete
	Settings    datatypes.JSON `json:"settings" gorm:"type:jsonb;not null"`
	Preview     *string        `json:"preview,omitempty" gorm:"type:varchar(500)"` // Preview image URL
	Tags        datatypes.JSON `json:"tags,omitempty" gorm:"type:jsonb"`
	IsDefault   bool           `json:"isDefault" gorm:"default:false"`
	CreatedAt   time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

// ==========================================
// SETTINGS HISTORY MODEL
// ==========================================

type SettingsHistory struct {
	ID         uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SettingsID uuid.UUID      `json:"settingsId" gorm:"type:uuid;not null;index"`
	Operation  string         `json:"operation" gorm:"type:varchar(50);not null"` // create, update, delete
	Changes    datatypes.JSON `json:"changes" gorm:"type:jsonb"`                   // JSON diff of changes
	UserID     *uuid.UUID     `json:"userId,omitempty" gorm:"type:uuid"`
	Reason     *string        `json:"reason,omitempty" gorm:"type:text"`
	CreatedAt  time.Time      `json:"createdAt" gorm:"autoCreateTime"`
}

// ==========================================
// SETTINGS VALIDATION MODEL
// ==========================================

type SettingsValidation struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Field     string         `json:"field" gorm:"type:varchar(255);not null"`
	Rule      string         `json:"rule" gorm:"type:varchar(100);not null"`
	Message   string         `json:"message" gorm:"type:text;not null"`
	Severity  string         `json:"severity" gorm:"type:varchar(20);not null"` // error, warning
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime"`
}

// ==========================================
// REQUEST/RESPONSE MODELS
// ==========================================

type CreateSettingsRequest struct {
	Context         SettingsContext       `json:"context" binding:"required"`
	Branding        *BrandingSettings     `json:"branding,omitempty"`
	Theme           *ThemeSettings        `json:"theme,omitempty"`
	Layout          *LayoutSettings       `json:"layout,omitempty"`
	Animations      *AnimationSettings    `json:"animations,omitempty"`
	Localization    *LocalizationSettings `json:"localization,omitempty"`
	Ecommerce       map[string]interface{} `json:"ecommerce,omitempty"`
	Security        map[string]interface{} `json:"security,omitempty"`
	Notifications   map[string]interface{} `json:"notifications,omitempty"`
	Marketing       map[string]interface{} `json:"marketing,omitempty"`
	Integrations    map[string]interface{} `json:"integrations,omitempty"`
	Performance     map[string]interface{} `json:"performance,omitempty"`
	Compliance      map[string]interface{} `json:"compliance,omitempty"`
	Features        *FeatureSettings      `json:"features,omitempty"`
	UserPreferences *UserPreferences      `json:"userPreferences,omitempty"`
	Application     *ApplicationSettings  `json:"application,omitempty"`
}

type UpdateSettingsRequest struct {
	// Use map[string]interface{} for branding to accept flexible schemas
	// (e.g., admin branding has general/colors/appearance/advanced structure)
	Branding        map[string]interface{} `json:"branding,omitempty"`
	Theme           *ThemeSettings        `json:"theme,omitempty"`
	Layout          *LayoutSettings       `json:"layout,omitempty"`
	Animations      *AnimationSettings    `json:"animations,omitempty"`
	Localization    *LocalizationSettings `json:"localization,omitempty"`
	Ecommerce       map[string]interface{} `json:"ecommerce,omitempty"`
	Security        map[string]interface{} `json:"security,omitempty"`
	Notifications   map[string]interface{} `json:"notifications,omitempty"`
	Marketing       map[string]interface{} `json:"marketing,omitempty"`
	Integrations    map[string]interface{} `json:"integrations,omitempty"`
	Performance     map[string]interface{} `json:"performance,omitempty"`
	Compliance      map[string]interface{} `json:"compliance,omitempty"`
	Features        *FeatureSettings      `json:"features,omitempty"`
	UserPreferences *UserPreferences      `json:"userPreferences,omitempty"`
	Application     *ApplicationSettings  `json:"application,omitempty"`
}

type SettingsResponse struct {
	Success bool     `json:"success"`
	Data    Settings `json:"data,omitempty"`
	Message string   `json:"message,omitempty"`
}

type SettingsListResponse struct {
	Success    bool       `json:"success"`
	Data       []Settings `json:"data,omitempty"`
	Message    string     `json:"message,omitempty"`
	Pagination *struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
		HasNext    bool  `json:"hasNext"`
		HasPrev    bool  `json:"hasPrevious"`
	} `json:"pagination,omitempty"`
}

type PresetResponse struct {
	Success bool            `json:"success"`
	Data    SettingsPreset  `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}

type PresetListResponse struct {
	Success    bool              `json:"success"`
	Data       []SettingsPreset  `json:"data,omitempty"`
	Message    string            `json:"message,omitempty"`
	Pagination *struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
		HasNext    bool  `json:"hasNext"`
		HasPrev    bool  `json:"hasPrevious"`
	} `json:"pagination,omitempty"`
}