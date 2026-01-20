package handlers

import (
	"net/http"

	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"
	"custom-domain-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// InternalHandlers handles internal service-to-service requests
type InternalHandlers struct {
	domainService *services.DomainService
}

// NewInternalHandlers creates new internal handlers
func NewInternalHandlers(domainService *services.DomainService) *InternalHandlers {
	return &InternalHandlers{
		domainService: domainService,
	}
}

// ResolveDomain handles GET /api/v1/internal/resolve
// @Summary Resolve domain
// @Description Resolve a custom domain to tenant information (internal use only)
// @Tags internal
// @Produce json
// @Param domain query string true "Domain name to resolve"
// @Success 200 {object} models.InternalResolveResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/internal/resolve [get]
func (h *InternalHandlers) ResolveDomain(c *gin.Context) {
	domainName := c.Query("domain")
	if domainName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "domain parameter is required",
			Code:  "MISSING_DOMAIN",
		})
		return
	}

	resolved, err := h.domainService.ResolveDomain(c.Request.Context(), domainName)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Str("domain", domainName).Msg("Failed to resolve domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to resolve domain",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, resolved)
}

// CheckDomain handles GET /api/v1/internal/check
// @Summary Check if domain exists
// @Description Check if a domain is registered in the system (internal use only)
// @Tags internal
// @Produce json
// @Param domain query string true "Domain name to check"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/internal/check [get]
func (h *InternalHandlers) CheckDomain(c *gin.Context) {
	domainName := c.Query("domain")
	if domainName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "domain parameter is required",
			Code:  "MISSING_DOMAIN",
		})
		return
	}

	resolved, err := h.domainService.ResolveDomain(c.Request.Context(), domainName)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusOK, gin.H{
				"exists":    false,
				"domain":    domainName,
				"is_active": false,
			})
			return
		}
		log.Error().Err(err).Str("domain", domainName).Msg("Failed to check domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to check domain",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exists":      true,
		"domain":      domainName,
		"is_active":   resolved.IsActive,
		"tenant_id":   resolved.TenantID,
		"tenant_slug": resolved.TenantSlug,
		"target_type": resolved.TargetType,
	})
}

// Health handles GET /health
// @Summary Health check
// @Description Service health check endpoint
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *InternalHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "custom-domain-service",
	})
}

// Ready handles GET /ready
// @Summary Readiness check
// @Description Service readiness check endpoint
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ready [get]
func (h *InternalHandlers) Ready(c *gin.Context) {
	// TODO: Add database connectivity check
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
