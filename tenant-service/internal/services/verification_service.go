package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/config"
	"tenant-service/internal/models"
	tsnats "tenant-service/internal/nats"
	"tenant-service/internal/redis"
	"tenant-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/security"
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
	// SECURITY: Invalidate any previous tokens for this session
	// This prevents old tokens (potentially with different emails) from being used
	if err := s.redisClient.InvalidateSessionTokens(ctx, sessionID.String()); err != nil {
		log.Printf("[VerificationService] Warning: failed to invalidate old tokens for session %s: %v", sessionID, err)
		// Continue anyway - this is a security enhancement, not a blocker
	} else {
		log.Printf("[VerificationService] Invalidated previous verification tokens for session %s", sessionID)
	}

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

	// Get business name and custom domain from session
	var dnsConfig *clients.CustomDomainDNSConfig
	if s.onboardingRepo != nil {
		session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"business_information", "application_configurations"})
		if err == nil && session != nil {
			if session.BusinessInformation != nil && businessName == "" {
				businessName = session.BusinessInformation.BusinessName
			}

			// Check if there's a custom domain in the store_setup configuration
			dnsConfig = s.buildDNSConfigFromSession(session)
		}
	}
	if businessName == "" {
		businessName = "Tesseract Hub"
	}

	// Send verification email directly via notification-service API
	// Simple, synchronous, reliable - no NATS complexity
	log.Printf("[VerificationService] Sending verification email to %s for session %s", security.MaskEmail(email), sessionID)

	// Use DNS-aware email sending if we have custom domain config
	if err := s.notificationClient.SendVerificationLinkEmailWithDNS(ctx, email, verificationLink, businessName, dnsConfig); err != nil {
		_ = s.redisClient.DeleteVerificationToken(ctx, token)
		return nil, fmt.Errorf("failed to send verification email: %w", err)
	}

	if dnsConfig != nil && dnsConfig.IsCustomDomain {
		log.Printf("[VerificationService] Verification email sent with DNS instructions for custom domain %s", dnsConfig.CustomDomain)
	} else {
		log.Printf("[VerificationService] Verification email sent successfully for session %s", sessionID)
	}

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

// storeSetupConfig represents the structure of store_setup configuration data
type storeSetupConfig struct {
	UseCustomDomain bool   `json:"use_custom_domain"`
	CustomDomain    string `json:"custom_domain"`
}

