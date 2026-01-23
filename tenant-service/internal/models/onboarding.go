package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONB is a custom type for PostgreSQL JSONB fields
// It can hold any valid JSON value (objects, arrays, primitives)
type JSONB json.RawMessage

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = JSONB(v)
		return nil
	case string:
		*j = JSONB([]byte(v))
		return nil
	default:
		return nil
	}
}

// MarshalJSON implements json.Marshaler
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*j = nil
		return nil
	}
	*j = JSONB(data)
	return nil
}

// NewJSONB creates a JSONB from any value
func NewJSONB(v interface{}) (JSONB, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return JSONB(data), nil
}

// MustNewJSONB creates a JSONB from any value, panics on error
func MustNewJSONB(v interface{}) JSONB {
	j, err := NewJSONB(v)
	if err != nil {
		panic(err)
	}
	return j
}

// OnboardingTemplate represents an onboarding flow template
type OnboardingTemplate struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name            string    `json:"name" gorm:"not null" validate:"required,min=2,max=255"`
	Description     string    `json:"description"`
	ApplicationType string    `json:"application_type" gorm:"not null" validate:"required,oneof=ecommerce saas marketplace b2b"`
	Version         int       `json:"version" gorm:"default:1"`
	IsActive        bool      `json:"is_active" gorm:"default:true"`
	IsDefault       bool      `json:"is_default" gorm:"default:false"`
	TemplateConfig  JSONB     `json:"template_config" gorm:"type:jsonb;default:'{}'"`
	Steps           JSONB     `json:"steps" gorm:"type:jsonb;default:'[]'"`
	Metadata        JSONB     `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OnboardingSession represents a tenant onboarding session
type OnboardingSession struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID           *uuid.UUID `json:"tenant_id" gorm:"type:uuid;index"`
	TemplateID         uuid.UUID  `json:"template_id" gorm:"type:uuid;not null"`
	ApplicationType    string     `json:"application_type" gorm:"not null;index" validate:"required"`
	Status             string     `json:"status" gorm:"default:'started';index" validate:"oneof=started in_progress completed failed abandoned draft"`
	CurrentStep        string     `json:"current_step" gorm:"index"`
	ProgressPercentage int        `json:"progress_percentage" gorm:"default:0"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	ExpiresAt          time.Time  `json:"expires_at" gorm:"index"`
	Metadata           JSONB      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Business model: ONLINE_STORE (single vendor, D2C) or MARKETPLACE (multi-vendor)
	BusinessModel string `json:"business_model" gorm:"type:varchar(50);default:'ONLINE_STORE';index" validate:"omitempty,oneof=ONLINE_STORE MARKETPLACE"`

	// Draft persistence fields
	DraftSavedAt    *time.Time `json:"draft_saved_at" gorm:"index"`
	DraftExpiresAt  *time.Time `json:"draft_expires_at" gorm:"index"`
	ReminderCount   int        `json:"reminder_count" gorm:"default:0"`
	LastReminderAt  *time.Time `json:"last_reminder_at"`
	BrowserClosedAt *time.Time `json:"browser_closed_at"`
	DraftFormData   JSONB      `json:"draft_form_data" gorm:"type:jsonb;default:'{}'"`

	// Relationships
	Template                  OnboardingTemplate         `json:"template,omitempty" gorm:"foreignKey:TemplateID"`
	BusinessInformation       *BusinessInformation       `json:"business_information,omitempty"`
	ContactInformation        []ContactInformation       `json:"contact_information,omitempty"`
	BusinessAddresses         []BusinessAddress          `json:"business_addresses,omitempty"`
	VerificationRecords       []VerificationRecord       `json:"verification_records,omitempty"`
	PaymentInformation        *PaymentInformation        `json:"payment_information,omitempty"`
	ApplicationConfigurations []ApplicationConfiguration `json:"application_configurations,omitempty"`
	Tasks                     []OnboardingTask           `json:"tasks,omitempty"`
	DomainReservations        []DomainReservation        `json:"domain_reservations,omitempty"`
	Notifications             []OnboardingNotification   `json:"notifications,omitempty"`
}

