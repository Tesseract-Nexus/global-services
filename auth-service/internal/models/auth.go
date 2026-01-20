package models

import (
	"github.com/google/uuid"
	"time"
)

// User represents a user in the system (B2C multi-tenant)
type User struct {
	ID                     uuid.UUID       `json:"id" db:"id"`
	TenantID               uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	VendorID               *string         `json:"vendor_id,omitempty" db:"vendor_id"` // Vendor isolation for marketplace (Tenant -> Vendor -> Staff)
	Email                  string          `json:"email" db:"email"`
	FirstName              string          `json:"first_name" db:"first_name"`
	LastName               string          `json:"last_name" db:"last_name"`
	Name                   string          `json:"name" db:"-"` // Computed from first_name + last_name
	Phone                  *string         `json:"phone" db:"phone"`
	Role                   string          `json:"role" db:"role"`
	Status                 string          `json:"status" db:"status"`
	Password               string          `json:"-" db:"password"` // Plain text password column (legacy)
	PasswordHash           *string         `json:"-" db:"-"`        // For compatibility
	EmailVerified          bool            `json:"email_verified" db:"email_verified"`
	PhoneVerified          bool            `json:"phone_verified" db:"phone_verified"`
	DateOfBirth            *time.Time      `json:"date_of_birth" db:"date_of_birth"`
	Gender                 *string         `json:"gender" db:"gender"`
	ProfilePictureURL      *string         `json:"profile_picture_url" db:"profile_picture_url"`
	MarketingConsent       bool            `json:"marketing_consent" db:"marketing_consent"`
	StoreID                *uuid.UUID      `json:"store_id" db:"store_id"`
	IsActive               bool            `json:"is_active" db:"-"` // Computed from status == 'active'
	LastLoginAt            *time.Time      `json:"last_login_at" db:"last_login_at"`
	TwoFactorEnabled       bool            `json:"two_factor_enabled" db:"two_factor_enabled"`
	TOTPSecret             string          `json:"-" db:"totp_secret"` // Hidden from JSON
	TwoFactorVerifiedAt    *time.Time      `json:"two_factor_verified_at" db:"two_factor_verified_at"`
	BackupCodesGeneratedAt *time.Time      `json:"backup_codes_generated_at" db:"backup_codes_generated_at"`
	CreatedAt              time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at" db:"updated_at"`
	Roles                  []Role          `json:"roles,omitempty"`
	Permissions            []Permission    `json:"permissions,omitempty"`
	Store                  *Store          `json:"store,omitempty"`
	OAuthProviders         []OAuthProvider `json:"oauth_providers,omitempty"`
}

// Role represents a role in the RBAC system
type Role struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Description string       `json:"description" db:"description"`
	TenantID    string       `json:"tenant_id" db:"tenant_id"`
	IsSystem    bool         `json:"is_system" db:"is_system"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a permission in the RBAC system
type Permission struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	Description string    `json:"description" db:"description"`
	IsSystem    bool      `json:"is_system" db:"is_system"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// UserRole represents the many-to-many relationship between users and roles
