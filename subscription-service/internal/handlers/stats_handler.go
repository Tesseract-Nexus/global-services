package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"subscription-service/internal/models"
	"subscription-service/internal/services"
)

type StatsHandler struct {
	service *services.SubscriptionService
}

func NewStatsHandler(service *services.SubscriptionService) *StatsHandler {
	return &StatsHandler{service: service}
}

func (h *StatsHandler) GetOverview(c *gin.Context) {
	stats, err := h.service.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch stats", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *StatsHandler) GetEnhancedStats(c *gin.Context) {
	stats, err := h.service.GetEnhancedStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch enhanced stats", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *StatsHandler) GetExpiringTrials(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	subs, err := h.service.GetExpiringTrials(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch expiring trials", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, subs)
}

func (h *StatsHandler) GetAdminInvoices(c *gin.Context) {
	status := c.Query("status")

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	result, err := h.service.GetAllInvoices(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch invoices", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
