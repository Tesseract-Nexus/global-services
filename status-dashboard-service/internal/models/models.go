package models

import (
	"time"

	"github.com/google/uuid"
)

// ServiceStatus represents the current status of a service
type ServiceStatus string

const (
	StatusHealthy   ServiceStatus = "healthy"
	StatusDegraded  ServiceStatus = "degraded"
	StatusUnhealthy ServiceStatus = "unhealthy"
	StatusUnknown   ServiceStatus = "unknown"
)

// Service represents a monitored service (in-memory)
type Service struct {
	ID             uuid.UUID     `json:"id"`
	Name           string        `json:"name"`
	DisplayName    string        `json:"displayName"`
	URL            string        `json:"url"`
	HealthPath     string        `json:"healthPath"`
	Category       string        `json:"category"`
	SLATarget      float64       `json:"slaTarget"`
	Status         ServiceStatus `json:"status"`
	LastCheckAt    *time.Time    `json:"lastCheckAt,omitempty"`
	ResponseTimeMs int64         `json:"responseTimeMs"`

	// In-memory stats tracking (rolling window)
	SuccessCount int64 `json:"-"`
	FailureCount int64 `json:"-"`
	TotalChecks  int64 `json:"-"`
}

// HealthCheck represents a single health check result
type HealthCheck struct {
	ServiceID      uuid.UUID     `json:"serviceId"`
	ServiceName    string        `json:"serviceName"`
	Status         ServiceStatus `json:"status"`
	ResponseTimeMs int64         `json:"responseTimeMs"`
	StatusCode     int           `json:"statusCode"`
	Error          string        `json:"error,omitempty"`
	CheckedAt      time.Time     `json:"checkedAt"`
}

// Incident represents a service incident (in-memory)
type Incident struct {
	ID          uuid.UUID  `json:"id"`
	ServiceID   uuid.UUID  `json:"serviceId"`
	ServiceName string     `json:"serviceName"`
	Title       string     `json:"title"`
	Status      string     `json:"status"` // investigating, monitoring, resolved
	StartedAt   time.Time  `json:"startedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

// OverallStats represents aggregate statistics
type OverallStats struct {
	TotalServices     int       `json:"totalServices"`
	HealthyServices   int       `json:"healthyServices"`
	DegradedServices  int       `json:"degradedServices"`
	UnhealthyServices int       `json:"unhealthyServices"`
	UnknownServices   int       `json:"unknownServices"`
	OverallUptime     float64   `json:"overallUptime"`
	AvgResponseMs     float64   `json:"avgResponseMs"`
	LastUpdated       time.Time `json:"lastUpdated"`
}

// ServiceSummary represents a summary of service health with SLA info
type ServiceSummary struct {
	ID             uuid.UUID     `json:"id"`
	Name           string        `json:"name"`
	DisplayName    string        `json:"displayName"`
	Category       string        `json:"category"`
	Status         ServiceStatus `json:"status"`
	Uptime30d      float64       `json:"uptime30d"`
	SLATarget      float64       `json:"slaTarget"`
	SLAMet         bool          `json:"slaMet"`
	ResponseTimeMs int64         `json:"responseTimeMs"`
	LastCheckAt    *time.Time    `json:"lastCheckAt,omitempty"`
}

// StatusResponse is the API response for status
type StatusResponse struct {
	Status      string           `json:"status"` // operational, degraded, outage
	Services    []ServiceSummary `json:"services"`
	Incidents   []Incident       `json:"incidents"`
	Stats       OverallStats     `json:"stats"`
	LastUpdated time.Time        `json:"lastUpdated"`
}
