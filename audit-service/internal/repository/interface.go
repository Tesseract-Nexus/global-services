package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tesseract-hub/audit-service/internal/models"
)

// AuditRepositoryInterface defines the contract for audit log operations
// This allows for different implementations (single-tenant, multi-tenant, mock, etc.)
type AuditRepositoryInterface interface {
	// Create creates a new audit log entry
	Create(ctx context.Context, tenantID string, log *models.AuditLog) error

	// GetByID retrieves an audit log by ID
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (*models.AuditLog, error)

	// List retrieves audit logs with filtering and pagination
	List(ctx context.Context, tenantID string, params ListParams) ([]models.AuditLog, int64, error)

	// GetSummary retrieves summary statistics
	GetSummary(ctx context.Context, tenantID string, fromDate, toDate time.Time) (*models.AuditSummary, error)

	// GetCriticalEvents retrieves critical events from the last N hours
	GetCriticalEvents(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error)

	// GetUserActivity retrieves activity for a specific user
	GetUserActivity(ctx context.Context, tenantID, userID string, limit int) ([]models.AuditLog, error)

	// GetResourceHistory retrieves audit history for a specific resource
	GetResourceHistory(ctx context.Context, tenantID, resourceType, resourceID string) ([]models.AuditLog, error)

	// GetFailedAuthAttempts retrieves failed authentication attempts
	GetFailedAuthAttempts(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error)

	// GetSuspiciousActivity retrieves potentially suspicious activities
	GetSuspiciousActivity(ctx context.Context, tenantID string) ([]models.AuditLog, error)

	// GetUserIPHistory retrieves IP addresses used by a user
	GetUserIPHistory(ctx context.Context, tenantID, userID string) ([]models.IPHistoryEntry, error)

	// Export retrieves all audit logs for export
	Export(ctx context.Context, tenantID string, params ListParams) ([]models.AuditLog, error)

	// GetRecentLogs retrieves recent logs for real-time updates
	GetRecentLogs(ctx context.Context, tenantID string, limit int) ([]models.AuditLog, error)

	// CleanupOldLogs deletes logs older than the retention period
	CleanupOldLogs(ctx context.Context, tenantID string, retentionDays int) (int64, error)

	// GetRetentionSettings retrieves retention settings for a tenant
	GetRetentionSettings(ctx context.Context, tenantID string) (*models.RetentionSettings, error)

	// SetRetentionSettings saves retention settings for a tenant
	SetRetentionSettings(ctx context.Context, tenantID string, settings *models.RetentionSettings) error
}

// Ensure MultiTenantRepository implements the interface
var _ AuditRepositoryInterface = (*MultiTenantRepository)(nil)
