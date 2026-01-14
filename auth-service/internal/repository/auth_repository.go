package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"auth-service/internal/models"
)

type AuthRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{
		db: db,
	}
}

// User management

// CreateUser creates a new user
func (r *AuthRepository) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (id, tenant_id, vendor_id, email, first_name, last_name, phone, role, status, password)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Default values
	if user.Role == "" {
		user.Role = models.RoleCustomer
	}
	if user.Status == "" {
		user.Status = "active"
	}

	_, err := r.db.Exec(query, user.ID, user.TenantID, user.VendorID, user.Email, user.FirstName,
		user.LastName, user.Phone, user.Role, user.Status, user.Password)
	return err
}

// GetUserByID retrieves a user by ID
func (r *AuthRepository) GetUserByID(userID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, tenant_id, vendor_id, email, first_name, last_name, phone, role, status, password
		FROM users WHERE id = $1
	`

	user := &models.User{}
	var phone sql.NullString
	var vendorID sql.NullString

	err := r.db.QueryRow(query, userID).Scan(
		&user.ID, &user.TenantID, &vendorID, &user.Email, &user.FirstName, &user.LastName,
		&phone, &user.Role, &user.Status, &user.Password,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	// Handle nullable fields
	if phone.Valid {
		user.Phone = &phone.String
	}
	if vendorID.Valid {
		user.VendorID = &vendorID.String
	}

	// Compute derived fields
	user.Name = user.FirstName + " " + user.LastName
	user.IsActive = user.Status == "active"

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (r *AuthRepository) GetUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, tenant_id, vendor_id, email, first_name, last_name, phone, role, status, password
		FROM users WHERE email = $1
		LIMIT 1
	`

	return r.getUserByQuery(query, email)
}

// GetUserByEmailAndStore retrieves a user by email and store
func (r *AuthRepository) GetUserByEmailAndStore(email string, storeID *uuid.UUID, tenantID string) (*models.User, error) {
	var query string
	var args []interface{}

	// If tenant_id is empty, query by email only
	if tenantID == "" {
		query = `
			SELECT id, tenant_id, vendor_id, email, first_name, last_name, phone, role, status, password
			FROM users WHERE email = $1 LIMIT 1
		`
		args = []interface{}{email}
	} else {
		// Query by email and tenant_id (store_id column doesn't exist in schema)
		query = `
			SELECT id, tenant_id, vendor_id, email, first_name, last_name, phone, role, status, password
			FROM users WHERE email = $1 AND tenant_id = $2 LIMIT 1
		`
		args = []interface{}{email, tenantID}
	}

	return r.getUserByQuery(query, args...)
}

