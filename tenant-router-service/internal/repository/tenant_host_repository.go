package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tenant-router-service/internal/models"
)

// TenantHostRepository defines the interface for tenant host data access
type TenantHostRepository interface {
	// CRUD operations
	Create(ctx context.Context, record *models.TenantHostRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.TenantHostRecord, error)
	GetBySlug(ctx context.Context, slug string) (*models.TenantHostRecord, error)
	GetByTenantID(ctx context.Context, tenantID string) (*models.TenantHostRecord, error)
	Update(ctx context.Context, record *models.TenantHostRecord) error
	Delete(ctx context.Context, slug string) error
	HardDelete(ctx context.Context, slug string) error

	// Query operations
	List(ctx context.Context, status *models.HostStatus, limit, offset int) ([]models.TenantHostRecord, int64, error)
	ListByStatus(ctx context.Context, status models.HostStatus) ([]models.TenantHostRecord, error)
	ListPendingRetry(ctx context.Context, maxRetries int) ([]models.TenantHostRecord, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	ExistsByHost(ctx context.Context, host string) (bool, error)

	// Status updates
	UpdateStatus(ctx context.Context, slug string, status models.HostStatus) error
	UpdateProvisioningState(ctx context.Context, slug string, field string, value bool, namespace string) error
	MarkProvisioned(ctx context.Context, slug string) error
	MarkFailed(ctx context.Context, slug string, errorMsg string) error

	// Activity logging
	LogActivity(ctx context.Context, log *models.ProvisioningActivityLog) error
	GetActivityLogs(ctx context.Context, tenantHostID uuid.UUID, limit int) ([]models.ProvisioningActivityLog, error)

	// Statistics
	GetStats(ctx context.Context) (*HostStats, error)

	// Startup reconciliation - find records that need to be reconciled
	ListIncomplete(ctx context.Context) ([]models.TenantHostRecord, error)
	ListDeleting(ctx context.Context) ([]models.TenantHostRecord, error)

	// Cleanup operations
	CleanupOldDeletedRecords(ctx context.Context, olderThan time.Duration) (int64, error)
	GetDeletedSlugInfo(ctx context.Context, slug string) (*DeletedSlugInfo, error)
}

// DeletedSlugInfo contains information about a recently deleted slug
type DeletedSlugInfo struct {
	Slug           string    `json:"slug"`
	DeletedAt      time.Time `json:"deleted_at"`
	AvailableAfter time.Time `json:"available_after"`
	DaysRemaining  int       `json:"days_remaining"`
}

// HostStats contains statistics about tenant hosts
type HostStats struct {
	Total       int64 `json:"total"`
	Pending     int64 `json:"pending"`
	Provisioned int64 `json:"provisioned"`
	Failed      int64 `json:"failed"`
	Deleting    int64 `json:"deleting"`
}

// tenantHostRepository implements TenantHostRepository
type tenantHostRepository struct {
	db *gorm.DB
}

// NewTenantHostRepository creates a new TenantHostRepository
func NewTenantHostRepository(db *gorm.DB) TenantHostRepository {
	return &tenantHostRepository{db: db}
}

// Create creates a new tenant host record
func (r *tenantHostRepository) Create(ctx context.Context, record *models.TenantHostRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// GetByID retrieves a tenant host record by ID
func (r *tenantHostRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TenantHostRecord, error) {
	var record models.TenantHostRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// GetBySlug retrieves a tenant host record by slug
func (r *tenantHostRepository) GetBySlug(ctx context.Context, slug string) (*models.TenantHostRecord, error) {
	var record models.TenantHostRecord
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// GetByTenantID retrieves a tenant host record by tenant ID
func (r *tenantHostRepository) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantHostRecord, error) {
	var record models.TenantHostRecord
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// Update updates a tenant host record
func (r *tenantHostRepository) Update(ctx context.Context, record *models.TenantHostRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// Delete soft deletes a tenant host record
func (r *tenantHostRepository) Delete(ctx context.Context, slug string) error {
	return r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Updates(map[string]interface{}{
			"status":     models.HostStatusDeleted,
			"deleted_at": time.Now(),
		}).Error
}

// HardDelete permanently deletes a tenant host record
func (r *tenantHostRepository) HardDelete(ctx context.Context, slug string) error {
	return r.db.WithContext(ctx).Unscoped().Where("slug = ?", slug).Delete(&models.TenantHostRecord{}).Error
}

// List retrieves tenant host records with optional status filter and pagination
func (r *tenantHostRepository) List(ctx context.Context, status *models.HostStatus, limit, offset int) ([]models.TenantHostRecord, int64, error) {
	var records []models.TenantHostRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&models.TenantHostRecord{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// ListByStatus retrieves all tenant host records with a specific status
func (r *tenantHostRepository) ListByStatus(ctx context.Context, status models.HostStatus) ([]models.TenantHostRecord, error) {
	var records []models.TenantHostRecord
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at ASC").
		Find(&records).Error
	return records, err
}

// ListPendingRetry retrieves failed records that can be retried
func (r *tenantHostRepository) ListPendingRetry(ctx context.Context, maxRetries int) ([]models.TenantHostRecord, error) {
	var records []models.TenantHostRecord
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", models.HostStatusFailed, maxRetries).
		Order("last_retry_at ASC NULLS FIRST").
		Find(&records).Error
	return records, err
}

// ExistsBySlug checks if a record exists with the given slug
func (r *tenantHostRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Count(&count).Error
	return count > 0, err
}

// ExistsByHost checks if a record exists with the given admin or storefront host
func (r *tenantHostRepository) ExistsByHost(ctx context.Context, host string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("admin_host = ? OR storefront_host = ?", host, host).
		Count(&count).Error
	return count > 0, err
}

// UpdateStatus updates only the status of a tenant host record
func (r *tenantHostRepository) UpdateStatus(ctx context.Context, slug string, status models.HostStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Update("status", status).Error
}

// UpdateProvisioningState updates a specific provisioning state field
func (r *tenantHostRepository) UpdateProvisioningState(ctx context.Context, slug string, field string, value bool, namespace string) error {
	updates := map[string]interface{}{
		field: value,
	}

	// Add namespace field if applicable
	switch field {
	case "certificate_created":
		updates["certificate_namespace"] = namespace
	case "gateway_patched":
		updates["gateway_namespace"] = namespace
	case "admin_vs_patched":
		updates["admin_vs_namespace"] = namespace
	case "storefront_vs_patched":
		updates["storefront_vs_namespace"] = namespace
	}

	return r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Updates(updates).Error
}

// MarkProvisioned marks a tenant host as fully provisioned
func (r *tenantHostRepository) MarkProvisioned(ctx context.Context, slug string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Updates(map[string]interface{}{
			"status":         models.HostStatusProvisioned,
			"provisioned_at": now,
			"last_error":     "",
		}).Error
}

// MarkFailed marks a tenant host as failed with error message
func (r *tenantHostRepository) MarkFailed(ctx context.Context, slug string, errorMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Where("slug = ?", slug).
		Updates(map[string]interface{}{
			"status":        models.HostStatusFailed,
			"last_error":    errorMsg,
			"last_retry_at": now,
		}).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// LogActivity logs a provisioning activity
func (r *tenantHostRepository) LogActivity(ctx context.Context, log *models.ProvisioningActivityLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetActivityLogs retrieves activity logs for a tenant host
func (r *tenantHostRepository) GetActivityLogs(ctx context.Context, tenantHostID uuid.UUID, limit int) ([]models.ProvisioningActivityLog, error) {
	var logs []models.ProvisioningActivityLog
	query := r.db.WithContext(ctx).
		Where("tenant_host_id = ?", tenantHostID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// ListIncomplete retrieves records that need reconciliation
// This includes: pending records, or records where not all resources are provisioned
func (r *tenantHostRepository) ListIncomplete(ctx context.Context) ([]models.TenantHostRecord, error) {
	var records []models.TenantHostRecord
	err := r.db.WithContext(ctx).
		Where("status = ? OR (status = ? AND (certificate_created = false OR gateway_patched = false OR admin_vs_patched = false OR storefront_vs_patched = false OR api_vs_patched = false))",
			models.HostStatusPending, models.HostStatusProvisioned).
		Order("created_at ASC").
		Find(&records).Error
	return records, err
}

// ListDeleting retrieves records that are marked for deletion
func (r *tenantHostRepository) ListDeleting(ctx context.Context) ([]models.TenantHostRecord, error) {
	var records []models.TenantHostRecord
	err := r.db.WithContext(ctx).
		Where("status = ?", models.HostStatusDeleting).
		Order("created_at ASC").
		Find(&records).Error
	return records, err
}

// GetStats retrieves statistics about tenant hosts
func (r *tenantHostRepository) GetStats(ctx context.Context) (*HostStats, error) {
	stats := &HostStats{}

	// Get counts for each status
	rows, err := r.db.WithContext(ctx).
		Model(&models.TenantHostRecord{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get host stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}

		switch models.HostStatus(status) {
		case models.HostStatusPending:
			stats.Pending = count
		case models.HostStatusProvisioned:
			stats.Provisioned = count
		case models.HostStatusFailed:
			stats.Failed = count
		case models.HostStatusDeleting:
			stats.Deleting = count
		}
		stats.Total += count
	}

	return stats, nil
}

// CleanupOldDeletedRecords permanently deletes records that were soft-deleted
// more than the specified duration ago (e.g., 15 days)
func (r *tenantHostRepository) CleanupOldDeletedRecords(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result := r.db.WithContext(ctx).
		Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Delete(&models.TenantHostRecord{})
	return result.RowsAffected, result.Error
}

// GetDeletedSlugInfo returns information about a recently deleted slug
// Returns nil if the slug was never deleted or is already past the retention period
func (r *tenantHostRepository) GetDeletedSlugInfo(ctx context.Context, slug string) (*DeletedSlugInfo, error) {
	var record models.TenantHostRecord
	// Use Unscoped to include soft-deleted records
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("slug = ? AND deleted_at IS NOT NULL", slug).
		First(&record).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found or not deleted
		}
		return nil, err
	}

	// Calculate when it will be available (15 days after deletion)
	retentionPeriod := 15 * 24 * time.Hour
	availableAfter := record.DeletedAt.Time.Add(retentionPeriod)
	daysRemaining := int(time.Until(availableAfter).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return &DeletedSlugInfo{
		Slug:           record.Slug,
		DeletedAt:      record.DeletedAt.Time,
		AvailableAfter: availableAfter,
		DaysRemaining:  daysRemaining,
	}, nil
}
