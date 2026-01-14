package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"verification-service/internal/config"
	"verification-service/internal/models"
	"verification-service/internal/providers"
	"verification-service/internal/repository"
	"verification-service/pkg/crypto"
	"verification-service/pkg/otp"
	"gorm.io/gorm"
)

// VerificationService handles verification business logic
type VerificationService struct {
	config           *config.Config
	verificationRepo *repository.VerificationRepository
	rateLimitRepo    *repository.RateLimitRepository
	emailProvider    providers.EmailProvider
	encryptor        *crypto.Encryptor
	otpGenerator     *otp.Generator
}

// NewVerificationService creates a new verification service
func NewVerificationService(
	cfg *config.Config,
	verificationRepo *repository.VerificationRepository,
	rateLimitRepo *repository.RateLimitRepository,
	emailProvider providers.EmailProvider,
) (*VerificationService, error) {
	encryptor, err := crypto.NewEncryptor(cfg.Security.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	otpGenerator := otp.NewGenerator(cfg.Security.OTPLength)

	return &VerificationService{
		config:           cfg,
		verificationRepo: verificationRepo,
		rateLimitRepo:    rateLimitRepo,
		emailProvider:    emailProvider,
		encryptor:        encryptor,
		otpGenerator:     otpGenerator,
	}, nil
}

// SendVerificationCode sends a verification code to the recipient
func (s *VerificationService) SendVerificationCode(ctx context.Context, req *models.SendVerificationRequest) (*models.SendVerificationResponse, error) {
	// Check rate limit for sending codes
	exceeded, _, err := s.rateLimitRepo.CheckLimit(
		ctx,
		req.Recipient,
		"send",
		s.config.RateLimit.MaxCodesPerHour,
		s.config.GetCooldownDuration(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}
	if exceeded {
		return nil, fmt.Errorf("rate limit exceeded: too many verification codes sent")
	}

	// Check if there's an active code
	activeCode, err := s.verificationRepo.GetActiveByRecipient(ctx, req.Recipient, req.Purpose)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check active code: %w", err)
	}

	// If active code exists and is still valid, don't send a new one
	if activeCode != nil && activeCode.IsValid() {
		expiresIn := int(time.Until(activeCode.ExpiresAt).Seconds())
		return &models.SendVerificationResponse{
			ID:        activeCode.ID,
			Recipient: activeCode.Recipient,
			Channel:   activeCode.Channel,
			Purpose:   activeCode.Purpose,
			ExpiresAt: activeCode.ExpiresAt,
			ExpiresIn: expiresIn,
		}, nil
	}

	// Generate new OTP
	code, err := s.otpGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Encrypt the code
	encryptedCode, err := s.encryptor.Encrypt(code)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt code: %w", err)
	}

	// Create code hash for lookup
	codeHash := crypto.Hash(code)

	// Create verification record
	expiresAt := time.Now().Add(s.config.GetOTPExpiry())
	verificationCode := &models.VerificationCode{
		ID:          uuid.New(),
		Recipient:   req.Recipient,
		Channel:     req.Channel,
		Code:        encryptedCode,
		CodeHash:    codeHash,
		Purpose:     req.Purpose,
		SessionID:   req.SessionID,
		TenantID:    req.TenantID,
		ExpiresAt:   expiresAt,
		MaxAttempts: s.config.RateLimit.MaxAttempts,
	}

	// Save to database
	if err := s.verificationRepo.Create(ctx, verificationCode); err != nil {
		return nil, fmt.Errorf("failed to save verification code: %w", err)
	}

	// Send email/SMS based on channel
	if err := s.sendCode(req.Channel, req.Recipient, code, req.Purpose); err != nil {
		return nil, fmt.Errorf("failed to send verification code: %w", err)
	}

	// Increment rate limit counter
	if err := s.rateLimitRepo.Increment(ctx, req.Recipient, "send"); err != nil {
		return nil, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	expiresIn := int(time.Until(expiresAt).Seconds())
	return &models.SendVerificationResponse{
		ID:        verificationCode.ID,
		Recipient: verificationCode.Recipient,
		Channel:   verificationCode.Channel,
		Purpose:   verificationCode.Purpose,
		ExpiresAt: expiresAt,
		ExpiresIn: expiresIn,
	}, nil
}

// VerifyCode verifies a verification code
func (s *VerificationService) VerifyCode(ctx context.Context, req *models.VerifyCodeRequest) (*models.VerifyCodeResponse, error) {
	// Normalize the code
	normalizedCode := otp.NormalizeCode(req.Code)

	// Create code hash for lookup
	codeHash := crypto.Hash(normalizedCode)

	// Get the verification code by hash
	verificationCode, err := s.verificationRepo.GetByCodeHash(ctx, codeHash)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.VerifyCodeResponse{
				Success:  false,
				Verified: false,
				Message:  "Invalid verification code",
			}, nil
		}
		return nil, fmt.Errorf("failed to retrieve verification code: %w", err)
	}

	// Check if recipient matches
	if verificationCode.Recipient != req.Recipient {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Invalid verification code",
		}, nil
	}

	// Check if purpose matches
	if verificationCode.Purpose != req.Purpose {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Invalid verification code",
		}, nil
	}

	// Log the attempt
	_ = s.verificationRepo.LogAttempt(ctx, &models.VerificationAttempt{
		VerificationCodeID: verificationCode.ID,
		Success:            false,
	})

	// Increment attempt count
	if err := s.verificationRepo.IncrementAttempts(ctx, verificationCode.ID); err != nil {
		return nil, fmt.Errorf("failed to increment attempts: %w", err)
	}

	// Check if code has expired
	if verificationCode.IsExpired() {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Verification code has expired",
		}, nil
	}

	// Check if code has been used
	if verificationCode.IsUsed {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Verification code has already been used",
		}, nil
	}

	// Check if max attempts reached
	if verificationCode.AttemptCount > verificationCode.MaxAttempts {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Maximum verification attempts exceeded",
		}, nil
	}

	// Decrypt and verify the code
	decryptedCode, err := s.encryptor.Decrypt(verificationCode.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt code: %w", err)
	}

	if decryptedCode != normalizedCode {
		return &models.VerifyCodeResponse{
			Success:  false,
			Verified: false,
			Message:  "Invalid verification code",
		}, nil
	}

	// Mark code as used
	if err := s.verificationRepo.MarkAsUsed(ctx, verificationCode.ID); err != nil {
		return nil, fmt.Errorf("failed to mark code as used: %w", err)
	}

	// Log successful attempt
	_ = s.verificationRepo.LogAttempt(ctx, &models.VerificationAttempt{
		VerificationCodeID: verificationCode.ID,
		Success:            true,
	})

	now := time.Now()
	return &models.VerifyCodeResponse{
		Success:    true,
		Verified:   true,
		VerifiedAt: &now,
		SessionID:  verificationCode.SessionID,
		TenantID:   verificationCode.TenantID,
		Message:    "Verification successful",
	}, nil
}

