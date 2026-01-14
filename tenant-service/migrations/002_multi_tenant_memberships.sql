-- Migration: 002_multi_tenant_memberships.sql
-- Description: Adds multi-tenant support with user-tenant memberships
-- This enables one user to own/manage multiple tenants with different roles

-- ============================================================================
-- STEP 1: Add slug column to tenants table
-- ============================================================================

-- Add slug column for URL-friendly tenant identification
-- Example: admin.tesserix.app/myapparels instead of UUID
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS slug VARCHAR(50);

-- Create unique index for slug (URL routing)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug) WHERE slug IS NOT NULL;

-- Add display_name for better UX (can be different from legal business name)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS display_name VARCHAR(255);

-- Add logo_url for tenant branding
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS logo_url TEXT;

-- Add primary_color and secondary_color for theming
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS primary_color VARCHAR(7) DEFAULT '#6366f1';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS secondary_color VARCHAR(7) DEFAULT '#8b5cf6';

-- Add timezone and currency defaults for the tenant
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS default_timezone VARCHAR(50) DEFAULT 'UTC';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS default_currency VARCHAR(3) DEFAULT 'USD';

-- Add owner_user_id to track the original owner (from auth-service)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS owner_user_id UUID;

-- Add pricing tier for subscription management
-- Tiers: free (default), starter, professional, enterprise
-- For now all tenants are on free tier until monetization is enabled
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS pricing_tier VARCHAR(50) DEFAULT 'free';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS pricing_tier_updated_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS billing_email VARCHAR(255);

-- Create index for pricing tier queries (useful for analytics and tier-based features)
CREATE INDEX IF NOT EXISTS idx_tenants_pricing_tier ON tenants(pricing_tier);

-- ============================================================================
-- STEP 2: Create reserved_slugs table for quick lookup
-- ============================================================================
-- This table stores reserved slugs that cannot be used by tenants
-- Managed via API for easy updates without code deployment

CREATE TABLE IF NOT EXISTS reserved_slugs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(50) NOT NULL UNIQUE,
    reason VARCHAR(255) NOT NULL,  -- Why it's reserved (system, brand, offensive, etc.)
    category VARCHAR(50) NOT NULL DEFAULT 'system',  -- system, brand, infrastructure, offensive
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),  -- Who added it (admin email or 'system')
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for fast lookup
CREATE INDEX IF NOT EXISTS idx_reserved_slugs_slug ON reserved_slugs(slug) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_reserved_slugs_category ON reserved_slugs(category);

