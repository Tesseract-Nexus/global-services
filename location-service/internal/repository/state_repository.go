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

// Cache TTL constants for states - static reference data
const (
	StateCacheTTL     = 24 * time.Hour // States rarely change
	StateListCacheTTL = 1 * time.Hour  // List cache
)

// StateRepository interface for state operations
type StateRepository interface {
	GetAll(ctx context.Context, search string, countryID string, limit, offset int) ([]models.State, int64, error)
	GetByCountryID(ctx context.Context, countryID string, search string) ([]models.State, error)
	GetByID(ctx context.Context, id string) (*models.State, error)
	Create(ctx context.Context, state *models.State) error
	Update(ctx context.Context, state *models.State) error
	Delete(ctx context.Context, id string) error
	// Health check methods
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

// stateRepository implements StateRepository
type stateRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewStateRepository creates a new state repository with optional Redis caching
func NewStateRepository(db *gorm.DB, redisClient *redis.Client) StateRepository {
	repo := &stateRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 2000,
			L1TTL:      5 * time.Minute,
			DefaultTTL: StateCacheTTL,
			KeyPrefix:  "tesseract:location:state:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateStateCacheKey creates a cache key for state lookups
func generateStateCacheKey(id string) string {
	return fmt.Sprintf("id:%s", id)
}

// invalidateStateCaches invalidates all state-related caches
func (r *stateRepository) invalidateStateCaches(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}
	// Invalidate specific state cache
	_ = r.cache.Delete(ctx, generateStateCacheKey(id))
	// Invalidate list caches
	_ = r.cache.DeletePattern(ctx, "list:*")
}

// RedisHealth returns the health status of Redis connection
func (r *stateRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *stateRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
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

// GetByID retrieves a state by its ID (with caching)
func (r *stateRepository) GetByID(ctx context.Context, id string) (*models.State, error) {
	cacheKey := generateStateCacheKey(id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:location:state:"+cacheKey).Result()
		if err == nil {
			var state models.State
			if err := json.Unmarshal([]byte(val), &state); err == nil {
				return &state, nil
			}
		}
	}

	// Query from database
	var state models.State
	if err := r.db.WithContext(ctx).Preload("Country").Where("id = ? AND active = ?", id, true).First(&state).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(state)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:location:state:"+cacheKey, data, StateCacheTTL)
		}
	}

	return &state, nil
}

// Create creates a new state
func (r *stateRepository) Create(ctx context.Context, state *models.State) error {
	err := r.db.WithContext(ctx).Create(state).Error
	if err == nil {
		r.invalidateStateCaches(ctx, state.ID)
	}
	return err
}

// Update updates an existing state
func (r *stateRepository) Update(ctx context.Context, state *models.State) error {
	err := r.db.WithContext(ctx).Save(state).Error
	if err == nil {
		r.invalidateStateCaches(ctx, state.ID)
	}
	return err
}

// Delete soft deletes a state
func (r *stateRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.State{}).Error
	if err == nil {
		r.invalidateStateCaches(ctx, id)
	}
	return err
}