// Helper method to scan user from query
func (r *AuthRepository) getUserByQuery(query string, args ...interface{}) (*models.User, error) {
	user := &models.User{}
	var phone sql.NullString
	var vendorID sql.NullString

	err := r.db.QueryRow(query, args...).Scan(
		&user.ID, &user.TenantID, &vendorID, &user.Email, &user.FirstName, &user.LastName,
		&phone, &user.Role, &user.Status, &user.Password,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	// Handle nullable fields
	if phone.Valid {
		user.Phone = &phone.String
	}
	if vendorID.Valid {
		user.VendorID = &vendorID.String
	}

	// Compute derived fields
	user.Name = user.FirstName + " " + user.LastName
	user.IsActive = user.Status == "active"

	return user, nil
}

// UpdateUser updates a user
func (r *AuthRepository) UpdateUser(user *models.User) error {
	query := `
		UPDATE users
		SET email = $2, first_name = $3, last_name = $4, phone = $5,
			role = $6, status = $7, password = $8, vendor_id = $9
		WHERE id = $1
	`

	user.UpdatedAt = time.Now()

	_, err := r.db.Exec(query, user.ID, user.Email, user.FirstName, user.LastName,
		user.Phone, user.Role, user.Status, user.Password, user.VendorID)

	return err
}

// GetUserWithRolesAndPermissions retrieves a user with their roles and permissions
func (r *AuthRepository) GetUserWithRolesAndPermissions(userID uuid.UUID) (*models.User, error) {
	user, err := r.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Get user roles
	roles, err := r.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	// Get all permissions (from roles and direct assignments)
	permissions, err := r.GetUserPermissions(userID)
	if err != nil {
		return nil, err
	}
	user.Permissions = permissions

	return user, nil
}

// ListUsers lists users for a tenant with pagination
func (r *AuthRepository) ListUsers(tenantID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT id, tenant_id, email, first_name, last_name, phone, role, status, password
		FROM users
		WHERE tenant_id = $1
		ORDER BY email
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var phone sql.NullString

		err := rows.Scan(
			&user.ID, &user.TenantID, &user.Email, &user.FirstName, &user.LastName,
			&phone, &user.Role, &user.Status, &user.Password,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if phone.Valid {
			user.Phone = &phone.String
		}

		// Compute derived fields
		user.Name = user.FirstName + " " + user.LastName
		user.IsActive = user.Status == "active"

		users = append(users, user)
	}

	return users, nil
}

// Session management

// CreateSession creates a new session
func (r *AuthRepository) CreateSession(session *models.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, tenant_id, access_token, refresh_token, expires_at, 
			is_active, ip_address, user_agent, two_factor_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	_, err := r.db.Exec(query, session.ID, session.UserID, session.TenantID, session.AccessToken,
		session.RefreshToken, session.ExpiresAt, session.IsActive, session.IPAddress,
		session.UserAgent, session.TwoFactorVerified, session.CreatedAt, session.UpdatedAt)
	return err
}

// GetSession retrieves a session by ID (alias for GetSessionByID)
func (r *AuthRepository) GetSession(sessionID uuid.UUID) (*models.Session, error) {
	return r.GetSessionByID(sessionID)
}

// GetSessionByID retrieves a session by ID
func (r *AuthRepository) GetSessionByID(sessionID uuid.UUID) (*models.Session, error) {
	query := `
		SELECT id, user_id, tenant_id, access_token, refresh_token, expires_at, is_active,
			ip_address, user_agent, two_factor_verified, two_factor_verified_at, created_at, updated_at
		FROM sessions WHERE id = $1 AND is_active = true
	`

	session := &models.Session{}
	var twoFactorVerifiedAt sql.NullTime

	err := r.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.UserID, &session.TenantID, &session.AccessToken,
		&session.RefreshToken, &session.ExpiresAt, &session.IsActive, &session.IPAddress,
		&session.UserAgent, &session.TwoFactorVerified, &twoFactorVerifiedAt,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}

	if twoFactorVerifiedAt.Valid {
		session.TwoFactorVerifiedAt = &twoFactorVerifiedAt.Time
	}

	return session, nil
}

// GetSessionByToken retrieves a session by token
func (r *AuthRepository) GetSessionByToken(token string) (*models.Session, error) {
	query := `
		SELECT id, user_id, tenant_id, access_token, refresh_token, expires_at, is_active,
			ip_address, user_agent, two_factor_verified, two_factor_verified_at, created_at, updated_at
		FROM sessions WHERE (access_token = $1 OR refresh_token = $1) AND is_active = true
	`

	session := &models.Session{}
	var twoFactorVerifiedAt sql.NullTime

	err := r.db.QueryRow(query, token).Scan(
		&session.ID, &session.UserID, &session.TenantID, &session.AccessToken,
		&session.RefreshToken, &session.ExpiresAt, &session.IsActive, &session.IPAddress,
		&session.UserAgent, &session.TwoFactorVerified, &twoFactorVerifiedAt,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}

	if twoFactorVerifiedAt.Valid {
		session.TwoFactorVerifiedAt = &twoFactorVerifiedAt.Time
	}

	return session, nil
}

