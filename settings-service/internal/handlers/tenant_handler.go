package handlers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"settings-service/internal/models"
	"settings-service/internal/repository"
)

// TenantHandler handles tenant-related API endpoints
type TenantHandler struct {
	repo   *repository.TenantRepository
	logger *logrus.Logger
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(repo *repository.TenantRepository) *TenantHandler {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	return &TenantHandler{
		repo:   repo,
		logger: logger,
	}
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

	tenant, err := h.repo.GetByID(tenantID)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get tenant")
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

	c.JSON(http.StatusOK, models.TenantAuditConfigResponse{
		Success: true,
		Data:    config,
	})
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
	tenants, err := h.repo.GetAllActive()
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

// buildAuditConfig constructs the TenantAuditConfig from a Tenant record
func (h *TenantHandler) buildAuditConfig(tenant *models.Tenant) models.TenantAuditConfig {
	// Get database configuration from environment variables
	// In production, each tenant could have its own database
	// For now, we use the shared audit database configuration
	dbConfig := h.getDatabaseConfig()

	return models.TenantAuditConfig{
		TenantID:       tenant.ID,
		ProductID:      tenant.Slug, // Using slug as product ID
		ProductName:    tenant.Name,
		VendorID:       tenant.Slug, // Using slug as vendor ID
		VendorName:     tenant.DisplayName,
		DatabaseConfig: dbConfig,
		IsActive:       tenant.Status == "active",
		CreatedAt:      tenant.CreatedAt,
		UpdatedAt:      tenant.UpdatedAt,
		Features:       h.getDefaultFeatures(tenant),
	}
}

// getDatabaseConfig returns the database configuration for audit logs
// Uses environment variables for configuration
func (h *TenantHandler) getDatabaseConfig() models.DatabaseConfig {
	port, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_PORT", "5432"))
	maxOpenConns, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_IDLE_CONNS", "5"))
	maxLifetime, _ := strconv.Atoi(getEnvOrDefault("AUDIT_DB_MAX_LIFETIME", "300"))

	return models.DatabaseConfig{
		Host:         getEnvOrDefault("AUDIT_DB_HOST", getEnvOrDefault("DB_HOST", "postgresql.database.svc.cluster.local")),
		Port:         port,
		User:         getEnvOrDefault("AUDIT_DB_USER", getEnvOrDefault("DB_USER", "tesserix_user")),
		Password:     getEnvOrDefault("AUDIT_DB_PASSWORD", getEnvOrDefault("DB_PASSWORD", "")),
		DatabaseName: getEnvOrDefault("AUDIT_DB_NAME", "audit_logs"),
		SSLMode:      getEnvOrDefault("AUDIT_DB_SSL_MODE", "require"),
		MaxOpenConns: maxOpenConns,
		MaxIdleConns: maxIdleConns,
		MaxLifetime:  maxLifetime,
	}
}

// getDefaultFeatures returns default audit feature settings based on tenant tier
func (h *TenantHandler) getDefaultFeatures(tenant *models.Tenant) models.TenantFeatures {
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
