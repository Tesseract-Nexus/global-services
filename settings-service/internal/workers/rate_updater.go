package workers

import (
	"log"
	"sync"
	"time"

	"settings-service/internal/services"
)

const (
	// DefaultUpdateInterval is the default interval for rate updates
	DefaultUpdateInterval = 1 * time.Hour

	// RetryInterval is the interval to wait before retrying after a failure
	RetryInterval = 5 * time.Minute

	// MaxRetries is the maximum number of consecutive retries
	MaxRetries = 3
)

// RateUpdater handles periodic updates of exchange rates
type RateUpdater struct {
	currencyService services.CurrencyService
	interval        time.Duration
	stopChan        chan struct{}
	doneChan        chan struct{}
	mu              sync.Mutex
	running         bool
	lastUpdate      time.Time
	lastError       error
	retryCount      int
}

// NewRateUpdater creates a new rate updater
func NewRateUpdater(currencyService services.CurrencyService, interval time.Duration) *RateUpdater {
	if interval == 0 {
		interval = DefaultUpdateInterval
	}

	return &RateUpdater{
		currencyService: currencyService,
		interval:        interval,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}
}

// Start begins the rate update loop
func (u *RateUpdater) Start() {
	u.mu.Lock()
	if u.running {
		u.mu.Unlock()
		return
	}
	u.running = true
	u.mu.Unlock()

	go u.run()
	log.Printf("Rate updater started with interval: %v", u.interval)
}

// Stop stops the rate update loop
func (u *RateUpdater) Stop() {
	u.mu.Lock()
	if !u.running {
		u.mu.Unlock()
		return
	}
	u.running = false
	u.mu.Unlock()

	close(u.stopChan)
	<-u.doneChan
	log.Println("Rate updater stopped")
}

// ForceUpdate triggers an immediate rate update
func (u *RateUpdater) ForceUpdate() error {
	return u.updateRates()
}

// LastUpdate returns the time of the last successful update
func (u *RateUpdater) LastUpdate() time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastUpdate
}

// LastError returns the last error encountered during update
func (u *RateUpdater) LastError() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastError
}

// IsRunning returns whether the updater is running
func (u *RateUpdater) IsRunning() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.running
}

// run is the main update loop
func (u *RateUpdater) run() {
	defer close(u.doneChan)

	// Initial update on startup
	if err := u.updateRates(); err != nil {
		log.Printf("Initial rate update failed: %v", err)
	}

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-u.stopChan:
			return
		case <-ticker.C:
			if err := u.updateRates(); err != nil {
				u.handleError(err)
			} else {
				u.resetRetryCount()
			}
		}
	}
}

// updateRates performs the actual rate update
func (u *RateUpdater) updateRates() error {
	log.Println("Updating exchange rates...")

	err := u.currencyService.RefreshRates()

	u.mu.Lock()
	defer u.mu.Unlock()

	if err != nil {
		u.lastError = err
		log.Printf("Failed to update exchange rates: %v", err)
		return err
	}

	u.lastUpdate = time.Now()
	u.lastError = nil
	log.Printf("Exchange rates updated successfully at %v", u.lastUpdate)
	return nil
}

// handleError handles update errors with retry logic
func (u *RateUpdater) handleError(err error) {
	u.mu.Lock()
	u.retryCount++
	retryCount := u.retryCount
	u.mu.Unlock()

	if retryCount <= MaxRetries {
		log.Printf("Rate update failed (attempt %d/%d), retrying in %v: %v",
			retryCount, MaxRetries, RetryInterval, err)

		// Schedule retry
		go func() {
			time.Sleep(RetryInterval)
			u.mu.Lock()
			running := u.running
			u.mu.Unlock()

			if running {
				if err := u.updateRates(); err != nil {
					log.Printf("Retry %d failed: %v", retryCount, err)
				} else {
					u.resetRetryCount()
				}
			}
		}()
	} else {
		log.Printf("Rate update failed after %d retries, will try again at next scheduled interval", MaxRetries)
		u.resetRetryCount()
	}
}

// resetRetryCount resets the retry counter
func (u *RateUpdater) resetRetryCount() {
	u.mu.Lock()
	u.retryCount = 0
	u.mu.Unlock()
}

// Status returns the current status of the updater
type UpdaterStatus struct {
	Running    bool      `json:"running"`
	LastUpdate time.Time `json:"last_update,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	Interval   string    `json:"interval"`
}

// Status returns the current status of the updater
func (u *RateUpdater) Status() UpdaterStatus {
	u.mu.Lock()
	defer u.mu.Unlock()

	status := UpdaterStatus{
		Running:  u.running,
		Interval: u.interval.String(),
	}

	if !u.lastUpdate.IsZero() {
		status.LastUpdate = u.lastUpdate
	}

	if u.lastError != nil {
		status.LastError = u.lastError.Error()
	}

	return status
}
