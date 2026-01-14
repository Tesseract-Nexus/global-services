package repository

import (
	"context"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"gorm.io/gorm"
)

// CurrencyRepository interface for currency operations
type CurrencyRepository interface {
	GetAll(ctx context.Context, search string, activeOnly bool, limit, offset int) ([]models.Currency, int64, error)
	GetByCode(ctx context.Context, code string) (*models.Currency, error)
	Create(ctx context.Context, currency *models.Currency) error
	Update(ctx context.Context, currency *models.Currency) error
	Delete(ctx context.Context, code string) error
	BulkUpsert(ctx context.Context, currencies []models.Currency) error
}

// currencyRepository implements CurrencyRepository
type currencyRepository struct {
	db *gorm.DB
}

// NewCurrencyRepository creates a new currency repository
func NewCurrencyRepository(db *gorm.DB) CurrencyRepository {
	return &currencyRepository{db: db}
}

// GetAll retrieves all currencies with optional filtering and pagination
func (r *currencyRepository) GetAll(ctx context.Context, search string, activeOnly bool, limit, offset int) ([]models.Currency, int64, error) {
	var currencies []models.Currency
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Currency{})

	// Apply active filter
	if activeOnly {
		query = query.Where("active = ?", true)
	}

	// Apply search filter
	if search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ? OR symbol ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("code ASC").Find(&currencies).Error; err != nil {
		return nil, 0, err
	}

	return currencies, total, nil
}

// GetByCode retrieves a currency by its code
func (r *currencyRepository) GetByCode(ctx context.Context, code string) (*models.Currency, error) {
	var currency models.Currency
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&currency).Error; err != nil {
		return nil, err
	}
	return &currency, nil
}

// Create creates a new currency
func (r *currencyRepository) Create(ctx context.Context, currency *models.Currency) error {
	return r.db.WithContext(ctx).Create(currency).Error
}

// Update updates an existing currency
func (r *currencyRepository) Update(ctx context.Context, currency *models.Currency) error {
	return r.db.WithContext(ctx).Save(currency).Error
}

// Delete soft deletes a currency
func (r *currencyRepository) Delete(ctx context.Context, code string) error {
	return r.db.WithContext(ctx).Where("code = ?", code).Delete(&models.Currency{}).Error
}

// BulkUpsert inserts or updates multiple currencies
func (r *currencyRepository) BulkUpsert(ctx context.Context, currencies []models.Currency) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, currency := range currencies {
			if err := tx.Save(&currency).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
