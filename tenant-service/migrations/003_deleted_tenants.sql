-- Migration: Add deleted_tenants audit table for tenant offboarding
-- This table archives tenant data before deletion for audit purposes

-- Create deleted_tenants table
CREATE TABLE IF NOT EXISTS deleted_tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    original_tenant_id UUID NOT NULL,
    slug VARCHAR(100) NOT NULL,
    business_name VARCHAR(255) NOT NULL,
    owner_user_id UUID NOT NULL,
    owner_email VARCHAR(255) NOT NULL,

    -- Archived tenant data (JSON snapshot of the full tenant record)
    tenant_data JSONB NOT NULL,

    -- Archived memberships data
    memberships_data JSONB DEFAULT '[]',

    -- Deletion metadata
    deleted_by_user_id UUID NOT NULL,
    deletion_reason TEXT,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Track what resources were cleaned up
    resources_cleaned JSONB DEFAULT '{}',
    cleanup_completed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for audit queries
CREATE INDEX IF NOT EXISTS idx_deleted_tenants_original_id ON deleted_tenants(original_tenant_id);
CREATE INDEX IF NOT EXISTS idx_deleted_tenants_slug ON deleted_tenants(slug);
CREATE INDEX IF NOT EXISTS idx_deleted_tenants_owner ON deleted_tenants(owner_email);
CREATE INDEX IF NOT EXISTS idx_deleted_tenants_date ON deleted_tenants(deleted_at);
CREATE INDEX IF NOT EXISTS idx_deleted_tenants_deleted_by ON deleted_tenants(deleted_by_user_id);

-- Add comment for documentation
COMMENT ON TABLE deleted_tenants IS 'Audit table for archived tenant data. Records are created before tenant deletion for compliance and recovery purposes.';
COMMENT ON COLUMN deleted_tenants.tenant_data IS 'Full JSON snapshot of the tenant record at time of deletion';
COMMENT ON COLUMN deleted_tenants.memberships_data IS 'JSON array of all user_tenant_memberships at time of deletion';
COMMENT ON COLUMN deleted_tenants.resources_cleaned IS 'JSON object tracking K8s resources cleaned up (certificates, virtualservices, etc)';
