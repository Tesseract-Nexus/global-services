package models

import "github.com/google/uuid"

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
