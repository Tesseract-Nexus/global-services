-- Migration: Create storefront_theme_settings table
-- Description: Stores vendor-specific storefront theme configuration

-- Create the storefront_theme_settings table
CREATE TABLE IF NOT EXISTS storefront_theme_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    theme_template VARCHAR(50) NOT NULL DEFAULT 'vibrant',
    primary_color VARCHAR(20) NOT NULL DEFAULT '#8B5CF6',
    secondary_color VARCHAR(20) NOT NULL DEFAULT '#EC4899',
    accent_color VARCHAR(20),
    logo_url TEXT,
    favicon_url TEXT,
    font_primary VARCHAR(100) DEFAULT 'Inter',
    font_secondary VARCHAR(100) DEFAULT 'system-ui',
    header_config JSONB DEFAULT '{}',
    homepage_config JSONB DEFAULT '{}',
    footer_config JSONB DEFAULT '{}',
    product_config JSONB DEFAULT '{}',
    checkout_config JSONB DEFAULT '{}',
    custom_css TEXT,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    CONSTRAINT storefront_theme_settings_tenant_unique UNIQUE (tenant_id)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_storefront_theme_tenant_id ON storefront_theme_settings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_storefront_theme_deleted_at ON storefront_theme_settings(deleted_at);
CREATE INDEX IF NOT EXISTS idx_storefront_theme_template ON storefront_theme_settings(theme_template);

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_storefront_theme_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_storefront_theme_updated_at ON storefront_theme_settings;
CREATE TRIGGER trigger_storefront_theme_updated_at
    BEFORE UPDATE ON storefront_theme_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_storefront_theme_updated_at();

-- Add comment to table
COMMENT ON TABLE storefront_theme_settings IS 'Stores vendor-specific storefront theme configuration including colors, fonts, and layout settings';

-- Add comments to columns
COMMENT ON COLUMN storefront_theme_settings.tenant_id IS 'The vendor/tenant ID that owns this storefront theme';
COMMENT ON COLUMN storefront_theme_settings.theme_template IS 'The theme template identifier (vibrant, minimal, dark, neon, ocean, sunset)';
COMMENT ON COLUMN storefront_theme_settings.primary_color IS 'Primary brand color in hex format';
COMMENT ON COLUMN storefront_theme_settings.secondary_color IS 'Secondary brand color in hex format';
COMMENT ON COLUMN storefront_theme_settings.accent_color IS 'Optional accent color in hex format';
COMMENT ON COLUMN storefront_theme_settings.header_config IS 'JSON configuration for header settings (announcement, nav links, etc.)';
COMMENT ON COLUMN storefront_theme_settings.homepage_config IS 'JSON configuration for homepage settings (hero, sections, etc.)';
COMMENT ON COLUMN storefront_theme_settings.footer_config IS 'JSON configuration for footer settings (links, social, contact, etc.)';
COMMENT ON COLUMN storefront_theme_settings.product_config IS 'JSON configuration for product display settings (grid, cards, etc.)';
COMMENT ON COLUMN storefront_theme_settings.checkout_config IS 'JSON configuration for checkout settings (guest checkout, fields, etc.)';
COMMENT ON COLUMN storefront_theme_settings.custom_css IS 'Custom CSS to be injected into the storefront';
