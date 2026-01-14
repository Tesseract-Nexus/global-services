-- Migration: 000001_init_schema
-- Description: Create initial location service schema with countries, states, currencies, timezones
-- Created: 2024-01-01

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS location;

-- Set search path
SET search_path TO location, public;

-- =====================================================
-- COUNTRIES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS countries (
    id VARCHAR(2) PRIMARY KEY,  -- ISO 3166-1 alpha-2
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100),
    capital VARCHAR(100),
    region VARCHAR(50),
    subregion VARCHAR(50),
    currency VARCHAR(3),  -- ISO 4217
    languages TEXT,  -- JSON array as string
    calling_code VARCHAR(10),
    flag_emoji VARCHAR(10),
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_countries_name ON countries(name);
CREATE INDEX IF NOT EXISTS idx_countries_region ON countries(region);
CREATE INDEX IF NOT EXISTS idx_countries_active ON countries(active);
CREATE INDEX IF NOT EXISTS idx_countries_deleted_at ON countries(deleted_at);

-- =====================================================
-- STATES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS states (
    id VARCHAR(10) PRIMARY KEY,  -- Country-State format: US-CA
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100),
    code VARCHAR(10) NOT NULL,  -- State/province code
    country_id VARCHAR(2) NOT NULL REFERENCES countries(id),
    type VARCHAR(20) DEFAULT 'state',  -- state, province, territory, etc.
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_states_country_id ON states(country_id);
CREATE INDEX IF NOT EXISTS idx_states_name ON states(name);
CREATE INDEX IF NOT EXISTS idx_states_code ON states(code);
CREATE INDEX IF NOT EXISTS idx_states_active ON states(active);
CREATE INDEX IF NOT EXISTS idx_states_deleted_at ON states(deleted_at);

-- =====================================================
-- CURRENCIES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS currencies (
    code VARCHAR(3) PRIMARY KEY,  -- ISO 4217
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    decimal_places INTEGER DEFAULT 2,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_currencies_active ON currencies(active);
CREATE INDEX IF NOT EXISTS idx_currencies_deleted_at ON currencies(deleted_at);

-- =====================================================
-- TIMEZONES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS timezones (
    id VARCHAR(50) PRIMARY KEY,  -- e.g., America/New_York
    name VARCHAR(100) NOT NULL,
    abbreviation VARCHAR(10),
    "offset" VARCHAR(10) NOT NULL,  -- e.g., -05:00
    dst BOOLEAN DEFAULT false,
    countries TEXT,  -- JSON array as string
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_timezones_deleted_at ON timezones(deleted_at);

-- =====================================================
-- LOCATION CACHE TABLE (for IP geolocation caching)
-- =====================================================
CREATE TABLE IF NOT EXISTS location_cache (
    id BIGSERIAL PRIMARY KEY,
    ip VARCHAR(45) UNIQUE NOT NULL,  -- Supports IPv4 and IPv6
    country_id VARCHAR(2) REFERENCES countries(id),
    state_id VARCHAR(10) REFERENCES states(id),
    city VARCHAR(100),
    postal_code VARCHAR(20),
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    timezone_id VARCHAR(50) REFERENCES timezones(id),
    isp VARCHAR(200),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_location_cache_ip ON location_cache(ip);
CREATE INDEX IF NOT EXISTS idx_location_cache_expires_at ON location_cache(expires_at);

-- =====================================================
-- MIGRATION TRACKING TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN DEFAULT false,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Record this migration
INSERT INTO schema_migrations (version, dirty) VALUES (1, false) ON CONFLICT (version) DO NOTHING;
