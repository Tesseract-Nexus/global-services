package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// CredentialRepository handles tenant credential database operations
// This enables multi-tenant credential isolation where the same user
// can have different passwords for different tenants
type CredentialRepository struct {
	db *gorm.DB
}

// NewCredentialRepository creates a new credential repository
func NewCredentialRepository(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

// ============================================================================
// Tenant Credential Operations
// ============================================================================

// CreateCredential creates a new tenant-specific credential for a user
func (r *CredentialRepository) CreateCredential(ctx context.Context, userID, tenantID uuid.UUID, password string, createdBy *uuid.UUID) (*models.TenantCredential, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	credential := &models.TenantCredential{
		UserID:        userID,
		TenantID:      tenantID,
		PasswordHash:  string(hashedPassword),
		PasswordSetAt: time.Now(),
		CreatedBy:     createdBy,
	}

	if err := r.db.WithContext(ctx).Create(credential).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant credential: %w", err)
	}

	return credential, nil
}

// CreateCredentialWithoutPassword creates a credential record without password
// This is used when password is stored only in Keycloak (single source of truth)
// The record is still needed for MFA settings, login tracking, and session management
func (r *CredentialRepository) CreateCredentialWithoutPassword(ctx context.Context, userID, tenantID uuid.UUID, createdBy *uuid.UUID) (*models.TenantCredential, error) {
	credential := &models.TenantCredential{
		UserID:        userID,
		TenantID:      tenantID,
		PasswordHash:  "", // No password hash - Keycloak is source of truth
		PasswordSetAt: time.Now(),
		CreatedBy:     createdBy,
	}

	if err := r.db.WithContext(ctx).Create(credential).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant credential: %w", err)
	}

	return credential, nil
}

// GetCredential retrieves a user's credential for a specific tenant
func (r *CredentialRepository) GetCredential(ctx context.Context, userID, tenantID uuid.UUID) (*models.TenantCredential, error) {
	var credential models.TenantCredential
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}
	return &credential, nil
}

// GetCredentialByEmail retrieves a user's credential for a specific tenant by email
// This is used during login when we know the email but not the user ID
func (r *CredentialRepository) GetCredentialByEmail(ctx context.Context, email string, tenantID uuid.UUID) (*models.TenantCredential, *models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil // User not found
		}
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	credential, err := r.GetCredential(ctx, user.ID, tenantID)
	if err != nil {
		return nil, nil, err
	}

	return credential, &user, nil
}

// ValidatePassword validates a password against the stored credential
func (r *CredentialRepository) ValidatePassword(ctx context.Context, userID, tenantID uuid.UUID, password string) (bool, error) {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return false, err
	}
	if credential == nil {
		return false, fmt.Errorf("no credential found for user %s in tenant %s", userID, tenantID)
	}

	// Check if account is locked
	if credential.LockedUntil != nil && credential.LockedUntil.After(time.Now()) {
		return false, fmt.Errorf("account is locked until %s", credential.LockedUntil.Format(time.RFC3339))
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password))
	return err == nil, nil
}

