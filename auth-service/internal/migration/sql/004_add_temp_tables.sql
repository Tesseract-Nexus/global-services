-- Add temporary tables for 2FA setup process

-- Temporary TOTP secrets (for 2FA setup flow)
CREATE TABLE IF NOT EXISTS temp_totp_secrets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Temporary backup codes (for 2FA setup flow)
CREATE TABLE IF NOT EXISTS temp_backup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_temp_totp_secrets_expires ON temp_totp_secrets(expires_at);
CREATE INDEX IF NOT EXISTS idx_temp_backup_codes_user_id ON temp_backup_codes(user_id);

-- Cleanup function for expired temp data
CREATE OR REPLACE FUNCTION cleanup_expired_temp_data()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    -- Delete expired TOTP secrets
    DELETE FROM temp_totp_secrets WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    -- Delete orphaned temp backup codes (older than 1 hour)
    DELETE FROM temp_backup_codes WHERE created_at < NOW() - INTERVAL '1 hour';
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;