// buildDNSConfigFromSession checks if the session has a custom domain and builds DNS config
// The CNAME targets point to the tenant's platform subdomains (e.g., awesome-store.tesserix.app)
func (s *VerificationService) buildDNSConfigFromSession(session *models.OnboardingSession) *clients.CustomDomainDNSConfig {
	if session == nil || len(session.ApplicationConfigurations) == 0 {
		return nil
	}

	// Find the store_setup configuration
	var storeConfig *models.ApplicationConfiguration
	for i := range session.ApplicationConfigurations {
		if session.ApplicationConfigurations[i].ApplicationType == "store_setup" {
			storeConfig = &session.ApplicationConfigurations[i]
			break
		}
	}

	if storeConfig == nil {
		log.Printf("[VerificationService] No store_setup configuration found in session")
		return nil
	}

	// Parse the configuration data to extract custom domain info
	var configData storeSetupConfig
	if err := json.Unmarshal(storeConfig.ConfigurationData, &configData); err != nil {
		log.Printf("[VerificationService] Failed to parse store_setup config: %v", err)
		return nil
	}

	// Check if custom domain is enabled and has a value
	if !configData.UseCustomDomain || configData.CustomDomain == "" {
		log.Printf("[VerificationService] Custom domain not enabled or not set (use_custom_domain=%v, custom_domain=%s)",
			configData.UseCustomDomain, configData.CustomDomain)
		return nil
	}

	// Get slug - prefer reserved slug, fall back to generating from business name
	var slug string
	var businessName string
	if session.BusinessInformation != nil {
		// First, check if there's a reserved slug
		if session.BusinessInformation.TenantSlug != "" {
			slug = session.BusinessInformation.TenantSlug
			log.Printf("[VerificationService] Using reserved slug: %s", slug)
		}
		businessName = session.BusinessInformation.BusinessName
	}

	// If no reserved slug, generate from business name
	if slug == "" {
		if businessName == "" {
			log.Printf("[VerificationService] Warning: No business name found, cannot generate slug for CNAME targets")
			return nil
		}
		slug = generateSlugFromBusinessName(businessName)
		if slug == "" {
			log.Printf("[VerificationService] Warning: Could not generate slug from business name: %s", businessName)
			return nil
		}
		log.Printf("[VerificationService] Generated slug from business name: %s -> %s", businessName, slug)
	}

	// Get base domain from config
	baseDomain := s.verificationConfig.BaseDomain
	if baseDomain == "" {
		baseDomain = "tesserix.app" // Default
	}

	// Build host URLs from the custom domain
	domain := configData.CustomDomain
	log.Printf("[VerificationService] Building DNS config for custom domain: %s (tenant slug: %s)", domain, slug)

	// Build routing configuration for custom domains
	// Custom domains must use A records (pointing to LoadBalancer IP) due to
	// Cloudflare cross-account CNAME restrictions (Error 1014)
	// If CustomDomainGatewayIP is configured, use A records; otherwise fallback to CNAME
	routingIP := s.verificationConfig.CustomDomainGatewayIP
	useARecords := routingIP != ""
	routingCNAMETarget := fmt.Sprintf("proxy.%s", baseDomain) // fallback if IP not configured

	// Build unique ACME CNAME target for this domain + session combination
	// Format: {domain-sanitized}-{session-short-id}.acme.tesserix.app
	// We use session ID (not tenant ID) because tenant ID may not exist yet during onboarding
	// This ensures each onboarding session gets a unique target, preventing cross-tenant certificate hijacking
	// e.g., domain="store.example.com", sessionID="a1b2c3d4-..." -> "store-example-com-a1b2c3d4.acme.tesserix.app"
	sanitizedDomain := strings.ReplaceAll(domain, ".", "-")
	acmeZone := fmt.Sprintf("acme.%s", baseDomain)

	// Get session short ID for uniqueness (first 8 chars of UUID)
	// Session ID is always available during onboarding, unlike tenant ID
	sessionShortID := session.ID.String()[:8]

	// Build ACME CNAME target with session-specific suffix
	acmeCNAMETarget := fmt.Sprintf("%s-%s.%s", sanitizedDomain, sessionShortID, acmeZone)
	log.Printf("[VerificationService] Generated session-specific ACME CNAME target: %s (session: %s)", acmeCNAMETarget, session.ID)

	return &clients.CustomDomainDNSConfig{
		IsCustomDomain: true,
		CustomDomain:   domain,

		// Customer's subdomain hosts (what they configure DNS for)
		StorefrontHost: domain,                          // customdomain.com
		AdminHost:      fmt.Sprintf("admin.%s", domain), // admin.customdomain.com
		APIHost:        fmt.Sprintf("api.%s", domain),   // api.customdomain.com

		// Routing configuration - A records to LoadBalancer IP (preferred)
		// Custom domains MUST use A records due to Cloudflare cross-account CNAME restrictions
		UseARecords:        useARecords,
		RoutingIP:          routingIP,
		RoutingCNAMETarget: routingCNAMETarget, // fallback if IP not configured

		TenantSlug: slug,
		BaseDomain: baseDomain,

		// CNAME Delegation for automatic SSL certificate management
		// Customer adds: _acme-challenge.theirdomain.com CNAME theirdomain-com.acme.tesserix.app
		// cert-manager follows the CNAME and creates TXT records in our Cloudflare zone
		UseCNAMEDelegation: true,                                      // Always show CNAME delegation option
		ACMEChallengeHost:  fmt.Sprintf("_acme-challenge.%s", domain), // The subdomain to add CNAME for
		ACMECNAMETarget:    acmeCNAMETarget,                           // Unique CNAME target per domain
	}
}

// generateSlugFromBusinessName creates a URL-safe slug from a business name
// This should match the logic used in tenant creation
func generateSlugFromBusinessName(businessName string) string {
	if businessName == "" {
		return ""
	}

	// Convert to lowercase
	slug := strings.ToLower(businessName)

	// Replace spaces and special characters with hyphens
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '_' || r == '-' {
			return '-'
		}
		return -1 // Remove other characters
	}, slug)

	// Remove consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Limit length (DNS label max is 63 chars, but we need room for suffixes like -admin)
	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.TrimRight(slug, "-")
	}

	return slug
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

	// SECURITY: Validate that token email matches the session's current contact email
	// This prevents using old tokens with different emails after contact info was changed
	if s.onboardingRepo != nil {
		session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"contact_information"})
		if err != nil {
			log.Printf("[VerificationService] Warning: Could not validate token email against session: %v", err)
			// Continue - session might be in a state where contact info isn't available
		} else if session != nil && len(session.ContactInformation) > 0 {
			currentEmail := session.ContactInformation[0].Email
			if currentEmail != "" && strings.ToLower(currentEmail) != strings.ToLower(tokenData.Email) {
				log.Printf("[VerificationService] SECURITY: Token email mismatch! Token has %s but session contact is %s",
					tokenData.Email, currentEmail)
				return nil, fmt.Errorf("verification token is invalid: email does not match current session contact")
			}
			log.Printf("[VerificationService] Token email validated against session contact: %s", tokenData.Email)
		}
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

	// Store verification status in Redis so IsEmailVerifiedByRecipient can find it later
	// This is critical for OTP-based verification where the verification-service may not
	// persist the status after verification
	if s.redisClient != nil && (purpose == "email_verification" || purpose == "email") {
		// Store for 24 hours - enough time for account setup to complete
		if err := s.redisClient.SaveEmailVerificationStatus(ctx, recipient, sessionID.String(), true, 24*time.Hour); err != nil {
			log.Printf("[VerificationService] Warning: Failed to save verification status to Redis: %v", err)
			// Don't fail - this is just a cache for faster lookups
		} else {
			log.Printf("[VerificationService] Saved email verification status to Redis for %s", recipient)
		}
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
// Fetches contact information from the session and checks verification status
func (s *VerificationService) GetVerificationStatus(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"email_verified": false,
		"phone_verified": false,
		"verifications":  []map[string]interface{}{},
	}

	// Get session to retrieve contact information
	if s.onboardingRepo == nil {
		return result, nil
	}

	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"contact_information"})
	if err != nil {
		return result, nil // Return defaults if session not found
	}

	if len(session.ContactInformation) == 0 {
		return result, nil
	}

	contact := session.ContactInformation[0]

	// Check email verification status
	if contact.Email != "" {
		isVerified, _ := s.IsEmailVerifiedByRecipient(ctx, contact.Email, "email_verification")
		result["email_verified"] = isVerified
	}

	// Check phone verification status
	if contact.Phone != "" {
		phoneVerified, _ := s.IsEmailVerifiedByRecipient(ctx, contact.Phone, "phone_verification")
		result["phone_verified"] = phoneVerified
	}

	return result, nil
}

