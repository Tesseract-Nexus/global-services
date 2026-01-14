package repository

import (
	"context"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"gorm.io/gorm"
)

// CountryRepository interface for country operations
type CountryRepository interface {
	GetAll(ctx context.Context, search string, region string, limit, offset int) ([]models.Country, int64, error)
	GetByID(ctx context.Context, id string) (*models.Country, error)
	Create(ctx context.Context, country *models.Country) error
	Update(ctx context.Context, country *models.Country) error
	Delete(ctx context.Context, id string) error
}

// countryRepository implements CountryRepository
type countryRepository struct {
	db *gorm.DB
}

// NewCountryRepository creates a new country repository
func NewCountryRepository(db *gorm.DB) CountryRepository {
	return &countryRepository{db: db}
}

// GetAll retrieves all countries with optional filtering and pagination
func (r *countryRepository) GetAll(ctx context.Context, search string, region string, limit, offset int) ([]models.Country, int64, error) {
	var countries []models.Country
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Country{}).Where("active = ?", true)

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR id ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Apply region filter
	if region != "" {
		query = query.Where("region = ?", region)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	if err := query.Order("name ASC").Limit(limit).Offset(offset).Find(&countries).Error; err != nil {
		return nil, 0, err
	}

	return countries, total, nil
}

// GetByID retrieves a country by its ID
func (r *countryRepository) GetByID(ctx context.Context, id string) (*models.Country, error) {
	var country models.Country
	if err := r.db.WithContext(ctx).Where("id = ? AND active = ?", id, true).First(&country).Error; err != nil {
		return nil, err
	}
	return &country, nil
}

// Create creates a new country
func (r *countryRepository) Create(ctx context.Context, country *models.Country) error {
	return r.db.WithContext(ctx).Create(country).Error
}

// Update updates an existing country
func (r *countryRepository) Update(ctx context.Context, country *models.Country) error {
	return r.db.WithContext(ctx).Save(country).Error
}

// Delete soft deletes a country
func (r *countryRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Country{}).Error
}
