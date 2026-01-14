-- Create auth database tables
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    azure_object_id VARCHAR(255) UNIQUE,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Roles table
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(name, tenant_id)
);

-- Permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- User roles junction table
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, role_id, tenant_id)
);

-- Role permissions junction table
CREATE TABLE IF NOT EXISTS role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(role_id, permission_id)
);

-- User permissions junction table (direct permissions)
CREATE TABLE IF NOT EXISTS user_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, permission_id, tenant_id)
);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    is_active BOOLEAN DEFAULT true,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_azure_object_id ON users(azure_object_id);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_tenant_id ON roles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_roles_is_system ON roles(is_system);

CREATE INDEX IF NOT EXISTS idx_permissions_name ON permissions(name);
CREATE INDEX IF NOT EXISTS idx_permissions_resource ON permissions(resource);
CREATE INDEX IF NOT EXISTS idx_permissions_action ON permissions(action);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_tenant_id ON user_roles(tenant_id);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

CREATE INDEX IF NOT EXISTS idx_user_permissions_user_id ON user_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_permission_id ON user_permissions(permission_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_tenant_id ON user_permissions(tenant_id);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_is_active ON sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Insert system permissions
INSERT INTO permissions (name, resource, action, description, is_system) VALUES
    -- User management permissions
    ('user:create', 'user', 'create', 'Create users', true),
    ('user:read', 'user', 'read', 'Read user information', true),
    ('user:update', 'user', 'update', 'Update user information', true),
    ('user:delete', 'user', 'delete', 'Delete users', true),
    
    -- Role management permissions
    ('role:create', 'role', 'create', 'Create roles', true),
    ('role:read', 'role', 'read', 'Read role information', true),
    ('role:update', 'role', 'update', 'Update role information', true),
    ('role:delete', 'role', 'delete', 'Delete roles', true),
    
    -- Category management permissions
    ('category:create', 'category', 'create', 'Create categories', true),
    ('category:read', 'category', 'read', 'Read category information', true),
    ('category:update', 'category', 'update', 'Update category information', true),
    ('category:delete', 'category', 'delete', 'Delete categories', true),
    ('category:approve', 'category', 'approve', 'Approve categories', true),
    
    -- Product management permissions
    ('product:create', 'product', 'create', 'Create products', true),
    ('product:read', 'product', 'read', 'Read product information', true),
    ('product:update', 'product', 'update', 'Update product information', true),
    ('product:delete', 'product', 'delete', 'Delete products', true),
    ('product:approve', 'product', 'approve', 'Approve products', true),
    
    -- Vendor management permissions
    ('vendor:create', 'vendor', 'create', 'Create vendors', true),
    ('vendor:read', 'vendor', 'read', 'Read vendor information', true),
    ('vendor:update', 'vendor', 'update', 'Update vendor information', true),
    ('vendor:delete', 'vendor', 'delete', 'Delete vendors', true),
    ('vendor:approve', 'vendor', 'approve', 'Approve vendors', true),
    
    -- Order management permissions
    ('order:create', 'order', 'create', 'Create orders', true),
    ('order:read', 'order', 'read', 'Read order information', true),
    ('order:update', 'order', 'update', 'Update order information', true),
    ('order:delete', 'order', 'delete', 'Delete orders', true),
    ('order:cancel', 'order', 'cancel', 'Cancel orders', true),
    ('order:refund', 'order', 'refund', 'Refund orders', true),
    
    -- Settings management permissions
    ('settings:read', 'settings', 'read', 'Read system settings', true),
    ('settings:update', 'settings', 'update', 'Update system settings', true),
    
    -- Dashboard and analytics permissions
    ('dashboard:view', 'dashboard', 'view', 'View dashboard', true),
    ('analytics:view', 'analytics', 'view', 'View analytics', true)
ON CONFLICT (name) DO NOTHING;

-- Insert system roles
INSERT INTO roles (name, description, tenant_id, is_system) VALUES
    ('super_admin', 'Super Administrator with all permissions', 'default-tenant', true),
    ('tenant_admin', 'Tenant Administrator with tenant-level permissions', 'default-tenant', true),
    ('category_manager', 'Category Manager with category management permissions', 'default-tenant', true),
    ('product_manager', 'Product Manager with product management permissions', 'default-tenant', true),
    ('vendor_manager', 'Vendor Manager with vendor management permissions', 'default-tenant', true),
    ('staff', 'Staff member with limited permissions', 'default-tenant', true),
    ('vendor', 'Vendor with vendor-specific permissions', 'default-tenant', true),
    ('customer', 'Customer with order permissions', 'default-tenant', true)
ON CONFLICT (name, tenant_id) DO NOTHING;

-- Assign permissions to super_admin role (all permissions)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'super_admin' AND r.tenant_id = 'default-tenant'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to tenant_admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'tenant_admin' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'user:create', 'user:read', 'user:update',
    'role:read',
    'category:create', 'category:read', 'category:update', 'category:delete', 'category:approve',
    'product:create', 'product:read', 'product:update', 'product:delete', 'product:approve',
    'vendor:create', 'vendor:read', 'vendor:update', 'vendor:delete', 'vendor:approve',
    'order:read', 'order:update', 'order:cancel', 'order:refund',
    'settings:read', 'settings:update',
    'dashboard:view', 'analytics:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to category_manager role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'category_manager' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'category:create', 'category:read', 'category:update', 'category:delete', 'category:approve',
    'dashboard:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to product_manager role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'product_manager' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'product:create', 'product:read', 'product:update', 'product:delete', 'product:approve',
    'category:read',
    'dashboard:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to vendor_manager role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'vendor_manager' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'vendor:create', 'vendor:read', 'vendor:update', 'vendor:delete', 'vendor:approve',
    'dashboard:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to staff role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'staff' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'category:read',
    'product:read',
    'vendor:read',
    'order:read',
    'dashboard:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to vendor role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'vendor' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'product:create', 'product:read', 'product:update',
    'order:read',
    'dashboard:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign permissions to customer role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'customer' AND r.tenant_id = 'default-tenant'
AND p.name IN (
    'order:create', 'order:read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;