// UpdateSession updates a session
func (r *AuthRepository) UpdateSession(session *models.Session) error {
	query := `
		UPDATE sessions 
		SET access_token = $2, refresh_token = $3, expires_at = $4, is_active = $5,
			two_factor_verified = $6, two_factor_verified_at = $7, updated_at = $8
		WHERE id = $1
	`

	session.UpdatedAt = time.Now()

	_, err := r.db.Exec(query, session.ID, session.AccessToken, session.RefreshToken,
		session.ExpiresAt, session.IsActive, session.TwoFactorVerified,
		session.TwoFactorVerifiedAt, session.UpdatedAt)
	return err
}

// DeactivateSession deactivates a session
func (r *AuthRepository) DeactivateSession(sessionID uuid.UUID) error {
	query := `UPDATE sessions SET is_active = false, updated_at = $2 WHERE id = $1`
	_, err := r.db.Exec(query, sessionID, time.Now())
	return err
}

// DeactivateUserSessions deactivates all sessions for a user
func (r *AuthRepository) DeactivateUserSessions(userID uuid.UUID) error {
	query := `UPDATE sessions SET is_active = false, updated_at = $2 WHERE user_id = $1`
	_, err := r.db.Exec(query, userID, time.Now())
	return err
}

// DeactivateOtherSessions deactivates all sessions except the specified session
func (r *AuthRepository) DeactivateOtherSessions(userID uuid.UUID, currentSessionID string) error {
	query := `UPDATE sessions SET is_active = false, updated_at = $3 WHERE user_id = $1 AND id != $2`
	_, err := r.db.Exec(query, userID, currentSessionID, time.Now())
	return err
}

// CleanupExpiredSessions removes expired sessions
func (r *AuthRepository) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expires_at < $1 OR is_active = false`
	_, err := r.db.Exec(query, time.Now())
	return err
}

// Role management

// GetRoleByName retrieves a role by name and tenant
func (r *AuthRepository) GetRoleByName(name, tenantID string) (*models.Role, error) {
	query := `SELECT id, name, description, tenant_id, is_system, created_at, updated_at 
			  FROM roles WHERE name = $1 AND tenant_id = $2`

	role := &models.Role{}
	err := r.db.QueryRow(query, name, tenantID).Scan(
		&role.ID, &role.Name, &role.Description, &role.TenantID,
		&role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role not found")
		}
		return nil, err
	}

	return role, nil
}

// AssignRoleToUser assigns a role to a user
func (r *AuthRepository) AssignRoleToUser(userID, roleID uuid.UUID, tenantID string) error {
	query := `
		INSERT INTO user_roles (id, user_id, role_id, tenant_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, role_id, tenant_id) DO NOTHING
	`

	_, err := r.db.Exec(query, uuid.New(), userID, roleID, tenantID, time.Now())
	return err
}

// RemoveRoleFromUser removes a role from a user
func (r *AuthRepository) RemoveRoleFromUser(userID, roleID uuid.UUID, tenantID string) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2 AND tenant_id = $3`
	_, err := r.db.Exec(query, userID, roleID, tenantID)
	return err
}

