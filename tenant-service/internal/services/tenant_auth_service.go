package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/Tesseract-Nexus/go-shared/security"
	"tenant-service/internal/clients"
	"tenant-service/internal/models"
	"tenant-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TenantAuthService handles tenant-aware authentication
// This enables multi-tenant credential isolation where the same email
// can have different passwords for different tenants
type TenantAuthService struct {
	credentialRepo     *repository.CredentialRepository
	membershipRepo     *repository.MembershipRepository
	keycloakClient     *auth.KeycloakAdminClient
	keycloakConfig     *KeycloakAuthConfig
	db                 *gorm.DB
	staffClient        StaffClientInterface          // For staff member credential validation
	notificationClient *clients.NotificationClient   // For sending emails
	verificationClient *clients.VerificationClient   // For email verification
}

// StaffClientInterface defines the interface for staff-service client
type StaffClientInterface interface {
	GetStaffByEmailForTenant(ctx context.Context, email string, tenantID uuid.UUID) (*clients.StaffMemberInfo, error)
	SyncKeycloakUserID(ctx context.Context, tenantID, staffID uuid.UUID, keycloakUserID string) error
}

// KeycloakAuthConfig holds Keycloak configuration for multi-tenant auth
type KeycloakAuthConfig struct {
	ClientID     string // Public client ID for token exchange
	ClientSecret string // Client secret (if confidential client)
}

// NewTenantAuthService creates a new tenant authentication service
func NewTenantAuthService(db *gorm.DB, keycloakClient *auth.KeycloakAdminClient, keycloakConfig *KeycloakAuthConfig) *TenantAuthService {
	return &TenantAuthService{
		credentialRepo: repository.NewCredentialRepository(db),
		membershipRepo: repository.NewMembershipRepository(db),
		keycloakClient: keycloakClient,
		keycloakConfig: keycloakConfig,
		db:             db,
	}
}

// SetStaffClient sets the staff service client for staff credential validation
func (s *TenantAuthService) SetStaffClient(client StaffClientInterface) {
	s.staffClient = client
}

// SetNotificationClient sets the notification client for sending emails
func (s *TenantAuthService) SetNotificationClient(client *clients.NotificationClient) {
	s.notificationClient = client
}

// SetVerificationClient sets the verification client for email verification
func (s *TenantAuthService) SetVerificationClient(client *clients.VerificationClient) {
	s.verificationClient = client
}

// GetUserByKeycloakOrLocalID resolves a user by either Keycloak ID or local ID
// This handles the case where JWT tokens contain Keycloak subject (sub) but
// existing users may have a different local ID in tenant_users table
// Priority: keycloak_id (from JWT) -> id (legacy/direct lookup)
func (s *TenantAuthService) GetUserByKeycloakOrLocalID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	var user models.User

	// First try to find by keycloak_id (this is what JWT tokens contain)
	if err := s.db.WithContext(ctx).Where("keycloak_id = ?", userID).First(&user).Error; err == nil {
		return &user, nil
	}

	// Fall back to local ID lookup (for backwards compatibility)
	if err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err == nil {
		return &user, nil
	}

	return nil, fmt.Errorf("user not found with id or keycloak_id: %s", userID)
}

// AuthContext defines the authentication context
// This prevents staff members from logging into the storefront
const (
	AuthContextCustomer = "customer" // For storefront/customer login - staff fallback NOT allowed
	AuthContextStaff    = "staff"    // For admin portal - staff fallback allowed
)

// ValidateCredentialsRequest represents a request to validate tenant-specific credentials
type ValidateCredentialsRequest struct {
	Email       string    `json:"email" validate:"required,email"`
	Password    string    `json:"password" validate:"required"`
	TenantID    uuid.UUID `json:"tenant_id"`    // Either tenant_id or tenant_slug required
	TenantSlug  string    `json:"tenant_slug"`  // Either tenant_id or tenant_slug required
	AuthContext string    `json:"auth_context"` // "customer" or "staff" - controls whether staff fallback is allowed
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
}

