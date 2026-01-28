-- Add favicon_url to tenants table for custom favicon support
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS favicon_url VARCHAR(512);

COMMENT ON COLUMN tenants.favicon_url IS 'URL to the tenant favicon image for browser tabs';
