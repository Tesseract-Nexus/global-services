package services

import (
	"context"
	"log"
	"time"

	"subscription-service/internal/clients"
	"subscription-service/internal/models"
	"subscription-service/internal/repository"
)

// ExpirationService handles background processing of expired trials and grace periods
type ExpirationService struct {
	repo         *repository.SubscriptionRepository
	tenantClient *clients.TenantClient
}

func NewExpirationService(repo *repository.SubscriptionRepository, tenantClient *clients.TenantClient) *ExpirationService {
	return &ExpirationService{repo: repo, tenantClient: tenantClient}
}

// StartProcessor runs the expiration checks periodically
func (s *ExpirationService) StartProcessor(ctx context.Context, interval time.Duration) {
	log.Printf("Expiration processor started (interval: %s)", interval)

	// Run immediately on startup
	s.processAll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Expiration processor stopped")
			return
		case <-ticker.C:
			s.processAll(ctx)
		}
	}
}

func (s *ExpirationService) processAll(ctx context.Context) {
	s.processExpiredTrials(ctx)
	s.processGraceExpired(ctx)
}

// processExpiredTrials finds trialing subscriptions where trial has ended and marks them expired
func (s *ExpirationService) processExpiredTrials(ctx context.Context) {
	subs, err := s.repo.FindExpiredTrials(ctx)
	if err != nil {
		log.Printf("Error finding expired trials: %v", err)
		return
	}

	for i := range subs {
		sub := &subs[i]
		sub.Status = models.StatusExpired
		if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
			log.Printf("Error expiring trial for tenant %s: %v", sub.TenantID, err)
			continue
		}
		log.Printf("Trial expired for tenant %s", sub.TenantID)
	}

	if len(subs) > 0 {
		log.Printf("Processed %d expired trials", len(subs))
	}
}

// processGraceExpired finds past_due subscriptions where grace period has ended and suspends them
func (s *ExpirationService) processGraceExpired(ctx context.Context) {
	subs, err := s.repo.FindGraceExpired(ctx)
	if err != nil {
		log.Printf("Error finding grace-expired subscriptions: %v", err)
		return
	}

	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		sub.Status = models.StatusSuspended
		sub.SuspendedAt = &now
		if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
			log.Printf("Error suspending tenant %s: %v", sub.TenantID, err)
			continue
		}

		// Notify tenant-service about suspension
		go func(tenantID string) {
			if err := s.tenantClient.UpdatePricingTier(context.Background(), tenantID, "suspended"); err != nil {
				log.Printf("Failed to notify tenant-service about suspension for %s: %v", tenantID, err)
			}
		}(sub.TenantID)

		log.Printf("Subscription suspended for tenant %s (grace period expired)", sub.TenantID)
	}

	if len(subs) > 0 {
		log.Printf("Processed %d grace-expired subscriptions", len(subs))
	}
}
