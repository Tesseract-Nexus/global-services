-- Migration: 005_tenant_credentials.sql
-- Description: Adds tenant-specific credential isolation for multi-tenant authentication
-- This enables the same email to have different passwords for different tenants
-- Enterprise requirement: Full credential isolation per tenant for security compliance

-- ============================================================================
-- STEP 1: Create tenant_credentials table
-- ============================================================================
-- This table stores tenant-specific passwords and authentication settings
-- Each user can have different credentials for each tenant they belong to

CREATE TABLE IF NOT EXISTS tenant_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- User reference (from tenant_users table)
    user_id UUID NOT NULL,

    -- Tenant this credential is for
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Tenant-specific password (bcrypt hash)
    -- Each tenant can have a different password for the same user
    password_hash VARCHAR(255) NOT NULL,

    -- Password metadata
    password_set_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    password_expires_at TIMESTAMP WITH TIME ZONE,
    password_rotation_required BOOLEAN DEFAULT false,
    last_password_change_at TIMESTAMP WITH TIME ZONE,

    -- MFA settings per tenant
    mfa_enabled BOOLEAN DEFAULT false,
    mfa_type VARCHAR(20), -- totp, sms, email, hardware_token
    mfa_secret VARCHAR(255), -- Encrypted TOTP secret or reference
    mfa_backup_codes JSONB DEFAULT '[]', -- Encrypted backup codes
    mfa_last_used_at TIMESTAMP WITH TIME ZONE,

    -- Login security
    login_attempts INTEGER DEFAULT 0,
    last_login_attempt_at TIMESTAMP WITH TIME ZONE,
    locked_until TIMESTAMP WITH TIME ZONE,
    last_successful_login_at TIMESTAMP WITH TIME ZONE,
    last_login_ip INET,
    last_login_user_agent TEXT,

    -- Session management
    active_sessions INTEGER DEFAULT 0,
    max_sessions INTEGER DEFAULT 5,

    -- Password history (to prevent reuse)
    password_history JSONB DEFAULT '[]', -- Array of previous password hashes

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID,
    updated_by UUID,

    -- Ensure unique credential per user-tenant pair
    CONSTRAINT uk_tenant_credentials_user_tenant UNIQUE(user_id, tenant_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tenant_cred_user_id ON tenant_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_tenant_cred_tenant_id ON tenant_credentials(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_cred_user_tenant ON tenant_credentials(user_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_cred_locked ON tenant_credentials(locked_until) WHERE locked_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tenant_cred_password_expires ON tenant_credentials(password_expires_at) WHERE password_expires_at IS NOT NULL;

-- ============================================================================
-- STEP 2: Create tenant_auth_policies table
-- ============================================================================
-- Each tenant can define their own authentication/security policies
-- This enables enterprise customers to enforce stricter security requirements

CREATE TABLE IF NOT EXISTS tenant_auth_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- One policy per tenant
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,

    -- Password policy
    password_min_length INTEGER DEFAULT 8,
    password_max_length INTEGER DEFAULT 128,
    password_require_uppercase BOOLEAN DEFAULT true,
    password_require_lowercase BOOLEAN DEFAULT true,
    password_require_numbers BOOLEAN DEFAULT true,
    password_require_special_chars BOOLEAN DEFAULT false,
    password_special_chars VARCHAR(100) DEFAULT '!@#$%^&*()_+-=[]{}|;:,.<>?',
    password_expiry_days INTEGER, -- NULL = no expiry
    password_history_count INTEGER DEFAULT 5, -- Prevent reuse of last N passwords

    -- Login policy
    max_login_attempts INTEGER DEFAULT 5,
    lockout_duration_minutes INTEGER DEFAULT 30,
    session_timeout_minutes INTEGER DEFAULT 480, -- 8 hours default
    max_concurrent_sessions INTEGER DEFAULT 5,

    -- MFA policy
    mfa_required BOOLEAN DEFAULT false,
    mfa_required_for_roles JSONB DEFAULT '["owner", "admin"]', -- Roles that must have MFA
    mfa_allowed_types JSONB DEFAULT '["totp", "email"]', -- Allowed MFA methods

    -- IP and device restrictions
    ip_whitelist_enabled BOOLEAN DEFAULT false,
    ip_whitelist JSONB DEFAULT '[]', -- Array of allowed IP ranges
    trusted_devices_enabled BOOLEAN DEFAULT false,
    require_device_verification BOOLEAN DEFAULT false,

    -- SSO/Federation settings (for enterprise)
    sso_enabled BOOLEAN DEFAULT false,
    sso_provider VARCHAR(50), -- okta, azure_ad, google, custom_saml
    sso_config JSONB DEFAULT '{}', -- Provider-specific configuration
    sso_required BOOLEAN DEFAULT false, -- If true, password login is disabled

    -- Advanced security
    require_email_verification BOOLEAN DEFAULT true,
    allow_password_reset BOOLEAN DEFAULT true,
    password_reset_token_expiry_hours INTEGER DEFAULT 24,
    notify_on_new_device_login BOOLEAN DEFAULT true,
    notify_on_password_change BOOLEAN DEFAULT true,

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_by UUID
);

-- Index for tenant lookup
CREATE INDEX IF NOT EXISTS idx_tenant_auth_policies_tenant ON tenant_auth_policies(tenant_id);

-- ============================================================================
-- STEP 3: Create tenant_auth_audit_log table
-- ============================================================================
-- Comprehensive audit logging for all authentication events per tenant

CREATE TABLE IF NOT EXISTS tenant_auth_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID, -- NULL for pre-auth events like failed logins by unknown users

    -- Event details
    event_type VARCHAR(50) NOT NULL, -- login_success, login_failed, password_changed, mfa_enabled, etc.
    event_status VARCHAR(20) NOT NULL DEFAULT 'success', -- success, failed, blocked

    -- Request context
    ip_address INET,
    user_agent TEXT,
    device_fingerprint VARCHAR(255),
    geo_location JSONB, -- {country, city, region}

    -- Event-specific details
    details JSONB DEFAULT '{}',
    error_message TEXT,

    -- Session info (for login events)
    session_id VARCHAR(255),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for audit queries
CREATE INDEX IF NOT EXISTS idx_auth_audit_tenant ON tenant_auth_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_user ON tenant_auth_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_event ON tenant_auth_audit_log(tenant_id, event_type);
CREATE INDEX IF NOT EXISTS idx_auth_audit_created ON tenant_auth_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_audit_ip ON tenant_auth_audit_log(ip_address);

-- Partition by month for large-scale deployments (optional, can be enabled later)
-- CREATE INDEX IF NOT EXISTS idx_auth_audit_created_month ON tenant_auth_audit_log(date_trunc('month', created_at));

-- ============================================================================
-- STEP 4: Create function to validate password against tenant policy
-- ============================================================================

CREATE OR REPLACE FUNCTION validate_password_policy(
    p_tenant_id UUID,
    p_password TEXT
) RETURNS TABLE(is_valid BOOLEAN, errors TEXT[]) AS $$
DECLARE
    policy tenant_auth_policies%ROWTYPE;
    error_list TEXT[] := '{}';
BEGIN
    -- Get tenant's auth policy (or use defaults)
    SELECT * INTO policy FROM tenant_auth_policies WHERE tenant_id = p_tenant_id;

    -- Use defaults if no policy exists
    IF policy.id IS NULL THEN
        policy.password_min_length := 8;
        policy.password_max_length := 128;
        policy.password_require_uppercase := true;
        policy.password_require_lowercase := true;
        policy.password_require_numbers := true;
        policy.password_require_special_chars := false;
    END IF;

    -- Validate length
    IF length(p_password) < policy.password_min_length THEN
        error_list := array_append(error_list,
            format('Password must be at least %s characters', policy.password_min_length));
    END IF;

    IF length(p_password) > policy.password_max_length THEN
        error_list := array_append(error_list,
            format('Password must be at most %s characters', policy.password_max_length));
    END IF;

    -- Validate uppercase
    IF policy.password_require_uppercase AND p_password !~ '[A-Z]' THEN
        error_list := array_append(error_list, 'Password must contain at least one uppercase letter');
    END IF;

    -- Validate lowercase
    IF policy.password_require_lowercase AND p_password !~ '[a-z]' THEN
        error_list := array_append(error_list, 'Password must contain at least one lowercase letter');
    END IF;

    -- Validate numbers
    IF policy.password_require_numbers AND p_password !~ '[0-9]' THEN
        error_list := array_append(error_list, 'Password must contain at least one number');
    END IF;

    -- Validate special characters
    IF policy.password_require_special_chars AND p_password !~ '[!@#$%^&*()_+\-=\[\]{}|;:,.<>?]' THEN
        error_list := array_append(error_list, 'Password must contain at least one special character');
    END IF;

    RETURN QUERY SELECT array_length(error_list, 1) IS NULL OR array_length(error_list, 1) = 0, error_list;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 5: Create function to check account lockout
-- ============================================================================

CREATE OR REPLACE FUNCTION check_account_lockout(
    p_user_id UUID,
    p_tenant_id UUID
) RETURNS TABLE(is_locked BOOLEAN, locked_until TIMESTAMP WITH TIME ZONE, remaining_attempts INTEGER) AS $$
DECLARE
    cred tenant_credentials%ROWTYPE;
    policy tenant_auth_policies%ROWTYPE;
    max_attempts INTEGER;
BEGIN
    SELECT * INTO cred FROM tenant_credentials WHERE user_id = p_user_id AND tenant_id = p_tenant_id;
    SELECT * INTO policy FROM tenant_auth_policies WHERE tenant_id = p_tenant_id;

    max_attempts := COALESCE(policy.max_login_attempts, 5);

    IF cred.id IS NULL THEN
        RETURN QUERY SELECT false, NULL::TIMESTAMP WITH TIME ZONE, max_attempts;
        RETURN;
    END IF;

    -- Check if currently locked
    IF cred.locked_until IS NOT NULL AND cred.locked_until > NOW() THEN
        RETURN QUERY SELECT true, cred.locked_until, 0;
        RETURN;
    END IF;

    -- Return current state
    RETURN QUERY SELECT false, NULL::TIMESTAMP WITH TIME ZONE, max_attempts - COALESCE(cred.login_attempts, 0);
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 6: Create function to record login attempt
-- ============================================================================

CREATE OR REPLACE FUNCTION record_login_attempt(
    p_user_id UUID,
    p_tenant_id UUID,
    p_success BOOLEAN,
    p_ip_address INET DEFAULT NULL,
    p_user_agent TEXT DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    policy tenant_auth_policies%ROWTYPE;
    max_attempts INTEGER;
    lockout_mins INTEGER;
BEGIN
    SELECT * INTO policy FROM tenant_auth_policies WHERE tenant_id = p_tenant_id;
    max_attempts := COALESCE(policy.max_login_attempts, 5);
    lockout_mins := COALESCE(policy.lockout_duration_minutes, 30);

    IF p_success THEN
        -- Reset on successful login
        UPDATE tenant_credentials
        SET login_attempts = 0,
            locked_until = NULL,
            last_successful_login_at = NOW(),
            last_login_ip = p_ip_address,
            last_login_user_agent = p_user_agent,
            updated_at = NOW()
        WHERE user_id = p_user_id AND tenant_id = p_tenant_id;
    ELSE
        -- Increment attempts on failure
        UPDATE tenant_credentials
        SET login_attempts = COALESCE(login_attempts, 0) + 1,
            last_login_attempt_at = NOW(),
            locked_until = CASE
                WHEN COALESCE(login_attempts, 0) + 1 >= max_attempts
                THEN NOW() + (lockout_mins || ' minutes')::INTERVAL
                ELSE locked_until
            END,
            updated_at = NOW()
        WHERE user_id = p_user_id AND tenant_id = p_tenant_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 7: Create default auth policies for existing tenants
-- ============================================================================

INSERT INTO tenant_auth_policies (tenant_id)
SELECT id FROM tenants
WHERE id NOT IN (SELECT tenant_id FROM tenant_auth_policies)
ON CONFLICT (tenant_id) DO NOTHING;

-- ============================================================================
-- STEP 8: Migrate existing credentials to tenant_credentials
-- ============================================================================
-- For existing users, create tenant-specific credentials using their global password
-- This ensures backward compatibility

INSERT INTO tenant_credentials (user_id, tenant_id, password_hash, password_set_at)
SELECT
    u.id as user_id,
    m.tenant_id,
    u.password as password_hash,
    u.created_at as password_set_at
FROM tenant_users u
JOIN user_tenant_memberships m ON m.user_id = u.id
WHERE u.password IS NOT NULL AND u.password != ''
ON CONFLICT (user_id, tenant_id) DO NOTHING;

-- ============================================================================
-- STEP 9: Add comments for documentation
-- ============================================================================

COMMENT ON TABLE tenant_credentials IS 'Stores tenant-specific credentials for multi-tenant credential isolation. Each user can have different passwords per tenant.';
COMMENT ON COLUMN tenant_credentials.user_id IS 'Reference to tenant_users.id - the global user account';
COMMENT ON COLUMN tenant_credentials.tenant_id IS 'The tenant this credential is for';
COMMENT ON COLUMN tenant_credentials.password_hash IS 'Bcrypt-hashed password specific to this user-tenant combination';
COMMENT ON COLUMN tenant_credentials.mfa_secret IS 'Encrypted TOTP secret for MFA (if enabled)';
COMMENT ON COLUMN tenant_credentials.password_history IS 'Array of previous password hashes to prevent reuse';

COMMENT ON TABLE tenant_auth_policies IS 'Per-tenant authentication and security policies for enterprise compliance';
COMMENT ON COLUMN tenant_auth_policies.sso_enabled IS 'If true, tenant uses external SSO provider';
COMMENT ON COLUMN tenant_auth_policies.sso_required IS 'If true, password login is disabled and SSO is mandatory';

COMMENT ON TABLE tenant_auth_audit_log IS 'Comprehensive audit trail of all authentication events per tenant';
COMMENT ON COLUMN tenant_auth_audit_log.event_type IS 'Type of auth event: login_success, login_failed, password_changed, mfa_enabled, session_revoked, etc.';

-- ============================================================================
-- STEP 10: Create trigger to auto-create auth policy for new tenants
-- ============================================================================

CREATE OR REPLACE FUNCTION auto_create_tenant_auth_policy()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO tenant_auth_policies (tenant_id) VALUES (NEW.id)
    ON CONFLICT (tenant_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_auto_create_auth_policy ON tenants;
CREATE TRIGGER trg_auto_create_auth_policy
    AFTER INSERT ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION auto_create_tenant_auth_policy();
