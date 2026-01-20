package handlers

import (
	"net/http"
	"strconv"

	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"
	"custom-domain-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DomainHandlers handles HTTP requests for domain operations
type DomainHandlers struct {
	domainService *services.DomainService
}

// NewDomainHandlers creates new domain handlers
func NewDomainHandlers(domainService *services.DomainService) *DomainHandlers {
	return &DomainHandlers{
		domainService: domainService,
	}
}

// CreateDomain handles POST /api/v1/domains
// @Summary Create a new custom domain
// @Description Create a new custom domain for the authenticated tenant
// @Tags domains
// @Accept json
// @Produce json
// @Param request body models.CreateDomainRequest true "Domain creation request"
// @Success 201 {object} models.DomainResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/domains [post]
func (h *DomainHandlers) CreateDomain(c *gin.Context) {
	tenantID, userID, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	var req models.CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Code:    "INVALID_REQUEST",
			Message: "Please check your request data and try again",
		})
		return
	}

	domain, err := h.domainService.CreateDomain(c.Request.Context(), tenantID, &req, userID)
	if err != nil {
		switch err {
		case repository.ErrDomainAlreadyExists:
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "domain already exists",
				Code:    "DOMAIN_EXISTS",
				Message: "This domain is already registered",
			})
		case repository.ErrDomainLimitExceeded:
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "domain limit exceeded",
				Code:    "LIMIT_EXCEEDED",
				Message: "You have reached the maximum number of domains allowed for your account",
			})
		default:
			log.Error().Err(err).Msg("Failed to create domain")
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "failed to create domain",
				Code:  "INTERNAL_ERROR",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, domain)
}

// GetDomain handles GET /api/v1/domains/:id
// @Summary Get domain details
// @Description Get details of a specific custom domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.DomainResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/domains/{id} [get]
func (h *DomainHandlers) GetDomain(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	domain, err := h.domainService.GetDomain(c.Request.Context(), tenantID, domainID)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to get domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get domain",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, domain)
}

// ListDomains handles GET /api/v1/domains
// @Summary List domains
// @Description List all custom domains for the authenticated tenant
// @Tags domains
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} models.DomainListResponse
// @Router /api/v1/domains [get]
func (h *DomainHandlers) ListDomains(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	domains, err := h.domainService.ListDomains(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list domains")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to list domains",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// UpdateDomain handles PATCH /api/v1/domains/:id
// @Summary Update domain settings
// @Description Update settings for a custom domain
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param request body models.UpdateDomainRequest true "Update request"
// @Success 200 {object} models.DomainResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/domains/{id} [patch]
func (h *DomainHandlers) UpdateDomain(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	var req models.UpdateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid request",
			Code:    "INVALID_REQUEST",
			Message: "Please check your request data and try again",
		})
		return
	}

	domain, err := h.domainService.UpdateDomain(c.Request.Context(), tenantID, domainID, &req)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to update domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to update domain",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, domain)
}

// DeleteDomain handles DELETE /api/v1/domains/:id
// @Summary Delete a domain
// @Description Delete a custom domain and cleanup all resources
// @Tags domains
// @Param id path string true "Domain ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/domains/{id} [delete]
func (h *DomainHandlers) DeleteDomain(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	err = h.domainService.DeleteDomain(c.Request.Context(), tenantID, domainID)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to delete domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to delete domain",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: "Domain deleted successfully",
	})
}

// VerifyDomain handles POST /api/v1/domains/:id/verify
// @Summary Verify domain DNS
// @Description Trigger DNS verification for a domain
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param request body models.VerifyDomainRequest false "Verification options"
// @Success 200 {object} models.DNSStatusResponse
// @Router /api/v1/domains/{id}/verify [post]
func (h *DomainHandlers) VerifyDomain(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	var req models.VerifyDomainRequest
	c.ShouldBindJSON(&req) // Optional body

	status, err := h.domainService.VerifyDomain(c.Request.Context(), tenantID, domainID, req.Force)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to verify domain")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "verification failed",
			Code:  "VERIFICATION_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetDNSStatus handles GET /api/v1/domains/:id/dns
// @Summary Get DNS status
// @Description Get DNS verification status and required records
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.DNSStatusResponse
// @Router /api/v1/domains/{id}/dns [get]
func (h *DomainHandlers) GetDNSStatus(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	status, err := h.domainService.GetDNSStatus(c.Request.Context(), tenantID, domainID)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to get DNS status")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get DNS status",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetSSLStatus handles GET /api/v1/domains/:id/ssl
// @Summary Get SSL status
// @Description Get SSL certificate status for a domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.SSLStatusResponse
// @Router /api/v1/domains/{id}/ssl [get]
func (h *DomainHandlers) GetSSLStatus(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	status, err := h.domainService.GetSSLStatus(c.Request.Context(), tenantID, domainID)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to get SSL status")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get SSL status",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetStats handles GET /api/v1/domains/stats
// @Summary Get domain statistics
// @Description Get domain statistics for the tenant
// @Tags domains
// @Produce json
// @Success 200 {object} models.DomainStatsResponse
// @Router /api/v1/domains/stats [get]
func (h *DomainHandlers) GetStats(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	stats, err := h.domainService.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get stats")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get statistics",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// HealthCheck handles GET /api/v1/domains/:id/health
// @Summary Get domain health
// @Description Get health check status for a domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Success 200 {object} models.HealthCheckResponse
// @Router /api/v1/domains/{id}/health [get]
func (h *DomainHandlers) HealthCheck(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	health, err := h.domainService.HealthCheck(c.Request.Context(), tenantID, domainID)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to get health check")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get health status",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, health)
}

// GetActivities handles GET /api/v1/domains/:id/activities
// @Summary Get domain activities
// @Description Get activity log for a domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Param limit query int false "Limit" default(20)
// @Success 200 {array} models.DomainActivity
// @Router /api/v1/domains/{id}/activities [get]
func (h *DomainHandlers) GetActivities(c *gin.Context) {
	tenantID, _, err := getTenantAndUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	domainID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid domain ID",
			Code:  "INVALID_ID",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	activities, err := h.domainService.GetActivities(c.Request.Context(), tenantID, domainID, limit)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "domain not found",
				Code:  "NOT_FOUND",
			})
			return
		}
		log.Error().Err(err).Msg("Failed to get activities")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get activities",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// Helper function to extract tenant ID and user ID from context
func getTenantAndUserFromContext(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	// Get tenant ID from header (set by Istio/JWT middleware)
	tenantIDStr := c.GetHeader("x-tenant-id")
	if tenantIDStr == "" {
		tenantIDStr = c.GetHeader("X-Tenant-Id")
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	// Get user ID from header
	userIDStr := c.GetHeader("x-user-id")
	if userIDStr == "" {
		userIDStr = c.GetHeader("X-User-Id")
	}

	userID, _ := uuid.Parse(userIDStr)

	return tenantID, userID, nil
}
