-- B2C Multi-tenant ecommerce setup
-- This migration configures the auth system for isolated accounts per store

-- 1. Add store/marketplace applications table
CREATE TABLE IF NOT EXISTS stores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL, -- store-abc, marketplace-xyz
    domain VARCHAR(255), -- custom domain: store-abc.com
    subdomain VARCHAR(100) UNIQUE, -- subdomain: abc.tesseract.com
    description TEXT,
    logo_url VARCHAR(500),
    theme_settings JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    owner_email VARCHAR(255) NOT NULL,
    plan VARCHAR(50) DEFAULT 'basic', -- basic, pro, enterprise
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 2. Add password support and B2C fields to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_number VARCHAR(20);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS date_of_birth DATE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS gender VARCHAR(10);
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture_url VARCHAR(500);
ALTER TABLE users ADD COLUMN IF NOT EXISTS marketing_consent BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES stores(id) ON DELETE CASCADE;

-- 3. Remove global email constraint and add store-specific constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_per_store_idx ON users(email, store_id);

-- 4. Add OAuth providers for social login
CREATE TABLE IF NOT EXISTS oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- google, facebook, apple, twitter
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    name VARCHAR(255),
    profile_picture_url VARCHAR(500),
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMP,
    profile_data JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider, provider_user_id, user_id)
);

-- 5. Add email verification and password reset
CREATE TABLE IF NOT EXISTS verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    token_type VARCHAR(50) NOT NULL, -- email_verification, password_reset, phone_verification
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 6. Update roles to be store-specific
ALTER TABLE roles ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES stores(id) ON DELETE CASCADE;
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_tenant_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS roles_name_per_store_idx ON roles(name, tenant_id, store_id);

-- 7. Update permissions to include store-specific ones
INSERT INTO permissions (name, resource, action, description, is_system) VALUES
    -- Customer permissions (B2C)
    ('customer.profile:read', 'profile', 'read', 'Read own profile', true),
    ('customer.profile:update', 'profile', 'update', 'Update own profile', true),
    ('customer.order:create', 'order', 'create', 'Place orders', true),
    ('customer.order:read', 'order', 'read', 'View own orders', true),
    ('customer.order:cancel', 'order', 'cancel', 'Cancel own orders', true),
    ('customer.wishlist:manage', 'wishlist', 'manage', 'Manage wishlist', true),
    ('customer.review:create', 'review', 'create', 'Write product reviews', true),
    ('customer.support:create', 'support', 'create', 'Create support tickets', true),
    
    -- Store owner permissions
    ('store.settings:manage', 'store', 'manage', 'Manage store settings', true),
    ('store.analytics:view', 'analytics', 'view', 'View store analytics', true),
    ('store.customers:view', 'customers', 'view', 'View customer list', true),
    ('store.orders:manage', 'orders', 'manage', 'Manage all store orders', true),
    ('store.products:manage', 'products', 'manage', 'Manage store products', true),
    ('store.staff:manage', 'staff', 'manage', 'Manage store staff', true),
    
    -- Marketplace permissions (multi-store)
    ('marketplace.stores:manage', 'stores', 'manage', 'Manage marketplace stores', true),
    ('marketplace.fees:manage', 'fees', 'manage', 'Manage marketplace fees', true),
    ('marketplace.disputes:manage', 'disputes', 'manage', 'Handle store disputes', true)
ON CONFLICT (name) DO NOTHING;

-- 8. Create B2C default roles
INSERT INTO roles (name, description, tenant_id, is_system) VALUES
    -- Customer roles (per store)
    ('customer', 'Store customer with ordering permissions', 'default-tenant', true),
    ('vip_customer', 'VIP customer with special privileges', 'default-tenant', true),
    
    -- Store management roles
    ('store_owner', 'Store owner with full store control', 'default-tenant', true),
    ('store_manager', 'Store manager with operational control', 'default-tenant', true),
    ('store_staff', 'Store staff with limited permissions', 'default-tenant', true),
    
    -- Marketplace roles
    ('marketplace_admin', 'Marketplace administrator', 'default-tenant', true),
    ('marketplace_moderator', 'Marketplace content moderator', 'default-tenant', true)
ON CONFLICT (name, tenant_id, store_id) DO NOTHING;

-- 9. Assign customer permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'customer' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'customer.profile:read', 'customer.profile:update',
    'customer.order:create', 'customer.order:read', 'customer.order:cancel',
    'customer.wishlist:manage', 'customer.review:create', 'customer.support:create'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- 10. Assign store owner permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'store_owner' AND r.tenant_id = 'default-tenant'
AND p.name LIKE 'store.%'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- 11. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_store_id ON users(store_id);
CREATE INDEX IF NOT EXISTS idx_users_email_verified ON users(email_verified);
CREATE INDEX IF NOT EXISTS idx_oauth_providers_user_id ON oauth_providers(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_providers_provider ON oauth_providers(provider);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_token ON verification_tokens(token);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_user_id ON verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_stores_slug ON stores(slug);
CREATE INDEX IF NOT EXISTS idx_stores_subdomain ON stores(subdomain);
CREATE INDEX IF NOT EXISTS idx_stores_owner_email ON stores(owner_email);

-- 12. Insert sample stores for testing
INSERT INTO stores (name, slug, subdomain, description, owner_email, is_active) VALUES
    ('Demo Electronics Store', 'demo-electronics', 'electronics', 'Sample electronics store for testing', 'owner@electronics-demo.com', true),
    ('Fashion Boutique', 'fashion-boutique', 'fashion', 'Sample fashion store for testing', 'owner@fashion-demo.com', true),
    ('Marketplace Central', 'marketplace-central', 'marketplace', 'Multi-vendor marketplace for testing', 'admin@marketplace-demo.com', true)
ON CONFLICT (slug) DO NOTHING;