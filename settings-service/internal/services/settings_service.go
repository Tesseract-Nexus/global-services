package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"settings-service/internal/models"
	"settings-service/internal/repository"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SettingsService interface {
	CreateSettings(req *models.CreateSettingsRequest, userID *uuid.UUID) (*models.Settings, error)
	GetSettings(id uuid.UUID) (*models.Settings, error)
	GetSettingsByContext(context models.SettingsContext) (*models.Settings, error)
	UpdateSettings(id uuid.UUID, req *models.UpdateSettingsRequest, userID *uuid.UUID) (*models.Settings, error)
	DeleteSettings(id uuid.UUID, userID *uuid.UUID) error
	ListSettings(filters repository.SettingsFilters) ([]models.Settings, int64, error)
	
	// Preset operations
	CreatePreset(preset *models.SettingsPreset) (*models.SettingsPreset, error)
	GetPreset(id uuid.UUID) (*models.SettingsPreset, error)
	ListPresets(filters repository.PresetFilters) ([]models.SettingsPreset, int64, error)
	UpdatePreset(id uuid.UUID, preset *models.SettingsPreset) (*models.SettingsPreset, error)
	DeletePreset(id uuid.UUID) error
	ApplyPreset(settingsID, presetID uuid.UUID, userID *uuid.UUID) (*models.Settings, error)
	
	// Advanced operations
	MergeSettings(baseID, overrideID uuid.UUID, userID *uuid.UUID) (*models.Settings, error)
	GetInheritedSettings(context models.SettingsContext) (*models.Settings, error)
	ValidateSettings(settings *models.Settings) ([]models.SettingsValidation, error)
	GetSettingsHistory(settingsID uuid.UUID, limit int) ([]models.SettingsHistory, error)
}

type settingsService struct {
	settingsRepo repository.SettingsRepository
}

// NewSettingsService creates a new settings service
func NewSettingsService(settingsRepo repository.SettingsRepository) SettingsService {
	return &settingsService{
		settingsRepo: settingsRepo,
	}
}

// ==========================================
// HELPER FUNCTIONS
// ==========================================

// structToJSON converts a struct to datatypes.JSON
func structToJSON(v interface{}) (datatypes.JSON, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON{}, err
	}
	return datatypes.JSON(data), nil
}

// ==========================================
// SETTINGS OPERATIONS
// ==========================================

