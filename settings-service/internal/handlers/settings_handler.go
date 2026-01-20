package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"settings-service/internal/models"
	"settings-service/internal/repository"
	"settings-service/internal/services"
)

type SettingsHandler struct {
	settingsService services.SettingsService
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(settingsService services.SettingsService) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
	}
}

// ==========================================
// SETTINGS HANDLERS
// ==========================================

// CreateSettings creates new settings
// @Summary Create new settings
// @Description Create new settings for a specific context
// @Tags settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-User-ID header string false "User ID"
// @Param settings body models.CreateSettingsRequest true "Settings data"
// @Success 201 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings [post]
func (h *SettingsHandler) CreateSettings(c *gin.Context) {
	// First, parse as a flexible structure that can handle string IDs
	var rawReq struct {
		Context struct {
			TenantID      interface{} `json:"tenantId"`
			ApplicationID interface{} `json:"applicationId"`
			UserID        interface{} `json:"userId,omitempty"`
			Scope         string      `json:"scope"`
		} `json:"context"`
		Branding        *models.BrandingSettings     `json:"branding,omitempty"`
		Theme           *models.ThemeSettings        `json:"theme,omitempty"`
		Layout          *models.LayoutSettings       `json:"layout,omitempty"`
		Animations      *models.AnimationSettings    `json:"animations,omitempty"`
		Localization    *models.LocalizationSettings `json:"localization,omitempty"`
		Ecommerce       map[string]interface{}       `json:"ecommerce,omitempty"`
		Security        map[string]interface{}       `json:"security,omitempty"`
		Notifications   map[string]interface{}       `json:"notifications,omitempty"`
		Marketing       map[string]interface{}       `json:"marketing,omitempty"`
		Integrations    map[string]interface{}       `json:"integrations,omitempty"`
		Performance     map[string]interface{}       `json:"performance,omitempty"`
		Compliance      map[string]interface{}       `json:"compliance,omitempty"`
		Features        *models.FeatureSettings      `json:"features,omitempty"`
		UserPreferences *models.UserPreferences      `json:"userPreferences,omitempty"`
		Application     *models.ApplicationSettings  `json:"application,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}
	
	// Convert string/UUID tenantID
	var tenantID uuid.UUID
	switch v := rawReq.Context.TenantID.(type) {
	case string:
		if parsed, err := uuid.Parse(v); err == nil {
			tenantID = parsed
		} else {
			// Generate deterministic UUID from string
			namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
			tenantID = uuid.NewSHA1(namespace, []byte(v))
		}
	default:
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "tenantId must be a string or UUID",
		})
		return
	}
	
	// Convert string/UUID applicationID
	var applicationID uuid.UUID
	switch v := rawReq.Context.ApplicationID.(type) {
	case string:
		if parsed, err := uuid.Parse(v); err == nil {
			applicationID = parsed
		} else {
			// Generate deterministic UUID from string
			namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
			applicationID = uuid.NewSHA1(namespace, []byte(v))
		}
	default:
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "applicationId must be a string or UUID",
		})
		return
	}
	
	// Convert userID if provided
	var userID *uuid.UUID
	if rawReq.Context.UserID != nil {
		switch v := rawReq.Context.UserID.(type) {
		case string:
			if parsed, err := uuid.Parse(v); err == nil {
				userID = &parsed
			} else {
				// Generate deterministic UUID from string
				namespace := uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8")
				parsed = uuid.NewSHA1(namespace, []byte(v))
				userID = &parsed
			}
		}
	}
	
	// Create the proper request structure
	req := models.CreateSettingsRequest{
		Context: models.SettingsContext{
			TenantID:      tenantID,
			ApplicationID: applicationID,
			UserID:        userID,
			Scope:         rawReq.Context.Scope,
		},
		Branding:        rawReq.Branding,
		Theme:           rawReq.Theme,
		Layout:          rawReq.Layout,
		Animations:      rawReq.Animations,
		Localization:    rawReq.Localization,
		Ecommerce:       rawReq.Ecommerce,
		Security:        rawReq.Security,
		Notifications:   rawReq.Notifications,
		Marketing:       rawReq.Marketing,
		Integrations:    rawReq.Integrations,
		Performance:     rawReq.Performance,
		Compliance:      rawReq.Compliance,
		Features:        rawReq.Features,
		UserPreferences: rawReq.UserPreferences,
		Application:     rawReq.Application,
	}
	
	// Get auth user ID from context (set by auth middleware)
	var authUserID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			authUserID = &id
		}
	}
	
	settings, err := h.settingsService.CreateSettings(&req, authUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to create settings: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, models.SettingsResponse{
		Success: true,
		Data:    *settings,
		Message: "Settings created successfully",
	})
}

// GetSettings retrieves settings by ID
// @Summary Get settings by ID
// @Description Retrieve settings by their unique identifier
// @Tags settings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Settings ID"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Router /api/v1/settings/{id} [get]
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid settings ID format",
		})
		return
	}
	
	settings, err := h.settingsService.GetSettings(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.SettingsResponse{
			Success: false,
			Message: "Settings not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
	})
}

// GetSettingsByContext retrieves settings by context
// @Summary Get settings by context
// @Description Retrieve settings by tenant, application, user, and scope
// @Tags settings
// @Produce json
// @Param applicationId query string true "Application ID"
// @Param tenantId query string false "Tenant ID (optional, uses JWT claim if not provided)"
// @Param userId query string false "User ID"
// @Param scope query string true "Settings scope"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Router /api/v1/settings/context [get]
func (h *SettingsHandler) GetSettingsByContext(c *gin.Context) {
	// Get tenant ID from gin context (set by IstioAuth middleware from JWT claims)
	// Falls back to query parameter for flexibility (e.g., storefront-specific settings)
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		// Try query parameter as fallback (for storefront-specific context)
		tenantIDStr = c.Query("tenantId")
	}
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Tenant ID is required (from JWT claim or tenantId query parameter)",
		})
		return
	}
	
	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		// Generate a deterministic UUID from the tenant string
		// This allows string-based tenant IDs to work consistently
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8") // Different namespace for tenants
		tenantID = uuid.NewSHA1(namespace, []byte(tenantIDStr))
	}
	
	applicationIDStr := c.Query("applicationId")
	if applicationIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "applicationId query parameter is required",
		})
		return
	}
	
	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		// Generate a deterministic UUID from the application string
		// This allows string-based application IDs to work consistently
		namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // UUID namespace for DNS
		applicationID = uuid.NewSHA1(namespace, []byte(applicationIDStr))
	}
	
	scope := c.Query("scope")
	if scope == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "scope query parameter is required",
		})
		return
	}
	
	context := models.SettingsContext{
		TenantID:      tenantID,
		ApplicationID: applicationID,
		Scope:         scope,
	}
	
	// Handle optional user ID
	if userIDStr := c.Query("userId"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.SettingsResponse{
				Success: false,
				Message: "Invalid user ID format",
			})
			return
		}
		context.UserID = &userID
	}
	
	settings, err := h.settingsService.GetSettingsByContext(context)
	if err != nil {
		c.JSON(http.StatusNotFound, models.SettingsResponse{
			Success: false,
			Message: "Settings not found for this context",
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
	})
}

// GetInheritedSettings retrieves inherited settings by context
// @Summary Get inherited settings
// @Description Retrieve settings with inheritance fallback
// @Tags settings
// @Produce json
// @Param applicationId query string true "Application ID"
// @Param tenantId query string false "Tenant ID (optional, uses JWT claim if not provided)"
// @Param userId query string false "User ID"
// @Param scope query string true "Settings scope"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Router /api/v1/settings/inherited [get]
func (h *SettingsHandler) GetInheritedSettings(c *gin.Context) {
	// Get tenant ID from gin context (set by IstioAuth middleware from JWT claims)
	// Falls back to query parameter for flexibility
	tenantIDStr := c.GetString("tenant_id")
	if tenantIDStr == "" {
		tenantIDStr = c.Query("tenantId")
	}
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Tenant ID is required (from JWT claim or tenantId query parameter)",
		})
		return
	}
	
	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		// Generate a deterministic UUID from the tenant string
		// This allows string-based tenant IDs to work consistently
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8") // Different namespace for tenants
		tenantID = uuid.NewSHA1(namespace, []byte(tenantIDStr))
	}
	
	applicationIDStr := c.Query("applicationId")
	if applicationIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "applicationId query parameter is required",
		})
		return
	}
	
	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		// Generate a deterministic UUID from the application string
		// This allows string-based application IDs to work consistently
		namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // UUID namespace for DNS
		applicationID = uuid.NewSHA1(namespace, []byte(applicationIDStr))
	}
	
	scope := c.Query("scope")
	if scope == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "scope query parameter is required",
		})
		return
	}
	
	context := models.SettingsContext{
		TenantID:      tenantID,
		ApplicationID: applicationID,
		Scope:         scope,
	}
	
	// Handle optional user ID
	if userIDStr := c.Query("userId"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.SettingsResponse{
				Success: false,
				Message: "Invalid user ID format",
			})
			return
		}
		context.UserID = &userID
	}
	
	settings, err := h.settingsService.GetInheritedSettings(context)
	if err != nil {
		c.JSON(http.StatusNotFound, models.SettingsResponse{
			Success: false,
			Message: "No settings found for this context",
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
	})
}

// UpdateSettings updates existing settings
// @Summary Update settings
// @Description Update existing settings by ID
// @Tags settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-User-ID header string false "User ID"
// @Param id path string true "Settings ID"
// @Param settings body models.UpdateSettingsRequest true "Updated settings data"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings/{id} [put]
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid settings ID format",
		})
		return
	}
	
	var req models.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}
	
	// Get user ID from context
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &id
		}
	}
	
	settings, err := h.settingsService.UpdateSettings(id, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to update settings: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
		Message: "Settings updated successfully",
	})
}

// DeleteSettings deletes settings by ID
// @Summary Delete settings
// @Description Delete settings by their unique identifier
// @Tags settings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-User-ID header string false "User ID"
// @Param id path string true "Settings ID"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings/{id} [delete]
func (h *SettingsHandler) DeleteSettings(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid settings ID format",
		})
		return
	}
	
	// Get user ID from context
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &id
		}
	}
	
	err = h.settingsService.DeleteSettings(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to delete settings: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Message: "Settings deleted successfully",
	})
}

// ListSettings lists settings with filtering
// @Summary List settings
// @Description List settings with optional filtering
// @Tags settings
// @Produce json
// @Param applicationId query string false "Application ID filter"
// @Param userId query string false "User ID filter"
// @Param scope query string false "Scope filter"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param sortBy query string false "Sort field"
// @Param sortOrder query string false "Sort order (ASC/DESC)"
// @Success 200 {object} models.SettingsListResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings [get]
func (h *SettingsHandler) ListSettings(c *gin.Context) {
	// Parse tenant ID from gin context (set by IstioAuth middleware from JWT claims)
	tenantIDStr := c.GetString("tenant_id")
	var tenantID *uuid.UUID
	if tenantIDStr != "" {
		if id, err := uuid.Parse(tenantIDStr); err == nil {
			tenantID = &id
		} else {
			// Try to generate deterministic UUID from string tenant ID
			namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
			parsed := uuid.NewSHA1(namespace, []byte(tenantIDStr))
			tenantID = &parsed
		}
	}
	
	// Build filters
	filters := repository.SettingsFilters{
		TenantID: tenantID,
		Page:     1,
		Limit:    20,
		SortBy:   "created_at",
		SortOrder: "DESC",
	}
	
	// Parse optional filters
	if applicationIDStr := c.Query("applicationId"); applicationIDStr != "" {
		if id, err := uuid.Parse(applicationIDStr); err == nil {
			filters.ApplicationID = &id
		}
	}
	
	if userIDStr := c.Query("userId"); userIDStr != "" {
		if id, err := uuid.Parse(userIDStr); err == nil {
			filters.UserID = &id
		}
	}
	
	if scope := c.Query("scope"); scope != "" {
		filters.Scope = &scope
	}
	
	// Parse pagination
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filters.Limit = limit
		}
	}
	
	// Parse sorting
	if sortBy := c.Query("sortBy"); sortBy != "" {
		filters.SortBy = sortBy
	}
	
	if sortOrder := c.Query("sortOrder"); sortOrder != "" {
		filters.SortOrder = sortOrder
	}
	
	settings, total, err := h.settingsService.ListSettings(filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to list settings: " + err.Error(),
		})
		return
	}
	
	// Calculate pagination info
	totalPages := int((total + int64(filters.Limit) - 1) / int64(filters.Limit))
	hasNext := filters.Page < totalPages
	hasPrev := filters.Page > 1
	
	response := models.SettingsListResponse{
		Success: true,
		Data:    settings,
		Pagination: &struct {
			Page       int   `json:"page"`
			Limit      int   `json:"limit"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"totalPages"`
			HasNext    bool  `json:"hasNext"`
			HasPrev    bool  `json:"hasPrevious"`
		}{
			Page:       filters.Page,
			Limit:      filters.Limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    hasNext,
			HasPrev:    hasPrev,
		},
	}
	
	c.JSON(http.StatusOK, response)
}

// ==========================================
// PRESET HANDLERS
// ==========================================

// ListPresets lists available settings presets
// @Summary List settings presets
// @Description List available settings presets with optional filtering
// @Tags presets
// @Produce json
// @Param category query string false "Preset category filter"
// @Param isDefault query bool false "Filter by default presets"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} models.PresetListResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/presets [get]
func (h *SettingsHandler) ListPresets(c *gin.Context) {
	filters := repository.PresetFilters{
		Page:      1,
		Limit:     20,
		SortBy:    "created_at",
		SortOrder: "DESC",
	}
	
	// Parse filters
	if category := c.Query("category"); category != "" {
		filters.Category = &category
	}
	
	if isDefaultStr := c.Query("isDefault"); isDefaultStr != "" {
		if isDefault, err := strconv.ParseBool(isDefaultStr); err == nil {
			filters.IsDefault = &isDefault
		}
	}
	
	// Parse pagination
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filters.Limit = limit
		}
	}
	
	presets, total, err := h.settingsService.ListPresets(filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to list presets: " + err.Error(),
		})
		return
	}
	
	// Calculate pagination info
	totalPages := int((total + int64(filters.Limit) - 1) / int64(filters.Limit))
	hasNext := filters.Page < totalPages
	hasPrev := filters.Page > 1
	
	response := models.PresetListResponse{
		Success: true,
		Data:    presets,
		Pagination: &struct {
			Page       int   `json:"page"`
			Limit      int   `json:"limit"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"totalPages"`
			HasNext    bool  `json:"hasNext"`
			HasPrev    bool  `json:"hasPrevious"`
		}{
			Page:       filters.Page,
			Limit:      filters.Limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    hasNext,
			HasPrev:    hasPrev,
		},
	}
	
	c.JSON(http.StatusOK, response)
}

// ApplyPreset applies a preset to existing settings
// @Summary Apply preset to settings
// @Description Apply a settings preset to existing settings
// @Tags presets
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-User-ID header string false "User ID"
// @Param settingsId path string true "Settings ID"
// @Param presetId path string true "Preset ID"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings/{settingsId}/apply-preset/{presetId} [post]
func (h *SettingsHandler) ApplyPreset(c *gin.Context) {
	settingsIDParam := c.Param("settingsId")
	settingsID, err := uuid.Parse(settingsIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid settings ID format",
		})
		return
	}
	
	presetIDParam := c.Param("presetId")
	presetID, err := uuid.Parse(presetIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid preset ID format",
		})
		return
	}
	
	// Get user ID from context
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if id, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &id
		}
	}
	
	settings, err := h.settingsService.ApplyPreset(settingsID, presetID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to apply preset: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
		Message: "Preset applied successfully",
	})
}

// GetSettingsHistory retrieves settings history
// @Summary Get settings history
// @Description Retrieve the change history for specific settings
// @Tags settings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Settings ID"
// @Param limit query int false "Maximum number of history entries"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Failure 500 {object} models.SettingsResponse
// @Router /api/v1/settings/{id}/history [get]
func (h *SettingsHandler) GetSettingsHistory(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "Invalid settings ID format",
		})
		return
	}
	
	limit := 50 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}
	
	history, err := h.settingsService.GetSettingsHistory(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SettingsResponse{
			Success: false,
			Message: "Failed to get settings history: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
	})
}

// GetPublicSettingsByContext returns settings for public storefront access
// This endpoint is used by storefronts to read marketing and localization settings
// without authentication. It reads tenantId from query parameter instead of X-Tenant-ID header.
// @Summary Get public settings by context
// @Description Get settings for a specific context without authentication (read-only)
// @Tags public
// @Accept json
// @Produce json
// @Param tenantId query string true "Tenant ID (storefront ID)"
// @Param applicationId query string true "Application ID"
// @Param scope query string true "Settings scope"
// @Success 200 {object} models.SettingsResponse
// @Failure 400 {object} models.SettingsResponse
// @Failure 404 {object} models.SettingsResponse
// @Router /api/v1/public/settings/context [get]
func (h *SettingsHandler) GetPublicSettingsByContext(c *gin.Context) {
	// Get tenantId from query parameter for public access
	tenantIDStr := c.Query("tenantId")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "tenantId query parameter is required",
		})
		return
	}

	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		// Generate a deterministic UUID from the tenant string
		namespace := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		tenantID = uuid.NewSHA1(namespace, []byte(tenantIDStr))
	}

	applicationIDStr := c.Query("applicationId")
	if applicationIDStr == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "applicationId query parameter is required",
		})
		return
	}

	// Try to parse as UUID first, if that fails, generate a deterministic UUID from the string
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
		applicationID = uuid.NewSHA1(namespace, []byte(applicationIDStr))
	}

	scope := c.Query("scope")
	if scope == "" {
		c.JSON(http.StatusBadRequest, models.SettingsResponse{
			Success: false,
			Message: "scope query parameter is required",
		})
		return
	}

	context := models.SettingsContext{
		TenantID:      tenantID,
		ApplicationID: applicationID,
		Scope:         scope,
	}

	settings, err := h.settingsService.GetSettingsByContext(context)
	if err != nil {
		// For public access, return empty response instead of error
		// This allows storefronts to gracefully handle missing settings
		c.JSON(http.StatusOK, models.SettingsResponse{
			Success: true,
			Message: "Settings not found for this context",
		})
		return
	}

	c.JSON(http.StatusOK, models.SettingsResponse{
		Success: true,
		Data:    *settings,
	})
}