// ValidateCredentialsResponse represents the response from credential validation
type ValidateCredentialsResponse struct {
	Valid          bool       `json:"valid"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	KeycloakUserID string     `json:"keycloak_user_id,omitempty"` // Keycloak user ID for token issuance
	TenantID       uuid.UUID  `json:"tenant_id"`
	TenantSlug     string     `json:"tenant_slug"`
	Email          string     `json:"email"`
	FirstName      string     `json:"first_name,omitempty"`
	LastName       string     `json:"last_name,omitempty"`
	Role           string     `json:"role,omitempty"`            // User's role in the tenant
	MFARequired    bool       `json:"mfa_required"`              // Whether MFA is required
	MFAEnabled     bool       `json:"mfa_enabled"`               // Whether user has MFA enabled
	AccountLocked  bool       `json:"account_locked"`            // Whether account is locked
	LockedUntil    *time.Time `json:"locked_until,omitempty"`    // When lockout expires
	RemainingAttempts int     `json:"remaining_attempts,omitempty"` // Remaining login attempts
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`

	// Keycloak tokens (only populated if IssueTokens is true in request)
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// ValidateCredentials validates tenant-specific credentials for a user
// This is the main entry point for tenant-aware authentication
func (s *TenantAuthService) ValidateCredentials(ctx context.Context, req *ValidateCredentialsRequest) (*ValidateCredentialsResponse, error) {
	// Resolve tenant
	var tenant *models.Tenant
	var err error

	if req.TenantID != uuid.Nil {
		tenant, err = s.membershipRepo.GetTenantByID(ctx, req.TenantID)
	} else if req.TenantSlug != "" {
		tenant, err = s.membershipRepo.GetTenantBySlug(ctx, req.TenantSlug)
	} else {
		return nil, fmt.Errorf("either tenant_id or tenant_slug is required")
	}

	if err != nil {
		return &ValidateCredentialsResponse{
			Valid:        false,
			ErrorCode:    "TENANT_NOT_FOUND",
			ErrorMessage: "The specified organization was not found",
		}, nil
	}

	// Get user by email
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// User not in tenant_users - try staff member fallback ONLY if auth_context is "staff"
			// SECURITY: Customer/storefront logins should NEVER fall back to staff credentials
			// This prevents store owners from logging into their own storefront using admin credentials
			if req.AuthContext != AuthContextCustomer && s.staffClient != nil && s.keycloakClient != nil && s.keycloakConfig != nil {
				log.Printf("[TenantAuthService] User not in tenant_users, trying staff fallback for %s (auth_context=%s)", security.MaskEmail(req.Email), req.AuthContext)
				return s.validateStaffCredentials(ctx, tenant, req)
			}
			// For customer context, don't fall back to staff - this is the security fix
			if req.AuthContext == AuthContextCustomer {
				log.Printf("[TenantAuthService] User not found and staff fallback disabled for customer context: %s", security.MaskEmail(req.Email))
			}
			// Don't reveal whether user exists
			s.logFailedAuthEvent(ctx, tenant.ID, nil, req.Email, req.IPAddress, req.UserAgent, "USER_NOT_FOUND")
			return &ValidateCredentialsResponse{
				Valid:        false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "INVALID_CREDENTIALS",
				ErrorMessage: "Invalid email or password",
			}, nil
		}
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	// Check if user has a membership in this tenant
	membership, err := s.membershipRepo.GetMembership(ctx, user.ID, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if membership == nil || !membership.IsActive {
		s.logFailedAuthEvent(ctx, tenant.ID, &user.ID, req.Email, req.IPAddress, req.UserAgent, "NO_MEMBERSHIP")
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "NO_ACCESS",
			ErrorMessage: "You do not have access to this organization",
		}, nil
	}

	// Check account lockout
	isLocked, lockedUntil, remainingAttempts, err := s.credentialRepo.CheckAccountLockout(ctx, user.ID, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check lockout: %w", err)
	}
	if isLocked {
		s.logFailedAuthEvent(ctx, tenant.ID, &user.ID, req.Email, req.IPAddress, req.UserAgent, "ACCOUNT_LOCKED")
		return &ValidateCredentialsResponse{
			Valid:         false,
			UserID:        &user.ID,
			TenantID:      tenant.ID,
			TenantSlug:    tenant.Slug,
			AccountLocked: true,
			LockedUntil:   lockedUntil,
			ErrorCode:     "ACCOUNT_LOCKED",
			ErrorMessage:  fmt.Sprintf("Account is locked until %s", lockedUntil.Format(time.RFC3339)),
		}, nil
	}

	// Get tenant-specific credential for MFA and lockout tracking
	// Note: Password is NOT stored in tenant_credentials - Keycloak is the source of truth
	credential, err := s.credentialRepo.GetCredential(ctx, user.ID, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	// Always validate password against Keycloak (single source of truth)
	var passwordValid bool
	var keycloakTokens *auth.TokenResponse

	keycloakUserID := ""
	if user.KeycloakID != nil {
		keycloakUserID = user.KeycloakID.String()
	}

	if s.keycloakClient != nil && s.keycloakConfig != nil && keycloakUserID != "" {
		// Validate password via Keycloak direct grant
		log.Printf("[TenantAuthService] Validating password via Keycloak direct grant, username=%s", security.MaskEmail(req.Email))
		tokens, kcErr := s.keycloakClient.GetTokenWithPassword(
			ctx,
			s.keycloakConfig.ClientID,
			s.keycloakConfig.ClientSecret,
			req.Email,
			req.Password,
		)
		if kcErr != nil {
			log.Printf("[TenantAuthService] Keycloak password validation failed: %v", kcErr)
			passwordValid = false
		} else {
			log.Printf("[TenantAuthService] Keycloak password validation succeeded")
			passwordValid = true
			keycloakTokens = tokens // Save tokens for session
		}
	} else {
		// No Keycloak configuration - this is a configuration error
		log.Printf("[TenantAuthService] ERROR: No Keycloak config available for password validation")
		return nil, fmt.Errorf("authentication service not properly configured")
	}

	if !passwordValid {
		// Record failed attempt
		if err := s.credentialRepo.RecordLoginAttempt(ctx, user.ID, tenant.ID, false, req.IPAddress, req.UserAgent); err != nil {
			log.Printf("[TenantAuthService] Warning: Failed to record login attempt: %v", err)
		}

		// Recheck remaining attempts after recording failure
		_, _, remainingAttempts, _ = s.credentialRepo.CheckAccountLockout(ctx, user.ID, tenant.ID)

		s.logFailedAuthEvent(ctx, tenant.ID, &user.ID, req.Email, req.IPAddress, req.UserAgent, "INVALID_PASSWORD")
		return &ValidateCredentialsResponse{
			Valid:             false,
			UserID:            &user.ID,
			TenantID:          tenant.ID,
			TenantSlug:        tenant.Slug,
			RemainingAttempts: remainingAttempts,
			ErrorCode:         "INVALID_CREDENTIALS",
			ErrorMessage:      "Invalid email or password",
		}, nil
	}

	// Record successful login
	if err := s.credentialRepo.RecordLoginAttempt(ctx, user.ID, tenant.ID, true, req.IPAddress, req.UserAgent); err != nil {
		log.Printf("[TenantAuthService] Warning: Failed to record successful login: %v", err)
	}

	// Get auth policy for MFA requirements
	mfaRequired := false
	policy, _ := s.credentialRepo.GetAuthPolicy(ctx, tenant.ID)
	if policy != nil {
		mfaRequired = policy.MFARequired
		// TODO: Check MFARequiredForRoles based on user's role
	}

	// Check if user has MFA enabled
	mfaEnabled := credential != nil && credential.MFAEnabled

	// Log successful auth event
	s.logSuccessAuthEvent(ctx, tenant.ID, &user.ID, req.IPAddress, req.UserAgent)

	response := &ValidateCredentialsResponse{
		Valid:          true,
		UserID:         &user.ID,
		KeycloakUserID: keycloakUserID,
		TenantID:       tenant.ID,
		TenantSlug:     tenant.Slug,
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Role:           membership.Role,
		MFARequired:    mfaRequired,
		MFAEnabled:     mfaEnabled,
	}

	// Attach tokens from Keycloak validation (already obtained during password validation)
	// Skip if MFA is required - tokens will be issued after MFA verification
	if keycloakTokens != nil && !mfaRequired {
		log.Printf("[TenantAuthService] Attaching tokens from Keycloak validation")
		response.AccessToken = keycloakTokens.AccessToken
		response.RefreshToken = keycloakTokens.RefreshToken
		response.IDToken = keycloakTokens.IDToken
		response.ExpiresIn = keycloakTokens.ExpiresIn
	}

	return response, nil
}

// validateStaffCredentials validates credentials for a staff member via Keycloak
// This is called as a fallback when user is not found in tenant_users
func (s *TenantAuthService) validateStaffCredentials(ctx context.Context, tenant *models.Tenant, req *ValidateCredentialsRequest) (*ValidateCredentialsResponse, error) {
	// Get staff info from staff-service
	staffInfo, err := s.staffClient.GetStaffByEmailForTenant(ctx, req.Email, tenant.ID)
	if err != nil {
		log.Printf("[TenantAuthService] Error getting staff info: %v", err)
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "INVALID_CREDENTIALS",
			ErrorMessage: "Invalid email or password",
		}, nil
	}

	if staffInfo == nil {
		// Staff not found
		s.logFailedAuthEvent(ctx, tenant.ID, nil, req.Email, req.IPAddress, req.UserAgent, "STAFF_NOT_FOUND")
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "INVALID_CREDENTIALS",
			ErrorMessage: "Invalid email or password",
		}, nil
	}

	// Check staff is active
	if !staffInfo.IsActive || staffInfo.AccountStatus != "active" {
		log.Printf("[TenantAuthService] Staff account not active: status=%s, isActive=%v", staffInfo.AccountStatus, staffInfo.IsActive)
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "ACCOUNT_INACTIVE",
			ErrorMessage: "Account is not active",
		}, nil
	}

	// Check staff has Keycloak user ID
	if staffInfo.KeycloakUserID == "" {
		log.Printf("[TenantAuthService] Staff has no Keycloak user ID: %s", security.MaskEmail(req.Email))
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "INVALID_CREDENTIALS",
			ErrorMessage: "Invalid email or password",
		}, nil
	}

	// Validate password via Keycloak direct grant
	log.Printf("[TenantAuthService] Validating staff password via Keycloak, email=%s", security.MaskEmail(req.Email))
	tokens, kcErr := s.keycloakClient.GetTokenWithPassword(
		ctx,
		s.keycloakConfig.ClientID,
		s.keycloakConfig.ClientSecret,
		req.Email,
		req.Password,
	)

	if kcErr != nil {
		log.Printf("[TenantAuthService] Keycloak staff password validation failed: %v", kcErr)
		s.logFailedAuthEvent(ctx, tenant.ID, &staffInfo.ID, req.Email, req.IPAddress, req.UserAgent, "INVALID_PASSWORD")
		return &ValidateCredentialsResponse{
			Valid:        false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "INVALID_CREDENTIALS",
			ErrorMessage: "Invalid email or password",
		}, nil
	}

	log.Printf("[TenantAuthService] Staff password validation succeeded for %s", security.MaskEmail(req.Email))

	// Extract actual Keycloak user ID from access token and sync if different
	if tokens != nil && tokens.AccessToken != "" {
		// Parse JWT without verification to get the sub claim
		// (token is already validated by Keycloak)
		parts := strings.Split(tokens.AccessToken, ".")
		if len(parts) >= 2 {
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err == nil {
				var claims map[string]interface{}
				if json.Unmarshal(payload, &claims) == nil {
					if sub, ok := claims["sub"].(string); ok && sub != "" {
						// If the Keycloak ID differs from what's stored, sync it
						if sub != staffInfo.KeycloakUserID {
							log.Printf("[TenantAuthService] Syncing keycloak_user_id: stored=%s, actual=%s",
								staffInfo.KeycloakUserID, sub)
							if syncErr := s.staffClient.SyncKeycloakUserID(ctx, tenant.ID, staffInfo.ID, sub); syncErr != nil {
								log.Printf("[TenantAuthService] Failed to sync keycloak_user_id: %v", syncErr)
							} else {
								// Update the response with the correct ID
								staffInfo.KeycloakUserID = sub
							}
						}
					}
				}
			}
		}
	}

	// Build successful response
	response := &ValidateCredentialsResponse{
		Valid:          true,
		UserID:         &staffInfo.ID,
		KeycloakUserID: staffInfo.KeycloakUserID,
		TenantID:       tenant.ID,
		TenantSlug:     tenant.Slug,
		Email:          staffInfo.Email,
		FirstName:      staffInfo.FirstName,
		LastName:       staffInfo.LastName,
		Role:           "staff", // Staff members have staff role by default
		MFARequired:    false,   // TODO: Check MFA for staff
		MFAEnabled:     false,
	}

	// Attach tokens
	if tokens != nil {
		response.AccessToken = tokens.AccessToken
		response.RefreshToken = tokens.RefreshToken
		response.IDToken = tokens.IDToken
		response.ExpiresIn = tokens.ExpiresIn
	}

	return response, nil
}

