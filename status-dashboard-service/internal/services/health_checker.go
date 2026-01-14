package services

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tesseract-hub/status-dashboard-service/internal/config"
	"github.com/tesseract-hub/status-dashboard-service/internal/models"
)

// HealthChecker performs health checks on services (stateless, in-memory)
type HealthChecker struct {
	cfg         *config.Config
	httpClient  *http.Client
	services    map[uuid.UUID]*models.Service
	incidents   map[uuid.UUID]*models.Incident
	mu          sync.RWMutex
	subscribers map[string]chan *models.HealthCheck
	subMu       sync.RWMutex
	stopChan    chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cfg *config.Config) *HealthChecker {
	return &HealthChecker{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		services:    make(map[uuid.UUID]*models.Service),
		incidents:   make(map[uuid.UUID]*models.Incident),
		subscribers: make(map[string]chan *models.HealthCheck),
		stopChan:    make(chan struct{}),
	}
}

// Initialize loads services from config into memory
func (h *HealthChecker) Initialize() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, svc := range h.cfg.Services {
		id := uuid.New()
		h.services[id] = &models.Service{
			ID:          id,
			Name:        svc.Name,
			DisplayName: svc.DisplayName,
			URL:         svc.URL,
			HealthPath:  svc.HealthPath,
			Category:    svc.Category,
			SLATarget:   svc.SLATarget,
			Status:      models.StatusUnknown,
		}
	}

	log.WithField("count", len(h.services)).Info("Initialized services for health checking")
}

// Start begins the health check loop
func (h *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(h.cfg.CheckInterval)
	defer ticker.Stop()

	// Run initial check
	h.checkAllServices(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.checkAllServices(ctx)
		}
	}
}

// Stop stops the health checker
func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

// checkAllServices checks all services concurrently
func (h *HealthChecker) checkAllServices(ctx context.Context) {
	h.mu.RLock()
	services := make([]*models.Service, 0, len(h.services))
	for _, svc := range h.services {
		services = append(services, svc)
	}
	h.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(chan *models.HealthCheck, len(services))

	for _, svc := range services {
		wg.Add(1)
		go func(service *models.Service) {
			defer wg.Done()
			result := h.checkService(ctx, service)
			results <- result
		}(svc)
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	for result := range results {
		h.updateServiceStatus(result)
		h.checkForIncident(result)
		h.notifySubscribers(result)
	}
}

// checkService performs a health check on a single service
func (h *HealthChecker) checkService(ctx context.Context, service *models.Service) *models.HealthCheck {
	url := service.URL + service.HealthPath
	start := time.Now()

	result := &models.HealthCheck{
		ServiceID:   service.ID,
		ServiceName: service.DisplayName,
		CheckedAt:   start,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Status = models.StatusUnhealthy
		result.Error = err.Error()
		return result
	}

	resp, err := h.httpClient.Do(req)
	elapsed := time.Since(start)
	result.ResponseTimeMs = elapsed.Milliseconds()

	if err != nil {
		result.Status = models.StatusUnhealthy
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		result.Status = models.StatusHealthy
	case resp.StatusCode >= 500:
		result.Status = models.StatusUnhealthy
		result.Error = "Server error"
	case resp.StatusCode >= 400:
		result.Status = models.StatusDegraded
		result.Error = "Client error"
	default:
		result.Status = models.StatusDegraded
	}

	// Check response time threshold (>2s is degraded)
	if result.ResponseTimeMs > 2000 && result.Status == models.StatusHealthy {
		result.Status = models.StatusDegraded
	}

	return result
}

// updateServiceStatus updates the service status based on health check
func (h *HealthChecker) updateServiceStatus(check *models.HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if svc, ok := h.services[check.ServiceID]; ok {
		now := check.CheckedAt
		svc.Status = check.Status
		svc.LastCheckAt = &now
		svc.ResponseTimeMs = check.ResponseTimeMs
		svc.TotalChecks++

		if check.Status == models.StatusHealthy {
			svc.SuccessCount++
		} else {
			svc.FailureCount++
		}
	}
}

// checkForIncident checks if we need to create/resolve an incident
func (h *HealthChecker) checkForIncident(check *models.HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find existing incident for this service
	var existingIncident *models.Incident
	for _, inc := range h.incidents {
		if inc.ServiceID == check.ServiceID && inc.Status != "resolved" {
			existingIncident = inc
			break
		}
	}

	if check.Status == models.StatusUnhealthy {
		if existingIncident == nil {
			// Create new incident
			incident := &models.Incident{
				ID:          uuid.New(),
				ServiceID:   check.ServiceID,
				ServiceName: check.ServiceName,
				Title:       check.ServiceName + " is experiencing issues",
				Status:      "investigating",
				StartedAt:   check.CheckedAt,
			}
			h.incidents[incident.ID] = incident

			log.WithFields(log.Fields{
				"service":     check.ServiceName,
				"incident_id": incident.ID,
			}).Warn("New incident created")
		}
	} else if check.Status == models.StatusHealthy {
		if existingIncident != nil {
			// Resolve incident
			now := time.Now()
			existingIncident.Status = "resolved"
			existingIncident.ResolvedAt = &now

			log.WithFields(log.Fields{
				"service":     check.ServiceName,
				"incident_id": existingIncident.ID,
			}).Info("Incident resolved")
		}
	}
}

// Subscribe adds a subscriber for real-time updates
func (h *HealthChecker) Subscribe(id string) chan *models.HealthCheck {
	h.subMu.Lock()
	defer h.subMu.Unlock()

	ch := make(chan *models.HealthCheck, 100)
	h.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber
func (h *HealthChecker) Unsubscribe(id string) {
	h.subMu.Lock()
	defer h.subMu.Unlock()

	if ch, ok := h.subscribers[id]; ok {
		close(ch)
		delete(h.subscribers, id)
	}
}

// notifySubscribers sends health check to all subscribers
func (h *HealthChecker) notifySubscribers(check *models.HealthCheck) {
	h.subMu.RLock()
	defer h.subMu.RUnlock()

	for _, ch := range h.subscribers {
		select {
		case ch <- check:
		default:
			// Channel full, skip
		}
	}
}

// GetServices returns all services
func (h *HealthChecker) GetServices() []*models.Service {
	h.mu.RLock()
	defer h.mu.RUnlock()

	services := make([]*models.Service, 0, len(h.services))
	for _, svc := range h.services {
		services = append(services, svc)
	}
	return services
}

// GetServiceByID returns a service by ID
func (h *HealthChecker) GetServiceByID(id uuid.UUID) *models.Service {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.services[id]
}

// GetActiveIncidents returns all active (unresolved) incidents
func (h *HealthChecker) GetActiveIncidents() []models.Incident {
	h.mu.RLock()
	defer h.mu.RUnlock()

	incidents := make([]models.Incident, 0)
	for _, inc := range h.incidents {
		if inc.Status != "resolved" {
			incidents = append(incidents, *inc)
		}
	}

	// Sort by start time (newest first)
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].StartedAt.After(incidents[j].StartedAt)
	})

	return incidents
}

