package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditAction represents the type of action performed
type AuditAction string

const (
	// Authentication actions
	ActionLogin          AuditAction = "LOGIN"
	ActionLogout         AuditAction = "LOGOUT"
	ActionLoginFailed    AuditAction = "LOGIN_FAILED"
	ActionPasswordReset  AuditAction = "PASSWORD_RESET"
	ActionPasswordChange AuditAction = "PASSWORD_CHANGE"

	// CRUD actions
	ActionCreate AuditAction = "CREATE"
	ActionRead   AuditAction = "READ"
	ActionUpdate AuditAction = "UPDATE"
	ActionDelete AuditAction = "DELETE"

	// Special actions
	ActionExport   AuditAction = "EXPORT"
	ActionImport   AuditAction = "IMPORT"
	ActionApprove  AuditAction = "APPROVE"
	ActionReject   AuditAction = "REJECT"
	ActionComplete AuditAction = "COMPLETE"
	ActionCancel   AuditAction = "CANCEL"

	// RBAC actions
	ActionRoleAssign   AuditAction = "ROLE_ASSIGN"
	ActionRoleRemove   AuditAction = "ROLE_REMOVE"
	ActionPermissionGrant AuditAction = "PERMISSION_GRANT"
	ActionPermissionRevoke AuditAction = "PERMISSION_REVOKE"

	// Configuration actions
	ActionConfigUpdate AuditAction = "CONFIG_UPDATE"
	ActionSettingChange AuditAction = "SETTING_CHANGE"

	// Other action
	ActionOther AuditAction = "OTHER"
)

// AuditResource represents the type of resource being audited
type AuditResource string

const (
	ResourceUser         AuditResource = "USER"
	ResourceRole         AuditResource = "ROLE"
	ResourcePermission   AuditResource = "PERMISSION"
	ResourceCategory     AuditResource = "CATEGORY"
	ResourceProduct      AuditResource = "PRODUCT"
	ResourceOrder        AuditResource = "ORDER"
	ResourceCustomer     AuditResource = "CUSTOMER"
	ResourceVendor       AuditResource = "VENDOR"
	ResourceReturn       AuditResource = "RETURN"
	ResourceRefund       AuditResource = "REFUND"
	ResourceShipment     AuditResource = "SHIPMENT"
	ResourcePayment      AuditResource = "PAYMENT"
	ResourceNotification AuditResource = "NOTIFICATION"
	ResourceDocument     AuditResource = "DOCUMENT"
	ResourceSettings     AuditResource = "SETTINGS"
	ResourceConfig       AuditResource = "CONFIG"
	ResourceAuth         AuditResource = "AUTH"
	ResourceInventory    AuditResource = "INVENTORY"
	ResourceStaff        AuditResource = "STAFF"
	ResourceCoupon       AuditResource = "COUPON"
	ResourceApproval     AuditResource = "APPROVAL"
	ResourceTicket       AuditResource = "TICKET"
	ResourceTenant       AuditResource = "TENANT"
	ResourceGiftCard     AuditResource = "GIFT_CARD"
	ResourceReview       AuditResource = "REVIEW"
	ResourceShippingRate AuditResource = "SHIPPING_RATE"
	ResourceOther        AuditResource = "OTHER"
)

// AuditStatus represents the outcome of the action
type AuditStatus string

const (
	StatusSuccess AuditStatus = "SUCCESS"
	StatusFailure AuditStatus = "FAILURE"
	StatusPending AuditStatus = "PENDING"
)

// AuditSeverity represents the severity/importance of the audit event
type AuditSeverity string

