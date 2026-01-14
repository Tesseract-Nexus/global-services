package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"auth-service/internal/clients"
	"auth-service/internal/events"
	"auth-service/internal/models"
	"auth-service/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PasswordHandlers handles password-related authentication
type PasswordHandlers struct {
	authService        *services.AuthService
	passwordService    *services.PasswordService
	emailService       *services.EmailService // We'll create this next
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
	eventsPublisher    *events.Publisher
}

// NewPasswordHandlers creates a new password handlers instance
func NewPasswordHandlers(authService *services.AuthService, passwordService *services.PasswordService, emailService *services.EmailService, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient, eventsPublisher *events.Publisher) *PasswordHandlers {
	return &PasswordHandlers{
		authService:        authService,
		passwordService:    passwordService,
		emailService:       emailService,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
		eventsPublisher:    eventsPublisher,
	}
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email            string  `json:"email" binding:"required,email"`
	Password         string  `json:"password" binding:"required,min=8"`
	Name             string  `json:"name" binding:"required"`
	PhoneNumber      *string `json:"phone_number"`
	MarketingConsent bool    `json:"marketing_consent"`
	StoreID          *string `json:"store_id"`  // For multi-tenant stores
	TenantID         string  `json:"tenant_id" binding:"required"`
	EmailVerified    bool    `json:"email_verified"` // Pre-verified via onboarding flow
}

// PasswordLoginRequest represents a password-based login request
type PasswordLoginRequest struct {
	Email      string  `json:"email" binding:"required,email"`
	Password   string  `json:"password" binding:"required"`
	StoreID    *string `json:"store_id"`
	TenantID   string  `json:"tenant_id" binding:"required"`
	RememberMe bool    `json:"remember_me"`
}

// ForgotPasswordRequest represents a forgot password request
type ForgotPasswordRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	StoreID  *string `json:"store_id"`
	TenantID string  `json:"tenant_id" binding:"required"`
}

// ResetPasswordRequest represents a password reset request
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ResendVerificationRequest represents a request to resend email verification
type ResendVerificationRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	StoreID  *string `json:"store_id"`
	TenantID string  `json:"tenant_id" binding:"required"`
}

// Register handles user registration with password
func (h *PasswordHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Parse tenant ID first
	tenantUUID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid tenant ID format",
		})
		return
	}

	// Check if user already exists with this email
	// For multi-tenant, we check by email only (users can belong to multiple tenants)
	existingUser, err := h.authService.GetUserByEmail(req.Email)
	if err == nil && existingUser != nil {
		// User already exists - return their ID for idempotency
		// This prevents duplicate users when onboarding is retried
		c.JSON(http.StatusOK, gin.H{
			"message":    "User already exists",
			"user_id":    existingUser.ID,
			"email_sent": false,
			"existing":   true,
		})
		return
	}

	// Validate password strength
	if err := h.passwordService.ValidatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password does not meet requirements",
			"details": err.Error(),
		})
		return
	}

	// Hash password
	hashedPassword, err := h.passwordService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process password",
			"details": err.Error(),
		})
		return
	}

	// Split name into first and last name (simple split on space)
	firstName := req.Name
	lastName := ""
	if parts := splitName(req.Name); len(parts) > 1 {
		firstName = parts[0]
		lastName = parts[1]
	}

	// Create user
	user := &models.User{
		Email:         req.Email,
		FirstName:     firstName,
		LastName:      lastName,
		Password:      hashedPassword, // Store hashed password in password column
		Phone:         req.PhoneNumber,
		TenantID:      tenantUUID,
		Role:          models.RoleCustomer, // Default role
		Status:        "active",
		EmailVerified: req.EmailVerified, // Pre-verified if coming from onboarding flow
	}

	// Register user
	createdUser, err := h.authService.RegisterUser(user)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Registration failed",
			"details": err.Error(),
		})
		return
	}

	// Send user registered notification (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := &clients.AuthNotification{
				TenantID:      req.TenantID,
				UserID:        createdUser.ID.String(),
				UserEmail:     createdUser.Email,
				UserName:      req.Name,
				EventType:     "REGISTERED",
				StorefrontURL: h.tenantClient.BuildStorefrontURL(ctx, req.TenantID),
			}

			if err := h.notificationClient.SendUserRegisteredNotification(ctx, notification); err != nil {
				log.Printf("[AUTH] Failed to send user registered notification: %v", err)
			}
		}()
	}

	// Generate email verification token
	verificationToken, err := h.passwordService.GenerateVerificationToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate verification token",
		})
		return
	}

	// Save verification token
	err = h.authService.SaveVerificationToken(createdUser.ID, verificationToken, "email_verification", h.passwordService.GetEmailVerificationExpiry())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save verification token",
		})
		return
	}

	// Send verification email
	err = h.emailService.SendVerificationEmail(createdUser.Email, createdUser.Name, verificationToken)
	if err != nil {
		// Log error but don't fail registration
		// User can resend verification email later
		c.JSON(http.StatusCreated, gin.H{
			"message":    "User registered successfully. Please check your email for verification (email sending failed, you can resend it).",
			"user_id":    createdUser.ID,
			"email_sent": false,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User registered successfully. Please check your email for verification.",
		"user_id":    createdUser.ID,
		"email_sent": true,
	})
}