func (s *settingsService) CreateSettings(req *models.CreateSettingsRequest, userID *uuid.UUID) (*models.Settings, error) {
	// Check if settings already exist for this context
	existing, err := s.settingsRepo.GetByContext(req.Context)
	if err == nil && existing != nil {
		return nil, errors.New("settings already exist for this context")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	
	// Create new settings with defaults
	settings := &models.Settings{
		ID:            uuid.New(),
		TenantID:      req.Context.TenantID,
		ApplicationID: req.Context.ApplicationID,
		UserID:        req.Context.UserID,
		Scope:         req.Context.Scope,
		Version:       1,
	}
	
	// Apply provided settings or use defaults
	if req.Branding != nil {
		brandingJSON, err := structToJSON(req.Branding)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal branding settings: %w", err)
		}
		settings.Branding = brandingJSON
	} else {
		defaultBranding := s.getDefaultBranding()
		brandingJSON, err := structToJSON(defaultBranding)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default branding settings: %w", err)
		}
		settings.Branding = brandingJSON
	}
	
	if req.Theme != nil {
		themeJSON, err := structToJSON(req.Theme)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal theme settings: %w", err)
		}
		settings.Theme = themeJSON
	} else {
		defaultTheme := s.getDefaultTheme()
		themeJSON, err := structToJSON(defaultTheme)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default theme settings: %w", err)
		}
		settings.Theme = themeJSON
	}
	
	if req.Layout != nil {
		layoutJSON, err := structToJSON(req.Layout)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal layout settings: %w", err)
		}
		settings.Layout = layoutJSON
	} else {
		defaultLayout := s.getDefaultLayout()
		layoutJSON, err := structToJSON(defaultLayout)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default layout settings: %w", err)
		}
		settings.Layout = layoutJSON
	}
	
	if req.Animations != nil {
		animationsJSON, err := structToJSON(req.Animations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal animations settings: %w", err)
		}
		settings.Animations = animationsJSON
	} else {
		defaultAnimations := s.getDefaultAnimations()
		animationsJSON, err := structToJSON(defaultAnimations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default animations settings: %w", err)
		}
		settings.Animations = animationsJSON
	}
	
	if req.Localization != nil {
		localizationJSON, err := structToJSON(req.Localization)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal localization settings: %w", err)
		}
		settings.Localization = localizationJSON
	} else {
		defaultLocalization := s.getDefaultLocalization()
		localizationJSON, err := structToJSON(defaultLocalization)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default localization settings: %w", err)
		}
		settings.Localization = localizationJSON
	}
	
	if req.Features != nil {
		featuresJSON, err := structToJSON(req.Features)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal features settings: %w", err)
		}
		settings.Features = featuresJSON
	} else {
		defaultFeatures := s.getDefaultFeatures()
		featuresJSON, err := structToJSON(defaultFeatures)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default features settings: %w", err)
		}
		settings.Features = featuresJSON
	}
	
	if req.UserPreferences != nil {
		userPreferencesJSON, err := structToJSON(req.UserPreferences)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal user preferences: %w", err)
		}
		settings.UserPreferences = userPreferencesJSON
	} else {
		defaultUserPreferences := s.getDefaultUserPreferences()
		userPreferencesJSON, err := structToJSON(defaultUserPreferences)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default user preferences: %w", err)
		}
		settings.UserPreferences = userPreferencesJSON
	}
	
	if req.Application != nil {
		applicationJSON, err := structToJSON(req.Application)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal application settings: %w", err)
		}
		settings.Application = applicationJSON
	} else {
		defaultApplication := s.getDefaultApplication()
		applicationJSON, err := structToJSON(defaultApplication)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default application settings: %w", err)
		}
		settings.Application = applicationJSON
	}
	
	// Handle comprehensive settings fields
	if req.Ecommerce != nil {
		ecommerceJSON, err := structToJSON(req.Ecommerce)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ecommerce settings: %w", err)
		}
		settings.Ecommerce = ecommerceJSON
	} else {
		settings.Ecommerce = datatypes.JSON("{}")
	}
	
	if req.Security != nil {
		securityJSON, err := structToJSON(req.Security)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal security settings: %w", err)
		}
		settings.Security = securityJSON
	} else {
		settings.Security = datatypes.JSON("{}")
	}
	
	if req.Notifications != nil {
		notificationsJSON, err := structToJSON(req.Notifications)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal notifications settings: %w", err)
		}
		settings.Notifications = notificationsJSON
	} else {
		settings.Notifications = datatypes.JSON("{}")
	}
	
	if req.Marketing != nil {
		marketingJSON, err := structToJSON(req.Marketing)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal marketing settings: %w", err)
		}
		settings.Marketing = marketingJSON
	} else {
		settings.Marketing = datatypes.JSON("{}")
	}
	
	if req.Integrations != nil {
		integrationsJSON, err := structToJSON(req.Integrations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal integrations settings: %w", err)
		}
		settings.Integrations = integrationsJSON
	} else {
		settings.Integrations = datatypes.JSON("{}")
	}
	
	if req.Performance != nil {
		performanceJSON, err := structToJSON(req.Performance)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal performance settings: %w", err)
		}
		settings.Performance = performanceJSON
	} else {
		settings.Performance = datatypes.JSON("{}")
	}
	
	if req.Compliance != nil {
		complianceJSON, err := structToJSON(req.Compliance)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal compliance settings: %w", err)
		}
		settings.Compliance = complianceJSON
	} else {
		settings.Compliance = datatypes.JSON("{}")
	}
	
	// Validate settings
	if validationErrors, err := s.ValidateSettings(settings); err != nil {
		return nil, err
	} else if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", validationErrors)
	}
	
	// Save settings
	if err := s.settingsRepo.Create(settings); err != nil {
		return nil, err
	}
	
	// Create history record
	s.createHistoryRecord(settings.ID, "create", nil, userID, "Settings created")
	
	return settings, nil
}

