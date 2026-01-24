package handlers

import (
	"log"
	"net/http"

	"github.com/Tesseract-Nexus/go-shared/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/services"
)

// AuthHandler handles tenant-aware authentication HTTP requests
// This enables multi-tenant credential isolation where the same email
// can have different passwords for different tenants
type AuthHandler struct {
	authSvc          *services.TenantAuthService
	staffClient      *clients.StaffClient
	deactivationSvc  *services.CustomerDeactivationService
	passwordResetSvc *services.PasswordResetService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authSvc *services.TenantAuthService, staffClient *clients.StaffClient) *AuthHandler {
	return &AuthHandler{
		authSvc:     authSvc,
		staffClient: staffClient,
	}
}

// SetDeactivationService sets the customer deactivation service
func (h *AuthHandler) SetDeactivationService(svc *services.CustomerDeactivationService) {
	h.deactivationSvc = svc
}

// SetPasswordResetService sets the password reset service
func (h *AuthHandler) SetPasswordResetService(svc *services.PasswordResetService) {
	h.passwordResetSvc = svc
}

// ValidateCredentialsRequest represents a request to validate tenant-specific credentials
type ValidateCredentialsRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	TenantID    string `json:"tenant_id"`    // Either tenant_id or tenant_slug required
	TenantSlug  string `json:"tenant_slug"`  // Either tenant_id or tenant_slug required
	AuthContext string `json:"auth_context"` // "customer" or "staff" - controls whether staff fallback is allowed
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
		Email:       req.Email,
		Password:    req.Password,
		TenantID:    tenantID,
		TenantSlug:  req.TenantSlug,
		AuthContext: req.AuthContext, // Pass auth context to control staff fallback
		IPAddress:   clientIP,
		UserAgent:   userAgent,
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
		log.Printf("[AuthHandler] Error getting user tenants for %s: %v", security.MaskEmail(req.Email), err)
		tenants = []services.TenantAuthInfo{} // Continue with empty list
	}

	// Also get tenants from staff table (employees)
	// This allows staff members to login even if they're not in tenant_users
	// Staff-service returns tenant IDs and we enrich with slug/name from our database
	if h.staffClient != nil {
		staffTenants, err := h.staffClient.GetStaffTenants(c.Request.Context(), req.Email)
		if err != nil {
			log.Printf("[AuthHandler] Error getting staff tenants for %s: %v", security.MaskEmail(req.Email), err)
			// Continue without staff tenants
		} else if len(staffTenants) > 0 {
			// Merge staff tenants into the list, avoiding duplicates
			existingTenantIDs := make(map[uuid.UUID]bool)
			for _, t := range tenants {
				existingTenantIDs[t.ID] = true
			}

			for _, st := range staffTenants {
				if !existingTenantIDs[st.ID] {
					// Staff-service returns basic tenant ID - we need to enrich with
					// tenant slug, name, display_name from our database
					tenantInfo := services.TenantAuthInfo{
						ID:   st.ID,
						Role: st.Role,
					}

					// Enrich with tenant details from local database
					if enrichedInfo, err := h.authSvc.GetTenantBasicInfo(c.Request.Context(), st.ID); err == nil && enrichedInfo != nil {
						tenantInfo.Slug = enrichedInfo.Slug
						tenantInfo.Name = enrichedInfo.Name
						tenantInfo.DisplayName = enrichedInfo.DisplayName
						tenantInfo.LogoURL = enrichedInfo.LogoURL
					} else {
						log.Printf("[AuthHandler] Warning: Could not enrich tenant %s for staff: %v", st.ID, err)
						// Skip this tenant if we can't enrich it - likely orphaned record
						continue
					}

					tenants = append(tenants, tenantInfo)
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

	// Get user ID from context (set by IstioAuth middleware from JWT claims)
	userIDVal, _ := c.Get("user_id")
	userIDStr := ""
	if userIDVal != nil {
		userIDStr = userIDVal.(string)
	}
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

	// Get user ID from context (set by IstioAuth middleware from JWT claims)
	userIDVal, _ := c.Get("user_id")
	userIDStr := ""
	if userIDVal != nil {
		userIDStr = userIDVal.(string)
	}
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

	// Get admin user ID from context (set by IstioAuth middleware from JWT claims)
	adminUserIDVal, _ := c.Get("user_id")
	adminUserIDStr := ""
	if adminUserIDVal != nil {
		adminUserIDStr = adminUserIDVal.(string)
	}
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

	canUnlock, err := h.authSvc.CanUnlockAccount(c.Request.Context(), adminUserID, tenantID)
	if err != nil {
		ErrorResponse(c, http.StatusForbidden, "Insufficient permissions", err)
		return
	}
	if !canUnlock {
		ErrorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

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
		Email:                  req.Email,
		Password:               "dummy-password-for-status-check", // Will be ignored
		TenantSlug:             req.TenantSlug,
		IPAddress:              c.ClientIP(),
		UserAgent:              c.GetHeader("User-Agent"),
		SkipPasswordValidation: true,
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

// RegisterCustomerRequest represents a request to register a new customer
type RegisterCustomerRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	FirstName  string `json:"first_name" binding:"required"`
	LastName   string `json:"last_name" binding:"required"`
	Phone      string `json:"phone"`
	TenantSlug string `json:"tenant_slug" binding:"required"`
}

// AcceptInvitationPublicRequest represents a request to accept an invitation without auth
type AcceptInvitationPublicRequest struct {
	Token     string `json:"token" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Phone     string `json:"phone"`
}

// RegisterCustomer registers a new customer for storefront direct registration
// POST /api/v1/auth/register
func (h *AuthHandler) RegisterCustomer(c *gin.Context) {
	var req RegisterCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get client IP and user agent for audit logging
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Register customer
	result, err := h.authSvc.RegisterCustomer(c.Request.Context(), &services.RegisterCustomerRequest{
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Phone:      req.Phone,
		TenantSlug: req.TenantSlug,
		IPAddress:  clientIP,
		UserAgent:  userAgent,
	})

	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to register customer", err)
		return
	}

	if !result.Success {
		// Return error with appropriate status code
		statusCode := http.StatusBadRequest
		if result.ErrorCode == "EMAIL_EXISTS" {
			statusCode = http.StatusConflict
		}

		c.JSON(statusCode, gin.H{
			"success":    false,
			"error_code": result.ErrorCode,
			"message":    result.ErrorMessage,
		})
		return
	}

	// Return successful registration
	response := gin.H{
		"success":     true,
		"user_id":     result.UserID,
		"tenant_id":   result.TenantID,
		"tenant_slug": result.TenantSlug,
		"email":       result.Email,
		"first_name":  result.FirstName,
		"last_name":   result.LastName,
	}

	// Include tokens if they were obtained
	if result.AccessToken != "" {
		response["access_token"] = result.AccessToken
		response["refresh_token"] = result.RefreshToken
		response["id_token"] = result.IDToken
		response["expires_in"] = result.ExpiresIn
	}

	SuccessResponse(c, http.StatusCreated, "Customer registered successfully", response)
}

// AcceptInvitationPublic accepts a tenant invitation and creates the user if needed
// POST /api/v1/invitations/accept-public
func (h *AuthHandler) AcceptInvitationPublic(c *gin.Context) {
	var req AcceptInvitationPublicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	result, err := h.authSvc.AcceptInvitationPublic(c.Request.Context(), &services.AcceptInvitationPublicRequest{
		Token:     req.Token,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	})
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Failed to accept invitation", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Invitation accepted", result)
}

// DeactivateAccountRequest represents a request to deactivate a customer account
type DeactivateAccountRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	TenantID   string `json:"tenant_id"`
	TenantSlug string `json:"tenant_slug"`
	Reason     string `json:"reason"` // Optional reason for deactivation
}

// DeactivateAccount handles customer self-service account deactivation
// POST /api/v1/auth/deactivate-account
func (h *AuthHandler) DeactivateAccount(c *gin.Context) {
	if h.deactivationSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Deactivation service not available", nil)
		return
	}

	var req DeactivateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid user_id format", err)
		return
	}

	// Parse tenant ID
	var tenantID uuid.UUID
	if req.TenantID != "" {
		tenantID, err = uuid.Parse(req.TenantID)
		if err != nil {
			ErrorResponse(c, http.StatusBadRequest, "Invalid tenant_id format", err)
			return
		}
	} else if req.TenantSlug != "" {
		// Get tenant ID from slug using auth service's membership repo
		// For now, require tenant_id directly
		ErrorResponse(c, http.StatusBadRequest, "tenant_id is required", nil)
		return
	} else {
		ErrorResponse(c, http.StatusBadRequest, "Either tenant_id or tenant_slug is required", nil)
		return
	}

	// Deactivate account
	result, err := h.deactivationSvc.DeactivateCustomer(c.Request.Context(), &services.DeactivateCustomerRequest{
		UserID:   userID,
		TenantID: tenantID,
		Reason:   req.Reason,
	})

	if err != nil {
		log.Printf("[AuthHandler] Failed to deactivate account: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to deactivate account", err)
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": result.Message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            result.Message,
		"deactivated_at":     result.DeactivatedAt,
		"scheduled_purge_at": result.ScheduledPurgeAt,
		"days_until_purge":   result.DaysUntilPurge,
	})
}

// CheckDeactivatedRequest represents a request to check if an account is deactivated
type CheckDeactivatedRequest struct {
	Email      string `json:"email" binding:"required,email"`
	TenantSlug string `json:"tenant_slug" binding:"required"`
}

// CheckDeactivatedAccount checks if an account is in deactivated state
// POST /api/v1/auth/check-deactivated
func (h *AuthHandler) CheckDeactivatedAccount(c *gin.Context) {
	if h.deactivationSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Deactivation service not available", nil)
		return
	}

	var req CheckDeactivatedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	result, err := h.deactivationSvc.CheckDeactivatedAccount(c.Request.Context(), req.Email, req.TenantSlug)
	if err != nil {
		log.Printf("[AuthHandler] Failed to check deactivation status: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to check account status", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"is_deactivated":   result.IsDeactivated,
		"can_reactivate":   result.CanReactivate,
		"days_until_purge": result.DaysUntilPurge,
		"deactivated_at":   result.DeactivatedAt,
		"purge_date":       result.PurgeDate,
	})
}

// ReactivateAccountRequest represents a request to reactivate a deactivated account
type ReactivateAccountRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	TenantSlug string `json:"tenant_slug" binding:"required"`
}

// ReactivateAccount reactivates a deactivated account
// POST /api/v1/auth/reactivate-account
func (h *AuthHandler) ReactivateAccount(c *gin.Context) {
	if h.deactivationSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Deactivation service not available", nil)
		return
	}

	var req ReactivateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	result, err := h.deactivationSvc.ReactivateCustomer(c.Request.Context(), &services.ReactivateCustomerRequest{
		Email:      req.Email,
		Password:   req.Password,
		TenantSlug: req.TenantSlug,
	})

	if err != nil {
		log.Printf("[AuthHandler] Failed to reactivate account: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to reactivate account", err)
		return
	}

	if !result.Success {
		statusCode := http.StatusBadRequest
		if result.ErrorCode == "INVALID_PASSWORD" {
			statusCode = http.StatusUnauthorized
		}

		c.JSON(statusCode, gin.H{
			"success":    false,
			"error_code": result.ErrorCode,
			"message":    result.ErrorMessage,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": result.Message,
	})
}

// RequestPasswordResetRequest represents a request to initiate password reset
type RequestPasswordResetRequest struct {
	Email      string `json:"email" binding:"required,email"`
	TenantSlug string `json:"tenant_slug" binding:"required"`
}

// RequestPasswordReset initiates the password reset flow
// POST /api/v1/auth/request-password-reset
func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	if h.passwordResetSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Password reset service not available", nil)
		return
	}

	var req RequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get client IP and user agent for security tracking
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.passwordResetSvc.RequestPasswordReset(c.Request.Context(), &services.RequestPasswordResetInput{
		Email:      req.Email,
		TenantSlug: req.TenantSlug,
		IPAddress:  clientIP,
		UserAgent:  userAgent,
	})

	if err != nil {
		log.Printf("[AuthHandler] Failed to request password reset: %v", err)
		// Don't reveal internal errors - return generic success
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "If an account exists with this email, you will receive a password reset link shortly.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": result.Success,
		"message": result.Message,
	})
}

// ValidateResetTokenRequest represents a request to validate a reset token
type ValidateResetTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// ValidateResetToken validates a password reset token
// POST /api/v1/auth/validate-reset-token
func (h *AuthHandler) ValidateResetToken(c *gin.Context) {
	if h.passwordResetSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Password reset service not available", nil)
		return
	}

	var req ValidateResetTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	result, err := h.passwordResetSvc.ValidateResetToken(c.Request.Context(), &services.ValidateResetTokenInput{
		Token: req.Token,
	})

	if err != nil {
		log.Printf("[AuthHandler] Failed to validate reset token: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"valid":   false,
			"message": "Invalid or expired reset link. Please request a new password reset.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      result.Valid,
		"email":      result.Email,
		"expires_at": result.ExpiresAt,
		"message":    result.Message,
	})
}

// ResetPasswordHandlerRequest represents a request to reset password with token
type ResetPasswordHandlerRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ResetPassword resets the password using a valid token
// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	if h.passwordResetSvc == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Password reset service not available", nil)
		return
	}

	var req ResetPasswordHandlerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get client IP and user agent for security tracking
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.passwordResetSvc.ResetPassword(c.Request.Context(), &services.ResetPasswordInput{
		Token:       req.Token,
		NewPassword: req.NewPassword,
		IPAddress:   clientIP,
		UserAgent:   userAgent,
	})

	if err != nil {
		log.Printf("[AuthHandler] Failed to reset password: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Failed to reset password. Please try again.",
		})
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": result.Message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": result.Message,
	})
}

// ============================================================================
// Progressive Lockout Admin Endpoints
// ============================================================================

// ListLockedAccountsRequest represents a request to list locked accounts
type ListLockedAccountsRequest struct {
	TenantID     string `json:"tenant_id" binding:"required"`
	PermanentOnly bool   `json:"permanent_only"` // If true, only show permanently locked accounts
}

// ListLockedAccounts returns all locked accounts for a tenant (admin operation)
// POST /api/v1/auth/admin/locked-accounts
func (h *AuthHandler) ListLockedAccounts(c *gin.Context) {
	var req ListLockedAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get admin user ID from context (set by IstioAuth middleware from JWT claims)
	adminUserIDVal, _ := c.Get("user_id")
	adminUserIDStr := ""
	if adminUserIDVal != nil {
		adminUserIDStr = adminUserIDVal.(string)
	}
	if adminUserIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid admin user ID", nil)
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// Verify admin has permission
	canUnlock, err := h.authSvc.CanUnlockAccount(c.Request.Context(), adminUserID, tenantID)
	if err != nil || !canUnlock {
		ErrorResponse(c, http.StatusForbidden, "Insufficient permissions to view locked accounts", err)
		return
	}

	// Get locked accounts
	accounts, err := h.authSvc.ListLockedAccounts(c.Request.Context(), tenantID, req.PermanentOnly)
	if err != nil {
		log.Printf("[AuthHandler] Failed to list locked accounts: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to list locked accounts", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Locked accounts retrieved", gin.H{
		"accounts":       accounts,
		"count":          len(accounts),
		"permanent_only": req.PermanentOnly,
	})
}

// GetLockoutStatusRequest represents a request to get detailed lockout status
type GetLockoutStatusRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required"`
}

// GetLockoutStatus returns detailed lockout status for a user (admin operation)
// POST /api/v1/auth/admin/lockout-status
func (h *AuthHandler) GetLockoutStatus(c *gin.Context) {
	var req GetLockoutStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get admin user ID from context (set by IstioAuth middleware from JWT claims)
	adminUserIDVal, _ := c.Get("user_id")
	adminUserIDStr := ""
	if adminUserIDVal != nil {
		adminUserIDStr = adminUserIDVal.(string)
	}
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

	// Verify admin has permission
	canUnlock, err := h.authSvc.CanUnlockAccount(c.Request.Context(), adminUserID, tenantID)
	if err != nil || !canUnlock {
		ErrorResponse(c, http.StatusForbidden, "Insufficient permissions to view lockout status", err)
		return
	}

	// Get detailed lockout status
	status, err := h.authSvc.GetLockoutStatus(c.Request.Context(), userID, tenantID)
	if err != nil {
		log.Printf("[AuthHandler] Failed to get lockout status: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get lockout status", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Lockout status retrieved", status)
}

// GetSecurityPolicyRequest represents a request to get tenant security policy
type GetSecurityPolicyRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
}

// GetSecurityPolicy returns the security policy for a tenant (admin operation)
// POST /api/v1/auth/admin/security-policy
func (h *AuthHandler) GetSecurityPolicy(c *gin.Context) {
	var req GetSecurityPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get admin user ID from context (set by IstioAuth middleware from JWT claims)
	adminUserIDVal, _ := c.Get("user_id")
	adminUserIDStr := ""
	if adminUserIDVal != nil {
		adminUserIDStr = adminUserIDVal.(string)
	}
	if adminUserIDStr == "" {
		ErrorResponse(c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid admin user ID", nil)
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// Verify admin has permission (owner or admin role)
	canUnlock, err := h.authSvc.CanUnlockAccount(c.Request.Context(), adminUserID, tenantID)
	if err != nil || !canUnlock {
		ErrorResponse(c, http.StatusForbidden, "Insufficient permissions to view security policy", err)
		return
	}

	// Get security policy
	policy, err := h.authSvc.GetSecurityPolicy(c.Request.Context(), tenantID)
	if err != nil {
		log.Printf("[AuthHandler] Failed to get security policy: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get security policy", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Security policy retrieved", policy)
}
