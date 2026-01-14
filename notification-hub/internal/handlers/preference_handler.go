package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/models"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/repository"
)

// PreferenceHandler handles preference-related HTTP requests
type PreferenceHandler struct {
	prefRepo repository.PreferenceRepository
}

// NewPreferenceHandler creates a new preference handler
func NewPreferenceHandler(prefRepo repository.PreferenceRepository) *PreferenceHandler {
	return &PreferenceHandler{
		prefRepo: prefRepo,
	}
}

// Get returns the user's notification preferences
func (h *PreferenceHandler) Get(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Get or create preferences
	preference, err := h.prefRepo.GetOrCreate(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    preference,
	})
}

// Update updates the user's notification preferences
func (h *PreferenceHandler) Update(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var req UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing preferences
	preference, err := h.prefRepo.GetOrCreate(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	// Update fields if provided
	if req.WebSocketEnabled != nil {
		preference.WebSocketEnabled = *req.WebSocketEnabled
	}
	if req.SSEEnabled != nil {
		preference.SSEEnabled = *req.SSEEnabled
	}
	if req.SoundEnabled != nil {
		preference.SoundEnabled = *req.SoundEnabled
	}
	if req.VibrationEnabled != nil {
		preference.VibrationEnabled = *req.VibrationEnabled
	}
	if req.GroupSimilar != nil {
		preference.GroupSimilar = *req.GroupSimilar
	}
	if req.QuietHoursEnabled != nil {
		preference.QuietHoursEnabled = *req.QuietHoursEnabled
	}
	if req.QuietHoursStart != nil {
		preference.QuietHoursStart = *req.QuietHoursStart
	}
	if req.QuietHoursEnd != nil {
		preference.QuietHoursEnd = *req.QuietHoursEnd
	}
	if req.CategoryPreferences != nil {
		preference.CategoryPreferences = models.JSONB(req.CategoryPreferences)
	}

	// Save preferences
	if err := h.prefRepo.Update(c.Request.Context(), preference); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Preferences updated",
		"data":    preference,
	})
}

// Reset resets the user's notification preferences to defaults
func (h *PreferenceHandler) Reset(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Delete existing preferences
	if err := h.prefRepo.Delete(c.Request.Context(), tenantID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset preferences"})
		return
	}

	// Get new default preferences
	preference, err := h.prefRepo.GetOrCreate(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Preferences reset to defaults",
		"data":    preference,
	})
}

// UpdatePreferenceRequest represents the request body for updating preferences
type UpdatePreferenceRequest struct {
	WebSocketEnabled    *bool                  `json:"websocket_enabled"`
	SSEEnabled          *bool                  `json:"sse_enabled"`
	SoundEnabled        *bool                  `json:"sound_enabled"`
	VibrationEnabled    *bool                  `json:"vibration_enabled"`
	GroupSimilar        *bool                  `json:"group_similar"`
	QuietHoursEnabled   *bool                  `json:"quiet_hours_enabled"`
	QuietHoursStart     *string                `json:"quiet_hours_start"`
	QuietHoursEnd       *string                `json:"quiet_hours_end"`
	CategoryPreferences map[string]interface{} `json:"category_preferences"`
}
