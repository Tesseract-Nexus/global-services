-- Migration: Add enhanced theme configuration columns
-- Description: Adds typography_config, layout_config, spacing_style_config, and advanced_config columns

-- Add new JSONB columns for enhanced configurations
ALTER TABLE storefront_theme_settings
ADD COLUMN IF NOT EXISTS typography_config JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS layout_config JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS spacing_style_config JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS advanced_config JSONB DEFAULT '{}';

-- Add comments for documentation
COMMENT ON COLUMN storefront_theme_settings.typography_config IS 'Typography settings: fonts, sizes, weights, line heights';
COMMENT ON COLUMN storefront_theme_settings.layout_config IS 'Layout settings: container width, homepage layout, header/footer layout';
COMMENT ON COLUMN storefront_theme_settings.spacing_style_config IS 'Spacing and style settings: border radius, shadows, animations';
COMMENT ON COLUMN storefront_theme_settings.advanced_config IS 'Advanced settings: custom CSS, visibility toggles, mobile settings';
