-- GeoTag API Schema: Rollback
-- Migration: 000004_geotag_schema.down.sql

SET search_path TO location, public;

-- Drop triggers first
DROP TRIGGER IF EXISTS places_search_vector_trigger ON places;
DROP TRIGGER IF EXISTS places_geohash_trigger ON places;
DROP TRIGGER IF EXISTS address_cache_updated_trigger ON address_cache;

-- Drop functions
DROP FUNCTION IF EXISTS places_search_vector_update();
DROP FUNCTION IF EXISTS places_geohash_update();
DROP FUNCTION IF EXISTS address_cache_updated_at();
DROP FUNCTION IF EXISTS generate_geohash(DECIMAL, DECIMAL, INT);

-- Drop indexes
DROP INDEX IF EXISTS idx_address_cache_lookup;
DROP INDEX IF EXISTS idx_address_cache_expires;
DROP INDEX IF EXISTS idx_address_cache_coords;
DROP INDEX IF EXISTS idx_address_cache_country;
DROP INDEX IF EXISTS idx_places_geohash;
DROP INDEX IF EXISTS idx_places_country;
DROP INDEX IF EXISTS idx_places_city;
DROP INDEX IF EXISTS idx_places_postal;
DROP INDEX IF EXISTS idx_places_external_id;
DROP INDEX IF EXISTS idx_places_deleted;
DROP INDEX IF EXISTS idx_places_search;

-- Drop tables
DROP TABLE IF EXISTS address_cache;
DROP TABLE IF EXISTS places;
