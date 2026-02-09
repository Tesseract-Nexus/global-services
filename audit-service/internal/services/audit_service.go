package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"audit-service/internal/models"
	auditNats "audit-service/internal/nats"
	"audit-service/internal/repository"
)

// AuditService handles business logic for audit logging
type AuditService struct {
	repo      repository.AuditRepositoryInterface
	logger    *logrus.Logger
	publisher *auditNats.Publisher
}

// NewAuditService creates a new audit service
func NewAuditService(repo repository.AuditRepositoryInterface, logger *logrus.Logger, publisher *auditNats.Publisher) *AuditService {
	return &AuditService{
		repo:      repo,
		logger:    logger,
		publisher: publisher,
	}
}

// LogAction logs an action to the audit trail
func (s *AuditService) LogAction(ctx context.Context, tenantID string, log *models.AuditLog) error {
	// Set timestamp if not already set
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// Ensure tenant ID is set
	log.TenantID = tenantID

	// Create audit log
	if err := s.repo.Create(ctx, tenantID, log); err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to create audit log")
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Publish event to NATS for real-time streaming
	if s.publisher != nil {
		go func() {
			if err := s.publisher.PublishAuditLog(context.Background(), "created", tenantID, log); err != nil {
				s.logger.WithError(err).WithField("tenant_id", tenantID).Warn("Failed to publish audit event to NATS")
			}
		}()
	}

	// Log critical events to application logger
	if log.IsCritical() || log.ShouldAlert() {
		s.logger.WithFields(logrus.Fields{
			"audit_id":    log.ID,
			"tenant_id":   log.TenantID,
			"user_id":     log.UserID,
			"action":      log.Action,
			"resource":    log.Resource,
			"resource_id": log.ResourceID,
			"severity":    log.Severity,
			"status":      log.Status,
		}).Warn("Critical audit event")
	}

	return nil
}

// GetAuditLog retrieves a single audit log by ID
func (s *AuditService) GetAuditLog(ctx context.Context, tenantID string, id uuid.UUID) (*models.AuditLog, error) {
	log, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("Failed to get audit log")
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}
	return log, nil
}

// SearchAuditLogs searches audit logs with filters
func (s *AuditService) SearchAuditLogs(ctx context.Context, tenantID string, filter *models.AuditLogFilter) ([]models.AuditLog, int64, error) {
	params := repository.ListParams{
		Action:    string(filter.Action),
		Resource:  string(filter.Resource),
		Status:    string(filter.Status),
		Severity:  string(filter.Severity),
		Search:    filter.SearchText,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
	}

	// Convert UserID pointer to string
	if filter.UserID != nil {
		params.UserID = filter.UserID.String()
	}

	// Convert time pointers to time values
	if filter.FromDate != nil {
		params.FromDate = *filter.FromDate
	}
	if filter.ToDate != nil {
		params.ToDate = *filter.ToDate
	}

	logs, total, err := s.repo.List(ctx, tenantID, params)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to search audit logs")
		return nil, 0, fmt.Errorf("failed to search audit logs: %w", err)
	}

	return logs, total, nil
}

// GetResourceHistory retrieves the audit history for a specific resource
func (s *AuditService) GetResourceHistory(ctx context.Context, tenantID, resourceType, resourceID string) ([]models.AuditLog, error) {
	logs, err := s.repo.GetResourceHistory(ctx, tenantID, resourceType, resourceID)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"tenant_id":     tenantID,
		}).Error("Failed to get resource history")
		return nil, fmt.Errorf("failed to get resource history: %w", err)
	}

	return logs, nil
}

// GetUserActivity retrieves all activity for a specific user
func (s *AuditService) GetUserActivity(ctx context.Context, tenantID, userID string, limit int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetUserActivity(ctx, tenantID, userID, limit)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"user_id":   userID,
			"tenant_id": tenantID,
		}).Error("Failed to get user activity")
		return nil, fmt.Errorf("failed to get user activity: %w", err)
	}

	return logs, nil
}

