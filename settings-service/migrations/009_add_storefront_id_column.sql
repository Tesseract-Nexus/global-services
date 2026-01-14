-- Migration: Add storefront_id to storefront_theme_settings
-- Description: Backfill storefront_id with tenant_id and add unique index

ALTER TABLE storefront_theme_settings
  ADD COLUMN IF NOT EXISTS storefront_id UUID;

UPDATE storefront_theme_settings
SET storefront_id = tenant_id
WHERE storefront_id IS NULL;

ALTER TABLE storefront_theme_settings
  ALTER COLUMN storefront_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_storefront_theme_storefront_id
  ON storefront_theme_settings(storefront_id);

COMMENT ON COLUMN storefront_theme_settings.storefront_id
  IS 'Storefront ID for multi-storefront theme settings';
