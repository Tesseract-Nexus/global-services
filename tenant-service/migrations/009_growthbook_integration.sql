-- Migration: 009_growthbook_integration.sql
-- Description: Add GrowthBook feature flags integration fields to tenants
-- Each tenant gets their own GrowthBook organization for isolated feature flag management

-- Add GrowthBook integration fields to tenants table
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS growthbook_org_id VARCHAR(255);
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS growthbook_sdk_key VARCHAR(255);
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS growthbook_admin_key VARCHAR(255);
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS growthbook_enabled BOOLEAN DEFAULT false;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS growthbook_provisioned_at TIMESTAMP WITH TIME ZONE;

-- Add comments for documentation
COMMENT ON COLUMN tenants.growthbook_org_id IS 'GrowthBook organization ID for this tenant';
COMMENT ON COLUMN tenants.growthbook_sdk_key IS 'GrowthBook SDK client key for frontend feature flag evaluation';
COMMENT ON COLUMN tenants.growthbook_admin_key IS 'GrowthBook admin API key for backend feature flag management (encrypted)';
COMMENT ON COLUMN tenants.growthbook_enabled IS 'Whether GrowthBook feature flags are enabled for this tenant';
COMMENT ON COLUMN tenants.growthbook_provisioned_at IS 'When GrowthBook organization was provisioned for this tenant';

-- Create index for queries filtering by GrowthBook status
CREATE INDEX IF NOT EXISTS idx_tenants_growthbook_enabled ON tenants(growthbook_enabled) WHERE growthbook_enabled = true;

-- Create index for lookups by GrowthBook org ID
CREATE INDEX IF NOT EXISTS idx_tenants_growthbook_org_id ON tenants(growthbook_org_id) WHERE growthbook_org_id IS NOT NULL;
