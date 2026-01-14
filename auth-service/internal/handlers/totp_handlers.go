package handlers

import (
	"net/http"
	"time"

	"auth-service/internal/middleware"
	"auth-service/internal/services"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// TOTPHandlers handles TOTP/2FA related requests
type TOTPHandlers struct {
	authService     *services.AuthService
	totpService     *services.TOTPService
	passwordService *services.PasswordService
	emailService    *services.EmailService
}

// NewTOTPHandlers creates a new TOTP handlers instance
func NewTOTPHandlers(authService *services.AuthService, totpService *services.TOTPService, passwordService *services.PasswordService, emailService *services.EmailService) *TOTPHandlers {
	return &TOTPHandlers{
		authService:     authService,
		totpService:     totpService,
		passwordService: passwordService,
		emailService:    emailService,
	}
}

// Enable2FARequest represents a request to enable 2FA
type Enable2FARequest struct {
	Password string `json:"password" binding:"required"`
}

// Verify2FASetupRequest represents a request to verify 2FA setup
type Verify2FASetupRequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify2FARequest represents a 2FA verification request during login
type Verify2FARequest struct {
	Code           string `json:"code" binding:"required"`
	BackupCode     string `json:"backup_code"`
	SessionID      string `json:"session_id" binding:"required"`
	RememberDevice bool   `json:"remember_device"`
}

// Disable2FARequest represents a request to disable 2FA
type Disable2FARequest struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code"`
}

// RegenerateBackupCodesRequest represents a request to regenerate backup codes
type RegenerateBackupCodesRequest struct {
	Password string `json:"password" binding:"required"`
}

// TwoFactorSetupResponse represents the response when setting up 2FA
type TwoFactorSetupResponse struct {
	Secret       string   `json:"secret"`
	QRCodeURL    string   `json:"qr_code_url"`
	BackupCodes  []string `json:"backup_codes"`
	Instructions string   `json:"instructions"`
}

// TwoFactorStatusResponse represents the 2FA status for a user
type TwoFactorStatusResponse struct {
	Enabled              bool       `json:"enabled"`
	VerifiedAt           *time.Time `json:"verified_at"`
	BackupCodesRemaining int        `json:"backup_codes_remaining"`
	BackupCodesGenerated *time.Time `json:"backup_codes_generated_at"`
}

// Enable2FA initiates 2FA setup for a user
func (h *TOTPHandlers) Enable2FA(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	var req Enable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user and verify password
	user, err := h.authService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user",
		})
		return
	}

	// Verify password
	if user.PasswordHash == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password authentication required",
		})
		return
	}

	if err := h.passwordService.VerifyPassword(req.Password, *user.PasswordHash); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid password",
		})
		return
	}

	// Check if 2FA is already enabled
	if user.TwoFactorEnabled {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Two-factor authentication is already enabled",
		})
		return
	}

	// Generate TOTP secret and QR code
	totpSecret, err := h.totpService.GenerateSecret(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate 2FA secret",
		})
		return
	}

	// Store the secret temporarily (not yet enabled)
	err = h.authService.StoreTempTOTPSecret(userID, totpSecret.Secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to store 2FA secret",
		})
		return
	}

	// Hash and store backup codes
	hashedBackupCodes := make([]string, len(totpSecret.BackupCodes))
	for i, code := range totpSecret.BackupCodes {
		hashed, err := bcrypt.GenerateFromPassword([]byte(code), 12)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate backup codes",
			})
			return
		}
		hashedBackupCodes[i] = string(hashed)
	}

	err = h.authService.StoreTempBackupCodes(userID, hashedBackupCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to store backup codes",
		})
		return
	}

	c.JSON(http.StatusOK, TwoFactorSetupResponse{
		Secret:       totpSecret.Secret,
		QRCodeURL:    totpSecret.QRCodeURL,
		BackupCodes:  totpSecret.BackupCodes,
		Instructions: "1. Scan the QR code with your authenticator app (Google Authenticator, Authy, etc.) or manually enter the secret. 2. Enter the 6-digit code from your app to verify setup. 3. Save your backup codes in a secure location.",
	})
}

// Verify2FASetup verifies the 2FA setup and enables it
func (h *TOTPHandlers) Verify2FASetup(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	var req Verify2FASetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get temporary TOTP secret
	tempSecret, err := h.authService.GetTempTOTPSecret(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No pending 2FA setup found. Please start the setup process again.",
		})
		return
	}

	// Verify the code
	valid, err := h.totpService.VerifySetup(tempSecret, req.Code)
	if err != nil || !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid verification code",
		})
		return
	}

	// Enable 2FA permanently
	err = h.authService.Enable2FA(userID, tempSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to enable 2FA",
		})
		return
	}

	// Move temp backup codes to permanent storage
	err = h.authService.ActivateBackupCodes(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to activate backup codes",
		})
		return
	}

	// Clean up temporary data
	h.authService.ClearTempTOTPData(userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Two-factor authentication has been successfully enabled",
		"enabled": true,
	})
}

