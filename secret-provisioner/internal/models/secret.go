package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecretCategory defines the type/category of secret
type SecretCategory string

const (
	CategoryPayment     SecretCategory = "payment"
	CategoryIntegration SecretCategory = "integration"
	CategoryAPIKey      SecretCategory = "api-key"
	CategoryOAuth       SecretCategory = "oauth"
	CategoryDatabase    SecretCategory = "database"
	CategoryWebhook     SecretCategory = "webhook"
)

// SecretScope defines the scope level for a secret
type SecretScope string

const (
	ScopeTenant  SecretScope = "tenant"
	ScopeVendor  SecretScope = "vendor"
	ScopeService SecretScope = "service"
)

// ValidationStatus represents the validation state of a secret
type ValidationStatus string

const (
	ValidationValid   ValidationStatus = "VALID"
	ValidationInvalid ValidationStatus = "INVALID"
	ValidationUnknown ValidationStatus = "UNKNOWN"
)

// SecretMetadata stores metadata about secrets (NOT the secret values!)
// This is stored in the database for UI display and status tracking.
type SecretMetadata struct {
	ID                uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID          string           `gorm:"type:varchar(100);not null;index:idx_secret_meta_tenant" json:"tenant_id"`
	Category          SecretCategory   `gorm:"type:varchar(50);not null;index:idx_secret_meta_category" json:"category"`
	Scope             SecretScope      `gorm:"type:varchar(20);not null" json:"scope"`
	ScopeID           *string          `gorm:"type:varchar(100)" json:"scope_id,omitempty"`
	Provider          string           `gorm:"type:varchar(50);not null;index:idx_secret_meta_provider" json:"provider"`
	KeyName           string           `gorm:"type:varchar(100);not null" json:"key_name"`
	GCPSecretName     string           `gorm:"type:varchar(255);not null;uniqueIndex" json:"gcp_secret_name"`
	GCPSecretVersion  *string          `gorm:"type:varchar(50)" json:"gcp_secret_version,omitempty"`
	Configured        bool             `gorm:"not null;default:true" json:"configured"`
	ValidationStatus  ValidationStatus `gorm:"type:varchar(20);default:'UNKNOWN'" json:"validation_status"`
	ValidationMessage *string          `gorm:"type:text" json:"validation_message,omitempty"`
	LastUpdatedBy     *string          `gorm:"type:varchar(100)" json:"last_updated_by,omitempty"`
	CreatedAt         time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for GORM
func (SecretMetadata) TableName() string {
	return "secret_metadata"
}

// BeforeCreate hook to set default ID
func (s *SecretMetadata) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// SecretAuditLog records all secret operations for audit trail
type SecretAuditLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string    `gorm:"type:varchar(100);not null;index:idx_audit_tenant" json:"tenant_id"`
	SecretName   string    `gorm:"type:varchar(255);not null;index:idx_audit_secret" json:"secret_name"`
	Category     string    `gorm:"type:varchar(50);not null" json:"category"`
	Provider     string    `gorm:"type:varchar(50);not null" json:"provider"`
	Action       string    `gorm:"type:varchar(50);not null" json:"action"`
	Status       string    `gorm:"type:varchar(20);not null" json:"status"`
	ErrorMessage *string   `gorm:"type:text" json:"error_message,omitempty"`
	ActorID      *string   `gorm:"type:varchar(100)" json:"actor_id,omitempty"`
	ActorService *string   `gorm:"type:varchar(100)" json:"actor_service,omitempty"`
	RequestID    *string   `gorm:"type:varchar(100)" json:"request_id,omitempty"`
	IPAddress    *string   `gorm:"type:varchar(50)" json:"ip_address,omitempty"`
	Metadata     []byte    `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime;index:idx_audit_time" json:"created_at"`
}

// TableName returns the table name for GORM
func (SecretAuditLog) TableName() string {
	return "secret_audit_log"
}

// BeforeCreate hook to set default ID
func (a *SecretAuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// AuditAction constants
const (
	AuditActionCreated   = "created"
	AuditActionUpdated   = "updated"
	AuditActionDeleted   = "deleted"
	AuditActionValidated = "validated"
	AuditActionAccessed  = "accessed"
)

// AuditStatus constants
const (
	AuditStatusSuccess = "success"
	AuditStatusFailure = "failure"
)
