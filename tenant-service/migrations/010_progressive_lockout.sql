-- Migration: 010_progressive_lockout.sql
-- Description: Add strict 2-tier progressive lockout for enterprise security
-- Date: 2026-01-20
--
-- Lockout Policy:
-- - Tier 1 (5 attempts): 30 minutes temporary lockout
-- - Tier 2 (7 attempts): Permanent lockout - requires admin unlock or password reset

-- ============================================================================
-- Add progressive lockout fields to tenant_credentials table
-- ============================================================================

-- Add lockout tracking columns
ALTER TABLE tenant_credentials
ADD COLUMN IF NOT EXISTS lockout_count INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS current_tier INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS permanently_locked BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS permanent_locked_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS unlocked_by UUID REFERENCES tenant_users(id),
ADD COLUMN IF NOT EXISTS unlocked_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS total_failed_attempts INTEGER DEFAULT 0;

-- Add index for permanent lockout queries (for admin dashboard)
CREATE INDEX IF NOT EXISTS idx_tenant_cred_permanently_locked
ON tenant_credentials(permanently_locked, tenant_id)
WHERE permanently_locked = TRUE;

-- Add comment for documentation
COMMENT ON COLUMN tenant_credentials.lockout_count IS 'Number of times account has been locked';
COMMENT ON COLUMN tenant_credentials.current_tier IS 'Current lockout tier (1=temporary, 2=permanent)';
COMMENT ON COLUMN tenant_credentials.permanently_locked IS 'True if account requires admin unlock or password reset';
COMMENT ON COLUMN tenant_credentials.permanent_locked_at IS 'Timestamp when permanently locked';
COMMENT ON COLUMN tenant_credentials.unlocked_by IS 'Admin user ID who unlocked the account';
COMMENT ON COLUMN tenant_credentials.unlocked_at IS 'Timestamp when admin unlocked the account';
COMMENT ON COLUMN tenant_credentials.total_failed_attempts IS 'Cumulative failed attempts for permanent lockout calculation';

-- ============================================================================
-- Add progressive lockout policy fields to tenant_auth_policies table
-- ============================================================================

ALTER TABLE tenant_auth_policies
ADD COLUMN IF NOT EXISTS enable_progressive_lockout BOOLEAN DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS tier1_lockout_minutes INTEGER DEFAULT 30,
ADD COLUMN IF NOT EXISTS permanent_lockout_threshold INTEGER DEFAULT 7,
ADD COLUMN IF NOT EXISTS lockout_reset_hours INTEGER DEFAULT 24;

-- Add comments for documentation
COMMENT ON COLUMN tenant_auth_policies.enable_progressive_lockout IS 'Enable strict 2-tier lockout (temporary then permanent)';
COMMENT ON COLUMN tenant_auth_policies.tier1_lockout_minutes IS 'Lockout duration for tier 1 (default: 30 min after 5 failed attempts)';
COMMENT ON COLUMN tenant_auth_policies.permanent_lockout_threshold IS 'Total failed attempts before permanent lockout (default: 7)';
COMMENT ON COLUMN tenant_auth_policies.lockout_reset_hours IS 'Hours after which lockout tracking resets (default: 24)';

-- ============================================================================
-- Rollback instructions (manual)
-- ============================================================================
-- To rollback this migration, run:
--
-- ALTER TABLE tenant_credentials
-- DROP COLUMN IF EXISTS lockout_count,
-- DROP COLUMN IF EXISTS current_tier,
-- DROP COLUMN IF EXISTS permanently_locked,
-- DROP COLUMN IF EXISTS permanent_locked_at,
-- DROP COLUMN IF EXISTS unlocked_by,
-- DROP COLUMN IF EXISTS unlocked_at,
-- DROP COLUMN IF EXISTS total_failed_attempts;
--
-- DROP INDEX IF EXISTS idx_tenant_cred_permanently_locked;
--
-- ALTER TABLE tenant_auth_policies
-- DROP COLUMN IF EXISTS enable_progressive_lockout,
-- DROP COLUMN IF EXISTS tier1_lockout_minutes,
-- DROP COLUMN IF EXISTS permanent_lockout_threshold,
-- DROP COLUMN IF EXISTS lockout_reset_hours;
