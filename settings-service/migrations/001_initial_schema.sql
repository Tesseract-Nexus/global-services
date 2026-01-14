-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Settings table
CREATE TABLE IF NOT EXISTS settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Context fields
    tenant_id UUID NOT NULL,
    application_id UUID NOT NULL,
    user_id UUID,
    scope VARCHAR(50) NOT NULL CHECK (scope IN ('global', 'tenant', 'application', 'user')),
    
    -- Settings data as JSONB for flexibility
    branding JSONB NOT NULL DEFAULT '{}',
    theme JSONB NOT NULL DEFAULT '{}',
    layout JSONB NOT NULL DEFAULT '{}',
    animations JSONB NOT NULL DEFAULT '{}',
    localization JSONB NOT NULL DEFAULT '{}',
    features JSONB NOT NULL DEFAULT '{}',
    user_preferences JSONB NOT NULL DEFAULT '{}',
    application JSONB NOT NULL DEFAULT '{}',
    
    -- Versioning and metadata
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Settings presets table
CREATE TABLE IF NOT EXISTS settings_presets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL CHECK (category IN ('theme', 'layout', 'complete')),
    settings JSONB NOT NULL,
    preview VARCHAR(500), -- URL to preview image
    tags JSONB DEFAULT '[]',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Settings history table for audit trail
CREATE TABLE IF NOT EXISTS settings_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settings_id UUID NOT NULL,
    operation VARCHAR(50) NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    changes JSONB,
    user_id UUID,
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Settings validation rules table
CREATE TABLE IF NOT EXISTS settings_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    field VARCHAR(255) NOT NULL,
    rule VARCHAR(100) NOT NULL,
    message TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('error', 'warning')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_settings_context ON settings (tenant_id, application_id, user_id, scope);
CREATE INDEX IF NOT EXISTS idx_settings_tenant ON settings (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_settings_application ON settings (application_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_settings_user ON settings (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_settings_scope ON settings (scope) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_settings_created_at ON settings (created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_settings_deleted_at ON settings (deleted_at);

CREATE INDEX IF NOT EXISTS idx_presets_category ON settings_presets (category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_presets_default ON settings_presets (is_default) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_presets_tags ON settings_presets USING GIN (tags) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_presets_created_at ON settings_presets (created_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_history_settings_id ON settings_histories (settings_id);
CREATE INDEX IF NOT EXISTS idx_history_created_at ON settings_histories (created_at);
CREATE INDEX IF NOT EXISTS idx_history_operation ON settings_histories (operation);

-- Foreign key constraints
ALTER TABLE settings_histories 
    ADD CONSTRAINT fk_settings_history_settings 
    FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE;

-- Unique constraint to prevent duplicate settings for same context
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_unique_context 
ON settings (tenant_id, application_id, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'), scope) 
WHERE deleted_at IS NULL;

-- Trigger to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_settings_updated_at 
    BEFORE UPDATE ON settings 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_presets_updated_at 
    BEFORE UPDATE ON settings_presets 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert some default presets
INSERT INTO settings_presets (name, description, category, settings, is_default, tags) 
VALUES 
(
    'Light Theme', 
    'Default light theme with modern colors', 
    'theme', 
    '{"theme": {"colorMode": "light", "colorScheme": "default", "borderRadius": 0.5, "fontScale": 1.0}}',
    true,
    '["light", "default", "modern"]'
),
(
    'Dark Theme', 
    'Modern dark theme with high contrast', 
    'theme', 
    '{"theme": {"colorMode": "dark", "colorScheme": "default", "borderRadius": 0.5, "fontScale": 1.0}}',
    false,
    '["dark", "modern", "contrast"]'
),
(
    'Compact Layout', 
    'Space-efficient layout for productivity', 
    'layout', 
    '{"layout": {"sidebar": {"position": "left", "collapsible": true, "defaultCollapsed": true, "width": 240}, "header": {"style": "minimal", "height": 48}}}',
    false,
    '["compact", "productivity", "minimal"]'
),
(
    'Default Complete', 
    'Complete default settings for new applications', 
    'complete', 
    '{"theme": {"colorMode": "light", "colorScheme": "default", "borderRadius": 0.5, "fontScale": 1.0}, "layout": {"sidebar": {"position": "left", "collapsible": true, "defaultCollapsed": false, "width": 280}}, "animations": {"globalSpeed": "normal", "reducedMotion": false}}',
    true,
    '["default", "complete", "starter"]'
)
ON CONFLICT DO NOTHING;