// PasswordLogin handles password-based login
func (h *PasswordHandlers) PasswordLogin(c *gin.Context) {
	var req PasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Parse store ID if provided
	var storeID *uuid.UUID
	if req.StoreID != nil && *req.StoreID != "" {
		parsed, err := uuid.Parse(*req.StoreID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid store ID format",
			})
			return
		}
		storeID = &parsed
	}

	// Get client info
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Authenticate with password
	user, accessToken, refreshToken, err := h.authService.AuthenticateWithPassword(
		req.Email, req.Password, storeID, req.TenantID, ipAddress, userAgent,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid credentials",
			"details": err.Error(),
		})
		return
	}

	// Check if email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "Email not verified",
			"message":        "Please verify your email before logging in",
			"email_verified": false,
		})
		return
	}

	// Update last login
	err = h.authService.UpdateLastLogin(user.ID)
	if err != nil {
		// Log error but don't fail login
	}

	// Return response
	response := LoginResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.authService.GetTokenExpiry().Seconds()),
	}

	// Set longer expiry for "remember me"
	if req.RememberMe {
		response.ExpiresIn = int((24 * time.Hour * 30).Seconds()) // 30 days
	}

	c.JSON(http.StatusOK, response)
}

// ForgotPassword initiates password reset process
func (h *PasswordHandlers) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Parse store ID if provided
	var storeID *uuid.UUID
	if req.StoreID != nil && *req.StoreID != "" {
		parsed, err := uuid.Parse(*req.StoreID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid store ID format",
			})
			return
		}
		storeID = &parsed
	}

	// Find user by email and store
	user, err := h.authService.GetUserByEmailAndStore(req.Email, storeID, req.TenantID)
	if err != nil {
		// Return success even if user not found for security
		c.JSON(http.StatusOK, gin.H{
			"message": "If the email exists, a password reset link has been sent.",
		})
		return
	}

	// Generate reset token
	resetToken, err := h.passwordService.GenerateResetToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate reset token",
		})
		return
	}

	// Save reset token
	err = h.authService.SaveVerificationToken(user.ID, resetToken, "password_reset", h.passwordService.GetPasswordResetExpiry())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save reset token",
		})
		return
	}

	// Send reset email
	err = h.emailService.SendPasswordResetEmail(user.Email, user.Name, resetToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to send reset email",
		})
		return
	}

	// Send password reset notification (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := &clients.AuthNotification{
				TenantID:  req.TenantID,
				UserID:    user.ID.String(),
				UserEmail: user.Email,
				UserName:  user.Name,
				EventType: "PASSWORD_RESET",
				ResetCode: resetToken,
				ResetURL:  h.tenantClient.BuildPasswordResetURL(ctx, req.TenantID, resetToken),
			}

			if err := h.notificationClient.SendPasswordResetNotification(ctx, notification); err != nil {
				log.Printf("[AUTH] Failed to send password reset notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset link has been sent to your email.",
	})
}

