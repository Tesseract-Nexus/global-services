-- Migration: 004_add_slug_to_business_info.sql
-- Description: Adds tenant_slug and storefront_slug columns to business_information table
-- These columns store the reserved slugs during onboarding before tenant creation

-- ============================================================================
-- STEP 1: Add tenant_slug column to business_information
-- ============================================================================
-- This column stores the reserved admin URL slug during onboarding
-- Pattern: {tenant_slug}-admin.tesserix.app

ALTER TABLE business_information
ADD COLUMN IF NOT EXISTS tenant_slug VARCHAR(50);

-- Create index for faster lookups during validation
CREATE INDEX IF NOT EXISTS idx_business_info_tenant_slug
ON business_information(tenant_slug)
WHERE tenant_slug IS NOT NULL;

-- ============================================================================
-- STEP 2: Add storefront_slug column to business_information
-- ============================================================================
-- This column stores the storefront URL slug during onboarding
-- Pattern: {storefront_slug}.tesserix.app
-- If not explicitly set, defaults to the same as tenant_slug

ALTER TABLE business_information
ADD COLUMN IF NOT EXISTS storefront_slug VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_business_info_storefront_slug
ON business_information(storefront_slug)
WHERE storefront_slug IS NOT NULL;

-- ============================================================================
-- STEP 3: Add existing store fields if missing
-- ============================================================================
-- These fields may be used for migration tracking

ALTER TABLE business_information
ADD COLUMN IF NOT EXISTS existing_store_platforms TEXT[];

ALTER TABLE business_information
ADD COLUMN IF NOT EXISTS has_existing_store BOOLEAN DEFAULT false;

ALTER TABLE business_information
ADD COLUMN IF NOT EXISTS migration_interest BOOLEAN DEFAULT false;

-- ============================================================================
-- STEP 4: Add comments for documentation
-- ============================================================================

COMMENT ON COLUMN business_information.tenant_slug IS 'Reserved admin URL slug. Pattern: {slug}-admin.tesserix.app';
COMMENT ON COLUMN business_information.storefront_slug IS 'Reserved storefront URL slug. Pattern: {slug}.tesserix.app';
COMMENT ON COLUMN business_information.existing_store_platforms IS 'Array of existing e-commerce platforms the merchant uses (for migration)';
COMMENT ON COLUMN business_information.has_existing_store IS 'Whether the merchant has an existing online store';
COMMENT ON COLUMN business_information.migration_interest IS 'Whether the merchant is interested in migrating from another platform';
