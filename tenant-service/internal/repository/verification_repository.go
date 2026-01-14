package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"gorm.io/gorm"
)

// VerificationRepository handles verification record operations
type VerificationRepository struct {
	db *gorm.DB
}

// NewVerificationRepository creates a new verification repository
func NewVerificationRepository(db *gorm.DB) *VerificationRepository {
	return &VerificationRepository{
		db: db,
	}
}

// CreateVerificationRecord creates a new verification record
func (r *VerificationRepository) CreateVerificationRecord(ctx context.Context, record *models.VerificationRecord) (*models.VerificationRecord, error) {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create verification record: %w", err)
	}

	return record, nil
}

// GetVerificationRecord retrieves a verification record by ID
func (r *VerificationRepository) GetVerificationRecord(ctx context.Context, id uuid.UUID) (*models.VerificationRecord, error) {
	var record models.VerificationRecord

	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("verification record not found")
		}
		return nil, fmt.Errorf("failed to get verification record: %w", err)
	}

	return &record, nil
}

// GetVerificationByTypeAndValue retrieves verification record by type and target value
func (r *VerificationRepository) GetVerificationByTypeAndValue(ctx context.Context, sessionID uuid.UUID, verificationType, targetValue string) (*models.VerificationRecord, error) {
	var record models.VerificationRecord

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND verification_type = ? AND target_value = ?",
		sessionID, verificationType, targetValue).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("verification record not found")
		}
		return nil, fmt.Errorf("failed to get verification record: %w", err)
	}

	return &record, nil
}

// GetActiveVerificationByCode retrieves an active verification record by code
func (r *VerificationRepository) GetActiveVerificationByCode(ctx context.Context, sessionID uuid.UUID, code string) (*models.VerificationRecord, error) {
	var record models.VerificationRecord

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND verification_code = ? AND status = ? AND expires_at > ?",
		sessionID, code, "pending", time.Now()).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("active verification record not found")
		}
		return nil, fmt.Errorf("failed to get verification record: %w", err)
	}

	return &record, nil
}

// UpdateVerificationRecord updates a verification record
func (r *VerificationRepository) UpdateVerificationRecord(ctx context.Context, record *models.VerificationRecord) (*models.VerificationRecord, error) {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, fmt.Errorf("failed to update verification record: %w", err)
	}

	return record, nil
}

// IncrementAttempts increments the attempt count for a verification record
func (r *VerificationRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&models.VerificationRecord{}).
		Where("id = ?", id).
		Update("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
		return fmt.Errorf("failed to increment verification attempts: %w", err)
	}

	return nil
}

// MarkAsVerified marks a verification record as verified
func (r *VerificationRepository) MarkAsVerified(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&models.VerificationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "verified",
			"verified_at": &now,
		}).Error; err != nil {
		return fmt.Errorf("failed to mark verification as verified: %w", err)
	}

	return nil
}

// MarkAsFailed marks a verification record as failed
func (r *VerificationRepository) MarkAsFailed(ctx context.Context, id uuid.UUID, reason string) error {
	if err := r.db.WithContext(ctx).Model(&models.VerificationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":   "failed",
			"metadata": gorm.Expr("jsonb_set(COALESCE(metadata, '{}'), '{failure_reason}', ?)", fmt.Sprintf(`"%s"`, reason)),
		}).Error; err != nil {
		return fmt.Errorf("failed to mark verification as failed: %w", err)
	}

	return nil
}

// GetExpiredVerifications retrieves expired verification records
func (r *VerificationRepository) GetExpiredVerifications(ctx context.Context) ([]models.VerificationRecord, error) {
	var records []models.VerificationRecord

	if err := r.db.WithContext(ctx).Where("status = ? AND expires_at < ?", "pending", time.Now()).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get expired verifications: %w", err)
	}

	return records, nil
}

// CleanupExpiredVerifications marks expired verifications as expired
func (r *VerificationRepository) CleanupExpiredVerifications(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Model(&models.VerificationRecord{}).
		Where("status = ? AND expires_at < ?", "pending", time.Now()).
		Update("status", "expired").Error; err != nil {
		return fmt.Errorf("failed to cleanup expired verifications: %w", err)
	}

	return nil
}

// GetVerificationsBySession retrieves all verification records for a session
func (r *VerificationRepository) GetVerificationsBySession(ctx context.Context, sessionID uuid.UUID) ([]models.VerificationRecord, error) {
	var records []models.VerificationRecord

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get verifications by session: %w", err)
	}

	return records, nil
}

// GetVerificationsBySessionAndType retrieves verification records by session and type
func (r *VerificationRepository) GetVerificationsBySessionAndType(ctx context.Context, sessionID uuid.UUID, verificationType string) ([]models.VerificationRecord, error) {
	var records []models.VerificationRecord

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND verification_type = ?",
		sessionID, verificationType).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get verifications by session and type: %w", err)
	}

	return records, nil
}

// IsVerified checks if a specific verification type is verified for a session
func (r *VerificationRepository) IsVerified(ctx context.Context, sessionID uuid.UUID, verificationType string) (bool, error) {
	var count int64

	if err := r.db.WithContext(ctx).Model(&models.VerificationRecord{}).
		Where("onboarding_session_id = ? AND verification_type = ? AND status = ?",
			sessionID, verificationType, "verified").Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check verification status: %w", err)
	}

	return count > 0, nil
}
