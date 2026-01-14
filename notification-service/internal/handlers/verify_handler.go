package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"notification-service/internal/services"
)

// VerifyHandler handles OTP/verification-related requests
type VerifyHandler struct {
	verifyService   *services.VerifyService
	devtestEnabled  bool
	testPhoneNumber string
}

// NewVerifyHandler creates a new verification handler
func NewVerifyHandler(verifyService *services.VerifyService, devtestEnabled bool, testPhoneNumber string) *VerifyHandler {
	return &VerifyHandler{
		verifyService:   verifyService,
		devtestEnabled:  devtestEnabled,
		testPhoneNumber: testPhoneNumber,
	}
}

// SendOTPRequest represents a request to send an OTP
type SendOTPRequest struct {
	To      string `json:"to" binding:"required"`       // Phone number (E.164) or email
	Channel string `json:"channel"`                     // sms, email, call, whatsapp (default: sms)
	Locale  string `json:"locale,omitempty"`            // Language locale (e.g., "en")
	Purpose string `json:"purpose,omitempty"`           // account_verification, password_reset, login, etc.
}

// VerifyOTPRequest represents a request to verify an OTP
type VerifyOTPRequest struct {
	To   string `json:"to" binding:"required"`   // Phone number (E.164) or email
	Code string `json:"code" binding:"required"` // The OTP code to verify
}

// ResendOTPRequest represents a request to resend an OTP
type ResendOTPRequest struct {
	To      string `json:"to" binding:"required"` // Phone number (E.164) or email
	Channel string `json:"channel"`               // sms, email, call, whatsapp (default: sms)
}

// CancelOTPRequest represents a request to cancel a pending OTP
type CancelOTPRequest struct {
	To string `json:"to" binding:"required"` // Phone number (E.164) or email
}

// SendOTP sends an OTP to the specified recipient
// POST /api/v1/verify/send
func (h *VerifyHandler) SendOTP(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Missing tenant_id",
		})
		return
	}

	var req SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// DEVTEST mode check - restrict to test phone number only
	if h.devtestEnabled && h.testPhoneNumber != "" && req.To != h.testPhoneNumber {
		log.Printf("[VERIFY-HANDLER] DEVTEST mode: blocked OTP to %s (only %s allowed)", req.To, h.testPhoneNumber)
		c.JSON(http.StatusForbidden, gin.H{
			"success":         false,
			"error":           "DEVTEST mode enabled: OTP can only be sent to test phone number",
			"devtest_enabled": true,
			"allowed_number":  h.testPhoneNumber,
		})
		return
	}

	// Default channel to SMS
	channel := services.VerifyChannelSMS
	if req.Channel != "" {
		channel = services.VerifyChannel(req.Channel)
	}

	log.Printf("[VERIFY-HANDLER] Sending OTP to %s via %s (tenant: %s, devtest: %v)", req.To, channel, tenantID, h.devtestEnabled)

	resp, err := h.verifyService.SendOTP(c.Request.Context(), req.To, channel)
	if err != nil {
		log.Printf("[VERIFY-HANDLER] Failed to send OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sid":     resp.SID,
			"to":      resp.To,
			"channel": resp.Channel,
			"status":  resp.Status,
		},
		"message": "Verification code sent successfully",
	})
}

// CheckOTP verifies an OTP code
// POST /api/v1/verify/check
func (h *VerifyHandler) CheckOTP(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Missing tenant_id",
		})
		return
	}

	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[VERIFY-HANDLER] Checking OTP for %s (tenant: %s)", req.To, tenantID)

	resp, err := h.verifyService.VerifyOTP(c.Request.Context(), req.To, req.Code)
	if err != nil {
		log.Printf("[VERIFY-HANDLER] OTP check failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
			"valid":   false,
		})
		return
	}

	isApproved := h.verifyService.IsApproved(resp)

	if isApproved {
		log.Printf("[VERIFY-HANDLER] OTP verified successfully for %s", req.To)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"valid":   true,
			"data": gin.H{
				"sid":     resp.SID,
				"to":      resp.To,
				"status":  resp.Status,
				"channel": resp.Channel,
			},
			"message": "Verification successful",
		})
	} else {
		log.Printf("[VERIFY-HANDLER] OTP invalid for %s (status: %s)", req.To, resp.Status)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"valid":   false,
			"data": gin.H{
				"sid":    resp.SID,
				"to":     resp.To,
				"status": resp.Status,
			},
			"message": "Invalid verification code",
		})
	}
}

// ResendOTP resends an OTP to the specified recipient
// POST /api/v1/verify/resend
func (h *VerifyHandler) ResendOTP(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Missing tenant_id",
		})
		return
	}

	var req ResendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// DEVTEST mode check - restrict to test phone number only
	if h.devtestEnabled && h.testPhoneNumber != "" && req.To != h.testPhoneNumber {
		log.Printf("[VERIFY-HANDLER] DEVTEST mode: blocked resend OTP to %s (only %s allowed)", req.To, h.testPhoneNumber)
		c.JSON(http.StatusForbidden, gin.H{
			"success":         false,
			"error":           "DEVTEST mode enabled: OTP can only be sent to test phone number",
			"devtest_enabled": true,
			"allowed_number":  h.testPhoneNumber,
		})
		return
	}

	// Default channel to SMS
	channel := services.VerifyChannelSMS
	if req.Channel != "" {
		channel = services.VerifyChannel(req.Channel)
	}

	log.Printf("[VERIFY-HANDLER] Resending OTP to %s via %s (tenant: %s, devtest: %v)", req.To, channel, tenantID, h.devtestEnabled)

	resp, err := h.verifyService.ResendOTP(c.Request.Context(), req.To, channel)
	if err != nil {
		log.Printf("[VERIFY-HANDLER] Failed to resend OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sid":     resp.SID,
			"to":      resp.To,
			"channel": resp.Channel,
			"status":  resp.Status,
		},
		"message": "Verification code resent successfully",
	})
}

// CancelOTP cancels a pending OTP verification
// POST /api/v1/verify/cancel
func (h *VerifyHandler) CancelOTP(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Missing tenant_id",
		})
		return
	}

	var req CancelOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[VERIFY-HANDLER] Cancelling OTP for %s (tenant: %s)", req.To, tenantID)

	err := h.verifyService.CancelOTP(c.Request.Context(), req.To)
	if err != nil {
		log.Printf("[VERIFY-HANDLER] Failed to cancel OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Verification cancelled",
	})
}

// GetAuthMode returns the current Twilio authentication mode (for debugging)
// GET /api/v1/verify/auth-mode
func (h *VerifyHandler) GetAuthMode(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Missing tenant_id",
		})
		return
	}

	provider := h.verifyService.GetProvider()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"authMode":        provider.GetAuthMode(),
			"testPhoneNumber": provider.GetTestPhoneNumber(),
			"devtestEnabled":  h.devtestEnabled,
		},
	})
}
