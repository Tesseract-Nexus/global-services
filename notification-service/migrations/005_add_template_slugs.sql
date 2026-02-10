-- Add slug column, backfill existing templates, insert new templates.

BEGIN;

-- ============================================================================
-- PART 1: Add slug column
-- ============================================================================

ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS slug VARCHAR(255);

-- ============================================================================
-- PART 2: Backfill slugs for existing 23 templates from migration 004
-- ============================================================================

-- default-tenant templates (19)
UPDATE notification_templates SET slug = 'order-confirmation' WHERE name = 'Order Confirmation' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'order-shipped' WHERE name = 'Order Shipped' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'order-cancelled' WHERE name = 'Order Cancelled' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'payment-confirmation' WHERE name = 'Payment Confirmation' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'payment-failed' WHERE name = 'Payment Failed' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'welcome-email' WHERE name = 'Customer Welcome' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'password-reset' WHERE name = 'Password Reset' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'verification-code' WHERE name = 'Email Verification' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'review-submitted-customer' WHERE name = 'Review Request' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'ticket-created' WHERE name = 'Ticket Created' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'vendor-application' WHERE name = 'Vendor Application Received' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'coupon-created' WHERE name = 'Coupon Created' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'low-stock-alert' WHERE name = 'Low Stock Alert' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'approval-request' WHERE name = 'Approval Required' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'domain-verified' WHERE name = 'Domain Verified' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'campaign-admin' WHERE name = 'Campaign Launched' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'gift-card-recipient' WHERE name = 'Gift Card Received' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'staff-invitation' WHERE name = 'Staff Invitation' AND tenant_id = 'default-tenant';
UPDATE notification_templates SET slug = 'tenant-welcome-pack' WHERE name = 'Tenant Welcome Pack' AND tenant_id = 'default-tenant';

-- platform templates (4)
UPDATE notification_templates SET slug = 'system-health-alert' WHERE name = 'System Health Alert' AND tenant_id = 'platform';
UPDATE notification_templates SET slug = 'audit-log-alert' WHERE name = 'Audit Log Alert' AND tenant_id = 'platform';
UPDATE notification_templates SET slug = 'security-alert' WHERE name = 'Security Alert' AND tenant_id = 'platform';
UPDATE notification_templates SET slug = 'new-tenant-signup' WHERE name = 'New Tenant Signup' AND tenant_id = 'platform';

-- ============================================================================
-- PART 3: Insert new templates
-- ============================================================================

-- ─── 1. order-delivered ────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Order Delivered',
    'order-delivered',
    'Sent when an order has been delivered',
    'EMAIL', 'order',
    'Your order #{{.order_id}} has been delivered',
    'Hi {{.customer_name}}, your order has been delivered. We hope you love it!

Delivered on: {{.delivery_date}}
Location: {{.delivery_location}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Order delivered</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Order #{{.order_id}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your order has been delivered. We hope you love it!</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Delivered on</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.delivery_date}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Location</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.delivery_location}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.store_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Leave a review</a>
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
    '{"order_id":"","customer_name":"","delivery_date":"","delivery_location":"","store_name":"","store_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 2. payment-refunded ──────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Payment Refunded',
    'payment-refunded',
    'Sent when a payment has been refunded',
    'EMAIL', 'payment',
    'Refund processed — {{.refund_amount}}',
    'Hi {{.customer_name}}, your refund of {{.refund_amount}} has been processed. It may take 5-10 business days to appear.

Amount: {{.refund_amount}}
Transaction: {{.transaction_id}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Refund processed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.refund_amount}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your refund of {{.refund_amount}} has been processed. It may take 5-10 business days to appear.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Amount</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.refund_amount}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Transaction</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.transaction_id}}</p>
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
    '{"customer_name":"","refund_amount":"","transaction_id":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 3. login-notification ────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Login Notification',
    'login-notification',
    'Sent when a new login is detected',
    'EMAIL', 'auth',
    'New sign-in to your account',
    'Hi {{.customer_name}}, we detected a new sign-in to your account.

