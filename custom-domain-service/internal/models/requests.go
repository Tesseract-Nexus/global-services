package models

import "github.com/google/uuid"

// CreateDomainRequest represents a request to create a new custom domain
type CreateDomainRequest struct {
	Domain     string     `json:"domain" binding:"required,fqdn"`
	TargetType TargetType `json:"target_type" binding:"omitempty,oneof=storefront admin api"`
	IncludeWWW bool       `json:"include_www"`
	SetPrimary bool       `json:"set_primary"`
}

// UpdateDomainRequest represents a request to update domain settings
type UpdateDomainRequest struct {
	RedirectWWW   *bool `json:"redirect_www"`
	ForceHTTPS    *bool `json:"force_https"`
	PrimaryDomain *bool `json:"primary_domain"`
}

// VerifyDomainRequest represents a request to verify DNS
type VerifyDomainRequest struct {
	Force bool `json:"force"` // Force re-verification even if already verified
}

// DomainResponse represents a domain in API responses
type DomainResponse struct {
	ID                 uuid.UUID          `json:"id"`
	TenantID           uuid.UUID          `json:"tenant_id"`
	Domain             string             `json:"domain"`
	DomainType         DomainType         `json:"domain_type"`
	TargetType         TargetType         `json:"target_type"`
	Status             DomainStatus       `json:"status"`
	StatusMessage      string             `json:"status_message,omitempty"`
	DNSVerified        bool               `json:"dns_verified"`
	DNSVerifiedAt      *string            `json:"dns_verified_at,omitempty"`
	SSLStatus          SSLStatus          `json:"ssl_status"`
	SSLExpiresAt       *string            `json:"ssl_expires_at,omitempty"`
	RoutingStatus      RoutingStatus      `json:"routing_status"`
	RedirectWWW        bool               `json:"redirect_www"`
	ForceHTTPS         bool               `json:"force_https"`
	PrimaryDomain      bool               `json:"primary_domain"`
	IncludeWWW         bool               `json:"include_www"`
	ActivatedAt        *string            `json:"activated_at,omitempty"`
	CreatedAt          string             `json:"created_at"`
	UpdatedAt          string             `json:"updated_at"`
	DNSRecords         []DNSRecord        `json:"dns_records,omitempty"`
	VerificationMethod VerificationMethod `json:"verification_method"`
}

// DomainListResponse represents a list of domains
type DomainListResponse struct {
	Domains    []DomainResponse `json:"domains"`
	Total      int64            `json:"total"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	HasMore    bool             `json:"has_more"`
}

// DNSStatusResponse represents DNS verification status
type DNSStatusResponse struct {
	DomainID       uuid.UUID   `json:"domain_id"`
	Domain         string      `json:"domain"`
	IsVerified     bool        `json:"is_verified"`
	VerifiedAt     *string     `json:"verified_at,omitempty"`
	LastCheckedAt  *string     `json:"last_checked_at,omitempty"`
	CheckAttempts  int         `json:"check_attempts"`
	Records        []DNSRecord `json:"records"`
	Message        string      `json:"message,omitempty"`
}

// SSLStatusResponse represents SSL certificate status
type SSLStatusResponse struct {
	DomainID      uuid.UUID `json:"domain_id"`
	Domain        string    `json:"domain"`
	Status        SSLStatus `json:"status"`
	Provider      string    `json:"provider"`
	ExpiresAt     *string   `json:"expires_at,omitempty"`
	DaysRemaining *int      `json:"days_remaining,omitempty"`
	AutoRenew     bool      `json:"auto_renew"`
	LastError     string    `json:"last_error,omitempty"`
}

// DomainStatsResponse represents domain statistics
type DomainStatsResponse struct {
	TenantID      uuid.UUID `json:"tenant_id"`
	TotalDomains  int       `json:"total_domains"`
	ActiveDomains int       `json:"active_domains"`
	PendingDomains int      `json:"pending_domains"`
	FailedDomains int       `json:"failed_domains"`
	MaxAllowed    int       `json:"max_allowed"`
	CanAddMore    bool      `json:"can_add_more"`
}

// HealthCheckResponse represents domain health check result
type HealthCheckResponse struct {
	DomainID     uuid.UUID `json:"domain_id"`
	Domain       string    `json:"domain"`
	IsHealthy    bool      `json:"is_healthy"`
	ResponseTime int64     `json:"response_time_ms"`
	StatusCode   int       `json:"status_code,omitempty"`
	SSLValid     bool      `json:"ssl_valid"`
	SSLExpiresIn int       `json:"ssl_expires_in_days,omitempty"`
	CheckedAt    string    `json:"checked_at"`
	Message      string    `json:"message,omitempty"`
}

// InternalResolveResponse represents domain resolution for internal services
type InternalResolveResponse struct {
	Domain       string    `json:"domain"`
	TenantID     uuid.UUID `json:"tenant_id"`
	TenantSlug   string    `json:"tenant_slug"`
	TargetType   TargetType `json:"target_type"`
	IsActive     bool      `json:"is_active"`
	IsPrimary    bool      `json:"is_primary"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
