package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"settings-service/internal/models"
	"settings-service/internal/repository"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// StorefrontThemeService defines the interface for storefront theme operations
type StorefrontThemeService interface {
	// Legacy methods (use tenantID as the key - kept for backward compatibility)
	CreateOrUpdate(tenantID uuid.UUID, req *models.CreateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)
	GetByTenantID(tenantID uuid.UUID) (*models.StorefrontThemeSettings, error)
	Update(tenantID uuid.UUID, req *models.UpdateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)
	Delete(tenantID uuid.UUID) error

	// New methods (use storefrontID as the key - preferred for multi-storefront support)
	GetByStorefrontID(storefrontID uuid.UUID) (*models.StorefrontThemeSettings, error)
	CreateOrUpdateByStorefrontID(storefrontID, tenantID uuid.UUID, req *models.CreateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)

	GetPresets() []models.StorefrontThemePreset
	ApplyPreset(tenantID uuid.UUID, presetID string, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)
	GetDefaults() *models.StorefrontThemeSettings
	CloneTheme(sourceTenantID, targetTenantID uuid.UUID, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)
	// History methods
	GetHistory(tenantID uuid.UUID, limit int) ([]models.StorefrontThemeHistory, error)
	GetHistoryVersion(tenantID uuid.UUID, version int) (*models.StorefrontThemeHistory, error)
	RestoreVersion(tenantID uuid.UUID, version int, userID *uuid.UUID) (*models.StorefrontThemeSettings, error)
}

type storefrontThemeService struct {
	repo repository.StorefrontThemeRepository
}

// NewStorefrontThemeService creates a new storefront theme service
func NewStorefrontThemeService(repo repository.StorefrontThemeRepository) StorefrontThemeService {
	return &storefrontThemeService{repo: repo}
}

