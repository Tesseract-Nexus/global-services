package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/clients"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/config"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	tsnats "github.com/tesseract-hub/domains/common/services/tenant-service/internal/nats"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/redis"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/repository"
	"github.com/tesseract-hub/go-shared/security"
)

// VerificationMethod constants
const (
	VerificationMethodOTP  = "otp"
	VerificationMethodLink = "link"
)

// VerificationService handles verification business logic
type VerificationService struct {
	verificationClient *clients.VerificationClient
	notificationClient *clients.NotificationClient
	redisClient        *redis.Client
	verificationConfig config.VerificationConfig
	natsClient         *tsnats.Client
	onboardingRepo     *repository.OnboardingRepository
}

// NewVerificationService creates a new verification service
func NewVerificationService(
	verificationClient *clients.VerificationClient,
	notificationClient *clients.NotificationClient,
	redisClient *redis.Client,
	verificationConfig config.VerificationConfig,
) *VerificationService {
	return &VerificationService{
		verificationClient: verificationClient,
		notificationClient: notificationClient,
		redisClient:        redisClient,
		verificationConfig: verificationConfig,
	}
}

// SetNATSClient sets the NATS client for event-driven verification emails
func (s *VerificationService) SetNATSClient(natsClient *tsnats.Client) {
	s.natsClient = natsClient
}

// SetOnboardingRepo sets the onboarding repository for session lookups
func (s *VerificationService) SetOnboardingRepo(repo *repository.OnboardingRepository) {
	s.onboardingRepo = repo
}

// GetVerificationMethod returns the current verification method
func (s *VerificationService) GetVerificationMethod() string {
	method := s.verificationConfig.Method
	if method != VerificationMethodOTP && method != VerificationMethodLink {
		return VerificationMethodLink // Default to link
	}
	return method
}

// StartEmailVerification initiates email verification process
func (s *VerificationService) StartEmailVerification(ctx context.Context, sessionID uuid.UUID, email string) (*models.VerificationRecord, error) {
	method := s.GetVerificationMethod()

	if method == VerificationMethodLink {
		return s.startEmailVerificationWithLink(ctx, sessionID, email)
	}

	return s.startEmailVerificationWithOTP(ctx, sessionID, email)
}

// StartEmailVerificationWithBusinessName initiates email verification with business name for personalized emails
func (s *VerificationService) StartEmailVerificationWithBusinessName(ctx context.Context, sessionID uuid.UUID, email, businessName string) (*models.VerificationRecord, error) {
	method := s.GetVerificationMethod()

	if method == VerificationMethodLink {
		return s.startEmailVerificationWithLinkAndBusinessName(ctx, sessionID, email, businessName)
	}

	return s.startEmailVerificationWithOTP(ctx, sessionID, email)
}

// startEmailVerificationWithOTP sends OTP code via verification service
func (s *VerificationService) startEmailVerificationWithOTP(ctx context.Context, sessionID uuid.UUID, email string) (*models.VerificationRecord, error) {
	// Call verification service to send code
	resp, err := s.verificationClient.SendCode(ctx, &clients.SendVerificationCodeRequest{
		Recipient: email,
		Channel:   "email",
		Purpose:   "email_verification",
		SessionID: &sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send verification code: %w", err)
	}

	// Map response to VerificationRecord for backwards compatibility
	record := &models.VerificationRecord{
		ID:                  resp.ID,
		OnboardingSessionID: sessionID,
		VerificationType:    "email",
		TargetValue:         email,
		Status:              "pending",
		ExpiresAt:           resp.ExpiresAt,
		MaxAttempts:         3, // Default from verification service
		Attempts:            0,
	}

	return record, nil
}

// startEmailVerificationWithLink generates token and sends verification link
func (s *VerificationService) startEmailVerificationWithLink(ctx context.Context, sessionID uuid.UUID, email string) (*models.VerificationRecord, error) {
	return s.startEmailVerificationWithLinkAndBusinessName(ctx, sessionID, email, "")
}

// startEmailVerificationWithLinkAndBusinessName generates token and sends verification link with business name
func (s *VerificationService) startEmailVerificationWithLinkAndBusinessName(ctx context.Context, sessionID uuid.UUID, email, businessName string) (*models.VerificationRecord, error) {
	// Generate secure token
	token, err := s.generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Calculate expiry
	ttl := time.Duration(s.verificationConfig.TokenExpiryHours) * time.Hour
	if ttl == 0 {
		ttl = 24 * time.Hour // Default 24 hours
	}
	expiresAt := time.Now().Add(ttl)

	// Save token to Redis
	tokenData := &redis.VerificationTokenData{
		Token:     token,
		SessionID: sessionID.String(),
		Email:     email,
		Purpose:   "email_verification",
	}

	if err := s.redisClient.SaveVerificationToken(ctx, token, tokenData, ttl); err != nil {
		return nil, fmt.Errorf("failed to save verification token: %w", err)
	}

	// Build verification link
	verificationLink := s.buildVerificationLink(token)

	// Get business name from session if not provided
	if businessName == "" && s.onboardingRepo != nil {
		session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"business_information"})
		if err == nil && session != nil && session.BusinessInformation != nil {
			businessName = session.BusinessInformation.BusinessName
		}
	}
	if businessName == "" {
		businessName = "Tesseract Hub"
	}

	// Send verification email directly via notification-service API
	// Simple, synchronous, reliable - no NATS complexity
	log.Printf("[VerificationService] Sending verification email to %s for session %s", security.MaskEmail(email), sessionID)
	if err := s.notificationClient.SendVerificationLinkEmail(ctx, email, verificationLink, businessName); err != nil {
		_ = s.redisClient.DeleteVerificationToken(ctx, token)
		return nil, fmt.Errorf("failed to send verification email: %w", err)
	}
	log.Printf("[VerificationService] Verification email sent successfully for session %s", sessionID)

	// Create verification record
	record := &models.VerificationRecord{
		ID:                  uuid.New(),
		OnboardingSessionID: sessionID,
		VerificationType:    "email",
		TargetValue:         email,
		Status:              "pending",
		ExpiresAt:           expiresAt,
		MaxAttempts:         1, // Link can only be used once
		Attempts:            0,
	}

	return record, nil
}