// GetRecentCriticalEvents retrieves recent critical events
func (s *AuditService) GetRecentCriticalEvents(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetCriticalEvents(ctx, tenantID, hours)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get critical events")
		return nil, fmt.Errorf("failed to get critical events: %w", err)
	}

	return logs, nil
}

// GetFailedAuthAttempts retrieves failed authentication attempts
func (s *AuditService) GetFailedAuthAttempts(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetFailedAuthAttempts(ctx, tenantID, hours)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get failed auth attempts")
		return nil, fmt.Errorf("failed to get failed auth attempts: %w", err)
	}

	return logs, nil
}

// GetSummary generates audit log summary statistics
func (s *AuditService) GetSummary(ctx context.Context, tenantID string, fromDate, toDate time.Time) (*models.AuditSummary, error) {
	summary, err := s.repo.GetSummary(ctx, tenantID, fromDate, toDate)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get audit summary")
		return nil, fmt.Errorf("failed to get audit summary: %w", err)
	}

	return summary, nil
}

// GetSuspiciousActivity detects potentially suspicious activity patterns
func (s *AuditService) GetSuspiciousActivity(ctx context.Context, tenantID string) ([]models.AuditLog, error) {
	logs, err := s.repo.GetSuspiciousActivity(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get suspicious activity")
		return nil, fmt.Errorf("failed to get suspicious activity: %w", err)
	}

	return logs, nil
}

// GetUserIPHistory retrieves all IP addresses used by a user
func (s *AuditService) GetUserIPHistory(ctx context.Context, tenantID, userID string) ([]models.IPHistoryEntry, error) {
	entries, err := s.repo.GetUserIPHistory(ctx, tenantID, userID)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"user_id":   userID,
			"tenant_id": tenantID,
		}).Error("Failed to get user IP history")
		return nil, fmt.Errorf("failed to get user IP history: %w", err)
	}

	return entries, nil
}

// GetRecentLogs retrieves recent logs for real-time updates
func (s *AuditService) GetRecentLogs(ctx context.Context, tenantID string, limit int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetRecentLogs(ctx, tenantID, limit)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get recent logs")
		return nil, fmt.Errorf("failed to get recent logs: %w", err)
	}

	return logs, nil
}

// ExportAuditLogs exports audit logs for the given filter
func (s *AuditService) ExportAuditLogs(ctx context.Context, tenantID string, filter *models.AuditLogFilter) ([]models.AuditLog, error) {
	params := repository.ListParams{
		Action:   string(filter.Action),
		Resource: string(filter.Resource),
		Status:   string(filter.Status),
		Severity: string(filter.Severity),
		Limit:    10000, // Max export limit
	}

	// Convert time pointers to time values
	if filter.FromDate != nil {
		params.FromDate = *filter.FromDate
	}
	if filter.ToDate != nil {
		params.ToDate = *filter.ToDate
	}

	logs, err := s.repo.Export(ctx, tenantID, params)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to export audit logs")
		return nil, fmt.Errorf("failed to export audit logs: %w", err)
	}

	return logs, nil
}

// ExportToJSON exports audit logs to JSON format
func (s *AuditService) ExportToJSON(ctx context.Context, tenantID string, filter *models.AuditLogFilter) ([]byte, error) {
	logs, err := s.ExportAuditLogs(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"count":     len(logs),
		"format":    "JSON",
	}).Info("Exported audit logs")

	return data, nil
}