Time: {{.login_time}}
Location: {{.login_location}}
IP: {{.ip_address}}
Device: {{.device_info}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">New sign-in detected</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Account security</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, we detected a new sign-in to your account.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Time</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.login_time}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Location</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.login_location}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">IP address</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.ip_address}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Device</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.device_info}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.reset_password_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Secure your account</a>
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
    '{"customer_name":"","login_time":"","login_location":"","ip_address":"","device_info":"","reset_password_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 4. verification-link ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Verification Link',
    'verification-link',
    'Sent during tenant onboarding for email verification',
    'EMAIL', 'tenant_onboarding',
    'Verify your email address',
    'Hi {{.customer_name}}, please verify your email to continue setting up your store.

{{.verification_link}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Verify your email</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Email verification</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, please verify your email to continue setting up your store.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.verification_link}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Verify email</a>
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
    '{"customer_name":"","verification_link":"","verification_expiry":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 5. review-submitted-admin ────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Review Submitted (Admin)',
    'review-submitted-admin',
    'Sent to admin when a new review is submitted',
    'EMAIL', 'review',
    'New review for {{.product_name}}',
    'A new {{.rating}}-star review has been submitted for {{.product_name}} and requires moderation.

Product: {{.product_name}}
Rating: {{.rating}}/{{.max_rating}}
Customer: {{.customer_name}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">New review submitted</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.product_name}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A new {{.rating}}-star review has been submitted for {{.product_name}} and requires moderation.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Product</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.product_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Rating</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.rating}}/{{.max_rating}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Customer</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.customer_name}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.reviews_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Review now</a>
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
    '{"product_name":"","customer_name":"","rating":"","max_rating":"","review_title":"","review_content":"","reviews_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 6. review-approved ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Review Approved',
    'review-approved',
    'Sent when a customer review is approved',
    'EMAIL', 'review',
    'Your review has been published',
    'Hi {{.customer_name}}, your review for {{.product_name}} has been approved and is now visible.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Review published</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.product_name}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your review for {{.product_name}} has been approved and is now visible.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.product_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View your review</a>
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
    '{"customer_name":"","product_name":"","review_title":"","product_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 7. review-rejected ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Review Rejected',
    'review-rejected',
    'Sent when a customer review is rejected',
    'EMAIL', 'review',
    'Update on your review',
    'Hi {{.customer_name}}, your review for {{.product_name}} could not be published. Reason: {{.reject_reason}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Review update</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.product_name}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your review for {{.product_name}} could not be published. Reason: {{.reject_reason}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.product_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Submit a new review</a>
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
    '{"customer_name":"","product_name":"","reject_reason":"","product_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 8. ticket-created-admin ──────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Ticket Created (Admin)',
    'ticket-created-admin',
    'Sent to support when a new ticket is created',
    'EMAIL', 'ticket',
    'New support ticket #{{.ticket_number}}',
    'A new support ticket has been created and needs attention.

Ticket: #{{.ticket_number}}
Subject: {{.ticket_subject}}
Priority: {{.ticket_priority}}
Category: {{.ticket_category}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">New support ticket</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">#{{.ticket_number}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">A new support ticket has been created and needs attention.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Ticket</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">#{{.ticket_number}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Subject</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.ticket_subject}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Priority</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.ticket_priority}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Category</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.ticket_category}}</p>
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
    '{"ticket_number":"","ticket_subject":"","ticket_priority":"","ticket_category":"","customer_name":"","customer_email":"","ticket_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 9. ticket-updated ────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Ticket Updated',
    'ticket-updated',
    'Sent when a support ticket is updated',
    'EMAIL', 'ticket',
    'Ticket #{{.ticket_number}} updated',
    'Hi {{.customer_name}}, your support ticket has been updated.

Status: {{.ticket_status}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Ticket updated</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">#{{.ticket_number}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your support ticket has been updated.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Status</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.ticket_status}}</p>
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
    '{"customer_name":"","ticket_number":"","ticket_status":"","ticket_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 10. ticket-resolved ──────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Ticket Resolved',
    'ticket-resolved',
    'Sent when a support ticket is resolved',
    'EMAIL', 'ticket',
    'Ticket #{{.ticket_number}} resolved',
    'Hi {{.customer_name}}, your support ticket has been resolved.

Resolution: {{.resolution}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Ticket resolved</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">#{{.ticket_number}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your support ticket has been resolved.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Resolution</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.resolution}}</p>
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
    '{"customer_name":"","ticket_number":"","resolution":"","ticket_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 11. ticket-closed ────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Ticket Closed',
    'ticket-closed',
    'Sent when a support ticket is closed',
    'EMAIL', 'ticket',
    'Ticket #{{.ticket_number}} closed',
    'Hi {{.customer_name}}, your support ticket has been closed. If you need further help, feel free to open a new ticket.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Ticket closed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">#{{.ticket_number}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, your support ticket has been closed. If you need further help, feel free to open a new ticket.</p>
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
    '{"customer_name":"","ticket_number":"","store_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 12. vendor-welcome ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Vendor Welcome',
    'vendor-welcome',
    'Welcome email sent to newly registered vendors',
    'EMAIL', 'vendor',
    'Welcome to {{.store_name}}',
    'Hi {{.vendor_name}}, welcome aboard! Your vendor account has been created. You can start setting up your store.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Welcome aboard</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Vendor account</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.vendor_name}}, welcome aboard! Your vendor account has been created. You can start setting up your store.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.vendor_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Get started</a>
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
    '{"vendor_name":"","vendor_email":"","vendor_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 13. vendor-approved ──────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Vendor Approved',
    'vendor-approved',
    'Sent when a vendor application is approved',
    'EMAIL', 'vendor',
    'Your vendor application has been approved',
    'Hi {{.vendor_name}}, great news — your application has been approved! You can now start listing products.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Application approved</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Vendor account</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.vendor_name}}, great news — your application has been approved! You can now start listing products.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.vendor_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Go to dashboard</a>
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
    '{"vendor_name":"","vendor_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 14. vendor-rejected ──────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Vendor Rejected',
    'vendor-rejected',
    'Sent when a vendor application is rejected',
    'EMAIL', 'vendor',
    'Update on your vendor application',
    'Hi {{.vendor_name}}, unfortunately your application could not be approved at this time. Reason: {{.status_reason}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Application update</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Vendor account</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.vendor_name}}, unfortunately your application could not be approved at this time. Reason: {{.status_reason}}</p>
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
    '{"vendor_name":"","status_reason":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 15. vendor-suspended ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Vendor Suspended',
    'vendor-suspended',
    'Sent when a vendor account is suspended',
    'EMAIL', 'vendor',
    'Your vendor account has been suspended',
    'Hi {{.vendor_name}}, your vendor account has been suspended. Reason: {{.status_reason}}. Please contact support for more information.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Account suspended</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Vendor account</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.vendor_name}}, your vendor account has been suspended. Reason: {{.status_reason}}. Please contact support for more information.</p>
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
    '{"vendor_name":"","status_reason":"","support_email":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 16. coupon-applied ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Coupon Applied',
    'coupon-applied',
    'Sent when a coupon is applied to an order',
    'EMAIL', 'coupon',
    'Coupon applied — {{.discount_amount}} off',
    'Hi {{.customer_name}}, the coupon {{.coupon_code}} has been applied to your order.

Coupon: {{.coupon_code}}
Discount: {{.discount_amount}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Coupon applied</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.discount_amount}} off</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.customer_name}}, the coupon {{.coupon_code}} has been applied to your order.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Coupon</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.coupon_code}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Discount</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.discount_amount}}</p>
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
    '{"customer_name":"","coupon_code":"","discount_amount":"","order_value":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 17. coupon-expired ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Coupon Expired',
    'coupon-expired',
    'Sent to admin when a coupon expires',
    'EMAIL', 'coupon',
    'Coupon {{.coupon_code}} has expired',
    'The coupon {{.coupon_code}} has expired and is no longer valid.

