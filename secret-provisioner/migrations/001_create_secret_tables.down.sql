-- Secret Provisioner Service - Rollback Schema
-- Drops all tables created by the up migration

-- Drop audit log table first (no foreign key dependencies)
DROP TABLE IF EXISTS secret_audit_log;

-- Drop metadata table
DROP TABLE IF EXISTS secret_metadata;
