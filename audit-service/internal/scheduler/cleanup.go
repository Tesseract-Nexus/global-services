package scheduler

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"audit-service/internal/config"
	"audit-service/internal/repository"
	"audit-service/internal/tenant"
)

// CleanupScheduler handles scheduled cleanup of old audit logs
type CleanupScheduler struct {
	repo           repository.AuditRepositoryInterface
	tenantRegistry *tenant.Registry
	config         config.RetentionConfig
	logger         *logrus.Logger
	cron           *cron.Cron
	mu             sync.Mutex
	running        bool
}

// NewCleanupScheduler creates a new cleanup scheduler
func NewCleanupScheduler(
	repo repository.AuditRepositoryInterface,
	tenantRegistry *tenant.Registry,
	cfg config.RetentionConfig,
	logger *logrus.Logger,
) *CleanupScheduler {
	return &CleanupScheduler{
		repo:           repo,
		tenantRegistry: tenantRegistry,
		config:         cfg,
		logger:         logger,
	}
}

// Start starts the cleanup scheduler
func (s *CleanupScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.config.CleanupEnabled {
		s.logger.Info("Audit log cleanup is disabled")
		return nil
	}

	s.cron = cron.New(cron.WithSeconds())

	// Add the cleanup job with the configured schedule
	schedule := s.config.CleanupSchedule
	if schedule == "" {
		schedule = "0 0 2 * * *" // Default: 2 AM daily (with seconds)
	}

	// Convert 5-field cron to 6-field (add seconds prefix)
	// Standard cron has 5 fields, robfig/cron with WithSeconds() expects 6
	fields := strings.Fields(schedule)
	if len(fields) == 5 {
		schedule = "0 " + schedule // Add seconds field
	}

	_, err := s.cron.AddFunc(schedule, s.runCleanup)
	if err != nil {
		s.logger.WithError(err).Error("Failed to schedule cleanup job")
		return err
	}

	s.cron.Start()
	s.running = true

	s.logger.WithFields(logrus.Fields{
		"schedule":       s.config.CleanupSchedule,
		"default_days":   s.config.DefaultDays,
		"min_days":       s.config.MinDays,
		"max_days":       s.config.MaxDays,
	}).Info("Audit log cleanup scheduler started")

	return nil
}

// Stop stops the cleanup scheduler
func (s *CleanupScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.cron == nil {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false
	s.logger.Info("Audit log cleanup scheduler stopped")
}

// runCleanup runs the cleanup job for all tenants
func (s *CleanupScheduler) runCleanup() {
	ctx := context.Background()
	startTime := time.Now()

	s.logger.Info("Starting scheduled audit log cleanup")

	// Get all active tenants from the registry
	tenants, err := s.tenantRegistry.GetAllTenants(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get tenant list for cleanup")
		return
	}

	var totalDeleted int64
	var tenantsProcessed int
	var errors []string

	for _, tenantID := range tenants {
		// Get tenant's retention settings
		settings, err := s.repo.GetRetentionSettings(ctx, tenantID)
		if err != nil {
			s.logger.WithError(err).WithField("tenant_id", tenantID).Warn("Failed to get retention settings")
			errors = append(errors, tenantID)
			continue
		}

		retentionDays := settings.RetentionDays
		if retentionDays <= 0 {
			retentionDays = s.config.DefaultDays
		}

		// Cleanup old logs
		deleted, err := s.repo.CleanupOldLogs(ctx, tenantID, retentionDays)
		if err != nil {
			s.logger.WithError(err).WithField("tenant_id", tenantID).Warn("Failed to cleanup logs")
			errors = append(errors, tenantID)
			continue
		}

		if deleted > 0 {
			s.logger.WithFields(logrus.Fields{
				"tenant_id":      tenantID,
				"retention_days": retentionDays,
				"logs_deleted":   deleted,
			}).Info("Cleaned up old audit logs")
		}

		totalDeleted += deleted
		tenantsProcessed++
	}

	duration := time.Since(startTime)

	s.logger.WithFields(logrus.Fields{
		"tenants_total":     len(tenants),
		"tenants_processed": tenantsProcessed,
		"tenants_failed":    len(errors),
		"logs_deleted":      totalDeleted,
		"duration":          duration.String(),
	}).Info("Completed scheduled audit log cleanup")
}

// RunNow triggers an immediate cleanup (for testing/manual trigger)
func (s *CleanupScheduler) RunNow() {
	go s.runCleanup()
}

// IsRunning returns whether the scheduler is running
func (s *CleanupScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetStats returns scheduler statistics
func (s *CleanupScheduler) GetStats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := map[string]interface{}{
		"running":        s.running,
		"enabled":        s.config.CleanupEnabled,
		"schedule":       s.config.CleanupSchedule,
		"default_days":   s.config.DefaultDays,
		"min_days":       s.config.MinDays,
		"max_days":       s.config.MaxDays,
	}

	if s.cron != nil && s.running {
		entries := s.cron.Entries()
		if len(entries) > 0 {
			stats["next_run"] = entries[0].Next.Format(time.RFC3339)
		}
	}

	return stats
}
