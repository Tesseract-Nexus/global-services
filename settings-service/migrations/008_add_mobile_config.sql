-- Migration: Add mobile configuration column
-- Description: Adds mobile_config JSONB column for mobile-specific settings

-- Add new JSONB column for mobile configuration
ALTER TABLE storefront_theme_settings
ADD COLUMN IF NOT EXISTS mobile_config JSONB DEFAULT '{}';

-- Add comment for documentation
COMMENT ON COLUMN storefront_theme_settings.mobile_config IS 'Mobile settings: navigation style, bottom nav, sticky add to cart, touch settings';
