package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"subscription-service/internal/clients"
	"subscription-service/internal/models"
	"subscription-service/internal/repository"
)

type WebhookService struct {
	repo         *repository.SubscriptionRepository
	tenantClient *clients.TenantClient
}

func NewWebhookService(repo *repository.SubscriptionRepository, tenantClient *clients.TenantClient) *WebhookService {
	return &WebhookService{repo: repo, tenantClient: tenantClient}
}

func (s *WebhookService) ProcessEvent(ctx context.Context, event stripe.Event) error {
	// Idempotency check
	exists, err := s.repo.EventExists(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("failed to check event existence: %w", err)
	}
	if exists {
		log.Printf("Event %s already processed, skipping", event.ID)
		return nil
	}

	// Store event
	payload, _ := json.Marshal(event.Data.Object)
	dbEvent := &models.SubscriptionEvent{
		StripeEventID: event.ID,
		EventType:     string(event.Type),
		Payload:       models.JSONB{"raw": string(payload)},
	}

	if err := s.repo.CreateEvent(ctx, dbEvent); err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Process based on event type
	var processingError string
	switch event.Type {
	case "checkout.session.completed":
		processingError = s.handleCheckoutCompleted(ctx, event)
	case "customer.subscription.updated":
		processingError = s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		processingError = s.handleSubscriptionDeleted(ctx, event)
	case "invoice.paid":
		processingError = s.handleInvoicePaid(ctx, event)
	case "invoice.payment_failed":
		processingError = s.handleInvoicePaymentFailed(ctx, event)
	case "invoice.created":
		processingError = s.handleInvoiceCreated(ctx, event)
	default:
		log.Printf("Unhandled event type: %s", event.Type)
	}

	// Mark as processed
	if err := s.repo.MarkEventProcessed(ctx, dbEvent.ID, processingError); err != nil {
		log.Printf("Failed to mark event %s as processed: %v", event.ID, err)
	}

	if processingError != "" {
		return fmt.Errorf("event processing error: %s", processingError)
	}

	return nil
}

// Suppress unused import warning
var _ = uuid.Nil

func (s *WebhookService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) string {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Sprintf("failed to parse checkout session: %v", err)
	}

	tenantID := sess.Metadata["tenant_id"]
	planIDStr := sess.Metadata["plan_id"]
	if tenantID == "" || planIDStr == "" {
		return "missing tenant_id or plan_id in session metadata"
	}

	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		return fmt.Sprintf("invalid plan_id: %v", err)
	}

	plan, err := s.repo.GetPlan(ctx, planID)
	if err != nil {
		return fmt.Sprintf("plan not found: %v", err)
	}

	sub, err := s.repo.GetSubscription(ctx, tenantID)
	if err != nil {
		sub = &models.TenantSubscription{
			TenantID:             tenantID,
			PlanID:               planID,
			StripeCustomerID:     sess.Customer.ID,
			StripeSubscriptionID: sess.Subscription.ID,
			Status:               models.StatusActive,
			BillingInterval:      models.IntervalMonthly,
			BillingEmail:         sess.CustomerEmail,
		}
		if err := s.repo.CreateSubscription(ctx, sub); err != nil {
			return fmt.Sprintf("failed to create subscription: %v", err)
		}
	} else {
		sub.PlanID = planID
		sub.StripeCustomerID = sess.Customer.ID
		sub.StripeSubscriptionID = sess.Subscription.ID
		sub.Status = models.StatusActive
		if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
			return fmt.Sprintf("failed to update subscription: %v", err)
		}
	}

	go func() {
		if err := s.tenantClient.UpdatePricingTier(context.Background(), tenantID, plan.Name); err != nil {
			log.Printf("Failed to sync pricing tier for tenant %s: %v", tenantID, err)
		}
	}()

	return ""
}

func (s *WebhookService) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) string {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Sprintf("failed to parse subscription: %v", err)
	}

	sub, err := s.repo.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return fmt.Sprintf("subscription not found for stripe_id %s: %v", stripeSub.ID, err)
	}

	sub.Status = mapStripeStatus(stripeSub.Status)
	sub.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd

	periodStart := time.Unix(stripeSub.CurrentPeriodStart, 0)
	periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
	sub.CurrentPeriodStart = &periodStart
	sub.CurrentPeriodEnd = &periodEnd

	if stripeSub.TrialStart > 0 {
		ts := time.Unix(stripeSub.TrialStart, 0)
		sub.TrialStart = &ts
	}
	if stripeSub.TrialEnd > 0 {
		te := time.Unix(stripeSub.TrialEnd, 0)
		sub.TrialEnd = &te
	}
	if stripeSub.CanceledAt > 0 {
		ca := time.Unix(stripeSub.CanceledAt, 0)
		sub.CanceledAt = &ca
	}

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Sprintf("failed to update subscription: %v", err)
	}

	go func() {
		if err := s.tenantClient.UpdatePricingTier(context.Background(), sub.TenantID, sub.Plan.Name); err != nil {
			log.Printf("Failed to sync pricing tier for tenant %s: %v", sub.TenantID, err)
		}
	}()

	return ""
}

