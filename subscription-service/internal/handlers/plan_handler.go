package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"subscription-service/internal/models"
	"subscription-service/internal/services"
)

type PlanHandler struct {
	service *services.PlanService
}

func NewPlanHandler(service *services.PlanService) *PlanHandler {
	return &PlanHandler{service: service}
}

func (h *PlanHandler) ListPlans(c *gin.Context) {
	activeOnly := c.Query("active") == "true"
	plans, err := h.service.ListPlans(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to list plans", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, plans)
}

func (h *PlanHandler) GetPlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid plan ID"})
		return
	}

	plan, err := h.service.GetPlan(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Plan not found"})
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *PlanHandler) CreatePlan(c *gin.Context) {
	var req models.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	plan, err := h.service.CreatePlan(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create plan", Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, plan)
}

func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid plan ID"})
		return
	}

	var req models.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	plan, err := h.service.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update plan", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *PlanHandler) DeletePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid plan ID"})
		return
	}

	if err := h.service.DeletePlan(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete plan", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plan deactivated"})
}

func (h *PlanHandler) SyncToStripe(c *gin.Context) {
	if err := h.service.SyncToStripe(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to sync plans to Stripe", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plans synced to Stripe"})
}
