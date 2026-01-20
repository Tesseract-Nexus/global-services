package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"location-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for currencies - static reference data
const (
	CurrencyCacheTTL     = 24 * time.Hour // Currencies rarely change
	CurrencyListCacheTTL = 1 * time.Hour  // List cache
)

// CurrencyRepository interface for currency operations
type CurrencyRepository interface {
	GetAll(ctx context.Context, search string, activeOnly bool, limit, offset int) ([]models.Currency, int64, error)
	GetByCode(ctx context.Context, code string) (*models.Currency, error)
	Create(ctx context.Context, currency *models.Currency) error
	Update(ctx context.Context, currency *models.Currency) error
	Delete(ctx context.Context, code string) error
	BulkUpsert(ctx context.Context, currencies []models.Currency) error
	// Health check methods
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

// currencyRepository implements CurrencyRepository
type currencyRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewCurrencyRepository creates a new currency repository with optional Redis caching
func NewCurrencyRepository(db *gorm.DB, redisClient *redis.Client) CurrencyRepository {
	repo := &currencyRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 300,
			L1TTL:      5 * time.Minute,
			DefaultTTL: CurrencyCacheTTL,
			KeyPrefix:  "tesseract:location:currency:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateCurrencyCacheKey creates a cache key for currency lookups
func generateCurrencyCacheKey(code string) string {
	return fmt.Sprintf("code:%s", code)
}

// invalidateCurrencyCaches invalidates all currency-related caches
func (r *currencyRepository) invalidateCurrencyCaches(ctx context.Context, code string) {
	if r.cache == nil {
		return
	}
	// Invalidate specific currency cache
	_ = r.cache.Delete(ctx, generateCurrencyCacheKey(code))
	// Invalidate list caches
	_ = r.cache.DeletePattern(ctx, "list:*")
}

// RedisHealth returns the health status of Redis connection
func (r *currencyRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *currencyRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
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

// GetByCode retrieves a currency by its code (with caching)
func (r *currencyRepository) GetByCode(ctx context.Context, code string) (*models.Currency, error) {
	cacheKey := generateCurrencyCacheKey(code)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:location:currency:"+cacheKey).Result()
		if err == nil {
			var currency models.Currency
			if err := json.Unmarshal([]byte(val), &currency); err == nil {
				return &currency, nil
			}
		}
	}

	// Query from database
	var currency models.Currency
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&currency).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(currency)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:location:currency:"+cacheKey, data, CurrencyCacheTTL)
		}
	}

	return &currency, nil
}

// Create creates a new currency
func (r *currencyRepository) Create(ctx context.Context, currency *models.Currency) error {
	err := r.db.WithContext(ctx).Create(currency).Error
	if err == nil {
		r.invalidateCurrencyCaches(ctx, currency.Code)
	}
	return err
}

// Update updates an existing currency
func (r *currencyRepository) Update(ctx context.Context, currency *models.Currency) error {
	err := r.db.WithContext(ctx).Save(currency).Error
	if err == nil {
		r.invalidateCurrencyCaches(ctx, currency.Code)
	}
	return err
}

// Delete soft deletes a currency
func (r *currencyRepository) Delete(ctx context.Context, code string) error {
	err := r.db.WithContext(ctx).Where("code = ?", code).Delete(&models.Currency{}).Error
	if err == nil {
		r.invalidateCurrencyCaches(ctx, code)
	}
	return err
}

// BulkUpsert inserts or updates multiple currencies
func (r *currencyRepository) BulkUpsert(ctx context.Context, currencies []models.Currency) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, currency := range currencies {
			if err := tx.Save(&currency).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		// Invalidate all currency caches
		for _, currency := range currencies {
			r.invalidateCurrencyCaches(ctx, currency.Code)
		}
	}
	return err
}