// validateGlobalPassword validates password against the global password in tenant_users
// This is for backward compatibility during migration to tenant_credentials
func (s *TenantAuthService) validateGlobalPassword(hashedPassword, plainPassword string) bool {
	// Using bcrypt for password comparison (imported at package level)
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	return err == nil
}

// ChangePassword changes a user's password in Keycloak
func (s *TenantAuthService) ChangePassword(ctx context.Context, userID, tenantID uuid.UUID, currentPassword, newPassword string, changedBy *uuid.UUID) error {
	// Get user by Keycloak ID or local ID (handles JWT subject vs local ID mismatch)
	user, err := s.GetUserByKeycloakOrLocalID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.KeycloakID == nil {
		return fmt.Errorf("user does not have a Keycloak account")
	}

	// Validate current password via Keycloak direct grant
	if s.keycloakClient == nil || s.keycloakConfig == nil {
		return fmt.Errorf("authentication service not properly configured")
	}

	_, err = s.keycloakClient.GetTokenWithPassword(
		ctx,
		s.keycloakConfig.ClientID,
		s.keycloakConfig.ClientSecret,
		user.Email,
		currentPassword,
	)
	if err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Validate new password against tenant's policy
	policy, _ := s.credentialRepo.GetAuthPolicy(ctx, tenantID)
	if policy != nil {
		if err := s.validatePasswordPolicy(newPassword, policy); err != nil {
			return err
		}
	}

	// Update password in Keycloak
	if err := s.keycloakClient.SetUserPassword(ctx, user.KeycloakID.String(), newPassword, false); err != nil {
		return fmt.Errorf("failed to update password in Keycloak: %w", err)
	}

	// Log password change event (use user.ID for consistent audit trail)
	localUserID := user.ID
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tenantID,
		UserID:      &localUserID,
		EventType:   models.AuthEventPasswordChanged,
		EventStatus: models.AuthEventStatusSuccess,
		Details:     models.MustNewJSONB(map[string]interface{}{"changed_by": changedBy}),
	}
	if auditErr := s.credentialRepo.LogAuthEvent(ctx, auditLog); auditErr != nil {
		log.Printf("[TenantAuthService] Warning: Failed to log password change event: %v", auditErr)
	}

	return nil
}

