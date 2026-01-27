package handlers

import (
	"net/http"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MigrationHandler handles secret naming migration endpoints
type MigrationHandler struct {
	migrationService *services.NamingMigrationService
	logger           *logrus.Entry
}

// NewMigrationHandler creates a new migration handler
func NewMigrationHandler(migrationService *services.NamingMigrationService, logger *logrus.Entry) *MigrationHandler {
	return &MigrationHandler{
		migrationService: migrationService,
		logger:           logger.WithField("handler", "migration"),
	}
}

// RunMigrationRequest is the request body for running a migration
type RunMigrationRequest struct {
	DeleteOldSecrets bool `json:"delete_old_secrets"`
}

// RunMigration handles POST /api/v1/admin/migration/naming
// This endpoint triggers the naming migration to fix underscore -> hyphen naming
func (h *MigrationHandler) RunMigration(c *gin.Context) {
	var req RunMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to not deleting old secrets if body is empty/invalid
		req.DeleteOldSecrets = false
	}

	h.logger.WithField("delete_old_secrets", req.DeleteOldSecrets).Info("starting naming migration")

	result, err := h.migrationService.RunMigration(c.Request.Context(), req.DeleteOldSecrets)
	if err != nil {
		h.logger.WithError(err).Error("migration failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// CheckMigration handles GET /api/v1/admin/migration/naming/check
// This endpoint performs a true dry-run check without making any changes
func (h *MigrationHandler) CheckMigration(c *gin.Context) {
	h.logger.Info("checking for secrets that need naming migration (dry-run)")

	// Use the dedicated dry-run method that doesn't make any changes
	result, err := h.migrationService.CheckMigration(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("migration check failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"dry_run":              true,
			"secrets_scanned":      result.SecretsScanned,
			"secrets_up_to_date":   result.SecretsSkipped,
			"pending_migrations":   result.PendingMigrations,
			"migration_count":      len(result.PendingMigrations),
			"errors":               result.Errors,
		},
	})
}
