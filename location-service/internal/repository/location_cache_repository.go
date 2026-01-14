package repository

import (
	"context"
	"time"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"gorm.io/gorm"
)

// LocationCacheRepository interface for location cache operations
type LocationCacheRepository interface {
	GetByIP(ctx context.Context, ip string) (*models.LocationCache, error)
	Set(ctx context.Context, cache *models.LocationCache) error
	Delete(ctx context.Context, ip string) error
	DeleteExpired(ctx context.Context) (int64, error)
	GetStats(ctx context.Context) (*CacheStats, error)
}

// CacheStats holds cache statistics
type CacheStats struct {
	TotalEntries   int64
	ExpiredEntries int64
	ValidEntries   int64
}

// locationCacheRepository implements LocationCacheRepository
type locationCacheRepository struct {
	db *gorm.DB
}

// NewLocationCacheRepository creates a new location cache repository
func NewLocationCacheRepository(db *gorm.DB) LocationCacheRepository {
	return &locationCacheRepository{db: db}
}

// GetByIP retrieves a cached location by IP address
func (r *locationCacheRepository) GetByIP(ctx context.Context, ip string) (*models.LocationCache, error) {
	var cache models.LocationCache
	if err := r.db.WithContext(ctx).
		Preload("Country").
		Preload("State").
		Preload("Timezone").
		Where("ip = ? AND expires_at > ?", ip, time.Now()).
		First(&cache).Error; err != nil {
		return nil, err
	}
	return &cache, nil
}

// Set creates or updates a cache entry
func (r *locationCacheRepository) Set(ctx context.Context, cache *models.LocationCache) error {
	// Use upsert to handle existing entries
	return r.db.WithContext(ctx).
		Where("ip = ?", cache.IP).
		Assign(cache).
		FirstOrCreate(cache).Error
}

// Delete removes a cache entry by IP
func (r *locationCacheRepository) Delete(ctx context.Context, ip string) error {
	return r.db.WithContext(ctx).Where("ip = ?", ip).Delete(&models.LocationCache{}).Error
}

// DeleteExpired removes all expired cache entries and returns count
func (r *locationCacheRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.LocationCache{})
	return result.RowsAffected, result.Error
}

// GetStats returns cache statistics
func (r *locationCacheRepository) GetStats(ctx context.Context) (*CacheStats, error) {
	var stats CacheStats

	// Total entries
	if err := r.db.WithContext(ctx).Model(&models.LocationCache{}).Count(&stats.TotalEntries).Error; err != nil {
		return nil, err
	}

	// Expired entries
	if err := r.db.WithContext(ctx).Model(&models.LocationCache{}).Where("expires_at < ?", time.Now()).Count(&stats.ExpiredEntries).Error; err != nil {
		return nil, err
	}

	stats.ValidEntries = stats.TotalEntries - stats.ExpiredEntries

	return &stats, nil
}
