package repository

import (
	"github.com/google/uuid"
	"settings-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StorefrontThemeRepository defines the interface for storefront theme data access
type StorefrontThemeRepository interface {
	Create(settings *models.StorefrontThemeSettings) error
	GetByID(id uuid.UUID) (*models.StorefrontThemeSettings, error)
	GetByTenantID(tenantID uuid.UUID) (*models.StorefrontThemeSettings, error)
	GetByStorefrontID(storefrontID uuid.UUID) (*models.StorefrontThemeSettings, error)
	Update(settings *models.StorefrontThemeSettings) error
	Delete(id uuid.UUID) error
	Upsert(settings *models.StorefrontThemeSettings) error
	UpsertByStorefrontID(settings *models.StorefrontThemeSettings) error
	List(page, limit int) ([]models.StorefrontThemeSettings, int64, error)
	// History methods
	CreateHistory(history *models.StorefrontThemeHistory) error
	GetHistory(settingsID uuid.UUID, limit int) ([]models.StorefrontThemeHistory, error)
	GetHistoryByVersion(settingsID uuid.UUID, version int) (*models.StorefrontThemeHistory, error)
	DeleteOldHistory(settingsID uuid.UUID, keepCount int) error
}

type storefrontThemeRepository struct {
	db *gorm.DB
}

// NewStorefrontThemeRepository creates a new storefront theme repository
func NewStorefrontThemeRepository(db *gorm.DB) StorefrontThemeRepository {
	return &storefrontThemeRepository{db: db}
}

// Create creates a new storefront theme settings record
func (r *storefrontThemeRepository) Create(settings *models.StorefrontThemeSettings) error {
	return r.db.Create(settings).Error
}

// GetByID retrieves storefront theme settings by ID
func (r *storefrontThemeRepository) GetByID(id uuid.UUID) (*models.StorefrontThemeSettings, error) {
	var settings models.StorefrontThemeSettings
	err := r.db.First(&settings, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetByTenantID retrieves storefront theme settings by tenant ID
func (r *storefrontThemeRepository) GetByTenantID(tenantID uuid.UUID) (*models.StorefrontThemeSettings, error) {
	var settings models.StorefrontThemeSettings
	err := r.db.First(&settings, "tenant_id = ?", tenantID).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetByStorefrontID retrieves storefront theme settings by storefront ID
func (r *storefrontThemeRepository) GetByStorefrontID(storefrontID uuid.UUID) (*models.StorefrontThemeSettings, error) {
	var settings models.StorefrontThemeSettings
	err := r.db.First(&settings, "storefront_id = ?", storefrontID).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// Update updates existing storefront theme settings
func (r *storefrontThemeRepository) Update(settings *models.StorefrontThemeSettings) error {
	settings.Version++
	return r.db.Save(settings).Error
}

// Delete soft-deletes storefront theme settings by ID
func (r *storefrontThemeRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.StorefrontThemeSettings{}, "id = ?", id).Error
}

// Upsert creates or updates storefront theme settings based on tenant ID
// Uses database-level UPSERT to avoid race conditions
func (r *storefrontThemeRepository) Upsert(settings *models.StorefrontThemeSettings) error {
	// Try to find existing settings for this tenant (or storefront as fallback)
	var existing models.StorefrontThemeSettings
	err := r.db.Where("tenant_id = ? OR storefront_id = ?", settings.TenantID, settings.StorefrontID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new settings - use Clauses to handle race condition
		return r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}},
			UpdateAll: true,
		}).Create(settings).Error
	}

	if err != nil {
		return err
	}

	// Update existing settings
	settings.ID = existing.ID
	settings.Version = existing.Version + 1
	settings.CreatedAt = existing.CreatedAt
	settings.CreatedBy = existing.CreatedBy

	return r.db.Save(settings).Error
}

// UpsertByStorefrontID creates or updates storefront theme settings based on storefront ID
// Uses database-level UPSERT to avoid race conditions
func (r *storefrontThemeRepository) UpsertByStorefrontID(settings *models.StorefrontThemeSettings) error {
	// Try to find existing settings for this storefront (or tenant as fallback)
	var existing models.StorefrontThemeSettings
	err := r.db.Where("storefront_id = ? OR tenant_id = ?", settings.StorefrontID, settings.TenantID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new settings - use Clauses to handle race condition
		return r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "storefront_id"}},
			UpdateAll: true,
		}).Create(settings).Error
	}

	if err != nil {
		return err
	}

	// Update existing settings
	settings.ID = existing.ID
	settings.Version = existing.Version + 1
	settings.CreatedAt = existing.CreatedAt
	settings.CreatedBy = existing.CreatedBy

	return r.db.Save(settings).Error
}

// List retrieves all storefront theme settings with pagination
func (r *storefrontThemeRepository) List(page, limit int) ([]models.StorefrontThemeSettings, int64, error) {
	var settings []models.StorefrontThemeSettings
	var total int64

	// Count total records
	if err := r.db.Model(&models.StorefrontThemeSettings{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	offset := (page - 1) * limit

	err := r.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&settings).Error
	return settings, total, err
}

// CreateHistory creates a new history record
func (r *storefrontThemeRepository) CreateHistory(history *models.StorefrontThemeHistory) error {
	return r.db.Create(history).Error
}

// GetHistory retrieves history records for a settings ID with limit
func (r *storefrontThemeRepository) GetHistory(settingsID uuid.UUID, limit int) ([]models.StorefrontThemeHistory, error) {
	var history []models.StorefrontThemeHistory
	if limit <= 0 {
		limit = 20
	}
	err := r.db.Where("theme_settings_id = ?", settingsID).
		Order("version DESC").
		Limit(limit).
		Find(&history).Error
	return history, err
}

// GetHistoryByVersion retrieves a specific history record by version
func (r *storefrontThemeRepository) GetHistoryByVersion(settingsID uuid.UUID, version int) (*models.StorefrontThemeHistory, error) {
	var history models.StorefrontThemeHistory
	err := r.db.Where("theme_settings_id = ? AND version = ?", settingsID, version).
		First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// DeleteOldHistory deletes old history records, keeping only the specified count
func (r *storefrontThemeRepository) DeleteOldHistory(settingsID uuid.UUID, keepCount int) error {
	// Get the version number at the cutoff point
	var cutoffHistory models.StorefrontThemeHistory
	err := r.db.Where("theme_settings_id = ?", settingsID).
		Order("version DESC").
		Offset(keepCount).
		First(&cutoffHistory).Error

	if err == gorm.ErrRecordNotFound {
		// Not enough records to delete
		return nil
	}
	if err != nil {
		return err
	}

	// Delete all records with version <= cutoff version
	return r.db.Where("theme_settings_id = ? AND version <= ?", settingsID, cutoffHistory.Version).
		Delete(&models.StorefrontThemeHistory{}).Error
}
