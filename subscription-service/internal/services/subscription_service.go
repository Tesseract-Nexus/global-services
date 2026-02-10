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
	"github.com/stripe/stripe-go/v76/paymentmethod"
	stripesub "github.com/stripe/stripe-go/v76/subscription"
	"subscription-service/internal/clients"
	"subscription-service/internal/config"
	"subscription-service/internal/models"
	"subscription-service/internal/repository"
)

type SubscriptionService struct {
	repo         *repository.SubscriptionRepository
	tenantClient *clients.TenantClient
	cfg          *config.Config
}

func NewSubscriptionService(repo *repository.SubscriptionRepository, tenantClient *clients.TenantClient, cfg *config.Config) *SubscriptionService {
	return &SubscriptionService{repo: repo, tenantClient: tenantClient, cfg: cfg}
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

func (s *SubscriptionService) GetAllInvoices(ctx context.Context, status string, limit, offset int) (*models.AdminInvoicesResponse, error) {
	invoices, total, err := s.repo.ListAllInvoices(ctx, status, limit, offset)
	if err != nil {
		return nil, err
	}
	return &models.AdminInvoicesResponse{
		Invoices: invoices,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

func (s *SubscriptionService) GetStats(ctx context.Context) (*models.SubscriptionStats, error) {
	return s.repo.GetStats(ctx)
}

// GetSubscriptionStatus computes the full subscription status for a tenant
func (s *SubscriptionService) GetSubscriptionStatus(ctx context.Context, tenantID string) (*models.SubscriptionStatusResponse, error) {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	now := time.Now()
	resp := &models.SubscriptionStatusResponse{
		TenantID:        sub.TenantID,
		Status:          string(sub.Status),
		PlanName:        sub.Plan.Name,
		PlanDisplayName: sub.Plan.DisplayName,
		Currency:        sub.Plan.Currency,
		Amount:          int64(sub.Plan.MonthlyPriceCents),
		MaxProducts:     sub.Plan.MaxProducts,
		MaxUsers:        sub.Plan.MaxUsers,
		MaxStorageMB:    sub.Plan.MaxStorageMB,
		CanRead:         true,
		CanWrite:        true,
		CanAccessAdmin:  true,
		WarningLevel:    "none",
	}

	switch sub.Status {
	case models.StatusTrialing:
		resp.IsTrialing = true
		resp.TrialEndsAt = sub.TrialEnd
		if sub.TrialEnd != nil {
			daysLeft := int(time.Until(*sub.TrialEnd).Hours() / 24)
			if daysLeft < 0 {
				daysLeft = 0
			}
			resp.TrialDaysLeft = daysLeft

			if sub.TrialEnd.Before(now) {
				// Trial has expired but not yet processed by expiration service
				resp.Status = "expired"
				resp.CanWrite = false
				resp.WarningLevel = "blocked"
				resp.WarningMessage = "Your trial has expired. Add a payment method to continue using your store."
				resp.WarningAction = "add_payment"
			} else if daysLeft <= 7 {
				resp.WarningLevel = "warning"
				resp.WarningMessage = fmt.Sprintf("Your trial ends in %d days. Add a payment method to continue uninterrupted.", daysLeft)
				resp.WarningAction = "add_payment"
			} else if daysLeft <= 14 {
				resp.WarningLevel = "info"
				resp.WarningMessage = fmt.Sprintf("Trial: %d days remaining.", daysLeft)
			}
		}

	case models.StatusActive:
		// All good, defaults are fine

	case models.StatusPastDue:
		resp.IsInGracePeriod = sub.GracePeriodEnd != nil && sub.GracePeriodEnd.After(now)
		if resp.IsInGracePeriod {
			graceDaysLeft := int(time.Until(*sub.GracePeriodEnd).Hours() / 24)
			if graceDaysLeft < 0 {
				graceDaysLeft = 0
			}
			resp.GraceDaysLeft = graceDaysLeft
			resp.GraceEndsAt = sub.GracePeriodEnd
			resp.WarningLevel = "critical"
			resp.WarningMessage = fmt.Sprintf("Payment failed. Your store will be suspended in %d days. Update your payment method.", graceDaysLeft)
			resp.WarningAction = "add_payment"
		} else {
			// Grace period expired but not yet processed
			resp.Status = "suspended"
			resp.CanWrite = false
			resp.WarningLevel = "blocked"
			resp.WarningMessage = "Your store has been suspended due to payment failure. Update your payment method to restore access."
			resp.WarningAction = "add_payment"
		}

	case models.StatusExpired, models.StatusSuspended, models.StatusCanceled:
		resp.CanWrite = false
		resp.WarningLevel = "blocked"
		if sub.Status == models.StatusExpired {
			resp.WarningMessage = "Your trial has expired. Add a payment method to continue using your store."
			resp.WarningAction = "add_payment"
		} else if sub.Status == models.StatusSuspended {
			resp.WarningMessage = "Your store has been suspended. Update your payment method to restore access."
			resp.WarningAction = "add_payment"
		} else {
			resp.WarningMessage = "Your subscription has been canceled. Contact support for assistance."
			resp.WarningAction = "contact_support"
		}
	}

	// Check for payment method
	resp.HasPaymentMethod = s.checkHasPaymentMethod(sub.StripeCustomerID)

	return resp, nil
}

func (s *SubscriptionService) checkHasPaymentMethod(stripeCustomerID string) bool {
	if stripeCustomerID == "" {
		return false
	}
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(stripeCustomerID),
		Type:     stripe.String("card"),
	}
	params.Filters.AddFilter("limit", "", "1")
	iter := paymentmethod.List(params)
	return iter.Next()
}

// CreateSetupCheckoutSession creates a Stripe Checkout session in setup mode to collect payment method
func (s *SubscriptionService) CreateSetupCheckoutSession(ctx context.Context, tenantID, successURL, cancelURL string) (*models.CheckoutResponse, error) {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.StripeCustomerID == "" {
		return nil, fmt.Errorf("no Stripe customer for tenant")
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(sub.StripeCustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSetup)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"tenant_id":    tenantID,
			"session_type": "setup_payment",
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create setup checkout session: %w", err)
	}

	return &models.CheckoutResponse{
		SessionID:   sess.ID,
		CheckoutURL: sess.URL,
	}, nil
}

// ExtendTrial extends the trial period for a tenant (platform admin action)
func (s *SubscriptionService) ExtendTrial(ctx context.Context, tenantID string, additionalDays int, reason string) error {
	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("subscription not found: %w", err)
	}

	if sub.TrialEnd == nil {
		return fmt.Errorf("subscription has no trial end date")
	}

	newTrialEnd := sub.TrialEnd.Add(time.Duration(additionalDays) * 24 * time.Hour)
	sub.TrialEnd = &newTrialEnd

	// If expired, revert to trialing
	if sub.Status == models.StatusExpired {
		sub.Status = models.StatusTrialing
	}

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	// Update Stripe subscription trial end if stripe sub exists
	if sub.StripeSubscriptionID != "" {
		params := &stripe.SubscriptionParams{
			TrialEnd: stripe.Int64(newTrialEnd.Unix()),
		}
		if _, err := stripesub.Update(sub.StripeSubscriptionID, params); err != nil {
			log.Printf("Warning: Failed to update Stripe trial_end for tenant %s: %v", tenantID, err)
		}
	}

	log.Printf("Trial extended by %d days for tenant %s (reason: %s)", additionalDays, tenantID, reason)
	return nil
}

// GetEnhancedStats returns subscription stats with trial analytics
func (s *SubscriptionService) GetEnhancedStats(ctx context.Context) (*models.EnhancedStats, error) {
	baseStats, err := s.repo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	conversionRate, _ := s.repo.GetTrialConversionRate(ctx)

	expiring7d, _ := s.repo.GetExpiringTrials(ctx, 7)
	expiring30d, _ := s.repo.GetExpiringTrials(ctx, 30)

	var expiredCount, suspendedCount int64
	s.repo.CountByStatus(ctx, models.StatusExpired, &expiredCount)
	s.repo.CountByStatus(ctx, models.StatusSuspended, &suspendedCount)

	return &models.EnhancedStats{
		SubscriptionStats:   *baseStats,
		TrialConversionRate: conversionRate,
		ExpiringTrials7d:    len(expiring7d),
		ExpiringTrials30d:   len(expiring30d),
		ExpiredCount:        int(expiredCount),
		SuspendedCount:      int(suspendedCount),
	}, nil
}

// GetExpiringTrials returns subscriptions with trial ending within N days
func (s *SubscriptionService) GetExpiringTrials(ctx context.Context, days int) ([]models.TenantSubscription, error) {
	return s.repo.GetExpiringTrials(ctx, days)
}

// CreateTrialSubscription creates a new trial subscription for a tenant (triggered by NATS tenant.created)
func (s *SubscriptionService) CreateTrialSubscription(ctx context.Context, tenantID, email, businessName, region string) error {
	// Idempotency: skip if subscription already exists
	existing, err := s.repo.GetSubscription(ctx, tenantID)
	if err == nil && existing != nil {
		log.Printf("Trial subscription already exists for tenant %s, skipping", tenantID)
		return nil
	}

	// Find the default trial plan for the region
	plan, err := s.repo.GetDefaultTrialPlan(ctx, region)
	if err != nil {
		// Fallback: try to find by configured plan name
		plan, err = s.repo.GetPlanByName(ctx, s.cfg.DefaultTrialPlan)
		if err != nil {
			return fmt.Errorf("no trial plan found for region %s: %w", region, err)
		}
	}

	// Create Stripe customer
	if email == "" {
		email = fmt.Sprintf("tenant-%s@placeholder.tesserix.app", tenantID)
	}
	stripeCustomerID, err := s.getOrCreateStripeCustomer(ctx, tenantID, email)
	if err != nil {
		return fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	// Determine trial end
	trialEnd := time.Now().Add(time.Duration(plan.TrialDays) * 24 * time.Hour)

	// Get the Stripe price ID for the monthly interval
	priceID := plan.StripeMonthlyPriceID
	if priceID == "" {
		// If no Stripe price is configured, create a local-only trial subscription
		log.Printf("No Stripe price configured for plan %s, creating local-only trial", plan.Name)
		now := time.Now()
		sub := &models.TenantSubscription{
			TenantID:         tenantID,
			PlanID:           plan.ID,
			StripeCustomerID: stripeCustomerID,
			Status:           models.StatusTrialing,
			BillingInterval:  models.IntervalMonthly,
			TrialStart:       &now,
			TrialEnd:         &trialEnd,
			BillingEmail:     email,
		}
		if err := s.repo.CreateSubscription(ctx, sub); err != nil {
			return fmt.Errorf("failed to create local subscription: %w", err)
		}

		go func() {
			if err := s.tenantClient.UpdatePricingTier(context.Background(), tenantID, plan.Name); err != nil {
				log.Printf("Failed to sync pricing tier for tenant %s: %v", tenantID, err)
			}
		}()

		return nil
	}

	// Create Stripe subscription with trial
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(priceID)},
		},
		TrialEnd:        stripe.Int64(trialEnd.Unix()),
		PaymentBehavior: stripe.String("default_incomplete"),
	}
	params.AddMetadata("tenant_id", tenantID)
	params.AddMetadata("plan_id", plan.ID.String())

	stripeSub, err := stripesub.New(params)
	if err != nil {
		return fmt.Errorf("failed to create Stripe subscription: %w", err)
	}

	// Save local subscription record
	now := time.Now()
	trialEndFromStripe := time.Unix(stripeSub.TrialEnd, 0)
	sub := &models.TenantSubscription{
		TenantID:             tenantID,
		PlanID:               plan.ID,
		StripeCustomerID:     stripeCustomerID,
		StripeSubscriptionID: stripeSub.ID,
		Status:               models.StatusTrialing,
		BillingInterval:      models.IntervalMonthly,
		TrialStart:           &now,
		TrialEnd:             &trialEndFromStripe,
		BillingEmail:         email,
	}
	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	// Async sync to tenant-service
	go func() {
		if err := s.tenantClient.UpdatePricingTier(context.Background(), tenantID, plan.Name); err != nil {
			log.Printf("Failed to sync pricing tier for tenant %s: %v", tenantID, err)
		}
	}()

	return nil
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