// GetOverallStats returns overall platform statistics
func (h *HealthChecker) GetOverallStats() *models.OverallStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := &models.OverallStats{
		TotalServices: len(h.services),
		LastUpdated:   time.Now(),
	}

	var totalResponseTime int64
	var totalSuccess, totalChecks int64

	for _, svc := range h.services {
		switch svc.Status {
		case models.StatusHealthy:
			stats.HealthyServices++
		case models.StatusDegraded:
			stats.DegradedServices++
		case models.StatusUnhealthy:
			stats.UnhealthyServices++
		default:
			stats.UnknownServices++
		}

		totalResponseTime += svc.ResponseTimeMs
		totalSuccess += svc.SuccessCount
		totalChecks += svc.TotalChecks
	}

	if len(h.services) > 0 {
		stats.AvgResponseMs = float64(totalResponseTime) / float64(len(h.services))
	}

	if totalChecks > 0 {
		stats.OverallUptime = float64(totalSuccess) / float64(totalChecks) * 100
	} else {
		stats.OverallUptime = 100.0
	}

	return stats
}

// GetServiceSummaries returns summaries for all services
func (h *HealthChecker) GetServiceSummaries() []models.ServiceSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	summaries := make([]models.ServiceSummary, 0, len(h.services))

	for _, svc := range h.services {
		uptime := 100.0
		if svc.TotalChecks > 0 {
			uptime = float64(svc.SuccessCount) / float64(svc.TotalChecks) * 100
		}

		summaries = append(summaries, models.ServiceSummary{
			ID:             svc.ID,
			Name:           svc.Name,
			DisplayName:    svc.DisplayName,
			Category:       svc.Category,
			Status:         svc.Status,
			Uptime30d:      uptime,
			SLATarget:      svc.SLATarget,
			SLAMet:         uptime >= svc.SLATarget,
			ResponseTimeMs: svc.ResponseTimeMs,
			LastCheckAt:    svc.LastCheckAt,
		})
	}

	// Sort by category then name
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Category != summaries[j].Category {
			return summaries[i].Category < summaries[j].Category
		}
		return summaries[i].DisplayName < summaries[j].DisplayName
	})

	return summaries
}
