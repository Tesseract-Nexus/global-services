-- Translation Service - Initial Schema
-- Creates tables for language management, translation caching, and tenant preferences

-- Languages table
CREATE TABLE IF NOT EXISTS languages (
    code VARCHAR(10) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100),
    rtl BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    region VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Translation cache table
CREATE TABLE IF NOT EXISTS translation_caches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(50) NOT NULL,
    source_lang VARCHAR(10) NOT NULL,
    target_lang VARCHAR(10) NOT NULL,
    source_hash VARCHAR(64) NOT NULL,
    source_text TEXT NOT NULL,
    translated_text TEXT NOT NULL,
    context VARCHAR(100),
    hit_count INTEGER DEFAULT 0,
    provider VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Create indexes for translation cache
CREATE INDEX IF NOT EXISTS idx_translation_cache_tenant ON translation_caches(tenant_id);
CREATE INDEX IF NOT EXISTS idx_translation_cache_languages ON translation_caches(source_lang, target_lang);
CREATE INDEX IF NOT EXISTS idx_translation_cache_expires ON translation_caches(expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_translation_cache_unique ON translation_caches(source_hash);

-- Translation statistics table
CREATE TABLE IF NOT EXISTS translation_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(50) NOT NULL UNIQUE,
    total_requests BIGINT DEFAULT 0,
    cache_hits BIGINT DEFAULT 0,
    cache_misses BIGINT DEFAULT 0,
    total_characters BIGINT DEFAULT 0,
    last_request_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Tenant language preferences table
CREATE TABLE IF NOT EXISTS tenant_language_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(50) NOT NULL UNIQUE,
    default_source_lang VARCHAR(10) DEFAULT 'en',
    default_target_lang VARCHAR(10) DEFAULT 'hi',
    enabled_languages JSONB,
    auto_detect BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- User language preferences table (multi-tenant)
-- Stores per-user language preferences within a tenant
-- Default is English (en) if user hasn't set a preference
CREATE TABLE IF NOT EXISTS user_language_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    preferred_language VARCHAR(10) NOT NULL DEFAULT 'en',
    source_language VARCHAR(10) DEFAULT 'en',
    auto_detect_source BOOLEAN DEFAULT TRUE,
    rtl_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- Unique constraint ensures one preference per user per tenant
    CONSTRAINT user_language_preferences_tenant_user_unique UNIQUE (tenant_id, user_id)
);

-- Create indexes for user language preferences
CREATE INDEX IF NOT EXISTS idx_user_lang_pref_tenant ON user_language_preferences(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_lang_pref_user ON user_language_preferences(user_id);
CREATE INDEX IF NOT EXISTS idx_user_lang_pref_language ON user_language_preferences(preferred_language);

-- Seed initial languages
INSERT INTO languages (code, name, native_name, rtl, is_active, region) VALUES
    -- Indian Languages
    ('hi', 'Hindi', 'हिन्दी', false, true, 'India'),
    ('ta', 'Tamil', 'தமிழ்', false, true, 'India'),
    ('te', 'Telugu', 'తెలుగు', false, true, 'India'),
    ('mr', 'Marathi', 'मराठी', false, true, 'India'),
    ('bn', 'Bengali', 'বাংলা', false, true, 'India'),
    ('gu', 'Gujarati', 'ગુજરાતી', false, true, 'India'),
    ('kn', 'Kannada', 'ಕನ್ನಡ', false, true, 'India'),
    ('ml', 'Malayalam', 'മലയാളം', false, true, 'India'),
    ('pa', 'Punjabi', 'ਪੰਜਾਬੀ', false, true, 'India'),
    ('or', 'Odia', 'ଓଡ଼ିଆ', false, true, 'India'),
    -- Global Languages
    ('en', 'English', 'English', false, true, 'Global'),
    ('es', 'Spanish', 'Español', false, true, 'Global'),
    ('fr', 'French', 'Français', false, true, 'Global'),
    ('de', 'German', 'Deutsch', false, true, 'Global'),
    ('pt', 'Portuguese', 'Português', false, true, 'Global'),
    ('it', 'Italian', 'Italiano', false, true, 'Global'),
    ('nl', 'Dutch', 'Nederlands', false, true, 'Global'),
    ('ru', 'Russian', 'Русский', false, true, 'Global'),
    ('zh', 'Chinese', '中文', false, true, 'Asia'),
    ('ja', 'Japanese', '日本語', false, true, 'Asia'),
    ('ko', 'Korean', '한국어', false, true, 'Asia'),
    -- Southeast Asian Languages
    ('th', 'Thai', 'ไทย', false, true, 'Southeast Asia'),
    ('vi', 'Vietnamese', 'Tiếng Việt', false, true, 'Southeast Asia'),
    ('id', 'Indonesian', 'Bahasa Indonesia', false, true, 'Southeast Asia'),
    ('ms', 'Malay', 'Bahasa Melayu', false, true, 'Southeast Asia'),
    ('tl', 'Filipino', 'Filipino', false, true, 'Southeast Asia'),
    -- Middle Eastern Languages
    ('ar', 'Arabic', 'العربية', true, true, 'Middle East'),
    ('fa', 'Persian', 'فارسی', true, true, 'Middle East'),
    ('he', 'Hebrew', 'עברית', true, true, 'Middle East'),
    ('tr', 'Turkish', 'Türkçe', false, true, 'Middle East')
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    rtl = EXCLUDED.rtl,
    region = EXCLUDED.region,
    updated_at = NOW();
