package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"auth-service/internal/models"
	"auth-service/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo       *repository.AuthRepository
	jwtService *JWTService
}

func NewAuthService(repo *repository.AuthRepository, jwtService *JWTService) *AuthService {
	return &AuthService{
		repo:       repo,
		jwtService: jwtService,
	}
}

// AuthenticateUser authenticates a user and creates a session
func (s *AuthService) AuthenticateUser(email, azureObjectID, name, tenantID, ipAddress, userAgent string) (*models.User, string, string, error) {
	var user *models.User
	var err error

	// For backward compatibility, try to find user by email

	// If not found by Azure Object ID, try by email
	if user == nil {
		user, err = s.repo.GetUserByEmail(email)
		if err != nil && err.Error() != "user not found" {
			return nil, "", "", fmt.Errorf("failed to get user by email: %w", err)
		}
	}

	// If user doesn't exist, create a new one
	if user == nil {
		// Parse tenant ID
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid tenant ID: %w", err)
		}

		// Split name into first and last name
		firstName := name
		lastName := ""
		parts := strings.Split(name, " ")
		if len(parts) > 1 {
			firstName = parts[0]
			lastName = strings.Join(parts[1:], " ")
		}

		user = &models.User{
			Email:     email,
			FirstName: firstName,
			LastName:  lastName,
			TenantID:  tenantUUID,
			Role:      models.RoleCustomer,
			Status:    "active",
		}

		if err := s.repo.CreateUser(user); err != nil {
			return nil, "", "", fmt.Errorf("failed to create user: %w", err)
		}

		// Assign default role to new user
		if err := s.assignDefaultRole(user); err != nil {
			return nil, "", "", fmt.Errorf("failed to assign default role: %w", err)
		}
	} else {
		// Update last login time
		now := time.Now()
		user.LastLoginAt = &now
		if err := s.repo.UpdateUser(user); err != nil {
			return nil, "", "", fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Get user with roles and permissions
	userWithPerms, err := s.repo.GetUserWithRolesAndPermissions(user.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Create session
	sessionID := uuid.New()
	accessToken, refreshToken, err := s.jwtService.GenerateTokens(userWithPerms, sessionID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Store session in database
	session := &models.Session{
		ID:           sessionID,
		UserID:       user.ID,
		TenantID:     tenantID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtService.GetRefreshTokenExpiry()),
		IsActive:     true,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	if err := s.repo.CreateSession(session); err != nil {
		return nil, "", "", fmt.Errorf("failed to create session: %w", err)
	}

	return userWithPerms, accessToken, refreshToken, nil
}

// assignDefaultRole assigns a default role to a new user
func (s *AuthService) assignDefaultRole(user *models.User) error {
	// Assign 'staff' role as default for new users
	// If the role doesn't exist (roles table not created), skip silently
	tenantIDStr := user.TenantID.String()
	role, err := s.repo.GetRoleByName(models.RoleStaff, tenantIDStr)
	if err != nil {
		// Role not found - skip assignment but don't fail user creation
		// This allows the system to work without the roles feature fully set up
		log.Printf("[AuthService] Warning: Could not assign default role to user %s: %v", user.Email, err)
		return nil
	}

	if err := s.repo.AssignRoleToUser(user.ID, role.ID, tenantIDStr); err != nil {
		log.Printf("[AuthService] Warning: Failed to assign role to user %s: %v", user.Email, err)
		return nil
	}
	return nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *AuthService) RefreshToken(refreshToken string) (string, string, error) {
	return s.jwtService.RefreshTokens(refreshToken, s.repo)
}

// ValidateToken validates an access token and returns user claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwtService.ValidateAccessToken(tokenString)
}

// RevokeToken revokes a token (logout)
func (s *AuthService) RevokeToken(tokenString string) error {
	return s.jwtService.RevokeToken(tokenString, s.repo)
}

// User Management

// GetUser retrieves a user by ID
func (s *AuthService) GetUser(userID uuid.UUID) (*models.User, error) {
	return s.repo.GetUserWithRolesAndPermissions(userID)
}

// GetUserByEmail retrieves a user by email
func (s *AuthService) GetUserByEmail(email string) (*models.User, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}
	return s.repo.GetUserWithRolesAndPermissions(user.ID)
}

// ListUsers retrieves users with pagination
func (s *AuthService) ListUsers(tenantID string, limit, offset int) ([]models.User, error) {
	return s.repo.ListUsers(tenantID, limit, offset)
}

// UpdateUser updates a user
func (s *AuthService) UpdateUser(user *models.User) error {
	return s.repo.UpdateUser(user)
}

// DeactivateUser deactivates a user
func (s *AuthService) DeactivateUser(userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}

	user.IsActive = false
	if err := s.repo.UpdateUser(user); err != nil {
		return err
	}

	// Deactivate all user sessions
	return s.repo.DeactivateUserSessions(userID)
}

