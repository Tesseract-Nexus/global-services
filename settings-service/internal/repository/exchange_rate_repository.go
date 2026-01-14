package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-hub/settings-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExchangeRateRepository defines the interface for exchange rate data access
type ExchangeRateRepository interface {
	// GetRate retrieves the exchange rate for a currency pair
	GetRate(baseCurrency, targetCurrency string) (*models.ExchangeRate, error)

	// GetRatesForBase retrieves all exchange rates for a base currency
	GetRatesForBase(baseCurrency string) ([]models.ExchangeRate, error)

	// GetAllRates retrieves all exchange rates
	GetAllRates() ([]models.ExchangeRate, error)

	// UpsertRate creates or updates an exchange rate
	UpsertRate(rate *models.ExchangeRate) error

	// BulkUpsertRates creates or updates multiple exchange rates in a single transaction
	BulkUpsertRates(rates []models.ExchangeRate) error

	// DeleteOldRates deletes rates older than the specified time
	DeleteOldRates(olderThan time.Time) error

	// GetLatestFetchTime returns the most recent fetch time
	GetLatestFetchTime() (*time.Time, error)
}

type exchangeRateRepository struct {
	db *gorm.DB
}

// NewExchangeRateRepository creates a new exchange rate repository
func NewExchangeRateRepository(db *gorm.DB) ExchangeRateRepository {
	return &exchangeRateRepository{db: db}
}

// GetRate retrieves the exchange rate for a currency pair
func (r *exchangeRateRepository) GetRate(baseCurrency, targetCurrency string) (*models.ExchangeRate, error) {
	var rate models.ExchangeRate
	err := r.db.Where("base_currency = ? AND target_currency = ?", baseCurrency, targetCurrency).
		First(&rate).Error
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

// GetRatesForBase retrieves all exchange rates for a base currency
func (r *exchangeRateRepository) GetRatesForBase(baseCurrency string) ([]models.ExchangeRate, error) {
	var rates []models.ExchangeRate
	err := r.db.Where("base_currency = ?", baseCurrency).
		Order("target_currency ASC").
		Find(&rates).Error
	if err != nil {
		return nil, err
	}
	return rates, nil
}

// GetAllRates retrieves all exchange rates
func (r *exchangeRateRepository) GetAllRates() ([]models.ExchangeRate, error) {
	var rates []models.ExchangeRate
	err := r.db.Order("base_currency ASC, target_currency ASC").
		Find(&rates).Error
	if err != nil {
		return nil, err
	}
	return rates, nil
}

// UpsertRate creates or updates an exchange rate
func (r *exchangeRateRepository) UpsertRate(rate *models.ExchangeRate) error {
	if rate.ID == uuid.Nil {
		rate.ID = uuid.New()
	}

	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "base_currency"}, {Name: "target_currency"}},
		DoUpdates: clause.AssignmentColumns([]string{"rate", "fetched_at", "updated_at"}),
	}).Create(rate).Error
}

// BulkUpsertRates creates or updates multiple exchange rates in a single transaction
func (r *exchangeRateRepository) BulkUpsertRates(rates []models.ExchangeRate) error {
	if len(rates) == 0 {
		return nil
	}

	// Assign UUIDs to rates that don't have them
	for i := range rates {
		if rates[i].ID == uuid.Nil {
			rates[i].ID = uuid.New()
		}
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "base_currency"}, {Name: "target_currency"}},
			DoUpdates: clause.AssignmentColumns([]string{"rate", "fetched_at", "updated_at"}),
		}).CreateInBatches(rates, 100).Error
	})
}

// DeleteOldRates deletes rates older than the specified time
func (r *exchangeRateRepository) DeleteOldRates(olderThan time.Time) error {
	return r.db.Where("fetched_at < ?", olderThan).Delete(&models.ExchangeRate{}).Error
}

// GetLatestFetchTime returns the most recent fetch time
func (r *exchangeRateRepository) GetLatestFetchTime() (*time.Time, error) {
	var rate models.ExchangeRate
	err := r.db.Order("fetched_at DESC").First(&rate).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rate.FetchedAt, nil
}
