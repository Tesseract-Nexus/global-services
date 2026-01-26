package models

import "time"

// ProvisionSecretsResponse is the response after provisioning secrets
// NOTE: This NEVER contains secret values
type ProvisionSecretsResponse struct {
	Status     string            `json:"status"`
	SecretRefs []SecretReference `json:"secret_refs"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// SecretReference contains reference information about a secret (NOT the value)
type SecretReference struct {
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Provider  string    `json:"provider"`
	Key       string    `json:"key"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidationResult contains the result of credential validation
type ValidationResult struct {
	Status  ValidationStatus  `json:"status"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// SecretMetadataResponse is the response for getting secret metadata
type SecretMetadataResponse struct {
	Secrets []SecretMetadataItem `json:"secrets"`
}

// SecretMetadataItem represents a single secret's metadata
type SecretMetadataItem struct {
	Name             string           `json:"name"`
	Category         SecretCategory   `json:"category"`
	Provider         string           `json:"provider"`
	KeyName          string           `json:"key_name"`
	Scope            SecretScope      `json:"scope"`
	ScopeID          *string          `json:"scope_id,omitempty"`
	Configured       bool             `json:"configured"`
	ValidationStatus ValidationStatus `json:"validation_status"`
	LastUpdated      time.Time        `json:"last_updated"`
}

// ListProvidersResponse is the response for listing configured providers
type ListProvidersResponse struct {
	TenantID  string           `json:"tenant_id"`
	Category  SecretCategory   `json:"category"`
	Providers []ProviderStatus `json:"providers"`
}

// ProviderStatus represents the configuration status of a provider
type ProviderStatus struct {
	Provider             string               `json:"provider"`
	TenantConfigured     bool                 `json:"tenant_configured"`
	VendorConfigurations []VendorConfigStatus `json:"vendor_configurations,omitempty"`
}

// VendorConfigStatus represents a vendor's configuration status
type VendorConfigStatus struct {
	VendorID   string `json:"vendor_id"`
	Configured bool   `json:"configured"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}
