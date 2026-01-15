package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"tenant-service/internal/clients"
	"tenant-service/internal/models"
	"tenant-service/internal/repository"
	"gorm.io/gorm"
)

// PasswordResetService handles password reset operations for storefront customers
type PasswordResetService struct {
	db                 *gorm.DB
	membershipRepo     *repository.MembershipRepository
	keycloakClient     *auth.KeycloakAdminClient
	notificationClient *clients.NotificationClient
	baseDomain         string // e.g., "tesserix.app" - used to construct tenant-specific URLs
}

// NewPasswordResetService creates a new password reset service
func NewPasswordResetService(db *gorm.DB, keycloakClient *auth.KeycloakAdminClient, notificationClient *clients.NotificationClient) *PasswordResetService {
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}
	return &PasswordResetService{
		db:                 db,
		membershipRepo:     repository.NewMembershipRepository(db),
		keycloakClient:     keycloakClient,
		notificationClient: notificationClient,
		baseDomain:         baseDomain,
	}
}

// getStorefrontURL constructs the tenant-specific storefront URL
// URL pattern: https://{slug}-store.{baseDomain}
func (s *PasswordResetService) getStorefrontURL(tenantSlug string) string {
	return fmt.Sprintf("https://%s-store.%s", tenantSlug, s.baseDomain)
}

