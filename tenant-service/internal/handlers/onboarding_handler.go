package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/services"
)

// OnboardingHandler handles onboarding HTTP requests
type OnboardingHandler struct {
	onboardingService *services.OnboardingService
	templateService   *services.TemplateService
}

// NewOnboardingHandler creates a new onboarding handler
func NewOnboardingHandler(
	onboardingService *services.OnboardingService,
	templateService *services.TemplateService,
) *OnboardingHandler {
	return &OnboardingHandler{
		onboardingService: onboardingService,
		templateService:   templateService,
	}
}

// StartOnboarding starts a new onboarding session
func (h *OnboardingHandler) StartOnboarding(c *gin.Context) {
	var req services.StartOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Auto-select default template if none provided
	if req.TemplateID == uuid.Nil {
		template, err := h.templateService.GetDefaultTemplate(c.Request.Context(), req.ApplicationType)
		if err != nil {
			ErrorResponse(c, http.StatusBadRequest, "No default template found for application type", err)
			return
		}
		req.TemplateID = template.ID
	} else {
		// Validate template exists
		_, err := h.templateService.GetTemplate(c.Request.Context(), req.TemplateID)
		if err != nil {
			ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", err)
			return
		}
	}

	session, err := h.onboardingService.StartOnboarding(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to start onboarding", err)
		return
	}

	SuccessResponse(c, http.StatusCreated, "Onboarding session created successfully", session)
}

// GetOnboardingSession retrieves an onboarding session
func (h *OnboardingHandler) GetOnboardingSession(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	// Get include relations from query params
	includeRelations := c.QueryArray("include")

	session, err := h.onboardingService.GetOnboardingSession(c.Request.Context(), sessionID, includeRelations)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Onboarding session not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Onboarding session retrieved successfully", session)
}

// UpdateBusinessInformation updates business information for a session
func (h *OnboardingHandler) UpdateBusinessInformation(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var businessInfo models.BusinessInformation
	if err := c.ShouldBindJSON(&businessInfo); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	updatedBusinessInfo, err := h.onboardingService.UpdateBusinessInformation(c.Request.Context(), sessionID, &businessInfo)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to update business information", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Business information updated successfully", updatedBusinessInfo)
}

// UpdateContactInformation updates contact information for a session
func (h *OnboardingHandler) UpdateContactInformation(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var contactInfo models.ContactInformation
	if err := c.ShouldBindJSON(&contactInfo); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	updatedContactInfo, err := h.onboardingService.UpdateContactInformation(c.Request.Context(), sessionID, &contactInfo)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to update contact information", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Contact information updated successfully", updatedContactInfo)
}

// UpdateBusinessAddressRequest includes both business and billing address
type UpdateBusinessAddressRequest struct {
	// Business address fields
	StreetAddress string `json:"street_address" binding:"required"`
	AddressLine2  string `json:"address_line_2"`
	City          string `json:"city" binding:"required"`
	StateProvince string `json:"state_province" binding:"required"`
	PostalCode    string `json:"postal_code" binding:"required"`
	Country       string `json:"country" binding:"required"`
	AddressType   string `json:"address_type"`

	// Billing address fields
	BillingSameAsBusiness bool   `json:"billing_same_as_business"`
	BillingStreetAddress  string `json:"billing_street_address"`
	BillingCity           string `json:"billing_city"`
	BillingState          string `json:"billing_state"`
	BillingPostalCode     string `json:"billing_postal_code"`
	BillingCountry        string `json:"billing_country"`
}

// UpdateBusinessAddress updates business address for a session
func (h *OnboardingHandler) UpdateBusinessAddress(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req UpdateBusinessAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Create business address
	address := &models.BusinessAddress{
		StreetAddress: req.StreetAddress,
		City:          req.City,
		StateProvince: req.StateProvince,
		PostalCode:    req.PostalCode,
		Country:       req.Country,
		AddressType:   "business",
		IsPrimary:     true,
	}

	updatedAddress, err := h.onboardingService.UpdateBusinessAddress(c.Request.Context(), sessionID, address)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to update business address", err)
		return
	}

	// Create billing address if different from business
	if !req.BillingSameAsBusiness && req.BillingStreetAddress != "" {
		billingAddress := &models.BusinessAddress{
			StreetAddress: req.BillingStreetAddress,
			City:          req.BillingCity,
			StateProvince: req.BillingState,
			PostalCode:    req.BillingPostalCode,
			Country:       req.BillingCountry,
			AddressType:   "billing",
			IsPrimary:     false,
		}
		_, err := h.onboardingService.UpdateBusinessAddress(c.Request.Context(), sessionID, billingAddress)
		if err != nil {
			// Log but don't fail - business address was saved
			c.Set("billing_error", err.Error())
		}
	}

	SuccessResponse(c, http.StatusOK, "Business address updated successfully", updatedAddress)
}

// CompleteOnboarding completes an onboarding session
func (h *OnboardingHandler) CompleteOnboarding(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	completedSession, err := h.onboardingService.CompleteOnboarding(c.Request.Context(), sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to complete onboarding", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Onboarding completed successfully", completedSession)
}

// GetProgress retrieves onboarding progress
func (h *OnboardingHandler) GetProgress(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	progress, err := h.onboardingService.GetProgress(c.Request.Context(), sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Failed to get progress", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Progress retrieved successfully", progress)
}