// SetPassword sets a password for a user in Keycloak
// This is used during onboarding or password reset
// Note: Password is stored ONLY in Keycloak, not in tenant_credentials
func (s *TenantAuthService) SetPassword(ctx context.Context, userID, tenantID uuid.UUID, password string, setBy *uuid.UUID) error {
	// Get user by Keycloak ID or local ID (handles JWT subject vs local ID mismatch)
	user, err := s.GetUserByKeycloakOrLocalID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.KeycloakID == nil {
		return fmt.Errorf("user does not have a Keycloak account")
	}

	// Validate password against tenant's policy
	policy, _ := s.credentialRepo.GetAuthPolicy(ctx, tenantID)
	if policy != nil {
		if err := s.validatePasswordPolicy(password, policy); err != nil {
			return err
		}
	}

	// Update password in Keycloak (single source of truth)
	if s.keycloakClient == nil {
		return fmt.Errorf("authentication service not properly configured")
	}

	if err := s.keycloakClient.SetUserPassword(ctx, user.KeycloakID.String(), password, false); err != nil {
		return fmt.Errorf("failed to set password in Keycloak: %w", err)
	}

	// Ensure a credential record exists for tracking (without password hash)
	// Use user.ID (local ID) for credential records, not the passed userID (which may be Keycloak ID)
	existing, err := s.credentialRepo.GetCredential(ctx, user.ID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to check existing credential: %w", err)
	}

	if existing == nil {
		// Create credential record without password (just for tracking/MFA)
		_, err = s.credentialRepo.CreateCredentialWithoutPassword(ctx, user.ID, tenantID, setBy)
		if err != nil {
			log.Printf("[TenantAuthService] Warning: Failed to create credential record: %v", err)
			// Don't fail - password was set in Keycloak successfully
		}
	}

	return nil
}

