-- Secret Provisioner Service - Initial Schema
-- Creates tables for secret metadata and audit logging

-- Enable UUID extension if not exists
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Table: secret_metadata
-- Stores metadata about secrets (NOT the secret values!)
-- =============================================================================
CREATE TABLE IF NOT EXISTS secret_metadata (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tenant/Scope identifiers
    tenant_id           VARCHAR(100) NOT NULL,
    category            VARCHAR(50) NOT NULL,    -- payment, integration, api-key, oauth, database, webhook
    scope               VARCHAR(20) NOT NULL,    -- tenant, vendor, service
    scope_id            VARCHAR(100),            -- vendorID if scope=vendor

    -- Secret identity
    provider            VARCHAR(50) NOT NULL,    -- stripe, razorpay, etc.
    key_name            VARCHAR(100) NOT NULL,   -- api-key, webhook-secret, etc.
    gcp_secret_name     VARCHAR(255) NOT NULL,   -- Full GCP Secret Manager name
    gcp_secret_version  VARCHAR(50),             -- Current version number

    -- Status
    configured          BOOLEAN NOT NULL DEFAULT true,
    validation_status   VARCHAR(20) DEFAULT 'UNKNOWN',  -- VALID, INVALID, UNKNOWN
    validation_message  TEXT,

    -- Audit
    last_updated_by     VARCHAR(100),
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    CONSTRAINT uq_secret_metadata UNIQUE (tenant_id, category, scope, scope_id, provider, key_name),
    CONSTRAINT uq_gcp_secret_name UNIQUE (gcp_secret_name)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_secret_meta_tenant ON secret_metadata(tenant_id);
CREATE INDEX IF NOT EXISTS idx_secret_meta_category ON secret_metadata(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_secret_meta_provider ON secret_metadata(tenant_id, category, provider);
CREATE INDEX IF NOT EXISTS idx_secret_meta_gcp ON secret_metadata(gcp_secret_name);

-- =============================================================================
-- Table: secret_audit_log
-- Records all secret operations for security auditing
-- =============================================================================
CREATE TABLE IF NOT EXISTS secret_audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Context
    tenant_id       VARCHAR(100) NOT NULL,
    secret_name     VARCHAR(255) NOT NULL,   -- GCP secret name
    category        VARCHAR(50) NOT NULL,
    provider        VARCHAR(50) NOT NULL,

    -- Action details
    action          VARCHAR(50) NOT NULL,    -- created, updated, deleted, validated, accessed
    status          VARCHAR(20) NOT NULL,    -- success, failure
    error_message   TEXT,

    -- Actor information
    actor_id        VARCHAR(100),            -- User ID who performed action
    actor_service   VARCHAR(100),            -- Service that made the request

    -- Request metadata
    request_id      VARCHAR(100),
    ip_address      VARCHAR(50),
    metadata        JSONB,                   -- Additional context (non-secret data only)

    -- Timestamp
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for audit queries
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON secret_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_secret ON secret_audit_log(secret_name);
CREATE INDEX IF NOT EXISTS idx_audit_time ON secret_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON secret_audit_log(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON secret_audit_log(actor_id, created_at DESC);

-- =============================================================================
-- Comments
-- =============================================================================
COMMENT ON TABLE secret_metadata IS 'Stores metadata about secrets managed by the Secret Provisioner. NEVER stores actual secret values.';
COMMENT ON COLUMN secret_metadata.gcp_secret_name IS 'The full secret name in GCP Secret Manager. Secret values are stored in GCP, not this table.';
COMMENT ON COLUMN secret_metadata.validation_status IS 'Result of credential validation: VALID, INVALID, or UNKNOWN';

COMMENT ON TABLE secret_audit_log IS 'Audit trail for all secret operations. Critical for security compliance.';
COMMENT ON COLUMN secret_audit_log.metadata IS 'Additional context as JSON. Must NEVER contain secret values.';
