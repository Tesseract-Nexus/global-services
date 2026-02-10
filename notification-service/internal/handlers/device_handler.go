package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-service/internal/models"
	"notification-service/internal/repository"
)

// DeviceHandler handles device registration for push notifications
type DeviceHandler struct {
	prefRepo repository.PreferenceRepository
}

// NewDeviceHandler creates a new device handler
func NewDeviceHandler(prefRepo repository.PreferenceRepository) *DeviceHandler {
	return &DeviceHandler{prefRepo: prefRepo}
}

// RegisterDeviceRequest represents a push subscription registration
type RegisterDeviceRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh" binding:"required"`
		Auth   string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
	Platform string `json:"platform"`
}

// UnregisterDeviceRequest represents a push subscription removal
type UnregisterDeviceRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
}

// Register registers a new push subscription for a user
func (h *DeviceHandler) Register(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		userIDStr = c.GetString("user_id")
	}
	// Also check Istio JWT claim headers
	if userIDStr == "" {
		userIDStr = c.GetHeader("x-jwt-claim-sub")
	}
	if tenantID == "" {
		tenantID = c.GetHeader("x-jwt-claim-tenant-id")
	}

	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var req RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Platform == "" {
		req.Platform = "web"
	}

	ctx := c.Request.Context()

	// Get existing preferences
	pref, err := h.prefRepo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	// Parse existing push subscriptions
	var subs []models.PushSubscription
	if pref.PushSubscriptions != nil {
		json.Unmarshal(pref.PushSubscriptions, &subs)
	}

	// Deduplicate by endpoint (same browser = same endpoint)
	for _, s := range subs {
		if s.Endpoint == req.Endpoint {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Subscription already registered"})
			return
		}
	}

	// Append new subscription
	newSub := models.PushSubscription{
		Endpoint: req.Endpoint,
		Keys: models.PushSubscriptionKeys{
			P256dh: req.Keys.P256dh,
			Auth:   req.Keys.Auth,
		},
		Platform:  req.Platform,
		CreatedAt: time.Now(),
	}
	subs = append(subs, newSub)

	subsJSON, err := json.Marshal(subs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal subscriptions"})
		return
	}

	// Ensure ID is set for upsert
	if pref.ID == uuid.Nil {
		pref.ID = uuid.New()
		pref.TenantID = tenantID
		pref.UserID = userID
	}

	pref.PushSubscriptions = subsJSON
	pref.PushEnabled = true

	if err := h.prefRepo.Upsert(ctx, pref); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Push subscription registered"})
}

// Unregister removes a push subscription for a user
func (h *DeviceHandler) Unregister(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		userIDStr = c.GetString("user_id")
	}
	if userIDStr == "" {
		userIDStr = c.GetHeader("x-jwt-claim-sub")
	}
	if tenantID == "" {
		tenantID = c.GetHeader("x-jwt-claim-tenant-id")
	}

	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var req UnregisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Get existing preferences
	pref, err := h.prefRepo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get preferences"})
		return
	}

	// Parse existing push subscriptions
	var subs []models.PushSubscription
	if pref.PushSubscriptions != nil {
		json.Unmarshal(pref.PushSubscriptions, &subs)
	}

	// Remove matching subscription
	var filtered []models.PushSubscription
	for _, s := range subs {
		if s.Endpoint != req.Endpoint {
			filtered = append(filtered, s)
		}
	}

	subsJSON, err := json.Marshal(filtered)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal subscriptions"})
		return
	}

	pref.PushSubscriptions = subsJSON

	if err := h.prefRepo.Upsert(ctx, pref); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Push subscription removed"})
}