const (
	SeverityLow      AuditSeverity = "LOW"
	SeverityMedium   AuditSeverity = "MEDIUM"
	SeverityHigh     AuditSeverity = "HIGH"
	SeverityCritical AuditSeverity = "CRITICAL"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`

	// Tenant and user info
	TenantID string    `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	UserID   uuid.UUID `json:"userId" gorm:"type:uuid;index"`
	Username string    `json:"username" gorm:"type:varchar(255)"`
	UserEmail string   `json:"userEmail" gorm:"type:varchar(255)"`

	// Action details
	Action   AuditAction   `json:"action" gorm:"type:varchar(50);not null;index"`
	Resource AuditResource `json:"resource" gorm:"type:varchar(50);not null;index"`
	ResourceID string      `json:"resourceId" gorm:"type:varchar(255);index"` // ID of the resource being acted upon
	ResourceName string    `json:"resourceName" gorm:"type:varchar(255)"` // Name/title of resource for display

	// Outcome
	Status   AuditStatus   `json:"status" gorm:"type:varchar(20);not null;index"`
	Severity AuditSeverity `json:"severity" gorm:"type:varchar(20);default:'MEDIUM';index"`

	// Request details
	Method       string `json:"method" gorm:"type:varchar(10)"` // HTTP method
	Path         string `json:"path" gorm:"type:varchar(500)"` // Request path
	Query        string `json:"query" gorm:"type:text"` // Query parameters
	IPAddress    string `json:"ipAddress" gorm:"type:varchar(45);index"` // IPv4 or IPv6
	UserAgent    string `json:"userAgent" gorm:"type:text"`
	RequestID    string `json:"requestId" gorm:"type:varchar(100);index"` // Correlation ID

	// Changes tracking
	OldValue datatypes.JSON `json:"oldValue" gorm:"type:jsonb"` // Previous state
	NewValue datatypes.JSON `json:"newValue" gorm:"type:jsonb"` // New state
	Changes  datatypes.JSON `json:"changes" gorm:"type:jsonb"`  // Diff of changes

	// Additional context
	Description string         `json:"description" gorm:"type:text"` // Human-readable description
	Metadata    datatypes.JSON `json:"metadata" gorm:"type:jsonb"`   // Additional metadata
	Tags        datatypes.JSON `json:"tags" gorm:"type:jsonb"`       // Tags for categorization

	// Error details (if failed)
	ErrorMessage string `json:"errorMessage" gorm:"type:text"`
	ErrorCode    string `json:"errorCode" gorm:"type:varchar(50)"`

	// Service info
	ServiceName string `json:"serviceName" gorm:"type:varchar(100);index"` // Which service created this log
	ServiceVersion string `json:"serviceVersion" gorm:"type:varchar(50)"`

	// Timestamps
	Timestamp time.Time `json:"timestamp" gorm:"index;not null"` // When the action occurred
	CreatedAt time.Time `json:"createdAt"`
}

// AuditLogSummary represents aggregated statistics
type AuditLogSummary struct {
	TotalLogs      int64                  `json:"totalLogs"`
	ByAction       map[string]int64       `json:"byAction"`
	ByResource     map[string]int64       `json:"byResource"`
	ByStatus       map[string]int64       `json:"byStatus"`
	BySeverity     map[string]int64       `json:"bySeverity"`
	TopUsers       []UserActivity         `json:"topUsers"`
	RecentFailures []AuditLog             `json:"recentFailures"`
	TimeRange      TimeRange              `json:"timeRange"`
}

// UserActivity represents user activity statistics
type UserActivity struct {
	UserID    uuid.UUID `json:"userId"`
	Username  string    `json:"username"`
	UserEmail string    `json:"userEmail"`
	Count     int64     `json:"count"`
	LastActivity time.Time `json:"lastActivity"`
}

// TimeRange represents a time range for filtering
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// AuditSummary is an alias for AuditLogSummary (for backwards compatibility)
type AuditSummary = AuditLogSummary

