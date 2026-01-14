package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/services"
)

// AuthHandler handles tenant-aware authentication HTTP requests
// This enables multi-tenant credential isolation where the same email
// can have different passwords for different tenants
type AuthHandler struct {
	authSvc     *services.TenantAuthService
	staffClient *clients.StaffClient
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authSvc *services.TenantAuthService, staffClient *clients.StaffClient) *AuthHandler {
	return &AuthHandler{
		authSvc:     authSvc,
		staffClient: staffClient,
	}
}

// ValidateCredentialsRequest represents a request to validate tenant-specific credentials
type ValidateCredentialsRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	TenantID   string `json:"tenant_id"`   // Either tenant_id or tenant_slug required
	TenantSlug string `json:"tenant_slug"` // Either tenant_id or tenant_slug required
}

// ValidateCredentials validates tenant-specific credentials
// POST /api/v1/auth/validate
// This endpoint is called by auth-bff during the login flow
func (h *AuthHandler) ValidateCredentials(c *gin.Context) {
	var req ValidateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate that either tenant_id or tenant_slug is provided
	if req.TenantID == "" && req.TenantSlug == "" {
		ErrorResponse(c, http.StatusBadRequest, "Either tenant_id or tenant_slug is required", nil)
		return
	}

	// Parse tenant ID if provided
	var tenantID uuid.UUID
	if req.TenantID != "" {
		var err error
		tenantID, err = uuid.Parse(req.TenantID)
		if err != nil {
			ErrorResponse(c, http.StatusBadRequest, "Invalid tenant_id format", err)
			return
		}
	}

	// Get client IP and user agent for audit logging
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Validate credentials
	result, err := h.authSvc.ValidateCredentials(c.Request.Context(), &services.ValidateCredentialsRequest{
		Email:      req.Email,
		Password:   req.Password,
		TenantID:   tenantID,
		TenantSlug: req.TenantSlug,
		IPAddress:  clientIP,
		UserAgent:  userAgent,
	})

	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to validate credentials", err)
		return
	}

	if !result.Valid {
		// NOTE: Staff authentication should go through Keycloak, not tenant_credentials.
		// Staff members have their passwords stored in Keycloak during account activation.
		// The auth-bff handles Keycloak authentication for all users including staff.

		// Return authentication failure with error details
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":            false,
			"valid":              false,
			"error_code":         result.ErrorCode,
			"message":            result.ErrorMessage,
			"account_locked":     result.AccountLocked,
			"locked_until":       result.LockedUntil,
			"remaining_attempts": result.RemainingAttempts,
			"tenant_id":          result.TenantID,
			"tenant_slug":        result.TenantSlug,
		})
		return
	}

	// Return successful authentication
	response := gin.H{
		"valid":        true,
		"user_id":      result.UserID,
		"tenant_id":    result.TenantID,
		"tenant_slug":  result.TenantSlug,
		"email":        result.Email,
		"first_name":   result.FirstName,
		"last_name":    result.LastName,
		"role":         result.Role,
		"mfa_required": result.MFARequired,
		"mfa_enabled":  result.MFAEnabled,
	}

	// Include tokens if they were obtained (direct grant)
	if result.AccessToken != "" {
		response["access_token"] = result.AccessToken
		response["refresh_token"] = result.RefreshToken
		response["id_token"] = result.IDToken
		response["expires_in"] = result.ExpiresIn
	}

	SuccessResponse(c, http.StatusOK, "Credentials validated successfully", response)
}

