package models

import (
	"time"

	"github.com/google/uuid"
)

// TenantAuditConfig represents the configuration for tenant audit logging
// This is returned by the /api/v1/tenants/{id}/audit-config endpoint
// for the audit-service to know where to store audit logs
type TenantAuditConfig struct {
	TenantID       uuid.UUID      `json:"tenant_id"`
	ProductID      string         `json:"product_id"`
	ProductName    string         `json:"product_name"`
	VendorID       string         `json:"vendor_id"`
	VendorName     string         `json:"vendor_name"`
	DatabaseConfig DatabaseConfig `json:"database_config"`
	IsActive       bool           `json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Features       TenantFeatures `json:"features"`
}

// DatabaseConfig contains database connection settings for audit logs
type DatabaseConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	DatabaseName string `json:"database_name"`
	SSLMode      string `json:"ssl_mode"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
	MaxLifetime  int    `json:"max_lifetime_seconds"`
}

// TenantFeatures contains feature flags for audit logging
type TenantFeatures struct {
	AuditLogsEnabled bool `json:"audit_logs_enabled"`
	RealTimeEnabled  bool `json:"real_time_enabled"`
	ExportEnabled    bool `json:"export_enabled"`
	RetentionDays    int  `json:"retention_days"`
	MaxLogsPerDay    int  `json:"max_logs_per_day"`
	EncryptionAtRest bool `json:"encryption_at_rest"`
}

// Tenant represents a tenant record from the tenants table
// This is a read-only model for the settings-service
type Tenant struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	Name            string     `json:"name" gorm:"not null"`
	Slug            string     `json:"slug" gorm:"unique;not null;size:50"`
	Subdomain       string     `json:"subdomain" gorm:"unique;not null"`
	DisplayName     string     `json:"display_name" gorm:"size:255"`
	BusinessType    string     `json:"business_type"`
	Industry        string     `json:"industry"`
	Status          string     `json:"status" gorm:"default:'active';index"`
	BusinessModel   string     `json:"business_model" gorm:"type:varchar(50);default:'ONLINE_STORE'"`
	DefaultTimezone string     `json:"default_timezone" gorm:"size:50;default:'UTC'"`
	DefaultCurrency string     `json:"default_currency" gorm:"size:3;default:'USD'"`
	PricingTier     string     `json:"pricing_tier" gorm:"size:50;default:'free'"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName returns the table name for Tenant
func (Tenant) TableName() string {
	return "tenants"
}

// TenantAuditConfigResponse is the API response wrapper
type TenantAuditConfigResponse struct {
	Success bool              `json:"success"`
	Data    TenantAuditConfig `json:"data,omitempty"`
	Message string            `json:"message,omitempty"`
}

// TenantListResponse is used for listing audit-enabled tenants
type TenantAuditListResponse struct {
	Success bool                `json:"success"`
	Data    []TenantAuditConfig `json:"data,omitempty"`
	Message string              `json:"message,omitempty"`
}