// CreateOrUpdate creates new or updates existing storefront theme settings
func (s *storefrontThemeService) CreateOrUpdate(tenantID uuid.UUID, req *models.CreateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Build the settings object
	settings := &models.StorefrontThemeSettings{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ThemeTemplate:  req.ThemeTemplate,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
		AccentColor:    req.AccentColor,
		LogoURL:        req.LogoURL,
		FaviconURL:     req.FaviconURL,
		FontPrimary:    req.FontPrimary,
		FontSecondary:  req.FontSecondary,
		ColorMode:      req.ColorMode,
		CustomCSS:      req.CustomCSS,
		Version:        1,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	// Set defaults if empty
	if settings.ThemeTemplate == "" {
		settings.ThemeTemplate = "vibrant"
	}
	if settings.PrimaryColor == "" {
		settings.PrimaryColor = "#8B5CF6"
	}
	if settings.SecondaryColor == "" {
		settings.SecondaryColor = "#EC4899"
	}
	if settings.FontPrimary == "" {
		settings.FontPrimary = "Inter"
	}
	if settings.FontSecondary == "" {
		settings.FontSecondary = "system-ui"
	}
	if settings.ColorMode == "" {
		settings.ColorMode = "both"
	}

	// Convert config maps to JSON
	if req.HeaderConfig != nil {
		headerJSON, err := json.Marshal(req.HeaderConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal header config: %w", err)
		}
		settings.HeaderConfig = datatypes.JSON(headerJSON)
	} else {
		defaultHeader := models.GetDefaultHeaderConfig()
		headerJSON, _ := json.Marshal(defaultHeader)
		settings.HeaderConfig = datatypes.JSON(headerJSON)
	}

	if req.HomepageConfig != nil {
		homepageJSON, err := json.Marshal(req.HomepageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal homepage config: %w", err)
		}
		settings.HomepageConfig = datatypes.JSON(homepageJSON)
	} else {
		defaultHomepage := models.GetDefaultHomepageConfig()
		homepageJSON, _ := json.Marshal(defaultHomepage)
		settings.HomepageConfig = datatypes.JSON(homepageJSON)
	}

	if req.FooterConfig != nil {
		footerJSON, err := json.Marshal(req.FooterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal footer config: %w", err)
		}
		settings.FooterConfig = datatypes.JSON(footerJSON)
	} else {
		defaultFooter := models.GetDefaultFooterConfig()
		footerJSON, _ := json.Marshal(defaultFooter)
		settings.FooterConfig = datatypes.JSON(footerJSON)
	}

	if req.ProductConfig != nil {
		productJSON, err := json.Marshal(req.ProductConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal product config: %w", err)
		}
		settings.ProductConfig = datatypes.JSON(productJSON)
	} else {
		defaultProduct := models.GetDefaultProductConfig()
		productJSON, _ := json.Marshal(defaultProduct)
		settings.ProductConfig = datatypes.JSON(productJSON)
	}

	if req.CheckoutConfig != nil {
		checkoutJSON, err := json.Marshal(req.CheckoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal checkout config: %w", err)
		}
		settings.CheckoutConfig = datatypes.JSON(checkoutJSON)
	} else {
		defaultCheckout := models.GetDefaultCheckoutConfig()
		checkoutJSON, _ := json.Marshal(defaultCheckout)
		settings.CheckoutConfig = datatypes.JSON(checkoutJSON)
	}

	// Handle enhanced configuration options
	if req.TypographyConfig != nil {
		typographyJSON, err := json.Marshal(req.TypographyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal typography config: %w", err)
		}
		settings.TypographyConfig = datatypes.JSON(typographyJSON)
	} else {
		settings.TypographyConfig = datatypes.JSON([]byte("{}"))
	}

	if req.LayoutConfig != nil {
		layoutJSON, err := json.Marshal(req.LayoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal layout config: %w", err)
		}
		settings.LayoutConfig = datatypes.JSON(layoutJSON)
	} else {
		settings.LayoutConfig = datatypes.JSON([]byte("{}"))
	}

	if req.SpacingStyleConfig != nil {
		spacingJSON, err := json.Marshal(req.SpacingStyleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spacing style config: %w", err)
		}
		settings.SpacingStyleConfig = datatypes.JSON(spacingJSON)
	} else {
		settings.SpacingStyleConfig = datatypes.JSON([]byte("{}"))
	}

	if req.MobileConfig != nil {
		mobileJSON, err := json.Marshal(req.MobileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mobile config: %w", err)
		}
		settings.MobileConfig = datatypes.JSON(mobileJSON)
	} else {
		settings.MobileConfig = datatypes.JSON([]byte("{}"))
	}

	if req.AdvancedConfig != nil {
		advancedJSON, err := json.Marshal(req.AdvancedConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal advanced config: %w", err)
		}
		settings.AdvancedConfig = datatypes.JSON(advancedJSON)
	} else {
		settings.AdvancedConfig = datatypes.JSON([]byte("{}"))
	}

	// Upsert the settings
	if err := s.repo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to save storefront theme settings: %w", err)
	}

	// Fetch the saved settings
	savedSettings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		return nil, err
	}

	// Save to history
	if err := s.saveHistory(savedSettings, "Settings saved", userID); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save history: %v\n", err)
	}

	return savedSettings, nil
}

// GetByTenantID retrieves storefront theme settings for a tenant
func (s *storefrontThemeService) GetByTenantID(tenantID uuid.UUID) (*models.StorefrontThemeSettings, error) {
	settings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings if not found
			return s.GetDefaults(), nil
		}
		return nil, err
	}
	return settings, nil
}

// GetByStorefrontID retrieves storefront theme settings by storefront ID
func (s *storefrontThemeService) GetByStorefrontID(storefrontID uuid.UUID) (*models.StorefrontThemeSettings, error) {
	settings, err := s.repo.GetByStorefrontID(storefrontID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings if not found
			return s.GetDefaults(), nil
		}
		return nil, err
	}
	return settings, nil
}