Code: {{.coupon_code}}
Valid until: {{.valid_until}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Coupon expired</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.coupon_code}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">The coupon {{.coupon_code}} has expired and is no longer valid.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Code</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.coupon_code}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Valid until</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.valid_until}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.coupons_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage coupons</a>
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
    '{"coupon_code":"","valid_until":"","coupons_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 18. approval-escalated ───────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Escalated',
    'approval-escalated',
    'Sent when an approval request is escalated',
    'EMAIL', 'approval',
    'Approval escalated — {{.action_type_display}}',
    'An approval request has been escalated to you for review.

Action: {{.action_type_display}}
Resource: {{.resource_type}} {{.resource_id}}
Requested by: {{.requester_name}}
Priority: {{.approval_priority}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Approval escalated</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.action_type_display}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">An approval request has been escalated to you for review.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Action</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.action_type_display}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Resource</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.resource_type}} {{.resource_id}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Requested by</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.requester_name}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Priority</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.approval_priority}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.approval_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Review request</a>
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
    '{"action_type_display":"","resource_type":"","resource_id":"","requester_name":"","approval_priority":"","approval_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 19. approval-granted ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Granted',
    'approval-granted',
    'Sent when an approval request is granted',
    'EMAIL', 'approval',
    'Your request has been approved',
    'Hi {{.requester_name}}, your {{.action_type_display}} request has been approved by {{.approver_name}}.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Request approved</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.action_type_display}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.requester_name}}, your {{.action_type_display}} request has been approved by {{.approver_name}}.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Action</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.action_type_display}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Approved by</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.approver_name}}</p>
        {{if .approval_comment}}<p style="margin:14px 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Comment</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.approval_comment}}</p>{{end}}
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.approval_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View details</a>
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
    '{"requester_name":"","action_type_display":"","approver_name":"","approval_comment":"","approval_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 20. approval-rejected ────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Rejected',
    'approval-rejected',
    'Sent when an approval request is rejected',
    'EMAIL', 'approval',
    'Your request was not approved',
    'Hi {{.requester_name}}, your {{.action_type_display}} request was not approved by {{.approver_name}}.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Request not approved</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.action_type_display}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.requester_name}}, your {{.action_type_display}} request was not approved by {{.approver_name}}.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Action</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.action_type_display}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Rejected by</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.approver_name}}</p>
        {{if .approval_comment}}<p style="margin:14px 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Comment</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.approval_comment}}</p>{{end}}
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.approval_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View details</a>
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
    '{"requester_name":"","action_type_display":"","approver_name":"","approval_comment":"","approval_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 21. approval-cancelled ───────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Cancelled',
    'approval-cancelled',
    'Sent when an approval request is cancelled',
    'EMAIL', 'approval',
    'Approval request cancelled',
    'Hi {{.requester_name}}, your {{.action_type_display}} approval request has been cancelled.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Request cancelled</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.action_type_display}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.requester_name}}, your {{.action_type_display}} approval request has been cancelled.</p>
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
    '{"requester_name":"","action_type_display":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 22. approval-expired ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Approval Expired',
    'approval-expired',
    'Sent when an approval request expires',
    'EMAIL', 'approval',
    'Approval request expired',
    'Hi {{.requester_name}}, your {{.action_type_display}} approval request has expired. Please submit a new request if still needed.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Request expired</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.action_type_display}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.requester_name}}, your {{.action_type_display}} approval request has expired. Please submit a new request if still needed.</p>
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
    '{"requester_name":"","action_type_display":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 23. domain-added ─────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Added',
    'domain-added',
    'Sent when a custom domain is added',
    'EMAIL', 'domain',
    'Domain {{.domain}} added',
    'Hi {{.owner_name}}, the domain {{.domain}} has been added to your store. DNS verification is in progress.