// Role Management

// AssignRole assigns a role to a user
func (s *AuthService) AssignRole(userID uuid.UUID, roleName, tenantID string) error {
	role, err := s.repo.GetRoleByName(roleName, tenantID)
	if err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	return s.repo.AssignRoleToUser(userID, role.ID, tenantID)
}

// RemoveRole removes a role from a user
func (s *AuthService) RemoveRole(userID uuid.UUID, roleName, tenantID string) error {
	role, err := s.repo.GetRoleByName(roleName, tenantID)
	if err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	return s.repo.RemoveRoleFromUser(userID, role.ID, tenantID)
}

// GetUserRoles retrieves all roles for a user
func (s *AuthService) GetUserRoles(userID uuid.UUID) ([]models.Role, error) {
	return s.repo.GetUserRoles(userID)
}

// Permission Checking

// HasPermission checks if a user has a specific permission
func (s *AuthService) HasPermission(userID uuid.UUID, permission string) (bool, error) {
	return s.repo.HasPermission(userID, permission)
}

// HasRole checks if a user has a specific role
func (s *AuthService) HasRole(userID uuid.UUID, roleName string) (bool, error) {
	return s.repo.HasRole(userID, roleName)
}

// HasAnyRole checks if a user has any of the specified roles
func (s *AuthService) HasAnyRole(userID uuid.UUID, roleNames []string) (bool, error) {
	for _, roleName := range roleNames {
		hasRole, err := s.repo.HasRole(userID, roleName)
		if err != nil {
			return false, err
		}
		if hasRole {
			return true, nil
		}
	}
	return false, nil
}

// CheckPermissions checks multiple permissions at once
func (s *AuthService) CheckPermissions(userID uuid.UUID, permissions []string) (map[string]bool, error) {
	result := make(map[string]bool)

	for _, permission := range permissions {
		hasPermission, err := s.repo.HasPermission(userID, permission)
		if err != nil {
			return nil, fmt.Errorf("failed to check permission %s: %w", permission, err)
		}
		result[permission] = hasPermission
	}

	return result, nil
}

// Session Management

// GetActiveSessions retrieves active sessions for a user
func (s *AuthService) GetActiveSessions(userID uuid.UUID) ([]models.Session, error) {
	// This would require a new repository method - for now, return empty slice
	return []models.Session{}, nil
}

// RevokeAllSessions revokes all sessions for a user
func (s *AuthService) RevokeAllSessions(userID uuid.UUID) error {
	return s.repo.DeactivateUserSessions(userID)
}

// CleanupExpiredSessions removes expired sessions from the database
func (s *AuthService) CleanupExpiredSessions() error {
	return s.repo.CleanupExpiredSessions()
}

// Utility methods

// IsSystemRole checks if a role is a system role
func (s *AuthService) IsSystemRole(roleName string) bool {
	systemRoles := []string{
		models.RoleSuperAdmin,
		models.RoleTenantAdmin,
		models.RoleCategoryManager,
		models.RoleProductManager,
		models.RoleVendorManager,
		models.RoleStaff,
		models.RoleVendor,
		models.RoleCustomer,
	}

	for _, sysRole := range systemRoles {
		if roleName == sysRole {
			return true
		}
	}
	return false
}

// GetAvailableRoles returns available roles for assignment
func (s *AuthService) GetAvailableRoles() []string {
	return []string{
		models.RoleTenantAdmin,
		models.RoleCategoryManager,
		models.RoleProductManager,
		models.RoleVendorManager,
		models.RoleStaff,
		models.RoleVendor,
		models.RoleCustomer,
	}
}