// GetTasks retrieves tasks for a session
func (h *OnboardingHandler) GetTasks(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	tasks, err := h.onboardingService.GetTasks(c.Request.Context(), sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get tasks", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Tasks retrieved successfully", tasks)
}

// UpdateTaskStatus updates task status
func (h *OnboardingHandler) UpdateTaskStatus(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid task ID", err)
		return
	}

	var req struct {
		Status         string                 `json:"status" binding:"required"`
		CompletionData map[string]interface{} `json:"completion_data,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := h.onboardingService.UpdateTaskStatus(c.Request.Context(), sessionID, taskID, req.Status, req.CompletionData); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to update task status", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Task status updated successfully", nil)
}

// ValidateSubdomain validates subdomain availability with suggestions
// If session_id is provided, the slug will be reserved for that session to prevent race conditions
// Optional storefront_slug can be passed to also save the storefront URL slug
func (h *OnboardingHandler) ValidateSubdomain(c *gin.Context) {
	subdomain := c.Query("subdomain")
	if subdomain == "" {
		ErrorResponse(c, http.StatusBadRequest, "Subdomain parameter is required", nil)
		return
	}

	// Get optional storefront slug (defaults to same as admin subdomain)
	storefrontSlug := c.Query("storefront_slug")

	// Get optional session ID for slug reservation
	sessionIDStr := c.Query("session_id")
	var sessionID *uuid.UUID
	if sessionIDStr != "" {
		if parsed, err := uuid.Parse(sessionIDStr); err == nil {
			sessionID = &parsed
		}
	}

	// If session ID is provided, validate AND reserve the slug
	if sessionID != nil {
		result, err := h.onboardingService.ValidateAndReserveSlug(c.Request.Context(), subdomain, *sessionID, storefrontSlug)
		if err != nil {
			ErrorResponse(c, http.StatusInternalServerError, "Failed to validate and reserve subdomain", err)
			return
		}
		SuccessResponse(c, http.StatusOK, result.Message, result)
		return
	}

	// Without session ID, just validate without reserving (for quick checks)
	// Pass nil for sessionID since we don't have one
	result, err := h.onboardingService.ValidateSlugWithSuggestions(c.Request.Context(), subdomain, nil)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to validate subdomain", err)
		return
	}

	// Return the full validation result with suggestions
	SuccessResponse(c, http.StatusOK, result.Message, result)
}

// ValidateStorefront validates storefront slug availability with suggestions
// If session_id is provided, the slug will be reserved for that session to prevent race conditions
func (h *OnboardingHandler) ValidateStorefront(c *gin.Context) {
	storefrontSlug := c.Query("storefront_slug")
	if storefrontSlug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Storefront slug parameter is required", nil)
		return
	}

	// Get optional session ID for slug reservation
	sessionIDStr := c.Query("session_id")
	var sessionID *uuid.UUID
	if sessionIDStr != "" {
		if parsed, err := uuid.Parse(sessionIDStr); err == nil {
			sessionID = &parsed
		}
	}

	// Validate the storefront slug (uses same validation as subdomain)
	// Storefront slugs share the same namespace as subdomains to prevent conflicts
	result, err := h.onboardingService.ValidateSlugWithSuggestions(c.Request.Context(), storefrontSlug, sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to validate storefront slug", err)
		return
	}

	// Return the full validation result with suggestions
	SuccessResponse(c, http.StatusOK, result.Message, result)
}

// ValidateBusinessName validates business name availability
func (h *OnboardingHandler) ValidateBusinessName(c *gin.Context) {
	businessName := c.Query("business_name")
	if businessName == "" {
		ErrorResponse(c, http.StatusBadRequest, "Business name parameter is required", nil)
		return
	}

	isAvailable, err := h.onboardingService.ValidateBusinessName(c.Request.Context(), businessName)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to validate business name", err)
		return
	}

	response := map[string]interface{}{
		"business_name": businessName,
		"available":     isAvailable,
	}

	SuccessResponse(c, http.StatusOK, "Business name validation completed", response)
}

// ListSessions lists onboarding sessions with pagination
func (h *OnboardingHandler) ListSessions(c *gin.Context) {
	_, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	_, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Build filters from query params
	filters := make(map[string]interface{})
	if applicationType := c.Query("application_type"); applicationType != "" {
		filters["application_type"] = applicationType
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if tenantID := c.Query("tenant_id"); tenantID != "" {
		if parsedTenantID, err := uuid.Parse(tenantID); err == nil {
			filters["tenant_id"] = parsedTenantID
		}
	}

	// Note: This would typically be in a separate admin handler
	// For now, we'll include it here but it should be protected by admin middleware

	ErrorResponse(c, http.StatusNotImplemented, "List sessions endpoint not implemented", nil)
}

// CompleteAccountSetup completes account setup by creating tenant and user account
func (h *OnboardingHandler) CompleteAccountSetup(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req struct {
		Password      string `json:"password" binding:"required,min=8"`
		AuthMethod    string `json:"auth_method" binding:"required,oneof=password social"`
		Timezone      string `json:"timezone"`
		Currency      string `json:"currency"`
		BusinessModel string `json:"business_model"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Set defaults if not provided
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.BusinessModel == "" {
		req.BusinessModel = "ONLINE_STORE"
	}

	result, err := h.onboardingService.CompleteAccountSetup(c.Request.Context(), sessionID, req.Password, req.AuthMethod, req.Timezone, req.Currency, req.BusinessModel)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to complete account setup", err)
		return
	}

	SuccessResponse(c, http.StatusCreated, "Account created successfully", result)
}
