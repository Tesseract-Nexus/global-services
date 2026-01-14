package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"audit-service/internal/cache"
	"audit-service/internal/database"
	"audit-service/internal/models"
)

// MultiTenantRepository handles audit log operations across multiple tenant databases
type MultiTenantRepository struct {
	dbManager *database.Manager
	cache     *cache.AuditCache
	logger    *logrus.Logger
}

// NewMultiTenantRepository creates a new multi-tenant audit repository
func NewMultiTenantRepository(dbManager *database.Manager, auditCache *cache.AuditCache, logger *logrus.Logger) *MultiTenantRepository {
	return &MultiTenantRepository{
		dbManager: dbManager,
		cache:     auditCache,
		logger:    logger,
	}
}

// Create creates a new audit log in the tenant's database
func (r *MultiTenantRepository) Create(ctx context.Context, tenantID string, log *models.AuditLog) error {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	// Set tenant ID on the log
	log.TenantID = tenantID

	if err := db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Cache the new log
	if r.cache != nil {
		r.cache.SetAuditLog(ctx, tenantID, log)
		r.cache.InvalidateAfterWrite(ctx, tenantID)
		r.cache.PushRecentLog(ctx, tenantID, log)
	}

	return nil
}

// GetByID retrieves an audit log by ID from the tenant's database
func (r *MultiTenantRepository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (*models.AuditLog, error) {
	// Check cache first
	if r.cache != nil {
		if log, err := r.cache.GetAuditLog(ctx, tenantID, id.String()); err == nil {
			return log, nil
		}
	}

	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	var log models.AuditLog
	if err := db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	// Cache the result
	if r.cache != nil {
		r.cache.SetAuditLog(ctx, tenantID, &log)
	}

	return &log, nil
}

// ListParams defines parameters for listing audit logs
type ListParams struct {
	Action     string
	Resource   string
	Status     string
	Severity   string
	UserID     string
	Search     string
	FromDate   time.Time
	ToDate     time.Time
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

// List retrieves audit logs with filtering and pagination
func (r *MultiTenantRepository) List(ctx context.Context, tenantID string, params ListParams) ([]models.AuditLog, int64, error) {
	// Build filter map for cache key
	filters := map[string]string{}
	if params.Action != "" {
		filters["action"] = params.Action
	}
	if params.Resource != "" {
		filters["resource"] = params.Resource
	}
	if params.Status != "" {
		filters["status"] = params.Status
	}
	if params.Severity != "" {
		filters["severity"] = params.Severity
	}
	if params.UserID != "" {
		filters["user_id"] = params.UserID
	}
	if params.Search != "" {
		filters["search"] = params.Search
	}

	// Check cache
	if r.cache != nil {
		page := params.Offset / params.Limit
		if logs, total, err := r.cache.GetAuditList(ctx, tenantID, filters, page, params.Limit); err == nil {
			return logs, total, nil
		}
	}

	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	query := db.WithContext(ctx).Model(&models.AuditLog{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.Resource != "" {
		query = query.Where("resource = ?", params.Resource)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Severity != "" {
		query = query.Where("severity = ?", params.Severity)
	}
	if params.UserID != "" {
		query = query.Where("user_id = ?", params.UserID)
	}
	if params.Search != "" {
		searchPattern := "%" + params.Search + "%"
		query = query.Where(
			"description ILIKE ? OR resource_name ILIKE ? OR username ILIKE ?",
			searchPattern, searchPattern, searchPattern,
		)
	}
	if !params.FromDate.IsZero() {
		query = query.Where("timestamp >= ?", params.FromDate)
	}
	if !params.ToDate.IsZero() {
		query = query.Where("timestamp <= ?", params.ToDate)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply sorting
	sortBy := params.SortBy
	if sortBy == "" {
		sortBy = "timestamp"
	}
	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	var logs []models.AuditLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}

	// Cache the result
	if r.cache != nil && len(logs) > 0 {
		page := params.Offset / params.Limit
		r.cache.SetAuditList(ctx, tenantID, filters, page, params.Limit, logs, total)
	}

	return logs, total, nil
}

// GetSummary retrieves summary statistics for a tenant
func (r *MultiTenantRepository) GetSummary(ctx context.Context, tenantID string, fromDate, toDate time.Time) (*models.AuditSummary, error) {
	fromStr := fromDate.Format(time.RFC3339)
	toStr := toDate.Format(time.RFC3339)

	// Check cache
	if r.cache != nil {
		if cached, err := r.cache.GetSummary(ctx, tenantID, fromStr, toStr); err == nil {
			return mapToSummary(cached), nil
		}
	}

	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	summary := &models.AuditSummary{
		TimeRange: models.TimeRange{
			From: fromDate,
			To:   toDate,
		},
		ByAction:   make(map[string]int64),
		ByResource: make(map[string]int64),
		ByStatus:   make(map[string]int64),
		BySeverity: make(map[string]int64),
	}

	baseQuery := db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate)

	// Total count
	baseQuery.Count(&summary.TotalLogs)

	// Count by action
	var actionCounts []struct {
		Action string
		Count  int64
	}
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("action, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate).
		Group("action").
		Find(&actionCounts)
	for _, ac := range actionCounts {
		summary.ByAction[ac.Action] = ac.Count
	}

	// Count by resource
	var resourceCounts []struct {
		Resource string
		Count    int64
	}
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("resource, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate).
		Group("resource").
		Find(&resourceCounts)
	for _, rc := range resourceCounts {
		summary.ByResource[rc.Resource] = rc.Count
	}

	// Count by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate).
		Group("status").
		Find(&statusCounts)
	for _, sc := range statusCounts {
		summary.ByStatus[sc.Status] = sc.Count
	}

	// Count by severity
	var severityCounts []struct {
		Severity string
		Count    int64
	}
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("severity, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate).
		Group("severity").
		Find(&severityCounts)
	for _, sc := range severityCounts {
		summary.BySeverity[sc.Severity] = sc.Count
	}

	// Top users
	var topUsers []models.UserActivity
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("user_id, username, COUNT(*) as activity_count, MAX(timestamp) as last_activity").
		Where("tenant_id = ? AND timestamp >= ? AND timestamp <= ?", tenantID, fromDate, toDate).
		Group("user_id, username").
		Order("activity_count DESC").
		Limit(10).
		Find(&topUsers)
	summary.TopUsers = topUsers

	// Recent failures
	var recentFailures []models.AuditLog
	db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("tenant_id = ? AND status = ? AND timestamp >= ? AND timestamp <= ?", tenantID, "FAILURE", fromDate, toDate).
		Order("timestamp DESC").
		Limit(10).
		Find(&recentFailures)
	summary.RecentFailures = recentFailures

	// Cache the result
	if r.cache != nil {
		r.cache.SetSummary(ctx, tenantID, fromStr, toStr, summaryToMap(summary))
	}

	return summary, nil
}

// GetCriticalEvents retrieves critical events from the last N hours
func (r *MultiTenantRepository) GetCriticalEvents(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error) {
	// Check cache
	if r.cache != nil {
		if logs, err := r.cache.GetCriticalEvents(ctx, tenantID, hours); err == nil {
			return logs, nil
		}
	}

	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND severity IN (?, ?) AND timestamp >= ?", tenantID, "CRITICAL", "HIGH", since).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get critical events: %w", err)
	}

	// Cache the result
	if r.cache != nil {
		r.cache.SetCriticalEvents(ctx, tenantID, hours, logs)
	}

	return logs, nil
}

// GetUserActivity retrieves activity for a specific user
func (r *MultiTenantRepository) GetUserActivity(ctx context.Context, tenantID, userID string, limit int) ([]models.AuditLog, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	if limit <= 0 {
		limit = 50
	}

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get user activity: %w", err)
	}

	return logs, nil
}

// GetResourceHistory retrieves audit history for a specific resource
func (r *MultiTenantRepository) GetResourceHistory(ctx context.Context, tenantID, resourceType, resourceID string) ([]models.AuditLog, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND resource = ? AND resource_id = ?", tenantID, resourceType, resourceID).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get resource history: %w", err)
	}

	return logs, nil
}

// GetFailedAuthAttempts retrieves failed authentication attempts
func (r *MultiTenantRepository) GetFailedAuthAttempts(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND action IN (?, ?, ?) AND status = ? AND timestamp >= ?",
			tenantID, "LOGIN", "AUTHENTICATE", "AUTH", "FAILURE", since).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get failed auth attempts: %w", err)
	}

	return logs, nil
}

