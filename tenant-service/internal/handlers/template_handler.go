package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/services"
)

// TemplateHandler handles template HTTP requests
type TemplateHandler struct {
	templateService *services.TemplateService
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(templateService *services.TemplateService) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
	}
}

// CreateTemplate creates a new onboarding template
func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	var template models.OnboardingTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	createdTemplate, err := h.templateService.CreateTemplate(c.Request.Context(), &template)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to create template", err)
		return
	}

	SuccessResponse(c, http.StatusCreated, "Template created successfully", createdTemplate)
}

// GetTemplate retrieves a template by ID
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", err)
		return
	}

	template, err := h.templateService.GetTemplate(c.Request.Context(), templateID)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Template not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Template retrieved successfully", template)
}

// GetTemplatesByApplicationType retrieves templates by application type
func (h *TemplateHandler) GetTemplatesByApplicationType(c *gin.Context) {
	applicationType := c.Param("applicationType")
	if applicationType == "" {
		ErrorResponse(c, http.StatusBadRequest, "Application type is required", nil)
		return
	}

	templates, err := h.templateService.GetTemplatesByApplicationType(c.Request.Context(), applicationType)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get templates", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Templates retrieved successfully", templates)
}

// GetDefaultTemplate retrieves the default template for an application type
func (h *TemplateHandler) GetDefaultTemplate(c *gin.Context) {
	applicationType := c.Param("applicationType")
	if applicationType == "" {
		ErrorResponse(c, http.StatusBadRequest, "Application type is required", nil)
		return
	}

	template, err := h.templateService.GetDefaultTemplate(c.Request.Context(), applicationType)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Default template not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Default template retrieved successfully", template)
}

// GetActiveTemplates retrieves all active templates
func (h *TemplateHandler) GetActiveTemplates(c *gin.Context) {
	templates, err := h.templateService.GetActiveTemplates(c.Request.Context())
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get active templates", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Active templates retrieved successfully", templates)
}

// UpdateTemplate updates an existing template
func (h *TemplateHandler) UpdateTemplate(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", err)
		return
	}

	var template models.OnboardingTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	template.ID = templateID

	updatedTemplate, err := h.templateService.UpdateTemplate(c.Request.Context(), &template)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to update template", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Template updated successfully", updatedTemplate)
}

// SetDefaultTemplate sets a template as default for its application type
func (h *TemplateHandler) SetDefaultTemplate(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", err)
		return
	}

	if err := h.templateService.SetDefaultTemplate(c.Request.Context(), templateID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to set default template", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Template set as default successfully", nil)
}

// DeleteTemplate deletes a template
func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", err)
		return
	}

	if err := h.templateService.DeleteTemplate(c.Request.Context(), templateID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to delete template", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Template deleted successfully", nil)
}

// ListTemplates lists templates with pagination and filtering
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Build filters from query params
	filters := make(map[string]interface{})
	if applicationType := c.Query("application_type"); applicationType != "" {
		filters["application_type"] = applicationType
	}
	if isActive := c.Query("is_active"); isActive != "" {
		if active, err := strconv.ParseBool(isActive); err == nil {
			filters["is_active"] = active
		}
	}
	if isDefault := c.Query("is_default"); isDefault != "" {
		if defaultFlag, err := strconv.ParseBool(isDefault); err == nil {
			filters["is_default"] = defaultFlag
		}
	}

	templates, total, err := h.templateService.ListTemplates(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to list templates", err)
		return
	}

	response := map[string]interface{}{
		"templates": templates,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	}

	SuccessResponse(c, http.StatusOK, "Templates listed successfully", response)
}

// ValidateTemplateConfiguration validates template configuration
func (h *TemplateHandler) ValidateTemplateConfiguration(c *gin.Context) {
	var req struct {
		Configuration map[string]interface{} `json:"configuration" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := h.templateService.ValidateTemplateConfiguration(c.Request.Context(), req.Configuration); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid template configuration", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Template configuration is valid", nil)
}