// CreateOrUpdateByStorefrontID creates or updates settings keyed by storefront ID
func (s *storefrontThemeService) CreateOrUpdateByStorefrontID(storefrontID, tenantID uuid.UUID, req *models.CreateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Build the settings object
	settings := &models.StorefrontThemeSettings{
		ID:             uuid.New(),
		TenantID:       tenantID,
		StorefrontID:   storefrontID,
		ThemeTemplate:  req.ThemeTemplate,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
		AccentColor:    req.AccentColor,
		LogoURL:        req.LogoURL,
		FaviconURL:     req.FaviconURL,
		FontPrimary:    req.FontPrimary,
		FontSecondary:  req.FontSecondary,
		ColorMode:      req.ColorMode,
		CustomCSS:      req.CustomCSS,
		Version:        1,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	// Set defaults if empty
	if settings.ThemeTemplate == "" {
		settings.ThemeTemplate = "vibrant"
	}
	if settings.PrimaryColor == "" {
		settings.PrimaryColor = "#8B5CF6"
	}
	if settings.SecondaryColor == "" {
		settings.SecondaryColor = "#EC4899"
	}
	if settings.FontPrimary == "" {
		settings.FontPrimary = "Inter"
	}
	if settings.FontSecondary == "" {
		settings.FontSecondary = "system-ui"
	}
	if settings.ColorMode == "" {
		settings.ColorMode = "both"
	}

	// Convert config maps to JSON
	if req.HeaderConfig != nil {
		headerJSON, err := json.Marshal(req.HeaderConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal header config: %w", err)
		}
		settings.HeaderConfig = datatypes.JSON(headerJSON)
	} else {
		defaultHeader := models.GetDefaultHeaderConfig()
		headerJSON, _ := json.Marshal(defaultHeader)
		settings.HeaderConfig = datatypes.JSON(headerJSON)
	}

	if req.HomepageConfig != nil {
		homepageJSON, err := json.Marshal(req.HomepageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal homepage config: %w", err)
		}
		settings.HomepageConfig = datatypes.JSON(homepageJSON)
	} else {
		defaultHomepage := models.GetDefaultHomepageConfig()
		homepageJSON, _ := json.Marshal(defaultHomepage)
		settings.HomepageConfig = datatypes.JSON(homepageJSON)
	}

	if req.FooterConfig != nil {
		footerJSON, err := json.Marshal(req.FooterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal footer config: %w", err)
		}
		settings.FooterConfig = datatypes.JSON(footerJSON)
	} else {
		defaultFooter := models.GetDefaultFooterConfig()
		footerJSON, _ := json.Marshal(defaultFooter)
		settings.FooterConfig = datatypes.JSON(footerJSON)
	}

	if req.ProductConfig != nil {
		productJSON, err := json.Marshal(req.ProductConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal product config: %w", err)
		}
		settings.ProductConfig = datatypes.JSON(productJSON)
	} else {
		defaultProduct := models.GetDefaultProductConfig()
		productJSON, _ := json.Marshal(defaultProduct)
		settings.ProductConfig = datatypes.JSON(productJSON)
	}

	if req.CheckoutConfig != nil {
		checkoutJSON, err := json.Marshal(req.CheckoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal checkout config: %w", err)
		}
		settings.CheckoutConfig = datatypes.JSON(checkoutJSON)
	} else {
		defaultCheckout := models.GetDefaultCheckoutConfig()
		checkoutJSON, _ := json.Marshal(defaultCheckout)
		settings.CheckoutConfig = datatypes.JSON(checkoutJSON)
	}

	// Handle enhanced configuration options
	if req.TypographyConfig != nil {
		typographyJSON, err := json.Marshal(req.TypographyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal typography config: %w", err)
		}
		settings.TypographyConfig = datatypes.JSON(typographyJSON)
	} else {
		settings.TypographyConfig = datatypes.JSON([]byte("{}"))
	}

	if req.LayoutConfig != nil {
		layoutJSON, err := json.Marshal(req.LayoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal layout config: %w", err)
		}
		settings.LayoutConfig = datatypes.JSON(layoutJSON)
	} else {
		settings.LayoutConfig = datatypes.JSON([]byte("{}"))
	}

	if req.SpacingStyleConfig != nil {
		spacingJSON, err := json.Marshal(req.SpacingStyleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spacing style config: %w", err)
		}
		settings.SpacingStyleConfig = datatypes.JSON(spacingJSON)
	} else {
		settings.SpacingStyleConfig = datatypes.JSON([]byte("{}"))
	}

	if req.MobileConfig != nil {
		mobileJSON, err := json.Marshal(req.MobileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mobile config: %w", err)
		}
		settings.MobileConfig = datatypes.JSON(mobileJSON)
	} else {
		settings.MobileConfig = datatypes.JSON([]byte("{}"))
	}

	if req.AdvancedConfig != nil {
		advancedJSON, err := json.Marshal(req.AdvancedConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal advanced config: %w", err)
		}
		settings.AdvancedConfig = datatypes.JSON(advancedJSON)
	} else {
		settings.AdvancedConfig = datatypes.JSON([]byte("{}"))
	}

	// Initialize with default content pages for new storefronts
	defaultContentPages := models.GetDefaultContentPages()
	contentPagesJSON, _ := json.Marshal(defaultContentPages)
	settings.ContentPages = datatypes.JSON(contentPagesJSON)

	// Upsert by storefront ID
	if err := s.repo.UpsertByStorefrontID(settings); err != nil {
		return nil, fmt.Errorf("failed to save storefront theme settings: %w", err)
	}

	// Fetch the saved settings
	savedSettings, err := s.repo.GetByStorefrontID(storefrontID)
	if err != nil {
		return nil, err
	}

	// Save to history
	if err := s.saveHistory(savedSettings, "Settings saved", userID); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save history: %v\n", err)
	}

	return savedSettings, nil
}