func (s *settingsService) GetSettings(id uuid.UUID) (*models.Settings, error) {
	return s.settingsRepo.GetByID(id)
}

func (s *settingsService) GetSettingsByContext(context models.SettingsContext) (*models.Settings, error) {
	return s.settingsRepo.GetByContext(context)
}

func (s *settingsService) UpdateSettings(id uuid.UUID, req *models.UpdateSettingsRequest, userID *uuid.UUID) (*models.Settings, error) {
	// Get existing settings
	settings, err := s.settingsRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	
	// Create a copy for change tracking
	originalSettings := *settings
	
	// Update fields if provided
	if req.Branding != nil {
		brandingJSON, err := structToJSON(req.Branding)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal branding settings: %w", err)
		}
		settings.Branding = brandingJSON
	}
	if req.Theme != nil {
		themeJSON, err := structToJSON(req.Theme)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal theme settings: %w", err)
		}
		settings.Theme = themeJSON
	}
	if req.Layout != nil {
		layoutJSON, err := structToJSON(req.Layout)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal layout settings: %w", err)
		}
		settings.Layout = layoutJSON
	}
	if req.Animations != nil {
		animationsJSON, err := structToJSON(req.Animations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal animations settings: %w", err)
		}
		settings.Animations = animationsJSON
	}
	if req.Localization != nil {
		localizationJSON, err := structToJSON(req.Localization)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal localization settings: %w", err)
		}
		settings.Localization = localizationJSON
	}
	if req.Features != nil {
		featuresJSON, err := structToJSON(req.Features)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal features settings: %w", err)
		}
		settings.Features = featuresJSON
	}
	if req.UserPreferences != nil {
		userPreferencesJSON, err := structToJSON(req.UserPreferences)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal user preferences: %w", err)
		}
		settings.UserPreferences = userPreferencesJSON
	}
	if req.Application != nil {
		applicationJSON, err := structToJSON(req.Application)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal application settings: %w", err)
		}
		settings.Application = applicationJSON
	}
	
	// Update comprehensive settings fields
	if req.Ecommerce != nil {
		ecommerceJSON, err := structToJSON(req.Ecommerce)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ecommerce settings: %w", err)
		}
		settings.Ecommerce = ecommerceJSON
	}
	if req.Security != nil {
		securityJSON, err := structToJSON(req.Security)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal security settings: %w", err)
		}
		settings.Security = securityJSON
	}
	if req.Notifications != nil {
		notificationsJSON, err := structToJSON(req.Notifications)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal notifications settings: %w", err)
		}
		settings.Notifications = notificationsJSON
	}
	if req.Marketing != nil {
		marketingJSON, err := structToJSON(req.Marketing)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal marketing settings: %w", err)
		}
		settings.Marketing = marketingJSON
	}
	if req.Integrations != nil {
		integrationsJSON, err := structToJSON(req.Integrations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal integrations settings: %w", err)
		}
		settings.Integrations = integrationsJSON
	}
	if req.Performance != nil {
		performanceJSON, err := structToJSON(req.Performance)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal performance settings: %w", err)
		}
		settings.Performance = performanceJSON
	}
	if req.Compliance != nil {
		complianceJSON, err := structToJSON(req.Compliance)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal compliance settings: %w", err)
		}
		settings.Compliance = complianceJSON
	}
	
	// Validate updated settings
	if validationErrors, err := s.ValidateSettings(settings); err != nil {
		return nil, err
	} else if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", validationErrors)
	}
	
	// Update settings
	if err := s.settingsRepo.Update(settings); err != nil {
		return nil, err
	}
	
	// Create history record with changes
	changes := s.calculateChanges(&originalSettings, settings)
	s.createHistoryRecord(settings.ID, "update", changes, userID, "Settings updated")
	
	return settings, nil
}