// BusinessInformation represents business details
type BusinessInformation struct {
	ID                    uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID   uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	BusinessName          string     `json:"business_name" gorm:"not null" validate:"required,min=2,max=255"`
	BusinessType          string     `json:"business_type" gorm:"not null" validate:"required"`
	Industry              string     `json:"industry" gorm:"not null" validate:"required"`
	BusinessDescription   string     `json:"business_description"`
	Website               string     `json:"website" validate:"omitempty,url"`
	RegistrationNumber    string     `json:"registration_number"`
	TaxID                 string     `json:"tax_id"`
	IncorporationDate     *time.Time `json:"incorporation_date"`
	EmployeeCount         string     `json:"employee_count"`
	AnnualRevenue         string     `json:"annual_revenue"`
	IsVerified            bool       `json:"is_verified" gorm:"default:false"`
	VerificationDocuments JSONB      `json:"verification_documents" gorm:"type:jsonb;default:'[]'"`
	// TenantSlug is the reserved URL-friendly slug for the tenant
	// This is set during onboarding when the user validates their store URL
	// and is used during account setup to create the tenant with this exact slug
	TenantSlug string `json:"tenant_slug" gorm:"size:50;index"`
	// StorefrontSlug is the slug for the default storefront
	// URL pattern: {storefront_slug}-store.{baseDomain}
	// If not set, defaults to the same as TenantSlug
	StorefrontSlug string `json:"storefront_slug" gorm:"size:50"`
	// Business model: ONLINE_STORE (single vendor, D2C) or MARKETPLACE (multi-vendor)
	BusinessModel string `json:"business_model" gorm:"type:varchar(50);default:'ONLINE_STORE'" validate:"omitempty,oneof=ONLINE_STORE MARKETPLACE"`
	// Existing store platforms for migration support
	// Values: shopify, etsy, temu, amazon, woocommerce, squarespace, bigcommerce, other
	ExistingStorePlatforms JSONB `json:"existing_store_platforms" gorm:"type:jsonb;default:'[]'"`
	HasExistingStore       bool  `json:"has_existing_store" gorm:"default:false"`
	MigrationInterest      bool  `json:"migration_interest" gorm:"default:false"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// ContactInformation represents contact details
type ContactInformation struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	FirstName           string    `json:"first_name" gorm:"not null" validate:"required,min=2,max=100"`
	LastName            string    `json:"last_name" gorm:"not null" validate:"required,min=2,max=100"`
	Email               string    `json:"email" gorm:"not null;index" validate:"required,email"`
	Phone               string    `json:"phone" gorm:"not null" validate:"required"`
	PhoneCountryCode    string    `json:"phone_country_code" gorm:"type:varchar(10);default:''"` // ISO country code (e.g., AU, US, IN)
	JobTitle            string    `json:"job_title"`
	IsPrimaryContact    bool      `json:"is_primary_contact" gorm:"default:true"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BusinessAddress represents business address information
type BusinessAddress struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	AddressType         string    `json:"address_type" gorm:"default:'business'" validate:"oneof=business billing shipping"`
	StreetAddress       string    `json:"street_address" gorm:"not null" validate:"required"`
	City                string    `json:"city" gorm:"not null" validate:"required"`
	StateProvince       string    `json:"state_province" gorm:"not null" validate:"required"`
	PostalCode          string    `json:"postal_code" gorm:"not null" validate:"required"`
	Country             string    `json:"country" gorm:"not null" validate:"required"`
	IsPrimary           bool      `json:"is_primary" gorm:"default:true"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// VerificationRecord represents verification attempts
type VerificationRecord struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	VerificationType    string     `json:"verification_type" gorm:"not null;index:idx_verification_type_status" validate:"required,oneof=email phone business identity"`
	VerificationMethod  string     `json:"verification_method" gorm:"not null" validate:"required,oneof=otp link document manual"`
	TargetValue         string     `json:"target_value" gorm:"not null" validate:"required"`
	VerificationCode    string     `json:"verification_code"`
	Status              string     `json:"status" gorm:"default:'pending';index:idx_verification_type_status" validate:"oneof=pending verified failed expired"`
	Attempts            int        `json:"attempts" gorm:"default:0"`
	MaxAttempts         int        `json:"max_attempts" gorm:"default:5"`
	ExpiresAt           time.Time  `json:"expires_at" gorm:"index"`
	VerifiedAt          *time.Time `json:"verified_at"`
	Metadata            JSONB      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// PaymentInformation represents payment and subscription details
type PaymentInformation struct {
	ID                            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID           uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	SubscriptionPlan              string     `json:"subscription_plan"`
	BillingCycle                  string     `json:"billing_cycle" validate:"omitempty,oneof=monthly quarterly yearly"`
	PaymentMethod                 string     `json:"payment_method" validate:"omitempty,oneof=credit_card bank_transfer paypal"`
	PaymentProvider               string     `json:"payment_provider" validate:"omitempty,oneof=stripe paypal razorpay"`
	PaymentProviderCustomerID     string     `json:"payment_provider_customer_id"`
	PaymentProviderSubscriptionID string     `json:"payment_provider_subscription_id"`
	TrialEndDate                  *time.Time `json:"trial_end_date"`
	BillingAddress                JSONB      `json:"billing_address" gorm:"type:jsonb"`
	PaymentStatus                 string     `json:"payment_status" gorm:"default:'pending';index" validate:"oneof=pending active failed cancelled"`
	SetupIntentID                 string     `json:"setup_intent_id"`
	Metadata                      JSONB      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt                     time.Time  `json:"created_at"`
	UpdatedAt                     time.Time  `json:"updated_at"`
}

// ApplicationConfiguration represents app-specific configuration
type ApplicationConfiguration struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	ApplicationType     string    `json:"application_type" gorm:"not null" validate:"required"`
	ConfigurationData   JSONB     `json:"configuration_data" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// OnboardingTask represents individual tasks in the onboarding flow
type OnboardingTask struct {
	ID                    uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID   uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	TaskID                string     `json:"task_id" gorm:"not null"`
	Name                  string     `json:"name" gorm:"not null" validate:"required"`
	Description           string     `json:"description"`
	TaskType              string     `json:"task_type" gorm:"not null" validate:"required"`
	Status                string     `json:"status" gorm:"default:'pending';index" validate:"oneof=pending in_progress completed skipped failed"`
	IsRequired            bool       `json:"is_required" gorm:"default:true"`
	OrderIndex            int        `json:"order_index" gorm:"not null"`
	EstimatedDurationMins int        `json:"estimated_duration_minutes"`
	Dependencies          JSONB      `json:"dependencies" gorm:"type:jsonb;default:'[]'"`
	CompletionData        JSONB      `json:"completion_data" gorm:"type:jsonb;default:'{}'"`
	StartedAt             *time.Time `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at"`
	SkippedAt             *time.Time `json:"skipped_at"`
	SkipReason            string     `json:"skip_reason"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// Relationships
	ExecutionLogs []TaskExecutionLog `json:"execution_logs,omitempty"`
}

// TaskExecutionLog represents task execution history
type TaskExecutionLog struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingTaskID uuid.UUID `json:"onboarding_task_id" gorm:"type:uuid;not null"`
	Action           string    `json:"action" gorm:"not null" validate:"required,oneof=started completed failed retried"`
	Details          JSONB     `json:"details" gorm:"type:jsonb;default:'{}'"`
	ErrorMessage     string    `json:"error_message"`
	PerformedBy      string    `json:"performed_by" gorm:"not null"`
	CreatedAt        time.Time `json:"created_at"`
}

// DomainReservation represents domain/subdomain reservations
type DomainReservation struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null"`
	DomainType          string     `json:"domain_type" gorm:"not null" validate:"required,oneof=subdomain custom_domain"`
	DomainValue         string     `json:"domain_value" gorm:"not null;unique;index" validate:"required"`
	Status              string     `json:"status" gorm:"default:'reserved';index" validate:"oneof=reserved verified active failed"`
	VerificationMethod  string     `json:"verification_method" validate:"omitempty,oneof=dns file meta_tag"`
	VerificationToken   string     `json:"verification_token"`
	VerifiedAt          *time.Time `json:"verified_at"`
	ExpiresAt           time.Time  `json:"expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// OnboardingNotification represents notifications sent during onboarding
type OnboardingNotification struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null;index"`
	NotificationType    string     `json:"notification_type" gorm:"not null" validate:"required,oneof=email sms in_app"`
	TemplateName        string     `json:"template_name" gorm:"not null" validate:"required"`
	Recipient           string     `json:"recipient" gorm:"not null" validate:"required"`
	Subject             string     `json:"subject"`
	Content             string     `json:"content"`
	Status              string     `json:"status" gorm:"default:'pending';index" validate:"oneof=pending sent delivered failed"`
	Provider            string     `json:"provider"`
	ProviderMessageID   string     `json:"provider_message_id"`
	SentAt              *time.Time `json:"sent_at"`
	DeliveredAt         *time.Time `json:"delivered_at"`
	ErrorMessage        string     `json:"error_message"`
	Metadata            JSONB      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt           time.Time  `json:"created_at"`
}

// WebhookEvent represents webhook events for integration
type WebhookEvent struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OnboardingSessionID uuid.UUID  `json:"onboarding_session_id" gorm:"type:uuid;not null"`
	EventType           string     `json:"event_type" gorm:"not null" validate:"required"`
	Payload             JSONB      `json:"payload" gorm:"type:jsonb;not null;default:'{}'"`
	WebhookURL          string     `json:"webhook_url"`
	Status              string     `json:"status" gorm:"default:'pending';index" validate:"oneof=pending sent failed"`
	Attempts            int        `json:"attempts" gorm:"default:0"`
	MaxAttempts         int        `json:"max_attempts" gorm:"default:3"`
	NextRetryAt         *time.Time `json:"next_retry_at" gorm:"index"`
	ResponseStatus      int        `json:"response_status"`
	ResponseBody        string     `json:"response_body"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// BeforeCreate hooks
func (o *OnboardingSession) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	if o.ExpiresAt.IsZero() {
		o.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // 7 days default
	}
	o.StartedAt = time.Now()
	return nil
}

func (b *BusinessInformation) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (c *ContactInformation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (v *VerificationRecord) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.ExpiresAt.IsZero() {
		v.ExpiresAt = time.Now().Add(15 * time.Minute) // 15 minutes default for OTP
	}
	return nil
}

func (t *OnboardingTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// Tenant represents a tenant in the system
type Tenant struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name         string    `json:"name" gorm:"not null" validate:"required,min=2,max=255"`
	Slug         string    `json:"slug" gorm:"unique;not null;size:50" validate:"required,min=3,max:50"`
	Subdomain    string    `json:"subdomain" gorm:"unique;not null" validate:"required"`
	DisplayName  string    `json:"display_name" gorm:"size:255"`
	LogoURL      string    `json:"logo_url"`
	BusinessType string    `json:"business_type"`
	Industry     string    `json:"industry"`
	Status       string    `json:"status" gorm:"default:'creating';index" validate:"oneof=creating active inactive suspended"`
	Mode         string    `json:"mode" gorm:"default:'development'" validate:"oneof=development production"`

	// Tenant URLs - stored for both custom domain and default tesserix.app domains
	// These URLs are the canonical endpoints for accessing the tenant's services
	AdminURL      string `json:"admin_url" gorm:"size:255"`      // e.g., https://admin.yahvismartfarm.com or https://default-store-admin.tesserix.app
	StorefrontURL string `json:"storefront_url" gorm:"size:255"` // e.g., https://yahvismartfarm.com or https://default-store.tesserix.app
	APIURL        string `json:"api_url" gorm:"size:255"`        // e.g., https://api.yahvismartfarm.com or https://default-store-api.tesserix.app

	// Custom domain configuration
	UseCustomDomain bool   `json:"use_custom_domain" gorm:"default:false"` // Whether custom domain is active
	CustomDomain    string `json:"custom_domain" gorm:"size:255"`          // Base custom domain (e.g., yahvismartfarm.com)

	// Business model: ONLINE_STORE (single vendor, D2C) or MARKETPLACE (multi-vendor)
	BusinessModel string `json:"business_model" gorm:"type:varchar(50);default:'ONLINE_STORE';index" validate:"omitempty,oneof=ONLINE_STORE MARKETPLACE"`

	// Theming
	PrimaryColor   string `json:"primary_color" gorm:"size:7;default:'#6366f1'"`
	SecondaryColor string `json:"secondary_color" gorm:"size:7;default:'#8b5cf6'"`

	// Defaults
	DefaultTimezone string `json:"default_timezone" gorm:"size:50;default:'UTC'"`
	DefaultCurrency string `json:"default_currency" gorm:"size:3;default:'USD'"`

	// Pricing tier for subscription management
	// Available tiers: free, starter, professional, enterprise
	// All tenants default to 'free' until monetization is enabled
	PricingTier          string     `json:"pricing_tier" gorm:"size:50;default:'free';index" validate:"oneof=free starter professional enterprise"`
	PricingTierUpdatedAt *time.Time `json:"pricing_tier_updated_at"`
	TrialEndsAt          *time.Time `json:"trial_ends_at"`
	BillingEmail         string     `json:"billing_email" gorm:"size:255"`

	// Owner tracking (user_id from auth-service)
	OwnerUserID *uuid.UUID `json:"owner_user_id" gorm:"type:uuid;index"`

	// Keycloak Organization ID for multi-tenant identity isolation
	// Each tenant maps 1:1 to a Keycloak Organization
	// Used for: user isolation per tenant, organization-scoped authentication
	KeycloakOrgID *uuid.UUID `json:"keycloak_org_id,omitempty" gorm:"type:uuid;index"`

	// GrowthBook feature flags integration
	// Each tenant gets their own GrowthBook organization for isolated feature flag management
	GrowthBookOrgID         string     `json:"growthbook_org_id,omitempty" gorm:"size:255"`
	GrowthBookSDKKey        string     `json:"-" gorm:"size:255"`                        // Hidden from JSON - sensitive
	GrowthBookAdminKey      string     `json:"-" gorm:"size:255"`                        // Hidden from JSON - sensitive
	GrowthBookEnabled       bool       `json:"growthbook_enabled" gorm:"default:false"`
	GrowthBookProvisionedAt *time.Time `json:"growthbook_provisioned_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Memberships []UserTenantMembership `json:"memberships,omitempty" gorm:"foreignKey:TenantID"`
}