// GetUserRoles retrieves all roles for a user
func (r *AuthRepository) GetUserRoles(userID uuid.UUID) ([]models.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.tenant_id, r.is_system, r.created_at, r.updated_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.TenantID,
			&role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// Permission management

// GetUserPermissions retrieves all permissions for a user (from roles and direct assignments)
func (r *AuthRepository) GetUserPermissions(userID uuid.UUID) ([]models.Permission, error) {
	query := `
		SELECT DISTINCT p.id, p.name, p.resource, p.action, p.description, p.is_system, p.created_at, p.updated_at
		FROM permissions p
		WHERE p.id IN (
			-- Permissions from roles
			SELECT rp.permission_id
			FROM role_permissions rp
			INNER JOIN user_roles ur ON rp.role_id = ur.role_id
			WHERE ur.user_id = $1
			
			UNION
			
			-- Direct permissions
			SELECT up.permission_id
			FROM user_permissions up
			WHERE up.user_id = $1
		)
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []models.Permission
	for rows.Next() {
		var perm models.Permission
		err := rows.Scan(&perm.ID, &perm.Name, &perm.Resource, &perm.Action,
			&perm.Description, &perm.IsSystem, &perm.CreatedAt, &perm.UpdatedAt)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// HasPermission checks if a user has a specific permission
func (r *AuthRepository) HasPermission(userID uuid.UUID, permission string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM permissions p
			WHERE p.name = $2 AND p.id IN (
				-- Permissions from roles
				SELECT rp.permission_id
				FROM role_permissions rp
				INNER JOIN user_roles ur ON rp.role_id = ur.role_id
				WHERE ur.user_id = $1
				
				UNION
				
				-- Direct permissions
				SELECT up.permission_id
				FROM user_permissions up
				WHERE up.user_id = $1
			)
		)
	`

	var exists bool
	err := r.db.QueryRow(query, userID, permission).Scan(&exists)
	return exists, err
}

// HasRole checks if a user has a specific role
func (r *AuthRepository) HasRole(userID uuid.UUID, roleName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM user_roles ur
			INNER JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = $1 AND r.name = $2
		)
	`

	var exists bool
	err := r.db.QueryRow(query, userID, roleName).Scan(&exists)
	return exists, err
}

// 2FA Methods

// StoreTempTOTPSecret stores a temporary TOTP secret during setup
func (r *AuthRepository) StoreTempTOTPSecret(userID uuid.UUID, secret string) error {
	// Store in a temporary cache or session storage
	// For now, we'll use a simple approach - you might want to use Redis
	query := `
		INSERT INTO temp_totp_secrets (user_id, secret, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET secret = $2, expires_at = $3
	`
	expiresAt := time.Now().Add(10 * time.Minute)
	_, err := r.db.Exec(query, userID, secret, expiresAt)
	return err
}

// GetTempTOTPSecret gets a temporary TOTP secret
func (r *AuthRepository) GetTempTOTPSecret(userID uuid.UUID) (string, error) {
	query := `
		SELECT secret FROM temp_totp_secrets 
		WHERE user_id = $1 AND expires_at > $2
	`
	var secret string
	err := r.db.QueryRow(query, userID, time.Now()).Scan(&secret)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no pending 2FA setup found")
		}
		return "", err
	}
	return secret, nil
}

// StoreTempBackupCodes stores temporary backup codes during setup
func (r *AuthRepository) StoreTempBackupCodes(userID uuid.UUID, hashedCodes []string) error {
	// Store in temporary table
	query := `DELETE FROM temp_backup_codes WHERE user_id = $1`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return err
	}

	for _, code := range hashedCodes {
		query := `INSERT INTO temp_backup_codes (user_id, code_hash) VALUES ($1, $2)`
		_, err := r.db.Exec(query, userID, code)
		if err != nil {
			return err
		}
	}
	return nil
}

// Enable2FA enables 2FA for a user
func (r *AuthRepository) Enable2FA(userID uuid.UUID, secret string) error {
	query := `
		UPDATE users 
		SET two_factor_enabled = true, totp_secret = $2, updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.Exec(query, userID, secret, time.Now())
	return err
}

// ActivateBackupCodes moves temporary backup codes to permanent storage
func (r *AuthRepository) ActivateBackupCodes(userID uuid.UUID) error {
	// Move from temp to permanent
	query := `
		INSERT INTO user_backup_codes (id, user_id, code_hash, created_at)
		SELECT gen_random_uuid(), user_id, code_hash, NOW()
		FROM temp_backup_codes
		WHERE user_id = $1
	`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return err
	}

	// Clean temp
	query = `DELETE FROM temp_backup_codes WHERE user_id = $1`
	_, err = r.db.Exec(query, userID)
	return err
}