// GetAvailablePermissions returns all available permissions
func (s *AuthService) GetAvailablePermissions() []string {
	permissions := models.SystemPermissions()
	result := make([]string, len(permissions))
	for i, perm := range permissions {
		result[i] = perm.Name
	}
	return result
}

// GetTokenExpiry returns the token expiry duration
func (s *AuthService) GetTokenExpiry() time.Duration {
	return 15 * time.Minute // Default 15 minutes
}

// Password Authentication Methods

// RegisterUser registers a new user with password
func (s *AuthService) RegisterUser(user *models.User) (*models.User, error) {
	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign default role
	if err := s.assignDefaultRole(user); err != nil {
		return nil, fmt.Errorf("failed to assign default role: %w", err)
	}

	return user, nil
}

// AuthenticateWithPassword authenticates user with email/password
func (s *AuthService) AuthenticateWithPassword(email, password string, storeID *uuid.UUID, tenantID, ipAddress, userAgent string) (*models.User, string, string, error) {
	user, err := s.repo.GetUserByEmailAndStore(email, storeID, tenantID)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid credentials")
	}

	if user.Password == "" {
		return nil, "", "", fmt.Errorf("password authentication not available")
	}

	// Verify password using bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", "", fmt.Errorf("invalid credentials")
	}

	// Get user with roles and permissions
	userWithPerms, err := s.repo.GetUserWithRolesAndPermissions(user.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get user permissions: %w", err)
	}

	// If a specific tenant_id was requested, verify access and use it for the JWT
	if tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err == nil {
			// Check if user has membership to the requested tenant
			hasMembership, membershipRole := s.repo.CheckUserTenantMembership(user.ID, tenantUUID)
			if hasMembership {
				// Override user's tenant_id with the requested one for JWT generation
				userWithPerms.TenantID = tenantUUID
				// Also update the role if the membership has a specific role (e.g., "owner", "admin")
				if membershipRole != "" {
					userWithPerms.Role = membershipRole
				}
			}
		}
	}

	// Create session
	sessionID := uuid.New()
	accessToken, refreshToken, err := s.jwtService.GenerateTokens(userWithPerms, sessionID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Store session in database
	session := &models.Session{
		ID:                sessionID,
		UserID:            user.ID,
		TenantID:          tenantID,
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		ExpiresAt:         time.Now().Add(s.jwtService.GetRefreshTokenExpiry()),
		IsActive:          true,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
		TwoFactorVerified: false, // Will be updated after 2FA verification
	}

	if err := s.repo.CreateSession(session); err != nil {
		return nil, "", "", fmt.Errorf("failed to create session: %w", err)
	}

	return userWithPerms, accessToken, refreshToken, nil
}

// GetUserByEmailAndStore gets user by email and store
func (s *AuthService) GetUserByEmailAndStore(email string, storeID *uuid.UUID, tenantID string) (*models.User, error) {
	return s.repo.GetUserByEmailAndStore(email, storeID, tenantID)
}

// UpdateLastLogin updates the user's last login time
func (s *AuthService) UpdateLastLogin(userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}

	now := time.Now()
	user.LastLoginAt = &now
	return s.repo.UpdateUser(user)
}

// UpdatePassword updates user's password
func (s *AuthService) UpdatePassword(userID uuid.UUID, hashedPassword string) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}

	user.PasswordHash = &hashedPassword
	return s.repo.UpdateUser(user)
}

// RevokeAllUserSessions revokes all sessions for a user
func (s *AuthService) RevokeAllUserSessions(userID uuid.UUID) error {
	return s.repo.DeactivateUserSessions(userID)
}

// GetUserByID retrieves a user by ID with roles and permissions
func (s *AuthService) GetUserByID(userID uuid.UUID) (*models.User, error) {
	return s.repo.GetUserWithRolesAndPermissions(userID)
}

// RevokeOtherUserSessions revokes all sessions except the specified session
func (s *AuthService) RevokeOtherUserSessions(userID uuid.UUID, currentSessionID string) error {
	return s.repo.DeactivateOtherSessions(userID, currentSessionID)
}

// Email Verification Methods

// SaveVerificationToken saves a verification token
func (s *AuthService) SaveVerificationToken(userID uuid.UUID, token, tokenType string, expiresAt time.Time) error {
	return s.repo.SaveVerificationToken(userID, token, tokenType, expiresAt)
}