// User represents a global user account (can belong to multiple tenants via memberships)
// This follows the GitHub organization model where one email can be part of multiple orgs/tenants
// The user-tenant relationship is managed through UserTenantMembership
type User struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	KeycloakID *uuid.UUID `json:"keycloak_id,omitempty" gorm:"type:uuid;index"` // Keycloak user ID for auth lookup
	Email      string     `json:"email" gorm:"unique;not null;index" validate:"required,email"`
	FirstName  string     `json:"first_name" gorm:"not null" validate:"required,min=2,max=100"`
	LastName   string     `json:"last_name" gorm:"not null" validate:"required,min=2,max=100"`
	Phone      string     `json:"phone"`
	Status     string     `json:"status" gorm:"default:'active';index" validate:"oneof=active inactive suspended"`
	Password   string     `json:"-" gorm:"not null"` // Hidden from JSON
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	// Relations - user can have memberships in multiple tenants
	Memberships []UserTenantMembership `json:"memberships,omitempty" gorm:"foreignKey:UserID"`
}

// TableName overrides the default table name to avoid conflict with auth-service users table
func (User) TableName() string {
	return "tenant_users"
}

// UserTenantMembership represents the many-to-many relationship between users and tenants
// This enables one user to own/manage multiple tenants with different roles
type UserTenantMembership struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID   uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`   // User ID from auth-service
	TenantID uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"` // FK to tenants table

	// Role within this tenant
	// owner: Full control, can delete tenant, manage billing
	// admin: Can manage products, orders, settings (not billing/delete)
	// manager: Can manage products, orders (not settings)
	// member: Read-only access with limited actions
	// viewer: Read-only access
	Role string `json:"role" gorm:"size:50;not null;default:'member'" validate:"oneof=owner admin manager member viewer"`

	// Fine-grained permissions as JSONB
	// Example: {"products": ["read", "write"], "orders": ["read"]}
	Permissions JSONB `json:"permissions" gorm:"type:jsonb;default:'{}'"`

	// Is this the user's default/primary tenant?
	IsDefault bool `json:"is_default" gorm:"default:false"`

	// Is this membership currently active?
	IsActive bool `json:"is_active" gorm:"default:true"`

	// Invitation tracking
	InvitedBy           *uuid.UUID `json:"invited_by" gorm:"type:uuid"`
	InvitedAt           *time.Time `json:"invited_at"`
	InvitationToken     string     `json:"invitation_token,omitempty" gorm:"size:255;index"`
	InvitationExpiresAt *time.Time `json:"invitation_expires_at"`
	AcceptedAt          *time.Time `json:"accepted_at"`

	// Activity tracking
	LastAccessedAt *time.Time `json:"last_accessed_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Tenant *Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}

// TableName specifies the table name for UserTenantMembership
func (UserTenantMembership) TableName() string {
	return "user_tenant_memberships"
}

// TenantActivityLog represents audit trail for tenant activities
type TenantActivityLog struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID     uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID       uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	Action       string     `json:"action" gorm:"size:100;not null;index"` // e.g., 'member.invited', 'settings.updated'
	ResourceType string     `json:"resource_type" gorm:"size:50"`          // e.g., 'product', 'order', 'settings'
	ResourceID   *uuid.UUID `json:"resource_id" gorm:"type:uuid"`
	Details      JSONB      `json:"details" gorm:"type:jsonb;default:'{}'"`
	IPAddress    string     `json:"ip_address" gorm:"size:45"` // IPv6 max length
	UserAgent    string     `json:"user_agent"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
}