// GetSuspiciousActivity retrieves potentially suspicious activities
func (r *MultiTenantRepository) GetSuspiciousActivity(ctx context.Context, tenantID string) ([]models.AuditLog, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	since := time.Now().Add(-24 * time.Hour)

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where(`tenant_id = ? AND timestamp >= ? AND (
			(action IN ('DELETE', 'BULK_DELETE', 'MASS_UPDATE') AND severity = 'HIGH') OR
			(status = 'FAILURE' AND severity IN ('HIGH', 'CRITICAL')) OR
			(action = 'EXPORT' AND resource IN ('CUSTOMERS', 'ORDERS', 'PAYMENTS'))
		)`, tenantID, since).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get suspicious activity: %w", err)
	}

	return logs, nil
}

// GetUserIPHistory retrieves IP addresses used by a user
func (r *MultiTenantRepository) GetUserIPHistory(ctx context.Context, tenantID, userID string) ([]models.IPHistoryEntry, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	var entries []models.IPHistoryEntry
	if err := db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("ip_address, COUNT(*) as count, MIN(timestamp) as first_seen, MAX(timestamp) as last_seen").
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Group("ip_address").
		Order("last_seen DESC").
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to get IP history: %w", err)
	}

	return entries, nil
}

// Export retrieves all audit logs for export (with streaming support for large datasets)
func (r *MultiTenantRepository) Export(ctx context.Context, tenantID string, params ListParams) ([]models.AuditLog, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	query := db.WithContext(ctx).Model(&models.AuditLog{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.Resource != "" {
		query = query.Where("resource = ?", params.Resource)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Severity != "" {
		query = query.Where("severity = ?", params.Severity)
	}
	if !params.FromDate.IsZero() {
		query = query.Where("timestamp >= ?", params.FromDate)
	}
	if !params.ToDate.IsZero() {
		query = query.Where("timestamp <= ?", params.ToDate)
	}

	query = query.Order("timestamp DESC")

	// Limit export size to prevent memory issues
	maxExport := 10000
	if params.Limit > 0 && params.Limit < maxExport {
		maxExport = params.Limit
	}
	query = query.Limit(maxExport)

	var logs []models.AuditLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to export audit logs: %w", err)
	}

	return logs, nil
}

// GetRecentLogs retrieves recent logs for real-time updates
func (r *MultiTenantRepository) GetRecentLogs(ctx context.Context, tenantID string, limit int) ([]models.AuditLog, error) {
	// Try cache first for real-time data
	if r.cache != nil {
		if logs, err := r.cache.GetRecentLogs(ctx, tenantID, limit); err == nil && len(logs) > 0 {
			return logs, nil
		}
	}

	// Fall back to database
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	if limit <= 0 {
		limit = 20
	}

	var logs []models.AuditLog
	if err := db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent logs: %w", err)
	}

	return logs, nil
}

// Helper functions
func mapToSummary(m map[string]interface{}) *models.AuditSummary {
	// Convert map back to summary struct
	summary := &models.AuditSummary{
		ByAction:   make(map[string]int64),
		ByResource: make(map[string]int64),
		ByStatus:   make(map[string]int64),
		BySeverity: make(map[string]int64),
	}

	if v, ok := m["total_logs"].(float64); ok {
		summary.TotalLogs = int64(v)
	}

	// Convert maps
	if v, ok := m["by_action"].(map[string]interface{}); ok {
		for k, val := range v {
			if f, ok := val.(float64); ok {
				summary.ByAction[k] = int64(f)
			}
		}
	}

	return summary
}

func summaryToMap(s *models.AuditSummary) map[string]interface{} {
	return map[string]interface{}{
		"total_logs":      s.TotalLogs,
		"by_action":       s.ByAction,
		"by_resource":     s.ByResource,
		"by_status":       s.ByStatus,
		"by_severity":     s.BySeverity,
		"top_users":       s.TopUsers,
		"recent_failures": s.RecentFailures,
		"time_range":      s.TimeRange,
	}
}

// CleanupOldLogs deletes audit logs older than the retention period for a tenant
func (r *MultiTenantRepository) CleanupOldLogs(ctx context.Context, tenantID string, retentionDays int) (int64, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	r.logger.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"retention_days": retentionDays,
		"cutoff_date":    cutoffDate,
	}).Info("Starting audit log cleanup")

	// Delete old logs in batches to avoid locking issues
	var totalDeleted int64
	batchSize := 1000

	for {
		result := db.WithContext(ctx).
			Where("tenant_id = ? AND timestamp < ?", tenantID, cutoffDate).
			Limit(batchSize).
			Delete(&models.AuditLog{})

		if result.Error != nil {
			return totalDeleted, fmt.Errorf("failed to delete old logs: %w", result.Error)
		}

		totalDeleted += result.RowsAffected

		// If we deleted less than batch size, we're done
		if result.RowsAffected < int64(batchSize) {
			break
		}
	}

	// Invalidate cache after cleanup
	if r.cache != nil {
		r.cache.InvalidateTenant(ctx, tenantID)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id":     tenantID,
		"logs_deleted":  totalDeleted,
		"cutoff_date":   cutoffDate,
	}).Info("Completed audit log cleanup")

	return totalDeleted, nil
}