func (s *settingsService) DeleteSettings(id uuid.UUID, userID *uuid.UUID) error {
	// Check if settings exist
	settings, err := s.settingsRepo.GetByID(id)
	if err != nil {
		return err
	}
	
	// Create history record before deletion
	s.createHistoryRecord(settings.ID, "delete", nil, userID, "Settings deleted")
	
	return s.settingsRepo.Delete(id)
}

func (s *settingsService) ListSettings(filters repository.SettingsFilters) ([]models.Settings, int64, error) {
	return s.settingsRepo.List(filters)
}

// ==========================================
// PRESET OPERATIONS
// ==========================================

func (s *settingsService) CreatePreset(preset *models.SettingsPreset) (*models.SettingsPreset, error) {
	preset.ID = uuid.New()
	if err := s.settingsRepo.CreatePreset(preset); err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *settingsService) GetPreset(id uuid.UUID) (*models.SettingsPreset, error) {
	return s.settingsRepo.GetPresetByID(id)
}

func (s *settingsService) ListPresets(filters repository.PresetFilters) ([]models.SettingsPreset, int64, error) {
	return s.settingsRepo.ListPresets(filters)
}

func (s *settingsService) UpdatePreset(id uuid.UUID, preset *models.SettingsPreset) (*models.SettingsPreset, error) {
	preset.ID = id
	if err := s.settingsRepo.UpdatePreset(preset); err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *settingsService) DeletePreset(id uuid.UUID) error {
	return s.settingsRepo.DeletePreset(id)
}

func (s *settingsService) ApplyPreset(settingsID, presetID uuid.UUID, userID *uuid.UUID) (*models.Settings, error) {
	// Verify settings exists
	_, err := s.settingsRepo.GetByID(settingsID)
	if err != nil {
		return nil, err
	}
	
	// Get preset
	preset, err := s.settingsRepo.GetPresetByID(presetID)
	if err != nil {
		return nil, err
	}
	
	// Parse preset settings
	var presetSettings models.UpdateSettingsRequest
	if err := json.Unmarshal(preset.Settings, &presetSettings); err != nil {
		return nil, err
	}
	
	// Apply preset to current settings
	return s.UpdateSettings(settingsID, &presetSettings, userID)
}

// ==========================================
// ADVANCED OPERATIONS
// ==========================================

func (s *settingsService) MergeSettings(baseID, overrideID uuid.UUID, userID *uuid.UUID) (*models.Settings, error) {
	// Get base and override settings
	base, err := s.settingsRepo.GetByID(baseID)
	if err != nil {
		return nil, err
	}
	
	override, err := s.settingsRepo.GetByID(overrideID)
	if err != nil {
		return nil, err
	}
	
	// Create merged settings (override takes precedence)
	merged := *base
	merged.ID = uuid.New()
	merged.Version = 1
	
	// Merge each section (simplified - in production, you'd want deep merging)
	merged.Branding = override.Branding
	merged.Theme = override.Theme
	merged.Layout = override.Layout
	merged.Animations = override.Animations
	merged.Localization = override.Localization
	merged.Features = override.Features
	merged.UserPreferences = override.UserPreferences
	merged.Application = override.Application
	
	// Set context fields
	merged.TenantID = base.TenantID
	merged.ApplicationID = base.ApplicationID
	merged.UserID = base.UserID
	merged.Scope = base.Scope
	
	// Save merged settings
	if err := s.settingsRepo.Create(&merged); err != nil {
		return nil, err
	}
	
	return &merged, nil
}

