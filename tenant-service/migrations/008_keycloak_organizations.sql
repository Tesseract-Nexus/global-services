-- Migration: Add Keycloak organization ID to tenants table
-- Description: Support for Keycloak Organizations multi-tenant identity isolation
-- Author: Platform Team
-- Date: 2026-01-19

-- =============================================================================
-- UP MIGRATION
-- =============================================================================

-- Add keycloak_org_id column to tenants table
-- This stores the Keycloak Organization ID for multi-tenant identity isolation
-- Each tenant maps to exactly one Keycloak Organization
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS keycloak_org_id UUID;

-- Create index for efficient organization lookups
-- Used when resolving tenant from Keycloak org context
CREATE INDEX IF NOT EXISTS idx_tenants_keycloak_org_id ON tenants(keycloak_org_id);

-- Add comment for documentation
COMMENT ON COLUMN tenants.keycloak_org_id IS 'Keycloak Organization ID for multi-tenant identity isolation. Maps 1:1 with tenant.';

-- =============================================================================
-- ROLLBACK (manual execution if needed)
-- =============================================================================
-- DROP INDEX IF EXISTS idx_tenants_keycloak_org_id;
-- ALTER TABLE tenants DROP COLUMN IF EXISTS keycloak_org_id;
