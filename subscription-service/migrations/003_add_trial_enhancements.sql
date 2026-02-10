-- Migration 003: Add trial enhancements for subscription management
-- Adds expired/suspended status values, grace period fields, and plan region support

-- 1. Drop and recreate the status CHECK constraint to add new values
ALTER TABLE tenant_subscriptions DROP CONSTRAINT IF EXISTS check_status;
ALTER TABLE tenant_subscriptions ADD CONSTRAINT check_status
    CHECK (status IN ('active', 'trialing', 'past_due', 'canceled', 'unpaid', 'expired', 'suspended'));

-- 2. Add grace period and suspension tracking columns to tenant_subscriptions
ALTER TABLE tenant_subscriptions ADD COLUMN IF NOT EXISTS grace_period_end TIMESTAMP;
ALTER TABLE tenant_subscriptions ADD COLUMN IF NOT EXISTS payment_failed_at TIMESTAMP;
ALTER TABLE tenant_subscriptions ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMP;

-- 3. Add region and default trial flag to subscription_plans
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS region VARCHAR(10) DEFAULT 'global';
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS is_default_trial BOOLEAN DEFAULT false;

-- 4. Add indexes for trial/grace period expiration queries
CREATE INDEX IF NOT EXISTS idx_tenant_subscriptions_trial_end ON tenant_subscriptions(trial_end);
CREATE INDEX IF NOT EXISTS idx_tenant_subscriptions_grace_end ON tenant_subscriptions(grace_period_end);
