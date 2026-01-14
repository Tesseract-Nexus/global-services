package repository

import (
	"context"
	"time"

	"github.com/tesseract-hub/domains/common/services/verification-service/internal/models"
	"gorm.io/gorm"
)

// RateLimitRepository handles database operations for rate limiting
type RateLimitRepository struct {
	db *gorm.DB
}

// NewRateLimitRepository creates a new rate limit repository
func NewRateLimitRepository(db *gorm.DB) *RateLimitRepository {
	return &RateLimitRepository{db: db}
}

// GetOrCreate retrieves or creates a rate limit record
func (r *RateLimitRepository) GetOrCreate(ctx context.Context, identifier, limitType string, windowDuration time.Duration) (*models.RateLimit, error) {
	var rateLimit models.RateLimit

	// Try to get existing record
	err := r.db.WithContext(ctx).
		Where("identifier = ? AND type = ?", identifier, limitType).
		First(&rateLimit).Error

	if err == gorm.ErrRecordNotFound {
		// Create new record
		now := time.Now()
		rateLimit = models.RateLimit{
			Identifier:  identifier,
			Type:        limitType,
			Count:       0,
			WindowStart: now,
			WindowEnd:   now.Add(windowDuration),
		}
		if err := r.db.WithContext(ctx).Create(&rateLimit).Error; err != nil {
			return nil, err
		}
		return &rateLimit, nil
	}

	if err != nil {
		return nil, err
	}

	// Check if window should be reset
	if rateLimit.ShouldReset() {
		now := time.Now()
		rateLimit.Count = 0
		rateLimit.WindowStart = now
		rateLimit.WindowEnd = now.Add(windowDuration)
		if err := r.db.WithContext(ctx).Save(&rateLimit).Error; err != nil {
			return nil, err
		}
	}

	return &rateLimit, nil
}

// Increment increments the rate limit counter
func (r *RateLimitRepository) Increment(ctx context.Context, identifier, limitType string) error {
	return r.db.WithContext(ctx).Model(&models.RateLimit{}).
		Where("identifier = ? AND type = ?", identifier, limitType).
		UpdateColumn("count", gorm.Expr("count + ?", 1)).Error
}

// CheckLimit checks if the rate limit has been exceeded
func (r *RateLimitRepository) CheckLimit(ctx context.Context, identifier, limitType string, maxCount int, windowDuration time.Duration) (bool, int, error) {
	rateLimit, err := r.GetOrCreate(ctx, identifier, limitType, windowDuration)
	if err != nil {
		return false, 0, err
	}

	exceeded := rateLimit.Count >= maxCount
	remaining := maxCount - rateLimit.Count
	if remaining < 0 {
		remaining = 0
	}

	return exceeded, remaining, nil
}

// DeleteExpired deletes expired rate limit records (cleanup)
func (r *RateLimitRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("window_end < ?", time.Now().Add(-24*time.Hour)).
		Delete(&models.RateLimit{}).Error
}