// TableName specifies the table name for TenantActivityLog
func (TenantActivityLog) TableName() string {
	return "tenant_activity_log"
}

// MembershipRole constants
const (
	MembershipRoleOwner   = "owner"
	MembershipRoleAdmin   = "admin"
	MembershipRoleManager = "manager"
	MembershipRoleMember  = "member"
	MembershipRoleViewer  = "viewer"
)

// ReservedSlug represents a slug that cannot be used by tenants
// Stored in database for easy management via API without code deployment
type ReservedSlug struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Slug      string    `json:"slug" gorm:"unique;not null;size:50"`
	Reason    string    `json:"reason" gorm:"not null;size:255"`
	Category  string    `json:"category" gorm:"not null;size:50;default:'system'"` // system, brand, infrastructure, offensive
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by" gorm:"size:255"` // admin email or 'system'
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for ReservedSlug
func (ReservedSlug) TableName() string {
	return "reserved_slugs"
}

// ReservedSlugCategory constants
const (
	ReservedSlugCategorySystem         = "system"         // System routes (admin, api, login)
	ReservedSlugCategoryBrand          = "brand"          // Brand protection (tesserix)
	ReservedSlugCategoryInfrastructure = "infrastructure" // Infrastructure (www, cdn, mail)
	ReservedSlugCategoryOffensive      = "offensive"      // Offensive/inappropriate content
)

