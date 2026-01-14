-- Audit Service Database Schema

-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tenant and user info
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID,
    username VARCHAR(255),
    user_email VARCHAR(255),

    -- Action details
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    resource_name VARCHAR(255),

    -- Outcome
    status VARCHAR(20) NOT NULL,
    severity VARCHAR(20) DEFAULT 'MEDIUM',

    -- Request details
    method VARCHAR(10),
    path VARCHAR(500),
    query TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    request_id VARCHAR(100),

    -- Changes tracking
    old_value JSONB,
    new_value JSONB,
    changes JSONB,

    -- Additional context
    description TEXT,
    metadata JSONB,
    tags JSONB,

    -- Error details
    error_message TEXT,
    error_code VARCHAR(50),

    -- Service info
    service_name VARCHAR(100),
    service_version VARCHAR(50),

    -- Timestamps
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs(resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_severity ON audit_logs(severity);
CREATE INDEX IF NOT EXISTS idx_audit_logs_ip_address ON audit_logs(ip_address);
CREATE INDEX IF NOT EXISTS idx_audit_logs_service_name ON audit_logs(service_name);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_timestamp ON audit_logs(tenant_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_timestamp ON audit_logs(user_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_user ON audit_logs(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_action ON audit_logs(tenant_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_resource ON audit_logs(tenant_id, resource);

-- GIN index for JSONB columns for fast JSON queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_metadata_gin ON audit_logs USING GIN (metadata);
CREATE INDEX IF NOT EXISTS idx_audit_logs_changes_gin ON audit_logs USING GIN (changes);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tags_gin ON audit_logs USING GIN (tags);

-- Full-text search index for description and resource_name
CREATE INDEX IF NOT EXISTS idx_audit_logs_description_fulltext ON audit_logs USING GIN (to_tsvector('english', COALESCE(description, '')));
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_name_fulltext ON audit_logs USING GIN (to_tsvector('english', COALESCE(resource_name, '')));

-- Partitioning by month (optional, for large deployments)
-- This can be enabled later if audit logs grow very large
-- CREATE TABLE audit_logs_2025_12 PARTITION OF audit_logs
--     FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');

-- Function to automatically clean up old audit logs (retention policy)
CREATE OR REPLACE FUNCTION cleanup_old_audit_logs(retention_days INTEGER DEFAULT 365)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM audit_logs
    WHERE timestamp < NOW() - (retention_days || ' days')::INTERVAL;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create a scheduled job to run cleanup (requires pg_cron extension)
-- SELECT cron.schedule('cleanup-audit-logs', '0 2 * * 0', 'SELECT cleanup_old_audit_logs(365);');

-- View for recent critical events
CREATE OR REPLACE VIEW recent_critical_events AS
SELECT
    id,
    tenant_id,
    user_id,
    username,
    action,
    resource,
    resource_id,
    resource_name,
    status,
    severity,
    ip_address,
    description,
    timestamp
FROM audit_logs
WHERE severity IN ('HIGH', 'CRITICAL')
    AND timestamp > NOW() - INTERVAL '7 days'
ORDER BY timestamp DESC;

-- View for failed authentication attempts
CREATE OR REPLACE VIEW failed_auth_attempts AS
SELECT
    id,
    tenant_id,
    user_id,
    username,
    user_email,
    ip_address,
    user_agent,
    error_message,
    timestamp,
    COUNT(*) OVER (PARTITION BY ip_address, DATE(timestamp)) as attempts_today
FROM audit_logs
WHERE action = 'LOGIN_FAILED'
    AND timestamp > NOW() - INTERVAL '24 hours'
ORDER BY timestamp DESC;

-- View for user activity summary
CREATE OR REPLACE VIEW user_activity_summary AS
SELECT
    tenant_id,
    user_id,
    username,
    user_email,
    COUNT(*) as total_actions,
    COUNT(CASE WHEN status = 'SUCCESS' THEN 1 END) as successful_actions,
    COUNT(CASE WHEN status = 'FAILURE' THEN 1 END) as failed_actions,
    MAX(timestamp) as last_activity,
    MIN(timestamp) as first_activity,
    ARRAY_AGG(DISTINCT action ORDER BY action) as actions_performed,
    ARRAY_AGG(DISTINCT resource ORDER BY resource) as resources_accessed
FROM audit_logs
WHERE timestamp > NOW() - INTERVAL '30 days'
    AND user_id IS NOT NULL
GROUP BY tenant_id, user_id, username, user_email
ORDER BY total_actions DESC;

-- View for resource modification history
CREATE OR REPLACE VIEW resource_modification_history AS
SELECT
    resource,
    resource_id,
    resource_name,
    action,
    user_id,
    username,
    old_value,
    new_value,
    changes,
    timestamp,
    ROW_NUMBER() OVER (PARTITION BY resource, resource_id ORDER BY timestamp DESC) as version
FROM audit_logs
WHERE action IN ('CREATE', 'UPDATE', 'DELETE')
    AND resource_id IS NOT NULL
ORDER BY timestamp DESC;

-- Comments for documentation
COMMENT ON TABLE audit_logs IS 'Comprehensive audit log for all system activities';
COMMENT ON COLUMN audit_logs.tenant_id IS 'Multi-tenant identifier';
COMMENT ON COLUMN audit_logs.action IS 'Type of action performed (LOGIN, CREATE, UPDATE, etc.)';
COMMENT ON COLUMN audit_logs.resource IS 'Type of resource affected (USER, ORDER, PRODUCT, etc.)';
COMMENT ON COLUMN audit_logs.old_value IS 'Previous state of the resource (for UPDATE actions)';
COMMENT ON COLUMN audit_logs.new_value IS 'New state of the resource (for CREATE/UPDATE actions)';
COMMENT ON COLUMN audit_logs.changes IS 'Diff showing what changed (for UPDATE actions)';
COMMENT ON COLUMN audit_logs.severity IS 'Importance level: LOW, MEDIUM, HIGH, CRITICAL';
COMMENT ON COLUMN audit_logs.metadata IS 'Additional context-specific information';
