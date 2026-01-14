-- Seed default settings for ecommerce-admin application
-- This ensures the application has working default settings across all scopes

-- Default Tenant Settings
INSERT INTO settings (
    tenant_id,
    application_id,
    scope,
    theme,
    layout,
    ecommerce,
    security,
    notifications,
    marketing,
    integrations,
    performance,
    compliance
) VALUES (
    '00000000-0000-0000-0000-000000000001',  -- Default tenant
    '614ee1b9-9704-5ac0-86b4-df3823c77218',  -- ecommerce-admin
    'tenant',
    '{
        "mode": "light",
        "primaryColor": "#1976d2",
        "secondaryColor": "#dc004e",
        "fontFamily": "Roboto"
    }',
    '{
        "sidebar": {
            "collapsed": false,
            "width": 280
        },
        "header": {
            "fixed": true
        }
    }',
    '{
        "store": {
            "name": "Demo Store",
            "tagline": "Your Online Shopping Destination",
            "contactEmail": "contact@demostore.com"
        },
        "catalog": {
            "products": {
                "enableReviews": true,
                "enableWishlist": true
            }
        }
    }',
    '{
        "twoFactor": {
            "enabled": false
        },
        "sessionTimeout": 3600
    }',
    '{
        "email": {
            "enabled": true
        },
        "push": {
            "enabled": false
        }
    }',
    '{
        "newsletter": {
            "enabled": true
        }
    }',
    '{
        "payment": {
            "enabled": false
        }
    }',
    '{
        "caching": {
            "enabled": true
        }
    }',
    '{
        "gdpr": {
            "enabled": false
        }
    }'
),
(
    '00000000-0000-0000-0000-000000000001',  -- Default tenant
    '614ee1b9-9704-5ac0-86b4-df3823c77218',  -- ecommerce-admin
    'global',
    '{
        "mode": "light",
        "primaryColor": "#1976d2"
    }',
    '{
        "compact": false
    }',
    '{}',
    '{
        "passwordPolicy": {
            "minLength": 8,
            "requireUppercase": true,
            "requireNumbers": true
        }
    }',
    '{
        "system": {
            "enabled": true
        }
    }',
    '{}',
    '{}',
    '{
        "cdn": {
            "enabled": false
        }
    }',
    '{
        "auditLog": {
            "enabled": true,
            "retentionDays": 90
        }
    }'
),
(
    '00000000-0000-0000-0000-000000000001',  -- Default tenant
    '614ee1b9-9704-5ac0-86b4-df3823c77218',  -- ecommerce-admin
    'application',
    '{
        "defaultMode": "light"
    }',
    '{
        "responsive": true
    }',
    '{
        "features": {
            "products": true,
            "orders": true,
            "categories": true,
            "coupons": true
        }
    }',
    '{}',
    '{}',
    '{}',
    '{}',
    '{}',
    '{}'
)
ON CONFLICT DO NOTHING;