// IPHistoryEntry represents an IP address history entry for a user
type IPHistoryEntry struct {
	IPAddress string    `json:"ipAddress"`
	Count     int64     `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// AuditLogFilter represents filter criteria for searching audit logs
type AuditLogFilter struct {
	TenantID     string        `json:"tenantId"`
	UserID       *uuid.UUID    `json:"userId"`
	Action       AuditAction   `json:"action"`
	Resource     AuditResource `json:"resource"`
	ResourceID   string        `json:"resourceId"`
	Status       AuditStatus   `json:"status"`
	Severity     AuditSeverity `json:"severity"`
	IPAddress    string        `json:"ipAddress"`
	ServiceName  string        `json:"serviceName"`
	FromDate     *time.Time    `json:"fromDate"`
	ToDate       *time.Time    `json:"toDate"`
	SearchText   string        `json:"searchText"` // Search in description, resource name, etc.
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	SortBy       string        `json:"sortBy"`
	SortOrder    string        `json:"sortOrder"` // ASC or DESC
}

// TableName specifies the table name
func (AuditLog) TableName() string {
	return "audit_logs"
}

// BeforeCreate hook to set timestamp
func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}
	return nil
}

// Helper methods

// IsSuccess checks if the action was successful
func (a *AuditLog) IsSuccess() bool {
	return a.Status == StatusSuccess
}

// IsFailure checks if the action failed
func (a *AuditLog) IsFailure() bool {
	return a.Status == StatusFailure
}

// IsCritical checks if the event is critical severity
func (a *AuditLog) IsCritical() bool {
	return a.Severity == SeverityCritical
}

// IsHighSeverity checks if the event is high severity
func (a *AuditLog) IsHighSeverity() bool {
	return a.Severity == SeverityHigh || a.Severity == SeverityCritical
}

// GetActionCategory returns the category of action
func (a *AuditLog) GetActionCategory() string {
	switch a.Action {
	case ActionLogin, ActionLogout, ActionLoginFailed, ActionPasswordReset, ActionPasswordChange:
		return "Authentication"
	case ActionCreate, ActionRead, ActionUpdate, ActionDelete:
		return "CRUD"
	case ActionRoleAssign, ActionRoleRemove, ActionPermissionGrant, ActionPermissionRevoke:
		return "RBAC"
	case ActionExport, ActionImport:
		return "Data Transfer"
	case ActionApprove, ActionReject, ActionComplete, ActionCancel:
		return "Workflow"
	case ActionConfigUpdate, ActionSettingChange:
		return "Configuration"
	default:
		return "Other"
	}
}

// ShouldAlert determines if this event should trigger an alert
func (a *AuditLog) ShouldAlert() bool {
	// Alert on critical events or failed sensitive operations
	if a.IsCritical() {
		return true
	}

	// Alert on failed authentication attempts
	if a.Action == ActionLoginFailed && a.IsFailure() {
		return true
	}

	// Alert on RBAC changes
	if a.GetActionCategory() == "RBAC" {
		return true
	}

	return false
}

// CreateAuditLog is a helper function to create audit logs easily
func CreateAuditLog(tenantID string, userID uuid.UUID, action AuditAction, resource AuditResource) *AuditLog {
	return &AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Status:    StatusSuccess,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
	}
}

// RetentionSettings represents tenant-specific audit log retention configuration
type RetentionSettings struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string    `json:"tenantId" gorm:"type:varchar(255);uniqueIndex;not null"`
	RetentionDays  int       `json:"retentionDays" gorm:"not null;default:180"`  // 3-12 months (90-365 days)
	LastCleanupAt  *time.Time `json:"lastCleanupAt"`
	LogsDeleted    int64     `json:"logsDeleted"`  // Total logs deleted in last cleanup
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName specifies the table name for retention settings
func (RetentionSettings) TableName() string {
	return "audit_retention_settings"
}

// RetentionOption represents a selectable retention period option
type RetentionOption struct {
	Months int    `json:"months"`
	Days   int    `json:"days"`
	Label  string `json:"label"`
}

// GetRetentionOptions returns available retention period options (3-12 months)
func GetRetentionOptions() []RetentionOption {
	return []RetentionOption{
		{Months: 3, Days: 90, Label: "3 months"},
		{Months: 4, Days: 120, Label: "4 months"},
		{Months: 5, Days: 150, Label: "5 months"},
		{Months: 6, Days: 180, Label: "6 months (Default)"},
		{Months: 7, Days: 210, Label: "7 months"},
		{Months: 8, Days: 240, Label: "8 months"},
		{Months: 9, Days: 270, Label: "9 months"},
		{Months: 10, Days: 300, Label: "10 months"},
		{Months: 11, Days: 330, Label: "11 months"},
		{Months: 12, Days: 365, Label: "12 months (1 year)"},
	}
}