// ClearTempTOTPData clears temporary TOTP setup data
func (r *AuthRepository) ClearTempTOTPData(userID uuid.UUID) error {
	query := `DELETE FROM temp_totp_secrets WHERE user_id = $1`
	_, err := r.db.Exec(query, userID)
	return err
}

// LogTwoFactorAttempt logs a 2FA attempt
func (r *AuthRepository) LogTwoFactorAttempt(userID uuid.UUID, attemptType string, success bool, ipAddress, userAgent string) error {
	query := `
		INSERT INTO two_factor_recovery_logs (id, user_id, attempt_type, success, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(query, uuid.New(), userID, attemptType, success, ipAddress, userAgent, time.Now())
	return err
}

// GetUserBackupCodes gets user backup codes
func (r *AuthRepository) GetUserBackupCodes(userID uuid.UUID) ([]models.BackupCode, error) {
	query := `
		SELECT id, user_id, code_hash, used_at, created_at
		FROM user_backup_codes
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []models.BackupCode
	for rows.Next() {
		var code models.BackupCode
		var usedAt sql.NullTime

		err := rows.Scan(&code.ID, &code.UserID, &code.CodeHash, &usedAt, &code.CreatedAt)
		if err != nil {
			return nil, err
		}

		if usedAt.Valid {
			code.UsedAt = &usedAt.Time
		}

		codes = append(codes, code)
	}

	return codes, nil
}

// UseBackupCode marks a backup code as used
func (r *AuthRepository) UseBackupCode(codeID uuid.UUID) error {
	query := `UPDATE user_backup_codes SET used_at = $2 WHERE id = $1`
	_, err := r.db.Exec(query, codeID, time.Now())
	return err
}

// Mark2FAVerified marks a session as 2FA verified
func (r *AuthRepository) Mark2FAVerified(sessionID uuid.UUID) error {
	query := `
		UPDATE sessions 
		SET two_factor_verified = true, two_factor_verified_at = $2, updated_at = $3
		WHERE id = $1
	`
	now := time.Now()
	_, err := r.db.Exec(query, sessionID, now, now)
	return err
}

// UpdateLast2FAVerification updates the user's last 2FA verification time
func (r *AuthRepository) UpdateLast2FAVerification(userID uuid.UUID) error {
	query := `UPDATE users SET two_factor_verified_at = $2, updated_at = $3 WHERE id = $1`
	now := time.Now()
	_, err := r.db.Exec(query, userID, now, now)
	return err
}