// TenantSlugReservation tracks claimed/held slugs for quick availability lookup
// - Pending: Temporarily held during onboarding (expires after 30 mins)
// - Active: Permanently claimed by a tenant
// - Released: Was abandoned or tenant was deleted
type TenantSlugReservation struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Slug       string     `json:"slug" gorm:"unique;not null;size:50"`
	Status     string     `json:"status" gorm:"not null;size:20;default:'pending'"` // pending, active, released
	SessionID  *uuid.UUID `json:"session_id" gorm:"type:uuid;index"`                // Onboarding session
	TenantID   *uuid.UUID `json:"tenant_id" gorm:"type:uuid;index"`                 // Tenant (when active)
	ReservedBy string     `json:"reserved_by" gorm:"size:255"`                      // User email or session ID
	ExpiresAt  *time.Time `json:"expires_at" gorm:"index"`                          // NULL for active (permanent)
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ReleasedAt *time.Time `json:"released_at"` // When abandoned/released
}

// TableName returns the table name for TenantSlugReservation
func (TenantSlugReservation) TableName() string {
	return "tenant_slug_reservations"
}

// SlugReservationStatus constants
const (
	SlugReservationPending  = "pending"  // Temporarily held during onboarding
	SlugReservationActive   = "active"   // Permanently claimed by tenant
	SlugReservationReleased = "released" // Was released/abandoned
)

// DefaultSlugReservationDuration is how long a pending reservation is held
const DefaultSlugReservationDuration = 30 * time.Minute

// BusinessModel constants
// Defines whether the tenant operates as a single-vendor store or multi-vendor marketplace
const (
	BusinessModelOnlineStore = "ONLINE_STORE" // Single vendor, direct-to-consumer (D2C)
	BusinessModelMarketplace = "MARKETPLACE"  // Multi-vendor platform with commissions
)

// PricingTier constants
// All tenants default to 'free' until monetization is enabled
const (
	PricingTierFree         = "free"         // Free tier - default for all new tenants
	PricingTierStarter      = "starter"      // Starter tier - future paid plan
	PricingTierProfessional = "professional" // Professional tier - future paid plan
	PricingTierEnterprise   = "enterprise"   // Enterprise tier - future paid plan
)

// PricingTierConfig holds configuration for each pricing tier
// This will be used when monetization is enabled
type PricingTierConfig struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	MonthlyPrice float64  `json:"monthly_price"` // In USD, 0 for free
	YearlyPrice  float64  `json:"yearly_price"`  // In USD, 0 for free
	Features     []string `json:"features"`
	MaxProducts  int      `json:"max_products"`   // -1 for unlimited
	MaxUsers     int      `json:"max_users"`      // -1 for unlimited
	MaxStorageMB int      `json:"max_storage_mb"` // -1 for unlimited
	IsEnabled    bool     `json:"is_enabled"`     // Whether this tier can be selected
}

// GetPricingTiers returns all available pricing tiers with their configurations
// For now, only 'free' is enabled; others are disabled until monetization
func GetPricingTiers() map[string]PricingTierConfig {
	return map[string]PricingTierConfig{
		PricingTierFree: {
			Name:         PricingTierFree,
			DisplayName:  "Free",
			Description:  "Perfect for getting started",
			MonthlyPrice: 0,
			YearlyPrice:  0,
			Features: []string{
				"Up to 100 products",
				"Basic analytics",
				"Email support",
				"Standard themes",
			},
			MaxProducts:  100,
			MaxUsers:     2,
			MaxStorageMB: 500,
			IsEnabled:    true, // Only free tier is enabled for now
		},
		PricingTierStarter: {
			Name:         PricingTierStarter,
			DisplayName:  "Starter",
			Description:  "For growing businesses",
			MonthlyPrice: 29,
			YearlyPrice:  290,
			Features: []string{
				"Up to 1,000 products",
				"Advanced analytics",
				"Priority email support",
				"Custom themes",
				"API access",
			},
			MaxProducts:  1000,
			MaxUsers:     5,
			MaxStorageMB: 2048,
			IsEnabled:    false, // Disabled until monetization
		},
		PricingTierProfessional: {
			Name:         PricingTierProfessional,
			DisplayName:  "Professional",
			Description:  "For scaling businesses",
			MonthlyPrice: 79,
			YearlyPrice:  790,
			Features: []string{
				"Up to 10,000 products",
				"Full analytics suite",
				"24/7 chat support",
				"White-label branding",
				"Advanced API access",
				"Custom integrations",
			},
			MaxProducts:  10000,
			MaxUsers:     15,
			MaxStorageMB: 10240,
			IsEnabled:    false, // Disabled until monetization
		},
		PricingTierEnterprise: {
			Name:         PricingTierEnterprise,
			DisplayName:  "Enterprise",
			Description:  "For large organizations",
			MonthlyPrice: 199,
			YearlyPrice:  1990,
			Features: []string{
				"Unlimited products",
				"Enterprise analytics",
				"Dedicated support",
				"Custom development",
				"SLA guarantee",
				"On-premise option",
			},
			MaxProducts:  -1,    // Unlimited
			MaxUsers:     -1,    // Unlimited
			MaxStorageMB: -1,    // Unlimited
			IsEnabled:    false, // Disabled until monetization
		},
	}
}

// TenantActivityAction constants
const (
	ActivityMemberInvited     = "member.invited"
	ActivityMemberAccepted    = "member.accepted"
	ActivityMemberRemoved     = "member.removed"
	ActivityMemberRoleChanged = "member.role_changed"
	ActivitySettingsUpdated   = "settings.updated"
	ActivityTenantCreated     = "tenant.created"
	ActivityTenantUpdated     = "tenant.updated"
)

