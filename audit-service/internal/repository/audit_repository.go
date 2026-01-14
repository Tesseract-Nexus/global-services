package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tesseract-hub/audit-service/internal/models"
)

// AuditRepository handles database operations for audit logs
type AuditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{
		db: db,
	}
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateBatch creates multiple audit log entries in a single transaction
func (r *AuditRepository) CreateBatch(ctx context.Context, logs []*models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 100).Error
}

// GetByID retrieves an audit log by ID
func (r *AuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	var log models.AuditLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// List retrieves audit logs with filtering and pagination
func (r *AuditRepository) List(ctx context.Context, filter *models.AuditLogFilter) ([]*models.AuditLog, int64, error) {
	var logs []*models.AuditLog
	var total int64

	// Build query
	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	sortBy := "timestamp"
	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}
	sortOrder := "DESC"
	if filter.SortOrder != "" {
		sortOrder = strings.ToUpper(filter.SortOrder)
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	limit := 50
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query = query.Limit(limit).Offset(offset)

	// Execute query
	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// applyFilters applies filter criteria to the query
func (r *AuditRepository) applyFilters(query *gorm.DB, filter *models.AuditLogFilter) *gorm.DB {
	if filter == nil {
		return query
	}

	// Tenant filter
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}

	// User filter
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	// Action filter
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}

	// Resource filter
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}

	// Resource ID filter
	if filter.ResourceID != "" {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}

	// Status filter
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	// Severity filter
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}

	// IP address filter
	if filter.IPAddress != "" {
		query = query.Where("ip_address = ?", filter.IPAddress)
	}

	// Service name filter
	if filter.ServiceName != "" {
		query = query.Where("service_name = ?", filter.ServiceName)
	}

	// Date range filter
	if filter.FromDate != nil {
		query = query.Where("timestamp >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("timestamp <= ?", *filter.ToDate)
	}

	// Search text filter (searches in description and resource_name)
	if filter.SearchText != "" {
		searchPattern := "%" + filter.SearchText + "%"
		query = query.Where(
			"description ILIKE ? OR resource_name ILIKE ? OR username ILIKE ? OR user_email ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}

	return query
}

// GetByResourceID retrieves all audit logs for a specific resource
func (r *AuditRepository) GetByResourceID(ctx context.Context, tenantID, resourceType, resourceID string, limit int) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND resource = ? AND resource_id = ?", tenantID, resourceType, resourceID).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// GetByUser retrieves all audit logs for a specific user
func (r *AuditRepository) GetByUser(ctx context.Context, tenantID string, userID uuid.UUID, limit int) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// GetRecentCriticalEvents retrieves recent critical severity events
func (r *AuditRepository) GetRecentCriticalEvents(ctx context.Context, tenantID string, hours int) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog

	cutoffTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND severity IN (?, ?) AND timestamp >= ?", tenantID, models.SeverityHigh, models.SeverityCritical, cutoffTime).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	return logs, nil
}

// GetFailedAuthAttempts retrieves failed authentication attempts
func (r *AuditRepository) GetFailedAuthAttempts(ctx context.Context, tenantID string, hours int) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog

	cutoffTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND action = ? AND status = ? AND timestamp >= ?",
			tenantID, models.ActionLoginFailed, models.StatusFailure, cutoffTime).
		Order("timestamp DESC").
		Limit(100).
		Find(&logs).Error

	if err != nil {
		return nil, err
	}

	return logs, nil
}

// GetSummary generates aggregated statistics for audit logs
func (r *AuditRepository) GetSummary(ctx context.Context, tenantID string, fromDate, toDate time.Time) (*models.AuditLogSummary, error) {
	summary := &models.AuditLogSummary{
		ByAction:   make(map[string]int64),
		ByResource: make(map[string]int64),
		ByStatus:   make(map[string]int64),
		BySeverity: make(map[string]int64),
		TimeRange: models.TimeRange{
			From: fromDate,
			To:   toDate,
		},
	}

	// Total logs
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Count(&total).Error; err != nil {
		return nil, err
	}
	summary.TotalLogs = total

	// By action
	var actionCounts []struct {
		Action string
		Count  int64
	}
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("action, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Group("action").
		Find(&actionCounts).Error; err != nil {
		return nil, err
	}
	for _, ac := range actionCounts {
		summary.ByAction[ac.Action] = ac.Count
	}

	// By resource
	var resourceCounts []struct {
		Resource string
		Count    int64
	}
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("resource, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Group("resource").
		Find(&resourceCounts).Error; err != nil {
		return nil, err
	}
	for _, rc := range resourceCounts {
		summary.ByResource[rc.Resource] = rc.Count
	}

	// By status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Group("status").
		Find(&statusCounts).Error; err != nil {
		return nil, err
	}
	for _, sc := range statusCounts {
		summary.ByStatus[sc.Status] = sc.Count
	}

	// By severity
	var severityCounts []struct {
		Severity string
		Count    int64
	}
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("severity, COUNT(*) as count").
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Group("severity").
		Find(&severityCounts).Error; err != nil {
		return nil, err
	}
	for _, sc := range severityCounts {
		summary.BySeverity[sc.Severity] = sc.Count
	}

	// Top users
	var topUsers []models.UserActivity
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("user_id, username, user_email, COUNT(*) as count, MAX(timestamp) as last_activity").
		Where("tenant_id = ? AND timestamp BETWEEN ? AND ? AND user_id IS NOT NULL", tenantID, fromDate, toDate).
		Group("user_id, username, user_email").
		Order("count DESC").
		Limit(10).
		Find(&topUsers).Error; err != nil {
		return nil, err
	}
	summary.TopUsers = topUsers

	// Recent failures
	var recentFailures []models.AuditLog
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND timestamp BETWEEN ? AND ?",
			tenantID, models.StatusFailure, fromDate, toDate).
		Order("timestamp DESC").
		Limit(20).
		Find(&recentFailures).Error; err != nil {
		return nil, err
	}
	summary.RecentFailures = recentFailures

	return summary, nil
}

// DeleteOldLogs deletes audit logs older than the retention period
func (r *AuditRepository) DeleteOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	result := r.db.WithContext(ctx).
		Where("timestamp < ?", cutoffDate).
		Delete(&models.AuditLog{})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// CountByIPAddress counts the number of actions from a specific IP address in a time window
func (r *AuditRepository) CountByIPAddress(ctx context.Context, ipAddress string, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("ip_address = ? AND timestamp >= ?", ipAddress, since).
		Count(&count).Error

	return count, err
}

// GetIPAddressHistory retrieves all unique IP addresses used by a user
func (r *AuditRepository) GetIPAddressHistory(ctx context.Context, tenantID string, userID uuid.UUID) ([]string, error) {
	var ipAddresses []string

	err := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("DISTINCT ip_address").
		Where("tenant_id = ? AND user_id = ? AND ip_address IS NOT NULL", tenantID, userID).
		Order("ip_address").
		Pluck("ip_address", &ipAddresses).Error

	return ipAddresses, err
}

// Export exports audit logs to a specified format (for bulk export)
func (r *AuditRepository) Export(ctx context.Context, filter *models.AuditLogFilter) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog

	// Build query
	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	// Apply filters (but no pagination for export)
	query = r.applyFilters(query, filter)

	// Apply sorting
	sortBy := "timestamp"
	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}
	query = query.Order(fmt.Sprintf("%s DESC", sortBy))

	// Execute query (limit to reasonable size for export)
	if err := query.Limit(10000).Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}
