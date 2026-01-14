package worker

import (
	"context"
	"log"
	"time"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/repository"
)

// CacheCleanupConfig holds configuration for the cleanup worker
type CacheCleanupConfig struct {
	Interval  time.Duration // How often to run cleanup
	BatchSize int           // How many entries to delete per batch
	Enabled   bool          // Whether cleanup is enabled
}

// DefaultCacheCleanupConfig returns sensible defaults
func DefaultCacheCleanupConfig() CacheCleanupConfig {
	return CacheCleanupConfig{
		Interval:  1 * time.Hour,
		BatchSize: 1000,
		Enabled:   true,
	}
}

// CacheCleanupWorker handles background cleanup of expired cache entries
type CacheCleanupWorker struct {
	cacheRepo repository.AddressCacheRepository
	config    CacheCleanupConfig
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewCacheCleanupWorker creates a new cleanup worker
func NewCacheCleanupWorker(
	cacheRepo repository.AddressCacheRepository,
	config CacheCleanupConfig,
) *CacheCleanupWorker {
	return &CacheCleanupWorker{
		cacheRepo: cacheRepo,
		config:    config,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start begins the background cleanup routine
func (w *CacheCleanupWorker) Start() {
	if !w.config.Enabled {
		log.Println("Cache cleanup worker is disabled")
		return
	}

	go w.run()
	log.Printf("Address cache cleanup worker started (interval: %v, batch size: %d)",
		w.config.Interval, w.config.BatchSize)
}

// Stop signals the worker to stop and waits for completion
func (w *CacheCleanupWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
	log.Println("Address cache cleanup worker stopped")
}

func (w *CacheCleanupWorker) run() {
	defer close(w.doneCh)

	// Run immediately on start
	w.cleanup()

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.cleanup()
		case <-w.stopCh:
			return
		}
	}
}

func (w *CacheCleanupWorker) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	totalDeleted := int64(0)
	iterations := 0
	maxIterations := 100 // Safety limit

	// Delete in batches until no more expired entries
	for iterations < maxIterations {
		deleted, err := w.cacheRepo.DeleteExpired(ctx, w.config.BatchSize)
		if err != nil {
			log.Printf("Error during cache cleanup: %v", err)
			break
		}

		totalDeleted += deleted
		iterations++

		if deleted < int64(w.config.BatchSize) {
			// No more expired entries
			break
		}

		// Small pause between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}

	duration := time.Since(startTime)

	if totalDeleted > 0 {
		log.Printf("Cache cleanup completed: deleted %d expired entries in %v (%d batches)",
			totalDeleted, duration, iterations)
	}
}

// RunOnce performs a single cleanup pass (for testing or manual triggers)
func (w *CacheCleanupWorker) RunOnce(ctx context.Context) (int64, error) {
	totalDeleted := int64(0)

	for {
		deleted, err := w.cacheRepo.DeleteExpired(ctx, w.config.BatchSize)
		if err != nil {
			return totalDeleted, err
		}

		totalDeleted += deleted

		if deleted < int64(w.config.BatchSize) {
			break
		}

		// Small pause between batches
		time.Sleep(50 * time.Millisecond)
	}

	return totalDeleted, nil
}
