package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/services"
)

// VerificationHandler handles verification HTTP requests
type VerificationHandler struct {
	verificationService *services.VerificationService
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(verificationService *services.VerificationService) *VerificationHandler {
	return &VerificationHandler{
		verificationService: verificationService,
	}
}

// SendCode sends a verification code
func (h *VerificationHandler) SendCode(c *gin.Context) {
	var req models.SendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	response, err := h.verificationService.SendVerificationCode(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to send verification code", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification code sent successfully", response)
}

// VerifyCode verifies a verification code
func (h *VerificationHandler) VerifyCode(c *gin.Context) {
	var req models.VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	response, err := h.verificationService.VerifyCode(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to verify code", err)
		return
	}

	if !response.Verified {
		c.JSON(http.StatusOK, models.APIResponse{
			Success: false,
			Message: response.Message,
			Data:    response,
		})
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification successful", response)
}

// ResendCode resends a verification code
func (h *VerificationHandler) ResendCode(c *gin.Context) {
	var req models.ResendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	response, err := h.verificationService.ResendCode(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to resend verification code", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification code resent successfully", response)
}

// GetStatus retrieves verification status
func (h *VerificationHandler) GetStatus(c *gin.Context) {
	recipient := c.Query("recipient")
	purpose := c.Query("purpose")

	if recipient == "" || purpose == "" {
		ErrorResponse(c, http.StatusBadRequest, "recipient and purpose query parameters are required", nil)
		return
	}

	req := &models.CheckStatusRequest{
		Recipient: recipient,
		Purpose:   purpose,
	}

	response, err := h.verificationService.GetVerificationStatus(c.Request.Context(), req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get verification status", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification status retrieved successfully", response)
}

// SendEmail sends a custom email (welcome, account created, etc.)
func (h *VerificationHandler) SendEmail(c *gin.Context) {
	var req models.SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	err := h.verificationService.SendCustomEmail(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to send email", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Email sent successfully", nil)
}
