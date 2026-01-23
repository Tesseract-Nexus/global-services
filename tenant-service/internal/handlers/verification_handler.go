package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/services"
)

// VerificationHandler handles verification HTTP requests
type VerificationHandler struct {
	verificationService *services.VerificationService
	onboardingService   *services.OnboardingService
	draftService        *services.DraftService
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(verificationService *services.VerificationService, onboardingService *services.OnboardingService) *VerificationHandler {
	return &VerificationHandler{
		verificationService: verificationService,
		onboardingService:   onboardingService,
	}
}

// SetDraftService sets the draft service (optional, used for cleanup after verification)
func (h *VerificationHandler) SetDraftService(draftService *services.DraftService) {
	h.draftService = draftService
}

// StartEmailVerification starts email verification process
func (h *VerificationHandler) StartEmailVerification(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	record, err := h.verificationService.StartEmailVerification(c.Request.Context(), sessionID, req.Email)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to start email verification", err)
		return
	}

	// Return without sensitive data
	response := map[string]interface{}{
		"id":                record.ID,
		"verification_type": record.VerificationType,
		"status":            record.Status,
		"expires_at":        record.ExpiresAt,
		"max_attempts":      record.MaxAttempts,
	}

	SuccessResponse(c, http.StatusOK, "Email verification started", response)
}

// StartPhoneVerification starts phone verification process
func (h *VerificationHandler) StartPhoneVerification(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req struct {
		Phone string `json:"phone" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	record, err := h.verificationService.StartPhoneVerification(c.Request.Context(), sessionID, req.Phone)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to start phone verification", err)
		return
	}

	// Return without sensitive data
	response := map[string]interface{}{
		"id":                record.ID,
		"verification_type": record.VerificationType,
		"status":            record.Status,
		"expires_at":        record.ExpiresAt,
		"max_attempts":      record.MaxAttempts,
	}

	SuccessResponse(c, http.StatusOK, "Phone verification started", response)
}

// VerifyCode verifies a verification code
func (h *VerificationHandler) VerifyCode(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Get the onboarding session to retrieve the contact email
	session, err := h.onboardingService.GetOnboardingSession(c.Request.Context(), sessionID, []string{"contact_information"})
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Session not found", err)
		return
	}

	// Get contact information to retrieve email
	if len(session.ContactInformation) == 0 || session.ContactInformation[0].Email == "" {
		ErrorResponse(c, http.StatusBadRequest, "No email found for verification", nil)
		return
	}

	record, err := h.verificationService.VerifyCodeWithRecipient(
		c.Request.Context(),
		sessionID,
		session.ContactInformation[0].Email,
		req.Code,
		"email_verification",
	)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Verification failed", err)
		return
	}

	// Mark email_verification task as completed and update session progress
	if err := h.onboardingService.CompleteTask(c.Request.Context(), sessionID, "email_verification"); err != nil {
		// Log warning but don't fail - verification succeeded
		log.Printf("[VerificationHandler] Warning: Failed to mark email_verification task complete for session %s: %v", sessionID, err)
	} else {
		log.Printf("[VerificationHandler] Marked email_verification task complete for session %s", sessionID)
	}

	// Return without sensitive data
	response := map[string]interface{}{
		"id":                record.ID,
		"verification_type": record.VerificationType,
		"status":            record.Status,
		"verified_at":       record.VerifiedAt,
	}

	SuccessResponse(c, http.StatusOK, "Verification successful", response)
}

// ResendVerificationCode resends a verification code
func (h *VerificationHandler) ResendVerificationCode(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req struct {
		VerificationType string `json:"verification_type" binding:"required,oneof=email phone"`
		TargetValue      string `json:"target_value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	record, err := h.verificationService.ResendVerificationCode(c.Request.Context(), sessionID, req.VerificationType, req.TargetValue)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to resend verification code", err)
		return
	}

	// Return without sensitive data
	response := map[string]interface{}{
		"id":                record.ID,
		"verification_type": record.VerificationType,
		"status":            record.Status,
		"expires_at":        record.ExpiresAt,
		"max_attempts":      record.MaxAttempts,
	}

	SuccessResponse(c, http.StatusOK, "Verification code resent", response)
}

// ResendVerificationByEmail resends verification email using only email address
// This is used when a verification link has expired and the user doesn't have the session ID
func (h *VerificationHandler) ResendVerificationByEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Valid email is required", err)
		return
	}

	// Find the pending session by email
	session, err := h.onboardingService.GetPendingSessionByEmail(c.Request.Context(), req.Email)
	if err != nil {
		log.Printf("[VerificationHandler] Error looking up session by email: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to lookup session", err)
		return
	}

	if session == nil {
		// Don't reveal whether email exists - return generic success for security
		log.Printf("[VerificationHandler] No pending session found for email, returning success anyway")
		SuccessResponse(c, http.StatusOK, "If your email is registered, you will receive a new verification link", nil)
		return
	}

	// Resend verification email
	record, err := h.verificationService.ResendVerificationCode(c.Request.Context(), session.ID, "email", req.Email)
	if err != nil {
		log.Printf("[VerificationHandler] Failed to resend verification: %v", err)
		ErrorResponse(c, http.StatusInternalServerError, "Failed to resend verification email", err)
		return
	}

	log.Printf("[VerificationHandler] Resent verification email for session %s", session.ID)

	response := map[string]interface{}{
		"id":         record.ID,
		"status":     record.Status,
		"expires_at": record.ExpiresAt,
	}

	SuccessResponse(c, http.StatusOK, "Verification email sent", response)
}