// ExportToCSV exports audit logs to CSV format
func (s *AuditService) ExportToCSV(ctx context.Context, tenantID string, filter *models.AuditLogFilter) ([]byte, error) {
	logs, err := s.ExportAuditLogs(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}

	// Create CSV data
	var csvData [][]string

	// Header row
	csvData = append(csvData, []string{
		"ID", "Timestamp", "Tenant ID", "User ID", "Username", "User Email",
		"Action", "Resource", "Resource ID", "Resource Name", "Status", "Severity",
		"Method", "Path", "IP Address", "Request ID", "Description",
		"Error Message", "Service Name",
	})

	// Data rows
	for _, log := range logs {
		csvData = append(csvData, []string{
			log.ID.String(),
			log.Timestamp.Format(time.RFC3339),
			log.TenantID,
			log.UserID.String(),
			log.Username,
			log.UserEmail,
			string(log.Action),
			string(log.Resource),
			log.ResourceID,
			log.ResourceName,
			string(log.Status),
			string(log.Severity),
			log.Method,
			log.Path,
			log.IPAddress,
			log.RequestID,
			log.Description,
			log.ErrorMessage,
			log.ServiceName,
		})
	}

	// Convert to CSV bytes
	var buf []byte
	writer := csv.NewWriter(&csvWriter{data: &buf})
	if err := writer.WriteAll(csvData); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"count":     len(logs),
		"format":    "CSV",
	}).Info("Exported audit logs")

	return buf, nil
}

// csvWriter is a helper to write CSV to byte slice
type csvWriter struct {
	data *[]byte
}

func (w *csvWriter) Write(p []byte) (n int, err error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

// GetRetentionSettings retrieves retention settings for a tenant
func (s *AuditService) GetRetentionSettings(ctx context.Context, tenantID string) (*models.RetentionSettings, error) {
	settings, err := s.repo.GetRetentionSettings(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get retention settings")
		return nil, fmt.Errorf("failed to get retention settings: %w", err)
	}
	return settings, nil
}

// SetRetentionSettings saves retention settings for a tenant
func (s *AuditService) SetRetentionSettings(ctx context.Context, tenantID string, retentionDays int) (*models.RetentionSettings, error) {
	// Validate retention days (minimum 2 years for GDPR/SOC 2 compliance)
	if retentionDays < 730 {
		retentionDays = 730
	}
	if retentionDays > 2555 {
		retentionDays = 2555 // Max 7 years
	}

	settings := &models.RetentionSettings{
		TenantID:      tenantID,
		RetentionDays: retentionDays,
	}

	if err := s.repo.SetRetentionSettings(ctx, tenantID, settings); err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to set retention settings")
		return nil, fmt.Errorf("failed to set retention settings: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"retention_days": retentionDays,
	}).Info("Retention settings updated")

	return settings, nil
}

// CleanupOldLogs deletes logs older than the tenant's retention period
func (s *AuditService) CleanupOldLogs(ctx context.Context, tenantID string) (int64, error) {
	// Get tenant's retention settings
	settings, err := s.repo.GetRetentionSettings(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get retention settings for cleanup")
		return 0, err
	}

	retentionDays := settings.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 180 // Default 6 months
	}

	// Perform cleanup
	deleted, err := s.repo.CleanupOldLogs(ctx, tenantID, retentionDays)
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to cleanup old logs")
		return deleted, err
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":      tenantID,
		"retention_days": retentionDays,
		"logs_deleted":   deleted,
	}).Info("Completed audit log cleanup")

	return deleted, nil
}

// GetRetentionOptions returns available retention period options
func (s *AuditService) GetRetentionOptions() []models.RetentionOption {
	return models.GetRetentionOptions()
}

// ============================================================================
// Field-level PII Access Logging
// ============================================================================

// LogPIIAccess logs which PII fields were accessed for a given customer/resource
func (s *AuditService) LogPIIAccess(ctx context.Context, tenantID string, userID uuid.UUID, username string, customerID string, fieldsAccessed []string, resource models.AuditResource, resourceID string) error {
	fieldsJSON, _ := json.Marshal(fieldsAccessed)

	log := &models.AuditLog{
		TenantID:          tenantID,
		UserID:            userID,
		Username:          username,
		Action:            models.ActionPIIAccess,
		Resource:          resource,
		ResourceID:        resourceID,
		Status:            models.StatusSuccess,
		Severity:          models.SeverityMedium,
		PIIFieldsAccessed: fieldsJSON,
		Description:       fmt.Sprintf("PII fields accessed on %s/%s: %v", resource, resourceID, fieldsAccessed),
		ServiceName:       "audit-service",
		Timestamp:         time.Now(),
	}

	return s.LogAction(ctx, tenantID, log)
}

