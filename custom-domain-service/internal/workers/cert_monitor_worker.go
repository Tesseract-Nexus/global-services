package workers

import (
	"context"
	"time"

	"custom-domain-service/internal/clients"
	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"

	"github.com/rs/zerolog/log"
)

// CertMonitorWorker handles certificate expiry monitoring and renewal
type CertMonitorWorker struct {
	cfg       *config.Config
	repo      *repository.DomainRepository
	k8sClient *clients.KubernetesClient
	stopCh    chan struct{}
}

// NewCertMonitorWorker creates a new certificate monitor worker
func NewCertMonitorWorker(
	cfg *config.Config,
	repo *repository.DomainRepository,
	k8sClient *clients.KubernetesClient,
) *CertMonitorWorker {
	return &CertMonitorWorker{
		cfg:       cfg,
		repo:      repo,
		k8sClient: k8sClient,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the certificate monitor worker
func (w *CertMonitorWorker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.cfg.Workers.CertMonitorInterval).Msg("Starting certificate monitor worker")

	ticker := time.NewTicker(w.cfg.Workers.CertMonitorInterval)
	defer ticker.Stop()

	// Run immediately on start
	w.run(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Certificate monitor worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			log.Info().Msg("Certificate monitor worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// Stop stops the worker
func (w *CertMonitorWorker) Stop() {
	close(w.stopCh)
}

func (w *CertMonitorWorker) run(ctx context.Context) {
	log.Debug().Msg("Running certificate expiry check")

	// Get domains with certificates expiring soon
	domains, err := w.repo.GetExpiringCertificates(ctx, w.cfg.SSL.RenewalDaysBefore)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get expiring certificates")
		return
	}

	if len(domains) == 0 {
		log.Debug().Msg("No certificates expiring soon")
		return
	}

	log.Info().Int("count", len(domains)).Msg("Found certificates expiring soon")

	for _, domain := range domains {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.renewCertificate(ctx, &domain)
	}
}

func (w *CertMonitorWorker) renewCertificate(ctx context.Context, domain *models.CustomDomain) {
	log.Info().Str("domain", domain.Domain).Msg("Initiating certificate renewal")

	// Update status to indicate renewal in progress
	if domain.SSLExpiresAt != nil {
		daysRemaining := int(time.Until(*domain.SSLExpiresAt).Hours() / 24)
		if daysRemaining <= 0 {
			// Certificate expired
			w.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusExpired, domain.SSLCertSecretName, domain.SSLExpiresAt, "")
		} else if daysRemaining <= 7 {
			// Expiring soon
			w.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusExpiring, domain.SSLCertSecretName, domain.SSLExpiresAt, "")
		}
	}

	// cert-manager should handle renewal automatically when the Certificate resource exists
	// Just verify the certificate status
	certResult, err := w.k8sClient.GetCertificateStatus(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to get certificate status")
		return
	}

	if certResult.IsReady && certResult.ExpiresAt != nil {
		// Certificate was renewed
		w.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusActive, certResult.SecretName, certResult.ExpiresAt, "")
		log.Info().Str("domain", domain.Domain).Time("expires_at", *certResult.ExpiresAt).Msg("Certificate renewed successfully")
	} else if certResult.Status == models.SSLStatusFailed {
		log.Error().Str("domain", domain.Domain).Str("error", certResult.Error).Msg("Certificate renewal failed")
		w.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusFailed, certResult.SecretName, nil, certResult.Error)
	}
}

// CheckAllActiveCertificates checks all active domain certificates (for status sync)
func (w *CertMonitorWorker) CheckAllActiveCertificates(ctx context.Context) error {
	domains, err := w.repo.GetAllActive(ctx)
	if err != nil {
		return err
	}

	for _, domain := range domains {
		if domain.SSLStatus != models.SSLStatusActive {
			continue
		}

		certResult, err := w.k8sClient.GetCertificateStatus(ctx, &domain)
		if err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to get certificate status")
			continue
		}

		if certResult.ExpiresAt != nil && (domain.SSLExpiresAt == nil || !domain.SSLExpiresAt.Equal(*certResult.ExpiresAt)) {
			// Update expiry date
			w.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusActive, certResult.SecretName, certResult.ExpiresAt, "")
		}
	}

	return nil
}
