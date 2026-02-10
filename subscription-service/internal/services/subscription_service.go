package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stripe/stripe-go/v76"
	billingportalsession "github.com/stripe/stripe-go/v76/billingportal/session"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"subscription-service/internal/clients"
	"subscription-service/internal/models"
	"subscription-service/internal/repository"
)

type SubscriptionService struct {
	repo         *repository.SubscriptionRepository
	tenantClient *clients.TenantClient
}

func NewSubscriptionService(repo *repository.SubscriptionRepository, tenantClient *clients.TenantClient) *SubscriptionService {
	return &SubscriptionService{repo: repo, tenantClient: tenantClient}
}

func (s *SubscriptionService) GetSubscription(ctx context.Context, tenantID string) (*models.TenantSubscription, error) {
	return s.repo.GetSubscription(ctx, tenantID)
}

func (s *SubscriptionService) CreateCheckoutSession(ctx context.Context, req models.CheckoutRequest) (*models.CheckoutResponse, error) {
	plan, err := s.repo.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	if plan.IsFree {
		return nil, fmt.Errorf("cannot create checkout for free plan")
	}

	stripeCustomerID, err := s.getOrCreateStripeCustomer(ctx, req.TenantID, req.BillingEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create Stripe customer: %w", err)
	}

	var priceID string
	switch req.BillingInterval {
	case models.IntervalYearly:
		priceID = plan.StripeYearlyPriceID
	default:
		priceID = plan.StripeMonthlyPriceID
	}

	if priceID == "" {
		return nil, fmt.Errorf("no Stripe price configured for plan %s interval %s", plan.Name, req.BillingInterval)
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeCustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
		Metadata: map[string]string{
			"tenant_id": req.TenantID,
			"plan_id":   req.PlanID.String(),
		},
	}

	if plan.TrialDays > 0 {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(int64(plan.TrialDays)),
		}
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	return &models.CheckoutResponse{
		SessionID:   sess.ID,
		CheckoutURL: sess.URL,
	}, nil
}

func (s *SubscriptionService) CreatePortalSession(ctx context.Context, req models.PortalRequest) (*models.PortalResponse, error) {
	sub, err := s.repo.GetSubscription(ctx, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.StripeCustomerID == "" {
		return nil, fmt.Errorf("no Stripe customer for tenant")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID),
		ReturnURL: stripe.String(req.ReturnURL),
	}

	sess, err := billingportalsession.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create portal session: %w", err)
	}

	return &models.PortalResponse{
		PortalURL: sess.URL,
	}, nil
}

func (s *SubscriptionService) CancelSubscription(ctx context.Context, tenantID string) error {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("subscription not found: %w", err)
	}

	sub.CancelAtPeriodEnd = true
	now := time.Now()
	sub.CanceledAt = &now

	return s.repo.UpdateSubscription(ctx, sub)
}

func (s *SubscriptionService) ReactivateSubscription(ctx context.Context, tenantID string) error {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("subscription not found: %w", err)
	}

	sub.CancelAtPeriodEnd = false
	sub.CanceledAt = nil

	return s.repo.UpdateSubscription(ctx, sub)
}

func (s *SubscriptionService) ChangePlan(ctx context.Context, tenantID string, req models.ChangePlanRequest) (*models.TenantSubscription, error) {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	plan, err := s.repo.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	sub.PlanID = req.PlanID
	if req.BillingInterval != "" {
		sub.BillingInterval = req.BillingInterval
	}
	sub.UpdatedAt = time.Now()

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	// Sync to tenant-service
	go func() {
		if err := s.tenantClient.UpdatePricingTier(context.Background(), tenantID, plan.Name); err != nil {
			log.Printf("Failed to sync pricing tier for tenant %s: %v", tenantID, err)
		}
	}()

	return s.repo.GetSubscription(ctx, tenantID)
}

func (s *SubscriptionService) GetInvoices(ctx context.Context, tenantID string) ([]models.SubscriptionInvoice, error) {
	return s.repo.ListInvoices(ctx, tenantID)
}

func (s *SubscriptionService) GetStats(ctx context.Context) (*models.SubscriptionStats, error) {
	return s.repo.GetStats(ctx)
}

func (s *SubscriptionService) getOrCreateStripeCustomer(ctx context.Context, tenantID, email string) (string, error) {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err == nil && sub.StripeCustomerID != "" {
		return sub.StripeCustomerID, nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"tenant_id": tenantID,
		},
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return cust.ID, nil
}
