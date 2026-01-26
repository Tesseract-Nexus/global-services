package repository

import (
	"context"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditRepository provides database operations for audit logs
type AuditRepository interface {
	Create(ctx context.Context, log *models.SecretAuditLog) error
	GetBySecretName(ctx context.Context, secretName string, limit int) ([]*models.SecretAuditLog, error)
	GetByTenant(ctx context.Context, tenantID string, since time.Time, limit int) ([]*models.SecretAuditLog, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SecretAuditLog, error)
}

// auditRepository implements AuditRepository
type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new audit repository instance
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

// Create creates a new audit log entry
func (r *auditRepository) Create(ctx context.Context, log *models.SecretAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetBySecretName retrieves audit logs for a specific secret
func (r *auditRepository) GetBySecretName(ctx context.Context, secretName string, limit int) ([]*models.SecretAuditLog, error) {
	var logs []*models.SecretAuditLog
	query := r.db.WithContext(ctx).
		Where("secret_name = ?", secretName).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByTenant retrieves audit logs for a tenant since a given time
func (r *auditRepository) GetByTenant(ctx context.Context, tenantID string, since time.Time, limit int) ([]*models.SecretAuditLog, error) {
	var logs []*models.SecretAuditLog
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND created_at >= ?", tenantID, since).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByID retrieves a specific audit log entry
func (r *auditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SecretAuditLog, error) {
	var log models.SecretAuditLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}