// ============================================================================
// MULTI-TENANT CREDENTIAL ISOLATION MODELS
// ============================================================================
// These models enable the same email to have different passwords for different tenants.
// This is an enterprise requirement for full credential isolation per tenant.

// TenantCredential stores tenant-specific credentials for multi-tenant credential isolation
// Each user can have different passwords per tenant
type TenantCredential struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID   uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	TenantID uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`

	// Tenant-specific password (bcrypt hash)
	PasswordHash string `json:"-" gorm:"not null"`

	// Password metadata
	PasswordSetAt            time.Time  `json:"password_set_at" gorm:"default:now()"`
	PasswordExpiresAt        *time.Time `json:"password_expires_at"`
	PasswordRotationRequired bool       `json:"password_rotation_required" gorm:"default:false"`
	LastPasswordChangeAt     *time.Time `json:"last_password_change_at"`

	// MFA settings per tenant
	MFAEnabled     bool       `json:"mfa_enabled" gorm:"default:false"`
	MFAType        string     `json:"mfa_type" gorm:"size:20"` // totp, sms, email, hardware_token
	MFASecret      string     `json:"-" gorm:"size:255"`       // Encrypted TOTP secret
	MFABackupCodes JSONB      `json:"-" gorm:"type:jsonb;default:'[]'"`
	MFALastUsedAt  *time.Time `json:"mfa_last_used_at"`

	// Login security
	LoginAttempts         int        `json:"login_attempts" gorm:"default:0"`
	LastLoginAttemptAt    *time.Time `json:"last_login_attempt_at"`
	LockedUntil           *time.Time `json:"locked_until"`
	LastSuccessfulLoginAt *time.Time `json:"last_successful_login_at"`
	LastLoginIP           string     `json:"last_login_ip" gorm:"size:45"` // IPv6 max length
	LastLoginUserAgent    string     `json:"last_login_user_agent"`

	// Progressive lockout fields
	// Tracks lockout escalation across multiple lockout cycles
	LockoutCount        int        `json:"lockout_count" gorm:"default:0"`         // Number of times account has been locked
	CurrentTier         int        `json:"current_tier" gorm:"default:0"`          // Current lockout tier (1-4, 0=not locked)
	PermanentlyLocked   bool       `json:"permanently_locked" gorm:"default:false"` // True if permanently locked (tier 4)
	PermanentLockedAt   *time.Time `json:"permanent_locked_at"`                    // When permanently locked
	UnlockedBy          *uuid.UUID `json:"unlocked_by" gorm:"type:uuid"`           // Admin who unlocked
	UnlockedAt          *time.Time `json:"unlocked_at"`                            // When admin unlocked
	TotalFailedAttempts int        `json:"total_failed_attempts" gorm:"default:0"` // Cumulative failed attempts (for tier calculation)

	// Session management
	ActiveSessions int `json:"active_sessions" gorm:"default:0"`
	MaxSessions    int `json:"max_sessions" gorm:"default:5"`

	// Password history (to prevent reuse)
	PasswordHistory JSONB `json:"-" gorm:"type:jsonb;default:'[]'"`

	// Audit fields
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedBy *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	UpdatedBy *uuid.UUID `json:"updated_by" gorm:"type:uuid"`

	// Relationships
	Tenant *Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}

// TableName specifies the table name for TenantCredential
func (TenantCredential) TableName() string {
	return "tenant_credentials"
}

func (tc *TenantCredential) BeforeCreate(tx *gorm.DB) error {
	if tc.ID == uuid.Nil {
		tc.ID = uuid.New()
	}
	tc.PasswordSetAt = time.Now()
	return nil
}

// TenantAuthPolicy stores per-tenant authentication and security policies
// Each tenant can define their own authentication/security policies
type TenantAuthPolicy struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;unique;index"`

	// Password policy
	PasswordMinLength           int    `json:"password_min_length" gorm:"default:8"`
	PasswordMaxLength           int    `json:"password_max_length" gorm:"default:128"`
	PasswordRequireUppercase    bool   `json:"password_require_uppercase" gorm:"default:true"`
	PasswordRequireLowercase    bool   `json:"password_require_lowercase" gorm:"default:true"`
	PasswordRequireNumbers      bool   `json:"password_require_numbers" gorm:"default:true"`
	PasswordRequireSpecialChars bool   `json:"password_require_special_chars" gorm:"default:false"`
	PasswordSpecialChars        string `json:"password_special_chars" gorm:"size:100"`
	PasswordExpiryDays          *int   `json:"password_expiry_days"`                   // NULL = no expiry
	PasswordHistoryCount        int    `json:"password_history_count" gorm:"default:5"` // Prevent reuse of last N passwords

	// Login policy
	MaxLoginAttempts       int `json:"max_login_attempts" gorm:"default:5"`
	LockoutDurationMinutes int `json:"lockout_duration_minutes" gorm:"default:30"`
	SessionTimeoutMinutes  int `json:"session_timeout_minutes" gorm:"default:480"` // 8 hours default
	MaxConcurrentSessions  int `json:"max_concurrent_sessions" gorm:"default:5"`

	// Progressive lockout policy
	// Strict 2-tier lockout: first lockout is temporary, second is permanent
	// Tier 1 (5 attempts): 30 minutes lockout
	// Tier 2 (7 attempts): Permanent lockout - requires admin unlock or password reset
	EnableProgressiveLockout  bool `json:"enable_progressive_lockout" gorm:"default:true"`
	Tier1LockoutMinutes       int  `json:"tier1_lockout_minutes" gorm:"default:30"`  // 5 failed attempts = 30 min lock
	PermanentLockoutThreshold int  `json:"permanent_lockout_threshold" gorm:"default:7"` // 7 failed attempts = permanent lock
	LockoutResetHours         int  `json:"lockout_reset_hours" gorm:"default:24"` // Reset tier after N hours of no failures

	// MFA policy
	MFARequired        bool  `json:"mfa_required" gorm:"default:false"`
	MFARequiredForRoles JSONB `json:"mfa_required_for_roles" gorm:"type:jsonb;default:'[\"owner\", \"admin\"]'"`
	MFAAllowedTypes    JSONB `json:"mfa_allowed_types" gorm:"type:jsonb;default:'[\"totp\", \"email\"]'"`

	// IP and device restrictions
	IPWhitelistEnabled        bool  `json:"ip_whitelist_enabled" gorm:"default:false"`
	IPWhitelist               JSONB `json:"ip_whitelist" gorm:"type:jsonb;default:'[]'"`
	TrustedDevicesEnabled     bool  `json:"trusted_devices_enabled" gorm:"default:false"`
	RequireDeviceVerification bool  `json:"require_device_verification" gorm:"default:false"`

	// SSO/Federation settings (for enterprise)
	SSOEnabled  bool   `json:"sso_enabled" gorm:"default:false"`
	SSOProvider string `json:"sso_provider" gorm:"size:50"` // okta, azure_ad, google, custom_saml
	SSOConfig   JSONB  `json:"sso_config" gorm:"type:jsonb;default:'{}'"`
	SSORequired bool   `json:"sso_required" gorm:"default:false"` // If true, password login is disabled

	// Advanced security
	RequireEmailVerification   bool `json:"require_email_verification" gorm:"default:true"`
	AllowPasswordReset         bool `json:"allow_password_reset" gorm:"default:true"`
	PasswordResetTokenExpiryHours int `json:"password_reset_token_expiry_hours" gorm:"default:24"`
	NotifyOnNewDeviceLogin     bool `json:"notify_on_new_device_login" gorm:"default:true"`
	NotifyOnPasswordChange     bool `json:"notify_on_password_change" gorm:"default:true"`

	// Audit fields
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *uuid.UUID `json:"updated_by" gorm:"type:uuid"`

	// Relationships
	Tenant *Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}

