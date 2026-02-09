package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"subscription-service/internal/models"
	"subscription-service/internal/services"
)

type SubscriptionHandler struct {
	service *services.SubscriptionService
}

func NewSubscriptionHandler(service *services.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Tenant ID required"})
		return
	}

	sub, err := h.service.GetSubscription(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Subscription not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) CreateCheckoutSession(c *gin.Context) {
	var req models.CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	resp, err := h.service.CreateCheckoutSession(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create checkout session", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SubscriptionHandler) CreatePortalSession(c *gin.Context) {
	var req models.PortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	resp, err := h.service.CreatePortalSession(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create portal session", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SubscriptionHandler) CancelSubscription(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Tenant ID required"})
		return
	}

	if err := h.service.CancelSubscription(c.Request.Context(), tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to cancel subscription", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Subscription will be canceled at period end"})
}

func (h *SubscriptionHandler) ReactivateSubscription(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Tenant ID required"})
		return
	}

	if err := h.service.ReactivateSubscription(c.Request.Context(), tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to reactivate subscription", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Subscription reactivated"})
}

func (h *SubscriptionHandler) ChangePlan(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Tenant ID required"})
		return
	}

	var req models.ChangePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	sub, err := h.service.ChangePlan(c.Request.Context(), tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to change plan", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) GetInvoices(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Tenant ID required"})
		return
	}

	invoices, err := h.service.GetInvoices(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch invoices", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoices)
}
