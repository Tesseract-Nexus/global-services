package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/product"
	"subscription-service/internal/models"
	"subscription-service/internal/repository"
)

type PlanService struct {
	repo *repository.SubscriptionRepository
}

func NewPlanService(repo *repository.SubscriptionRepository) *PlanService {
	return &PlanService{repo: repo}
}

func (s *PlanService) ListPlans(ctx context.Context, activeOnly bool, currency string) ([]models.SubscriptionPlan, error) {
	return s.repo.ListPlans(ctx, activeOnly, currency)
}

func (s *PlanService) GetPlan(ctx context.Context, id uuid.UUID) (*models.SubscriptionPlan, error) {
	return s.repo.GetPlan(ctx, id)
}

func (s *PlanService) CreatePlan(ctx context.Context, req models.CreatePlanRequest) (*models.SubscriptionPlan, error) {
	currency := req.Currency
	if currency == "" {
		currency = "usd"
	}

	plan := &models.SubscriptionPlan{
		Name:              req.Name,
		DisplayName:       req.DisplayName,
		Description:       req.Description,
		MonthlyPriceCents: req.MonthlyPriceCents,
		YearlyPriceCents:  req.YearlyPriceCents,
		Currency:          currency,
		MaxProducts:       req.MaxProducts,
		MaxUsers:          req.MaxUsers,
		MaxStorageMB:      req.MaxStorageMB,
		Features:          req.Features,
		SortOrder:         req.SortOrder,
		IsActive:          req.IsActive,
		IsFree:            req.IsFree,
		TrialDays:         req.TrialDays,
	}

	if err := s.repo.CreatePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	return plan, nil
}

func (s *PlanService) UpdatePlan(ctx context.Context, id uuid.UUID, req models.UpdatePlanRequest) (*models.SubscriptionPlan, error) {
	plan, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	if req.DisplayName != nil {
		plan.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}
	if req.MonthlyPriceCents != nil {
		plan.MonthlyPriceCents = *req.MonthlyPriceCents
	}
	if req.YearlyPriceCents != nil {
		plan.YearlyPriceCents = *req.YearlyPriceCents
	}
	if req.MaxProducts != nil {
		plan.MaxProducts = *req.MaxProducts
	}
	if req.MaxUsers != nil {
		plan.MaxUsers = *req.MaxUsers
	}
	if req.MaxStorageMB != nil {
		plan.MaxStorageMB = *req.MaxStorageMB
	}
	if req.Features != nil {
		plan.Features = *req.Features
	}
	if req.SortOrder != nil {
		plan.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}
	if req.TrialDays != nil {
		plan.TrialDays = *req.TrialDays
	}

	plan.UpdatedAt = time.Now()

	if err := s.repo.UpdatePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	return plan, nil
}

func (s *PlanService) DeletePlan(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePlan(ctx, id)
}

// SyncToStripe creates/updates Stripe Products and Prices for all plans
func (s *PlanService) SyncToStripe(ctx context.Context) error {
	plans, err := s.repo.ListPlans(ctx, false, "")
	if err != nil {
		return fmt.Errorf("failed to list plans: %w", err)
	}

	for i := range plans {
		plan := &plans[i]
		if plan.IsFree {
			continue
		}

		// Create or update Stripe Product
		if plan.StripeProductID == "" {
			params := &stripe.ProductParams{
				Name:        stripe.String(plan.DisplayName),
				Description: stripe.String(plan.Description),
				Metadata:    map[string]string{"plan_id": plan.ID.String(), "plan_name": plan.Name},
			}
			prod, err := product.New(params)
			if err != nil {
				return fmt.Errorf("failed to create Stripe product for %s: %w", plan.Name, err)
			}
			plan.StripeProductID = prod.ID
		} else {
			params := &stripe.ProductParams{
				Name:        stripe.String(plan.DisplayName),
				Description: stripe.String(plan.Description),
			}
			if _, err := product.Update(plan.StripeProductID, params); err != nil {
				return fmt.Errorf("failed to update Stripe product for %s: %w", plan.Name, err)
			}
		}

		// Create monthly price if not exists
		if plan.StripeMonthlyPriceID == "" && plan.MonthlyPriceCents > 0 {
			params := &stripe.PriceParams{
				Product:    stripe.String(plan.StripeProductID),
				UnitAmount: stripe.Int64(int64(plan.MonthlyPriceCents)),
				Currency:   stripe.String(plan.Currency),
				Recurring: &stripe.PriceRecurringParams{
					Interval: stripe.String(string(stripe.PriceRecurringIntervalMonth)),
				},
				Metadata: map[string]string{"plan_id": plan.ID.String(), "interval": "monthly"},
			}
			p, err := price.New(params)
			if err != nil {
				return fmt.Errorf("failed to create monthly price for %s: %w", plan.Name, err)
			}
			plan.StripeMonthlyPriceID = p.ID
		}

		// Create yearly price if not exists
		if plan.StripeYearlyPriceID == "" && plan.YearlyPriceCents > 0 {
			params := &stripe.PriceParams{
				Product:    stripe.String(plan.StripeProductID),
				UnitAmount: stripe.Int64(int64(plan.YearlyPriceCents)),
				Currency:   stripe.String(plan.Currency),
				Recurring: &stripe.PriceRecurringParams{
					Interval: stripe.String(string(stripe.PriceRecurringIntervalYear)),
				},
				Metadata: map[string]string{"plan_id": plan.ID.String(), "interval": "yearly"},
			}
			p, err := price.New(params)
			if err != nil {
				return fmt.Errorf("failed to create yearly price for %s: %w", plan.Name, err)
			}
			plan.StripeYearlyPriceID = p.ID
		}

		if err := s.repo.UpdatePlan(ctx, plan); err != nil {
			return fmt.Errorf("failed to save plan %s after Stripe sync: %w", plan.Name, err)
		}
	}

	return nil
}
