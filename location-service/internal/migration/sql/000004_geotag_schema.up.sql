-- GeoTag API Schema: Address Caching & Places Database
-- Migration: 000004_geotag_schema.up.sql

SET search_path TO location, public;

-- ============================================================================
-- Table: address_cache
-- Unified cache for all address lookup operations (geocode, reverse, autocomplete)
-- ============================================================================
CREATE TABLE IF NOT EXISTS address_cache (
    id BIGSERIAL PRIMARY KEY,

    -- Cache identification
    cache_type VARCHAR(20) NOT NULL,      -- 'geocode', 'reverse', 'autocomplete', 'place_details'
    cache_key_hash VARCHAR(64) NOT NULL,  -- SHA256 hash for fast indexed lookup
    cache_key TEXT NOT NULL,              -- Original key for debugging/inspection

    -- Result data (common fields)
    formatted_address TEXT,
    place_id VARCHAR(500),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),

    -- Normalized address components
    street_number VARCHAR(50),
    street_name VARCHAR(255),
    city VARCHAR(255),
    district VARCHAR(255),
    state_code VARCHAR(10),
    state_name VARCHAR(255),
    country_code VARCHAR(2),
    country_name VARCHAR(100),
    postal_code VARCHAR(20),

    -- Full JSON response (for autocomplete suggestions array)
    response_json JSONB,

    -- Metadata
    provider VARCHAR(50) NOT NULL,        -- Which provider returned this result
    hit_count INT DEFAULT 0,              -- Usage tracking
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Unique constraint on cache type + key hash
    CONSTRAINT address_cache_unique_key UNIQUE (cache_type, cache_key_hash)
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_address_cache_lookup
    ON address_cache(cache_type, cache_key_hash);

CREATE INDEX IF NOT EXISTS idx_address_cache_expires
    ON address_cache(expires_at);

CREATE INDEX IF NOT EXISTS idx_address_cache_coords
    ON address_cache(latitude, longitude)
    WHERE latitude IS NOT NULL AND longitude IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_address_cache_country
    ON address_cache(country_code)
    WHERE country_code IS NOT NULL;

-- ============================================================================
-- Table: places
-- Permanent storage for geocoded places (our own geotag database)
-- Never expires - builds up over time for internal lookups
-- ============================================================================
CREATE TABLE IF NOT EXISTS places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- External identifiers
    external_place_id VARCHAR(500),       -- Original provider's place ID

    -- Location data
    formatted_address TEXT NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    geohash VARCHAR(12),                  -- For efficient nearby queries

    -- Normalized address components
    street_number VARCHAR(50),
    street_name VARCHAR(255),
    city VARCHAR(255),
    district VARCHAR(255),
    state_code VARCHAR(10),
    state_name VARCHAR(255),
    country_code VARCHAR(2),
    country_name VARCHAR(100),
    postal_code VARCHAR(20),

    -- Metadata
    place_types TEXT[],                   -- e.g., ['street_address', 'premise']
    source_provider VARCHAR(50),          -- Which provider originally returned this
    confidence DECIMAL(3, 2),             -- Confidence score 0.00-1.00
    verified BOOLEAN DEFAULT false,       -- Has been manually verified

    -- Full-text search
    search_vector tsvector,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Foreign key to countries table
    CONSTRAINT fk_places_country
        FOREIGN KEY (country_code)
        REFERENCES countries(id)
        ON DELETE SET NULL
);

-- Indexes for places table
CREATE INDEX IF NOT EXISTS idx_places_geohash
    ON places(geohash)
    WHERE geohash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_places_country
    ON places(country_code)
    WHERE country_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_places_city
    ON places(city, country_code)
    WHERE city IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_places_postal
    ON places(postal_code, country_code)
    WHERE postal_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_places_external_id
    ON places(external_place_id)
    WHERE external_place_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_places_deleted
    ON places(deleted_at)
    WHERE deleted_at IS NULL;

-- GIN index for full-text search
CREATE INDEX IF NOT EXISTS idx_places_search
    ON places USING gin(search_vector);

