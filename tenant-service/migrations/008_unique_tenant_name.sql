-- Migration: Add unique index on tenant name (case-insensitive)
-- This ensures business names are unique across tenants to prevent confusion
-- The uniqueness check is also enforced at the application layer during onboarding

-- Create unique index on lower(name) for case-insensitive uniqueness
-- This uses a functional index to handle case-insensitive comparison
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_name_unique_lower
ON tenants (LOWER(name))
WHERE status != 'deleted';

-- Add comment explaining the constraint
COMMENT ON INDEX idx_tenants_name_unique_lower IS 'Ensures business names are unique (case-insensitive) across active tenants';

-- Also add unique constraint on business_information for ongoing onboarding sessions
-- This prevents race conditions where two users try to register the same business name simultaneously
CREATE UNIQUE INDEX IF NOT EXISTS idx_business_info_name_unique_lower
ON business_information (LOWER(business_name));

-- Add comment explaining the constraint
COMMENT ON INDEX idx_business_info_name_unique_lower IS 'Prevents duplicate business names during concurrent onboarding sessions';