// validatePasswordPolicy validates a password against the tenant's policy
func (s *TenantAuthService) validatePasswordPolicy(password string, policy *models.TenantAuthPolicy) error {
	if len(password) < policy.PasswordMinLength {
		return fmt.Errorf("password must be at least %d characters", policy.PasswordMinLength)
	}
	if len(password) > policy.PasswordMaxLength {
		return fmt.Errorf("password must be at most %d characters", policy.PasswordMaxLength)
	}

	if policy.PasswordRequireUppercase {
		hasUpper := false
		for _, c := range password {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}

	if policy.PasswordRequireLowercase {
		hasLower := false
		for _, c := range password {
			if c >= 'a' && c <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}

	if policy.PasswordRequireNumbers {
		hasNumber := false
		for _, c := range password {
			if c >= '0' && c <= '9' {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return fmt.Errorf("password must contain at least one number")
		}
	}

	if policy.PasswordRequireSpecialChars {
		hasSpecial := false
		specialChars := policy.PasswordSpecialChars
		if specialChars == "" {
			specialChars = "!@#$%^&*()_+-=[]{}|;:,.<>?"
		}
		for _, c := range password {
			for _, s := range specialChars {
				if c == s {
					hasSpecial = true
					break
				}
			}
			if hasSpecial {
				break
			}
		}
		if !hasSpecial {
			return fmt.Errorf("password must contain at least one special character")
		}
	}

	return nil
}

// GetUserTenants returns all tenants a user has credentials for
func (s *TenantAuthService) GetUserTenants(ctx context.Context, email string) ([]TenantAuthInfo, error) {
	// Get user
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []TenantAuthInfo{}, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get all memberships for the user
	memberships, err := s.membershipRepo.GetUserMemberships(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memberships: %w", err)
	}

	tenants := make([]TenantAuthInfo, 0, len(memberships))
	for _, m := range memberships {
		if m.Tenant != nil {
			tenants = append(tenants, TenantAuthInfo{
				ID:          m.Tenant.ID,
				Slug:        m.Tenant.Slug,
				Name:        m.Tenant.Name,
				DisplayName: m.Tenant.DisplayName,
				LogoURL:     m.Tenant.LogoURL,
				Role:        m.Role,
				IsDefault:   m.IsDefault,
			})
		}
	}

	return tenants, nil
}

// GetTenantBasicInfo returns basic tenant info by ID (for enriching staff tenant lookups)
func (s *TenantAuthService) GetTenantBasicInfo(ctx context.Context, tenantID uuid.UUID) (*TenantAuthInfo, error) {
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return nil, err
	}

	return &TenantAuthInfo{
		ID:          tenant.ID,
		Slug:        tenant.Slug,
		Name:        tenant.Name,
		DisplayName: tenant.DisplayName,
		LogoURL:     tenant.LogoURL,
	}, nil
}

// TenantAuthInfo represents basic tenant information for tenant selection during auth
type TenantAuthInfo struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name,omitempty"`
	LogoURL     string    `json:"logo_url,omitempty"`
	Role        string    `json:"role"`
	IsDefault   bool      `json:"is_default"`
}

// UnlockAccount manually unlocks a locked account
func (s *TenantAuthService) UnlockAccount(ctx context.Context, userID, tenantID uuid.UUID, unlockedBy uuid.UUID) error {
	// Reset login attempts and unlock
	now := time.Now()
	updates := map[string]interface{}{
		"login_attempts": 0,
		"locked_until":   nil,
		"updated_at":     now,
		"updated_by":     unlockedBy,
	}

	if err := s.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to unlock account: %w", err)
	}

	// Log unlock event
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tenantID,
		UserID:      &userID,
		EventType:   models.AuthEventAccountUnlocked,
		EventStatus: models.AuthEventStatusSuccess,
		Details:     models.MustNewJSONB(map[string]interface{}{"unlocked_by": unlockedBy}),
	}
	if auditErr := s.credentialRepo.LogAuthEvent(ctx, auditLog); auditErr != nil {
		log.Printf("[TenantAuthService] Warning: Failed to log unlock event: %v", auditErr)
	}

	return nil
}