-- Insert default reserved slugs
INSERT INTO reserved_slugs (slug, reason, category, created_by) VALUES
    -- System routes
    ('admin', 'System route - admin panel', 'system', 'system'),
    ('api', 'System route - API endpoint', 'system', 'system'),
    ('app', 'System route - application', 'system', 'system'),
    ('auth', 'System route - authentication', 'system', 'system'),
    ('login', 'System route - login page', 'system', 'system'),
    ('logout', 'System route - logout', 'system', 'system'),
    ('register', 'System route - registration', 'system', 'system'),
    ('signup', 'System route - sign up', 'system', 'system'),
    ('signin', 'System route - sign in', 'system', 'system'),
    ('dashboard', 'System route - dashboard', 'system', 'system'),
    ('settings', 'System route - settings', 'system', 'system'),
    ('profile', 'System route - user profile', 'system', 'system'),
    ('account', 'System route - account management', 'system', 'system'),
    ('billing', 'System route - billing', 'system', 'system'),
    ('support', 'System route - support', 'system', 'system'),
    ('help', 'System route - help center', 'system', 'system'),
    ('docs', 'System route - documentation', 'system', 'system'),
    ('status', 'System route - status page', 'system', 'system'),
    ('health', 'System route - health check', 'system', 'system'),
    ('metrics', 'System route - metrics endpoint', 'system', 'system'),
    ('webhooks', 'System route - webhooks', 'system', 'system'),
    ('oauth', 'System route - OAuth', 'system', 'system'),
    ('callback', 'System route - OAuth callback', 'system', 'system'),
    -- Brand protection
    ('tesserix', 'Brand protection - company name', 'brand', 'system'),
    ('tesseract', 'Brand protection - company name', 'brand', 'system'),
    ('tesserix-app', 'Brand protection - company name', 'brand', 'system'),
    -- Infrastructure
    ('www', 'Infrastructure - web server', 'infrastructure', 'system'),
    ('mail', 'Infrastructure - mail server', 'infrastructure', 'system'),
    ('email', 'Infrastructure - email', 'infrastructure', 'system'),
    ('smtp', 'Infrastructure - SMTP', 'infrastructure', 'system'),
    ('ftp', 'Infrastructure - FTP', 'infrastructure', 'system'),
    ('cdn', 'Infrastructure - CDN', 'infrastructure', 'system'),
    ('static', 'Infrastructure - static files', 'infrastructure', 'system'),
    ('assets', 'Infrastructure - assets', 'infrastructure', 'system'),
    ('images', 'Infrastructure - images', 'infrastructure', 'system'),
    ('files', 'Infrastructure - files', 'infrastructure', 'system'),
    ('uploads', 'Infrastructure - uploads', 'infrastructure', 'system'),
    ('media', 'Infrastructure - media files', 'infrastructure', 'system'),
    -- Testing/Development
    ('test', 'Reserved - testing', 'system', 'system'),
    ('demo', 'Reserved - demo accounts', 'system', 'system'),
    ('staging', 'Reserved - staging environment', 'system', 'system'),
    ('dev', 'Reserved - development', 'system', 'system'),
    ('sandbox', 'Reserved - sandbox testing', 'system', 'system')
ON CONFLICT (slug) DO NOTHING;

-- ============================================================================
-- STEP 2b: Create tenant_slug_reservations table for tracking claimed slugs
-- ============================================================================
-- This table tracks slugs that are:
-- 1. Currently being held during onboarding (temporary reservation)
-- 2. Already claimed by active tenants (permanent until tenant deleted)
-- Quick lookup table for real-time slug availability validation

CREATE TABLE IF NOT EXISTS tenant_slug_reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- The reserved slug (normalized, lowercase, hyphenated)
    slug VARCHAR(50) NOT NULL UNIQUE,

    -- Reservation status
    -- 'pending': Being held during onboarding (temporary, expires)
    -- 'active': Claimed by an active tenant (permanent)
    -- 'released': Was released/abandoned (soft delete for audit)
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    -- Link to onboarding session (for pending reservations)
    session_id UUID,

    -- Link to tenant (for active reservations, set when tenant is created)
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,

    -- Who reserved it (user email or session identifier)
    reserved_by VARCHAR(255),

    -- Expiration for pending reservations (e.g., 30 minutes)
    -- NULL for active (permanent) reservations
    expires_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    released_at TIMESTAMP WITH TIME ZONE  -- When it was released/abandoned
);

-- Index for fast slug lookup (primary use case)
CREATE UNIQUE INDEX IF NOT EXISTS idx_slug_reservations_slug
    ON tenant_slug_reservations(slug)
    WHERE status IN ('pending', 'active');

-- Index for finding expired pending reservations (cleanup job)
CREATE INDEX IF NOT EXISTS idx_slug_reservations_expires
    ON tenant_slug_reservations(expires_at)
    WHERE status = 'pending' AND expires_at IS NOT NULL;

-- Index for session lookup (to release on abandon)
CREATE INDEX IF NOT EXISTS idx_slug_reservations_session
    ON tenant_slug_reservations(session_id)
    WHERE session_id IS NOT NULL;

-- Index for tenant lookup
CREATE INDEX IF NOT EXISTS idx_slug_reservations_tenant
    ON tenant_slug_reservations(tenant_id)
    WHERE tenant_id IS NOT NULL;

-- Function to automatically release expired reservations
CREATE OR REPLACE FUNCTION cleanup_expired_slug_reservations()
RETURNS INTEGER AS $$
DECLARE
    released_count INTEGER;
