-- Migration: Add Vendor Isolation to Auth Service
-- Supports Tenant -> Vendor -> Staff hierarchy for marketplace data isolation

-- Add vendor_id column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Create indexes for vendor-based queries
CREATE INDEX IF NOT EXISTS idx_users_tenant_vendor ON users (tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_users_vendor ON users (vendor_id) WHERE vendor_id IS NOT NULL;

-- Comment explaining the hierarchy
COMMENT ON COLUMN users.vendor_id IS 'Vendor ID for marketplace isolation. Hierarchy: Tenant -> Vendor -> Staff/User';
