package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DomainStatus represents the overall status of a custom domain
type DomainStatus string

const (
	DomainStatusPending      DomainStatus = "pending"
	DomainStatusVerifying    DomainStatus = "verifying"
	DomainStatusProvisioning DomainStatus = "provisioning"
	DomainStatusActive       DomainStatus = "active"
	DomainStatusInactive     DomainStatus = "inactive"
	DomainStatusFailed       DomainStatus = "failed"
	DomainStatusDeleting     DomainStatus = "deleting"
)

// DomainType represents the type of domain
type DomainType string

const (
	DomainTypeApex      DomainType = "apex"
	DomainTypeSubdomain DomainType = "subdomain"
)

// TargetType represents what the domain points to
type TargetType string

const (
	TargetTypeStorefront TargetType = "storefront"
	TargetTypeAdmin      TargetType = "admin"
	TargetTypeAPI        TargetType = "api"
)

// VerificationMethod represents DNS verification method
type VerificationMethod string

const (
	VerificationMethodCNAME VerificationMethod = "cname"
	VerificationMethodTXT   VerificationMethod = "txt"
)

// SSLStatus represents SSL certificate status
type SSLStatus string

const (
	SSLStatusPending      SSLStatus = "pending"
	SSLStatusProvisioning SSLStatus = "provisioning"
	SSLStatusActive       SSLStatus = "active"
	SSLStatusFailed       SSLStatus = "failed"
	SSLStatusExpiring     SSLStatus = "expiring"
	SSLStatusExpired      SSLStatus = "expired"
)

// RoutingStatus represents routing configuration status
type RoutingStatus string

const (
	RoutingStatusPending     RoutingStatus = "pending"
	RoutingStatusConfiguring RoutingStatus = "configuring"
	RoutingStatusActive      RoutingStatus = "active"
	RoutingStatusFailed      RoutingStatus = "failed"
)

// CustomDomain represents a custom domain configuration
type CustomDomain struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  uuid.UUID      `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Domain    string         `json:"domain" gorm:"uniqueIndex;not null;size:255"`
	DomainType DomainType    `json:"domain_type" gorm:"size:20;default:'apex'"`
	TargetType TargetType    `json:"target_type" gorm:"size:20;default:'storefront'"`

	// Tenant info (denormalized for performance)
	TenantSlug string `json:"tenant_slug" gorm:"size:100;index"`

	// Verification
	VerificationMethod VerificationMethod `json:"verification_method" gorm:"size:20;default:'txt'"`
	VerificationToken  string             `json:"verification_token" gorm:"size:100"`
	VerificationRecord string             `json:"verification_record" gorm:"size:255"`
	DNSVerified        bool               `json:"dns_verified" gorm:"default:false"`
	DNSVerifiedAt      *time.Time         `json:"dns_verified_at"`
	DNSLastCheckedAt   *time.Time         `json:"dns_last_checked_at"`
	DNSCheckAttempts   int                `json:"dns_check_attempts" gorm:"default:0"`

	// Session tracking for security - each onboarding session gets a unique verification token
	// This prevents cross-tenant token reuse and verification hijacking
	SessionID string `json:"session_id" gorm:"size:100;index"`

	// SSL/TLS
	SSLStatus         SSLStatus  `json:"ssl_status" gorm:"size:20;default:'pending'"`
	SSLProvider       string     `json:"ssl_provider" gorm:"size:50;default:'letsencrypt'"`
	SSLExpiresAt      *time.Time `json:"ssl_expires_at"`
	SSLCertSecretName string     `json:"ssl_cert_secret_name" gorm:"size:100"`
	SSLLastError      string     `json:"ssl_last_error" gorm:"size:500"`

	// Routing
	RoutingStatus      RoutingStatus `json:"routing_status" gorm:"size:20;default:'pending'"`
	VirtualServiceName string        `json:"virtual_service_name" gorm:"size:100"`
	GatewayPatched     bool          `json:"gateway_patched" gorm:"default:false"`

	// Keycloak
	KeycloakUpdated   bool       `json:"keycloak_updated" gorm:"default:false"`
	KeycloakUpdatedAt *time.Time `json:"keycloak_updated_at"`

	// Cloudflare Tunnel
	CloudflareTunnelConfigured bool       `json:"cloudflare_tunnel_configured" gorm:"default:false"`
	CloudflareDNSConfigured    bool       `json:"cloudflare_dns_configured" gorm:"default:false"`
	CloudflareZoneID           string     `json:"cloudflare_zone_id" gorm:"size:100"`
	TunnelLastCheckedAt        *time.Time `json:"tunnel_last_checked_at"`

	// Overall Status
	Status        DomainStatus `json:"status" gorm:"size:20;default:'pending';index"`
	StatusMessage string       `json:"status_message" gorm:"size:500"`
	ActivatedAt   *time.Time   `json:"activated_at"`

	// Settings
	RedirectWWW   bool `json:"redirect_www" gorm:"default:true"`
	ForceHTTPS    bool `json:"force_https" gorm:"default:true"`
	PrimaryDomain bool `json:"primary_domain" gorm:"default:false"`
	IncludeWWW    bool `json:"include_www" gorm:"default:true"`

	// Audit
	CreatedBy uuid.UUID      `json:"created_by" gorm:"type:uuid"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName returns the table name for GORM
