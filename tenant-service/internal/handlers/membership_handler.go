package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/services"
	sharedMiddleware "github.com/Tesseract-Nexus/go-shared/middleware"
)

// MembershipHandler handles membership-related HTTP requests
type MembershipHandler struct {
	membershipSvc *services.MembershipService
	staffClient   *clients.StaffClient
	tenantSvc     *services.TenantService // For enriching staff tenants with details
}

// NewMembershipHandler creates a new membership handler
func NewMembershipHandler(membershipSvc *services.MembershipService) *MembershipHandler {
	return &MembershipHandler{
		membershipSvc: membershipSvc,
	}
}

// NewMembershipHandlerWithStaff creates a membership handler with staff client for staff tenant lookup
func NewMembershipHandlerWithStaff(membershipSvc *services.MembershipService, staffClient *clients.StaffClient, tenantSvc *services.TenantService) *MembershipHandler {
	return &MembershipHandler{
		membershipSvc: membershipSvc,
		staffClient:   staffClient,
		tenantSvc:     tenantSvc,
	}
}

// GetUserTenants returns all tenants the current user has access to
// For platform_owner users, returns all tenants in the platform
// GET /api/v1/users/me/tenants
func (h *MembershipHandler) GetUserTenants(c *gin.Context) {
	// Get user ID from Istio-validated JWT claims (trusted)
	// Falls back to X-User-ID header for legacy compatibility during migration
	userIDStr := c.GetHeader("x-jwt-claim-sub")
	if userIDStr == "" {
		userIDStr = c.GetString("user_id") // Set by IstioAuth middleware
	}
	if userIDStr == "" {
		userIDStr = c.GetHeader("X-User-ID") // Legacy fallback
	}
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	keycloakID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	// Resolve Keycloak ID to local user ID for membership lookup
	// This handles the case where existing users have different local and Keycloak IDs
	userID, err := h.membershipSvc.ResolveUserID(c.Request.Context(), keycloakID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to resolve user", err)
		return
	}

	// Check if user is a platform owner using TRUSTED Istio header
	// SECURITY: Do NOT use X-User-Role header - it can be spoofed
	// The x-jwt-claim-platform-owner header is set by Istio after JWT validation
	isPlatformOwner := sharedMiddleware.IsPlatformOwner(c)
	if !isPlatformOwner {
		// Fallback: check x-jwt-claim-platform-owner header directly
		platformOwnerClaim := c.GetHeader("x-jwt-claim-platform-owner")
		isPlatformOwner = strings.EqualFold(platformOwnerClaim, "true")
	}

	if isPlatformOwner {
		// Return all tenants for platform admins
		allTenants, err := h.membershipSvc.GetAllTenants(c.Request.Context())
		if err != nil {
			ErrorResponse(c, http.StatusInternalServerError, "Failed to get all tenants", err)
			return
		}

		// Also get user's own memberships to merge with platform tenants
		userTenants, _ := h.membershipSvc.GetUserTenants(c.Request.Context(), userID)

		// Create a map of user's own tenants for quick lookup
		userTenantMap := make(map[string]bool)
		for _, t := range userTenants {
			userTenantMap[t.TenantID.String()] = true
		}

		// Mark platform tenants that the user also owns/is member of
		for i := range allTenants {
			if userTenantMap[allTenants[i].TenantID.String()] {
				// Find the user's role for this tenant
				for _, ut := range userTenants {
					if ut.TenantID == allTenants[i].TenantID {
						allTenants[i].IsDefault = ut.IsDefault
						allTenants[i].IsOwner = ut.IsOwner
						// Keep platform_admin role but note ownership
						break
					}
				}
			}
		}

		SuccessResponse(c, http.StatusOK, "All platform tenants retrieved", gin.H{
			"tenants":          allTenants,
			"count":            len(allTenants),
			"is_platform_admin": true,
		})
		return
	}

	// Regular user - return only their tenants
	tenants, err := h.membershipSvc.GetUserTenants(c.Request.Context(), userID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get user tenants", err)
		return
	}

	// If no memberships found and staff client is available, try staff tenants
	// Staff members are not in the tenant_users table, they're in staff table
	if len(tenants) == 0 && h.staffClient != nil {
		log.Printf("[MEMBERSHIP] User %s has no memberships, trying staff tenant lookup with keycloak ID %s", userID, keycloakID)
		staffTenants, staffErr := h.staffClient.GetStaffTenantsById(c.Request.Context(), keycloakID)
		if staffErr != nil {
			log.Printf("[MEMBERSHIP] Staff tenant lookup failed: %v", staffErr)
			// Don't fail - just continue with empty tenants
		} else if len(staffTenants) > 0 {
			log.Printf("[MEMBERSHIP] Found %d staff tenants for user %s", len(staffTenants), userID)
			// Convert staff tenants to UserTenantSummary format
			for _, st := range staffTenants {
				tenantSummary := services.UserTenantSummary{
					TenantID:  st.ID,
					Role:      st.Role,
					IsOwner:   strings.EqualFold(st.Role, "owner"),
					IsDefault: len(staffTenants) == 1, // Default if only one tenant
					Status:    "active",
				}

				// Enrich with tenant slug and name if tenant service is available
				if h.tenantSvc != nil {
					if tenant, tenantErr := h.tenantSvc.GetTenantByID(c.Request.Context(), st.ID); tenantErr == nil && tenant != nil {
						tenantSummary.Slug = tenant.Slug
						tenantSummary.Name = tenant.Name
						if tenant.DisplayName != "" {
							tenantSummary.DisplayName = tenant.DisplayName
						} else {
							tenantSummary.DisplayName = tenant.Name
						}
					}
				}

				tenants = append(tenants, tenantSummary)
			}
		}
	}

	SuccessResponse(c, http.StatusOK, "User tenants retrieved", gin.H{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// GetUserDefaultTenant returns the user's default tenant
// GET /api/v1/users/me/tenants/default
func (h *MembershipHandler) GetUserDefaultTenant(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	tenant, err := h.membershipSvc.GetUserDefaultTenant(c.Request.Context(), userID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get default tenant", err)
		return
	}

	if tenant == nil {
		ErrorResponse(c, http.StatusNotFound, "No default tenant found", nil)
		return
	}

	SuccessResponse(c, http.StatusOK, "Default tenant retrieved", gin.H{
		"tenant": tenant,
	})
}

// SetUserDefaultTenant sets the user's default tenant
// PUT /api/v1/users/me/tenants/default
func (h *MembershipHandler) SetUserDefaultTenant(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	if err := h.membershipSvc.SetUserDefaultTenant(c.Request.Context(), userID, tenantID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to set default tenant", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Default tenant updated", nil)
}

// GetTenantContext returns the context for accessing a specific tenant
// GET /api/v1/tenants/:slug/context
func (h *MembershipHandler) GetTenantContext(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	slug := c.Param("id")
	if slug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Tenant slug is required", nil)
		return
	}

	ctx, err := h.membershipSvc.GetUserTenantContext(c.Request.Context(), userID, slug)
	if err != nil {
		ErrorResponse(c, http.StatusForbidden, "Access denied to tenant", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Tenant context retrieved", gin.H{
		"context": ctx,
	})
}

// VerifyTenantAccess checks if the user has access to a tenant
// GET /api/v1/tenants/:id/access
func (h *MembershipHandler) VerifyTenantAccess(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	slug := c.Param("id")
	if slug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Tenant slug is required", nil)
		return
	}

	hasAccess, err := h.membershipSvc.VerifyTenantAccess(c.Request.Context(), userID, slug)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to verify access", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Access verified", gin.H{
		"has_access": hasAccess,
		"slug":       slug,
	})
}