// generateSecureToken generates a cryptographically secure token
func (s *VerificationService) generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// buildVerificationLink creates the verification URL
func (s *VerificationService) buildVerificationLink(token string) string {
	baseURL := s.verificationConfig.OnboardingAppURL
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/onboarding/verify-email?token=%s", baseURL, token)
}

// VerifyByToken verifies email using the token from verification link
func (s *VerificationService) VerifyByToken(ctx context.Context, token string) (*models.VerificationRecord, error) {
	// Get token data from Redis
	tokenData, err := s.redisClient.GetVerificationToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve verification token: %w", err)
	}

	if tokenData == nil {
		return nil, fmt.Errorf("verification token not found or expired")
	}

	// Parse session ID
	sessionID, err := uuid.Parse(tokenData.SessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID in token: %w", err)
	}

	// Mark email as verified in Redis
	if err := s.redisClient.SaveEmailVerificationStatus(ctx, tokenData.Email, tokenData.SessionID, true, 7*24*time.Hour); err != nil {
		return nil, fmt.Errorf("failed to save verification status: %w", err)
	}

	// Delete the used token
	if err := s.redisClient.DeleteVerificationToken(ctx, token); err != nil {
		// Log but don't fail - verification is already complete
		fmt.Printf("Warning: failed to delete used verification token: %v\n", err)
	}

	// Create verification record
	now := time.Now()
	record := &models.VerificationRecord{
		OnboardingSessionID: sessionID,
		VerificationType:    "email",
		TargetValue:         tokenData.Email,
		Status:              "verified",
		VerifiedAt:          &now,
	}

	return record, nil
}

// GetTokenInfo retrieves information about a verification token (for frontend display)
func (s *VerificationService) GetTokenInfo(ctx context.Context, token string) (*redis.VerificationTokenData, error) {
	tokenData, err := s.redisClient.GetVerificationToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve verification token: %w", err)
	}

	if tokenData == nil {
		return nil, fmt.Errorf("verification token not found or expired")
	}

	return tokenData, nil
}

