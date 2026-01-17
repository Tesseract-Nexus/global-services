package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/redis"
	"tenant-service/internal/services"
)

// TestHandler provides endpoints for E2E testing
// These endpoints are only available in dev/test environments
type TestHandler struct {
	verificationService *services.VerificationService
	onboardingService   *services.OnboardingService
	redisClient         *redis.Client
}

// NewTestHandler creates a new test handler
func NewTestHandler(
	verificationService *services.VerificationService,
	onboardingService *services.OnboardingService,
	redisClient *redis.Client,
) *TestHandler {
	return &TestHandler{
		verificationService: verificationService,
		onboardingService:   onboardingService,
		redisClient:         redisClient,
	}
}

// IsTestMode returns true if the service is running in test/dev mode
func IsTestMode() bool {
	env := os.Getenv("ENVIRONMENT")
	return env == "development" || env == "dev" || env == "test" || env == "devtest" || env == ""
}

// VerifyEmailForTest directly marks an email as verified for testing purposes
// POST /api/v1/test/verify-email
func (h *TestHandler) VerifyEmailForTest(c *gin.Context) {
	if !IsTestMode() {
		ErrorResponse(c, http.StatusForbidden, "Test endpoints are only available in dev/test environments", nil)
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	// Mark the email verification task as completed
	if err := h.onboardingService.CompleteEmailVerificationTask(c.Request.Context(), sessionID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to mark email as verified", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Email verified for testing", map[string]interface{}{
		"session_id": sessionID.String(),
		"email":      req.Email,
		"verified":   true,
	})
}

// GetVerificationTokenForTest retrieves the verification token for a session
// GET /api/v1/test/verification-token
func (h *TestHandler) GetVerificationTokenForTest(c *gin.Context) {
	if !IsTestMode() {
		ErrorResponse(c, http.StatusForbidden, "Test endpoints are only available in dev/test environments", nil)
		return
	}

	sessionIDStr := c.Query("sessionId")
	email := c.Query("email")

	if sessionIDStr == "" || email == "" {
		ErrorResponse(c, http.StatusBadRequest, "sessionId and email are required", nil)
		return
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	// Get verification token from Redis
	if h.redisClient == nil {
		ErrorResponse(c, http.StatusServiceUnavailable, "Redis not available", nil)
		return
	}

	// Try to find the token in Redis by scanning keys
	token, err := h.redisClient.GetVerificationTokenBySession(c.Request.Context(), sessionID.String(), email)
	if err != nil {
		ErrorResponse(c, http.StatusNotFound, "Verification token not found", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Verification token retrieved", map[string]interface{}{
		"session_id": sessionID.String(),
		"email":      email,
		"token":      token,
	})
}

// GetOTPForTest retrieves the OTP code for a session (if OTP method is used)
// GET /api/v1/test/otp
func (h *TestHandler) GetOTPForTest(c *gin.Context) {
	if !IsTestMode() {
		ErrorResponse(c, http.StatusForbidden, "Test endpoints are only available in dev/test environments", nil)
		return
	}

	sessionIDStr := c.Query("sessionId")
	email := c.Query("email")

	if sessionIDStr == "" || email == "" {
		ErrorResponse(c, http.StatusBadRequest, "sessionId and email are required", nil)
		return
	}

	// For OTP, we would need to retrieve from verification service
	// Since we're using link-based verification, return a helpful message
	ErrorResponse(c, http.StatusNotImplemented, "OTP retrieval not implemented - using link-based verification", nil)
}
