-- Migration: Add keycloak_internal_id to staff table
-- Purpose: Track staff user IDs in the internal Keycloak realm
-- Required for dual-realm architecture where staff authenticate via tesseract-internal realm

-- Add keycloak_internal_id column to staff table
-- This column stores the Keycloak user ID in the internal realm
-- Staff members will have IDs in both realms during migration
ALTER TABLE staff
ADD COLUMN IF NOT EXISTS keycloak_internal_id UUID;

-- Create index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_staff_keycloak_internal_id
ON staff(keycloak_internal_id)
WHERE keycloak_internal_id IS NOT NULL;

-- Add comment for documentation
COMMENT ON COLUMN staff.keycloak_internal_id IS
'Keycloak user ID in the tesseract-internal realm. Used for admin/staff authentication.';

-- Also ensure keycloak_org_id exists on tenants (may already exist from previous migration)
ALTER TABLE tenants
ADD COLUMN IF NOT EXISTS keycloak_org_id UUID;

COMMENT ON COLUMN tenants.keycloak_org_id IS
'Keycloak Organization ID in the customer realm. Used for customer identity isolation per tenant.';