// RequestPasswordResetInput represents input for requesting a password reset
type RequestPasswordResetInput struct {
	Email      string `json:"email" validate:"required,email"`
	TenantSlug string `json:"tenant_slug" validate:"required"`
	IPAddress  string `json:"ip_address,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
}

// RequestPasswordResetOutput represents the response from requesting a password reset
type RequestPasswordResetOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// RequestPasswordReset initiates a password reset flow
// Returns success even if email doesn't exist (security best practice - don't reveal if account exists)
func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, input *RequestPasswordResetInput) (*RequestPasswordResetOutput, error) {
	// Resolve tenant
	tenant, err := s.membershipRepo.GetTenantBySlug(ctx, input.TenantSlug)
	if err != nil {
		log.Printf("[PasswordResetService] Tenant not found: %s", input.TenantSlug)
		// Return success anyway to not reveal tenant existence
		return &RequestPasswordResetOutput{
			Success: true,
			Message: "If an account exists with this email, you will receive a password reset link shortly.",
		}, nil
	}

	// Find user by email
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", input.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[PasswordResetService] User not found: %s", input.Email)
			// Return success anyway to not reveal if account exists
			return &RequestPasswordResetOutput{
				Success: true,
				Message: "If an account exists with this email, you will receive a password reset link shortly.",
			}, nil
		}
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	// Check if user has membership in this tenant
	membership, err := s.membershipRepo.GetMembership(ctx, user.ID, tenant.ID)
	if err != nil || membership == nil {
		log.Printf("[PasswordResetService] User %s has no membership in tenant %s", input.Email, input.TenantSlug)
		// Return success anyway
		return &RequestPasswordResetOutput{
			Success: true,
			Message: "If an account exists with this email, you will receive a password reset link shortly.",
		}, nil
	}

	// Invalidate any existing tokens for this user/tenant
	s.invalidateExistingTokens(ctx, user.ID, tenant.ID)

	// Generate a secure random token
	rawToken, hashedToken, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create token record
	tokenRecord := &models.PasswordResetToken{
		Token:          hashedToken,
		UserID:         user.ID,
		TenantID:       tenant.ID,
		Email:          user.Email,
		ExpiresAt:      time.Now().Add(models.DefaultPasswordResetTokenExpiry),
		RequestedIP:    input.IPAddress,
		RequestedAgent: input.UserAgent,
	}

	if err := s.db.WithContext(ctx).Create(tokenRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to create token record: %w", err)
	}

	// Build reset link with raw (unhashed) token using tenant-specific URL
	storefrontURL := s.getStorefrontURL(tenant.Slug)
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", storefrontURL, rawToken)

	// Send password reset email
	if s.notificationClient != nil {
		storeName := tenant.Name
		if storeName == "" {
			storeName = tenant.Slug
		}
		firstName := user.FirstName
		if firstName == "" {
			firstName = "there"
		}

		emailData := &clients.PasswordResetEmailData{
			Email:     user.Email,
			FirstName: firstName,
			StoreName: storeName,
			ResetLink: resetLink,
			ExpiresIn: "1 hour",
		}

		if err := s.notificationClient.SendPasswordResetEmail(ctx, emailData); err != nil {
			log.Printf("[PasswordResetService] Failed to send password reset email: %v", err)
			// Don't fail the request - token was created, user might try again
		} else {
			log.Printf("[PasswordResetService] Password reset email sent to %s", user.Email)
		}
	}

	return &RequestPasswordResetOutput{
		Success: true,
		Message: "If an account exists with this email, you will receive a password reset link shortly.",
	}, nil
}

// ValidateResetTokenInput represents input for validating a reset token
type ValidateResetTokenInput struct {
	Token string `json:"token" validate:"required"`
}

// ValidateResetTokenOutput represents the response from validating a reset token
type ValidateResetTokenOutput struct {
	Valid     bool   `json:"valid"`
	Email     string `json:"email,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ValidateResetToken validates a password reset token
func (s *PasswordResetService) ValidateResetToken(ctx context.Context, input *ValidateResetTokenInput) (*ValidateResetTokenOutput, error) {
	// Hash the provided token to compare with stored hash
	hashedToken := hashToken(input.Token)

	// Find the token record
	var tokenRecord models.PasswordResetToken
	if err := s.db.WithContext(ctx).
		Where("token = ? AND is_used = false AND expires_at > ?", hashedToken, time.Now()).
		First(&tokenRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ValidateResetTokenOutput{
				Valid:   false,
				Message: "Invalid or expired reset link. Please request a new password reset.",
			}, nil
		}
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}

	// Mask email for display
	maskedEmail := maskEmail(tokenRecord.Email)

	return &ValidateResetTokenOutput{
		Valid:     true,
		Email:     maskedEmail,
		ExpiresAt: tokenRecord.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// ResetPasswordInput represents input for resetting a password
type ResetPasswordInput struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
	IPAddress   string `json:"ip_address,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
}

// ResetPasswordOutput represents the response from resetting a password
type ResetPasswordOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ResetPassword resets the password using a valid token
func (s *PasswordResetService) ResetPassword(ctx context.Context, input *ResetPasswordInput) (*ResetPasswordOutput, error) {
	// Hash the provided token to compare with stored hash
	hashedToken := hashToken(input.Token)

	// Find and validate the token record
	var tokenRecord models.PasswordResetToken
	if err := s.db.WithContext(ctx).
		Where("token = ? AND is_used = false AND expires_at > ?", hashedToken, time.Now()).
		First(&tokenRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ResetPasswordOutput{
				Success: false,
				Message: "Invalid or expired reset link. Please request a new password reset.",
			}, nil
		}
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}

	// Get the user
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", tokenRecord.UserID).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check user has Keycloak account
	if user.KeycloakID == nil {
		return &ResetPasswordOutput{
			Success: false,
			Message: "Unable to reset password. Please contact support.",
		}, nil
	}

	// Update password in Keycloak
	if s.keycloakClient == nil {
		return nil, fmt.Errorf("authentication service not properly configured")
	}

	if err := s.keycloakClient.SetUserPassword(ctx, user.KeycloakID.String(), input.NewPassword, false); err != nil {
		log.Printf("[PasswordResetService] Failed to set password in Keycloak: %v", err)
		return &ResetPasswordOutput{
			Success: false,
			Message: "Failed to reset password. Please try again.",
		}, nil
	}

	// Mark token as used
	now := time.Now()
	if err := s.db.WithContext(ctx).
		Model(&tokenRecord).
		Updates(map[string]interface{}{
			"is_used":    true,
			"used_at":    now,
			"used_ip":    input.IPAddress,
			"used_agent": input.UserAgent,
		}).Error; err != nil {
		log.Printf("[PasswordResetService] Warning: Failed to mark token as used: %v", err)
	}

	// Log password reset event
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tokenRecord.TenantID,
		UserID:      &user.ID,
		EventType:   models.AuthEventPasswordReset,
		EventStatus: models.AuthEventStatusSuccess,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		Details:     models.MustNewJSONB(map[string]interface{}{"method": "password_reset_token"}),
	}
	if auditErr := s.db.WithContext(ctx).Create(auditLog).Error; auditErr != nil {
		log.Printf("[PasswordResetService] Warning: Failed to log password reset event: %v", auditErr)
	}

	log.Printf("[PasswordResetService] Password reset successful for user %s", user.Email)

	return &ResetPasswordOutput{
		Success: true,
		Message: "Your password has been reset successfully. You can now sign in with your new password.",
	}, nil
}

// invalidateExistingTokens marks all existing unused tokens for a user/tenant as used
func (s *PasswordResetService) invalidateExistingTokens(ctx context.Context, userID, tenantID uuid.UUID) {
	now := time.Now()
	s.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND tenant_id = ? AND is_used = false", userID, tenantID).
		Updates(map[string]interface{}{
			"is_used": true,
			"used_at": now,
		})
}

// generateSecureToken generates a cryptographically secure random token
// Returns: rawToken (to send to user), hashedToken (to store in DB)
func generateSecureToken() (string, string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	rawToken := hex.EncodeToString(bytes)
	hashedToken := hashToken(rawToken)
	return rawToken, hashedToken, nil
}

// hashToken creates a SHA256 hash of a token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// maskEmail masks an email for display (e.g., "t***@e***")
func maskEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	parts := make([]byte, 0, len(email))
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex <= 0 {
		return "***"
	}

	// Show first char, mask middle, show @, show first char of domain, mask rest
	parts = append(parts, email[0])
	for i := 1; i < atIndex; i++ {
		parts = append(parts, '*')
	}
	parts = append(parts, '@')
	if atIndex+1 < len(email) {
		parts = append(parts, email[atIndex+1])
		for i := atIndex + 2; i < len(email); i++ {
			if email[i] == '.' {
				parts = append(parts, '.')
			} else {
				parts = append(parts, '*')
			}
		}
	}
	return string(parts)
}
