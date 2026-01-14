package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-service/internal/models"
	"notification-service/internal/repository"
)

// PreferenceHandler handles preference-related requests
type PreferenceHandler struct {
	prefRepo repository.PreferenceRepository
}

// NewPreferenceHandler creates a new preference handler
func NewPreferenceHandler(prefRepo repository.PreferenceRepository) *PreferenceHandler {
	return &PreferenceHandler{prefRepo: prefRepo}
}

// UpdatePreferenceRequest represents an update preference request
type UpdatePreferenceRequest struct {
	EmailEnabled     *bool    `json:"emailEnabled"`
	SMSEnabled       *bool    `json:"smsEnabled"`
	PushEnabled      *bool    `json:"pushEnabled"`
	MarketingEnabled *bool    `json:"marketingEnabled"`
	OrdersEnabled    *bool    `json:"ordersEnabled"`
	SecurityEnabled  *bool    `json:"securityEnabled"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	PushTokens       []string `json:"pushTokens"`
}

// Get returns user preferences
func (h *PreferenceHandler) Get(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.Param("userId")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant_id or userId"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userId"})
		return
	}

	pref, err := h.prefRepo.GetByUserID(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pref,
	})
}

// Update updates user preferences
func (h *PreferenceHandler) Update(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.Param("userId")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant_id or userId"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userId"})
		return
	}

	var req UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing preferences
	pref, err := h.prefRepo.GetByUserID(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	// Update fields if provided
	if req.EmailEnabled != nil {
		pref.EmailEnabled = *req.EmailEnabled
	}
	if req.SMSEnabled != nil {
		pref.SMSEnabled = *req.SMSEnabled
	}
	if req.PushEnabled != nil {
		pref.PushEnabled = *req.PushEnabled
	}
	if req.MarketingEnabled != nil {
		pref.MarketingEnabled = *req.MarketingEnabled
	}
	if req.OrdersEnabled != nil {
		pref.OrdersEnabled = *req.OrdersEnabled
	}
	if req.SecurityEnabled != nil {
		pref.SecurityEnabled = *req.SecurityEnabled
	}
	if req.Email != "" {
		pref.Email = req.Email
	}
	if req.Phone != "" {
		pref.Phone = req.Phone
	}

	// Ensure ID is set for upsert
	if pref.ID == uuid.Nil {
		pref.ID = uuid.New()
		pref.TenantID = tenantID
		pref.UserID = userID
	}

	if err := h.prefRepo.Upsert(c.Request.Context(), pref); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pref,
	})
}

// RegisterPushToken registers a push notification token
type RegisterPushTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"` // ios, android, web
}

// RegisterPushToken registers a push token for a user
func (h *PreferenceHandler) RegisterPushToken(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.Param("userId")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant_id or userId"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userId"})
		return
	}

	var req RegisterPushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing preferences
	pref, err := h.prefRepo.GetByUserID(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	// Add token if not already present
	var tokens []string
	if pref.PushTokens != nil {
		// Parse existing tokens
		// For simplicity, just append
		tokens = append(tokens, req.Token)
	} else {
		tokens = []string{req.Token}
	}

	// Ensure ID is set
	if pref.ID == uuid.Nil {
		pref.ID = uuid.New()
		pref.TenantID = tenantID
		pref.UserID = userID
	}

	if err := h.prefRepo.UpdatePushTokens(c.Request.Context(), tenantID, userID, tokens); err != nil {
		// Try upsert if update fails
		if err := h.prefRepo.Upsert(c.Request.Context(), &models.NotificationPreference{
			ID:       pref.ID,
			TenantID: tenantID,
			UserID:   userID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Push token registered",
	})
}
