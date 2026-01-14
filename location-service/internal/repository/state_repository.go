package repository

import (
	"context"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"gorm.io/gorm"
)

// StateRepository interface for state operations
type StateRepository interface {
	GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.State, int64, error)
	GetByCountryID(ctx context.Context, countryID string, search string) ([]models.State, error)
	GetByID(ctx context.Context, id string) (*models.State, error)
	Create(ctx context.Context, state *models.State) error
	Update(ctx context.Context, state *models.State) error
	Delete(ctx context.Context, id string) error
}

// stateRepository implements StateRepository
type stateRepository struct {
	db *gorm.DB
}

// NewStateRepository creates a new state repository
func NewStateRepository(db *gorm.DB) StateRepository {
	return &stateRepository{db: db}
}

// GetAll retrieves all states with optional filtering and pagination
func (r *stateRepository) GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.State, int64, error) {
	var states []models.State
	var total int64

	query := r.db.WithContext(ctx).Model(&models.State{}).Where("active = ?", true)

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Apply country filter
	if countryID != "" {
		query = query.Where("country_id = ?", countryID)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	if err := query.Preload("Country").Order("name ASC").Limit(limit).Offset(offset).Find(&states).Error; err != nil {
		return nil, 0, err
	}

	return states, total, nil
}

// GetByCountryID retrieves all states for a specific country
func (r *stateRepository) GetByCountryID(ctx context.Context, countryID string, search string) ([]models.State, error) {
	var states []models.State

	query := r.db.WithContext(ctx).Where("country_id = ? AND active = ?", countryID, true)

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Order("name ASC").Find(&states).Error; err != nil {
		return nil, err
	}

	return states, nil
}

// GetByID retrieves a state by its ID
func (r *stateRepository) GetByID(ctx context.Context, id string) (*models.State, error) {
	var state models.State
	if err := r.db.WithContext(ctx).Preload("Country").Where("id = ? AND active = ?", id, true).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

// Create creates a new state
func (r *stateRepository) Create(ctx context.Context, state *models.State) error {
	return r.db.WithContext(ctx).Create(state).Error
}

// Update updates an existing state
func (r *stateRepository) Update(ctx context.Context, state *models.State) error {
	return r.db.WithContext(ctx).Save(state).Error
}

// Delete soft deletes a state
func (r *stateRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.State{}).Error
}
