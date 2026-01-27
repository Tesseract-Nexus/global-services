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
// This endpoint performs a dry-run check to see what would be migrated
func (h *MigrationHandler) CheckMigration(c *gin.Context) {
	h.logger.Info("checking for secrets that need naming migration")

	// Run migration with delete=false to just check
	result, err := h.migrationService.RunMigration(c.Request.Context(), false)
	if err != nil {
		h.logger.WithError(err).Error("migration check failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return as a check result without actually migrating
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"secrets_scanned":     result.SecretsScanned,
			"secrets_to_migrate":  result.SecretsMigrated, // These were actually migrated in dry-run
			"secrets_up_to_date":  result.SecretsSkipped,
			"potential_migrations": result.MigratedSecrets,
		},
	})
}
