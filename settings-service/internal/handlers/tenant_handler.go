package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"settings-service/internal/models"
)

// TenantHandler handles tenant-related API endpoints
type TenantHandler struct {
	tenantServiceURL string
	httpClient       *http.Client
	logger           *logrus.Logger
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler() *TenantHandler {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	tenantServiceURL := os.Getenv("TENANT_SERVICE_URL")
	if tenantServiceURL == "" {
		tenantServiceURL = "http://tenant-service:8080"
	}

	return &TenantHandler{
		tenantServiceURL: tenantServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// TenantServiceResponse is the response from tenant-service
type TenantServiceResponse struct {
	Success bool                   `json:"success"`
	Data    TenantServiceTenant    `json:"data"`
	Message string                 `json:"message,omitempty"`
}

// TenantServiceTenant is the tenant model from tenant-service
type TenantServiceTenant struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Subdomain       string    `json:"subdomain"`
	DisplayName     string    `json:"display_name"`
	BusinessType    string    `json:"business_type"`
	Industry        string    `json:"industry"`
	Status          string    `json:"status"`
	BusinessModel   string    `json:"business_model"`
	DefaultTimezone string    `json:"default_timezone"`
	DefaultCurrency string    `json:"default_currency"`
	PricingTier     string    `json:"pricing_tier"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TenantListResponse is the response for listing tenants
type TenantListResponse struct {
	Success bool                  `json:"success"`
	Data    []TenantServiceTenant `json:"data"`
}

// GetAuditConfig returns the audit configuration for a specific tenant
// @Summary Get tenant audit configuration
// @Description Returns database configuration and feature flags for audit logging
// @Tags tenants
// @Accept json
// @Produce json
// @Param id path string true "Tenant ID"
// @Success 200 {object} models.TenantAuditConfigResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/audit-config [get]
func (h *TenantHandler) GetAuditConfig(c *gin.Context) {
	tenantIDStr := c.Param("id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		h.logger.WithField("tenant_id", tenantIDStr).Error("Invalid tenant ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid tenant ID format",
		})
		return
	}

	// Call tenant-service to get tenant info
	tenant, err := h.getTenantFromService(tenantID.String())
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get tenant from tenant-service")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve tenant",
		})
		return
	}

	if tenant == nil {
		h.logger.WithField("tenant_id", tenantID).Warn("Tenant not found")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Tenant not found",
		})
		return
	}

	// Build the audit configuration response
	config := h.buildAuditConfig(tenant)

	h.logger.WithFields(logrus.Fields{
		"tenant_id":   tenantID,
		"tenant_name": tenant.Name,
	}).Info("Audit config retrieved successfully")

	// Return config directly without wrapper for internal service-to-service calls
	// The audit-service expects TenantInfo struct directly, not wrapped in {success, data}
	c.JSON(http.StatusOK, config)
}

// ListAuditEnabledTenants returns all tenants with audit logging enabled
// @Summary List audit-enabled tenants
// @Description Returns all active tenants with their audit configuration
// @Tags tenants
// @Accept json
// @Produce json
// @Success 200 {object} models.TenantAuditListResponse
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/audit-enabled [get]
func (h *TenantHandler) ListAuditEnabledTenants(c *gin.Context) {
	tenants, err := h.getActiveTenants()
	if err != nil {
		h.logger.WithError(err).Error("Failed to list active tenants")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve tenants",
		})
		return
	}

	configs := make([]models.TenantAuditConfig, 0, len(tenants))
	for i := range tenants {
		config := h.buildAuditConfig(&tenants[i])
		configs = append(configs, config)
	}

	h.logger.WithField("count", len(configs)).Info("Audit-enabled tenants listed")

	c.JSON(http.StatusOK, models.TenantAuditListResponse{
		Success: true,
		Data:    configs,
	})
}

// getTenantFromService calls tenant-service to get tenant info
// Uses the internal endpoint that's designed for service-to-service calls
func (h *TenantHandler) getTenantFromService(tenantID string) (*TenantServiceTenant, error) {
	url := fmt.Sprintf("%s/internal/tenants/%s", h.tenantServiceURL, tenantID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "settings-service")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tenant-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenant-service returned status %d", resp.StatusCode)
	}

	var response TenantServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("tenant-service returned error: %s", response.Message)
	}

	return &response.Data, nil
}

// getActiveTenants calls tenant-service to list all active tenants
func (h *TenantHandler) getActiveTenants() ([]TenantServiceTenant, error) {
	url := fmt.Sprintf("%s/api/v1/tenants?status=active", h.tenantServiceURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "settings-service")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tenant-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenant-service returned status %d", resp.StatusCode)
	}

	var response TenantListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Data, nil
}

// buildAuditConfig constructs the TenantAuditConfig from a Tenant record
func (h *TenantHandler) buildAuditConfig(tenant *TenantServiceTenant) models.TenantAuditConfig {
	// Get database configuration from environment variables
	// In production, each tenant could have its own database
	// For now, we use the shared audit database configuration
	dbConfig := h.getDatabaseConfig()

	tenantUUID, _ := uuid.Parse(tenant.ID)

	displayName := tenant.DisplayName
	if displayName == "" {
		displayName = tenant.Name
	}

	return models.TenantAuditConfig{
		TenantID:       tenantUUID,
		ProductID:      tenant.Slug, // Using slug as product ID
		ProductName:    tenant.Name,
		VendorID:       tenant.Slug, // Using slug as vendor ID
		VendorName:     displayName,
		DatabaseConfig: dbConfig,
		IsActive:       tenant.Status == "active",
		CreatedAt:      tenant.CreatedAt,
		UpdatedAt:      tenant.UpdatedAt,
		Features:       h.getDefaultFeatures(tenant),
	}
}

// getDatabaseConfig returns the database configuration for audit logs
// Uses environment variables for configuration, with database password from GCP Secret Manager
func (h *TenantHandler) getDatabaseConfig() models.DatabaseConfig {
	port, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_PORT", "5432"))
	maxOpenConns, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_IDLE_CONNS", "5"))
	maxLifetime, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_LIFETIME", "300"))

	// Get password from GCP Secret Manager (same as main database password)
	// This uses the go-shared secrets package which handles GCP Secret Manager loading
	password := getEnvOrDefault("AUDIT_DB_PASSWORD", "")
	if password == "" {
		// Fall back to the shared database password loaded from GCP Secret Manager
		password = secrets.GetDBPassword()
	}

	return models.DatabaseConfig{
		Host:         getEnvOrDefault("AUDIT_DB_HOST", getEnvOrDefault("DB_HOST", "postgresql.database.svc.cluster.local")),
		Port:         port,
		User:         getEnvOrDefault("AUDIT_DB_USER", getEnvOrDefault("DB_USER", "postgres")),
		Password:     password,
		DatabaseName: getEnvOrDefault("AUDIT_DB_NAME", "audit_logs"),
		SSLMode:      getEnvOrDefault("AUDIT_DB_SSL_MODE", "disable"),
		MaxOpenConns: maxOpenConns,
		MaxIdleConns: maxIdleConns,
		MaxLifetime:  maxLifetime,
	}
}

// getDefaultFeatures returns default audit feature settings based on tenant tier
func (h *TenantHandler) getDefaultFeatures(tenant *TenantServiceTenant) models.TenantFeatures {
	// Default features - can be customized per pricing tier
	features := models.TenantFeatures{
		AuditLogsEnabled: true,
		RealTimeEnabled:  true,
		ExportEnabled:    true,
		RetentionDays:    90,
		MaxLogsPerDay:    100000,
		EncryptionAtRest: true,
	}

	// Adjust based on pricing tier
	switch tenant.PricingTier {
	case "free":
		features.RetentionDays = 30
		features.MaxLogsPerDay = 10000
		features.ExportEnabled = false
	case "starter":
		features.RetentionDays = 60
		features.MaxLogsPerDay = 50000
	case "professional":
		features.RetentionDays = 90
		features.MaxLogsPerDay = 100000
	case "enterprise":
		features.RetentionDays = 365
		features.MaxLogsPerDay = -1 // Unlimited
	}

	return features
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