// ResetPassword resets user password with token
func (h *PasswordHandlers) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate password strength
	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password does not meet requirements",
			"details": err.Error(),
		})
		return
	}

	// Verify reset token
	userID, err := h.authService.VerifyToken(req.Token, "password_reset")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid or expired reset token",
			"details": err.Error(),
		})
		return
	}

	// Hash new password
	hashedPassword, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process password",
		})
		return
	}

	// Update password
	err = h.authService.UpdatePassword(userID, hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update password",
			"details": err.Error(),
		})
		return
	}

	// Mark token as used
	err = h.authService.MarkTokenAsUsed(req.Token)
	if err != nil {
		// Log error but don't fail the operation
	}

	// Revoke all existing sessions for security
	err = h.authService.RevokeAllUserSessions(userID)
	if err != nil {
		// Log error but don't fail the operation
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully. Please log in with your new password.",
	})
}

// ChangePassword changes the password for an authenticated user
func (h *PasswordHandlers) ChangePassword(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized - user ID not found in context",
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Parse user ID
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	// Get user to verify current password
	user, err := h.authService.GetUserByID(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	// Verify current password
	if err := h.passwordService.VerifyPassword(req.CurrentPassword, user.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Current password is incorrect",
		})
		return
	}

	// Validate new password strength
	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "New password does not meet requirements",
			"details": err.Error(),
		})
		return
	}

	// Check if new password is same as current
	if req.CurrentPassword == req.NewPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "New password must be different from current password",
		})
		return
	}

	// Hash new password
	hashedPassword, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process password",
		})
		return
	}

	// Update password
	err = h.authService.UpdatePassword(uid, hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update password",
			"details": err.Error(),
		})
		return
	}

	// Optionally revoke all other sessions for security
	// We'll keep the current session active but revoke others
	err = h.authService.RevokeOtherUserSessions(uid, c.GetString("session_id"))
	if err != nil {
		// Log error but don't fail the operation
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been changed successfully.",
	})
}

// VerifyEmail verifies user email with token
func (h *PasswordHandlers) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Verification token is required",
		})
		return
	}

	// Verify email token
	userID, err := h.authService.VerifyToken(token, "email_verification")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid or expired verification token",
			"details": err.Error(),
		})
		return
	}

	// Mark email as verified
	err = h.authService.MarkEmailAsVerified(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to verify email",
			"details": err.Error(),
		})
		return
	}

	// Get user details for notification (email verification confirmation is optional)
	user, _ := h.authService.GetUserByID(userID)
	if h.notificationClient != nil && user != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := &clients.AuthNotification{
				TenantID:  user.TenantID.String(),
				UserID:    userID.String(),
				UserEmail: user.Email,
				UserName:  user.Name,
				EventType: "EMAIL_VERIFIED",
			}

			if err := h.notificationClient.SendEmailVerifiedNotification(ctx, notification); err != nil {
				log.Printf("[AUTH] Failed to send email verified notification: %v", err)
			}
		}()
	}

	// Mark token as used
	err = h.authService.MarkTokenAsUsed(token)
	if err != nil {
		// Log error but don't fail the operation
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Email verified successfully. You can now log in.",
		"email_verified": true,
	})
}

// ResendVerification resends email verification
func (h *PasswordHandlers) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Parse store ID if provided
	var storeID *uuid.UUID
	if req.StoreID != nil && *req.StoreID != "" {
		parsed, err := uuid.Parse(*req.StoreID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid store ID format",
			})
			return
		}
		storeID = &parsed
	}

	// Find user
	user, err := h.authService.GetUserByEmailAndStore(req.Email, storeID, req.TenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email is already verified",
		})
		return
	}

	// Generate new verification token
	verificationToken, err := h.passwordService.GenerateVerificationToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate verification token",
		})
		return
	}

	// Save verification token
	err = h.authService.SaveVerificationToken(user.ID, verificationToken, "email_verification", h.passwordService.GetEmailVerificationExpiry())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save verification token",
		})
		return
	}

	// Send verification email
	err = h.emailService.SendVerificationEmail(user.Email, user.Name, verificationToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to send verification email",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification email has been sent.",
	})
}

// splitName splits a full name into first and last name
func splitName(fullName string) []string {
	// Simple split on first space
	spaceIdx := -1
	for i, c := range fullName {
		if c == ' ' {
			spaceIdx = i
			break
		}
	}

	if spaceIdx == -1 {
		return []string{fullName}
	}

	firstName := fullName[:spaceIdx]
	lastName := fullName[spaceIdx+1:]
	return []string{firstName, lastName}
}