Domain: {{.domain}}
Status: Pending verification

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Domain added</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, the domain {{.domain}} has been added to your store. DNS verification is in progress.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Domain</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.domain}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Status</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">Pending verification</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage domains</a>
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
    '{"owner_name":"","domain":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 24. domain-ssl-ready ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain SSL Ready',
    'domain-ssl-ready',
    'Sent when SSL is provisioned for a domain',
    'EMAIL', 'domain',
    'SSL certificate ready for {{.domain}}',
    'Hi {{.owner_name}}, an SSL certificate has been issued for {{.domain}}. Your domain is now secure.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">SSL certificate ready</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, an SSL certificate has been issued for {{.domain}}. Your domain is now secure.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View domain</a>
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
    '{"owner_name":"","domain":"","ssl_provider":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 25. domain-activated ─────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Activated',
    'domain-activated',
    'Sent when a domain is activated and live',
    'EMAIL', 'domain',
    'Domain {{.domain}} is live',
    'Hi {{.owner_name}}, your domain {{.domain}} is now active and serving traffic.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Domain is live</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, your domain {{.domain}} is now active and serving traffic.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="https://{{.domain}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Visit your store</a>
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
    '{"owner_name":"","domain":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 26. domain-failed ────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Failed',
    'domain-failed',
    'Sent when domain setup fails',
    'EMAIL', 'domain',
    'Domain setup failed for {{.domain}}',
    'Hi {{.owner_name}}, we were unable to complete the setup for {{.domain}}. Reason: {{.failure_reason}}

