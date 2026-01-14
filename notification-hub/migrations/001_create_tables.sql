-- Notification Hub Database Schema
-- Migration: 001_create_tables

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    title VARCHAR(500) NOT NULL,
    message TEXT,
    icon VARCHAR(255),
    action_url VARCHAR(2048),
    source_service VARCHAR(100) NOT NULL,
    source_event_id VARCHAR(255),
    entity_type VARCHAR(100),
    entity_id UUID,
    metadata JSONB DEFAULT '{}',
    group_key VARCHAR(255),
    group_count INT DEFAULT 1,
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP WITH TIME ZONE,
    is_archived BOOLEAN DEFAULT FALSE,
    archived_at TIMESTAMP WITH TIME ZONE,
    priority VARCHAR(20) DEFAULT 'normal',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for notifications
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_user
    ON notifications(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_user_unread
    ON notifications(tenant_id, user_id, is_read) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_user_created
    ON notifications(tenant_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_type
    ON notifications(type);
CREATE INDEX IF NOT EXISTS idx_notifications_source_event
    ON notifications(source_event_id);
CREATE INDEX IF NOT EXISTS idx_notifications_group_key
    ON notifications(tenant_id, user_id, group_key) WHERE group_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_expires
    ON notifications(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_entity
    ON notifications(entity_type, entity_id) WHERE entity_id IS NOT NULL;

-- Notification preferences table
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL,
    websocket_enabled BOOLEAN DEFAULT TRUE,
    sse_enabled BOOLEAN DEFAULT TRUE,
    category_preferences JSONB DEFAULT '{}',
    sound_enabled BOOLEAN DEFAULT TRUE,
    vibration_enabled BOOLEAN DEFAULT TRUE,
    quiet_hours_enabled BOOLEAN DEFAULT FALSE,
    quiet_hours_start TIME,
    quiet_hours_end TIME,
    quiet_hours_timezone VARCHAR(50),
    group_similar BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, user_id)
);

-- Indexes for preferences
CREATE INDEX IF NOT EXISTS idx_notification_preferences_tenant_user
    ON notification_preferences(tenant_id, user_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to auto-update updated_at
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_notification_preferences_updated_at ON notification_preferences;
CREATE TRIGGER update_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