// Verify2FA verifies a 2FA code during login
func (h *TOTPHandlers) Verify2FA(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get session information
	session, err := h.authService.GetSessionByID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid session",
		})
		return
	}

	// Get user
	user, err := h.authService.GetUser(session.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user",
		})
		return
	}

	if !user.TwoFactorEnabled || user.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Two-factor authentication is not enabled",
		})
		return
	}

	var verified bool
	var usedBackupCode bool

	// Try TOTP code first
	if req.Code != "" {
		verified = h.totpService.ValidateCode(user.TOTPSecret, req.Code)
		if verified {
			h.authService.LogTwoFactorAttempt(user.ID, "totp_code", true, c.ClientIP(), c.GetHeader("User-Agent"))
		}
	}

	// If TOTP failed and backup code provided, try backup code
	if !verified && req.BackupCode != "" {
		backupCodes, err := h.authService.GetUserBackupCodes(user.ID)
		if err == nil {
			for _, code := range backupCodes {
				if code.UsedAt == nil && h.totpService.ValidateBackupCode(req.BackupCode, code.CodeHash) {
					// Mark backup code as used
					err = h.authService.UseBackupCode(code.ID)
					if err == nil {
						verified = true
						usedBackupCode = true
						h.authService.LogTwoFactorAttempt(user.ID, "backup_code", true, c.ClientIP(), c.GetHeader("User-Agent"))
						break
					}
				}
			}
		}
	}

	// Log failed attempt
	if !verified {
		attemptType := "totp_code"
		if req.BackupCode != "" {
			attemptType = "backup_code"
		}
		h.authService.LogTwoFactorAttempt(user.ID, attemptType, false, c.ClientIP(), c.GetHeader("User-Agent"))

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid verification code",
		})
		return
	}

	// Update session to mark 2FA as verified
	err = h.authService.Mark2FAVerified(session.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update session",
		})
		return
	}

	// Update user's last 2FA verification time
	err = h.authService.UpdateLast2FAVerification(user.ID)
	if err != nil {
		// Log error but don't fail the request
	}

	response := gin.H{
		"message":    "Two-factor authentication verified successfully",
		"verified":   true,
		"session_id": session.ID,
	}

	if usedBackupCode {
		// Get remaining backup codes count
		remaining, _ := h.authService.GetBackupCodesCount(user.ID)
		response["backup_code_used"] = true
		response["backup_codes_remaining"] = remaining

		if remaining <= 2 {
			response["warning"] = "You have few backup codes remaining. Consider generating new ones."
		}
	}

	c.JSON(http.StatusOK, response)
}

// Disable2FA disables two-factor authentication
func (h *TOTPHandlers) Disable2FA(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	var req Disable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user and verify password
	user, err := h.authService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user",
		})
		return
	}

	// Verify password
	if user.PasswordHash == nil || h.passwordService.VerifyPassword(req.Password, *user.PasswordHash) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid password",
		})
		return
	}

	// If 2FA is enabled, require TOTP code for additional security
	if user.TwoFactorEnabled && req.Code != "" {
		if !h.totpService.ValidateCode(user.TOTPSecret, req.Code) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid 2FA code",
			})
			return
		}
	}

	// Disable 2FA
	err = h.authService.Disable2FA(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to disable 2FA",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Two-factor authentication has been disabled",
		"enabled": false,
	})
}

// Get2FAStatus returns the current 2FA status for the user
func (h *TOTPHandlers) Get2FAStatus(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	status, err := h.authService.Get2FAStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get 2FA status",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// RegenerateBackupCodes generates new backup codes
func (h *TOTPHandlers) RegenerateBackupCodes(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	var req RegenerateBackupCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user and verify password
	user, err := h.authService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user",
		})
		return
	}

	// Verify password
	if user.PasswordHash == nil || h.passwordService.VerifyPassword(req.Password, *user.PasswordHash) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid password",
		})
		return
	}

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Two-factor authentication is not enabled",
		})
		return
	}

	// Generate new backup codes
	newCodes, err := h.totpService.GenerateBackupCodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate backup codes",
		})
		return
	}

	// Hash the codes before storing
	hashedCodes := make([]string, len(newCodes))
	for i, code := range newCodes {
		hashed, err := bcrypt.GenerateFromPassword([]byte(code), 12)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to process backup codes",
			})
			return
		}
		hashedCodes[i] = string(hashed)
	}

	// Replace old backup codes with new ones
	err = h.authService.ReplaceBackupCodes(userID, hashedCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to store backup codes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "New backup codes generated successfully",
		"backup_codes": newCodes,
		"warning":      "Save these codes in a secure location. They won't be shown again.",
	})
}

// GetRecoveryInfo provides information for account recovery
func (h *TOTPHandlers) GetRecoveryInfo(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	recoveryInfo, err := h.authService.GetRecoveryInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get recovery information",
		})
		return
	}

	c.JSON(http.StatusOK, recoveryInfo)
}
