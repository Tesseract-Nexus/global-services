package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"subscription-service/internal/models"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Plans

func (r *SubscriptionRepository) ListPlans(ctx context.Context, activeOnly bool) ([]models.SubscriptionPlan, error) {
	var plans []models.SubscriptionPlan
	query := r.db.WithContext(ctx).Order("sort_order ASC")
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	err := query.Find(&plans).Error
	return plans, err
}

func (r *SubscriptionRepository) GetPlan(ctx context.Context, id uuid.UUID) (*models.SubscriptionPlan, error) {
	var plan models.SubscriptionPlan
	err := r.db.WithContext(ctx).First(&plan, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *SubscriptionRepository) GetPlanByName(ctx context.Context, name string) (*models.SubscriptionPlan, error) {
	var plan models.SubscriptionPlan
	err := r.db.WithContext(ctx).First(&plan, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *SubscriptionRepository) CreatePlan(ctx context.Context, plan *models.SubscriptionPlan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *SubscriptionRepository) UpdatePlan(ctx context.Context, plan *models.SubscriptionPlan) error {
	plan.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(plan).Error
}

func (r *SubscriptionRepository) DeletePlan(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.SubscriptionPlan{}).Where("id = ?", id).Update("is_active", false).Error
}

// Subscriptions

func (r *SubscriptionRepository) GetSubscription(ctx context.Context, tenantID string) (*models.TenantSubscription, error) {
	var sub models.TenantSubscription
	err := r.db.WithContext(ctx).Preload("Plan").First(&sub, "tenant_id = ?", tenantID).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*models.TenantSubscription, error) {
	var sub models.TenantSubscription
	err := r.db.WithContext(ctx).Preload("Plan").First(&sub, "stripe_subscription_id = ?", stripeSubID).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID string) (*models.TenantSubscription, error) {
	var sub models.TenantSubscription
	err := r.db.WithContext(ctx).Preload("Plan").First(&sub, "stripe_customer_id = ?", stripeCustomerID).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) CreateSubscription(ctx context.Context, sub *models.TenantSubscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

func (r *SubscriptionRepository) UpdateSubscription(ctx context.Context, sub *models.TenantSubscription) error {
	sub.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(sub).Error
}

// Invoices

func (r *SubscriptionRepository) ListInvoices(ctx context.Context, tenantID string) ([]models.SubscriptionInvoice, error) {
	var invoices []models.SubscriptionInvoice
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&invoices).Error
	return invoices, err
}

func (r *SubscriptionRepository) UpsertInvoice(ctx context.Context, invoice *models.SubscriptionInvoice) error {
	return r.db.WithContext(ctx).
		Where("stripe_invoice_id = ?", invoice.StripeInvoiceID).
		Assign(invoice).
		FirstOrCreate(invoice).Error
}

// Events

func (r *SubscriptionRepository) CreateEvent(ctx context.Context, event *models.SubscriptionEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *SubscriptionRepository) EventExists(ctx context.Context, stripeEventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SubscriptionEvent{}).Where("stripe_event_id = ?", stripeEventID).Count(&count).Error
	return count > 0, err
}

func (r *SubscriptionRepository) MarkEventProcessed(ctx context.Context, eventID uuid.UUID, processingError string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"processed":    true,
		"processed_at": &now,
	}
	if processingError != "" {
		updates["processing_error"] = processingError
	}
	return r.db.WithContext(ctx).Model(&models.SubscriptionEvent{}).Where("id = ?", eventID).Updates(updates).Error
}

// Stats

func (r *SubscriptionRepository) GetStats(ctx context.Context) (*models.SubscriptionStats, error) {
	stats := &models.SubscriptionStats{}

	var activeCount, trialingCount, pastDueCount, canceledCount int64
	r.db.WithContext(ctx).Model(&models.TenantSubscription{}).Where("status = ?", "active").Count(&activeCount)
	r.db.WithContext(ctx).Model(&models.TenantSubscription{}).Where("status = ?", "trialing").Count(&trialingCount)
	r.db.WithContext(ctx).Model(&models.TenantSubscription{}).Where("status = ?", "past_due").Count(&pastDueCount)
	r.db.WithContext(ctx).Model(&models.TenantSubscription{}).Where("status = ?", "canceled").Count(&canceledCount)

	stats.ActiveCount = int(activeCount)
	stats.TrialingCount = int(trialingCount)
	stats.PastDueCount = int(pastDueCount)
	stats.CanceledCount = int(canceledCount)

	// MRR: monthly prices for active monthly subs + yearly/12 for yearly
	var mrr int64
	r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(
			CASE
				WHEN ts.billing_interval = 'monthly' THEN sp.monthly_price_cents
				WHEN ts.billing_interval = 'yearly' THEN sp.yearly_price_cents / 12
				ELSE 0
			END
		), 0) as mrr
		FROM tenant_subscriptions ts
		JOIN subscription_plans sp ON ts.plan_id = sp.id
		WHERE ts.status IN ('active', 'trialing')
	`).Scan(&mrr)
	stats.MRR = int(mrr)

	// Total revenue from paid invoices
	var totalRevenue int64
	r.db.WithContext(ctx).Model(&models.SubscriptionInvoice{}).Where("status = ?", "paid").Select("COALESCE(SUM(amount_paid_cents), 0)").Scan(&totalRevenue)
	stats.TotalRevenue = int(totalRevenue)

	return stats, nil
}
