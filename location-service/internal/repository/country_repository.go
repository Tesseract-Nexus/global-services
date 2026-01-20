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

// Cache TTL constants for countries - static reference data
const (
	CountryCacheTTL     = 24 * time.Hour // Countries rarely change
	CountryListCacheTTL = 1 * time.Hour  // List cache
)

// CountryRepository interface for country operations
type CountryRepository interface {
	GetAll(ctx context.Context, search string, region string, limit, offset int) ([]models.Country, int64, error)
	GetByID(ctx context.Context, id string) (*models.Country, error)
	Create(ctx context.Context, country *models.Country) error
	Update(ctx context.Context, country *models.Country) error
	Delete(ctx context.Context, id string) error
	// Health check methods
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

// countryRepository implements CountryRepository
type countryRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewCountryRepository creates a new country repository with optional Redis caching
func NewCountryRepository(db *gorm.DB, redisClient *redis.Client) CountryRepository {
	repo := &countryRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 500,
			L1TTL:      5 * time.Minute,
			DefaultTTL: CountryCacheTTL,
			KeyPrefix:  "tesseract:location:country:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateCountryCacheKey creates a cache key for country lookups
func generateCountryCacheKey(id string) string {
	return fmt.Sprintf("id:%s", id)
}

// invalidateCountryCaches invalidates all country-related caches
func (r *countryRepository) invalidateCountryCaches(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}
	// Invalidate specific country cache
	_ = r.cache.Delete(ctx, generateCountryCacheKey(id))
	// Invalidate list caches
	_ = r.cache.DeletePattern(ctx, "list:*")
}

// RedisHealth returns the health status of Redis connection
func (r *countryRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *countryRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
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

// GetByID retrieves a country by its ID (with caching)
func (r *countryRepository) GetByID(ctx context.Context, id string) (*models.Country, error) {
	cacheKey := generateCountryCacheKey(id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:location:country:"+cacheKey).Result()
		if err == nil {
			var country models.Country
			if err := json.Unmarshal([]byte(val), &country); err == nil {
				return &country, nil
			}
		}
	}

	// Query from database
	var country models.Country
	if err := r.db.WithContext(ctx).Where("id = ? AND active = ?", id, true).First(&country).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(country)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:location:country:"+cacheKey, data, CountryCacheTTL)
		}
	}

	return &country, nil
}

// Create creates a new country
func (r *countryRepository) Create(ctx context.Context, country *models.Country) error {
	err := r.db.WithContext(ctx).Create(country).Error
	if err == nil {
		r.invalidateCountryCaches(ctx, country.ID)
	}
	return err
}

// Update updates an existing country
func (r *countryRepository) Update(ctx context.Context, country *models.Country) error {
	err := r.db.WithContext(ctx).Save(country).Error
	if err == nil {
		r.invalidateCountryCaches(ctx, country.ID)
	}
	return err
}

// Delete soft deletes a country
func (r *countryRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Country{}).Error
	if err == nil {
		r.invalidateCountryCaches(ctx, id)
	}
	return err
}