Domain: {{.domain}}
Error: {{.failure_reason}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Domain setup failed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, we were unable to complete the setup for {{.domain}}. Reason: {{.failure_reason}}</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Domain</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.domain}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Error</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.failure_reason}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage domains</a>
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
    '{"owner_name":"","domain":"","failure_reason":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 27. domain-removed ───────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Removed',
    'domain-removed',
    'Sent when a domain is removed',
    'EMAIL', 'domain',
    'Domain {{.domain}} removed',
    'Hi {{.owner_name}}, the domain {{.domain}} has been removed from your store.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Domain removed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, the domain {{.domain}} has been removed from your store.</p>
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
    '{"owner_name":"","domain":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 28. domain-migrated ──────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Migrated',
    'domain-migrated',
    'Sent when a domain is migrated to new infrastructure',
    'EMAIL', 'domain',
    'Domain {{.domain}} migrated',
    'Hi {{.owner_name}}, your domain {{.domain}} has been migrated. Reason: {{.migration_reason}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Domain migrated</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, your domain {{.domain}} has been migrated. Reason: {{.migration_reason}}</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">View domain</a>
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
    '{"owner_name":"","domain":"","migration_reason":"","migrated_from":"","migrated_to":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 29. domain-ssl-expiring ──────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain SSL Expiring',
    'domain-ssl-expiring',
    'Sent when an SSL certificate is about to expire',
    'EMAIL', 'domain',
    'SSL certificate expiring for {{.domain}}',
    'Hi {{.owner_name}}, the SSL certificate for {{.domain}} will expire on {{.ssl_expires_at}}. Renewal is in progress.

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">SSL certificate expiring</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, the SSL certificate for {{.domain}} will expire on {{.ssl_expires_at}}. Renewal is in progress.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage domains</a>
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
    '{"owner_name":"","domain":"","ssl_expires_at":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 30. domain-health-failed ─────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Domain Health Failed',
    'domain-health-failed',
    'Sent when domain health check fails',
    'EMAIL', 'domain',
    'Health check failed for {{.domain}}',
    'Hi {{.owner_name}}, the health check for {{.domain}} has failed. Our team is investigating.

