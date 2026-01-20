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

// Cache TTL constants for timezones - static reference data
const (
	TimezoneCacheTTL     = 24 * time.Hour // Timezones rarely change
	TimezoneListCacheTTL = 1 * time.Hour  // List cache
)

// TimezoneRepository interface for timezone operations
type TimezoneRepository interface {
	GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.Timezone, int64, error)
	GetByID(ctx context.Context, id string) (*models.Timezone, error)
	Create(ctx context.Context, timezone *models.Timezone) error
	Update(ctx context.Context, timezone *models.Timezone) error
	Delete(ctx context.Context, id string) error
	BulkUpsert(ctx context.Context, timezones []models.Timezone) error
	// Health check methods
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

// timezoneRepository implements TimezoneRepository
type timezoneRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewTimezoneRepository creates a new timezone repository with optional Redis caching
func NewTimezoneRepository(db *gorm.DB, redisClient *redis.Client) TimezoneRepository {
	repo := &timezoneRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 500,
			L1TTL:      5 * time.Minute,
			DefaultTTL: TimezoneCacheTTL,
			KeyPrefix:  "tesseract:location:timezone:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateTimezoneCacheKey creates a cache key for timezone lookups
func generateTimezoneCacheKey(id string) string {
	return fmt.Sprintf("id:%s", id)
}

// invalidateTimezoneCaches invalidates all timezone-related caches
func (r *timezoneRepository) invalidateTimezoneCaches(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}
	// Invalidate specific timezone cache
	_ = r.cache.Delete(ctx, generateTimezoneCacheKey(id))
	// Invalidate list caches
	_ = r.cache.DeletePattern(ctx, "list:*")
}

// RedisHealth returns the health status of Redis connection
func (r *timezoneRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *timezoneRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
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

// GetByID retrieves a timezone by its ID (with caching)
func (r *timezoneRepository) GetByID(ctx context.Context, id string) (*models.Timezone, error) {
	cacheKey := generateTimezoneCacheKey(id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:location:timezone:"+cacheKey).Result()
		if err == nil {
			var timezone models.Timezone
			if err := json.Unmarshal([]byte(val), &timezone); err == nil {
				return &timezone, nil
			}
		}
	}

	// Query from database
	var timezone models.Timezone
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&timezone).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(timezone)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:location:timezone:"+cacheKey, data, TimezoneCacheTTL)
		}
	}

	return &timezone, nil
}

// Create creates a new timezone
func (r *timezoneRepository) Create(ctx context.Context, timezone *models.Timezone) error {
	err := r.db.WithContext(ctx).Create(timezone).Error
	if err == nil {
		r.invalidateTimezoneCaches(ctx, timezone.ID)
	}
	return err
}

// Update updates an existing timezone
func (r *timezoneRepository) Update(ctx context.Context, timezone *models.Timezone) error {
	err := r.db.WithContext(ctx).Save(timezone).Error
	if err == nil {
		r.invalidateTimezoneCaches(ctx, timezone.ID)
	}
	return err
}

// Delete deletes a timezone
func (r *timezoneRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Timezone{}).Error
	if err == nil {
		r.invalidateTimezoneCaches(ctx, id)
	}
	return err
}

// BulkUpsert inserts or updates multiple timezones
func (r *timezoneRepository) BulkUpsert(ctx context.Context, timezones []models.Timezone) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, timezone := range timezones {
			if err := tx.Save(&timezone).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		// Invalidate all timezone caches
		for _, timezone := range timezones {
			r.invalidateTimezoneCaches(ctx, timezone.ID)
		}
	}
	return err
}
