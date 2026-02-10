-- 004_reseed_email_templates.sql
-- Clean reseed of all email templates with professional design.
--
-- Design system:
--   - Zinc palette (neutral gray, no blue tint)
--   - White card with 3px accent line at top (brand color)
--   - Logo/name inside card, centered
--   - Clean typography, system font stack
--   - Subtle info boxes (#fafafa, no border)
--   - Minimal footer (store name + "Powered by mark8ly" for store emails, "Powered by Tesserix" for platform emails)
--
-- Template types:
--   - 18 mark8ly templates (tenant_id = 'default-tenant') — use brand_primary_color, brand_logo_url
--   - 4 platform templates (tenant_id = 'platform') — use platform_logo_url
--
-- Variable convention: snake_case (matches frontend EmailTemplate interface)

BEGIN;

-- Wipe old seeds so we start clean
DELETE FROM notification_templates WHERE tenant_id IN ('system', 'default-tenant', 'platform');

-- ============================================================================
-- MARK8LY TEMPLATES (tenant_id = 'default-tenant')
-- ============================================================================

-- ─── ORDER ──────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Order Confirmation',
    'Sent to the customer when a new order is placed',
    'EMAIL', 'order',
    'Order confirmed — #{{.order_id}}',
    'Hi {{.customer_name}},

Thanks for your order. We''ll send you an update when it ships.

Order: {{.order_id}}
Total: {{.order_total}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Order confirmed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Order #{{.order_id}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, thanks for your order. We''ll send you an update when it ships.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Order number</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.order_id}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Total</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.order_total}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View order</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"order_id":"","customer_name":"","customer_email":"","order_total":"","order_items":"","order_status":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Order Shipped',
    'Sent when an order has shipped with tracking info',
    'EMAIL', 'order',
    'Your order #{{.order_id}} is on the way',
    'Hi {{.customer_name}},

Your order has shipped.

Order: {{.order_id}}
Tracking: {{.tracking_number}}
Track: {{.tracking_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Your order is on the way</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Order #{{.order_id}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your order has shipped. You can track it using the details below.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Tracking number</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.tracking_number}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.tracking_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Track package</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"order_id":"","customer_name":"","tracking_number":"","tracking_url":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Order Cancelled',
    'Sent when an order is cancelled',
    'EMAIL', 'order',
    'Order #{{.order_id}} has been cancelled',
    'Hi {{.customer_name}},

Your order #{{.order_id}} has been cancelled. If you didn''t request this, please contact us.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Order cancelled</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Order #{{.order_id}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your order has been cancelled. If you didn''t request this, please reach out to us.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Contact support</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"order_id":"","customer_name":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── PAYMENT ────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Payment Confirmation',
    'Sent when a payment is successfully processed',
    'EMAIL', 'payment',
    'Payment received — {{.payment_amount}}',
    'Hi {{.customer_name}},

We''ve received your payment of {{.payment_amount}}.

Transaction: {{.transaction_id}}
Method: {{.payment_method}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Payment received</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.payment_amount}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your payment has been processed successfully.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Amount</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.payment_amount}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Transaction</p>
        <p style="margin:0 0 14px;font-size:13px;color:#3f3f46;font-family:''SF Mono'',SFMono-Regular,Menlo,monospace;">{{.transaction_id}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Method</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.payment_method}}</p>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"customer_name":"","payment_amount":"","payment_method":"","transaction_id":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Payment Failed',
    'Sent when a payment attempt fails',
    'EMAIL', 'payment',
    'Payment failed — action required',
    'Hi {{.customer_name}},

Your payment of {{.payment_amount}} could not be processed. Please update your payment method and try again.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Payment failed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Action required</p>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your payment of <strong>{{.payment_amount}}</strong> could not be processed. Please update your payment method and try again.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Update payment</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"customer_name":"","payment_amount":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── CUSTOMER ───────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Customer Welcome',
    'Sent to new customers when they create an account',
    'EMAIL', 'customer',
    'Welcome to {{.store_name}}',
    'Hi {{.customer_name}},

Welcome to {{.store_name}}. Your account is ready.

Start browsing: {{.store_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Welcome to {{.store_name}}</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your account is ready. You can now browse products, track orders, and more.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Start shopping</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"customer_name":"","customer_email":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── AUTH ────────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Password Reset',
    'Sent when a user requests a password reset',
    'EMAIL', 'auth',
    'Reset your password',
    'We received a request to reset your password.

Reset here: {{.reset_url}}

If you didn''t request this, ignore this email. This link expires in 1 hour.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Reset your password</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">We received a request to reset your password. Click below to choose a new one.</p>
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.reset_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Reset password</a>
      </td></tr></table>
      <p style="margin:0;font-size:13px;line-height:1.5;color:#a1a1aa;">If you didn''t request this, you can ignore this email. This link expires in 1 hour.</p>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"user_name":"","user_email":"","reset_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Email Verification',
    'Sent to verify a user''s email address',
    'EMAIL', 'auth',
    'Verify your email',
    'Your verification code is: {{.verification_code}}

This code expires in 10 minutes.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Verify your email</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Enter this code to verify your email address:</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="text-align:center;padding:24px;background:#fafafa;border-radius:8px;">
        <span style="font-size:32px;font-weight:700;letter-spacing:6px;color:#18181b;">{{.verification_code}}</span>
      </td></tr></table>
      <p style="margin:0;font-size:13px;color:#a1a1aa;text-align:center;">This code expires in 10 minutes.</p>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"user_name":"","user_email":"","verification_code":"","verification_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── REVIEW ─────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Review Request',
    'Sent to ask a customer to review their purchase',
    'EMAIL', 'review',
    'How was {{.product_name}}?',
    'Hi {{.customer_name}},

We''d love to hear what you think of {{.product_name}}.

Leave a review: {{.review_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">How was your purchase?</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, we''d love to hear what you think of <strong>{{.product_name}}</strong>. Your feedback helps other shoppers.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.review_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Write a review</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"customer_name":"","product_name":"","review_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── TICKET ─────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Ticket Created',
    'Sent when a support ticket is created',
    'EMAIL', 'ticket',
    'Re: {{.ticket_subject}} (#{{.ticket_id}})',
    'Hi {{.customer_name}},

We''ve received your request and will get back to you shortly.

Ticket: {{.ticket_id}}
Subject: {{.ticket_subject}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">We got your message</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Ticket #{{.ticket_id}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, we''ve received your request and will get back to you shortly.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Subject</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.ticket_subject}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.ticket_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View ticket</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"ticket_id":"","customer_name":"","ticket_subject":"","ticket_status":"","ticket_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── VENDOR ─────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Vendor Application Received',
    'Sent when a new vendor applies to sell on the marketplace',
    'EMAIL', 'vendor',
    'New vendor application — {{.vendor_name}}',
    'A new vendor has applied to {{.store_name}}.

Vendor: {{.vendor_name}} ({{.vendor_email}})

Review: {{.action_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">New vendor application</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A new vendor has applied to sell on your marketplace.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Vendor</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.vendor_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Email</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.vendor_email}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.action_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Review application</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"vendor_name":"","vendor_email":"","store_name":"","action_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── COUPON ─────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Coupon Created',
    'Sent when a new coupon is available for a customer',
    'EMAIL', 'coupon',
    'Your discount code: {{.coupon_code}}',
    'Hi {{.customer_name}},

Use code {{.coupon_code}} to get {{.discount_amount}} off.

Expires: {{.expiry_date}}

Shop now: {{.store_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">You have a discount</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, here''s a discount code for you:</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 8px;"><tr><td style="text-align:center;padding:24px;background:#fafafa;border:1px dashed #e4e4e7;border-radius:8px;">
        <p style="margin:0 0 6px;font-size:24px;font-weight:700;letter-spacing:2px;color:#18181b;">{{.coupon_code}}</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#71717a;">{{.discount_amount}} off</p>
      </td></tr></table>
      <p style="margin:0 0 24px;font-size:13px;color:#a1a1aa;text-align:center;">Expires {{.expiry_date}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Shop now</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"customer_name":"","coupon_code":"","discount_amount":"","expiry_date":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── INVENTORY ──────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Low Stock Alert',
    'Sent to store admin when product stock falls below threshold',
    'EMAIL', 'inventory',
    'Low stock: {{.product_name}}',
    'Low stock warning for {{.product_name}} (SKU: {{.sku}}).

Current stock: {{.current_stock}} (threshold: {{.threshold}})

Manage inventory: {{.inventory_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Low stock alert</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A product has fallen below its stock threshold and may need restocking.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Product</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.product_name}} ({{.sku}})</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Current stock</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.current_stock}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Threshold</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.threshold}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.inventory_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage inventory</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"product_name":"","sku":"","current_stock":"","threshold":"","store_name":"","inventory_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── APPROVAL ───────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Required',
    'Sent when an item needs approval',
    'EMAIL', 'approval',
    'Approval needed: {{.item_name}}',
    '{{.requester_name}} submitted "{{.item_name}}" for approval.

Type: {{.approval_type}}
Review: {{.approval_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Approval needed</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;"><strong>{{.requester_name}}</strong> submitted an item for your review.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Item</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.item_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Type</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.approval_type}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.approval_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Review</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"requester_name":"","approval_type":"","item_name":"","approval_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── DOMAIN ─────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Verified',
    'Sent when a custom domain passes DNS verification',
    'EMAIL', 'domain',
    'Domain verified: {{.domain_name}}',
    'Your domain {{.domain_name}} has been verified and is now active.

DNS: {{.dns_status}}
SSL: {{.ssl_status}}

Manage domains: {{.settings_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Domain verified</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Your custom domain is now active and serving traffic.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Domain</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.domain_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">DNS</p>
        <p style="margin:0 0 14px;font-size:15px;color:#3f3f46;">{{.dns_status}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">SSL</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.ssl_status}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage domains</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"domain_name":"","dns_status":"","ssl_status":"","store_name":"","settings_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── CAMPAIGN ───────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Campaign Launched',
    'Sent to admin when a marketing campaign goes live',
    'EMAIL', 'campaign',
    'Campaign live: {{.campaign_name}}',
    'Your campaign "{{.campaign_name}}" is now live.

Runs: {{.start_date}} - {{.end_date}}

View: {{.campaign_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Campaign is live</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Your campaign is now active and reaching customers.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Campaign</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.campaign_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Duration</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.start_date}} &ndash; {{.end_date}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.campaign_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View campaign</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"campaign_name":"","campaign_status":"","start_date":"","end_date":"","store_name":"","campaign_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── GIFT CARD ──────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Gift Card Received',
    'Sent to the recipient of a gift card',
    'EMAIL', 'gift_card',
    'You received a gift card from {{.sender_name}}',
    'Hi {{.recipient_name}},

{{.sender_name}} sent you a {{.gift_card_amount}} gift card.

Code: {{.gift_card_code}}
Message: {{.gift_card_message}}

Redeem: {{.redeem_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">You received a gift card</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.recipient_name}}, <strong>{{.sender_name}}</strong> sent you a gift card.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 8px;"><tr><td style="text-align:center;padding:24px;background:#fafafa;border:1px dashed #e4e4e7;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Value</p>
        <p style="margin:0 0 16px;font-size:28px;font-weight:700;color:#18181b;">{{.gift_card_amount}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Code</p>
        <p style="margin:0;font-size:18px;font-weight:600;letter-spacing:2px;color:#18181b;">{{.gift_card_code}}</p>
      </td></tr></table>
      {{if .gift_card_message}}<p style="margin:16px 0 0;font-size:14px;color:#71717a;text-align:center;font-style:italic;">&ldquo;{{.gift_card_message}}&rdquo;</p>{{end}}
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px auto 0;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.redeem_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Redeem now</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"recipient_name":"","sender_name":"","gift_card_code":"","gift_card_amount":"","gift_card_message":"","redeem_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── STAFF ──────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Staff Invitation',
    'Sent when a staff member is invited to the store',
    'EMAIL', 'staff',
    'You''re invited to join {{.store_name}}',
    'Hi {{.staff_name}},

{{.inviter_name}} invited you to join {{.store_name}} as {{.role}}.

Accept: {{.invite_url}}

This invitation expires in 7 days.',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#18181b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">You''re invited</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.staff_name}}, <strong>{{.inviter_name}}</strong> invited you to join <strong>{{.store_name}}</strong> as <strong>{{.role}}</strong>.</p>
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.invite_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Accept invitation</a>
      </td></tr></table>
      <p style="margin:0;font-size:13px;color:#a1a1aa;">This invitation expires in 7 days.</p>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"staff_name":"","staff_email":"","role":"","inviter_name":"","invite_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ============================================================================
-- PLATFORM TEMPLATES (tenant_id = 'platform')
-- ============================================================================

-- ─── SYSTEM HEALTH ──────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'platform',
    'System Health Alert',
    'Sent when a service health issue is detected',
    'EMAIL', 'system_health',
    '[{{.alert_type}}] {{.service_name}} — {{.environment}}',
    'System Alert: {{.alert_type}}

Service: {{.service_name}}
Environment: {{.environment}}
Time: {{.timestamp}}

{{.alert_message}}

Dashboard: {{.dashboard_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#18181b;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">Tesserix</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">System health alert</h1>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fef2f2;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Service</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.service_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Environment</p>
        <p style="margin:0 0 14px;font-size:15px;color:#3f3f46;">{{.environment}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Alert</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#dc2626;">{{.alert_type}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Message</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.alert_message}}</p>
      </td></tr></table>
      <p style="margin:0 0 24px;font-size:13px;color:#a1a1aa;">{{.timestamp}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.dashboard_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View dashboard</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">Tesserix</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://tesserix.app" style="color:#a1a1aa;text-decoration:none;">Tesserix</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"service_name":"","alert_type":"","alert_message":"","timestamp":"","environment":"","dashboard_url":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── AUDIT ──────────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'platform',
    'Audit Log Alert',
    'Sent for significant audit events',
    'EMAIL', 'audit',
    'Audit: {{.action}} by {{.actor_name}}',
    'Audit Event

Actor: {{.actor_name}} ({{.actor_email}})
Action: {{.action}}
Resource: {{.resource_type}} {{.resource_id}}
Time: {{.timestamp}}
Details: {{.details}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#18181b;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">Tesserix</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Audit event</h1>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Actor</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.actor_name}} ({{.actor_email}})</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Action</p>
        <p style="margin:0 0 14px;font-size:13px;color:#18181b;font-family:''SF Mono'',SFMono-Regular,Menlo,monospace;">{{.action}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Resource</p>
        <p style="margin:0 0 14px;font-size:15px;color:#3f3f46;">{{.resource_type}} {{.resource_id}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Details</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.details}}</p>
      </td></tr></table>
      <p style="margin:0;font-size:13px;color:#a1a1aa;">{{.timestamp}}</p>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">Tesserix</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://tesserix.app" style="color:#a1a1aa;text-decoration:none;">Tesserix</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"actor_name":"","actor_email":"","action":"","resource_type":"","resource_id":"","timestamp":"","details":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── SECURITY ───────────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'platform',
    'Security Alert',
    'Sent for suspicious login or security events',
    'EMAIL', 'security',
    'Security alert: {{.event_type}}',
    'Security Event Detected

User: {{.user_name}} ({{.user_email}})
Event: {{.event_type}}
IP: {{.ip_address}}
Location: {{.location}}
Time: {{.timestamp}}

Take action: {{.action_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#dc2626;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">Tesserix</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">Security alert</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A security event has been detected on the platform.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fef2f2;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Event</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#dc2626;">{{.event_type}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">User</p>
        <p style="margin:0 0 14px;font-size:15px;color:#18181b;">{{.user_name}} ({{.user_email}})</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">IP / Location</p>
        <p style="margin:0;font-size:15px;color:#3f3f46;">{{.ip_address}} &mdash; {{.location}}</p>
      </td></tr></table>
      <p style="margin:0 0 24px;font-size:13px;color:#a1a1aa;">{{.timestamp}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#dc2626;border-radius:6px;">
        <a href="{{.action_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Review event</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">Tesserix</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://tesserix.app" style="color:#a1a1aa;text-decoration:none;">Tesserix</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"user_name":"","user_email":"","event_type":"","ip_address":"","location":"","timestamp":"","action_url":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── PLATFORM ADMIN ─────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'platform',
    'New Tenant Signup',
    'Sent to platform admins when a new tenant signs up',
    'EMAIL', 'platform_admin',
    'New tenant: {{.details}}',
    'New tenant signup on Tesserix.

Type: {{.notification_type}}
Details: {{.details}}
Time: {{.timestamp}}

Dashboard: {{.dashboard_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#18181b;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">Tesserix</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 24px;font-size:20px;font-weight:600;color:#18181b;">New tenant signup</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A new tenant has signed up on the platform.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#f0fdf4;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Details</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.details}}</p>
      </td></tr></table>
      <p style="margin:0 0 24px;font-size:13px;color:#a1a1aa;">{{.timestamp}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.dashboard_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View dashboard</a>
      </td></tr></table>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">Tesserix</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://tesserix.app" style="color:#a1a1aa;text-decoration:none;">Tesserix</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"admin_name":"","admin_email":"","notification_type":"","details":"","timestamp":"","dashboard_url":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── TENANT ONBOARDING ─────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'platform',
    'Tenant Welcome Pack',
    'Sent to new store owners when their store is created',
    'EMAIL', 'tenant_onboarding',
    'Your store is ready — {{.business_name}}',
    'Congratulations! Your store {{.business_name}} has been successfully created.

Admin Panel: {{.admin_url}}
Storefront: {{.storefront_url}}

Email: {{.email}}

Quick Start:
1. Add your products
2. Configure payments
3. Set up shipping
4. Customize your store

Need help? Contact {{.support_email}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#18181b;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">Tesserix</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 0;">
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Your store is ready</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.business_name}} is now live</p>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Congratulations! Your store <strong>{{.business_name}}</strong> has been successfully created. Here''s everything you need to get started.</p>
      <p style="margin:0 0 12px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;font-weight:600;">Your store URLs</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 8px;"><tr><td style="padding:14px 16px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;">Admin Panel</p>
        <a href="{{.admin_url}}" style="font-size:14px;color:#18181b;font-weight:500;text-decoration:none;word-break:break-all;">{{.admin_url}}</a>
      </td></tr></table>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:14px 16px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;">Storefront</p>
        <a href="{{.storefront_url}}" style="font-size:14px;color:#18181b;font-weight:500;text-decoration:none;word-break:break-all;">{{.storefront_url}}</a>
      </td></tr></table>
      <p style="margin:0 0 16px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;font-weight:600;">Quick start</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;">
        <tr>
          <td style="width:28px;vertical-align:top;padding:0 0 14px;"><div style="width:24px;height:24px;background:#f4f4f5;border-radius:50%;text-align:center;line-height:24px;font-size:12px;font-weight:600;color:#52525b;">1</div></td>
          <td style="vertical-align:top;padding:2px 0 14px 10px;"><p style="margin:0 0 2px;font-size:14px;font-weight:600;color:#18181b;">Add your products</p><p style="margin:0;font-size:13px;color:#71717a;line-height:1.5;">Upload inventory, set prices, and configure variants</p></td>
        </tr>
        <tr>
          <td style="width:28px;vertical-align:top;padding:0 0 14px;"><div style="width:24px;height:24px;background:#f4f4f5;border-radius:50%;text-align:center;line-height:24px;font-size:12px;font-weight:600;color:#52525b;">2</div></td>
          <td style="vertical-align:top;padding:2px 0 14px 10px;"><p style="margin:0 0 2px;font-size:14px;font-weight:600;color:#18181b;">Configure payments</p><p style="margin:0;font-size:13px;color:#71717a;line-height:1.5;">Connect Stripe, PayPal, or Razorpay to accept payments</p></td>
        </tr>
        <tr>
          <td style="width:28px;vertical-align:top;padding:0 0 14px;"><div style="width:24px;height:24px;background:#f4f4f5;border-radius:50%;text-align:center;line-height:24px;font-size:12px;font-weight:600;color:#52525b;">3</div></td>
          <td style="vertical-align:top;padding:2px 0 14px 10px;"><p style="margin:0 0 2px;font-size:14px;font-weight:600;color:#18181b;">Set up shipping</p><p style="margin:0;font-size:13px;color:#71717a;line-height:1.5;">Define shipping zones, rates, and delivery options</p></td>
        </tr>
        <tr>
          <td style="width:28px;vertical-align:top;padding:0 0 0;"><div style="width:24px;height:24px;background:#f4f4f5;border-radius:50%;text-align:center;line-height:24px;font-size:12px;font-weight:600;color:#52525b;">4</div></td>
          <td style="vertical-align:top;padding:2px 0 0 10px;"><p style="margin:0 0 2px;font-size:14px;font-weight:600;color:#18181b;">Customize your store</p><p style="margin:0;font-size:13px;color:#71717a;line-height:1.5;">Brand your storefront, add pages, and configure settings</p></td>
        </tr>
      </table>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 16px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 10px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;font-weight:600;">Account details</p>
        <table role="presentation" cellpadding="0" cellspacing="0" width="100%">
          <tr><td style="padding:2px 0;font-size:13px;color:#71717a;width:60px;">Email</td><td style="padding:2px 0 2px 8px;font-size:14px;color:#18181b;font-weight:500;">{{.email}}</td></tr>
          <tr><td style="padding:2px 0;font-size:13px;color:#71717a;width:60px;">Store</td><td style="padding:2px 0 2px 8px;font-size:14px;color:#18181b;font-weight:500;">{{.business_name}}</td></tr>
          {{if .tenant_slug}}<tr><td style="padding:2px 0;font-size:13px;color:#71717a;width:60px;">ID</td><td style="padding:2px 0 2px 8px;font-size:14px;color:#18181b;font-weight:500;font-family:''SF Mono'',SFMono-Regular,Menlo,monospace;">{{.tenant_slug}}</td></tr>{{end}}
        </table>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0" align="center"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.admin_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Open Admin Panel &rarr;</a>
      </td></tr></table>
    </td></tr>
    <tr><td style="height:36px;"></td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0 0 8px;font-size:13px;color:#71717a;">Need help? <a href="mailto:{{.support_email}}" style="color:#18181b;text-decoration:none;font-weight:500;">Contact support</a></p>
  <p style="margin:0;font-size:12px;color:#a1a1aa;">Tesserix</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://tesserix.app" style="color:#a1a1aa;text-decoration:none;">Tesserix</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"business_name":"","admin_url":"","storefront_url":"","email":"","tenant_slug":"","support_email":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

COMMIT;