func (s *settingsService) GetInheritedSettings(context models.SettingsContext) (*models.Settings, error) {
	// Try to get user-specific settings first
	if context.Scope == "user" && context.UserID != nil {
		if settings, err := s.settingsRepo.GetByContext(context); err == nil {
			return settings, nil
		}
	}
	
	// Fall back to application settings
	appContext := context
	appContext.Scope = "application"
	appContext.UserID = nil
	if settings, err := s.settingsRepo.GetByContext(appContext); err == nil {
		return settings, nil
	}
	
	// Fall back to tenant settings
	tenantContext := context
	tenantContext.Scope = "tenant"
	tenantContext.UserID = nil
	tenantContext.ApplicationID = uuid.Nil
	if settings, err := s.settingsRepo.GetByContext(tenantContext); err == nil {
		return settings, nil
	}
	
	// Fall back to global settings
	globalContext := context
	globalContext.Scope = "global"
	globalContext.UserID = nil
	globalContext.ApplicationID = uuid.Nil
	globalContext.TenantID = uuid.Nil
	
	return s.settingsRepo.GetByContext(globalContext)
}

func (s *settingsService) ValidateSettings(settings *models.Settings) ([]models.SettingsValidation, error) {
	var validationErrors []models.SettingsValidation
	
	// Validate theme settings
	if len(settings.Theme) > 0 {
		var theme models.ThemeSettings
		if err := json.Unmarshal(settings.Theme, &theme); err == nil {
			if theme.ColorMode != "light" && theme.ColorMode != "dark" && theme.ColorMode != "auto" {
				validationErrors = append(validationErrors, models.SettingsValidation{
					Field:    "theme.colorMode",
					Rule:     "enum",
					Message:  "Color mode must be 'light', 'dark', or 'auto'",
					Severity: "error",
				})
			}
		}
	}
	
	// Validate localization settings
	if len(settings.Localization) > 0 {
		var localization models.LocalizationSettings
		if err := json.Unmarshal(settings.Localization, &localization); err == nil {
			if len(localization.Language) != 2 {
				validationErrors = append(validationErrors, models.SettingsValidation{
					Field:    "localization.language",
					Rule:     "format",
					Message:  "Language must be a 2-character ISO 639-1 code",
					Severity: "error",
				})
			}
		}
	}
	
	// Add more validation rules as needed
	
	return validationErrors, nil
}

func (s *settingsService) GetSettingsHistory(settingsID uuid.UUID, limit int) ([]models.SettingsHistory, error) {
	return s.settingsRepo.GetHistory(settingsID, limit)
}

// ==========================================
// HELPER METHODS
// ==========================================

func (s *settingsService) createHistoryRecord(settingsID uuid.UUID, operation string, changes interface{}, userID *uuid.UUID, reason string) {
	var changesJSON []byte
	if changes != nil {
		changesJSON, _ = json.Marshal(changes)
	}
	
	history := &models.SettingsHistory{
		SettingsID: settingsID,
		Operation:  operation,
		Changes:    changesJSON,
		UserID:     userID,
		Reason:     &reason,
	}
	
	s.settingsRepo.CreateHistory(history)
}

func (s *settingsService) calculateChanges(original, updated *models.Settings) map[string]interface{} {
	changes := make(map[string]interface{})
	
	// This is a simplified change detection - in production you'd want more sophisticated diff
	originalJSON, _ := json.Marshal(original)
	updatedJSON, _ := json.Marshal(updated)
	
	changes["before"] = string(originalJSON)
	changes["after"] = string(updatedJSON)
	changes["timestamp"] = time.Now()
	
	return changes
}

// ==========================================
// DEFAULT SETTINGS GENERATORS
// ==========================================

func (s *settingsService) getDefaultBranding() models.BrandingSettings {
	return models.BrandingSettings{
		CompanyName: "Tesseract Hub",
		BrandColors: map[string]interface{}{
			"primary":   "#3b82f6",
			"secondary": "#6b7280",
			"accent":    "#8b5cf6",
		},
		Fonts: map[string]interface{}{
			"primary":   "Inter",
			"secondary": "system-ui",
		},
		Metadata: map[string]interface{}{
			"description": "Modern business application",
		},
	}
}