// GetVerificationStatus gets verification status for a session
func (h *VerificationHandler) GetVerificationStatus(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	status, err := h.verificationService.GetVerificationStatus(c.Request.Context(), sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get verification status", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification status retrieved", status)
}

// CheckVerification checks if a specific verification type is verified
func (h *VerificationHandler) CheckVerification(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	verificationType := c.Param("type")
	if verificationType == "" {
		ErrorResponse(c, http.StatusBadRequest, "Verification type is required", nil)
		return
	}

	isVerified, err := h.verificationService.IsVerified(c.Request.Context(), sessionID, verificationType)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to check verification", err)
		return
	}

	response := map[string]interface{}{
		"verification_type": verificationType,
		"is_verified":       isVerified,
	}

	SuccessResponse(c, http.StatusOK, "Verification check completed", response)
}

// GetVerificationMethod returns the current verification method (otp or link)
func (h *VerificationHandler) GetVerificationMethod(c *gin.Context) {
	method := h.verificationService.GetVerificationMethod()

	response := map[string]interface{}{
		"method": method,
	}

	SuccessResponse(c, http.StatusOK, "Verification method retrieved", response)
}

// VerifyByToken verifies email using a token from verification link
func (h *VerificationHandler) VerifyByToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	record, err := h.verificationService.VerifyByToken(c.Request.Context(), req.Token)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Verification failed", err)
		return
	}

	// Mark the email verification task as completed
	if err := h.onboardingService.CompleteEmailVerificationTask(c.Request.Context(), record.OnboardingSessionID); err != nil {
		// Log but don't fail - verification was successful
		log.Printf("Warning: failed to complete email_verification task: %v", err)
	}

	// Clean up the draft data since onboarding is complete
	// This prevents the recovery prompt from showing for completed sessions
	if h.draftService != nil {
		if err := h.draftService.DeleteDraft(c.Request.Context(), record.OnboardingSessionID); err != nil {
			log.Printf("Warning: failed to delete draft after verification: %v", err)
		} else {
			log.Printf("Draft deleted for completed session: %s", record.OnboardingSessionID)
		}
	}

	response := map[string]interface{}{
		"verification_type":   record.VerificationType,
		"status":              record.Status,
		"verified_at":         record.VerifiedAt,
		"email":               record.TargetValue,
		"session_id":          record.OnboardingSessionID,
		"verified":            true,
		"onboarding_complete": true, // Signal to frontend to clear session
	}

	SuccessResponse(c, http.StatusOK, "Email verified successfully", response)
}

// GetTokenInfo retrieves information about a verification token (GET with query param)
func (h *VerificationHandler) GetTokenInfo(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		ErrorResponse(c, http.StatusBadRequest, "Token is required", nil)
		return
	}

	h.getTokenInfoInternal(c, token)
}

// GetTokenInfoPost retrieves information about a verification token (POST with body)
func (h *VerificationHandler) GetTokenInfoPost(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Token is required", err)
		return
	}

	h.getTokenInfoInternal(c, req.Token)
}

// getTokenInfoInternal contains the shared logic for token info retrieval
func (h *VerificationHandler) getTokenInfoInternal(c *gin.Context, token string) {
	tokenData, err := h.verificationService.GetTokenInfo(c.Request.Context(), token)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Token not found or expired", err)
		return
	}

	// Mask email for display
	email := tokenData.Email
	maskedEmail := maskEmail(email)

	response := map[string]interface{}{
		"email":      maskedEmail,
		"session_id": tokenData.SessionID,
		"expires_at": tokenData.ExpiresAt,
		"valid":      true,
	}

	SuccessResponse(c, http.StatusOK, "Token info retrieved", response)
}

// maskEmail masks an email address for display (e.g., j***n@example.com)
func maskEmail(email string) string {
	parts := make([]string, 2)
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}

	if atIndex == -1 || atIndex < 2 {
		return email
	}

	parts[0] = email[:atIndex]
	parts[1] = email[atIndex:]

	local := parts[0]
	if len(local) <= 2 {
		return email
	}

	masked := string(local[0])
	for i := 1; i < len(local)-1; i++ {
		masked += "*"
	}
	masked += string(local[len(local)-1])

	return masked + parts[1]
}
