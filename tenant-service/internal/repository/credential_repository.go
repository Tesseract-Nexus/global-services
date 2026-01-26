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

// RecordLoginAttempt records a login attempt and handles progressive lockout logic
// Time-based progressive lockout (NO permanent locks):
// - Tier 1 (5 attempts): 10 minutes lockout
// - Tier 2 (10 attempts): 1 hour lockout
// - Tier 3 (15 attempts): 6 hours lockout
// - Tier 4 (20+ attempts): 24 hours lockout (auto-unlocks after)
func (r *CredentialRepository) RecordLoginAttempt(ctx context.Context, userID, tenantID uuid.UUID, success bool, ipAddress, userAgent string) error {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if credential == nil {
		return nil // No credential to update
	}

	// Check if time-locked (skip permanent lock check - we don't use permanent locks anymore)
	if credential.LockedUntil != nil && credential.LockedUntil.After(time.Now()) {
		return nil // Still locked, don't update
	}

	// Get tenant's auth policy
	policy, err := r.GetAuthPolicy(ctx, tenantID)
	if err != nil {
		return err
	}

	// Default policy values - time-based progressive system (NO permanent locks)
	maxAttempts := 5         // First lockout at 5 attempts
	enableProgressive := true
	tier1Minutes := 10       // 10 minute lockout (Tier 1)
	tier2Minutes := 60       // 1 hour lockout (Tier 2)
	tier3Minutes := 360      // 6 hour lockout (Tier 3)
	tier4Minutes := 1440     // 24 hour lockout (Tier 4 - max, NOT permanent)
	tier2Threshold := 10     // Tier 2 at 10 attempts
	tier3Threshold := 15     // Tier 3 at 15 attempts
	tier4Threshold := 20     // Tier 4 at 20+ attempts
	lockoutResetHours := 48  // Reset counters after 48 hours of no failures

	if policy != nil {
		maxAttempts = policy.MaxLoginAttempts
		enableProgressive = policy.EnableProgressiveLockout
		if policy.Tier1LockoutMinutes > 0 {
			tier1Minutes = policy.Tier1LockoutMinutes
		}
		if policy.LockoutResetHours > 0 {
			lockoutResetHours = policy.LockoutResetHours
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if success {
		// Reset on successful login
		updates["login_attempts"] = 0
		updates["locked_until"] = nil
		updates["current_tier"] = 0
		updates["last_successful_login_at"] = now
		updates["last_login_ip"] = ipAddress
		updates["last_login_user_agent"] = userAgent
		// Note: We don't reset total_failed_attempts or lockout_count on success
		// This allows progressive escalation across lockout cycles
	} else {
		// Check if we should reset tier based on time since last failure
		totalFailed := credential.TotalFailedAttempts
		if credential.LastLoginAttemptAt != nil {
			hoursSinceLastFailure := now.Sub(*credential.LastLoginAttemptAt).Hours()
			if hoursSinceLastFailure >= float64(lockoutResetHours) {
				// Reset progressive tracking after configured hours of inactivity
				totalFailed = 0
				updates["lockout_count"] = 0
				updates["current_tier"] = 0
			}
		}

		// Increment attempts on failure
		newAttempts := credential.LoginAttempts + 1
		totalFailed++
		updates["login_attempts"] = newAttempts
		updates["total_failed_attempts"] = totalFailed
		updates["last_login_attempt_at"] = now

		// Time-based progressive lockout (NO permanent locks - all lockouts auto-unlock)
		if newAttempts >= maxAttempts {
			lockoutCount := credential.LockoutCount + 1
			updates["lockout_count"] = lockoutCount

			// Determine lockout duration based on total failed attempts across sessions
			var lockoutMinutes int
			var tier int

			if enableProgressive {
				if totalFailed >= tier4Threshold {
					// Tier 4: Maximum lockout (24 hours) - still auto-unlocks
					lockoutMinutes = tier4Minutes
					tier = 4
				} else if totalFailed >= tier3Threshold {
					// Tier 3: 6 hour lockout
					lockoutMinutes = tier3Minutes
					tier = 3
				} else if totalFailed >= tier2Threshold {
					// Tier 2: 1 hour lockout
					lockoutMinutes = tier2Minutes
					tier = 2
				} else {
					// Tier 1: 10 minute lockout
					lockoutMinutes = tier1Minutes
					tier = 1
				}
			} else {
				// Progressive disabled - use tier 1 lockout
				lockoutMinutes = tier1Minutes
				tier = 1
			}

			updates["current_tier"] = tier
			lockedUntil := now.Add(time.Duration(lockoutMinutes) * time.Minute)
			updates["locked_until"] = lockedUntil
			// Never set permanently_locked = true - all lockouts are time-based
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

// LockoutStatus contains detailed information about an account's lockout state
type LockoutStatus struct {
	IsLocked            bool       `json:"is_locked"`
	IsPermanentlyLocked bool       `json:"is_permanently_locked"`
	LockedUntil         *time.Time `json:"locked_until,omitempty"`
	CurrentTier         int        `json:"current_tier"`
	LockoutCount        int        `json:"lockout_count"`
	RemainingAttempts   int        `json:"remaining_attempts"`
	TotalFailedAttempts int        `json:"total_failed_attempts"`
	PermanentLockedAt   *time.Time `json:"permanent_locked_at,omitempty"`
	UnlockedBy          *uuid.UUID `json:"unlocked_by,omitempty"`
	UnlockedAt          *time.Time `json:"unlocked_at,omitempty"`
}

// CheckAccountLockout checks if an account is currently locked
func (r *CredentialRepository) CheckAccountLockout(ctx context.Context, userID, tenantID uuid.UUID) (bool, *time.Time, int, error) {
	status, err := r.GetLockoutStatus(ctx, userID, tenantID)
	if err != nil {
		return false, nil, 0, err
	}
	return status.IsLocked || status.IsPermanentlyLocked, status.LockedUntil, status.RemainingAttempts, nil
}

// GetLockoutStatus returns detailed lockout status for an account
func (r *CredentialRepository) GetLockoutStatus(ctx context.Context, userID, tenantID uuid.UUID) (*LockoutStatus, error) {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return &LockoutStatus{
			IsLocked:          false,
			RemainingAttempts: 5, // Default max attempts
		}, nil
	}

	// Get tenant's auth policy
	policy, err := r.GetAuthPolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	maxAttempts := 5
	if policy != nil {
		maxAttempts = policy.MaxLoginAttempts
	}

	status := &LockoutStatus{
		CurrentTier:         credential.CurrentTier,
		LockoutCount:        credential.LockoutCount,
		TotalFailedAttempts: credential.TotalFailedAttempts,
		IsPermanentlyLocked: credential.PermanentlyLocked,
		PermanentLockedAt:   credential.PermanentLockedAt,
		UnlockedBy:          credential.UnlockedBy,
		UnlockedAt:          credential.UnlockedAt,
	}

	// Check if permanently locked
	if credential.PermanentlyLocked {
		status.IsLocked = true
		status.RemainingAttempts = 0
		return status, nil
	}

	// Check if currently locked (time-based)
	if credential.LockedUntil != nil && credential.LockedUntil.After(time.Now()) {
		status.IsLocked = true
		status.LockedUntil = credential.LockedUntil
		status.RemainingAttempts = 0
		return status, nil
	}

	// Not locked - calculate remaining attempts
	remaining := maxAttempts - credential.LoginAttempts
	if remaining < 0 {
		remaining = 0
	}
	status.RemainingAttempts = remaining

	return status, nil
}

// UnlockAccount unlocks a locked account (admin operation)
// This resets the lockout state but preserves audit trail
func (r *CredentialRepository) UnlockAccount(ctx context.Context, userID, tenantID, adminUserID uuid.UUID) error {
	credential, err := r.GetCredential(ctx, userID, tenantID)
	if err != nil {
		return err
	}
	if credential == nil {
		return fmt.Errorf("credential not found for user %s in tenant %s", userID, tenantID)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"login_attempts":        0,
		"locked_until":          nil,
		"permanently_locked":    false,
		"permanent_locked_at":   nil,
		"current_tier":          0,
		"total_failed_attempts": 0,
		"lockout_count":         0,
		"unlocked_by":           adminUserID,
		"unlocked_at":           now,
		"updated_at":            now,
	}

	if err := r.db.WithContext(ctx).
		Model(&models.TenantCredential{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to unlock account: %w", err)
	}

	return nil
}

// LockedAccountInfo contains information about a locked account for admin listing
type LockedAccountInfo struct {
	UserID              uuid.UUID  `json:"user_id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	Email               string     `json:"email"`
	FirstName           string     `json:"first_name"`
	LastName            string     `json:"last_name"`
	IsPermanentlyLocked bool       `json:"is_permanently_locked"`
	LockedUntil         *time.Time `json:"locked_until,omitempty"`
	CurrentTier         int        `json:"current_tier"`
	LockoutCount        int        `json:"lockout_count"`
	TotalFailedAttempts int        `json:"total_failed_attempts"`
	PermanentLockedAt   *time.Time `json:"permanent_locked_at,omitempty"`
	LastLoginAttemptAt  *time.Time `json:"last_login_attempt_at,omitempty"`
	LastLoginIP         string     `json:"last_login_ip,omitempty"`
}

// ListLockedAccounts returns all locked accounts for a tenant
func (r *CredentialRepository) ListLockedAccounts(ctx context.Context, tenantID uuid.UUID, permanentOnly bool) ([]LockedAccountInfo, error) {
	var results []LockedAccountInfo

	query := r.db.WithContext(ctx).
		Table("tenant_credentials tc").
		Select(`
			tc.user_id,
			tc.tenant_id,
			u.email,
			u.first_name,
			u.last_name,
			tc.permanently_locked as is_permanently_locked,
			tc.locked_until,
			tc.current_tier,
			tc.lockout_count,
			tc.total_failed_attempts,
			tc.permanent_locked_at,
			tc.last_login_attempt_at,
			tc.last_login_ip
		`).
		Joins("JOIN tenant_users u ON tc.user_id = u.id").
		Where("tc.tenant_id = ?", tenantID)

	if permanentOnly {
		query = query.Where("tc.permanently_locked = ?", true)
	} else {
		// Include both permanent and time-based locks
		query = query.Where("tc.permanently_locked = ? OR (tc.locked_until IS NOT NULL AND tc.locked_until > ?)", true, time.Now())
	}

	if err := query.Order("tc.permanent_locked_at DESC NULLS LAST, tc.locked_until DESC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to list locked accounts: %w", err)
	}

	return results, nil
}

// ListPermanentlyLockedAccounts returns all permanently locked accounts for a tenant
func (r *CredentialRepository) ListPermanentlyLockedAccounts(ctx context.Context, tenantID uuid.UUID) ([]LockedAccountInfo, error) {
	return r.ListLockedAccounts(ctx, tenantID, true)
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
