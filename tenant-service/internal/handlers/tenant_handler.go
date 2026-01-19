package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/services"
)

// TenantHandler handles tenant-related HTTP requests
type TenantHandler struct {
	tenantService      *services.TenantService
	offboardingService *services.OffboardingService
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(tenantService *services.TenantService, offboardingService *services.OffboardingService) *TenantHandler {
	return &TenantHandler{
		tenantService:      tenantService,
		offboardingService: offboardingService,
	}
}

// CreateTenantForUserRequest represents the request to create a tenant for an existing user
type CreateTenantForUserRequest struct {
	Name           string `json:"name" binding:"required,min=2"`
	Slug           string `json:"slug" binding:"required,min=3"`
	Industry       string `json:"industry"`
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
}

// CreateTenantForUser creates a new tenant for an authenticated user
// This is a simplified flow for existing users who want to create additional stores
// @Summary Create tenant for existing user
// @Description Creates a new tenant and assigns the current user as owner
// @Tags tenants
// @Accept json
// @Produce json
// @Param X-User-ID header string true "Authenticated user ID"
// @Param request body CreateTenantForUserRequest true "Tenant creation request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/create-for-user [post]
func (h *TenantHandler) CreateTenantForUser(c *gin.Context) {
	// Get user ID from header (set by auth middleware)
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User ID is required", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID format", err)
		return
	}

	var req CreateTenantForUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Create tenant for user
	result, err := h.tenantService.CreateTenantForUser(c.Request.Context(), &services.CreateTenantForUserRequest{
		UserID:         userID,
		Name:           req.Name,
		Slug:           req.Slug,
		Industry:       req.Industry,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
	})

	if err != nil {
		// Check for specific error types
		if err.Error() == "slug already exists" {
			ErrorResponse(c, http.StatusConflict, "A store with this slug already exists", err)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to create tenant", err)
		return
	}

	SuccessResponse(c, http.StatusCreated, "Tenant created successfully", result)
}

// CheckSlugAvailability checks if a slug is available for use
// @Summary Check slug availability
// @Description Checks if a tenant slug is available
// @Tags tenants
// @Produce json
// @Param slug query string true "Slug to check"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/tenants/check-slug [get]
func (h *TenantHandler) CheckSlugAvailability(c *gin.Context) {
	slug := c.Query("slug")
	if slug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Slug parameter is required", nil)
		return
	}

	available, reason := h.tenantService.CheckSlugAvailability(c.Request.Context(), slug)

	SuccessResponse(c, http.StatusOK, "Slug availability checked", map[string]interface{}{
		"slug":      slug,
		"available": available,
		"reason":    reason,
	})
}

// DeleteTenantRequest represents the request to delete a tenant
type DeleteTenantRequest struct {
	ConfirmationText string `json:"confirmation_text" binding:"required"`
	Reason           string `json:"reason"`
}

// DeleteTenant deletes a tenant and archives all data
// @Summary Delete tenant
// @Description Permanently deletes a tenant and archives all data for audit purposes. Only the tenant owner can perform this action.
// @Tags tenants
// @Accept json
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param X-User-ID header string true "Authenticated user ID"
// @Param request body DeleteTenantRequest true "Deletion request with confirmation"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{tenantId} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	// Get tenant ID from path
	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID format", err)
		return
	}

	// Get user ID from header (set by auth middleware)
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User ID is required", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID format", err)
		return
	}

	// Parse request body
	var req DeleteTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Call offboarding service
	result, err := h.offboardingService.DeleteTenant(c.Request.Context(), &services.DeleteTenantRequest{
		TenantID:         tenantID,
		UserID:           userID,
		ConfirmationText: req.ConfirmationText,
		Reason:           req.Reason,
	})

	if err != nil {
		// Check for specific error types
		errMsg := err.Error()
		if errMsg == "tenant not found" {
			ErrorResponse(c, http.StatusNotFound, errMsg, nil)
			return
		}
		if errMsg == "only the tenant owner can delete the tenant" {
			ErrorResponse(c, http.StatusForbidden, errMsg, nil)
			return
		}
		if len(errMsg) > 28 && errMsg[:28] == "invalid confirmation text:" {
			ErrorResponse(c, http.StatusBadRequest, errMsg, nil)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to delete tenant", err)
		return
	}

	SuccessResponse(c, http.StatusOK, result.Message, result)
}

// GetTenantDeletionInfo gets information needed for the tenant deletion UI
// @Summary Get tenant deletion info
// @Description Gets tenant information and requirements for deletion. Only the tenant owner can access this.
// @Tags tenants
// @Produce json
// @Param tenantId path string true "Tenant ID"
// @Param X-User-ID header string true "Authenticated user ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/tenants/{tenantId}/deletion [get]
func (h *TenantHandler) GetTenantDeletionInfo(c *gin.Context) {
	// Get tenant ID from path
	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID format", err)
		return
	}

	// Get user ID from header
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User ID is required", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID format", err)
		return
	}

	// Get deletion info
	info, err := h.offboardingService.GetTenantForDeletion(c.Request.Context(), tenantID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "tenant not found" {
			ErrorResponse(c, http.StatusNotFound, errMsg, nil)
			return
		}
		if errMsg == "only the tenant owner can view deletion information" {
			ErrorResponse(c, http.StatusForbidden, errMsg, nil)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get deletion info", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Tenant deletion info retrieved", info)
}

