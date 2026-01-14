package background

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/config"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/services"
)

// Runner manages background jobs for draft persistence
type Runner struct {
	draftSvc       *services.DraftService
	config         config.DraftConfig
	stopCh         chan struct{}
	wg             sync.WaitGroup
	cleanupTicker  *time.Ticker
	reminderTicker *time.Ticker
}

// NewRunner creates a new background runner
func NewRunner(draftSvc *services.DraftService, cfg config.DraftConfig) *Runner {
	return &Runner{
		draftSvc: draftSvc,
		config:   cfg,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background job processing
func (r *Runner) Start() {
	log.Println("Starting background job runner...")

	// Cleanup job - runs every CleanupInterval minutes
	cleanupInterval := time.Duration(r.config.CleanupInterval) * time.Minute
	r.cleanupTicker = time.NewTicker(cleanupInterval)
	log.Printf("Draft cleanup job scheduled every %v", cleanupInterval)

	// Reminder job - runs every ReminderInterval hours
	reminderInterval := time.Duration(r.config.ReminderInterval) * time.Hour
	r.reminderTicker = time.NewTicker(reminderInterval)
	log.Printf("Draft reminder job scheduled every %v", reminderInterval)

	// Start cleanup goroutine
	r.wg.Add(1)
	go r.runCleanupJob()

	// Start reminder goroutine
	r.wg.Add(1)
	go r.runReminderJob()

	log.Println("Background job runner started successfully")
}

// Stop gracefully stops all background jobs
func (r *Runner) Stop() {
	log.Println("Stopping background job runner...")
	close(r.stopCh)

	if r.cleanupTicker != nil {
		r.cleanupTicker.Stop()
	}
	if r.reminderTicker != nil {
		r.reminderTicker.Stop()
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Background job runner stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Println("Background job runner stop timeout - forcing shutdown")
	}
}

// runCleanupJob runs the draft cleanup job periodically
func (r *Runner) runCleanupJob() {
	defer r.wg.Done()

	// Run immediately on start
	r.executeCleanup()

	for {
		select {
		case <-r.stopCh:
			log.Println("Cleanup job stopping...")
			return
		case <-r.cleanupTicker.C:
			r.executeCleanup()
		}
	}
}

// executeCleanup performs the actual cleanup
func (r *Runner) executeCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("Running draft cleanup job...")
	cleaned, err := r.draftSvc.CleanupExpiredDrafts(ctx)
	if err != nil {
		log.Printf("Error in draft cleanup job: %v", err)
	} else {
		log.Printf("Draft cleanup job completed: %d drafts cleaned", cleaned)
	}
}

// runReminderJob runs the reminder job periodically
func (r *Runner) runReminderJob() {
	defer r.wg.Done()

	// Don't run immediately - wait for first interval
	for {
		select {
		case <-r.stopCh:
			log.Println("Reminder job stopping...")
			return
		case <-r.reminderTicker.C:
			r.executeReminders()
		}
	}
}

// executeReminders sends reminder emails
func (r *Runner) executeReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Println("Running draft reminder job...")
	sent, err := r.draftSvc.ProcessReminders(ctx)
	if err != nil {
		log.Printf("Error in draft reminder job: %v", err)
	} else {
		log.Printf("Draft reminder job completed: %d reminders sent", sent)
	}
}

// RunOnce runs cleanup and reminders once (for testing/manual trigger)
func (r *Runner) RunOnce(ctx context.Context) error {
	log.Println("Running one-time cleanup and reminders...")

	cleaned, err := r.draftSvc.CleanupExpiredDrafts(ctx)
	if err != nil {
		log.Printf("Cleanup error: %v", err)
	} else {
		log.Printf("Cleaned %d expired drafts", cleaned)
	}

	sent, err := r.draftSvc.ProcessReminders(ctx)
	if err != nil {
		log.Printf("Reminder error: %v", err)
	} else {
		log.Printf("Sent %d reminders", sent)
	}

	return nil
}
