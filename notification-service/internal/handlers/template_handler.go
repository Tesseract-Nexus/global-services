package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-service/internal/models"
	"notification-service/internal/repository"
	"notification-service/internal/template"
)

// TemplateHandler handles template-related requests
type TemplateHandler struct {
	templateRepo repository.TemplateRepository
	templateEng  *template.Engine
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(templateRepo repository.TemplateRepository) *TemplateHandler {
	return &TemplateHandler{
		templateRepo: templateRepo,
		templateEng:  template.NewEngine(),
	}
}

// CreateTemplateRequest represents a create template request
type CreateTemplateRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Description  string                 `json:"description"`
	Channel      string                 `json:"channel" binding:"required,oneof=EMAIL SMS PUSH"`
	Category     string                 `json:"category"`
	Subject      string                 `json:"subject"`
	BodyTemplate string                 `json:"bodyTemplate"`
	HTMLTemplate string                 `json:"htmlTemplate"`
	Variables    map[string]interface{} `json:"variables"`
	DefaultData  map[string]interface{} `json:"defaultData"`
	Tags         []string               `json:"tags"`
}

// List returns templates for a tenant
func (h *TemplateHandler) List(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id"})
		return
	}

	isActive := true
	filters := repository.TemplateFilters{
		Channel:  c.Query("channel"),
		Category: c.Query("category"),
		IsActive: &isActive,
		Search:   c.Query("search"),
		Limit:    parseIntWithDefault(c.Query("limit"), 50),
		Offset:   parseIntWithDefault(c.Query("offset"), 0),
	}

	templates, total, err := h.templateRepo.List(c.Request.Context(), tenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
		"pagination": gin.H{
			"limit":  filters.Limit,
			"offset": filters.Offset,
			"total":  total,
		},
	})
}

// Get returns a single template
func (h *TemplateHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	tmpl, err := h.templateRepo.GetByID(c.Request.Context(), id)
	if err != nil || tmpl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tmpl,
	})
}

// Create creates a new template
func (h *TemplateHandler) Create(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id"})
		return
	}

	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if template with same name exists
	existing, _ := h.templateRepo.GetByName(c.Request.Context(), tenantID, req.Name)
	if existing != nil && existing.TenantID == tenantID {
		c.JSON(http.StatusConflict, gin.H{"error": "Template with this name already exists"})
		return
	}

	tmpl := &models.NotificationTemplate{
		TenantID:     tenantID,
		Name:         req.Name,
		Description:  req.Description,
		Channel:      models.NotificationChannel(req.Channel),
		Category:     req.Category,
		Subject:      req.Subject,
		BodyTemplate: req.BodyTemplate,
		HTMLTemplate: req.HTMLTemplate,
		IsActive:     true,
		IsSystem:     false,
		Version:      1,
	}

	if err := h.templateRepo.Create(c.Request.Context(), tmpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    tmpl,
	})
}

// Update updates a template
func (h *TemplateHandler) Update(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	tmpl, err := h.templateRepo.GetByID(c.Request.Context(), id)
	if err != nil || tmpl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	// Can't update system templates
	if tmpl.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify system templates"})
		return
	}

	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	tmpl.Name = req.Name
	tmpl.Description = req.Description
	tmpl.Channel = models.NotificationChannel(req.Channel)
	tmpl.Category = req.Category
	tmpl.Subject = req.Subject
	tmpl.BodyTemplate = req.BodyTemplate
	tmpl.HTMLTemplate = req.HTMLTemplate
	tmpl.Version++

	if err := h.templateRepo.Update(c.Request.Context(), tmpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tmpl,
	})
}

// Delete deletes a template
func (h *TemplateHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	tmpl, err := h.templateRepo.GetByID(c.Request.Context(), id)
	if err != nil || tmpl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	// Can't delete system templates
	if tmpl.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system templates"})
		return
	}

	if err := h.templateRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Template deleted",
	})
}

// TestRequest represents a template test request
type TestRequest struct {
	Variables map[string]interface{} `json:"variables"`
}

// Test renders a template with test variables
func (h *TemplateHandler) Test(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	tmpl, err := h.templateRepo.GetByID(c.Request.Context(), id)
	if err != nil || tmpl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var req TestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := gin.H{
		"success": true,
	}

	// Render subject
	if tmpl.Subject != "" {
		if rendered, err := h.templateEng.RenderText(tmpl.Subject, req.Variables); err == nil {
			result["subject"] = rendered
		} else {
			result["subjectError"] = err.Error()
		}
	}

	// Render body
	if tmpl.BodyTemplate != "" {
		if rendered, err := h.templateEng.RenderText(tmpl.BodyTemplate, req.Variables); err == nil {
			result["body"] = rendered
		} else {
			result["bodyError"] = err.Error()
		}
	}

	// Render HTML
	if tmpl.HTMLTemplate != "" {
		if rendered, err := h.templateEng.RenderHTML(tmpl.HTMLTemplate, req.Variables); err == nil {
			result["html"] = rendered
		} else {
			result["htmlError"] = err.Error()
		}
	}

	c.JSON(http.StatusOK, result)
}
