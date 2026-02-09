package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONB type for PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONB")
	}
	return json.Unmarshal(bytes, j)
}

// SubscriptionStatus enum
type SubscriptionStatus string

const (
	StatusActive   SubscriptionStatus = "active"
	StatusTrialing SubscriptionStatus = "trialing"
	StatusPastDue  SubscriptionStatus = "past_due"
	StatusCanceled SubscriptionStatus = "canceled"
	StatusUnpaid   SubscriptionStatus = "unpaid"
)

// InvoiceStatus enum
type InvoiceStatus string

const (
	InvoiceDraft InvoiceStatus = "draft"
	InvoiceOpen  InvoiceStatus = "open"
	InvoicePaid  InvoiceStatus = "paid"
	InvoiceVoid  InvoiceStatus = "void"
)

// BillingInterval enum
type BillingInterval string

const (
	IntervalMonthly BillingInterval = "monthly"
	IntervalYearly  BillingInterval = "yearly"
)

// SubscriptionPlan maps to Stripe Products/Prices
type SubscriptionPlan struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name                 string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"name"`
	DisplayName          string    `gorm:"type:varchar(100);not null" json:"displayName"`
	Description          string    `gorm:"type:text" json:"description"`
	MonthlyPriceCents    int       `gorm:"not null;default:0" json:"monthlyPriceCents"`
	YearlyPriceCents     int       `gorm:"not null;default:0" json:"yearlyPriceCents"`
	Currency             string    `gorm:"type:varchar(3);not null;default:'usd'" json:"currency"`
	StripeProductID      string    `gorm:"type:varchar(255)" json:"stripeProductId,omitempty"`
	StripeMonthlyPriceID string    `gorm:"type:varchar(255)" json:"stripeMonthlyPriceId,omitempty"`
	StripeYearlyPriceID  string    `gorm:"type:varchar(255)" json:"stripeYearlyPriceId,omitempty"`
	MaxProducts          int       `gorm:"not null;default:100" json:"maxProducts"`
	MaxUsers             int       `gorm:"not null;default:2" json:"maxUsers"`
	MaxStorageMB         int       `gorm:"not null;default:500" json:"maxStorageMb"`
	Features             JSONB     `gorm:"type:jsonb" json:"features,omitempty"`
	SortOrder            int       `gorm:"not null;default:0" json:"sortOrder"`
	IsActive             bool      `gorm:"not null;default:true" json:"isActive"`
	IsFree               bool      `gorm:"not null;default:false" json:"isFree"`
	TrialDays            int       `gorm:"not null;default:0" json:"trialDays"`
	CreatedAt            time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt            time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (SubscriptionPlan) TableName() string {
	return "subscription_plans"
}

// TenantSubscription — one per tenant
type TenantSubscription struct {
	ID                   uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string             `gorm:"type:varchar(255);uniqueIndex;not null" json:"tenantId"`
	PlanID               uuid.UUID          `gorm:"type:uuid;not null" json:"planId"`
	StripeCustomerID     string             `gorm:"type:varchar(255)" json:"stripeCustomerId,omitempty"`
	StripeSubscriptionID string             `gorm:"type:varchar(255)" json:"stripeSubscriptionId,omitempty"`
	Status               SubscriptionStatus `gorm:"type:varchar(50);not null;default:'active';index" json:"status"`
	BillingInterval      BillingInterval    `gorm:"type:varchar(20);not null;default:'monthly'" json:"billingInterval"`
	CurrentPeriodStart   *time.Time         `json:"currentPeriodStart,omitempty"`
	CurrentPeriodEnd     *time.Time         `json:"currentPeriodEnd,omitempty"`
	TrialStart           *time.Time         `json:"trialStart,omitempty"`
	TrialEnd             *time.Time         `json:"trialEnd,omitempty"`
	CancelAtPeriodEnd    bool               `gorm:"not null;default:false" json:"cancelAtPeriodEnd"`
	CanceledAt           *time.Time         `json:"canceledAt,omitempty"`
	BillingEmail         string             `gorm:"type:varchar(255)" json:"billingEmail,omitempty"`
	CreatedAt            time.Time          `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt            time.Time          `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	Plan SubscriptionPlan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}

func (TenantSubscription) TableName() string {
	return "tenant_subscriptions"
}

// SubscriptionInvoice cached from Stripe
type SubscriptionInvoice struct {
	ID               uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID         string        `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	SubscriptionID   uuid.UUID     `gorm:"type:uuid" json:"subscriptionId"`
	StripeInvoiceID  string        `gorm:"type:varchar(255);uniqueIndex" json:"stripeInvoiceId"`
	StripeHostedURL  string        `gorm:"type:text" json:"stripeHostedUrl,omitempty"`
	StripeInvoicePDF string        `gorm:"type:text" json:"stripeInvoicePdf,omitempty"`
	Status           InvoiceStatus `gorm:"type:varchar(50);not null;default:'draft'" json:"status"`
	AmountDueCents   int           `gorm:"not null;default:0" json:"amountDueCents"`
	AmountPaidCents  int           `gorm:"not null;default:0" json:"amountPaidCents"`
	Currency         string        `gorm:"type:varchar(3);not null;default:'usd'" json:"currency"`
	PeriodStart      *time.Time    `json:"periodStart,omitempty"`
	PeriodEnd        *time.Time    `json:"periodEnd,omitempty"`
	PaidAt           *time.Time    `json:"paidAt,omitempty"`
	Description      string        `gorm:"type:text" json:"description,omitempty"`
	CreatedAt        time.Time     `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (SubscriptionInvoice) TableName() string {
	return "subscription_invoices"
}

// SubscriptionEvent — webhook audit log
type SubscriptionEvent struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	StripeEventID   string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"stripeEventId"`
	EventType       string     `gorm:"type:varchar(100);not null" json:"eventType"`
	TenantID        string     `gorm:"type:varchar(255)" json:"tenantId,omitempty"`
	Payload         JSONB      `gorm:"type:jsonb" json:"payload,omitempty"`
	Processed       bool       `gorm:"not null;default:false" json:"processed"`
	ProcessedAt     *time.Time `json:"processedAt,omitempty"`
	ProcessingError string     `gorm:"type:text" json:"processingError,omitempty"`
	RetryCount      int        `gorm:"not null;default:0" json:"retryCount"`
	CreatedAt       time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (SubscriptionEvent) TableName() string {
	return "subscription_events"
}