// GetPIIAccessLogs retrieves all PII access events for a specific customer/resource
func (s *AuditService) GetPIIAccessLogs(ctx context.Context, tenantID string, customerID string, fromDate, toDate time.Time) ([]models.AuditLog, int64, error) {
	filter := &models.AuditLogFilter{
		TenantID:   tenantID,
		Action:     models.ActionPIIAccess,
		ResourceID: customerID,
		FromDate:   &fromDate,
		ToDate:     &toDate,
		Limit:      1000,
		SortBy:     "timestamp",
		SortOrder:  "DESC",
	}

	return s.SearchAuditLogs(ctx, tenantID, filter)
}

// ============================================================================
// Encryption/Decryption Audit Logging
// ============================================================================

// LogEncryptionOperation logs an encryption or decryption operation
func (s *AuditService) LogEncryptionOperation(ctx context.Context, tenantID string, userID uuid.UUID, operation string, entityType string, entityID string, success bool, errorMsg string) error {
	action := models.ActionEncrypt
	if operation == "decrypt" {
		action = models.ActionDecrypt
	}

	status := models.StatusSuccess
	severity := models.SeverityLow
	if !success {
		if operation == "decrypt" {
			action = models.ActionDecryptFail
		} else {
			action = models.ActionEncryptFail
		}
		status = models.StatusFailure
		severity = models.SeverityCritical
	}

	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		Resource:     models.ResourceEncryption,
		ResourceID:   entityID,
		ResourceName: entityType,
		Status:       status,
		Severity:     severity,
		Description:  fmt.Sprintf("Encryption operation '%s' on %s/%s", operation, entityType, entityID),
		ErrorMessage: errorMsg,
		ServiceName:  "audit-service",
		Timestamp:    time.Now(),
	}

	return s.LogAction(ctx, tenantID, log)
}

// ============================================================================
// Failed Authorization Logging
// ============================================================================

// LogFailedAuthorization logs a failed authorization attempt with full context
func (s *AuditService) LogFailedAuthorization(ctx context.Context, tenantID string, userID uuid.UUID, username string, resource models.AuditResource, resourceID string, requiredPermission string, userRoles []string, ipAddress string, path string) error {
	authzContext, _ := json.Marshal(map[string]interface{}{
		"required_permission": requiredPermission,
		"user_roles":          userRoles,
		"attempted_resource":  fmt.Sprintf("%s/%s", resource, resourceID),
	})

	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		Username:     username,
		Action:       models.ActionAuthzDenied,
		Resource:     resource,
		ResourceID:   resourceID,
		Status:       models.StatusFailure,
		Severity:     models.SeverityHigh,
		IPAddress:    ipAddress,
		Path:         path,
		AuthzContext: authzContext,
		Description:  fmt.Sprintf("Authorization denied for %s: required permission '%s', user roles: %v", username, requiredPermission, userRoles),
		ServiceName:  "audit-service",
		Timestamp:    time.Now(),
	}

	return s.LogAction(ctx, tenantID, log)
}

// ============================================================================
// Consent Change Tracking
// ============================================================================

// LogConsentChange logs a consent grant, withdrawal, or update
func (s *AuditService) LogConsentChange(ctx context.Context, tenantID string, userID uuid.UUID, username string, action models.AuditAction, consentPurpose string, customerID string, ipAddress string) error {
	log := &models.AuditLog{
		TenantID:       tenantID,
		UserID:         userID,
		Username:       username,
		Action:         action,
		Resource:       models.ResourceConsent,
		ResourceID:     customerID,
		Status:         models.StatusSuccess,
		Severity:       models.SeverityHigh,
		IPAddress:      ipAddress,
		ConsentPurpose: consentPurpose,
		Description:    fmt.Sprintf("Consent %s: purpose '%s' for customer %s", action, consentPurpose, customerID),
		ServiceName:    "audit-service",
		Timestamp:      time.Now(),
	}

	return s.LogAction(ctx, tenantID, log)
}