// TableName specifies the table name for TenantAuthPolicy
func (TenantAuthPolicy) TableName() string {
	return "tenant_auth_policies"
}

func (tap *TenantAuthPolicy) BeforeCreate(tx *gorm.DB) error {
	if tap.ID == uuid.Nil {
		tap.ID = uuid.New()
	}
	// Set default special chars (can't use GORM default due to special char escaping issues)
	if tap.PasswordSpecialChars == "" {
		tap.PasswordSpecialChars = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	}
	return nil
}

// TenantAuthAuditLog stores comprehensive audit trail of all authentication events per tenant
type TenantAuthAuditLog struct {
	ID       uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID   *uuid.UUID `json:"user_id" gorm:"type:uuid;index"` // NULL for pre-auth events

	// Event details
	EventType   string `json:"event_type" gorm:"size:50;not null;index"` // login_success, login_failed, password_changed, etc.
	EventStatus string `json:"event_status" gorm:"size:20;not null;default:'success'"`

	// Request context
	IPAddress         string `json:"ip_address" gorm:"size:45"`
	UserAgent         string `json:"user_agent"`
	DeviceFingerprint string `json:"device_fingerprint" gorm:"size:255"`
	GeoLocation       JSONB  `json:"geo_location" gorm:"type:jsonb"` // {country, city, region}

	// Event-specific details
	Details      JSONB  `json:"details" gorm:"type:jsonb;default:'{}'"`
	ErrorMessage string `json:"error_message"`

	// Session info (for login events)
	SessionID string `json:"session_id" gorm:"size:255"`

	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName specifies the table name for TenantAuthAuditLog
func (TenantAuthAuditLog) TableName() string {
	return "tenant_auth_audit_log"
}

func (taal *TenantAuthAuditLog) BeforeCreate(tx *gorm.DB) error {
	if taal.ID == uuid.Nil {
		taal.ID = uuid.New()
	}
	return nil
}

// AuthEventType constants for tenant auth audit logging
const (
	AuthEventLoginSuccess       = "login_success"
	AuthEventLoginFailed        = "login_failed"
	AuthEventPasswordChanged    = "password_changed"
	AuthEventPasswordReset      = "password_reset"
	AuthEventMFAEnabled         = "mfa_enabled"
	AuthEventMFADisabled        = "mfa_disabled"
	AuthEventMFAVerified        = "mfa_verified"
	AuthEventMFAFailed          = "mfa_failed"
	AuthEventAccountLocked      = "account_locked"
	AuthEventAccountUnlocked    = "account_unlocked"
	AuthEventSessionCreated     = "session_created"
	AuthEventSessionRevoked     = "session_revoked"
	AuthEventNewDeviceLogin     = "new_device_login"
	AuthEventSuspiciousActivity = "suspicious_activity"
)

// AuthEventStatus constants
const (
	AuthEventStatusSuccess = "success"
	AuthEventStatusFailed  = "failed"
	AuthEventStatusBlocked = "blocked"
)

// MFAType constants
const (
	MFATypeTOTP          = "totp"
	MFATypeSMS           = "sms"
	MFATypeEmail         = "email"
	MFATypeHardwareToken = "hardware_token"
)

func (t *Tenant) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (m *UserTenantMembership) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (l *TenantActivityLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// DeletedTenant represents an archived tenant record for audit purposes
// This table stores a snapshot of the tenant data before deletion
type DeletedTenant struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OriginalTenantID uuid.UUID `json:"original_tenant_id" gorm:"type:uuid;not null;index"`
	Slug             string    `json:"slug" gorm:"size:100;not null;index"`
	BusinessName     string    `json:"business_name" gorm:"size:255;not null"`
	OwnerUserID      uuid.UUID `json:"owner_user_id" gorm:"type:uuid;not null"`
	OwnerEmail       string    `json:"owner_email" gorm:"size:255;not null;index"`

	// Archived tenant data (JSON snapshot)
	TenantData      JSONB `json:"tenant_data" gorm:"type:jsonb;not null"`
	MembershipsData JSONB `json:"memberships_data" gorm:"type:jsonb;default:'[]'"`
	VendorsData     JSONB `json:"vendors_data" gorm:"type:jsonb;default:'[]'"`
	StorefrontsData JSONB `json:"storefronts_data" gorm:"type:jsonb;default:'[]'"`

	// Deletion metadata
	DeletedByUserID uuid.UUID `json:"deleted_by_user_id" gorm:"type:uuid;not null;index"`
	DeletionReason  string    `json:"deletion_reason" gorm:"type:text"`
	DeletedAt       time.Time `json:"deleted_at" gorm:"index"`

	// Track what resources were cleaned up
	ResourcesCleaned    JSONB      `json:"resources_cleaned" gorm:"type:jsonb;default:'{}'"`
	CleanupCompletedAt  *time.Time `json:"cleanup_completed_at"`
}

// TableName specifies the table name for DeletedTenant
func (DeletedTenant) TableName() string {
	return "deleted_tenants"
}

func (d *DeletedTenant) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.DeletedAt.IsZero() {
		d.DeletedAt = time.Now()
	}
	return nil
}

