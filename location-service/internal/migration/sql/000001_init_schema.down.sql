-- Migration: 000001_init_schema (DOWN)
-- Description: Rollback initial location service schema

SET search_path TO location, public;

DROP TABLE IF EXISTS location_cache;
DROP TABLE IF EXISTS states;
DROP TABLE IF EXISTS timezones;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS countries;
DROP TABLE IF EXISTS schema_migrations;

DROP SCHEMA IF EXISTS location;
