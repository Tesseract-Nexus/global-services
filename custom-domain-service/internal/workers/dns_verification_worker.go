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

	// Check CNAME delegation if enabled for this domain
	if domain.CNAMEDelegationEnabled && !domain.CNAMEDelegationVerified && w.cfg.CNAMEDelegation.Enabled {
		w.verifyCNAMEDelegation(ctx, domain)
	}

	// Verify DNS ownership (TXT or CNAME record)
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

// verifyCNAMEDelegation checks if CNAME delegation is properly configured
// Uses the stored CNAMEDelegationTarget from the database for security
func (w *DNSVerificationWorker) verifyCNAMEDelegation(ctx context.Context, domain *models.CustomDomain) {
	// Skip if already verified or too many attempts
	if domain.CNAMEDelegationVerified {
		return
	}

	maxAttempts := w.cfg.CNAMEDelegation.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 100
	}

	if domain.CNAMEDelegationCheckAttempts >= maxAttempts {
		log.Warn().
			Str("domain", domain.Domain).
			Int("attempts", domain.CNAMEDelegationCheckAttempts).
			Msg("CNAME delegation exceeded max verification attempts")
		return
	}

	// Use the stored CNAME delegation target from the database
	// This ensures we validate against the tenant-specific target that was assigned at domain creation
	// If not stored (legacy domains), fall back to generating based on tenant ID
	expectedTarget := domain.CNAMEDelegationTarget
	if expectedTarget == "" {
		// Fallback for domains created before CNAMEDelegationTarget was stored
		expectedTarget = w.dnsVerifier.GetCNAMEDelegationTargetForTenant(domain.Domain, domain.TenantID.String())
		log.Debug().
			Str("domain", domain.Domain).
			Str("generated_target", expectedTarget).
			Msg("CNAME delegation target not stored, using generated target")
	}

	// Verify CNAME delegation against the stored/expected target
	cnameResult, err := w.dnsVerifier.VerifyCNAMEDelegationWithTarget(ctx, domain.Domain, expectedTarget)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("CNAME delegation verification error")
		return
	}

	// Update CNAME delegation status
	if err := w.repo.UpdateCNAMEDelegationVerification(ctx, domain.ID, cnameResult.IsVerified, domain.CNAMEDelegationCheckAttempts+1); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update CNAME delegation status")
		return
	}

	if cnameResult.IsVerified {
		log.Info().
			Str("domain", domain.Domain).
			Str("expected_target", expectedTarget).
			Str("found_cname", cnameResult.FoundCNAME).
			Msg("CNAME delegation verified successfully - DNS-01 challenges now possible")
	} else {
		log.Debug().
			Str("domain", domain.Domain).
			Str("expected_target", expectedTarget).
			Str("message", cnameResult.Message).
			Msg("CNAME delegation not yet verified")
	}
}