// logFailedAuthEvent logs a failed authentication event
func (s *TenantAuthService) logFailedAuthEvent(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, email, ipAddress, userAgent, reason string) {
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		EventType:   models.AuthEventLoginFailed,
		EventStatus: models.AuthEventStatusFailed,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Details:     models.MustNewJSONB(map[string]interface{}{"email": email, "reason": reason}),
	}
	if err := s.credentialRepo.LogAuthEvent(ctx, auditLog); err != nil {
		log.Printf("[TenantAuthService] Warning: Failed to log failed auth event: %v", err)
	}
}

// logSuccessAuthEvent logs a successful authentication event
func (s *TenantAuthService) logSuccessAuthEvent(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, ipAddress, userAgent string) {
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tenantID,
		UserID:      userID,
		EventType:   models.AuthEventLoginSuccess,
		EventStatus: models.AuthEventStatusSuccess,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}
	if err := s.credentialRepo.LogAuthEvent(ctx, auditLog); err != nil {
		log.Printf("[TenantAuthService] Warning: Failed to log success auth event: %v", err)
	}
}

// RegisterCustomerRequest represents a request to register a new customer
type RegisterCustomerRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required,min=8"`
	FirstName  string `json:"first_name" validate:"required"`
	LastName   string `json:"last_name" validate:"required"`
	Phone      string `json:"phone,omitempty"`
	TenantSlug string `json:"tenant_slug" validate:"required"`
	IPAddress  string `json:"ip_address,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
}

// RegisterCustomerResponse represents the response from customer registration
type RegisterCustomerResponse struct {
	Success        bool       `json:"success"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	KeycloakUserID string     `json:"keycloak_user_id,omitempty"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	TenantSlug     string     `json:"tenant_slug"`
	Email          string     `json:"email"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`

	// Keycloak tokens for immediate login after registration
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// RegisterCustomer registers a new customer for the storefront
// This creates the user in Keycloak and returns tokens for immediate login
func (s *TenantAuthService) RegisterCustomer(ctx context.Context, req *RegisterCustomerRequest) (*RegisterCustomerResponse, error) {
	// Resolve tenant by slug
	tenant, err := s.membershipRepo.GetTenantBySlug(ctx, req.TenantSlug)
	if err != nil {
		return &RegisterCustomerResponse{
			Success:      false,
			ErrorCode:    "TENANT_NOT_FOUND",
			ErrorMessage: "The specified store was not found",
		}, nil
	}

	// Check if user already exists in Keycloak
	if s.keycloakClient == nil {
		return nil, fmt.Errorf("authentication service not properly configured")
	}

	// Check if email is already registered FOR THIS SPECIFIC TENANT
	// This allows same email to register on different storefronts
	var existingEmailCount int64
	err = s.db.WithContext(ctx).
		Table("tenant_users").
		Joins("JOIN user_tenant_memberships ON tenant_users.id = user_tenant_memberships.user_id").
		Where("tenant_users.email = ? AND user_tenant_memberships.tenant_id = ?", req.Email, tenant.ID).
		Count(&existingEmailCount).Error
	if err != nil {
		log.Printf("[TenantAuthService] Error checking existing email for tenant: %v", err)
	}
	if existingEmailCount > 0 {
		return &RegisterCustomerResponse{
			Success:      false,
			TenantID:     tenant.ID,
			TenantSlug:   tenant.Slug,
			ErrorCode:    "EMAIL_EXISTS",
			ErrorMessage: "An account with this email already exists for this store",
		}, nil
	}

	// Check if phone is already registered FOR THIS SPECIFIC TENANT (if phone provided)
	if req.Phone != "" {
		var existingPhoneCount int64
		err = s.db.WithContext(ctx).
			Table("tenant_users").
			Joins("JOIN user_tenant_memberships ON tenant_users.id = user_tenant_memberships.user_id").
			Where("tenant_users.phone = ? AND user_tenant_memberships.tenant_id = ?", req.Phone, tenant.ID).
			Count(&existingPhoneCount).Error
		if err != nil {
			log.Printf("[TenantAuthService] Error checking existing phone for tenant: %v", err)
		}
		if existingPhoneCount > 0 {
			return &RegisterCustomerResponse{
				Success:      false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "PHONE_EXISTS",
				ErrorMessage: "An account with this phone number already exists for this store",
			}, nil
		}
	}

	// Validate password against tenant's policy
	policy, _ := s.credentialRepo.GetAuthPolicy(ctx, tenant.ID)
	if policy != nil {
		if err := s.validatePasswordPolicy(req.Password, policy); err != nil {
			return &RegisterCustomerResponse{
				Success:      false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "INVALID_PASSWORD",
				ErrorMessage: err.Error(),
			}, nil
		}
	}

	// Check if user already exists in Keycloak (may have registered on another storefront)
	var keycloakUserID string
	var isExistingKeycloakUser bool

	existingKeycloakUser, err := s.keycloakClient.GetUserByEmail(ctx, req.Email)
	if err != nil {
		log.Printf("[TenantAuthService] Error checking Keycloak user: %v", err)
	}

	if existingKeycloakUser != nil && existingKeycloakUser.ID != "" {
		// User exists in Keycloak - they're registering on a new storefront
		keycloakUserID = existingKeycloakUser.ID
		isExistingKeycloakUser = true
		log.Printf("[TenantAuthService] User already exists in Keycloak: %s (registering on new storefront)", keycloakUserID)

		// Verify the password is correct for the existing account
		_, err := s.keycloakClient.GetTokenWithPassword(
			ctx,
			s.keycloakConfig.ClientID,
			s.keycloakConfig.ClientSecret,
			req.Email,
			req.Password,
		)
		if err != nil {
			log.Printf("[TenantAuthService] Password verification failed for existing user: %v", err)
			return &RegisterCustomerResponse{
				Success:      false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "INVALID_CREDENTIALS",
				ErrorMessage: "An account with this email exists. Please use the correct password or log in instead.",
			}, nil
		}
	} else {
		// Create new user in Keycloak
		keycloakUserID, err = s.keycloakClient.CreateUser(ctx, auth.UserRepresentation{
			Email:         req.Email,
			Username:      req.Email,
			FirstName:     req.FirstName,
			LastName:      req.LastName,
			Enabled:       true,
			EmailVerified: false, // Require email verification
		})
		if err != nil {
			log.Printf("[TenantAuthService] Failed to create Keycloak user: %v", err)
			return &RegisterCustomerResponse{
				Success:      false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "REGISTRATION_FAILED",
				ErrorMessage: "Failed to create account. Please try again.",
			}, nil
		}

		log.Printf("[TenantAuthService] Created Keycloak user: %s", keycloakUserID)

		// Set password in Keycloak for new users only
		if err := s.keycloakClient.SetUserPassword(ctx, keycloakUserID, req.Password, false); err != nil {
			log.Printf("[TenantAuthService] Failed to set password: %v", err)
			// Try to clean up the created user
			_ = s.keycloakClient.DeleteUser(ctx, keycloakUserID)
			return &RegisterCustomerResponse{
				Success:      false,
				TenantID:     tenant.ID,
				TenantSlug:   tenant.Slug,
				ErrorCode:    "REGISTRATION_FAILED",
				ErrorMessage: "Failed to set password. Please try again.",
			}, nil
		}
	}

	_ = isExistingKeycloakUser // Mark as used

	// Get tokens via password grant for immediate login
	var tokens *auth.TokenResponse
	if s.keycloakConfig != nil {
		tokens, err = s.keycloakClient.GetTokenWithPassword(
			ctx,
			s.keycloakConfig.ClientID,
			s.keycloakConfig.ClientSecret,
			req.Email,
			req.Password,
		)
		if err != nil {
			log.Printf("[TenantAuthService] Warning: Failed to get tokens after registration: %v", err)
			// Don't fail registration - user can log in manually
		}
	}

	// Parse Keycloak UUID
	keycloakUUID, _ := uuid.Parse(keycloakUserID)

	// Create or get user record in tenant_users (for tracking)
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new user record with phone
			user = models.User{
				Email:      req.Email,
				FirstName:  req.FirstName,
				LastName:   req.LastName,
				Phone:      req.Phone,
				KeycloakID: &keycloakUUID,
			}
			if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
				log.Printf("[TenantAuthService] Warning: Failed to create user record: %v", err)
				// Don't fail - Keycloak user was created successfully
			}
		}
	} else {
		// Update existing user with Keycloak ID and phone if not set
		updates := make(map[string]interface{})
		if user.KeycloakID == nil {
			updates["keycloak_id"] = &keycloakUUID
		}
		if user.Phone == "" && req.Phone != "" {
			updates["phone"] = req.Phone
		}
		if len(updates) > 0 {
			s.db.WithContext(ctx).Model(&user).Updates(updates)
		}
	}

	// Create membership if user was created
	if user.ID != uuid.Nil {
		membership := &models.UserTenantMembership{
			UserID:   user.ID,
			TenantID: tenant.ID,
			Role:     "customer",
			IsActive: true,
		}
		// Use raw SQL for ON CONFLICT DO NOTHING
		s.db.WithContext(ctx).Exec(
			"INSERT INTO user_tenant_memberships (user_id, tenant_id, role, is_active) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING",
			membership.UserID, membership.TenantID, membership.Role, membership.IsActive,
		)
	}

	// Log registration event
	auditLog := &models.TenantAuthAuditLog{
		TenantID:    tenant.ID,
		UserID:      &user.ID,
		EventType:   "account_created",
		EventStatus: models.AuthEventStatusSuccess,
		IPAddress:   req.IPAddress,
		UserAgent:   req.UserAgent,
		Details:     models.MustNewJSONB(map[string]interface{}{"email": req.Email, "method": "direct_registration"}),
	}
	if auditErr := s.credentialRepo.LogAuthEvent(ctx, auditLog); auditErr != nil {
		log.Printf("[TenantAuthService] Warning: Failed to log registration event: %v", auditErr)
	}

	// Send verification email if not already an existing Keycloak user
	// (existing users already verified their email on their first store)
	if !isExistingKeycloakUser && s.verificationClient != nil {
		go func() {
			sendCtx := context.Background()
			_, err := s.verificationClient.SendCode(sendCtx, &clients.SendVerificationCodeRequest{
				Recipient: req.Email,
				Channel:   "email",
				Purpose:   "customer_email_verification",
				TenantID:  &tenant.ID,
				Metadata: map[string]interface{}{
					"first_name":  req.FirstName,
					"tenant_slug": tenant.Slug,
					"store_name":  tenant.Name,
				},
			})
			if err != nil {
				log.Printf("[TenantAuthService] Warning: Failed to send verification email: %v", err)
			} else {
				log.Printf("[TenantAuthService] Verification email sent to %s", security.MaskEmail(req.Email))
			}
		}()
	}

	// Send welcome email
	if s.notificationClient != nil {
		go func() {
			sendCtx := context.Background()
			storeName := tenant.Name
			if storeName == "" {
				storeName = tenant.Slug
			}
			if err := s.notificationClient.SendCustomerWelcomeEmail(sendCtx, req.Email, req.FirstName, storeName); err != nil {
				log.Printf("[TenantAuthService] Warning: Failed to send welcome email: %v", err)
			} else {
				log.Printf("[TenantAuthService] Welcome email sent to %s", security.MaskEmail(req.Email))
			}
		}()
	}

	response := &RegisterCustomerResponse{
		Success:        true,
		UserID:         &user.ID,
		KeycloakUserID: keycloakUserID,
		TenantID:       tenant.ID,
		TenantSlug:     tenant.Slug,
		Email:          req.Email,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
	}

	// Attach tokens if available
	if tokens != nil {
		response.AccessToken = tokens.AccessToken
		response.RefreshToken = tokens.RefreshToken
		response.IDToken = tokens.IDToken
		response.ExpiresIn = tokens.ExpiresIn
	}

	return response, nil
}

// AcceptInvitationPublicRequest represents a request to accept an invitation without authentication
type AcceptInvitationPublicRequest struct {
	Token     string
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     string
}

// AcceptInvitationPublicResponse represents the response from accepting a public invitation
type AcceptInvitationPublicResponse struct {
	Success    bool       `json:"success"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	TenantSlug string     `json:"tenant_slug"`
	Message    string     `json:"message,omitempty"`
}

// CanUnlockAccount checks if the admin user has permission to unlock accounts in a tenant
func (s *TenantAuthService) CanUnlockAccount(ctx context.Context, adminUserID, tenantID uuid.UUID) (bool, error) {
	// Check if the admin user has membership in this tenant with admin role
	membership, err := s.membershipRepo.GetMembership(ctx, adminUserID, tenantID)
	if err != nil {
		return false, fmt.Errorf("failed to get membership: %w", err)
	}

	// Only admin roles can unlock accounts
	return membership.Role == "admin" || membership.Role == "owner", nil
}

// AcceptInvitationPublic accepts a tenant invitation and creates the user account
func (s *TenantAuthService) AcceptInvitationPublic(ctx context.Context, req *AcceptInvitationPublicRequest) (*AcceptInvitationPublicResponse, error) {
	// This is a placeholder implementation
	// The actual implementation would:
	// 1. Validate the invitation token
	// 2. Create the user in Keycloak
	// 3. Create local user record
	// 4. Accept the invitation
	return nil, fmt.Errorf("AcceptInvitationPublic not yet implemented")
}
