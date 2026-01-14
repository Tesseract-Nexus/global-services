-- Add 2FA (Two-Factor Authentication) support
-- This migration adds TOTP and backup codes functionality

-- 1. Add 2FA fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_verified_at TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS backup_codes_generated_at TIMESTAMP;

-- 2. Create table for backup codes
CREATE TABLE IF NOT EXISTS user_backup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL, -- bcrypt hash of the backup code
    used_at TIMESTAMP, -- NULL if not used, timestamp if used
    created_at TIMESTAMP DEFAULT NOW()
);

-- 3. Create table for 2FA recovery attempts (security logging)
CREATE TABLE IF NOT EXISTS two_factor_recovery_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    attempt_type VARCHAR(50) NOT NULL, -- 'totp_code', 'backup_code', 'recovery_email'
    success BOOLEAN NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 4. Add 2FA session tracking
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS two_factor_verified BOOLEAN DEFAULT false;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS two_factor_verified_at TIMESTAMP;

-- 5. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_two_factor_enabled ON users(two_factor_enabled);
CREATE INDEX IF NOT EXISTS idx_users_totp_secret ON users(totp_secret) WHERE totp_secret IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_backup_codes_user_id ON user_backup_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_backup_codes_used ON user_backup_codes(used_at) WHERE used_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_recovery_logs_user_id ON two_factor_recovery_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_recovery_logs_created_at ON two_factor_recovery_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_two_factor ON sessions(two_factor_verified);

-- 6. Add constraint to ensure backup codes are unique per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_backup_codes_unique_per_user 
ON user_backup_codes(user_id, code_hash) WHERE used_at IS NULL;

-- 7. Update existing permissions for 2FA management
INSERT INTO permissions (name, resource, action, description, is_system) VALUES
    ('user.2fa:enable', 'user', 'enable_2fa', 'Enable two-factor authentication', true),
    ('user.2fa:disable', 'user', 'disable_2fa', 'Disable two-factor authentication', true),
    ('user.2fa:generate_backup', 'user', 'generate_backup', 'Generate backup codes', true),
    ('user.2fa:view_backup', 'user', 'view_backup', 'View backup codes', true),
    ('admin.2fa:force_disable', 'user', 'force_disable_2fa', 'Force disable user 2FA (admin)', true),
    ('admin.2fa:view_status', 'user', 'view_2fa_status', 'View user 2FA status (admin)', true)
ON CONFLICT (name) DO NOTHING;

-- 8. Assign 2FA permissions to existing roles
-- Regular users can manage their own 2FA
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name IN ('customer', 'vip_customer', 'vendor', 'staff', 'store_staff', 'store_manager') 
AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'user.2fa:enable', 'user.2fa:disable', 
    'user.2fa:generate_backup', 'user.2fa:view_backup'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Admins can manage any user's 2FA
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'tenant_admin', 'store_owner', 'marketplace_admin') 
AND r.tenant_id = 'default-tenant'
AND p.name LIKE '%2fa%'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- 9. Create function to clean up old backup codes (optional utility)
CREATE OR REPLACE FUNCTION cleanup_old_backup_codes()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    -- Delete backup codes older than 1 year that haven't been used
    DELETE FROM user_backup_codes 
    WHERE created_at < NOW() - INTERVAL '1 year' 
    AND used_at IS NULL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- 10. Create function to get user 2FA status
CREATE OR REPLACE FUNCTION get_user_2fa_status(user_uuid UUID)
RETURNS TABLE (
    two_factor_enabled BOOLEAN,
    backup_codes_count INTEGER,
    unused_backup_codes INTEGER,
    last_verified TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        u.two_factor_enabled,
        COUNT(bc.id)::INTEGER as backup_codes_count,
        COUNT(CASE WHEN bc.used_at IS NULL THEN 1 END)::INTEGER as unused_backup_codes,
        u.two_factor_verified_at as last_verified
    FROM users u
    LEFT JOIN user_backup_codes bc ON bc.user_id = u.id
    WHERE u.id = user_uuid
    GROUP BY u.id, u.two_factor_enabled, u.two_factor_verified_at;
END;
$$ LANGUAGE plpgsql;

-- 11. Add comments for documentation
COMMENT ON COLUMN users.two_factor_enabled IS 'Whether 2FA is enabled for this user';
COMMENT ON COLUMN users.totp_secret IS 'Base32-encoded TOTP secret key for authenticator apps';
COMMENT ON COLUMN users.two_factor_verified_at IS 'Last time user successfully verified with 2FA';
COMMENT ON TABLE user_backup_codes IS 'Backup codes for 2FA recovery (hashed)';
COMMENT ON TABLE two_factor_recovery_logs IS 'Audit log for 2FA recovery attempts';
COMMENT ON COLUMN sessions.two_factor_verified IS 'Whether this session has completed 2FA challenge';

-- 12. Insert sample data for testing (optional - remove in production)
-- This creates a test user with 2FA setup for development purposes
DO $$
DECLARE
    test_user_id UUID;
BEGIN
    -- Only insert if we're in a development environment
    IF current_setting('server_version_num')::integer >= 130000 THEN
        -- Get or create a test user
        INSERT INTO users (email, name, password_hash, email_verified, tenant_id, is_active)
        VALUES ('test-2fa@tesseract-hub.com', 'Test 2FA User', '$2a$12$test.hash.for.development', true, 'default-tenant', true)
        ON CONFLICT (email, store_id) DO NOTHING
        RETURNING id INTO test_user_id;
        
        -- If user was created, add some test backup codes
        IF test_user_id IS NOT NULL THEN
            INSERT INTO user_backup_codes (user_id, code_hash) VALUES
            (test_user_id, '$2a$12$test.backup.code.hash.1'),
            (test_user_id, '$2a$12$test.backup.code.hash.2');
        END IF;
    END IF;
END $$;