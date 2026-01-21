package models

import "time"

// TenantCreatedEvent represents the event received when a tenant is created
// This matches the event published by tenant-service
type TenantCreatedEvent struct {
	EventType    string    `json:"event_type"`
	TenantID     string    `json:"tenant_id"`
	SessionID    string    `json:"session_id"`
	Product      string    `json:"product"`
	BusinessName string    `json:"business_name"`
	Slug         string    `json:"slug"`
	Email        string    `json:"email"`
	// Host URLs provided by tenant-service
	AdminHost         string `json:"admin_host"`           // e.g., "mystore-admin.tesserix.app" or "admin.customdomain.com"
	StorefrontHost    string `json:"storefront_host"`      // e.g., "mystore.tesserix.app" or "customdomain.com"
	StorefrontWwwHost string `json:"storefront_www_host"`  // e.g., "www.customdomain.com" (only for custom domains)
	APIHost           string `json:"api_host"`             // e.g., "mystore-api.tesserix.app" or "api.customdomain.com"
	BaseDomain        string `json:"base_domain"`          // e.g., "tesserix.app"
	IsCustomDomain    bool   `json:"is_custom_domain"`     // true if using custom domain
	Timestamp         time.Time `json:"timestamp"`
}

// TenantDeletedEvent represents the event received when a tenant is deleted
type TenantDeletedEvent struct {
	EventType      string    `json:"event_type"`
	TenantID       string    `json:"tenant_id"`
	Slug           string    `json:"slug"`
	AdminHost      string    `json:"admin_host"`
	StorefrontHost string    `json:"storefront_host"`
	Timestamp      time.Time `json:"timestamp"`
}

// TenantHost represents the hosts configured for a tenant
type TenantHost struct {
	TenantID       string    `json:"tenant_id"`
	Slug           string    `json:"slug"`
	AdminHost      string    `json:"admin_host"`
	StorefrontHost string    `json:"storefront_host"`
	CertName       string    `json:"cert_name"`
	Status         string    `json:"status"` // "pending", "provisioned", "failed"
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProvisionResult represents the result of provisioning tenant resources
type ProvisionResult struct {
	TenantID       string   `json:"tenant_id"`
	Slug           string   `json:"slug"`
	AdminHost      string   `json:"admin_host"`
	StorefrontHost string   `json:"storefront_host"`
	CertName       string   `json:"cert_name"`
	Errors         []string `json:"errors,omitempty"`
	Success        bool     `json:"success"`
}