// Update updates storefront theme settings
func (s *storefrontThemeService) Update(tenantID uuid.UUID, req *models.UpdateStorefrontThemeRequest, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Get existing settings
	settings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create with defaults and then update
			createReq := &models.CreateStorefrontThemeRequest{
				ThemeTemplate:  "vibrant",
				PrimaryColor:   "#8B5CF6",
				SecondaryColor: "#EC4899",
			}
			settings, err = s.CreateOrUpdate(tenantID, createReq, userID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Update fields if provided
	if req.ThemeTemplate != nil {
		settings.ThemeTemplate = *req.ThemeTemplate
	}
	if req.PrimaryColor != nil {
		settings.PrimaryColor = *req.PrimaryColor
	}
	if req.SecondaryColor != nil {
		settings.SecondaryColor = *req.SecondaryColor
	}
	if req.AccentColor != nil {
		settings.AccentColor = req.AccentColor
	}
	if req.LogoURL != nil {
		settings.LogoURL = req.LogoURL
	}
	if req.FaviconURL != nil {
		settings.FaviconURL = req.FaviconURL
	}
	if req.FontPrimary != nil {
		settings.FontPrimary = *req.FontPrimary
	}
	if req.FontSecondary != nil {
		settings.FontSecondary = *req.FontSecondary
	}
	if req.ColorMode != nil {
		settings.ColorMode = *req.ColorMode
	}
	if req.CustomCSS != nil {
		settings.CustomCSS = req.CustomCSS
	}

	// Update config objects if provided
	if req.HeaderConfig != nil {
		headerJSON, err := json.Marshal(req.HeaderConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal header config: %w", err)
		}
		settings.HeaderConfig = datatypes.JSON(headerJSON)
	}

	if req.HomepageConfig != nil {
		homepageJSON, err := json.Marshal(req.HomepageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal homepage config: %w", err)
		}
		settings.HomepageConfig = datatypes.JSON(homepageJSON)
	}

	if req.FooterConfig != nil {
		footerJSON, err := json.Marshal(req.FooterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal footer config: %w", err)
		}
		settings.FooterConfig = datatypes.JSON(footerJSON)
	}

	if req.ProductConfig != nil {
		productJSON, err := json.Marshal(req.ProductConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal product config: %w", err)
		}
		settings.ProductConfig = datatypes.JSON(productJSON)
	}

	if req.CheckoutConfig != nil {
		checkoutJSON, err := json.Marshal(req.CheckoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal checkout config: %w", err)
		}
		settings.CheckoutConfig = datatypes.JSON(checkoutJSON)
	}

	// Update enhanced config objects if provided
	if req.TypographyConfig != nil {
		typographyJSON, err := json.Marshal(req.TypographyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal typography config: %w", err)
		}
		settings.TypographyConfig = datatypes.JSON(typographyJSON)
	}

	if req.LayoutConfig != nil {
		layoutJSON, err := json.Marshal(req.LayoutConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal layout config: %w", err)
		}
		settings.LayoutConfig = datatypes.JSON(layoutJSON)
	}

	if req.SpacingStyleConfig != nil {
		spacingJSON, err := json.Marshal(req.SpacingStyleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spacing style config: %w", err)
		}
		settings.SpacingStyleConfig = datatypes.JSON(spacingJSON)
	}

	if req.MobileConfig != nil {
		mobileJSON, err := json.Marshal(req.MobileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mobile config: %w", err)
		}
		settings.MobileConfig = datatypes.JSON(mobileJSON)
	}

	if req.AdvancedConfig != nil {
		advancedJSON, err := json.Marshal(req.AdvancedConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal advanced config: %w", err)
		}
		settings.AdvancedConfig = datatypes.JSON(advancedJSON)
	}

	// Handle content pages
	if req.ContentPages != nil {
		contentPagesJSON, err := json.Marshal(req.ContentPages)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal content pages: %w", err)
		}
		settings.ContentPages = datatypes.JSON(contentPagesJSON)
	}

	settings.UpdatedBy = userID

	// Save the updated settings
	if err := s.repo.Update(settings); err != nil {
		return nil, fmt.Errorf("failed to update storefront theme settings: %w", err)
	}

	return settings, nil
}

