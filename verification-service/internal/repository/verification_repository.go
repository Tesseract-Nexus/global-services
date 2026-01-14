package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"verification-service/internal/models"
	"gorm.io/gorm"
)

// VerificationRepository handles database operations for verification codes
type VerificationRepository struct {
	db *gorm.DB
}

// NewVerificationRepository creates a new verification repository
func NewVerificationRepository(db *gorm.DB) *VerificationRepository {
	return &VerificationRepository{db: db}
}

// Create creates a new verification code
func (r *VerificationRepository) Create(ctx context.Context, code *models.VerificationCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

// GetByCodeHash retrieves a verification code by its hash
func (r *VerificationRepository) GetByCodeHash(ctx context.Context, codeHash string) (*models.VerificationCode, error) {
	var code models.VerificationCode
	err := r.db.WithContext(ctx).
		Where("code_hash = ? AND is_used = ?", codeHash, false).
		Order("created_at DESC").
		First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// GetLatestByRecipient retrieves the latest verification code for a recipient
func (r *VerificationRepository) GetLatestByRecipient(ctx context.Context, recipient, purpose string) (*models.VerificationCode, error) {
	var code models.VerificationCode
	err := r.db.WithContext(ctx).
		Where("recipient = ? AND purpose = ?", recipient, purpose).
		Order("created_at DESC").
		First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// GetActiveByRecipient retrieves active (not expired, not used) verification code
func (r *VerificationRepository) GetActiveByRecipient(ctx context.Context, recipient, purpose string) (*models.VerificationCode, error) {
	var code models.VerificationCode
	err := r.db.WithContext(ctx).
		Where("recipient = ? AND purpose = ? AND is_used = ? AND expires_at > ?",
			recipient, purpose, false, time.Now()).
		Order("created_at DESC").
		First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// Update updates a verification code
func (r *VerificationRepository) Update(ctx context.Context, code *models.VerificationCode) error {
	return r.db.WithContext(ctx).Save(code).Error
}

// MarkAsUsed marks a verification code as used
func (r *VerificationRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.VerificationCode{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_used":     true,
			"verified_at": now,
		}).Error
}

// IncrementAttempts increments the attempt count
func (r *VerificationRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.VerificationCode{}).
		Where("id = ?", id).
		UpdateColumn("attempt_count", gorm.Expr("attempt_count + ?", 1)).Error
}

// CountRecentCodes counts verification codes sent within a time window
func (r *VerificationRepository) CountRecentCodes(ctx context.Context, recipient string, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.VerificationCode{}).
		Where("recipient = ? AND created_at >= ?", recipient, since).
		Count(&count).Error
	return count, err
}

// DeleteExpired deletes expired verification codes (cleanup)
func (r *VerificationRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? AND is_used = ?", time.Now().Add(-24*time.Hour), false).
		Delete(&models.VerificationCode{}).Error
}

// LogAttempt logs a verification attempt
func (r *VerificationRepository) LogAttempt(ctx context.Context, attempt *models.VerificationAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

// GetVerifiedCode retrieves a verified code for a recipient and purpose
func (r *VerificationRepository) GetVerifiedCode(ctx context.Context, recipient, purpose string) (*models.VerificationCode, error) {
	var code models.VerificationCode
	err := r.db.WithContext(ctx).
		Where("recipient = ? AND purpose = ? AND verified_at IS NOT NULL",
			recipient, purpose).
		Order("verified_at DESC").
		First(&code).Error
	if err != nil {
		return nil, err
	}
	return &code, nil
}
