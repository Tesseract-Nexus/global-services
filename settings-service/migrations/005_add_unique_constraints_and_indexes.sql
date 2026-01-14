-- Migration: Add unique constraints and indexes for production-grade data integrity
-- Date: 2025-12-18
-- Description: Ensures settings are unique per storefront (tenant) context and adds performance indexes

-- ==========================================
-- SETTINGS TABLE
-- ==========================================

-- Drop existing indexes if they exist (to recreate with better naming)
DROP INDEX IF EXISTS idx_settings_tenant;
DROP INDEX IF EXISTS idx_settings_app;
DROP INDEX IF EXISTS idx_settings_user;
DROP INDEX IF EXISTS idx_settings_scope;
DROP INDEX IF EXISTS idx_settings_unique_context;

-- Add individual indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_settings_tenant_id ON settings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_settings_application_id ON settings(application_id);
CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_settings_scope ON settings(scope);

-- Composite index for context queries (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_settings_context ON settings(tenant_id, application_id, scope);

-- Unique constraint for settings without user_id (application-level settings per tenant)
-- This ensures only ONE settings record per (tenant, application, scope) when user_id is NULL
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_unique_tenant_app_scope
ON settings(tenant_id, application_id, scope)
WHERE user_id IS NULL AND deleted_at IS NULL;

-- Unique constraint for user-specific settings
-- This ensures only ONE settings record per (tenant, application, scope, user) combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_unique_tenant_app_scope_user
ON settings(tenant_id, application_id, scope, user_id)
WHERE user_id IS NOT NULL AND deleted_at IS NULL;

-- ==========================================
-- SETTINGS_PRESETS TABLE
-- ==========================================

-- Drop existing indexes if they exist
DROP INDEX IF EXISTS idx_preset_name_category;
DROP INDEX IF EXISTS idx_preset_category;
DROP INDEX IF EXISTS idx_preset_default;

-- Unique constraint on (name, category) - prevent duplicate preset names within a category
CREATE UNIQUE INDEX IF NOT EXISTS idx_presets_unique_name_category
ON settings_presets(name, category)
WHERE deleted_at IS NULL;

-- Index for category filtering
CREATE INDEX IF NOT EXISTS idx_presets_category ON settings_presets(category);

-- Index for default preset lookup
CREATE INDEX IF NOT EXISTS idx_presets_is_default ON settings_presets(is_default) WHERE is_default = true;

-- Index for featured presets
CREATE INDEX IF NOT EXISTS idx_presets_featured ON settings_presets(featured) WHERE featured = true;

-- ==========================================
-- SETTINGS_HISTORY TABLE
-- ==========================================

-- Drop existing indexes if they exist
DROP INDEX IF EXISTS idx_history_settings;
DROP INDEX IF EXISTS idx_history_operation;
DROP INDEX IF EXISTS idx_history_user;
DROP INDEX IF EXISTS idx_history_created;

-- Index for fetching history by settings ID (most common query)
CREATE INDEX IF NOT EXISTS idx_history_settings_id ON settings_history(settings_id);

-- Composite index for audit queries (settings + time range)
CREATE INDEX IF NOT EXISTS idx_history_settings_created ON settings_history(settings_id, created_at DESC);

-- Index for filtering by operation type
CREATE INDEX IF NOT EXISTS idx_history_operation ON settings_history(operation);

-- Index for user audit trail
CREATE INDEX IF NOT EXISTS idx_history_user_id ON settings_history(user_id) WHERE user_id IS NOT NULL;

-- ==========================================
-- SETTINGS_VALIDATIONS TABLE
-- ==========================================

-- Drop existing indexes if they exist
DROP INDEX IF EXISTS idx_validation_field_rule;
DROP INDEX IF EXISTS idx_validation_severity;

-- Unique constraint on (field, rule) - prevent duplicate validation rules
CREATE UNIQUE INDEX IF NOT EXISTS idx_validations_unique_field_rule
ON settings_validations(field, rule);

-- Index for filtering by severity
CREATE INDEX IF NOT EXISTS idx_validations_severity ON settings_validations(severity);

-- ==========================================
-- STOREFRONT_THEME_SETTINGS TABLE
-- ==========================================

-- Ensure unique index on tenant_id exists (already defined in model but ensuring it's there)
-- This prevents duplicate theme settings per storefront
CREATE UNIQUE INDEX IF NOT EXISTS idx_storefront_theme_unique_tenant
ON storefront_theme_settings(tenant_id)
WHERE deleted_at IS NULL;

-- Index for theme template filtering
CREATE INDEX IF NOT EXISTS idx_storefront_theme_template ON storefront_theme_settings(theme_template);

-- ==========================================
-- DATA INTEGRITY CHECKS
-- ==========================================

-- Function to check for duplicate settings before the unique index is applied
-- Run this SELECT to identify any duplicates that need to be resolved:
-- SELECT tenant_id, application_id, scope, user_id, COUNT(*)
-- FROM settings
-- WHERE deleted_at IS NULL
-- GROUP BY tenant_id, application_id, scope, user_id
-- HAVING COUNT(*) > 1;

-- Comment: If duplicates exist, you can resolve by:
-- 1. Identifying the most recent/correct record
-- 2. Soft-deleting or hard-deleting the duplicates
-- 3. Then applying this migration

-- ==========================================
-- VERIFICATION QUERIES
-- ==========================================

-- Run these to verify the indexes were created successfully:
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'settings';
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'settings_presets';
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'settings_history';
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'settings_validations';
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'storefront_theme_settings';