// Customer account deactivation constants
const (
	// DataRetentionDays is the number of days to retain deactivated account data before permanent deletion
	DataRetentionDays = 90

	// Deactivation reason constants
	DeactivationReasonSelfService = "self_service"
	DeactivationReasonAdminAction = "admin_action"
	DeactivationReasonInactivity  = "inactivity"
)

// DeactivatedMembership archives customer membership data when they self-deactivate from a storefront.
// This enables 90-day data retention before permanent deletion and supports reactivation within that window.
// Following the same pattern as DeletedTenant for audit/archive purposes.
type DeactivatedMembership struct {
	ID                   uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	OriginalMembershipID uuid.UUID `json:"original_membership_id" gorm:"type:uuid;not null;index"`
	UserID               uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	TenantID             uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Email                string    `json:"email" gorm:"size:255;not null;index"`
	FirstName            string    `json:"first_name" gorm:"size:100"`
	LastName             string    `json:"last_name" gorm:"size:100"`

	// Archived membership data (JSON snapshot for audit)
	MembershipData JSONB `json:"membership_data" gorm:"type:jsonb;default:'{}'"`

	// Deactivation metadata
	DeactivationReason string    `json:"deactivation_reason" gorm:"type:text"`
	DeactivatedAt      time.Time `json:"deactivated_at" gorm:"not null;index"`
	ScheduledPurgeAt   time.Time `json:"scheduled_purge_at" gorm:"not null;index"` // DeactivatedAt + 90 days

	// Reactivation tracking
	ReactivatedAt     *time.Time `json:"reactivated_at"`
	ReactivationCount int        `json:"reactivation_count" gorm:"default:0"`

	// Purge tracking
	IsPurged bool       `json:"is_purged" gorm:"default:false;index"`
	PurgedAt *time.Time `json:"purged_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for DeactivatedMembership
func (DeactivatedMembership) TableName() string {
	return "deactivated_memberships"
}

func (d *DeactivatedMembership) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.DeactivatedAt.IsZero() {
		d.DeactivatedAt = time.Now()
	}
	if d.ScheduledPurgeAt.IsZero() {
		d.ScheduledPurgeAt = d.DeactivatedAt.AddDate(0, 0, DataRetentionDays)
	}
	return nil
}

// DaysUntilPurge returns the number of days remaining until the account is permanently deleted
func (d *DeactivatedMembership) DaysUntilPurge() int {
	if d.IsPurged {
		return 0
	}
	remaining := time.Until(d.ScheduledPurgeAt).Hours() / 24
	if remaining < 0 {
		return 0
	}
	return int(remaining)
}

// CanReactivate returns true if the account can still be reactivated
func (d *DeactivatedMembership) CanReactivate() bool {
	return !d.IsPurged && d.ReactivatedAt == nil && time.Now().Before(d.ScheduledPurgeAt)
}

// ============================================================================
// PASSWORD RESET TOKEN MODEL
// ============================================================================
// Secure password reset tokens for self-service password recovery.
// Tokens are single-use, time-limited, and tenant-aware.

// DefaultPasswordResetTokenExpiry is the default token expiry (1 hour)
const DefaultPasswordResetTokenExpiry = 1 * time.Hour

// PasswordResetToken stores secure tokens for password reset requests
type PasswordResetToken struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Token     string    `json:"-" gorm:"type:varchar(255);not null;uniqueIndex"` // Hashed token
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Email     string    `json:"email" gorm:"size:255;not null;index"`

	// Token lifecycle
	ExpiresAt   time.Time  `json:"expires_at" gorm:"not null;index"`
	UsedAt      *time.Time `json:"used_at"`
	IsUsed      bool       `json:"is_used" gorm:"default:false;index"`

	// Security tracking
	RequestedIP    string `json:"requested_ip" gorm:"size:45"`
	RequestedAgent string `json:"requested_agent"`
	UsedIP         string `json:"used_ip" gorm:"size:45"`
	UsedAgent      string `json:"used_agent"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for PasswordResetToken
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

func (p *PasswordResetToken) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = time.Now().Add(DefaultPasswordResetTokenExpiry)
	}
	return nil
}

// IsValid returns true if the token is still valid (not used and not expired)
func (p *PasswordResetToken) IsValid() bool {
	return !p.IsUsed && time.Now().Before(p.ExpiresAt)
}
