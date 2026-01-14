-- Migration: 000002_seed_data (DOWN)
-- Description: Remove seed data

SET search_path TO location, public;

-- Remove all seeded data
DELETE FROM states;
DELETE FROM timezones;
DELETE FROM currencies;
DELETE FROM countries;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = 2;
