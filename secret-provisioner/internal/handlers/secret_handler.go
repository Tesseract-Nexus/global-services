package handlers

import (
	"net/http"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/middleware"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SecretHandler handles HTTP requests for secret operations
type SecretHandler struct {
	service *services.ProvisionerService
	logger  *logrus.Entry
}

// NewSecretHandler creates a new secret handler
func NewSecretHandler(service *services.ProvisionerService, logger *logrus.Entry) *SecretHandler {
	return &SecretHandler{
		service: service,
		logger:  logger,
	}
}

// ProvisionSecrets handles POST /api/v1/secrets
func (h *SecretHandler) ProvisionSecrets(c *gin.Context) {
	var req models.ProvisionSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	// Get context values
	tenantID := middleware.GetTenantID(c)
	if tenantID != "" && tenantID != req.TenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "TENANT_MISMATCH",
			Message: "Tenant ID in request does not match authenticated tenant",
		})
		return
	}

	actorID := middleware.GetActorID(c)
	actorService := middleware.GetInternalService(c)
	requestID := middleware.GetRequestID(c)

	resp, err := h.service.ProvisionSecrets(c.Request.Context(), &req, actorID, actorService, requestID)
	if err != nil {
		h.logger.WithError(err).Error("failed to provision secrets")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "PROVISIONING_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMetadata handles GET /api/v1/secrets/metadata
func (h *SecretHandler) GetMetadata(c *gin.Context) {
	var req models.GetSecretMetadataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	// Verify tenant access
	tenantID := middleware.GetTenantID(c)
	if tenantID != "" && tenantID != req.TenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "TENANT_MISMATCH",
			Message: "Tenant ID in request does not match authenticated tenant",
		})
		return
	}

	resp, err := h.service.GetSecretMetadata(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("failed to get metadata")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "METADATA_FETCH_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListProviders handles GET /api/v1/secrets/providers
func (h *SecretHandler) ListProviders(c *gin.Context) {
	var req models.ListProvidersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	// Verify tenant access
	tenantID := middleware.GetTenantID(c)
	if tenantID != "" && tenantID != req.TenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "TENANT_MISMATCH",
			Message: "Tenant ID in request does not match authenticated tenant",
		})
		return
	}

	resp, err := h.service.ListProviders(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("failed to list providers")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "LIST_PROVIDERS_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteSecret handles DELETE /api/v1/secrets/:name
func (h *SecretHandler) DeleteSecret(c *gin.Context) {
	secretName := c.Param("name")
	if secretName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Secret name is required",
		})
		return
	}

	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "Tenant ID is required",
		})
		return
	}

	actorID := middleware.GetActorID(c)
	actorService := middleware.GetInternalService(c)
	requestID := middleware.GetRequestID(c)

	err := h.service.DeleteSecret(c.Request.Context(), secretName, tenantID, actorID, actorService, requestID)
	if err != nil {
		h.logger.WithError(err).Error("failed to delete secret")
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "DELETE_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Secret deleted successfully",
	})
}