// Delete deletes storefront theme settings for a tenant
func (s *storefrontThemeService) Delete(tenantID uuid.UUID) error {
	settings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		return err
	}
	return s.repo.Delete(settings.ID)
}

// GetPresets returns all available theme presets
func (s *storefrontThemeService) GetPresets() []models.StorefrontThemePreset {
	return models.DefaultThemePresets
}

// ApplyPreset applies a theme preset to the storefront settings
func (s *storefrontThemeService) ApplyPreset(tenantID uuid.UUID, presetID string, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Find the preset
	var preset *models.StorefrontThemePreset
	for _, p := range models.DefaultThemePresets {
		if p.ID == presetID {
			preset = &p
			break
		}
	}

	if preset == nil {
		return nil, fmt.Errorf("preset not found: %s", presetID)
	}

	// Create update request with preset values
	updateReq := &models.UpdateStorefrontThemeRequest{
		ThemeTemplate:  &presetID,
		PrimaryColor:   &preset.PrimaryColor,
		SecondaryColor: &preset.SecondaryColor,
	}

	if preset.AccentColor != "" {
		updateReq.AccentColor = &preset.AccentColor
	}

	return s.Update(tenantID, updateReq, userID)
}

// GetDefaults returns default storefront theme settings
func (s *storefrontThemeService) GetDefaults() *models.StorefrontThemeSettings {
	defaultHeader := models.GetDefaultHeaderConfig()
	headerJSON, _ := json.Marshal(defaultHeader)

	defaultHomepage := models.GetDefaultHomepageConfig()
	homepageJSON, _ := json.Marshal(defaultHomepage)

	defaultFooter := models.GetDefaultFooterConfig()
	footerJSON, _ := json.Marshal(defaultFooter)

	defaultProduct := models.GetDefaultProductConfig()
	productJSON, _ := json.Marshal(defaultProduct)

	defaultCheckout := models.GetDefaultCheckoutConfig()
	checkoutJSON, _ := json.Marshal(defaultCheckout)

	// Add default content pages
	defaultContentPages := models.GetDefaultContentPages()
	contentPagesJSON, _ := json.Marshal(defaultContentPages)

	return &models.StorefrontThemeSettings{
		ID:             uuid.Nil,
		ThemeTemplate:  "vibrant",
		PrimaryColor:   "#8B5CF6",
		SecondaryColor: "#EC4899",
		FontPrimary:    "Inter",
		FontSecondary:  "system-ui",
		ColorMode:      "both",
		HeaderConfig:   datatypes.JSON(headerJSON),
		HomepageConfig: datatypes.JSON(homepageJSON),
		FooterConfig:   datatypes.JSON(footerJSON),
		ProductConfig:  datatypes.JSON(productJSON),
		CheckoutConfig: datatypes.JSON(checkoutJSON),
		ContentPages:   datatypes.JSON(contentPagesJSON),
		Version:        1,
	}
}