// ResendCode resends a verification code
func (s *VerificationService) ResendCode(ctx context.Context, req *models.ResendCodeRequest) (*models.SendVerificationResponse, error) {
	// Get the latest code
	latestCode, err := s.verificationRepo.GetLatestByRecipient(ctx, req.Recipient, req.Purpose)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No previous code, send a new one
			return s.SendVerificationCode(ctx, &models.SendVerificationRequest{
				Recipient: req.Recipient,
				Channel:   req.Channel,
				Purpose:   req.Purpose,
				SessionID: req.SessionID,
			})
		}
		return nil, fmt.Errorf("failed to get latest code: %w", err)
	}

	// Check if we can resend
	if !latestCode.CanResend() {
		return nil, fmt.Errorf("cannot resend code yet: code is still valid")
	}

	// Send a new code
	return s.SendVerificationCode(ctx, &models.SendVerificationRequest{
		Recipient: req.Recipient,
		Channel:   req.Channel,
		Purpose:   req.Purpose,
		SessionID: req.SessionID,
	})
}

// GetVerificationStatus checks the verification status for a recipient
func (s *VerificationService) GetVerificationStatus(ctx context.Context, req *models.CheckStatusRequest) (*models.VerificationStatusResponse, error) {
	// Check if there's a verified code
	verifiedCode, err := s.verificationRepo.GetVerifiedCode(ctx, req.Recipient, req.Purpose)
	if err == nil && verifiedCode != nil {
		return &models.VerificationStatusResponse{
			Recipient:    req.Recipient,
			Purpose:      req.Purpose,
			IsVerified:   true,
			VerifiedAt:   verifiedCode.VerifiedAt,
			PendingCode:  false,
			CanResend:    false,
			AttemptsLeft: 0,
		}, nil
	}

	// Check for active code
	activeCode, err := s.verificationRepo.GetActiveByRecipient(ctx, req.Recipient, req.Purpose)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to get active code: %w", err)
	}

	if activeCode != nil {
		attemptsLeft := activeCode.MaxAttempts - activeCode.AttemptCount
		if attemptsLeft < 0 {
			attemptsLeft = 0
		}

		return &models.VerificationStatusResponse{
			Recipient:    req.Recipient,
			Purpose:      req.Purpose,
			IsVerified:   false,
			PendingCode:  activeCode.IsValid(),
			ExpiresAt:    &activeCode.ExpiresAt,
			CanResend:    activeCode.CanResend(),
			AttemptsLeft: attemptsLeft,
		}, nil
	}

	// No code found
	return &models.VerificationStatusResponse{
		Recipient:    req.Recipient,
		Purpose:      req.Purpose,
		IsVerified:   false,
		PendingCode:  false,
		CanResend:    true,
		AttemptsLeft: 0,
	}, nil
}

