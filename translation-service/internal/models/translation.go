package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Language represents a supported language
type Language struct {
	Code       string `json:"code" gorm:"primaryKey;type:varchar(10)"`
	Name       string `json:"name" gorm:"type:varchar(100);not null"`
	NativeName string `json:"native_name" gorm:"type:varchar(100)"`
	RTL        bool   `json:"rtl" gorm:"default:false"`
	IsActive   bool   `json:"is_active" gorm:"default:true"`
	Region     string `json:"region" gorm:"type:varchar(50)"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TranslationCache stores cached translations for performance
type TranslationCache struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID     string    `json:"tenant_id" gorm:"type:varchar(50);index;not null"`
	SourceLang   string    `json:"source_lang" gorm:"type:varchar(10);not null;index"`
	TargetLang   string    `json:"target_lang" gorm:"type:varchar(10);not null;index"`
	SourceHash   string    `json:"source_hash" gorm:"type:varchar(64);not null;uniqueIndex:idx_translation_cache_unique"`
	SourceText   string    `json:"source_text" gorm:"type:text;not null"`
	TranslatedText string  `json:"translated_text" gorm:"type:text;not null"`
	Context      string    `json:"context" gorm:"type:varchar(100)"` // e.g., "product_name", "category", "ui_label"
	HitCount     int       `json:"hit_count" gorm:"default:0"`
	Provider     string    `json:"provider" gorm:"type:varchar(50)"` // libretranslate, huggingface, etc.
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"index"`
}

// GenerateSourceHash creates a unique hash for source text + languages
func GenerateSourceHash(sourceLang, targetLang, sourceText, context string) string {
	data := sourceLang + "|" + targetLang + "|" + sourceText + "|" + context
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// TranslationRequest represents a translation request from the API
type TranslationRequest struct {
	Text       string `json:"text" binding:"required"`
	SourceLang string `json:"source_lang"` // Optional, auto-detect if empty
	TargetLang string `json:"target_lang" binding:"required"`
	Context    string `json:"context"` // Optional context for better translation
}

// TranslationResponse represents the response for a single translation
type TranslationResponse struct {
	OriginalText   string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
	SourceLang     string `json:"source_lang"`
	TargetLang     string `json:"target_lang"`
	Cached         bool   `json:"cached"`
	Provider       string `json:"provider,omitempty"`
}

// BatchTranslationRequest represents a batch translation request
type BatchTranslationRequest struct {
	Items      []TranslationItem `json:"items" binding:"required,min=1,max=50"`
	SourceLang string            `json:"source_lang"` // Default source lang for all items
	TargetLang string            `json:"target_lang" binding:"required"`
}

// TranslationItem represents a single item in a batch request
type TranslationItem struct {
	ID         string `json:"id"` // Client-provided ID for matching responses
	Text       string `json:"text" binding:"required"`
	SourceLang string `json:"source_lang"` // Override batch default
	Context    string `json:"context"`
}

// BatchTranslationResponse represents the response for batch translation
type BatchTranslationResponse struct {
	Items      []BatchTranslationItem `json:"items"`
	TotalCount int                    `json:"total_count"`
	CachedCount int                   `json:"cached_count"`
	TargetLang string                 `json:"target_lang"`
}

// BatchTranslationItem represents a single translated item in batch response
type BatchTranslationItem struct {
	ID             string `json:"id"`
	OriginalText   string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
	SourceLang     string `json:"source_lang"`
	Cached         bool   `json:"cached"`
	Error          string `json:"error,omitempty"`
}

// DetectLanguageRequest represents a language detection request
type DetectLanguageRequest struct {
	Text string `json:"text" binding:"required"`
}

// DetectLanguageResponse represents the response for language detection
type DetectLanguageResponse struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// TranslationStats represents translation statistics per tenant
type TranslationStats struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID        string    `json:"tenant_id" gorm:"type:varchar(50);uniqueIndex;not null"`
	TotalRequests   int64     `json:"total_requests" gorm:"default:0"`
	CacheHits       int64     `json:"cache_hits" gorm:"default:0"`
	CacheMisses     int64     `json:"cache_misses" gorm:"default:0"`
	TotalCharacters int64     `json:"total_characters" gorm:"default:0"`
	LastRequestAt   time.Time `json:"last_request_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TenantLanguagePreference stores language preferences per tenant
type TenantLanguagePreference struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID          string    `json:"tenant_id" gorm:"type:varchar(50);uniqueIndex;not null"`
	DefaultSourceLang string    `json:"default_source_lang" gorm:"type:varchar(10);default:'en'"`
	DefaultTargetLang string    `json:"default_target_lang" gorm:"type:varchar(10);default:'hi'"`
	EnabledLanguages  []byte    `json:"enabled_languages" gorm:"type:jsonb"` // JSON array of enabled language codes
	AutoDetect        bool      `json:"auto_detect" gorm:"default:true"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UserLanguagePreference stores language preferences per user within a tenant
// This is the main table for user-specific language settings
type UserLanguagePreference struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TenantID          string    `json:"tenant_id" gorm:"type:varchar(50);not null;uniqueIndex:idx_user_lang_pref_tenant_user"`
	UserID            uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_user_lang_pref_tenant_user"`
	PreferredLanguage string    `json:"preferred_language" gorm:"type:varchar(10);default:'en';not null"` // ISO 639-1 language code
	SourceLanguage    string    `json:"source_language" gorm:"type:varchar(10);default:'en'"`             // Default source language for translations
	AutoDetectSource  bool      `json:"auto_detect_source" gorm:"default:true"`                            // Auto-detect source language
	RTLEnabled        bool      `json:"rtl_enabled" gorm:"default:false"`                                  // Right-to-left text support
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TableName returns the table name for UserLanguagePreference
func (UserLanguagePreference) TableName() string {
	return "user_language_preferences"
}

// BeforeCreate hook for UserLanguagePreference
func (p *UserLanguagePreference) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	// Default to English if no preferred language is set
	if p.PreferredLanguage == "" {
		p.PreferredLanguage = "en"
	}
	if p.SourceLanguage == "" {
		p.SourceLanguage = "en"
	}
	return nil
}