// GetRetentionSettings retrieves retention settings for a tenant
func (r *MultiTenantRepository) GetRetentionSettings(ctx context.Context, tenantID string) (*models.RetentionSettings, error) {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	// Auto-migrate the retention settings table
	if err := db.AutoMigrate(&models.RetentionSettings{}); err != nil {
		r.logger.WithError(err).Warn("Failed to auto-migrate retention settings table")
	}

	var settings models.RetentionSettings
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&settings)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Return default settings if not found
			return &models.RetentionSettings{
				TenantID:      tenantID,
				RetentionDays: 180, // Default 6 months
			}, nil
		}
		return nil, fmt.Errorf("failed to get retention settings: %w", result.Error)
	}

	return &settings, nil
}

// SetRetentionSettings saves retention settings for a tenant
func (r *MultiTenantRepository) SetRetentionSettings(ctx context.Context, tenantID string, settings *models.RetentionSettings) error {
	db, err := r.dbManager.GetDB(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get database for tenant %s: %w", tenantID, err)
	}

	// Auto-migrate the retention settings table
	if err := db.AutoMigrate(&models.RetentionSettings{}); err != nil {
		r.logger.WithError(err).Warn("Failed to auto-migrate retention settings table")
	}

	// Validate retention days (90-365)
	if settings.RetentionDays < 90 {
		settings.RetentionDays = 90
	}
	if settings.RetentionDays > 365 {
		settings.RetentionDays = 365
	}

	settings.TenantID = tenantID
	settings.UpdatedAt = time.Now()

	// Upsert the settings
	result := db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Assign(models.RetentionSettings{
			RetentionDays: settings.RetentionDays,
			UpdatedAt:     settings.UpdatedAt,
		}).
		FirstOrCreate(settings)

	if result.Error != nil {
		return fmt.Errorf("failed to save retention settings: %w", result.Error)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"retention_days": settings.RetentionDays,
	}).Info("Retention settings updated")

	return nil
}
