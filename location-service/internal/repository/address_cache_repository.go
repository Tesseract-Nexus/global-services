package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AddressCacheRepository interface for address cache operations
type AddressCacheRepository interface {
	// GetByHash retrieves a cached entry by type and key hash
	GetByHash(ctx context.Context, cacheType, keyHash string) (*models.AddressCacheEntry, error)

	// Set creates or updates a cache entry (upsert)
	Set(ctx context.Context, entry *models.AddressCacheEntry) error

	// IncrementHits increments the hit count for a cache entry
	IncrementHits(ctx context.Context, id uint64) error

	// Delete removes a cache entry by ID
	Delete(ctx context.Context, id uint64) error

	// DeleteExpired removes expired entries in batches
	DeleteExpired(ctx context.Context, batchSize int) (int64, error)

	// GetStats returns cache statistics
	GetStats(ctx context.Context) (*models.CacheStats, error)

	// GetAllByType retrieves all entries of a specific type (for debugging)
	GetAllByType(ctx context.Context, cacheType string, limit, offset int) ([]models.AddressCacheEntry, int64, error)

	// ClearAll removes all cache entries
	ClearAll(ctx context.Context) (int64, error)
}

// addressCacheRepository implements AddressCacheRepository
type addressCacheRepository struct {
	db *gorm.DB
}

// NewAddressCacheRepository creates a new address cache repository
func NewAddressCacheRepository(db *gorm.DB) AddressCacheRepository {
	return &addressCacheRepository{db: db}
}

// GetByHash retrieves a cached entry by type and key hash (non-expired only)
func (r *addressCacheRepository) GetByHash(ctx context.Context, cacheType, keyHash string) (*models.AddressCacheEntry, error) {
	var entry models.AddressCacheEntry
	err := r.db.WithContext(ctx).
		Where("cache_type = ? AND cache_key_hash = ? AND expires_at > ?",
			cacheType, keyHash, time.Now()).
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// Set creates or updates a cache entry using upsert
func (r *addressCacheRepository) Set(ctx context.Context, entry *models.AddressCacheEntry) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "cache_type"},
				{Name: "cache_key_hash"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"formatted_address",
				"place_id",
				"latitude",
				"longitude",
				"street_number",
				"street_name",
				"city",
				"district",
				"state_code",
				"state_name",
				"country_code",
				"country_name",
				"postal_code",
				"response_json",
				"provider",
				"expires_at",
				"updated_at",
			}),
		}).
		Create(entry).Error
}

// IncrementHits increments the hit count and updates last accessed time
func (r *addressCacheRepository) IncrementHits(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"hit_count":  gorm.Expr("hit_count + 1"),
			"updated_at": time.Now(),
		}).Error
}

// Delete removes a cache entry by ID
func (r *addressCacheRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Delete(&models.AddressCacheEntry{}, id).Error
}

// DeleteExpired removes expired entries in batches and returns count
func (r *addressCacheRepository) DeleteExpired(ctx context.Context, batchSize int) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Limit(batchSize).
		Delete(&models.AddressCacheEntry{})
	return result.RowsAffected, result.Error
}

// GetStats returns comprehensive cache statistics
func (r *addressCacheRepository) GetStats(ctx context.Context) (*models.CacheStats, error) {
	stats := &models.CacheStats{
		ByType: make(map[string]int64),
	}
	now := time.Now()

	// Total entries
	if err := r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Count(&stats.TotalEntries).Error; err != nil {
		return nil, err
	}

	// Expired entries
	if err := r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Where("expires_at < ?", now).
		Count(&stats.ExpiredEntries).Error; err != nil {
		return nil, err
	}

	stats.ValidEntries = stats.TotalEntries - stats.ExpiredEntries

	// Total hits
	var totalHits struct{ Sum int64 }
	r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Select("COALESCE(SUM(hit_count), 0) as sum").
		Scan(&totalHits)
	stats.TotalHits = totalHits.Sum

	// Entries by type
	var typeCounts []struct {
		CacheType string
		Count     int64
	}
	r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Select("cache_type, COUNT(*) as count").
		Group("cache_type").
		Scan(&typeCounts)
	for _, tc := range typeCounts {
		stats.ByType[tc.CacheType] = tc.Count
	}

	// Average hit count
	if stats.TotalEntries > 0 {
		stats.AvgHitCount = float64(stats.TotalHits) / float64(stats.TotalEntries)
	}

	// Oldest and newest entries
	var oldest models.AddressCacheEntry
	if err := r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Order("created_at ASC").
		First(&oldest).Error; err == nil {
		stats.OldestEntry = &oldest.CreatedAt
	}

	var newest models.AddressCacheEntry
	if err := r.db.WithContext(ctx).
		Model(&models.AddressCacheEntry{}).
		Order("created_at DESC").
		First(&newest).Error; err == nil {
		stats.NewestEntry = &newest.CreatedAt
	}

	// Estimate savings (assume $0.003 per API call saved)
	if stats.TotalHits > 0 {
		savings := float64(stats.TotalHits) * 0.003
		stats.EstimatedSavings = "$" + formatFloat(savings, 2)
	}

	return stats, nil
}

// GetAllByType retrieves all entries of a specific type with pagination
func (r *addressCacheRepository) GetAllByType(ctx context.Context, cacheType string, limit, offset int) ([]models.AddressCacheEntry, int64, error) {
	var entries []models.AddressCacheEntry
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AddressCacheEntry{})

	if cacheType != "" {
		query = query.Where("cache_type = ?", cacheType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// ClearAll removes all cache entries
func (r *addressCacheRepository) ClearAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("1=1").
		Delete(&models.AddressCacheEntry{})
	return result.RowsAffected, result.Error
}

// formatFloat formats a float64 to a string with specified decimal places
func formatFloat(f float64, decimals int) string {
	format := "%." + string(rune('0'+decimals)) + "f"
	return fmt.Sprintf(format, f)
}
