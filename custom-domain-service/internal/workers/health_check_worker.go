package workers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"

	"github.com/rs/zerolog/log"
)

// HealthCheckWorker performs health checks on active domains
type HealthCheckWorker struct {
	cfg        *config.Config
	repo       *repository.DomainRepository
	httpClient *http.Client
	stopCh     chan struct{}
}

// NewHealthCheckWorker creates a new health check worker
func NewHealthCheckWorker(
	cfg *config.Config,
	repo *repository.DomainRepository,
) *HealthCheckWorker {
	return &HealthCheckWorker{
		cfg:  cfg,
		repo: repo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stopCh: make(chan struct{}),
	}
}

// Start starts the health check worker
func (w *HealthCheckWorker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.cfg.Workers.HealthCheckInterval).Msg("Starting health check worker")

	ticker := time.NewTicker(w.cfg.Workers.HealthCheckInterval)
	defer ticker.Stop()

	// Run immediately on start
	w.run(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Health check worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			log.Info().Msg("Health check worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// Stop stops the worker
func (w *HealthCheckWorker) Stop() {
	close(w.stopCh)
}

func (w *HealthCheckWorker) run(ctx context.Context) {
	log.Debug().Msg("Running health checks")

	domains, err := w.repo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active domains")
		return
	}

	if len(domains) == 0 {
		return
	}

	log.Debug().Int("count", len(domains)).Msg("Checking health of active domains")

	for _, domain := range domains {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.checkDomain(ctx, &domain)

		// Small delay between checks
		time.Sleep(500 * time.Millisecond)
	}
}

func (w *HealthCheckWorker) checkDomain(ctx context.Context, domain *models.CustomDomain) {
	health := &models.DomainHealth{
		DomainID:  domain.ID,
		CheckedAt: time.Now(),
	}

	url := fmt.Sprintf("https://%s", domain.Domain)
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		health.IsHealthy = false
		health.ErrorMessage = fmt.Sprintf("Failed to create request: %v", err)
		w.saveHealth(ctx, health)
		return
	}

	resp, err := w.httpClient.Do(req)
	health.ResponseTime = time.Since(start).Milliseconds()

	if err != nil {
		health.IsHealthy = false
		health.ErrorMessage = fmt.Sprintf("Request failed: %v", err)
		w.saveHealth(ctx, health)
		log.Debug().Str("domain", domain.Domain).Err(err).Msg("Health check failed")
		return
	}
	defer resp.Body.Close()

	health.StatusCode = resp.StatusCode
	health.IsHealthy = resp.StatusCode >= 200 && resp.StatusCode < 400

	// Check SSL certificate
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		health.SSLValid = time.Now().Before(cert.NotAfter)
		health.SSLExpiresIn = int(time.Until(cert.NotAfter).Hours() / 24)
	}

	if !health.IsHealthy {
		health.ErrorMessage = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
	}

	w.saveHealth(ctx, health)

	if !health.IsHealthy {
		log.Warn().
			Str("domain", domain.Domain).
			Int("status", health.StatusCode).
			Str("error", health.ErrorMessage).
			Msg("Domain health check failed")
	}
}

func (w *HealthCheckWorker) saveHealth(ctx context.Context, health *models.DomainHealth) {
	if err := w.repo.SaveHealthCheck(ctx, health); err != nil {
		log.Warn().Err(err).Msg("Failed to save health check result")
	}
}
