package models

import (
	"time"

	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Plan DTOs

type CreatePlanRequest struct {
	Name              string `json:"name" binding:"required"`
	DisplayName       string `json:"displayName" binding:"required"`
	Description       string `json:"description"`
	MonthlyPriceCents int    `json:"monthlyPriceCents"`
	YearlyPriceCents  int    `json:"yearlyPriceCents"`
	Currency          string `json:"currency"`
	MaxProducts       int    `json:"maxProducts"`
	MaxUsers          int    `json:"maxUsers"`
	MaxStorageMB      int    `json:"maxStorageMb"`
	Features          JSONB  `json:"features"`
	SortOrder         int    `json:"sortOrder"`
	IsActive          bool   `json:"isActive"`
	IsFree            bool   `json:"isFree"`
	TrialDays         int    `json:"trialDays"`
}

type UpdatePlanRequest struct {
	DisplayName       *string `json:"displayName"`
	Description       *string `json:"description"`
	MonthlyPriceCents *int    `json:"monthlyPriceCents"`
	YearlyPriceCents  *int    `json:"yearlyPriceCents"`
	MaxProducts       *int    `json:"maxProducts"`
	MaxUsers          *int    `json:"maxUsers"`
	MaxStorageMB      *int    `json:"maxStorageMb"`
	Features          *JSONB  `json:"features"`
	SortOrder         *int    `json:"sortOrder"`
	IsActive          *bool   `json:"isActive"`
	TrialDays         *int    `json:"trialDays"`
}

// Subscription DTOs

type CheckoutRequest struct {
	TenantID        string          `json:"tenantId" binding:"required"`
	PlanID          uuid.UUID       `json:"planId" binding:"required"`
	BillingInterval BillingInterval `json:"billingInterval" binding:"required"`
	BillingEmail    string          `json:"billingEmail" binding:"required"`
	SuccessURL      string          `json:"successUrl" binding:"required"`
	CancelURL       string          `json:"cancelUrl" binding:"required"`
}

type CheckoutResponse struct {
	SessionID   string `json:"sessionId"`
	CheckoutURL string `json:"checkoutUrl"`
}

type PortalRequest struct {
	TenantID  string `json:"tenantId" binding:"required"`
	ReturnURL string `json:"returnUrl" binding:"required"`
}

type PortalResponse struct {
	PortalURL string `json:"portalUrl"`
}

type ChangePlanRequest struct {
	PlanID          uuid.UUID       `json:"planId" binding:"required"`
	BillingInterval BillingInterval `json:"billingInterval"`
}

type SubscriptionStats struct {
	MRR           int `json:"mrr"`
	ActiveCount   int `json:"activeCount"`
	TrialingCount int `json:"trialingCount"`
	PastDueCount  int `json:"pastDueCount"`
	CanceledCount int `json:"canceledCount"`
	TotalRevenue  int `json:"totalRevenue"`
}

// SubscriptionStatusResponse — computed subscription status for a tenant
type SubscriptionStatusResponse struct {
	TenantID        string `json:"tenantId"`
	Status          string `json:"status"`
	PlanName        string `json:"planName"`
	PlanDisplayName string `json:"planDisplayName"`
	Currency        string `json:"currency"`
	Amount          int64  `json:"amount"`

	IsTrialing    bool       `json:"isTrialing"`
	TrialDaysLeft int        `json:"trialDaysLeft"`
	TrialEndsAt   *time.Time `json:"trialEndsAt,omitempty"`

	IsInGracePeriod bool       `json:"isInGracePeriod"`
	GraceDaysLeft   int        `json:"graceDaysLeft"`
	GraceEndsAt     *time.Time `json:"graceEndsAt,omitempty"`

	CanRead        bool `json:"canRead"`
	CanWrite       bool `json:"canWrite"`
	CanAccessAdmin bool `json:"canAccessAdmin"`

	WarningLevel   string `json:"warningLevel"`
	WarningMessage string `json:"warningMessage"`
	WarningAction  string `json:"warningAction"`

	HasPaymentMethod bool `json:"hasPaymentMethod"`
	MaxProducts      int  `json:"maxProducts"`
	MaxUsers         int  `json:"maxUsers"`
	MaxStorageMB     int  `json:"maxStorageMb"`
}

// SetupPaymentRequest for creating a setup checkout session
type SetupPaymentRequest struct {
	SuccessURL string `json:"successUrl" binding:"required"`
	CancelURL  string `json:"cancelUrl" binding:"required"`
}

// ExtendTrialRequest for platform admin to extend a tenant's trial
type ExtendTrialRequest struct {
	AdditionalDays int    `json:"additionalDays" binding:"required,min=1,max=365"`
	Reason         string `json:"reason" binding:"required"`
}

// EnhancedStats extends SubscriptionStats with trial analytics
type EnhancedStats struct {
	SubscriptionStats
	TrialConversionRate float64 `json:"trialConversionRate"`
	ExpiringTrials7d    int     `json:"expiringTrials7d"`
	ExpiringTrials30d   int     `json:"expiringTrials30d"`
	ExpiredCount        int     `json:"expiredCount"`
	SuspendedCount      int     `json:"suspendedCount"`
}

// Stripe Settings DTOs

type StripeSettingsResponse struct {
	SecretKeyConfigured     bool   `json:"secretKeyConfigured"`
	SecretKeyHint           string `json:"secretKeyHint"`
	WebhookSecretConfigured bool   `json:"webhookSecretConfigured"`
	WebhookSecretHint       string `json:"webhookSecretHint"`
	KeySource               string `json:"keySource"`
}

type UpdateStripeKeysRequest struct {
	SecretKey     *string `json:"secretKey"`
	WebhookSecret *string `json:"webhookSecret"`
}

type VerifyStripeKeyRequest struct {
	SecretKey string `json:"secretKey" binding:"required"`
}

type VerifyStripeKeyResponse struct {
	Valid       bool   `json:"valid"`
	AccountID   string `json:"accountId,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Livemode    bool   `json:"livemode"`
	Error       string `json:"error,omitempty"`
}

// Admin Invoices (cross-tenant)

type AdminInvoicesResponse struct {
	Invoices []SubscriptionInvoice `json:"invoices"`
	Total    int64                 `json:"total"`
	Limit    int                   `json:"limit"`
	Offset   int                   `json:"offset"`
}
