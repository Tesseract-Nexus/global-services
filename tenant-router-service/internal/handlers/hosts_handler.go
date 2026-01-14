package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tenant-router-service/internal/models"
	"tenant-router-service/internal/services"
)

// HostsHandler handles tenant hosts management endpoints
type HostsHandler struct {
	routerService *services.RouterService
}

// NewHostsHandler creates a new hosts handler
func NewHostsHandler(routerService *services.RouterService) *HostsHandler {
	return &HostsHandler{
		routerService: routerService,
	}
}

// ListHosts returns all managed tenant hosts with pagination and filtering
func (h *HostsHandler) ListHosts(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	statusStr := c.Query("status")

	var status *models.HostStatus
	if statusStr != "" {
		s := models.HostStatus(statusStr)
		status = &s
	}

	hosts, total, err := h.routerService.ListTenantHosts(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hosts,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetHost returns a specific tenant's hosts
func (h *HostsHandler) GetHost(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "slug is required",
		})
		return
	}

	host, err := h.routerService.GetTenantHost(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if host == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "tenant host not found",
		})
		return
	}

	// Get certificate status
	certStatus, _ := h.routerService.GetCertificateStatus(c.Request.Context(), slug)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"data":        host,
		"cert_status": certStatus,
	})
}

// AddHostRequest represents a request to add a tenant host
type AddHostRequest struct {
	Slug     string `json:"slug" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required"`
}

// AddHost manually adds a tenant host
func (h *HostsHandler) AddHost(c *gin.Context) {
	slug := c.Param("slug")

	var req AddHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body, use slug from path
		req.Slug = slug
		if req.Slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "slug is required",
			})
			return
		}
		if req.TenantID == "" {
			req.TenantID = slug // Use slug as tenant ID if not provided
		}
	}

	if slug != "" {
		req.Slug = slug
	}

	result, err := h.routerService.AddTenantHost(c.Request.Context(), req.Slug, req.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusPartialContent
	}

	c.JSON(status, gin.H{
		"success": result.Success,
		"data":    result,
	})
}

// RemoveHost manually removes a tenant host
func (h *HostsHandler) RemoveHost(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "slug is required",
		})
		return
	}

	if err := h.routerService.RemoveTenantHost(c.Request.Context(), slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "tenant host removed",
	})
}

// SyncHost forces a sync of a tenant's hosts
func (h *HostsHandler) SyncHost(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "slug is required",
		})
		return
	}

	// Get tenant ID from existing host or use slug
	tenantID := slug
	if host, err := h.routerService.GetTenantHost(c.Request.Context(), slug); err == nil && host != nil {
		tenantID = host.TenantID
	}

	result, err := h.routerService.SyncTenant(c.Request.Context(), slug, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": result.Success,
		"data":    result,
	})
}

// SyncVSRoutesRequest represents a request to sync VirtualService routes
type SyncVSRoutesRequest struct {
	VSType string `json:"vs_type" binding:"required"` // admin, storefront, or api
}

// SyncVSRoutes syncs a tenant's VirtualService routes with the template
// This is useful when the template is updated with new routes
func (h *HostsHandler) SyncVSRoutes(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "slug is required",
		})
		return
	}

	var req SyncVSRoutesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "vs_type is required (admin, storefront, or api)",
		})
		return
	}

	if err := h.routerService.SyncVirtualServiceRoutes(c.Request.Context(), slug, req.VSType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VirtualService routes synced successfully",
		"slug":    slug,
		"vs_type": req.VSType,
	})
}

// SyncAllVSRoutesRequest represents a request to sync all VirtualService routes
type SyncAllVSRoutesRequest struct {
	VSType string `json:"vs_type" binding:"required"` // admin, storefront, or api
}

// SyncAllVSRoutes syncs all tenant VirtualServices for a given template
func (h *HostsHandler) SyncAllVSRoutes(c *gin.Context) {
	var req SyncAllVSRoutesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "vs_type is required (admin, storefront, or api)",
		})
		return
	}

	synced, err := h.routerService.SyncAllVirtualServiceRoutes(c.Request.Context(), req.VSType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VirtualService routes synced successfully",
		"vs_type": req.VSType,
		"synced":  synced,
	})
}

// GetStats returns statistics about tenant hosts
func (h *HostsHandler) GetStats(c *gin.Context) {
	stats, err := h.routerService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