func (s *settingsService) getDefaultTheme() models.ThemeSettings {
	return models.ThemeSettings{
		ColorMode:    "light",
		ColorScheme:  "default",
		BorderRadius: 0.5,
		FontScale:    1.0,
	}
}

func (s *settingsService) getDefaultLayout() models.LayoutSettings {
	return models.LayoutSettings{
		Sidebar: map[string]interface{}{
			"position":         "left",
			"collapsible":      true,
			"defaultCollapsed": false,
			"width":            280,
		},
		Navigation: map[string]interface{}{
			"style":           "vertical",
			"showBreadcrumbs": true,
			"showSearchBar":   true,
		},
		Header: map[string]interface{}{
			"style":  "default",
			"height": 64,
			"sticky": true,
		},
		Page: map[string]interface{}{
			"layout":   "default",
			"maxWidth": 1200,
			"padding":  1.5,
		},
		Footer: map[string]interface{}{
			"enabled": true,
		},
	}
}

func (s *settingsService) getDefaultAnimations() models.AnimationSettings {
	return models.AnimationSettings{
		GlobalSpeed: "normal",
		Transitions: map[string]interface{}{
			"easing":   "ease",
			"duration": 200,
		},
		Effects: map[string]interface{}{
			"fadeIn":  true,
			"slideIn": true,
			"scaleIn": false,
			"bounceIn": false,
		},
		PageTransitions: map[string]interface{}{
			"enabled": true,
			"type":    "fade",
		},
		ReducedMotion: false,
	}
}

func (s *settingsService) getDefaultLocalization() models.LocalizationSettings {
	return models.LocalizationSettings{
		Language:   "en",
		Region:     "US",
		Currency: map[string]interface{}{
			"code":     "USD",
			"symbol":   "$",
			"position": "before",
		},
		Timezone:   "America/New_York",
		DateFormat: "MM/DD/YYYY",
		TimeFormat: "12h",
		NumberFormat: map[string]interface{}{
			"decimal":   ".",
			"thousands": ",",
			"precision": 2,
		},
		RTL: false,
	}
}

func (s *settingsService) getDefaultFeatures() models.FeatureSettings {
	return models.FeatureSettings{
		Flags:        map[string]interface{}{},
		Modules:      map[string]interface{}{},
		Integrations: map[string]interface{}{},
	}
}

func (s *settingsService) getDefaultUserPreferences() models.UserPreferences {
	return models.UserPreferences{
		Dashboard: map[string]interface{}{
			"layout": "grid",
		},
		Notifications: map[string]interface{}{
			"email":   true,
			"push":    true,
			"desktop": false,
			"sound":   true,
		},
		Privacy: map[string]interface{}{
			"analytics":   true,
			"tracking":    false,
			"dataSharing": false,
		},
		Accessibility: map[string]interface{}{
			"highContrast": false,
			"largeText":    false,
			"reducedMotion": false,
			"screenReader": false,
		},
	}
}

func (s *settingsService) getDefaultApplication() models.ApplicationSettings {
	return models.ApplicationSettings{
		Name:        "Application",
		Version:     "1.0.0",
		Environment: "development",
		APIEndpoints: map[string]interface{}{},
		Security: map[string]interface{}{
			"sessionTimeout":    60,
			"maxLoginAttempts":  5,
			"passwordPolicy": map[string]interface{}{
				"minLength":        8,
				"requireUppercase": true,
				"requireLowercase": true,
				"requireNumbers":   true,
				"requireSymbols":   false,
			},
		},
		Performance: map[string]interface{}{
			"cacheTimeout":   300000,
			"requestTimeout": 30000,
			"batchSize":      100,
		},
	}
}