// StartPhoneVerification initiates phone verification process
func (s *VerificationService) StartPhoneVerification(ctx context.Context, sessionID uuid.UUID, phone string) (*models.VerificationRecord, error) {
	// Call verification service to send code
	resp, err := s.verificationClient.SendCode(ctx, &clients.SendVerificationCodeRequest{
		Recipient: phone,
		Channel:   "sms",
		Purpose:   "phone_verification",
		SessionID: &sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send verification code: %w", err)
	}

	// Map response to VerificationRecord
	record := &models.VerificationRecord{
		ID:                  resp.ID,
		OnboardingSessionID: sessionID,
		VerificationType:    "phone",
		TargetValue:         phone,
		Status:              "pending",
		ExpiresAt:           resp.ExpiresAt,
		MaxAttempts:         3,
		Attempts:            0,
	}

	return record, nil
}

// VerifyCode verifies a verification code
func (s *VerificationService) VerifyCode(ctx context.Context, sessionID uuid.UUID, code string) (*models.VerificationRecord, error) {
	// We need to get the verification status first to know the recipient
	// This is a limitation of the current API design
	// For now, we'll need to pass recipient in a different way
	// Let's use email verification as the primary use case

	// TODO: This needs to be refactored to pass recipient properly
	// For now, this is a placeholder that will fail
	return nil, fmt.Errorf("verification with sessionID lookup not yet implemented - use VerifyCodeWithRecipient instead")
}

// VerifyCodeWithRecipient verifies a verification code with recipient information
func (s *VerificationService) VerifyCodeWithRecipient(ctx context.Context, sessionID uuid.UUID, recipient, code, purpose string) (*models.VerificationRecord, error) {
	// Call verification service to verify code
	resp, err := s.verificationClient.VerifyCode(ctx, &clients.VerifyCodeRequest{
		Recipient: recipient,
		Code:      code,
		Purpose:   purpose,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify code: %w", err)
	}

	if !resp.Verified {
		return nil, fmt.Errorf("%s", resp.Message)
	}

	// Map response to VerificationRecord
	verificationType := "email"
	if purpose == "phone_verification" {
		verificationType = "phone"
	}

	record := &models.VerificationRecord{
		OnboardingSessionID: sessionID,
		VerificationType:    verificationType,
		TargetValue:         recipient,
		Status:              "verified",
		VerifiedAt:          resp.VerifiedAt,
	}

	return record, nil
}

// ResendVerificationCode resends a verification code
func (s *VerificationService) ResendVerificationCode(ctx context.Context, sessionID uuid.UUID, verificationType, targetValue string) (*models.VerificationRecord, error) {
	// For email verification, check the verification method
	if verificationType == "email" {
		method := s.GetVerificationMethod()
		if method == VerificationMethodLink {
			// Use link-based verification - same as StartEmailVerification
			return s.startEmailVerificationWithLink(ctx, sessionID, targetValue)
		}
	}

	// Fall through to OTP-based verification for phone or when method is OTP
	channel := "email"
	purpose := "email_verification"
	if verificationType == "phone" {
		channel = "sms"
		purpose = "phone_verification"
	}

	// Call verification service to resend code
	resp, err := s.verificationClient.ResendCode(ctx, &clients.ResendCodeRequest{
		Recipient: targetValue,
		Channel:   channel,
		Purpose:   purpose,
		SessionID: &sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resend verification code: %w", err)
	}

	// Map response to VerificationRecord
	record := &models.VerificationRecord{
		ID:                  resp.ID,
		OnboardingSessionID: sessionID,
		VerificationType:    verificationType,
		TargetValue:         targetValue,
		Status:              "pending",
		ExpiresAt:           resp.ExpiresAt,
		MaxAttempts:         3,
		Attempts:            0,
	}

	return record, nil
}

// GetVerificationStatus gets the verification status for a session
func (s *VerificationService) GetVerificationStatus(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error) {
	// This method would need to be refactored to query verification-service
	// For now, return a placeholder response
	// TODO: Implement proper status tracking
	return map[string]interface{}{
		"email_verified": false,
		"phone_verified": false,
		"verifications":  []map[string]interface{}{},
		"note":           "Status tracking via verification-service not yet fully implemented",
	}, nil
}

// IsVerified checks if a specific verification type is verified
func (s *VerificationService) IsVerified(ctx context.Context, sessionID uuid.UUID, verificationType string) (bool, error) {
	// This would need recipient information to check with verification-service
	// For now, return false
	// TODO: Implement proper verification check
	return false, nil
}

// GetVerificationStatusByRecipient gets verification status by recipient
func (s *VerificationService) GetVerificationStatusByRecipient(ctx context.Context, recipient, purpose string) (map[string]interface{}, error) {
	// Call verification service to get status
	resp, err := s.verificationClient.GetStatus(ctx, recipient, purpose)
	if err != nil {
		return nil, fmt.Errorf("failed to get verification status: %w", err)
	}

	return map[string]interface{}{
		"recipient":     resp.Recipient,
		"purpose":       resp.Purpose,
		"is_verified":   resp.IsVerified,
		"verified_at":   resp.VerifiedAt,
		"pending_code":  resp.PendingCode,
		"expires_at":    resp.ExpiresAt,
		"can_resend":    resp.CanResend,
		"attempts_left": resp.AttemptsLeft,
	}, nil
}

// CleanupExpiredVerifications is now handled by verification-service
func (s *VerificationService) CleanupExpiredVerifications(ctx context.Context) error {
	// No-op - cleanup is handled by verification-service
	return nil
}

// Private helper methods

// maskSensitiveValue masks sensitive values for display
func (s *VerificationService) maskSensitiveValue(value, valueType string) string {
	switch valueType {
	case "email":
		parts := strings.Split(value, "@")
		if len(parts) != 2 {
			return value
		}
		local := parts[0]
		domain := parts[1]

		if len(local) <= 2 {
			return value
		}

		masked := local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:]
		return masked + "@" + domain

	case "phone":
		if len(value) <= 4 {
			return value
		}
		return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]

	default:
		return value
	}
}
