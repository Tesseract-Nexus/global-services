-- Migration 004: Seed INR pricing plans for India region
-- Uses ON CONFLICT for idempotency (matches existing seed pattern)

INSERT INTO subscription_plans (
    name, display_name, description,
    monthly_price_cents, yearly_price_cents, currency,
    max_products, max_users, max_storage_mb,
    features, sort_order, is_active, is_free, trial_days,
    region, is_default_trial
)
VALUES
    (
        'starter_inr', 'Starter', 'Perfect for small businesses in India',
        49900, 499000, 'inr',
        1000, 5, 2048,
        '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true}',
        10, true, false, 180,
        'in', true
    ),
    (
        'professional_inr', 'Professional', 'For growing businesses in India',
        99900, 999000, 'inr',
        10000, 15, 10240,
        '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true}',
        11, true, false, 90,
        'in', false
    ),
    (
        'enterprise_inr', 'Enterprise', 'For large-scale operations in India',
        249900, 2499000, 'inr',
        -1, -1, -1,
        '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true, "dedicated_support": true, "sla": true}',
        12, true, false, 30,
        'in', false
    )
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    monthly_price_cents = EXCLUDED.monthly_price_cents,
    yearly_price_cents = EXCLUDED.yearly_price_cents,
    currency = EXCLUDED.currency,
    max_products = EXCLUDED.max_products,
    max_users = EXCLUDED.max_users,
    max_storage_mb = EXCLUDED.max_storage_mb,
    features = EXCLUDED.features,
    sort_order = EXCLUDED.sort_order,
    trial_days = EXCLUDED.trial_days,
    region = EXCLUDED.region,
    is_default_trial = EXCLUDED.is_default_trial;