// GetBackupCodesCount gets the count of remaining backup codes
func (r *AuthRepository) GetBackupCodesCount(userID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM user_backup_codes 
		WHERE user_id = $1 AND used_at IS NULL
	`
	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	return count, err
}

// Disable2FA disables 2FA for a user
func (r *AuthRepository) Disable2FA(userID uuid.UUID) error {
	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Disable 2FA
	query := `
		UPDATE users 
		SET two_factor_enabled = false, totp_secret = '', updated_at = $2
		WHERE id = $1
	`
	_, err = tx.Exec(query, userID, time.Now())
	if err != nil {
		return err
	}

	// Delete backup codes
	query = `DELETE FROM user_backup_codes WHERE user_id = $1`
	_, err = tx.Exec(query, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Get2FAStatus gets the 2FA status for a user
func (r *AuthRepository) Get2FAStatus(userID uuid.UUID) (map[string]interface{}, error) {
	var status map[string]interface{}

	// Get user 2FA info
	query := `
		SELECT two_factor_enabled, two_factor_verified_at, backup_codes_generated_at
		FROM users WHERE id = $1
	`
	var (
		enabled                bool
		verifiedAt             sql.NullTime
		backupCodesGeneratedAt sql.NullTime
	)

	err := r.db.QueryRow(query, userID).Scan(&enabled, &verifiedAt, &backupCodesGeneratedAt)
	if err != nil {
		return nil, err
	}

	// Get backup codes count
	backupCount, err := r.GetBackupCodesCount(userID)
	if err != nil {
		return nil, err
	}

	status = map[string]interface{}{
		"enabled":                enabled,
		"backup_codes_remaining": backupCount,
	}

	if verifiedAt.Valid {
		status["verified_at"] = verifiedAt.Time
	}
	if backupCodesGeneratedAt.Valid {
		status["backup_codes_generated_at"] = backupCodesGeneratedAt.Time
	}

	return status, nil
}

// ReplaceBackupCodes replaces backup codes for a user
func (r *AuthRepository) ReplaceBackupCodes(userID uuid.UUID, hashedCodes []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete old codes
	query := `DELETE FROM user_backup_codes WHERE user_id = $1`
	_, err = tx.Exec(query, userID)
	if err != nil {
		return err
	}

	// Insert new codes
	for _, code := range hashedCodes {
		query := `
			INSERT INTO user_backup_codes (id, user_id, code_hash, created_at)
			VALUES ($1, $2, $3, $4)
		`
		_, err = tx.Exec(query, uuid.New(), userID, code, time.Now())
		if err != nil {
			return err
		}
	}

	// Update user record
	query = `UPDATE users SET backup_codes_generated_at = $2, updated_at = $3 WHERE id = $1`
	now := time.Now()
	_, err = tx.Exec(query, userID, now, now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetRecoveryInfo gets recovery information for a user
func (r *AuthRepository) GetRecoveryInfo(userID uuid.UUID) (map[string]interface{}, error) {
	// Get recent recovery attempts
	query := `
		SELECT attempt_type, success, created_at
		FROM two_factor_recovery_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 10
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []map[string]interface{}
	for rows.Next() {
		var (
			attemptType string
			success     bool
			createdAt   time.Time
		)

		err := rows.Scan(&attemptType, &success, &createdAt)
		if err != nil {
			return nil, err
		}

		attempts = append(attempts, map[string]interface{}{
			"type":       attemptType,
			"success":    success,
			"created_at": createdAt,
		})
	}

	// Get backup codes status
	backupCount, _ := r.GetBackupCodesCount(userID)

	return map[string]interface{}{
		"recent_attempts":        attempts,
		"backup_codes_remaining": backupCount,
	}, nil
}

// Email Verification Methods

// SaveVerificationToken saves a verification token
func (r *AuthRepository) SaveVerificationToken(userID uuid.UUID, token, tokenType string, expiresAt time.Time) error {
	query := `
		INSERT INTO verification_tokens (id, user_id, token, token_type, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(query, uuid.New(), userID, token, tokenType, expiresAt, time.Now())
	return err
}

// VerifyToken verifies a token and returns the user ID
func (r *AuthRepository) VerifyToken(token, tokenType string) (uuid.UUID, error) {
	query := `
		SELECT user_id FROM verification_tokens
		WHERE token = $1 AND token_type = $2 AND expires_at > $3 AND used_at IS NULL
	`
	var userID uuid.UUID
	err := r.db.QueryRow(query, token, tokenType, time.Now()).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("invalid or expired token")
		}
		return uuid.Nil, err
	}
	return userID, nil
}

// MarkTokenAsUsed marks a token as used
func (r *AuthRepository) MarkTokenAsUsed(token string) error {
	query := `UPDATE verification_tokens SET used_at = $2 WHERE token = $1`
	_, err := r.db.Exec(query, token, time.Now())
	return err
}

// Tenant Membership Methods

// CheckUserTenantMembership checks if a user has access to a specific tenant
// Returns (hasMembership, membershipRole)
func (r *AuthRepository) CheckUserTenantMembership(userID, tenantID uuid.UUID) (bool, string) {
	query := `
		SELECT role FROM user_tenant_memberships
		WHERE user_id = $1 AND tenant_id = $2
		LIMIT 1
	`
	var role string
	err := r.db.QueryRow(query, userID, tenantID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ""
		}
		return false, ""
	}
	return true, role
}