// GetDefaultUserPreference returns default user language preference
func GetDefaultUserPreference(tenantID string, userID uuid.UUID) *UserLanguagePreference {
	return &UserLanguagePreference{
		TenantID:          tenantID,
		UserID:            userID,
		PreferredLanguage: "en",
		SourceLanguage:    "en",
		AutoDetectSource:  true,
		RTLEnabled:        false,
	}
}

// UserPreferenceRequest represents a request to set user language preference
type UserPreferenceRequest struct {
	PreferredLanguage string `json:"preferred_language" binding:"required,min=2,max=10"`
	SourceLanguage    string `json:"source_language,omitempty"`
	AutoDetectSource  *bool  `json:"auto_detect_source,omitempty"`
	RTLEnabled        *bool  `json:"rtl_enabled,omitempty"`
}

// UserPreferenceResponse represents the response for user language preference
type UserPreferenceResponse struct {
	Success bool                     `json:"success"`
	Data    *UserLanguagePreference  `json:"data,omitempty"`
	Message string                   `json:"message,omitempty"`
}

// BeforeCreate hook to generate UUID
func (t *TranslationCache) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.SourceHash == "" {
		t.SourceHash = GenerateSourceHash(t.SourceLang, t.TargetLang, t.SourceText, t.Context)
	}
	return nil
}

// BeforeCreate hook for TranslationStats
func (s *TranslationStats) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for TenantLanguagePreference
func (p *TenantLanguagePreference) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// SupportedLanguages returns a list of commonly supported regional languages
var SupportedLanguages = []Language{
	// Indian Languages
	{Code: "hi", Name: "Hindi", NativeName: "हिन्दी", RTL: false, Region: "India"},
	{Code: "ta", Name: "Tamil", NativeName: "தமிழ்", RTL: false, Region: "India"},
	{Code: "te", Name: "Telugu", NativeName: "తెలుగు", RTL: false, Region: "India"},
	{Code: "mr", Name: "Marathi", NativeName: "मराठी", RTL: false, Region: "India"},
	{Code: "bn", Name: "Bengali", NativeName: "বাংলা", RTL: false, Region: "India"},
	{Code: "gu", Name: "Gujarati", NativeName: "ગુજરાતી", RTL: false, Region: "India"},
	{Code: "kn", Name: "Kannada", NativeName: "ಕನ್ನಡ", RTL: false, Region: "India"},
	{Code: "ml", Name: "Malayalam", NativeName: "മലയാളം", RTL: false, Region: "India"},
	{Code: "pa", Name: "Punjabi", NativeName: "ਪੰਜਾਬੀ", RTL: false, Region: "India"},
	{Code: "or", Name: "Odia", NativeName: "ଓଡ଼ିଆ", RTL: false, Region: "India"},

	// Global Languages
	{Code: "en", Name: "English", NativeName: "English", RTL: false, Region: "Global"},
	{Code: "es", Name: "Spanish", NativeName: "Español", RTL: false, Region: "Global"},
	{Code: "fr", Name: "French", NativeName: "Français", RTL: false, Region: "Global"},
	{Code: "de", Name: "German", NativeName: "Deutsch", RTL: false, Region: "Global"},
	{Code: "pt", Name: "Portuguese", NativeName: "Português", RTL: false, Region: "Global"},
	{Code: "it", Name: "Italian", NativeName: "Italiano", RTL: false, Region: "Global"},
	{Code: "nl", Name: "Dutch", NativeName: "Nederlands", RTL: false, Region: "Global"},
	{Code: "ru", Name: "Russian", NativeName: "Русский", RTL: false, Region: "Global"},
	{Code: "zh", Name: "Chinese", NativeName: "中文", RTL: false, Region: "Asia"},
	{Code: "ja", Name: "Japanese", NativeName: "日本語", RTL: false, Region: "Asia"},
	{Code: "ko", Name: "Korean", NativeName: "한국어", RTL: false, Region: "Asia"},

	// Southeast Asian Languages
	{Code: "th", Name: "Thai", NativeName: "ไทย", RTL: false, Region: "Southeast Asia"},
	{Code: "vi", Name: "Vietnamese", NativeName: "Tiếng Việt", RTL: false, Region: "Southeast Asia"},
	{Code: "id", Name: "Indonesian", NativeName: "Bahasa Indonesia", RTL: false, Region: "Southeast Asia"},
	{Code: "ms", Name: "Malay", NativeName: "Bahasa Melayu", RTL: false, Region: "Southeast Asia"},
	{Code: "tl", Name: "Filipino", NativeName: "Filipino", RTL: false, Region: "Southeast Asia"},

	// Middle Eastern Languages
	{Code: "ar", Name: "Arabic", NativeName: "العربية", RTL: true, Region: "Middle East"},
	{Code: "fa", Name: "Persian", NativeName: "فارسی", RTL: true, Region: "Middle East"},
	{Code: "he", Name: "Hebrew", NativeName: "עברית", RTL: true, Region: "Middle East"},
	{Code: "tr", Name: "Turkish", NativeName: "Türkçe", RTL: false, Region: "Middle East"},
}