// GetTenantInfo returns basic tenant information for internal service-to-service calls
// This endpoint doesn't require user authentication, only internal service header
// @Summary Get tenant info (internal)
// @Description Returns basic tenant information for internal service calls
// @Tags internal
// @Produce json
// @Param id path string true "Tenant ID"
// @Param X-Internal-Service header string true "Internal service name"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /internal/tenants/{id} [get]
func (h *TenantHandler) GetTenantInfo(c *gin.Context) {
	// Verify internal service header
	internalService := c.GetHeader("X-Internal-Service")
	if internalService == "" {
		ErrorResponse(c, http.StatusUnauthorized, "Internal service header required", nil)
		return
	}

	// Get tenant ID from path
	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID format", err)
		return
	}

	// Get tenant info
	info, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Tenant not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Tenant info retrieved", info)
}

// GetTenantBySlug returns basic tenant information by slug for internal service-to-service calls
// @Summary Get tenant info by slug (internal)
// @Description Returns basic tenant information by slug for internal service calls
// @Tags internal
// @Produce json
// @Param slug path string true "Tenant slug"
// @Param X-Internal-Service header string true "Internal service name"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /internal/tenants/by-slug/{slug} [get]
func (h *TenantHandler) GetTenantBySlug(c *gin.Context) {
	// Verify internal service header
	internalService := c.GetHeader("X-Internal-Service")
	if internalService == "" {
		ErrorResponse(c, http.StatusUnauthorized, "Internal service header required", nil)
		return
	}

	// Get tenant slug from path
	slug := c.Param("slug")
	if slug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Tenant slug is required", nil)
		return
	}

	// Get tenant info by slug
	info, err := h.tenantService.GetTenantBySlug(c.Request.Context(), slug)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Tenant not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Tenant info retrieved", info)
}

// GetTenantOnboardingData returns onboarding data for a tenant (for settings pre-population)
// This endpoint implements multi-tenant security by verifying user access before returning data.
// @Summary Get tenant onboarding data
// @Description Returns business info, contact info, and addresses collected during tenant onboarding.
// @Description This data is used to pre-populate settings pages. Requires active membership to tenant.
// @Tags tenants
// @Produce json
// @Param id path string true "Tenant ID (UUID)"
// @Param X-User-ID header string true "Authenticated user ID"
// @Success 200 {object} map[string]interface{} "Onboarding data retrieved successfully"
// @Failure 400 {object} map[string]interface{} "Invalid tenant ID format"
// @Failure 401 {object} map[string]interface{} "User ID is required"
// @Failure 403 {object} map[string]interface{} "Access denied - user does not have access to this tenant"
// @Failure 404 {object} map[string]interface{} "Tenant not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/tenants/{id}/onboarding-data [get]
func (h *TenantHandler) GetTenantOnboardingData(c *gin.Context) {
	// Get tenant ID from path
	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID format", err)
		return
	}

	// Get user ID from header (required for access control)
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User ID is required", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID format", err)
		return
	}

	// Get onboarding data with access control
	data, err := h.tenantService.GetTenantOnboardingData(c.Request.Context(), tenantID, userID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "access denied: user does not have access to this tenant" {
			ErrorResponse(c, http.StatusForbidden, errMsg, nil)
			return
		}
		if errMsg == "tenant not found" || errMsg[:14] == "tenant not found" {
			ErrorResponse(c, http.StatusNotFound, "Tenant not found", err)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get onboarding data", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Onboarding data retrieved", data)
}

// GetTenantGrowthBookConfig returns the GrowthBook configuration for a tenant
// @Summary Get tenant GrowthBook configuration
// @Description Returns the GrowthBook SDK key and org ID for a tenant
// @Tags tenants
// @Produce json
// @Param id path string true "Tenant ID or slug"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/growthbook [get]
func (h *TenantHandler) GetTenantGrowthBookConfig(c *gin.Context) {
	tenantIDOrSlug := c.Param("id")

	// Try to get by ID first, then by slug
	config, err := h.tenantService.GetTenantGrowthBookConfig(c.Request.Context(), tenantIDOrSlug)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "tenant not found" || errMsg == "growthbook not provisioned" {
			ErrorResponse(c, http.StatusNotFound, errMsg, nil)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get GrowthBook config", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "GrowthBook configuration retrieved", config)
}

// GetTenantGrowthBookSDKKey returns only the SDK key for client-side usage
// This endpoint is used by storefronts and admin apps to get the SDK key
// @Summary Get tenant GrowthBook SDK key
// @Description Returns the GrowthBook SDK client key for a tenant
// @Tags tenants
// @Produce json
// @Param id path string true "Tenant ID or slug"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/growthbook/sdk-key [get]
func (h *TenantHandler) GetTenantGrowthBookSDKKey(c *gin.Context) {
	tenantIDOrSlug := c.Param("id")

	sdkKey, err := h.tenantService.GetTenantGrowthBookSDKKey(c.Request.Context(), tenantIDOrSlug)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "tenant not found" || errMsg == "growthbook not provisioned" {
			ErrorResponse(c, http.StatusNotFound, errMsg, nil)
			return
		}
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get SDK key", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sdk_key": sdkKey,
		},
	})
}