// UpdatePassword updates the password for a tenant-specific credential
func (r *CredentialRepository) UpdatePassword(ctx context.Context, userID, tenantID uuid.UUID, newPassword string, updatedBy *uuid.UUID) error {
	// Get current credential for password history
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if credential == nil {
		return fmt.Errorf("credential not found")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// TODO: Add current password to history and check against history

	now := time.Now()
	updates := map[string]interface{}{
		"password_hash":           string(hashedPassword),
		"password_set_at":         now,
		"last_password_change_at": now,
		"login_attempts":          0, // Reset login attempts on password change
		"locked_until":            nil,
		"updated_at":              now,
		"updated_by":              updatedBy,
	}

	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// RecordLoginAttempt records a login attempt and handles lockout logic
func (r *CredentialRepository) RecordLoginAttempt(ctx context.Context, userID, tenantID uuid.UUID, success bool, ipAddress, userAgent string) error {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if credential == nil {
		return nil // No credential to update
	}

	// Get tenant's auth policy
	policy, err := r.GetAuthPolicy(ctx, tenantID)
	if err != nil {
		return err
	}

	maxAttempts := 5
	lockoutMins := 30
	if policy != nil {
		maxAttempts = policy.MaxLoginAttempts
		lockoutMins = policy.LockoutDurationMinutes
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if success {
		// Reset on successful login
		updates["login_attempts"] = 0
		updates["locked_until"] = nil
		updates["last_successful_login_at"] = now
		updates["last_login_ip"] = ipAddress
		updates["last_login_user_agent"] = userAgent
	} else {
		// Increment attempts on failure
		newAttempts := credential.LoginAttempts + 1
		updates["login_attempts"] = newAttempts
		updates["last_login_attempt_at"] = now

		// Lock account if max attempts exceeded
		if newAttempts >= maxAttempts {
			lockedUntil := now.Add(time.Duration(lockoutMins) * time.Minute)
			updates["locked_until"] = lockedUntil
		}
	}

	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to record login attempt: %w", err)
	}

	return nil
}

// CheckAccountLockout checks if an account is currently locked
func (r *CredentialRepository) CheckAccountLockout(ctx context.Context, userID, tenantID uuid.UUID) (bool, *time.Time, int, error) {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return false, nil, 0, err
	}
	if credential == nil {
		return false, nil, 5, nil // Default max attempts
	}

	// Get tenant's auth policy
	policy, err := r.GetAuthPolicy(ctx, tenantID)
	if err != nil {
		return false, nil, 0, err
	}

	maxAttempts := 5
	if policy != nil {
		maxAttempts = policy.MaxLoginAttempts
	}

	// Check if currently locked
	if credential.LockedUntil != nil && credential.LockedUntil.After(time.Now()) {
		return true, credential.LockedUntil, 0, nil
	}

	// Return remaining attempts
	remaining := maxAttempts - credential.LoginAttempts
	if remaining < 0 {
		remaining = 0
	}

	return false, nil, remaining, nil
}

// ============================================================================
// Tenant Auth Policy Operations
// ============================================================================

// GetAuthPolicy retrieves the authentication policy for a tenant
func (r *CredentialRepository) GetAuthPolicy(ctx context.Context, tenantID uuid.UUID) (*models.TenantAuthPolicy, error) {
	var policy models.TenantAuthPolicy
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No custom policy, use defaults
		}
		return nil, fmt.Errorf("failed to get auth policy: %w", err)
	}
	return &policy, nil
}

// CreateAuthPolicy creates a new authentication policy for a tenant
func (r *CredentialRepository) CreateAuthPolicy(ctx context.Context, tenantID uuid.UUID) (*models.TenantAuthPolicy, error) {
	policy := &models.TenantAuthPolicy{
		TenantID: tenantID,
		// All other fields use defaults from the model
	}

	if err := r.db.WithContext(ctx).Create(policy).Error; err != nil {
		return nil, fmt.Errorf("failed to create auth policy: %w", err)
	}

	return policy, nil
}

// UpdateAuthPolicy updates the authentication policy for a tenant
func (r *CredentialRepository) UpdateAuthPolicy(ctx context.Context, policy *models.TenantAuthPolicy) error {
	policy.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(policy).Error; err != nil {
		return fmt.Errorf("failed to update auth policy: %w", err)
	}
	return nil
}

// ============================================================================
// Tenant Auth Audit Log Operations
// ============================================================================

// LogAuthEvent logs an authentication event for audit purposes
func (r *CredentialRepository) LogAuthEvent(ctx context.Context, log *models.TenantAuthAuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to log auth event: %w", err)
	}
	return nil
}

// GetAuthAuditLogs retrieves authentication audit logs for a tenant
func (r *CredentialRepository) GetAuthAuditLogs(ctx context.Context, tenantID uuid.UUID, eventType string, limit, offset int) ([]models.TenantAuthAuditLog, error) {
	var logs []models.TenantAuthAuditLog
	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get auth audit logs: %w", err)
	}

	return logs, nil
}

// GetUserAuthAuditLogs retrieves authentication audit logs for a specific user
func (r *CredentialRepository) GetUserAuthAuditLogs(ctx context.Context, tenantID, userID uuid.UUID, limit int) ([]models.TenantAuthAuditLog, error) {
	var logs []models.TenantAuthAuditLog
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get user auth audit logs: %w", err)
	}
	return logs, nil
}

