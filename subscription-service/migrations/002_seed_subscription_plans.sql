INSERT INTO subscription_plans (name, display_name, description, monthly_price_cents, yearly_price_cents, currency, max_products, max_users, max_storage_mb, features, sort_order, is_active, is_free, trial_days)
VALUES
    ('free', 'Free', 'Get started with basic features', 0, 0, 'usd', 100, 2, 500, '{"basic_analytics": true, "email_support": true}', 0, true, true, 0),
    ('starter', 'Starter', 'Perfect for small businesses', 2900, 29000, 'usd', 1000, 5, 2048, '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true}', 1, true, false, 14),
    ('professional', 'Professional', 'For growing businesses', 7900, 79000, 'usd', 10000, 15, 10240, '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true}', 2, true, false, 14),
    ('enterprise', 'Enterprise', 'For large-scale operations', 19900, 199000, 'usd', -1, -1, -1, '{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true, "dedicated_support": true, "sla": true}', 3, true, false, 30)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    monthly_price_cents = EXCLUDED.monthly_price_cents,
    yearly_price_cents = EXCLUDED.yearly_price_cents,
    max_products = EXCLUDED.max_products,
    max_users = EXCLUDED.max_users,
    max_storage_mb = EXCLUDED.max_storage_mb,
    features = EXCLUDED.features,
    sort_order = EXCLUDED.sort_order,
    trial_days = EXCLUDED.trial_days;