// sendCode sends the code via the appropriate channel
func (s *VerificationService) sendCode(channel, recipient, code, purpose string) error {
	switch channel {
	case "email":
		return s.emailProvider.SendVerificationEmail(recipient, code, purpose)
	case "sms":
		// TODO: Implement SMS provider
		return fmt.Errorf("SMS channel not yet implemented")
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// SendCustomEmail sends a custom email (welcome, account created, verification link, etc.)
func (s *VerificationService) SendCustomEmail(ctx context.Context, req *models.SendEmailRequest) error {
	var subject, htmlBody string

	switch req.EmailType {
	case "welcome":
		firstName := req.FirstName
		if firstName == "" {
			firstName = "there" // fallback
		}
		subject, htmlBody = providers.FormatWelcomeEmail(firstName)

	case "account_created":
		firstName := req.FirstName
		if firstName == "" {
			firstName = "there" // fallback
		}
		businessName := req.BusinessName
		if businessName == "" {
			businessName = "Your Business"
		}
		subdomain := req.Subdomain
		if subdomain == "" {
			subdomain = "your-store"
		}
		subject, htmlBody = providers.FormatAccountCreatedEmail(firstName, businessName, subdomain)

	case "email_verification_link":
		verificationLink := req.VerificationLink
		if verificationLink == "" {
			return fmt.Errorf("verification_link is required for email_verification_link type")
		}
		businessName := req.BusinessName
		if businessName == "" {
			businessName = "Tesseract Hub"
		}
		subject, htmlBody = providers.FormatVerificationLinkEmail(verificationLink, businessName, req.Recipient)

	case "welcome_pack":
		firstName := req.FirstName
		if firstName == "" {
			firstName = "there"
		}
		businessName := req.BusinessName
		if businessName == "" {
			businessName = "Your Business"
		}
		adminURL := req.AdminURL
		if adminURL == "" {
			adminURL = "https://admin.tesserix.app"
		}
		storefrontURL := req.StorefrontURL
		if storefrontURL == "" {
			storefrontURL = "https://store.tesserix.app"
		}
		subject, htmlBody = providers.FormatWelcomePackEmail(firstName, businessName, req.TenantSlug, adminURL, storefrontURL, req.Recipient)

	default:
		return fmt.Errorf("unsupported email type: %s", req.EmailType)
	}

	// Send the email
	return s.emailProvider.SendEmail(req.Recipient, subject, htmlBody)
}
