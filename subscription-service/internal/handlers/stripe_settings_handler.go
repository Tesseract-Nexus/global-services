package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"subscription-service/internal/models"
	"subscription-service/internal/services"
)

type StripeSettingsHandler struct {
	service *services.StripeSettingsService
}

func NewStripeSettingsHandler(service *services.StripeSettingsService) *StripeSettingsHandler {
	return &StripeSettingsHandler{service: service}
}

func (h *StripeSettingsHandler) GetStripeSettings(c *gin.Context) {
	result := h.service.GetStripeStatus(c.Request.Context())
	c.JSON(http.StatusOK, result)
}

func (h *StripeSettingsHandler) UpdateStripeKeys(c *gin.Context) {
	var req models.UpdateStripeKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.SecretKey == nil && req.WebhookSecret == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "At least one key must be provided"})
		return
	}

	if err := h.service.UpdateStripeKeys(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stripe keys updated successfully"})
}

func (h *StripeSettingsHandler) VerifyStripeKey(c *gin.Context) {
	var req models.VerifyStripeKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Secret key is required"})
		return
	}

	result := h.service.VerifyStripeKey(c.Request.Context(), req.SecretKey)
	c.JSON(http.StatusOK, result)
}

func (h *StripeSettingsHandler) ReloadStripeKeys(c *gin.Context) {
	result := h.service.ReloadKeys(c.Request.Context())
	c.JSON(http.StatusOK, result)
}
