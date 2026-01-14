package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-hub/settings-service/internal/models"
	"github.com/tesseract-hub/settings-service/internal/services"
)

// StorefrontThemeHandler handles HTTP requests for storefront theme settings
type StorefrontThemeHandler struct {
	service services.StorefrontThemeService
}

// NewStorefrontThemeHandler creates a new storefront theme handler
func NewStorefrontThemeHandler(service services.StorefrontThemeService) *StorefrontThemeHandler {
	return &StorefrontThemeHandler{service: service}
}

// parseTenantID parses tenant ID from path parameter or header, supporting both UUID and string formats
func parseTenantID(c *gin.Context) (uuid.UUID, error) {
	tenantIDStr := c.Param("tenantId")
	if tenantIDStr == "" {
		tenantIDStr = c.GetHeader("X-Tenant-ID")
	}

	if tenantIDStr == "" {
		return uuid.Nil, nil
	}

	// Try to parse as UUID first
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		// Generate a deterministic UUID from the tenant string
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		tenantID = uuid.NewSHA1(namespace, []byte(tenantIDStr))
	}

	return tenantID, nil
}

// parseStorefrontID parses storefront ID from path parameter or header
// The path param "tenantId" is actually used as storefrontId for backward compatibility
func parseStorefrontID(c *gin.Context) (uuid.UUID, error) {
	// For backward compatibility, the path param is still named "tenantId" but used as storefrontId
	storefrontIDStr := c.Param("tenantId")
	if storefrontIDStr == "" {
		storefrontIDStr = c.GetHeader("X-Storefront-ID")
	}

	if storefrontIDStr == "" {
		return uuid.Nil, nil
	}

	// Try to parse as UUID first
	storefrontID, err := uuid.Parse(storefrontIDStr)
	if err != nil {
		// Generate a deterministic UUID from the storefront string
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		storefrontID = uuid.NewSHA1(namespace, []byte(storefrontIDStr))
	}

	return storefrontID, nil
}

// getUserID extracts user ID from context
func getUserID(c *gin.Context) *uuid.UUID {
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			return &id
		}
	}
	return nil
}

// GetStorefrontTheme retrieves storefront theme settings
// @Summary Get storefront theme settings
// @Description Retrieve storefront theme settings for a specific storefront
// @Tags storefront-theme
// @Produce json
// @Param tenantId path string true "Storefront ID (path param named tenantId for backward compatibility)"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId} [get]
func (h *StorefrontThemeHandler) GetStorefrontTheme(c *gin.Context) {
	// Parse storefront ID (using path param named tenantId for backward compatibility)
	storefrontID, err := parseStorefrontID(c)
	if err != nil || storefrontID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Storefront ID is required",
		})
		return
	}

	// Get tenant ID from header if available (optional)
	tenantID, _ := parseTenantID(c)
	if tenantID == uuid.Nil {
		tenantID = storefrontID // Fallback for backward compatibility
	}

	// Try to get settings by storefront ID first
	settings, err := h.service.GetByStorefrontID(storefrontID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to get storefront theme settings: " + err.Error(),
		})
		return
	}

	// If we got defaults (no stored settings found), also try looking up by tenant ID
	// This handles the case where the path param is a tenant_id but settings are stored by storefront_id
	if settings.StorefrontID == uuid.Nil {
		tenantSettings, tenantErr := h.service.GetByTenantID(storefrontID)
		if tenantErr == nil && tenantSettings.StorefrontID != uuid.Nil {
			settings = tenantSettings
		}
	}

	// Set IDs if returning defaults
	if settings.StorefrontID == uuid.Nil {
		settings.StorefrontID = storefrontID
	}
	if settings.TenantID == uuid.Nil {
		settings.TenantID = tenantID
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
	})
}

// CreateOrUpdateStorefrontTheme creates or updates storefront theme settings
// @Summary Create or update storefront theme settings
// @Description Create new or update existing storefront theme settings
// @Tags storefront-theme
// @Accept json
// @Produce json
// @Param tenantId path string true "Storefront ID (path param named tenantId for backward compatibility)"
// @Param settings body models.CreateStorefrontThemeRequest true "Theme settings"
// @Success 200 {object} models.StorefrontThemeResponse
// @Success 201 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId} [post]
func (h *StorefrontThemeHandler) CreateOrUpdateStorefrontTheme(c *gin.Context) {
	// Parse storefront ID (using path param named tenantId for backward compatibility)
	storefrontID, err := parseStorefrontID(c)
	if err != nil || storefrontID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Storefront ID is required",
		})
		return
	}

	// Get tenant ID from header if available (optional)
	tenantID, _ := parseTenantID(c)
	if tenantID == uuid.Nil {
		tenantID = storefrontID // Fallback for backward compatibility
	}

	var req models.CreateStorefrontThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.CreateOrUpdateByStorefrontID(storefrontID, tenantID, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to save storefront theme settings: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
		Message: "Storefront theme settings saved successfully",
	})
}

// UpdateStorefrontTheme partially updates storefront theme settings
// @Summary Update storefront theme settings
// @Description Partially update storefront theme settings for a tenant
// @Tags storefront-theme
// @Accept json
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param settings body models.UpdateStorefrontThemeRequest true "Theme settings to update"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId} [patch]
func (h *StorefrontThemeHandler) UpdateStorefrontTheme(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	var req models.UpdateStorefrontThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.Update(tenantID, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to update storefront theme settings: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
		Message: "Storefront theme settings updated successfully",
	})
}