BEGIN
    UPDATE tenant_slug_reservations
    SET status = 'released',
        released_at = NOW(),
        updated_at = NOW()
    WHERE status = 'pending'
      AND expires_at IS NOT NULL
      AND expires_at < NOW();

    GET DIAGNOSTICS released_count = ROW_COUNT;
    RETURN released_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 3: Create user_tenant_memberships table
-- ============================================================================

-- This table enables many-to-many relationship between users and tenants
-- A user can belong to multiple tenants with different roles
CREATE TABLE IF NOT EXISTS user_tenant_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- User ID from auth-service (external reference)
    user_id UUID NOT NULL,

    -- Tenant this membership belongs to
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Role within this tenant
    -- owner: Full control, can delete tenant, manage billing
    -- admin: Can manage products, orders, settings (not billing/delete)
    -- manager: Can manage products, orders (not settings)
    -- member: Read-only access with limited actions
    -- viewer: Read-only access
    role VARCHAR(50) NOT NULL DEFAULT 'member',

    -- Permissions as JSONB for fine-grained control
    -- Example: {"products": ["read", "write"], "orders": ["read"]}
    permissions JSONB DEFAULT '{}',

    -- Is this the user's default/primary tenant?
    -- When user logs in, they go to this tenant first
    is_default BOOLEAN DEFAULT false,

    -- Is this membership currently active?
    is_active BOOLEAN DEFAULT true,

    -- Invitation tracking
    invited_by UUID,                    -- User ID who sent the invitation
    invited_at TIMESTAMP WITH TIME ZONE,
    invitation_token VARCHAR(255),      -- Token for accepting invitation
    invitation_expires_at TIMESTAMP WITH TIME ZONE,
    accepted_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    last_accessed_at TIMESTAMP WITH TIME ZONE,  -- Last time user accessed this tenant
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Ensure unique membership per user-tenant pair
    CONSTRAINT uk_user_tenant UNIQUE(user_id, tenant_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_utm_user_id ON user_tenant_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_utm_tenant_id ON user_tenant_memberships(tenant_id);
CREATE INDEX IF NOT EXISTS idx_utm_user_active ON user_tenant_memberships(user_id, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_utm_user_default ON user_tenant_memberships(user_id, is_default) WHERE is_default = true;
CREATE INDEX IF NOT EXISTS idx_utm_invitation_token ON user_tenant_memberships(invitation_token) WHERE invitation_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_utm_role ON user_tenant_memberships(tenant_id, role);

-- ============================================================================
-- STEP 3: Create tenant_activity_log table for audit trail
-- ============================================================================

CREATE TABLE IF NOT EXISTS tenant_activity_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,              -- User who performed the action
    action VARCHAR(100) NOT NULL,       -- e.g., 'member.invited', 'settings.updated', 'product.created'
    resource_type VARCHAR(50),          -- e.g., 'product', 'order', 'settings'
    resource_id UUID,                   -- ID of the affected resource
    details JSONB DEFAULT '{}',         -- Additional context
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tal_tenant_id ON tenant_activity_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tal_user_id ON tenant_activity_log(user_id);
CREATE INDEX IF NOT EXISTS idx_tal_created_at ON tenant_activity_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tal_action ON tenant_activity_log(tenant_id, action);

-- ============================================================================
-- STEP 4: Create function to ensure only one default tenant per user
-- ============================================================================

CREATE OR REPLACE FUNCTION ensure_single_default_tenant()
RETURNS TRIGGER AS $$
BEGIN
    -- If setting this membership as default, unset others for this user
    IF NEW.is_default = true THEN
        UPDATE user_tenant_memberships
        SET is_default = false, updated_at = NOW()
        WHERE user_id = NEW.user_id
          AND id != NEW.id
          AND is_default = true;
    END IF;

    -- If this is the user's first tenant, make it default
    IF NEW.is_default IS NULL OR NEW.is_default = false THEN
        IF NOT EXISTS (
            SELECT 1 FROM user_tenant_memberships
            WHERE user_id = NEW.user_id
              AND is_default = true
              AND id != COALESCE(NEW.id, uuid_nil())
        ) THEN
            NEW.is_default := true;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for insert and update
DROP TRIGGER IF EXISTS trg_ensure_single_default_tenant ON user_tenant_memberships;
CREATE TRIGGER trg_ensure_single_default_tenant
    BEFORE INSERT OR UPDATE OF is_default ON user_tenant_memberships
    FOR EACH ROW
    EXECUTE FUNCTION ensure_single_default_tenant();

-- ============================================================================
-- STEP 5: Create function to auto-generate slug from tenant name
-- ============================================================================

CREATE OR REPLACE FUNCTION generate_tenant_slug(tenant_name TEXT)
RETURNS TEXT AS $$
DECLARE
    base_slug TEXT;
    final_slug TEXT;
    counter INTEGER := 0;
BEGIN
    -- Convert to lowercase, replace spaces and special chars with hyphens
    base_slug := lower(regexp_replace(tenant_name, '[^a-zA-Z0-9]+', '-', 'g'));
    -- Remove leading/trailing hyphens
    base_slug := trim(both '-' from base_slug);
    -- Limit length to 45 chars to allow for suffix
    base_slug := substring(base_slug from 1 for 45);

    final_slug := base_slug;

    -- Check for uniqueness and append number if needed
    WHILE EXISTS (SELECT 1 FROM tenants WHERE slug = final_slug) LOOP
        counter := counter + 1;
        final_slug := base_slug || '-' || counter;
    END LOOP;

    RETURN final_slug;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 6: Update existing tenants with slugs (if any exist)
-- ============================================================================

-- Generate slugs for existing tenants that don't have one
UPDATE tenants
SET slug = generate_tenant_slug(COALESCE(name, 'tenant-' || id::text)),
    display_name = COALESCE(display_name, name)
WHERE slug IS NULL;

-- Now make slug NOT NULL after populating existing records
ALTER TABLE tenants ALTER COLUMN slug SET NOT NULL;

-- ============================================================================
-- STEP 7: Create view for easy user-tenant queries
-- ============================================================================

CREATE OR REPLACE VIEW v_user_tenants AS
SELECT
    utm.user_id,
    utm.tenant_id,
    utm.role,
    utm.permissions,
    utm.is_default,
    utm.is_active,
    utm.last_accessed_at,
    t.name AS tenant_name,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name,
    t.logo_url AS tenant_logo_url,
    t.status AS tenant_status,
    t.mode AS tenant_mode,
    t.primary_color,
    t.secondary_color
FROM user_tenant_memberships utm
JOIN tenants t ON t.id = utm.tenant_id
WHERE utm.is_active = true AND t.status = 'active';

-- ============================================================================
-- STEP 8: Add comments for documentation
-- ============================================================================

COMMENT ON TABLE user_tenant_memberships IS 'Links users (from auth-service) to tenants with role-based access. Enables multi-tenant ownership.';
COMMENT ON COLUMN user_tenant_memberships.user_id IS 'User ID from auth-service (external reference, not FK)';
COMMENT ON COLUMN user_tenant_memberships.role IS 'Role: owner (full control), admin (manage), manager (limited manage), member (limited), viewer (read-only)';
COMMENT ON COLUMN user_tenant_memberships.permissions IS 'Fine-grained permissions as JSONB. Empty means use role defaults.';
COMMENT ON COLUMN user_tenant_memberships.is_default IS 'If true, this is the default tenant shown after login';

COMMENT ON COLUMN tenants.slug IS 'URL-friendly identifier for path-based routing. Example: admin.tesserix.app/myapparels';
COMMENT ON COLUMN tenants.owner_user_id IS 'Original owner user ID from auth-service. Used for billing and account ownership.';

COMMENT ON TABLE tenant_activity_log IS 'Audit trail for all tenant activities. Useful for security and compliance.';

COMMENT ON VIEW v_user_tenants IS 'Convenient view for querying user tenant access with tenant details';
