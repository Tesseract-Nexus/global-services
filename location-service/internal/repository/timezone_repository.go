package repository

import (
	"context"

	"location-service/internal/models"
	"gorm.io/gorm"
)

// TimezoneRepository interface for timezone operations
type TimezoneRepository interface {
	GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.Timezone, int64, error)
	GetByID(ctx context.Context, id string) (*models.Timezone, error)
	Create(ctx context.Context, timezone *models.Timezone) error
	Update(ctx context.Context, timezone *models.Timezone) error
	Delete(ctx context.Context, id string) error
	BulkUpsert(ctx context.Context, timezones []models.Timezone) error
}

// timezoneRepository implements TimezoneRepository
type timezoneRepository struct {
	db *gorm.DB
}

// NewTimezoneRepository creates a new timezone repository
func NewTimezoneRepository(db *gorm.DB) TimezoneRepository {
	return &timezoneRepository{db: db}
}

// GetAll retrieves all timezones with optional filtering and pagination
func (r *timezoneRepository) GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.Timezone, int64, error) {
	var timezones []models.Timezone
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Timezone{})

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR id ILIKE ? OR abbreviation ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Apply country filter (search in JSON countries array)
	if countryID != "" {
		query = query.Where("countries LIKE ?", "%\""+countryID+"\"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("id ASC").Find(&timezones).Error; err != nil {
		return nil, 0, err
	}

	return timezones, total, nil
}

// GetByID retrieves a timezone by its ID
func (r *timezoneRepository) GetByID(ctx context.Context, id string) (*models.Timezone, error) {
	var timezone models.Timezone
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&timezone).Error; err != nil {
		return nil, err
	}
	return &timezone, nil
}

// Create creates a new timezone
func (r *timezoneRepository) Create(ctx context.Context, timezone *models.Timezone) error {
	return r.db.WithContext(ctx).Create(timezone).Error
}

// Update updates an existing timezone
func (r *timezoneRepository) Update(ctx context.Context, timezone *models.Timezone) error {
	return r.db.WithContext(ctx).Save(timezone).Error
}

// Delete deletes a timezone
func (r *timezoneRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Timezone{}).Error
}

// BulkUpsert inserts or updates multiple timezones
func (r *timezoneRepository) BulkUpsert(ctx context.Context, timezones []models.Timezone) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, timezone := range timezones {
			if err := tx.Save(&timezone).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