// GetUserTenantsForAuth returns all tenants a user has access to for login selection
// POST /api/v1/auth/tenants
// This endpoint is called when a user enters their email to show tenant selection
// It combines tenants from both tenant_users (owners/admins) and staff (employees)
func (h *AuthHandler) GetUserTenantsForAuth(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get tenants from tenant_users table (owners/admins)
	tenants, err := h.authSvc.GetUserTenants(c.Request.Context(), req.Email)
	if err != nil {
		log.Printf("[AuthHandler] Error getting user tenants for %s: %v", req.Email, err)
		tenants = []services.TenantAuthInfo{} // Continue with empty list
	}

	// Also get tenants from staff table (employees)
	// This allows staff members to login even if they're not in tenant_users
	if h.staffClient != nil {
		staffTenants, err := h.staffClient.GetStaffTenants(c.Request.Context(), req.Email)
		if err != nil {
			log.Printf("[AuthHandler] Error getting staff tenants for %s: %v", req.Email, err)
			// Continue without staff tenants
		} else if len(staffTenants) > 0 {
			// Merge staff tenants into the list, avoiding duplicates
			existingTenantIDs := make(map[uuid.UUID]bool)
			for _, t := range tenants {
				existingTenantIDs[t.ID] = true
			}

			for _, st := range staffTenants {
				if !existingTenantIDs[st.ID] {
					// Enrich staff tenant with tenant details (slug, name)
					tenantInfo, err := h.authSvc.GetTenantBasicInfo(c.Request.Context(), st.ID)
					if err != nil {
						log.Printf("[AuthHandler] Error getting tenant info for %s: %v", st.ID, err)
						// Still add with what we have
						tenants = append(tenants, services.TenantAuthInfo{
							ID:          st.ID,
							Role:        st.Role,
							DisplayName: st.DisplayName,
						})
					} else {
						tenants = append(tenants, services.TenantAuthInfo{
							ID:          st.ID,
							Slug:        tenantInfo.Slug,
							Name:        tenantInfo.Name,
							DisplayName: tenantInfo.DisplayName,
							LogoURL:     tenantInfo.LogoURL,
							Role:        st.Role,
						})
					}
				}
			}
		}
	}

	// Don't reveal if user doesn't exist - just return empty array
	SuccessResponse(c, http.StatusOK, "User tenants retrieved", gin.H{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// ChangePasswordRequest represents a request to change password for a specific tenant
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	TenantID        string `json:"tenant_id" binding:"required"`
}

// ChangePassword changes a user's password for a specific tenant
// POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get user ID from context (set by auth middleware)
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

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// Change password
	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, tenantID, req.CurrentPassword, req.NewPassword, &userID); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Failed to change password", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Password changed successfully", nil)
}

// SetPasswordRequest represents a request to set password for a specific tenant
type SetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
	TenantID string `json:"tenant_id" binding:"required"`
}

// SetPassword sets a password for a user in a specific tenant
// This is used during password reset flow
// POST /api/v1/auth/set-password
func (h *AuthHandler) SetPassword(c *gin.Context) {
	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get user ID from context (set by auth middleware after token validation)
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

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// Set password
	if err := h.authSvc.SetPassword(c.Request.Context(), userID, tenantID, req.Password, &userID); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Failed to set password", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Password set successfully", nil)
}

// UnlockAccountRequest represents a request to unlock a locked account
type UnlockAccountRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required"`
}

// UnlockAccount unlocks a locked user account (admin operation)
// POST /api/v1/auth/unlock-account
func (h *AuthHandler) UnlockAccount(c *gin.Context) {
	var req UnlockAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get admin user ID from context
	adminUserIDStr := c.GetHeader("X-User-ID")
	if adminUserIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid admin user ID", nil)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// TODO: Add authorization check - only tenant admins should be able to unlock accounts

	// Unlock account
	if err := h.authSvc.UnlockAccount(c.Request.Context(), userID, tenantID, adminUserID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to unlock account", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Account unlocked successfully", nil)
}

// CheckAccountStatus returns the account status for a user in a tenant
// POST /api/v1/auth/account-status
func (h *AuthHandler) CheckAccountStatus(c *gin.Context) {
	var req struct {
		Email      string `json:"email" binding:"required,email"`
		TenantSlug string `json:"tenant_slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// This is a lightweight check without full credential validation
	// Used to show users if their account is locked before they enter password
	result, err := h.authSvc.ValidateCredentials(c.Request.Context(), &services.ValidateCredentialsRequest{
		Email:      req.Email,
		Password:   "dummy-password-for-status-check", // Won't be validated
		TenantSlug: req.TenantSlug,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})

	if err != nil {
		// Return generic response to avoid info leakage
		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"account_exists": false,
		})
		return
	}

	// Only return account locked status, not whether credentials are valid
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"account_exists":     result.ErrorCode != "USER_NOT_FOUND" && result.ErrorCode != "NO_ACCESS",
		"account_locked":     result.AccountLocked,
		"locked_until":       result.LockedUntil,
		"remaining_attempts": result.RemainingAttempts,
	})
}
