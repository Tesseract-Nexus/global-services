package repository

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/tesseract-hub/settings-service/internal/models"
	"gorm.io/gorm"
)

type SettingsRepository interface {
	Create(settings *models.Settings) error
	GetByID(id uuid.UUID) (*models.Settings, error)
	GetByContext(context models.SettingsContext) (*models.Settings, error)
	Update(settings *models.Settings) error
	Delete(id uuid.UUID) error
	List(filters SettingsFilters) ([]models.Settings, int64, error)
	
	// Preset operations
	CreatePreset(preset *models.SettingsPreset) error
	GetPresetByID(id uuid.UUID) (*models.SettingsPreset, error)
	ListPresets(filters PresetFilters) ([]models.SettingsPreset, int64, error)
	UpdatePreset(preset *models.SettingsPreset) error
	DeletePreset(id uuid.UUID) error
	
	// History operations
	CreateHistory(history *models.SettingsHistory) error
	GetHistory(settingsID uuid.UUID, limit int) ([]models.SettingsHistory, error)
}

type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository creates a new settings repository
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

// ==========================================
// SETTINGS OPERATIONS
// ==========================================

type SettingsFilters struct {
	TenantID      *uuid.UUID
	ApplicationID *uuid.UUID
	UserID        *uuid.UUID
	Scope         *string
	Page          int
	Limit         int
	SortBy        string
	SortOrder     string
}

func (r *settingsRepository) Create(settings *models.Settings) error {
	return r.db.Create(settings).Error
}

func (r *settingsRepository) GetByID(id uuid.UUID) (*models.Settings, error) {
	var settings models.Settings
	err := r.db.First(&settings, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *settingsRepository) GetByContext(context models.SettingsContext) (*models.Settings, error) {
	var settings models.Settings
	query := r.db.Where("tenant_id = ? AND application_id = ? AND scope = ?", 
		context.TenantID, context.ApplicationID, context.Scope)
	
	if context.UserID != nil {
		query = query.Where("user_id = ?", *context.UserID)
	} else {
		query = query.Where("user_id IS NULL")
	}
	
	err := query.First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *settingsRepository) Update(settings *models.Settings) error {
	settings.Version++
	return r.db.Save(settings).Error
}

func (r *settingsRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Settings{}, "id = ?", id).Error
}

func (r *settingsRepository) List(filters SettingsFilters) ([]models.Settings, int64, error) {
	var settings []models.Settings
	var total int64
	
	query := r.db.Model(&models.Settings{})
	
	// Apply filters
	if filters.TenantID != nil {
		query = query.Where("tenant_id = ?", *filters.TenantID)
	}
	if filters.ApplicationID != nil {
		query = query.Where("application_id = ?", *filters.ApplicationID)
	}
	if filters.UserID != nil {
		query = query.Where("user_id = ?", *filters.UserID)
	}
	if filters.Scope != nil {
		query = query.Where("scope = ?", *filters.Scope)
	}
	
	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Apply pagination and sorting
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 {
		filters.Limit = 20
	}
	
	offset := (filters.Page - 1) * filters.Limit
	
	// Apply sorting
	sortBy := "created_at"
	if filters.SortBy != "" {
		sortBy = filters.SortBy
	}
	sortOrder := "DESC"
	if filters.SortOrder != "" {
		sortOrder = filters.SortOrder
	}
	
	orderClause := fmt.Sprintf("%s %s", sortBy, sortOrder)
	
	err := query.Order(orderClause).Offset(offset).Limit(filters.Limit).Find(&settings).Error
	return settings, total, err
}

// ==========================================
// PRESET OPERATIONS
// ==========================================

type PresetFilters struct {
	Category  *string
	Tags      []string
	IsDefault *bool
	Page      int
	Limit     int
	SortBy    string
	SortOrder string
}

func (r *settingsRepository) CreatePreset(preset *models.SettingsPreset) error {
	return r.db.Create(preset).Error
}

func (r *settingsRepository) GetPresetByID(id uuid.UUID) (*models.SettingsPreset, error) {
	var preset models.SettingsPreset
	err := r.db.First(&preset, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &preset, nil
}

func (r *settingsRepository) ListPresets(filters PresetFilters) ([]models.SettingsPreset, int64, error) {
	var presets []models.SettingsPreset
	var total int64
	
	query := r.db.Model(&models.SettingsPreset{})
	
	// Apply filters
	if filters.Category != nil {
		query = query.Where("category = ?", *filters.Category)
	}
	if filters.IsDefault != nil {
		query = query.Where("is_default = ?", *filters.IsDefault)
	}
	if len(filters.Tags) > 0 {
		// PostgreSQL JSONB array contains operation
		for _, tag := range filters.Tags {
			query = query.Where("tags::jsonb ? ?", tag)
		}
	}
	
	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Apply pagination and sorting
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 {
		filters.Limit = 20
	}
	
	offset := (filters.Page - 1) * filters.Limit
	
	// Apply sorting
	sortBy := "created_at"
	if filters.SortBy != "" {
		sortBy = filters.SortBy
	}
	sortOrder := "DESC"
	if filters.SortOrder != "" {
		sortOrder = filters.SortOrder
	}
	
	orderClause := fmt.Sprintf("%s %s", sortBy, sortOrder)
	
	err := query.Order(orderClause).Offset(offset).Limit(filters.Limit).Find(&presets).Error
	return presets, total, err
}

func (r *settingsRepository) UpdatePreset(preset *models.SettingsPreset) error {
	return r.db.Save(preset).Error
}

func (r *settingsRepository) DeletePreset(id uuid.UUID) error {
	return r.db.Delete(&models.SettingsPreset{}, "id = ?", id).Error
}

// ==========================================
// HISTORY OPERATIONS
// ==========================================

func (r *settingsRepository) CreateHistory(history *models.SettingsHistory) error {
	return r.db.Create(history).Error
}

func (r *settingsRepository) GetHistory(settingsID uuid.UUID, limit int) ([]models.SettingsHistory, error) {
	var history []models.SettingsHistory
	
	if limit <= 0 {
		limit = 50 // Default limit
	}
	
	err := r.db.Where("settings_id = ?", settingsID).
		Order("created_at DESC").
		Limit(limit).
		Find(&history).Error
		
	return history, err
}