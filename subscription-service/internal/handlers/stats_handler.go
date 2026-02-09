package handlers

import (
	"net/http"

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