// CloneTheme clones theme settings from one tenant to another
func (s *storefrontThemeService) CloneTheme(sourceTenantID, targetTenantID uuid.UUID, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Get source settings
	sourceSettings, err := s.GetByTenantID(sourceTenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source theme settings: %w", err)
	}

	// Create a copy for the target tenant
	targetSettings := &models.StorefrontThemeSettings{
		ID:                 uuid.New(),
		TenantID:           targetTenantID,
		ThemeTemplate:      sourceSettings.ThemeTemplate,
		PrimaryColor:       sourceSettings.PrimaryColor,
		SecondaryColor:     sourceSettings.SecondaryColor,
		AccentColor:        sourceSettings.AccentColor,
		LogoURL:            sourceSettings.LogoURL,
		FaviconURL:         sourceSettings.FaviconURL,
		FontPrimary:        sourceSettings.FontPrimary,
		FontSecondary:      sourceSettings.FontSecondary,
		HeaderConfig:       sourceSettings.HeaderConfig,
		HomepageConfig:     sourceSettings.HomepageConfig,
		FooterConfig:       sourceSettings.FooterConfig,
		ProductConfig:      sourceSettings.ProductConfig,
		CheckoutConfig:     sourceSettings.CheckoutConfig,
		CustomCSS:          sourceSettings.CustomCSS,
		TypographyConfig:   sourceSettings.TypographyConfig,
		LayoutConfig:       sourceSettings.LayoutConfig,
		SpacingStyleConfig: sourceSettings.SpacingStyleConfig,
		AdvancedConfig:     sourceSettings.AdvancedConfig,
		MobileConfig:       sourceSettings.MobileConfig,
		ContentPages:       sourceSettings.ContentPages,
		Version:            1,
		CreatedBy:          userID,
		UpdatedBy:          userID,
	}

	// Upsert the cloned settings
	if err := s.repo.Upsert(targetSettings); err != nil {
		return nil, fmt.Errorf("failed to save cloned theme settings: %w", err)
	}

	// Fetch and return the saved settings
	return s.repo.GetByTenantID(targetTenantID)
}

// saveHistory saves a snapshot of the current settings to history
func (s *storefrontThemeService) saveHistory(settings *models.StorefrontThemeSettings, changeSummary string, userID *uuid.UUID) error {
	// Create snapshot JSON
	snapshot, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to create settings snapshot: %w", err)
	}

	history := &models.StorefrontThemeHistory{
		ID:              uuid.New(),
		ThemeSettingsID: settings.ID,
		TenantID:        settings.TenantID,
		Version:         settings.Version,
		Snapshot:        datatypes.JSON(snapshot),
		CreatedBy:       userID,
	}

	if changeSummary != "" {
		history.ChangeSummary = &changeSummary
	}

	if err := s.repo.CreateHistory(history); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}

	// Keep only last 20 versions
	if err := s.repo.DeleteOldHistory(settings.ID, 20); err != nil {
		// Log but don't fail - cleanup is not critical
		fmt.Printf("Warning: failed to cleanup old history: %v\n", err)
	}

	return nil
}

// GetHistory retrieves version history for a tenant's theme settings
func (s *storefrontThemeService) GetHistory(tenantID uuid.UUID, limit int) ([]models.StorefrontThemeHistory, error) {
	settings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []models.StorefrontThemeHistory{}, nil
		}
		return nil, err
	}

	return s.repo.GetHistory(settings.ID, limit)
}

// GetHistoryVersion retrieves a specific version from history
func (s *storefrontThemeService) GetHistoryVersion(tenantID uuid.UUID, version int) (*models.StorefrontThemeHistory, error) {
	settings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		return nil, fmt.Errorf("settings not found: %w", err)
	}

	return s.repo.GetHistoryByVersion(settings.ID, version)
}

// RestoreVersion restores settings from a historical version
func (s *storefrontThemeService) RestoreVersion(tenantID uuid.UUID, version int, userID *uuid.UUID) (*models.StorefrontThemeSettings, error) {
	// Get the history record
	history, err := s.GetHistoryVersion(tenantID, version)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	// Unmarshal the snapshot into settings
	var restoredSettings models.StorefrontThemeSettings
	if err := json.Unmarshal(history.Snapshot, &restoredSettings); err != nil {
		return nil, fmt.Errorf("failed to parse history snapshot: %w", err)
	}

	// Get current settings to preserve ID and update version
	currentSettings, err := s.repo.GetByTenantID(tenantID)
	if err != nil {
		return nil, fmt.Errorf("current settings not found: %w", err)
	}

	// Update restored settings with current ID and increment version
	restoredSettings.ID = currentSettings.ID
	restoredSettings.TenantID = tenantID
	restoredSettings.Version = currentSettings.Version + 1
	restoredSettings.UpdatedBy = userID
	restoredSettings.CreatedAt = currentSettings.CreatedAt
	restoredSettings.CreatedBy = currentSettings.CreatedBy

	// Save the restored settings
	if err := s.repo.Update(&restoredSettings); err != nil {
		return nil, fmt.Errorf("failed to restore settings: %w", err)
	}

	// Save history entry for the restore
	changeSummary := fmt.Sprintf("Restored from version %d", version)
	if err := s.saveHistory(&restoredSettings, changeSummary, userID); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save history after restore: %v\n", err)
	}

	return &restoredSettings, nil
}