type UserRole struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	RoleID    uuid.UUID `json:"role_id" db:"role_id"`
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RolePermission represents the many-to-many relationship between roles and permissions
type RolePermission struct {
	ID           uuid.UUID `json:"id" db:"id"`
	RoleID       uuid.UUID `json:"role_id" db:"role_id"`
	PermissionID uuid.UUID `json:"permission_id" db:"permission_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// UserPermission represents direct permissions assigned to users (bypassing roles)
type UserPermission struct {
	ID           uuid.UUID `json:"id" db:"id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	PermissionID uuid.UUID `json:"permission_id" db:"permission_id"`
	TenantID     string    `json:"tenant_id" db:"tenant_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Session represents a user session
type Session struct {
	ID                  uuid.UUID  `json:"id" db:"id"`
	UserID              uuid.UUID  `json:"user_id" db:"user_id"`
	TenantID            string     `json:"tenant_id" db:"tenant_id"`
	AccessToken         string     `json:"-" db:"access_token"`  // SECURITY: Never expose tokens in API responses
	RefreshToken        string     `json:"-" db:"refresh_token"` // SECURITY: Never expose tokens in API responses
	ExpiresAt           time.Time  `json:"expires_at" db:"expires_at"`
	IsActive            bool       `json:"is_active" db:"is_active"`
	IPAddress           string     `json:"-" db:"ip_address"`  // SECURITY: PII - hide from API responses
	UserAgent           string     `json:"-" db:"user_agent"`  // SECURITY: Hide from API responses
	TwoFactorVerified   bool       `json:"two_factor_verified" db:"two_factor_verified"`
	TwoFactorVerifiedAt *time.Time `json:"two_factor_verified_at" db:"two_factor_verified_at"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	TenantID    string    `json:"tenant_id"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	SessionID   uuid.UUID `json:"session_id"`
}

// Predefined system roles
const (
	RoleSuperAdmin      = "super_admin"
	RoleTenantAdmin     = "tenant_admin"
	RoleCategoryManager = "category_manager"
	RoleProductManager  = "product_manager"
	RoleVendorManager   = "vendor_manager"
	RoleStaff           = "staff"
	RoleVendor          = "vendor"
	RoleCustomer        = "customer"
)

// Predefined system permissions
const (
	// User management
	PermissionUserCreate = "user:create"
	PermissionUserRead   = "user:read"
	PermissionUserUpdate = "user:update"
	PermissionUserDelete = "user:delete"

	// Role management
	PermissionRoleCreate = "role:create"
	PermissionRoleRead   = "role:read"
	PermissionRoleUpdate = "role:update"
	PermissionRoleDelete = "role:delete"

	// Category management
	PermissionCategoryCreate  = "category:create"
	PermissionCategoryRead    = "category:read"
	PermissionCategoryUpdate  = "category:update"
	PermissionCategoryDelete  = "category:delete"
	PermissionCategoryApprove = "category:approve"

	// Product management
	PermissionProductCreate  = "product:create"
	PermissionProductRead    = "product:read"
	PermissionProductUpdate  = "product:update"
	PermissionProductDelete  = "product:delete"
	PermissionProductApprove = "product:approve"

	// Vendor management
	PermissionVendorCreate  = "vendor:create"
	PermissionVendorRead    = "vendor:read"
	PermissionVendorUpdate  = "vendor:update"
	PermissionVendorDelete  = "vendor:delete"
	PermissionVendorApprove = "vendor:approve"

	// Order management
	PermissionOrderCreate = "order:create"
	PermissionOrderRead   = "order:read"
	PermissionOrderUpdate = "order:update"
	PermissionOrderDelete = "order:delete"
	PermissionOrderCancel = "order:cancel"
	PermissionOrderRefund = "order:refund"

	// Settings management
	PermissionSettingsRead   = "settings:read"
	PermissionSettingsUpdate = "settings:update"

	// Dashboard access
	PermissionDashboardView = "dashboard:view"
	PermissionAnalyticsView = "analytics:view"

	// Security management
	PermissionSecurityManage = "security:manage" // Unlock accounts, view lockout status
)

// SystemPermissions returns all system permissions
func SystemPermissions() []Permission {
	return []Permission{
		{Name: PermissionUserCreate, Resource: "user", Action: "create", Description: "Create users", IsSystem: true},
		{Name: PermissionUserRead, Resource: "user", Action: "read", Description: "Read user information", IsSystem: true},
		{Name: PermissionUserUpdate, Resource: "user", Action: "update", Description: "Update user information", IsSystem: true},
		{Name: PermissionUserDelete, Resource: "user", Action: "delete", Description: "Delete users", IsSystem: true},

		{Name: PermissionRoleCreate, Resource: "role", Action: "create", Description: "Create roles", IsSystem: true},
		{Name: PermissionRoleRead, Resource: "role", Action: "read", Description: "Read role information", IsSystem: true},
		{Name: PermissionRoleUpdate, Resource: "role", Action: "update", Description: "Update role information", IsSystem: true},
		{Name: PermissionRoleDelete, Resource: "role", Action: "delete", Description: "Delete roles", IsSystem: true},

		{Name: PermissionCategoryCreate, Resource: "category", Action: "create", Description: "Create categories", IsSystem: true},
		{Name: PermissionCategoryRead, Resource: "category", Action: "read", Description: "Read category information", IsSystem: true},
		{Name: PermissionCategoryUpdate, Resource: "category", Action: "update", Description: "Update category information", IsSystem: true},
		{Name: PermissionCategoryDelete, Resource: "category", Action: "delete", Description: "Delete categories", IsSystem: true},
		{Name: PermissionCategoryApprove, Resource: "category", Action: "approve", Description: "Approve categories", IsSystem: true},

		{Name: PermissionProductCreate, Resource: "product", Action: "create", Description: "Create products", IsSystem: true},
		{Name: PermissionProductRead, Resource: "product", Action: "read", Description: "Read product information", IsSystem: true},
		{Name: PermissionProductUpdate, Resource: "product", Action: "update", Description: "Update product information", IsSystem: true},
		{Name: PermissionProductDelete, Resource: "product", Action: "delete", Description: "Delete products", IsSystem: true},
		{Name: PermissionProductApprove, Resource: "product", Action: "approve", Description: "Approve products", IsSystem: true},

		{Name: PermissionVendorCreate, Resource: "vendor", Action: "create", Description: "Create vendors", IsSystem: true},
		{Name: PermissionVendorRead, Resource: "vendor", Action: "read", Description: "Read vendor information", IsSystem: true},
		{Name: PermissionVendorUpdate, Resource: "vendor", Action: "update", Description: "Update vendor information", IsSystem: true},
		{Name: PermissionVendorDelete, Resource: "vendor", Action: "delete", Description: "Delete vendors", IsSystem: true},
		{Name: PermissionVendorApprove, Resource: "vendor", Action: "approve", Description: "Approve vendors", IsSystem: true},

		{Name: PermissionOrderCreate, Resource: "order", Action: "create", Description: "Create orders", IsSystem: true},
		{Name: PermissionOrderRead, Resource: "order", Action: "read", Description: "Read order information", IsSystem: true},
		{Name: PermissionOrderUpdate, Resource: "order", Action: "update", Description: "Update order information", IsSystem: true},
		{Name: PermissionOrderDelete, Resource: "order", Action: "delete", Description: "Delete orders", IsSystem: true},
		{Name: PermissionOrderCancel, Resource: "order", Action: "cancel", Description: "Cancel orders", IsSystem: true},
		{Name: PermissionOrderRefund, Resource: "order", Action: "refund", Description: "Refund orders", IsSystem: true},

		{Name: PermissionSettingsRead, Resource: "settings", Action: "read", Description: "Read system settings", IsSystem: true},
		{Name: PermissionSettingsUpdate, Resource: "settings", Action: "update", Description: "Update system settings", IsSystem: true},

		{Name: PermissionDashboardView, Resource: "dashboard", Action: "view", Description: "View dashboard", IsSystem: true},
		{Name: PermissionAnalyticsView, Resource: "analytics", Action: "view", Description: "View analytics", IsSystem: true},

		{Name: PermissionSecurityManage, Resource: "security", Action: "manage", Description: "Manage security settings and unlock accounts", IsSystem: true},
	}
}

// SystemRoles returns all system roles with their permissions
func SystemRoles() map[string][]string {
	return map[string][]string{
		RoleSuperAdmin: {
			// All permissions
			PermissionUserCreate, PermissionUserRead, PermissionUserUpdate, PermissionUserDelete,
			PermissionRoleCreate, PermissionRoleRead, PermissionRoleUpdate, PermissionRoleDelete,
			PermissionCategoryCreate, PermissionCategoryRead, PermissionCategoryUpdate, PermissionCategoryDelete, PermissionCategoryApprove,
			PermissionProductCreate, PermissionProductRead, PermissionProductUpdate, PermissionProductDelete, PermissionProductApprove,
			PermissionVendorCreate, PermissionVendorRead, PermissionVendorUpdate, PermissionVendorDelete, PermissionVendorApprove,
			PermissionOrderCreate, PermissionOrderRead, PermissionOrderUpdate, PermissionOrderDelete, PermissionOrderCancel, PermissionOrderRefund,
			PermissionSettingsRead, PermissionSettingsUpdate,
			PermissionDashboardView, PermissionAnalyticsView,
			PermissionSecurityManage,
		},
		RoleTenantAdmin: {
			// Tenant-level admin permissions
			PermissionUserCreate, PermissionUserRead, PermissionUserUpdate,
			PermissionRoleRead,
			PermissionCategoryCreate, PermissionCategoryRead, PermissionCategoryUpdate, PermissionCategoryDelete, PermissionCategoryApprove,
			PermissionProductCreate, PermissionProductRead, PermissionProductUpdate, PermissionProductDelete, PermissionProductApprove,
			PermissionVendorCreate, PermissionVendorRead, PermissionVendorUpdate, PermissionVendorDelete, PermissionVendorApprove,
			PermissionOrderRead, PermissionOrderUpdate, PermissionOrderCancel, PermissionOrderRefund,
			PermissionSettingsRead, PermissionSettingsUpdate,
			PermissionDashboardView, PermissionAnalyticsView,
			PermissionSecurityManage,
		},
		RoleCategoryManager: {
			PermissionCategoryCreate, PermissionCategoryRead, PermissionCategoryUpdate, PermissionCategoryDelete, PermissionCategoryApprove,
			PermissionDashboardView,
		},
		RoleProductManager: {
			PermissionProductCreate, PermissionProductRead, PermissionProductUpdate, PermissionProductDelete, PermissionProductApprove,
			PermissionCategoryRead,
			PermissionDashboardView,
		},
		RoleVendorManager: {
			PermissionVendorCreate, PermissionVendorRead, PermissionVendorUpdate, PermissionVendorDelete, PermissionVendorApprove,
			PermissionDashboardView,
		},
		RoleStaff: {
			PermissionCategoryRead,
			PermissionProductRead,
			PermissionVendorRead,
			PermissionOrderRead,
			PermissionDashboardView,
		},
		RoleVendor: {
			PermissionProductCreate, PermissionProductRead, PermissionProductUpdate,
			PermissionOrderRead,
			PermissionDashboardView,
		},
		RoleCustomer: {
			PermissionOrderCreate, PermissionOrderRead,
		},
	}
}

// 2FA Models

// BackupCode represents a backup code for 2FA recovery
type BackupCode struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	CodeHash  string     `json:"-" db:"code_hash"` // Hidden from JSON
	UsedAt    *time.Time `json:"used_at" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// TwoFactorRecoveryLog represents a 2FA recovery attempt log
type TwoFactorRecoveryLog struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      *uuid.UUID `json:"user_id" db:"user_id"`
	AttemptType string     `json:"attempt_type" db:"attempt_type"`
	Success     bool       `json:"success" db:"success"`
	IPAddress   string     `json:"ip_address" db:"ip_address"`
	UserAgent   string     `json:"user_agent" db:"user_agent"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// VerificationToken represents email/phone verification tokens
type VerificationToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Token     string     `json:"-" db:"token"` // SECURITY: Never expose verification tokens
	TokenType string     `json:"token_type" db:"token_type"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt    *time.Time `json:"used_at" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// Store represents a marketplace store
type Store struct {
	ID            uuid.UUID `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Slug          string    `json:"slug" db:"slug"`
	Domain        *string   `json:"domain" db:"domain"`
	Subdomain     *string   `json:"subdomain" db:"subdomain"`
	Description   *string   `json:"description" db:"description"`
	LogoURL       *string   `json:"logo_url" db:"logo_url"`
	ThemeSettings string    `json:"theme_settings" db:"theme_settings"` // JSON
	IsActive      bool      `json:"is_active" db:"is_active"`
	OwnerEmail    string    `json:"owner_email" db:"owner_email"`
	Plan          string    `json:"plan" db:"plan"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// OAuthProvider represents OAuth provider connections
type OAuthProvider struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	Provider          string     `json:"provider" db:"provider"`
	ProviderUserID    string     `json:"provider_user_id" db:"provider_user_id"`
	Email             *string    `json:"email" db:"email"`
	Name              *string    `json:"name" db:"name"`
	ProfilePictureURL *string    `json:"profile_picture_url" db:"profile_picture_url"`
	AccessToken       *string    `json:"-" db:"access_token"`  // Hidden from JSON
	RefreshToken      *string    `json:"-" db:"refresh_token"` // Hidden from JSON
	TokenExpiresAt    *time.Time `json:"token_expires_at" db:"token_expires_at"`
	ProfileData       string     `json:"profile_data" db:"profile_data"` // JSON
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}
