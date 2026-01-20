package workers

import (
	"context"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"
	"custom-domain-service/internal/services"

	"github.com/rs/zerolog/log"
)

// DNSVerificationWorker handles background DNS verification
type DNSVerificationWorker struct {
	cfg         *config.Config
	repo        *repository.DomainRepository
	dnsVerifier *services.DNSVerifier
	domainSvc   *services.DomainService
	stopCh      chan struct{}
}

// NewDNSVerificationWorker creates a new DNS verification worker
func NewDNSVerificationWorker(
	cfg *config.Config,
	repo *repository.DomainRepository,
	dnsVerifier *services.DNSVerifier,
	domainSvc *services.DomainService,
) *DNSVerificationWorker {
	return &DNSVerificationWorker{
		cfg:         cfg,
		repo:        repo,
		dnsVerifier: dnsVerifier,
		domainSvc:   domainSvc,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the DNS verification worker
func (w *DNSVerificationWorker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.cfg.Workers.DNSVerificationInterval).Msg("Starting DNS verification worker")

	ticker := time.NewTicker(w.cfg.Workers.DNSVerificationInterval)
	defer ticker.Stop()

	// Run immediately on start
	w.run(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("DNS verification worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			log.Info().Msg("DNS verification worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// Stop stops the worker
func (w *DNSVerificationWorker) Stop() {
	close(w.stopCh)
}

func (w *DNSVerificationWorker) run(ctx context.Context) {
	log.Debug().Msg("Running DNS verification check")

	// Get domains pending verification
	domains, err := w.repo.GetPendingVerification(ctx, 50)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get domains pending verification")
		return
	}

	if len(domains) == 0 {
		return
	}

	log.Info().Int("count", len(domains)).Msg("Processing domains for DNS verification")

	for _, domain := range domains {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.verifyDomain(ctx, &domain)

		// Small delay between verifications to avoid DNS rate limiting
		time.Sleep(2 * time.Second)
	}
}

func (w *DNSVerificationWorker) verifyDomain(ctx context.Context, domain *models.CustomDomain) {
	log.Debug().Str("domain", domain.Domain).Msg("Verifying domain DNS")

	// Skip if too many attempts
	if domain.DNSCheckAttempts >= 100 {
		log.Warn().Str("domain", domain.Domain).Int("attempts", domain.DNSCheckAttempts).Msg("Domain exceeded max verification attempts")
		if err := w.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "DNS verification failed after maximum attempts"); err != nil {
			log.Error().Err(err).Msg("Failed to update domain status")
		}
		return
	}

	// Verify DNS
	result, err := w.dnsVerifier.VerifyDomain(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("DNS verification error")
		return
	}

	// Update verification status
	if err := w.repo.UpdateDNSVerification(ctx, domain.ID, result.IsVerified, domain.DNSCheckAttempts+1); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update verification status")
		return
	}

	if result.IsVerified {
		log.Info().Str("domain", domain.Domain).Msg("Domain DNS verified successfully")
		// Note: The domain service will handle provisioning when status changes to provisioning
	} else {
		log.Debug().Str("domain", domain.Domain).Str("message", result.Message).Msg("Domain DNS not yet verified")
	}
}
