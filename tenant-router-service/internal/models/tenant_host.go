package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HostStatus represents the status of a tenant host configuration
type HostStatus string

const (
	HostStatusPending     HostStatus = "pending"      // Initial state, waiting for provisioning
	HostStatusProvisioned HostStatus = "provisioned"  // All resources created successfully
	HostStatusFailed      HostStatus = "failed"       // Provisioning failed
	HostStatusDeleting    HostStatus = "deleting"     // Being deleted
	HostStatusDeleted     HostStatus = "deleted"      // Soft deleted
)

// TenantHostRecord stores metadata about tenant host configurations
// This is the database model for tracking provisioned tenant subdomains
type TenantHostRecord struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	TenantID       string     `gorm:"type:varchar(255);not null;index:idx_tenant_host_tenant_id" json:"tenant_id"`
	Slug           string     `gorm:"type:varchar(63);not null;index:idx_tenant_host_slug_lookup" json:"slug"` // Partial unique index created via migration
	AdminHost      string     `gorm:"type:varchar(255);not null;index:idx_tenant_host_admin" json:"admin_host"`
	StorefrontHost string     `gorm:"type:varchar(255);not null;index:idx_tenant_host_storefront" json:"storefront_host"`
	APIHost        string     `gorm:"type:varchar(255);index:idx_tenant_host_api" json:"api_host"` // Mobile/external API host (e.g., awesome-store-api.tesserix.app)
	CertName       string     `gorm:"type:varchar(255);not null" json:"cert_name"`
	Status         HostStatus `gorm:"type:varchar(50);not null;default:'pending';index:idx_tenant_host_status" json:"status"`

	// Resource tracking
	CertificateCreated   bool `gorm:"default:false" json:"certificate_created"`
	GatewayPatched       bool `gorm:"default:false" json:"gateway_patched"`
	AdminVSPatched       bool `gorm:"default:false" json:"admin_vs_patched"`
	StorefrontVSPatched  bool `gorm:"default:false" json:"storefront_vs_patched"`
	APIVSPatched         bool `gorm:"default:false" json:"api_vs_patched"` // API VirtualService for mobile/external access

	// Namespace tracking for cross-namespace discovery
	CertificateNamespace  string `gorm:"type:varchar(255)" json:"certificate_namespace,omitempty"`
	GatewayNamespace      string `gorm:"type:varchar(255)" json:"gateway_namespace,omitempty"`
	AdminVSNamespace      string `gorm:"type:varchar(255)" json:"admin_vs_namespace,omitempty"`
	StorefrontVSNamespace string `gorm:"type:varchar(255)" json:"storefront_vs_namespace,omitempty"`
	APIVSNamespace        string `gorm:"type:varchar(255)" json:"api_vs_namespace,omitempty"`

	// Error tracking
	LastError    string     `gorm:"type:text" json:"last_error,omitempty"`
	RetryCount   int        `gorm:"default:0" json:"retry_count"`
	LastRetryAt  *time.Time `json:"last_retry_at,omitempty"`

	// Metadata
	Product      string `gorm:"type:varchar(100)" json:"product,omitempty"`       // e.g., "marketplace", "ecommerce"
	BusinessName string `gorm:"type:varchar(255)" json:"business_name,omitempty"`
	Email        string `gorm:"type:varchar(255)" json:"email,omitempty"`

	// Timestamps
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	ProvisionedAt *time.Time   `json:"provisioned_at,omitempty"`
}

// TableName specifies the table name for TenantHostRecord
func (TenantHostRecord) TableName() string {
	return "tenant_host_records"
}

// BeforeCreate sets UUID before creating record
func (t *TenantHostRecord) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// IsFullyProvisioned returns true if all resources are provisioned
func (t *TenantHostRecord) IsFullyProvisioned() bool {
	return t.CertificateCreated && t.GatewayPatched && t.AdminVSPatched && t.StorefrontVSPatched && t.APIVSPatched
}

// MarkProvisioned updates the status and timestamp when fully provisioned
func (t *TenantHostRecord) MarkProvisioned() {
	t.Status = HostStatusProvisioned
	now := time.Now()
	t.ProvisionedAt = &now
}

// MarkFailed updates the status and error message
func (t *TenantHostRecord) MarkFailed(err string) {
	t.Status = HostStatusFailed
	t.LastError = err
	t.RetryCount++
	now := time.Now()
	t.LastRetryAt = &now
}

// ProvisioningActivityLog tracks individual provisioning actions for audit
type ProvisioningActivityLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	TenantHostID uuid.UUID `gorm:"type:uuid;not null;index:idx_activity_tenant_host" json:"tenant_host_id"`
	Action       string    `gorm:"type:varchar(100);not null" json:"action"` // create_cert, patch_gateway, patch_vs, delete_cert, etc.
	Resource     string    `gorm:"type:varchar(100)" json:"resource"`        // certificate, gateway, virtualservice
	Namespace    string    `gorm:"type:varchar(255)" json:"namespace"`
	Success      bool      `gorm:"default:false" json:"success"`
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	Duration     int64     `gorm:"default:0" json:"duration_ms"` // Duration in milliseconds
	CreatedAt    time.Time `json:"created_at"`
}

// TableName specifies the table name for ProvisioningActivityLog
func (ProvisioningActivityLog) TableName() string {
	return "provisioning_activity_logs"
}

// BeforeCreate sets UUID before creating record
func (p *ProvisioningActivityLog) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
