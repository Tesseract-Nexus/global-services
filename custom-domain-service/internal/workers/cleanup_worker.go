package workers

import (
	"context"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/repository"

	"github.com/rs/zerolog/log"
)

// CleanupWorker handles cleanup of old data
type CleanupWorker struct {
	cfg    *config.Config
	repo   *repository.DomainRepository
	stopCh chan struct{}
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(
	cfg *config.Config,
	repo *repository.DomainRepository,
) *CleanupWorker {
	return &CleanupWorker{
		cfg:    cfg,
		repo:   repo,
		stopCh: make(chan struct{}),
	}
}

// Start starts the cleanup worker
func (w *CleanupWorker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.cfg.Workers.CleanupInterval).Msg("Starting cleanup worker")

	ticker := time.NewTicker(w.cfg.Workers.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Cleanup worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			log.Info().Msg("Cleanup worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// Stop stops the worker
func (w *CleanupWorker) Stop() {
	close(w.stopCh)
}

func (w *CleanupWorker) run(ctx context.Context) {
	log.Debug().Msg("Running cleanup")

	// Cleanup health checks older than 7 days
	healthDeleted, err := w.repo.CleanupOldHealthChecks(ctx, 7*24*time.Hour)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old health checks")
	} else if healthDeleted > 0 {
		log.Info().Int64("deleted", healthDeleted).Msg("Cleaned up old health checks")
	}

	// Cleanup activities older than 30 days
	activitiesDeleted, err := w.repo.CleanupOldActivities(ctx, 30*24*time.Hour)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old activities")
	} else if activitiesDeleted > 0 {
		log.Info().Int64("deleted", activitiesDeleted).Msg("Cleaned up old activities")
	}
}