// ============================================================================
// MFA Operations
// ============================================================================

// EnableMFA enables MFA for a tenant credential
func (r *CredentialRepository) EnableMFA(ctx context.Context, userID, tenantID uuid.UUID, mfaType, mfaSecret string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"mfa_enabled": true,
		"mfa_type":    mfaType,
		"mfa_secret":  mfaSecret, // Should be encrypted before storing
		"updated_at":  now,
	}

	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to enable MFA: %w", err)
	}

	return nil
}

// DisableMFA disables MFA for a tenant credential
func (r *CredentialRepository) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error {
	now := time.Now()
	updates := map[string]interface{}{
		"mfa_enabled":      false,
		"mfa_type":         nil,
		"mfa_secret":       nil,
		"mfa_backup_codes": "[]",
		"updated_at":       now,
	}

	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to disable MFA: %w", err)
	}

	return nil
}

// GetMFASecret retrieves the MFA secret for TOTP verification
func (r *CredentialRepository) GetMFASecret(ctx context.Context, userID, tenantID uuid.UUID) (string, error) {
	var credential models.TenantCredential
	if err := r.db.WithContext(ctx).
		Select("mfa_secret").
		Where("user_id = ? AND tenant_id = ? AND mfa_enabled = ?", userID, tenantID, true).
		First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("MFA not enabled")
		}
		return "", fmt.Errorf("failed to get MFA secret: %w", err)
	}
	return credential.MFASecret, nil
}

// RecordMFAUsage records when MFA was last used
func (r *CredentialRepository) RecordMFAUsage(ctx context.Context, userID, tenantID uuid.UUID) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("mfa_last_used_at", now).Error; err != nil {
		return fmt.Errorf("failed to record MFA usage: %w", err)
	}
	return nil
}

// ============================================================================
// Session Management
// ============================================================================

// IncrementActiveSessions increments the active session count
func (r *CredentialRepository) IncrementActiveSessions(ctx context.Context, userID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		UpdateColumn("active_sessions", gorm.Expr("active_sessions + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to increment active sessions: %w", result.Error)
	}
	return nil
}

// DecrementActiveSessions decrements the active session count
func (r *CredentialRepository) DecrementActiveSessions(ctx context.Context, userID, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ? AND active_sessions > 0", userID, tenantID).
		UpdateColumn("active_sessions", gorm.Expr("active_sessions - 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to decrement active sessions: %w", result.Error)
	}
	return nil
}

// GetActiveSessions returns the current active session count
func (r *CredentialRepository) GetActiveSessions(ctx context.Context, userID, tenantID uuid.UUID) (int, int, error) {
	var credential models.TenantCredential
	if err := r.db.WithContext(ctx).
		Select("active_sessions, max_sessions").
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, 5, nil // Default max sessions
		}
		return 0, 0, fmt.Errorf("failed to get session count: %w", err)
	}
	return credential.ActiveSessions, credential.MaxSessions, nil
}

// ============================================================================
// Bulk Operations
// ============================================================================

// GetAllUserCredentials retrieves all tenant credentials for a user
func (r *CredentialRepository) GetAllUserCredentials(ctx context.Context, userID uuid.UUID) ([]models.TenantCredential, error) {
	var credentials []models.TenantCredential
	if err := r.db.WithContext(ctx).
		Preload("Tenant").
		Where("user_id = ?", userID).
		Find(&credentials).Error; err != nil {
		return nil, fmt.Errorf("failed to get user credentials: %w", err)
	}
	return credentials, nil
}

// DeleteUserCredentials deletes all credentials for a user (when user is deleted)
func (r *CredentialRepository) DeleteUserCredentials(ctx context.Context, userID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.TenantCredential{}).Error; err != nil {
		return fmt.Errorf("failed to delete user credentials: %w", err)
	}
	return nil
}

// DeleteTenantCredentials deletes all credentials for a tenant (when tenant is deleted)
func (r *CredentialRepository) DeleteTenantCredentials(ctx context.Context, tenantID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Delete(&models.TenantCredential{}).Error; err != nil {
		return fmt.Errorf("failed to delete tenant credentials: %w", err)
	}
	return nil
}