func (s *WebhookService) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) string {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Sprintf("failed to parse subscription: %v", err)
	}

	sub, err := s.repo.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return fmt.Sprintf("subscription not found for stripe_id %s: %v", stripeSub.ID, err)
	}

	sub.Status = models.StatusCanceled
	now := time.Now()
	sub.CanceledAt = &now

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Sprintf("failed to update subscription: %v", err)
	}

	go func() {
		if err := s.tenantClient.UpdatePricingTier(context.Background(), sub.TenantID, "free"); err != nil {
			log.Printf("Failed to downgrade tenant %s to free: %v", sub.TenantID, err)
		}
	}()

	return ""
}

func (s *WebhookService) handleInvoicePaid(ctx context.Context, event stripe.Event) string {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Sprintf("failed to parse invoice: %v", err)
	}

	sub, err := s.repo.GetSubscriptionByStripeCustomerID(ctx, stripeInvoice.Customer.ID)
	if err != nil {
		log.Printf("No subscription found for customer %s, skipping invoice", stripeInvoice.Customer.ID)
		return ""
	}

	now := time.Now()
	invoice := &models.SubscriptionInvoice{
		TenantID:        sub.TenantID,
		SubscriptionID:  sub.ID,
		StripeInvoiceID: stripeInvoice.ID,
		Status:          models.InvoicePaid,
		AmountDueCents:  int(stripeInvoice.AmountDue),
		AmountPaidCents: int(stripeInvoice.AmountPaid),
		Currency:        string(stripeInvoice.Currency),
		PaidAt:          &now,
		Description:     stripeInvoice.Description,
	}

	if stripeInvoice.HostedInvoiceURL != "" {
		invoice.StripeHostedURL = stripeInvoice.HostedInvoiceURL
	}
	if stripeInvoice.InvoicePDF != "" {
		invoice.StripeInvoicePDF = stripeInvoice.InvoicePDF
	}
	if stripeInvoice.PeriodStart > 0 {
		ps := time.Unix(stripeInvoice.PeriodStart, 0)
		invoice.PeriodStart = &ps
	}
	if stripeInvoice.PeriodEnd > 0 {
		pe := time.Unix(stripeInvoice.PeriodEnd, 0)
		invoice.PeriodEnd = &pe
	}

	if err := s.repo.UpsertInvoice(ctx, invoice); err != nil {
		return fmt.Sprintf("failed to upsert invoice: %v", err)
	}

	return ""
}

func (s *WebhookService) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) string {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Sprintf("failed to parse invoice: %v", err)
	}

	sub, err := s.repo.GetSubscriptionByStripeCustomerID(ctx, stripeInvoice.Customer.ID)
	if err != nil {
		log.Printf("No subscription found for customer %s", stripeInvoice.Customer.ID)
		return ""
	}

	sub.Status = models.StatusPastDue
	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Sprintf("failed to update subscription status: %v", err)
	}

	return ""
}

func (s *WebhookService) handleInvoiceCreated(ctx context.Context, event stripe.Event) string {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Sprintf("failed to parse invoice: %v", err)
	}

	sub, err := s.repo.GetSubscriptionByStripeCustomerID(ctx, stripeInvoice.Customer.ID)
	if err != nil {
		log.Printf("No subscription found for customer %s, skipping invoice cache", stripeInvoice.Customer.ID)
		return ""
	}

	invoice := &models.SubscriptionInvoice{
		TenantID:        sub.TenantID,
		SubscriptionID:  sub.ID,
		StripeInvoiceID: stripeInvoice.ID,
		Status:          models.InvoiceStatus(stripeInvoice.Status),
		AmountDueCents:  int(stripeInvoice.AmountDue),
		AmountPaidCents: int(stripeInvoice.AmountPaid),
		Currency:        string(stripeInvoice.Currency),
		Description:     stripeInvoice.Description,
	}

	if stripeInvoice.HostedInvoiceURL != "" {
		invoice.StripeHostedURL = stripeInvoice.HostedInvoiceURL
	}
	if stripeInvoice.InvoicePDF != "" {
		invoice.StripeInvoicePDF = stripeInvoice.InvoicePDF
	}

	if err := s.repo.UpsertInvoice(ctx, invoice); err != nil {
		return fmt.Sprintf("failed to cache invoice: %v", err)
	}

	return ""
}

func mapStripeStatus(status stripe.SubscriptionStatus) models.SubscriptionStatus {
	switch status {
	case stripe.SubscriptionStatusActive:
		return models.StatusActive
	case stripe.SubscriptionStatusTrialing:
		return models.StatusTrialing
	case stripe.SubscriptionStatusPastDue:
		return models.StatusPastDue
	case stripe.SubscriptionStatusCanceled:
		return models.StatusCanceled
	case stripe.SubscriptionStatusUnpaid:
		return models.StatusUnpaid
	default:
		return models.StatusActive
	}
}
