-- Migration: Add phone_country_code column to contact_information table
-- This column stores the ISO country code (e.g., IN, US, GB) for phone numbers
-- The Go model already expects this field but it was missing from the schema

-- Add phone_country_code column if it doesn't exist
ALTER TABLE contact_information
ADD COLUMN IF NOT EXISTS phone_country_code VARCHAR(10) DEFAULT '';

-- Add comment explaining the column
COMMENT ON COLUMN contact_information.phone_country_code IS 'ISO 3166-1 alpha-2 country code for the phone number (e.g., IN, US, GB)';