// IsVerified checks if a specific verification type is verified for a session
func (s *VerificationService) IsVerified(ctx context.Context, sessionID uuid.UUID, verificationType string) (bool, error) {
	// Get session to retrieve contact information
	if s.onboardingRepo == nil {
		return false, fmt.Errorf("onboarding repository not configured")
	}

	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"contact_information"})
	if err != nil {
		return false, fmt.Errorf("failed to get session: %w", err)
	}

	if len(session.ContactInformation) == 0 {
		return false, fmt.Errorf("no contact information found")
	}

	contact := session.ContactInformation[0]

	// Determine recipient based on verification type
	var recipient string
	switch verificationType {
	case "email", "email_verification":
		recipient = contact.Email
	case "phone", "phone_verification":
		recipient = contact.Phone
	default:
		return false, fmt.Errorf("unknown verification type: %s", verificationType)
	}

	if recipient == "" {
		return false, fmt.Errorf("no %s found in contact information", verificationType)
	}

	return s.IsEmailVerifiedByRecipient(ctx, recipient, verificationType)
}

// IsEmailVerifiedByRecipient checks if a recipient's email/phone is verified
// First checks Redis (for link-based verification), then falls back to verification-service (for OTP)
func (s *VerificationService) IsEmailVerifiedByRecipient(ctx context.Context, recipient, purpose string) (bool, error) {
	// First check Redis for link-based verification status
	// This is where VerifyByToken stores the verification status
	if s.redisClient != nil && (purpose == "email_verification" || purpose == "email") {
		status, err := s.redisClient.GetEmailVerificationStatus(ctx, recipient)
		if err != nil {
			log.Printf("[VerificationService] Warning: Redis lookup failed for %s: %v", recipient, err)
			// Continue to verification-service fallback
		} else if status != nil && status.IsVerified {
			log.Printf("[VerificationService] Email %s verified via link (from Redis)", recipient)
			return true, nil
		}
	}

	// Fall back to verification-service for OTP-based verification
	if s.verificationClient == nil {
		return false, fmt.Errorf("verification client not configured")
	}

	resp, err := s.verificationClient.GetStatus(ctx, recipient, purpose)
	if err != nil {
		// If verification-service also fails, but we checked Redis above, return false gracefully
		log.Printf("[VerificationService] Warning: verification-service lookup failed for %s: %v", recipient, err)
		return false, nil
	}

	return resp.IsVerified, nil
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

// SaveVerificationStatusToRedis saves email verification status to Redis
// This is a public method that can be called by OnboardingService to mark email as verified
// after completing the email verification task
func (s *VerificationService) SaveVerificationStatusToRedis(ctx context.Context, email, sessionID string) error {
	if s.redisClient == nil {
		log.Printf("[VerificationService] Warning: Redis client not available, cannot save verification status")
		return nil // Don't fail if Redis is not available
	}

	// Store for 24 hours - enough time for account setup to complete
	if err := s.redisClient.SaveEmailVerificationStatus(ctx, email, sessionID, true, 24*time.Hour); err != nil {
		log.Printf("[VerificationService] Warning: Failed to save verification status to Redis for %s: %v", email, err)
		return err
	}

	log.Printf("[VerificationService] Saved email verification status to Redis for %s (session %s)", email, sessionID)
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