// VerifyToken verifies a token and returns the user ID
func (s *AuthService) VerifyToken(token, tokenType string) (uuid.UUID, error) {
	return s.repo.VerifyToken(token, tokenType)
}

// MarkTokenAsUsed marks a token as used
func (s *AuthService) MarkTokenAsUsed(token string) error {
	return s.repo.MarkTokenAsUsed(token)
}

// MarkEmailAsVerified marks user's email as verified
func (s *AuthService) MarkEmailAsVerified(userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}

	user.EmailVerified = true
	return s.repo.UpdateUser(user)
}

// 2FA Methods

// StoreTempTOTPSecret stores a temporary TOTP secret during setup
func (s *AuthService) StoreTempTOTPSecret(userID uuid.UUID, secret string) error {
	return s.repo.StoreTempTOTPSecret(userID, secret)
}

// GetTempTOTPSecret gets a temporary TOTP secret
func (s *AuthService) GetTempTOTPSecret(userID uuid.UUID) (string, error) {
	return s.repo.GetTempTOTPSecret(userID)
}

// StoreTempBackupCodes stores temporary backup codes during setup
func (s *AuthService) StoreTempBackupCodes(userID uuid.UUID, hashedCodes []string) error {
	return s.repo.StoreTempBackupCodes(userID, hashedCodes)
}

// Enable2FA enables 2FA for a user
func (s *AuthService) Enable2FA(userID uuid.UUID, secret string) error {
	return s.repo.Enable2FA(userID, secret)
}

// ActivateBackupCodes moves temporary backup codes to permanent storage
func (s *AuthService) ActivateBackupCodes(userID uuid.UUID) error {
	return s.repo.ActivateBackupCodes(userID)
}

// ClearTempTOTPData clears temporary TOTP setup data
func (s *AuthService) ClearTempTOTPData(userID uuid.UUID) error {
	return s.repo.ClearTempTOTPData(userID)
}

// GetSessionByID gets a session by ID
func (s *AuthService) GetSessionByID(sessionID string) (*models.Session, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID format")
	}
	return s.repo.GetSessionByID(sessionUUID)
}

// LogTwoFactorAttempt logs a 2FA attempt
func (s *AuthService) LogTwoFactorAttempt(userID uuid.UUID, attemptType string, success bool, ipAddress, userAgent string) error {
	return s.repo.LogTwoFactorAttempt(userID, attemptType, success, ipAddress, userAgent)
}

// GetUserBackupCodes gets user backup codes
func (s *AuthService) GetUserBackupCodes(userID uuid.UUID) ([]models.BackupCode, error) {
	return s.repo.GetUserBackupCodes(userID)
}

// UseBackupCode marks a backup code as used
func (s *AuthService) UseBackupCode(codeID uuid.UUID) error {
	return s.repo.UseBackupCode(codeID)
}

// Mark2FAVerified marks a session as 2FA verified
func (s *AuthService) Mark2FAVerified(sessionID uuid.UUID) error {
	return s.repo.Mark2FAVerified(sessionID)
}

// UpdateLast2FAVerification updates the user's last 2FA verification time
func (s *AuthService) UpdateLast2FAVerification(userID uuid.UUID) error {
	return s.repo.UpdateLast2FAVerification(userID)
}

// GetBackupCodesCount gets the count of remaining backup codes
func (s *AuthService) GetBackupCodesCount(userID uuid.UUID) (int, error) {
	return s.repo.GetBackupCodesCount(userID)
}

// Disable2FA disables 2FA for a user
func (s *AuthService) Disable2FA(userID uuid.UUID) error {
	return s.repo.Disable2FA(userID)
}

// Get2FAStatus gets the 2FA status for a user
func (s *AuthService) Get2FAStatus(userID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.Get2FAStatus(userID)
}

// ReplaceBackupCodes replaces backup codes for a user
func (s *AuthService) ReplaceBackupCodes(userID uuid.UUID, hashedCodes []string) error {
	return s.repo.ReplaceBackupCodes(userID, hashedCodes)
}

// GetRecoveryInfo gets recovery information for a user
func (s *AuthService) GetRecoveryInfo(userID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.GetRecoveryInfo(userID)
}