Domain: {{.domain}}
Error: {{.failure_reason}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Health check failed</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">{{.domain}}</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.owner_name}}, the health check for {{.domain}} has failed. Our team is investigating.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Domain</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.domain}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Error</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.failure_reason}}</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.domain_settings_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Manage domains</a>
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
    '{"owner_name":"","domain":"","failure_reason":"","domain_settings_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 31. gift-card-purchaser ──────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Gift Card Purchase Confirmation',
    'gift-card-purchaser',
    'Sent to the purchaser after buying a gift card',
    'EMAIL', 'gift_card',
    'Your gift card purchase is confirmed',
    'Hi {{.purchaser_name}}, your gift card has been purchased and delivered to {{.recipient_name}}.

Code: {{.gift_card_code}}
Amount: {{.gift_card_balance}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Gift card purchased</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Purchase confirmation</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.purchaser_name}}, your gift card has been purchased and delivered to {{.recipient_name}}.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Code</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.gift_card_code}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Amount</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.gift_card_balance}}</p>
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
    '{"purchaser_name":"","recipient_name":"","gift_card_code":"","gift_card_balance":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 32. gift-card-activated ──────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Gift Card Activated',
    'gift-card-activated',
    'Sent when a gift card is activated',
    'EMAIL', 'gift_card',
    'Your gift card is ready to use',
    'Hi {{.recipient_name}}, your gift card has been activated and is ready to use.

Code: {{.gift_card_code}}
Balance: {{.gift_card_balance}}

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
      <h1 style="margin:0 0 4px;font-size:20px;font-weight:600;color:#18181b;">Gift card activated</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#71717a;">Ready to use</p>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.recipient_name}}, your gift card has been activated and is ready to use.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#fafafa;border-radius:8px;">
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Code</p>
        <p style="margin:0 0 14px;font-size:15px;font-weight:600;color:#18181b;">{{.gift_card_code}}</p>
        <p style="margin:0 0 2px;font-size:12px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.5px;">Balance</p>
        <p style="margin:0;font-size:15px;font-weight:600;color:#18181b;">{{.gift_card_balance}}</p>
      </td></tr></table>
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
    '{"recipient_name":"","gift_card_code":"","gift_card_balance":"","store_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 33. campaign-broadcast ───────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Campaign Broadcast',
    'campaign-broadcast',
    'Used for broadcast campaign emails',
    'EMAIL', 'campaign',
    '{{.campaign_name}}',
    '{{.campaign_name}} - {{.store_name}}',
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
      {{.campaign_body}}
      {{if .campaign_cta_url}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0 0;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.campaign_cta_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">{{.campaign_cta_text}}</a>
      </td></tr></table>{{end}}
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
  <p style="margin:8px 0 0;font-size:11px;color:#d4d4d8;"><a href="{{.unsubscribe_url}}" style="color:#a1a1aa;text-decoration:none;">Unsubscribe</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"campaign_name":"","campaign_body":"","campaign_cta_text":"","campaign_cta_url":"","unsubscribe_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 34. campaign-newsletter ──────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Campaign Newsletter',
    'campaign-newsletter',
    'Used for newsletter campaign emails',
    'EMAIL', 'campaign',
    '{{.campaign_name}}',
    '{{.campaign_name}} - {{.store_name}}',
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
      {{.campaign_body}}
      {{if .campaign_cta_url}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0 0;"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.campaign_cta_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">{{.campaign_cta_text}}</a>
      </td></tr></table>{{end}}
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">{{.store_name}}</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
  <p style="margin:8px 0 0;font-size:11px;color:#d4d4d8;"><a href="{{.unsubscribe_url}}" style="color:#a1a1aa;text-decoration:none;">Unsubscribe</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"campaign_name":"","campaign_body":"","campaign_cta_text":"","campaign_cta_url":"","unsubscribe_url":"","store_name":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ============================================================================
-- PART 4: Constraints
-- ============================================================================

-- Backfill any remaining NULL slugs as fallback
UPDATE notification_templates SET slug = LOWER(REPLACE(REPLACE(REPLACE(name, ' ', '-'), '(', ''), ')', '')) WHERE slug IS NULL;

ALTER TABLE notification_templates ALTER COLUMN slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_template_slug_tenant ON notification_templates(slug, tenant_id) WHERE deleted_at IS NULL;

COMMIT;
