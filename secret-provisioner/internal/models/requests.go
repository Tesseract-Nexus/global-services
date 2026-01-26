package models

// ProvisionSecretsRequest is the request body for creating/updating secrets
type ProvisionSecretsRequest struct {
	TenantID string            `json:"tenant_id" binding:"required"`
	Category SecretCategory    `json:"category" binding:"required"`
	Scope    SecretScope       `json:"scope" binding:"required"`
	ScopeID  *string           `json:"scope_id"` // VendorID if scope=vendor
	Provider string            `json:"provider" binding:"required"`
	Secrets  map[string]string `json:"secrets" binding:"required"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Validate bool              `json:"validate"`
}

// GetSecretMetadataRequest is the request for getting secret metadata
type GetSecretMetadataRequest struct {
	TenantID string         `form:"tenant_id" binding:"required"`
	Category SecretCategory `form:"category"`
	Provider string         `form:"provider"`
	Scope    SecretScope    `form:"scope"`
	ScopeID  string         `form:"scope_id"`
}

// ListProvidersRequest is the request for listing configured providers
type ListProvidersRequest struct {
	TenantID string         `form:"tenant_id" binding:"required"`
	Category SecretCategory `form:"category" binding:"required"`
}

// DeleteSecretRequest is the request for deleting a secret
type DeleteSecretRequest struct {
	SecretName string `json:"secret_name" binding:"required"`
	TenantID   string `json:"tenant_id" binding:"required"`
}