// InviteMember invites a new member to a tenant
// POST /api/v1/tenants/:tenantId/members/invite
func (h *MembershipHandler) InviteMember(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	invitedBy, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role" binding:"required,oneof=admin manager member viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	inviteReq := &services.InviteMemberRequest{
		TenantID:  tenantID,
		Email:     req.Email,
		Role:      req.Role,
		InvitedBy: invitedBy,
	}

	resp, err := h.membershipSvc.InviteMember(c.Request.Context(), inviteReq)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	SuccessResponse(c, http.StatusCreated, "Invitation sent", gin.H{
		"invitation_token": resp.InvitationToken,
		"expires_at":       resp.ExpiresAt,
	})
}

// AcceptInvitation accepts a member invitation
// POST /api/v1/invitations/accept
func (h *MembershipHandler) AcceptInvitation(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	membership, err := h.membershipSvc.AcceptInvitation(c.Request.Context(), req.Token, userID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	SuccessResponse(c, http.StatusOK, "Invitation accepted", gin.H{
		"membership": membership,
	})
}

// RemoveMember removes a member from a tenant
// DELETE /api/v1/tenants/:tenantId/members/:memberId
func (h *MembershipHandler) RemoveMember(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	removedBy, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	memberIDStr := c.Param("memberId")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid member ID", nil)
		return
	}

	if err := h.membershipSvc.RemoveMember(c.Request.Context(), tenantID, memberID, removedBy); err != nil {
		ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	SuccessResponse(c, http.StatusOK, "Member removed", nil)
}

// UpdateMemberRole updates a member's role
// PUT /api/v1/tenants/:tenantId/members/:memberId/role
func (h *MembershipHandler) UpdateMemberRole(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	updatedBy, err := uuid.Parse(userIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	tenantIDStr := c.Param("id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	memberIDStr := c.Param("memberId")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid member ID", nil)
		return
	}

	var req struct {
		Role string `json:"role" binding:"required,oneof=admin manager member viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := h.membershipSvc.UpdateMemberRole(c.Request.Context(), tenantID, memberID, updatedBy, req.Role); err != nil {
		ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	SuccessResponse(c, http.StatusOK, "Member role updated", nil)
}

// ValidateSlug validates if a slug is available
// GET /api/v1/validation/slug
func (h *MembershipHandler) ValidateSlug(c *gin.Context) {
	slug := c.Query("slug")
	if slug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Slug is required", nil)
		return
	}

	var excludeTenantID *uuid.UUID
	if excludeStr := c.Query("exclude_tenant_id"); excludeStr != "" {
		if id, err := uuid.Parse(excludeStr); err == nil {
			excludeTenantID = &id
		}
	}

	valid, message, err := h.membershipSvc.ValidateSlug(c.Request.Context(), slug, excludeTenantID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to validate slug", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Slug validated", gin.H{
		"valid":   valid,
		"slug":    slug,
		"message": message,
	})
}

// GenerateSlug generates a unique slug from a name
// GET /api/v1/validation/slug/generate
func (h *MembershipHandler) GenerateSlug(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		ErrorResponse(c, http.StatusBadRequest, "Name is required", nil)
		return
	}

	slug, err := h.membershipSvc.GenerateSlug(c.Request.Context(), name)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to generate slug", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Slug generated", gin.H{
		"slug": slug,
		"name": name,
	})
}