// DeleteStorefrontTheme deletes storefront theme settings (resets to defaults)
// @Summary Delete storefront theme settings
// @Description Delete storefront theme settings for a tenant (resets to defaults)
// @Tags storefront-theme
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId} [delete]
func (h *StorefrontThemeHandler) DeleteStorefrontTheme(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	err = h.service.Delete(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to delete storefront theme settings: " + err.Error(),
		})
		return
	}

	// Return default settings
	defaults := h.service.GetDefaults()
	defaults.TenantID = tenantID

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    defaults,
		Message: "Storefront theme settings reset to defaults",
	})
}

// GetThemePresets returns all available theme presets
// @Summary Get theme presets
// @Description Get all available storefront theme presets
// @Tags storefront-theme
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/storefront-theme/presets [get]
func (h *StorefrontThemeHandler) GetThemePresets(c *gin.Context) {
	presets := h.service.GetPresets()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    presets,
	})
}

// ApplyThemePreset applies a theme preset to a tenant's storefront
// @Summary Apply theme preset
// @Description Apply a theme preset to a tenant's storefront settings
// @Tags storefront-theme
// @Accept json
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param presetId path string true "Preset ID"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 404 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId}/apply-preset/{presetId} [post]
func (h *StorefrontThemeHandler) ApplyThemePreset(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	presetID := c.Param("presetId")
	if presetID == "" {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Preset ID is required",
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.ApplyPreset(tenantID, presetID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to apply theme preset: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
		Message: "Theme preset applied successfully",
	})
}

// CloneThemeRequest represents the request body for cloning theme settings
type CloneThemeRequest struct {
	SourceTenantID string `json:"sourceTenantId" binding:"required"`
}

// RestoreVersionRequest represents the request body for restoring a version
type RestoreVersionRequest struct {
	Version int `json:"version" binding:"required"`
}

// CloneTheme clones theme settings from one storefront to another
// @Summary Clone theme settings
// @Description Clone theme settings from a source storefront to the target storefront
// @Tags storefront-theme
// @Accept json
// @Produce json
// @Param tenantId path string true "Target Tenant ID"
// @Param request body CloneThemeRequest true "Clone request with source tenant ID"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 404 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId}/clone [post]
func (h *StorefrontThemeHandler) CloneTheme(c *gin.Context) {
	targetTenantID, err := parseTenantID(c)
	if err != nil || targetTenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Target Tenant ID is required",
		})
		return
	}

	var req CloneThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Parse source tenant ID
	var sourceTenantID uuid.UUID
	sourceTenantID, err = uuid.Parse(req.SourceTenantID)
	if err != nil {
		// Generate a deterministic UUID from the tenant string
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		sourceTenantID = uuid.NewSHA1(namespace, []byte(req.SourceTenantID))
	}

	userID := getUserID(c)

	settings, err := h.service.CloneTheme(sourceTenantID, targetTenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to clone theme settings: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
		Message: "Theme settings cloned successfully",
	})
}

// GetThemeHistory retrieves version history for a tenant's theme settings
// @Summary Get theme history
// @Description Get version history for theme settings
// @Tags storefront-theme
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param limit query int false "Number of history items to return (default 20)"
// @Success 200 {object} models.ThemeHistoryListResponse
// @Failure 400 {object} models.ThemeHistoryListResponse
// @Failure 500 {object} models.ThemeHistoryListResponse
// @Router /api/v1/storefront-theme/{tenantId}/history [get]
func (h *StorefrontThemeHandler) GetThemeHistory(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ThemeHistoryListResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	history, err := h.service.GetHistory(tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ThemeHistoryListResponse{
			Success: false,
			Message: "Failed to get theme history: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ThemeHistoryListResponse{
		Success: true,
		Data:    history,
		Total:   int64(len(history)),
	})
}

// GetThemeHistoryVersion retrieves a specific version from history
// @Summary Get specific history version
// @Description Get a specific version from theme history
// @Tags storefront-theme
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param version path int true "Version number"
// @Success 200 {object} models.ThemeHistoryResponse
// @Failure 400 {object} models.ThemeHistoryResponse
// @Failure 404 {object} models.ThemeHistoryResponse
// @Failure 500 {object} models.ThemeHistoryResponse
// @Router /api/v1/storefront-theme/{tenantId}/history/{version} [get]
func (h *StorefrontThemeHandler) GetThemeHistoryVersion(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ThemeHistoryResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ThemeHistoryResponse{
			Success: false,
			Message: "Invalid version number",
		})
		return
	}

	history, err := h.service.GetHistoryVersion(tenantID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ThemeHistoryResponse{
			Success: false,
			Message: "Version not found: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ThemeHistoryResponse{
		Success: true,
		Data:    history,
	})
}

// RestoreThemeVersion restores settings from a historical version
// @Summary Restore theme version
// @Description Restore theme settings from a historical version
// @Tags storefront-theme
// @Accept json
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param version path int true "Version number to restore"
// @Success 200 {object} models.StorefrontThemeResponse
// @Failure 400 {object} models.StorefrontThemeResponse
// @Failure 404 {object} models.StorefrontThemeResponse
// @Failure 500 {object} models.StorefrontThemeResponse
// @Router /api/v1/storefront-theme/{tenantId}/restore/{version} [post]
func (h *StorefrontThemeHandler) RestoreThemeVersion(c *gin.Context) {
	tenantID, err := parseTenantID(c)
	if err != nil || tenantID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Tenant ID is required",
		})
		return
	}

	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.StorefrontThemeResponse{
			Success: false,
			Message: "Invalid version number",
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.RestoreVersion(tenantID, version, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.StorefrontThemeResponse{
			Success: false,
			Message: "Failed to restore version: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StorefrontThemeResponse{
		Success: true,
		Data:    settings,
		Message: "Settings restored from version " + versionStr,
	})
}