// GetConsentHistory retrieves consent change history for a customer
func (s *AuditService) GetConsentHistory(ctx context.Context, tenantID string, customerID string) ([]models.AuditLog, int64, error) {
	filter := &models.AuditLogFilter{
		TenantID:   tenantID,
		Resource:   models.ResourceConsent,
		ResourceID: customerID,
		Limit:      500,
		SortBy:     "timestamp",
		SortOrder:  "DESC",
	}

	return s.SearchAuditLogs(ctx, tenantID, filter)
}

// ============================================================================
// Compliance Query Endpoint
// ============================================================================

// ComplianceReport represents a compliance summary report
type ComplianceReport struct {
	TenantID           string                 `json:"tenantId"`
	GeneratedAt        time.Time              `json:"generatedAt"`
	Period             TimeRange              `json:"period"`
	RetentionSettings  *models.RetentionSettings `json:"retentionSettings"`
	TotalAuditLogs     int64                  `json:"totalAuditLogs"`
	PIIAccessCount     int64                  `json:"piiAccessCount"`
	EncryptionOps      map[string]int64       `json:"encryptionOps"`
	FailedAuthAttempts int64                  `json:"failedAuthAttempts"`
	AuthzDenials       int64                  `json:"authzDenials"`
	ConsentChanges     map[string]int64       `json:"consentChanges"`
	CriticalEvents     int64                  `json:"criticalEvents"`
	DataExports        int64                  `json:"dataExports"`
}

// TimeRange represents a time range
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// GenerateComplianceReport generates a compliance report for the given period
func (s *AuditService) GenerateComplianceReport(ctx context.Context, tenantID string, fromDate, toDate time.Time) (*ComplianceReport, error) {
	report := &ComplianceReport{
		TenantID:    tenantID,
		GeneratedAt: time.Now(),
		Period:      TimeRange{From: fromDate, To: toDate},
	}

	// Get retention settings
	settings, err := s.GetRetentionSettings(ctx, tenantID)
	if err == nil {
		report.RetentionSettings = settings
	}

	// Get summary for the period
	summary, err := s.GetSummary(ctx, tenantID, fromDate, toDate)
	if err == nil {
		report.TotalAuditLogs = summary.TotalLogs
		if count, ok := summary.ByAction[string(models.ActionPIIAccess)]; ok {
			report.PIIAccessCount = count
		}
		if count, ok := summary.ByAction[string(models.ActionLoginFailed)]; ok {
			report.FailedAuthAttempts = count
		}
		if count, ok := summary.ByAction[string(models.ActionAuthzDenied)]; ok {
			report.AuthzDenials = count
		}
		if count, ok := summary.BySeverity[string(models.SeverityCritical)]; ok {
			report.CriticalEvents = count
		}
		if count, ok := summary.ByAction[string(models.ActionExport)]; ok {
			report.DataExports = count
		}
	}

	// Aggregate encryption operations
	report.EncryptionOps = map[string]int64{}
	for _, action := range []models.AuditAction{models.ActionEncrypt, models.ActionDecrypt, models.ActionEncryptFail, models.ActionDecryptFail} {
		if summary != nil {
			if count, ok := summary.ByAction[string(action)]; ok {
				report.EncryptionOps[string(action)] = count
			}
		}
	}

	// Aggregate consent changes
	report.ConsentChanges = map[string]int64{}
	for _, action := range []models.AuditAction{models.ActionConsentGrant, models.ActionConsentWithdraw, models.ActionConsentUpdate} {
		if summary != nil {
			if count, ok := summary.ByAction[string(action)]; ok {
				report.ConsentChanges[string(action)] = count
			}
		}
	}

	return report, nil
}
