-- Migration: Add storefront theme history table for version tracking
-- Version: 007
-- Description: Tracks historical versions of theme settings for rollback capability

-- Create the theme history table
CREATE TABLE IF NOT EXISTS storefront_theme_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    theme_settings_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    version INT NOT NULL,
    snapshot JSONB NOT NULL,
    change_summary TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Foreign key constraint
    CONSTRAINT fk_storefront_theme_history_settings
        FOREIGN KEY (theme_settings_id)
        REFERENCES storefront_theme_settings(id)
        ON DELETE CASCADE
);

-- Create indexes for efficient querying
CREATE INDEX idx_theme_history_settings_id ON storefront_theme_history(theme_settings_id);
CREATE INDEX idx_theme_history_tenant_id ON storefront_theme_history(tenant_id);
CREATE INDEX idx_theme_history_version ON storefront_theme_history(version DESC);
CREATE INDEX idx_theme_history_created_at ON storefront_theme_history(created_at DESC);

-- Unique constraint: each version number should be unique per theme settings
CREATE UNIQUE INDEX idx_theme_history_settings_version
    ON storefront_theme_history(theme_settings_id, version);

-- Add mobile_config column to storefront_theme_settings if not exists
ALTER TABLE storefront_theme_settings
ADD COLUMN IF NOT EXISTS mobile_config JSONB DEFAULT '{}';

-- Add comment to table
COMMENT ON TABLE storefront_theme_history IS 'Stores historical versions of storefront theme settings for rollback functionality';
COMMENT ON COLUMN storefront_theme_history.version IS 'Sequential version number, increments with each save';
COMMENT ON COLUMN storefront_theme_history.snapshot IS 'Complete JSON snapshot of all theme settings at this version';
COMMENT ON COLUMN storefront_theme_history.change_summary IS 'Optional human-readable summary of changes made';