-- ============================================================================
-- Function: Update search vector on places insert/update
-- ============================================================================
CREATE OR REPLACE FUNCTION places_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.formatted_address, '') || ' ' ||
        COALESCE(NEW.street_name, '') || ' ' ||
        COALESCE(NEW.city, '') || ' ' ||
        COALESCE(NEW.district, '') || ' ' ||
        COALESCE(NEW.state_name, '') || ' ' ||
        COALESCE(NEW.country_name, '') || ' ' ||
        COALESCE(NEW.postal_code, '')
    );
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for search vector updates
DROP TRIGGER IF EXISTS places_search_vector_trigger ON places;
CREATE TRIGGER places_search_vector_trigger
    BEFORE INSERT OR UPDATE ON places
    FOR EACH ROW EXECUTE FUNCTION places_search_vector_update();

-- ============================================================================
-- Function: Generate geohash from coordinates
-- Simple implementation for PostgreSQL (6-char hash_length ~1.2km)
-- ============================================================================
CREATE OR REPLACE FUNCTION generate_geohash(lat DECIMAL, lng DECIMAL, hash_length INT DEFAULT 6)
RETURNS VARCHAR AS $$
DECLARE
    base32_chars VARCHAR := '0123456789bcdefghjkmnpqrstuvwxyz';
    geohash VARCHAR := '';
    min_lat DECIMAL := -90.0;
    max_lat DECIMAL := 90.0;
    min_lng DECIMAL := -180.0;
    max_lng DECIMAL := 180.0;
    mid DECIMAL;
    bit INT := 0;
    ch INT := 0;
    is_lng BOOLEAN := true;
BEGIN
    WHILE length(geohash) < hash_length LOOP
        IF is_lng THEN
            mid := (min_lng + max_lng) / 2;
            IF lng >= mid THEN
                ch := ch | (16 >> bit);
                min_lng := mid;
            ELSE
                max_lng := mid;
            END IF;
        ELSE
            mid := (min_lat + max_lat) / 2;
            IF lat >= mid THEN
                ch := ch | (16 >> bit);
                min_lat := mid;
            ELSE
                max_lat := mid;
            END IF;
        END IF;

        is_lng := NOT is_lng;
        bit := bit + 1;

        IF bit = 5 THEN
            geohash := geohash || substr(base32_chars, ch + 1, 1);
            bit := 0;
            ch := 0;
        END IF;
    END LOOP;

    RETURN geohash;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- ============================================================================
-- Function: Update geohash on places insert/update
-- ============================================================================
CREATE OR REPLACE FUNCTION places_geohash_update() RETURNS trigger AS $$
BEGIN
    IF NEW.latitude IS NOT NULL AND NEW.longitude IS NOT NULL THEN
        NEW.geohash := generate_geohash(NEW.latitude, NEW.longitude, 8);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for geohash updates
DROP TRIGGER IF EXISTS places_geohash_trigger ON places;
CREATE TRIGGER places_geohash_trigger
    BEFORE INSERT OR UPDATE OF latitude, longitude ON places
    FOR EACH ROW EXECUTE FUNCTION places_geohash_update();

-- ============================================================================
-- Function: Update address_cache updated_at timestamp
-- ============================================================================
CREATE OR REPLACE FUNCTION address_cache_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS address_cache_updated_trigger ON address_cache;
CREATE TRIGGER address_cache_updated_trigger
    BEFORE UPDATE ON address_cache
    FOR EACH ROW EXECUTE FUNCTION address_cache_updated_at();

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE address_cache IS 'Cache for address lookup operations (geocode, reverse geocode, autocomplete)';
COMMENT ON TABLE places IS 'Permanent storage of geocoded places for internal GeoTag API';
COMMENT ON COLUMN address_cache.cache_type IS 'Type: geocode, reverse, autocomplete, place_details';
COMMENT ON COLUMN address_cache.cache_key_hash IS 'SHA256 hash of normalized query for fast lookups';
COMMENT ON COLUMN address_cache.hit_count IS 'Number of times this cache entry has been accessed';
COMMENT ON COLUMN places.geohash IS 'Geohash for efficient nearby queries (8 chars = ~38m precision)';
COMMENT ON COLUMN places.confidence IS 'Geocoding confidence score from 0.00 to 1.00';
COMMENT ON COLUMN places.verified IS 'Whether address has been manually verified';