func (CustomDomain) TableName() string {
	return "custom_domains"
}

// BeforeCreate hook to generate UUID if not set
func (d *CustomDomain) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.VerificationToken == "" {
		d.VerificationToken = generateVerificationToken()
	}
	return nil
}

// IsVerified returns true if DNS is verified
func (d *CustomDomain) IsVerified() bool {
	return d.DNSVerified && d.DNSVerifiedAt != nil
}

// IsSSLActive returns true if SSL is active
func (d *CustomDomain) IsSSLActive() bool {
	return d.SSLStatus == SSLStatusActive
}

// IsActive returns true if domain is fully active
func (d *CustomDomain) IsActive() bool {
	return d.Status == DomainStatusActive
}

// CanRetryVerification returns true if verification can be retried
func (d *CustomDomain) CanRetryVerification() bool {
	return d.Status == DomainStatusPending || d.Status == DomainStatusVerifying || d.Status == DomainStatusFailed
}

// GetAllHosts returns all hosts for this domain (including www if enabled)
func (d *CustomDomain) GetAllHosts() []string {
	hosts := []string{d.Domain}
	if d.IncludeWWW && d.DomainType == DomainTypeApex {
		hosts = append(hosts, "www."+d.Domain)
	}
	return hosts
}

// generateVerificationToken generates a secure verification token
func generateVerificationToken() string {
	return uuid.New().String()[:32]
}

// DomainActivity represents an activity log entry for a domain
type DomainActivity struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DomainID  uuid.UUID `json:"domain_id" gorm:"type:uuid;not null;index"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Action    string    `json:"action" gorm:"size:50;not null"`
	Status    string    `json:"status" gorm:"size:20;not null"`
	Message   string    `json:"message" gorm:"size:500"`
	Duration  int64     `json:"duration_ms"`
	Metadata  string    `json:"metadata" gorm:"type:jsonb"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName returns the table name for GORM
func (DomainActivity) TableName() string {
	return "domain_activities"
}

// DNSRecord represents a DNS record that needs to be configured
type DNSRecord struct {
	RecordType string `json:"record_type"` // TXT, CNAME, A
	Host       string `json:"host"`
	Value      string `json:"value"`
	TTL        int    `json:"ttl"`
	Purpose    string `json:"purpose"` // verification, routing
	IsVerified bool   `json:"is_verified"`
}

// DomainHealth represents health check status
type DomainHealth struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DomainID      uuid.UUID `json:"domain_id" gorm:"type:uuid;not null;index"`
	IsHealthy     bool      `json:"is_healthy"`
	ResponseTime  int64     `json:"response_time_ms"`
	StatusCode    int       `json:"status_code"`
	ErrorMessage  string    `json:"error_message" gorm:"size:500"`
	CheckedAt     time.Time `json:"checked_at"`
	SSLValid      bool      `json:"ssl_valid"`
	SSLExpiresIn  int       `json:"ssl_expires_in_days"`
}

// TableName returns the table name for GORM
func (DomainHealth) TableName() string {
	return "domain_health_checks"
}
