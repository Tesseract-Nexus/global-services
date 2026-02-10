-- 004_reseed_email_templates.sql
-- Clean reseed of all email templates with consistent branding, categories, and variables.
--
-- Fixes:
--   - Categories now match frontend: order, payment, customer, auth, etc. (was: orders, marketing, security)
--   - Variables use snake_case (was: camelCase)
--   - HTML uses Tesserix slate design system (#0F172A banner, slate palette)
--   - tenant_id consistently 'platform' for platform templates, 'default-tenant' for mark8ly
--   - Covers all 18 frontend categories (4 platform + 14 mark8ly)

BEGIN;

-- Wipe old seeds so we start clean
DELETE FROM notification_templates WHERE tenant_id IN ('system', 'default-tenant', 'platform');

-- ============================================================================
-- HELPER: Consistent HTML wrapper
-- All templates share the same structure:
--   - Slate-900 (#0F172A) banner
--   - White card on slate-50 background
--   - Slate-700 body text, slate-500 secondary text
--   - Consistent button styling
-- ============================================================================

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
    'Order Confirmed - #{{.order_id}}',
    'Hi {{.customer_name}},

Thank you for your order!

Order Number: {{.order_id}}
Total: {{.order_total}}

We will notify you when your order ships.

Thanks for shopping with {{.store_name}}!',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <!-- Banner -->
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <!-- Body -->
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Order Confirmed!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Thank you for your order. We''re getting it ready for you.</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Order Number</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:18px;font-weight:600;">{{.order_id}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Total</p>
      <p style="margin:0;color:#0F172A;font-size:18px;font-weight:600;">{{.order_total}}</p>
    </div>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">We''ll notify you when your order ships.</p>
    <a href="{{.store_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Visit Store</a>
  </td></tr>
  <!-- Footer -->
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Your Order #{{.order_id}} Has Shipped!',
    'Hi {{.customer_name}},

Great news! Your order has shipped.

Order: {{.order_id}}
Tracking: {{.tracking_number}}
Track: {{.tracking_url}}

Thanks for shopping with {{.store_name}}!',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Your Order Has Shipped!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Great news &mdash; your order is on its way!</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Order Number</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:18px;font-weight:600;">{{.order_id}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Tracking Number</p>
      <p style="margin:0;color:#0F172A;font-size:18px;font-weight:600;">{{.tracking_number}}</p>
    </div>
    <a href="{{.tracking_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Track Your Package</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Order #{{.order_id}} Has Been Cancelled',
    'Hi {{.customer_name}},

Your order #{{.order_id}} has been cancelled.

If you did not request this, please contact us.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Order Cancelled</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Your order has been cancelled.</p>
    <div style="background-color:#FEF2F2;border:1px solid #FECACA;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Order Number</p>
      <p style="margin:0;color:#DC2626;font-size:18px;font-weight:600;">{{.order_id}}</p>
    </div>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">If you did not request this cancellation, please contact us immediately.</p>
    <a href="{{.store_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Contact Support</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Payment Received - {{.payment_amount}}',
    'Hi {{.customer_name}},

We''ve received your payment of {{.payment_amount}}.

Transaction: {{.transaction_id}}
Method: {{.payment_method}}

Thank you!
{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Payment Received</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Your payment has been processed successfully.</p>
    <div style="background-color:#F0FDF4;border:1px solid #BBF7D0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Amount</p>
      <p style="margin:0 0 16px;color:#059669;font-size:20px;font-weight:700;">{{.payment_amount}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Transaction ID</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:14px;font-weight:600;">{{.transaction_id}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Payment Method</p>
      <p style="margin:0;color:#0F172A;font-size:14px;font-weight:600;">{{.payment_method}}</p>
    </div>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Payment Failed - Action Required',
    'Hi {{.customer_name}},

Your payment of {{.payment_amount}} could not be processed.

Please update your payment method and try again.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#DC2626;font-size:24px;font-weight:700;">Payment Failed</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Unfortunately, your payment of <strong>{{.payment_amount}}</strong> could not be processed.</p>
    <div style="background-color:#FEF2F2;border:1px solid #FECACA;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0;color:#DC2626;font-size:14px;font-weight:600;">Please update your payment method and try again.</p>
    </div>
    <a href="{{.store_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Update Payment</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Welcome to {{.store_name}}!',
    'Hi {{.customer_name}},

Welcome to {{.store_name}}! We''re excited to have you.

Start browsing: {{.store_url}}

Happy shopping!',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Welcome!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Thanks for joining {{.store_name}}. Your account is ready and you can start shopping right away.</p>
    <a href="{{.store_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Start Shopping</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Reset Your Password',
    'Hi {{.user_name}},

We received a request to reset your password.

Reset here: {{.reset_url}}

If you didn''t request this, ignore this email.

This link expires in 1 hour.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Reset Your Password</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">We received a request to reset your password. Click the button below to choose a new one.</p>
    <div style="text-align:center;margin:0 0 24px;">
      <a href="{{.reset_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Reset Password</a>
    </div>
    <p style="margin:0 0 8px;color:#64748B;font-size:14px;line-height:1.6;">If you didn''t request this, you can safely ignore this email.</p>
    <p style="margin:0;color:#94A3B8;font-size:12px;">This link expires in 1 hour.</p>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Verify Your Email',
    'Hi {{.user_name}},

Your verification code is: {{.verification_code}}

This code expires in 10 minutes.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Verify Your Email</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Use the code below to verify your email address.</p>
    <div style="text-align:center;margin:0 0 24px;">
      <span style="display:inline-block;background-color:#F1F5F9;padding:16px 32px;font-size:32px;letter-spacing:8px;font-weight:700;border-radius:8px;color:#0F172A;border:1px solid #E2E8F0;">{{.verification_code}}</span>
    </div>
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">This code expires in 10 minutes.</p>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'How was your purchase, {{.customer_name}}?',
    'Hi {{.customer_name}},

We hope you''re enjoying {{.product_name}}!

We''d love to hear your thoughts. Leave a review: {{.review_url}}

Thanks!
{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">How was your purchase?</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">We hope you''re enjoying <strong>{{.product_name}}</strong>. Your feedback helps other shoppers and helps us improve.</p>
    <div style="text-align:center;margin:0 0 24px;">
      <a href="{{.review_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Write a Review</a>
    </div>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Ticket #{{.ticket_id}} - {{.ticket_subject}}',
    'Hi {{.customer_name}},

Your support ticket has been created.

Ticket: {{.ticket_id}}
Subject: {{.ticket_subject}}

We''ll get back to you shortly.

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Ticket Created</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">We''ve received your support request and will get back to you shortly.</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Ticket ID</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.ticket_id}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Subject</p>
      <p style="margin:0;color:#0F172A;font-size:16px;font-weight:600;">{{.ticket_subject}}</p>
    </div>
    <a href="{{.ticket_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">View Ticket</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'New Vendor Application - {{.vendor_name}}',
    'Hello,

A new vendor has applied to {{.store_name}}.

Vendor: {{.vendor_name}} ({{.vendor_email}})

Review: {{.action_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">New Vendor Application</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">A new vendor has applied to sell on your marketplace.</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Vendor</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.vendor_name}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Email</p>
      <p style="margin:0;color:#0F172A;font-size:14px;">{{.vendor_email}}</p>
    </div>
    <a href="{{.action_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Review Application</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'You''ve got a discount! Code: {{.coupon_code}}',
    'Hi {{.customer_name}},

Great news! Use code {{.coupon_code}} to get {{.discount_amount}} off.

Expires: {{.expiry_date}}

Shop now: {{.store_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">You''ve Got a Discount!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.customer_name}},</p>
    <div style="background-color:#F0FDF4;border:2px dashed #10B981;border-radius:8px;padding:24px;margin:0 0 24px;text-align:center;">
      <p style="margin:0 0 8px;color:#059669;font-size:14px;font-weight:600;">YOUR CODE</p>
      <p style="margin:0 0 8px;color:#0F172A;font-size:28px;font-weight:700;letter-spacing:2px;">{{.coupon_code}}</p>
      <p style="margin:0;color:#059669;font-size:16px;font-weight:600;">{{.discount_amount}} off</p>
    </div>
    <p style="margin:0 0 24px;color:#64748B;font-size:14px;text-align:center;">Expires {{.expiry_date}}</p>
    <div style="text-align:center;">
      <a href="{{.store_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Shop Now</a>
    </div>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Low Stock Alert: {{.product_name}}',
    'Low stock warning for {{.product_name}} (SKU: {{.sku}}).

Current stock: {{.current_stock}} (threshold: {{.threshold}})

Manage inventory: {{.inventory_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#D97706;font-size:24px;font-weight:700;">Low Stock Alert</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">A product has fallen below the low-stock threshold.</p>
    <div style="background-color:#FFFBEB;border:1px solid #FDE68A;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Product</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.product_name}} ({{.sku}})</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Current Stock / Threshold</p>
      <p style="margin:0;color:#D97706;font-size:20px;font-weight:700;">{{.current_stock}} / {{.threshold}}</p>
    </div>
    <a href="{{.inventory_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Manage Inventory</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Approval Needed: {{.item_name}}',
    '{{.requester_name}} has submitted "{{.item_name}}" for approval.

Type: {{.approval_type}}
Review: {{.approval_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Approval Required</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;"><strong>{{.requester_name}}</strong> has submitted an item for your approval.</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Item</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.item_name}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Type</p>
      <p style="margin:0;color:#0F172A;font-size:14px;">{{.approval_type}}</p>
    </div>
    <a href="{{.approval_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Review Now</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Domain Verified: {{.domain_name}}',
    'Great news! Your domain {{.domain_name}} has been verified and is now active.

DNS: {{.dns_status}}
SSL: {{.ssl_status}}

Manage domains: {{.settings_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#059669;font-size:24px;font-weight:700;">Domain Verified</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Your custom domain is now active.</p>
    <div style="background-color:#F0FDF4;border:1px solid #BBF7D0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Domain</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.domain_name}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">DNS / SSL</p>
      <p style="margin:0;color:#059669;font-size:14px;font-weight:600;">{{.dns_status}} / {{.ssl_status}}</p>
    </div>
    <a href="{{.settings_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Manage Domains</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'Campaign Live: {{.campaign_name}}',
    'Your campaign "{{.campaign_name}}" is now live!

Runs: {{.start_date}} - {{.end_date}}

View: {{.campaign_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Campaign Live!</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">Your campaign is now active and reaching customers.</p>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Campaign</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.campaign_name}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Duration</p>
      <p style="margin:0;color:#0F172A;font-size:14px;">{{.start_date}} &mdash; {{.end_date}}</p>
    </div>
    <a href="{{.campaign_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">View Campaign</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    'You''ve received a gift card from {{.sender_name}}!',
    'Hi {{.recipient_name}},

{{.sender_name}} has sent you a {{.gift_card_amount}} gift card!

Code: {{.gift_card_code}}
Message: {{.gift_card_message}}

Redeem: {{.redeem_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">You''ve Got a Gift Card!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.recipient_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;"><strong>{{.sender_name}}</strong> sent you a gift card.</p>
    <div style="background-color:#F8FAFC;border:2px dashed #E2E8F0;border-radius:8px;padding:24px;margin:0 0 16px;text-align:center;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Gift Card Value</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:28px;font-weight:700;">{{.gift_card_amount}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Code</p>
      <p style="margin:0;color:#0F172A;font-size:18px;font-weight:600;letter-spacing:2px;">{{.gift_card_code}}</p>
    </div>
    <p style="margin:0 0 24px;color:#64748B;font-size:14px;text-align:center;font-style:italic;">&ldquo;{{.gift_card_message}}&rdquo;</p>
    <div style="text-align:center;">
      <a href="{{.redeem_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Redeem Now</a>
    </div>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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

{{.inviter_name}} has invited you to join {{.store_name}} as {{.role}}.

Accept: {{.invite_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#0F172A{{end}};padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">{{.store_name}}</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">You''re Invited!</h1>
    <p style="margin:0 0 16px;color:#334155;font-size:16px;line-height:1.6;">Hi {{.staff_name}},</p>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;"><strong>{{.inviter_name}}</strong> has invited you to join <strong>{{.store_name}}</strong> as <strong>{{.role}}</strong>.</p>
    <div style="text-align:center;margin:0 0 24px;">
      <a href="{{.invite_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Accept Invitation</a>
    </div>
    <p style="margin:0;color:#94A3B8;font-size:12px;">This invitation will expire in 7 days.</p>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">{{.store_name}} &mdash; Powered by Tesserix</p>
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
    '[{{.alert_type}}] {{.service_name}} - {{.environment}}',
    'System Alert: {{.alert_type}}

Service: {{.service_name}}
Environment: {{.environment}}
Time: {{.timestamp}}

{{.alert_message}}

Dashboard: {{.dashboard_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:#0F172A;padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix Platform" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">Tesserix Platform</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#DC2626;font-size:24px;font-weight:700;">System Health Alert</h1>
    <div style="background-color:#FEF2F2;border:1px solid #FECACA;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 12px;color:#64748B;font-size:14px;">Service</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.service_name}} ({{.environment}})</p>
      <p style="margin:0 0 12px;color:#64748B;font-size:14px;">Alert Type</p>
      <p style="margin:0 0 16px;color:#DC2626;font-size:16px;font-weight:600;">{{.alert_type}}</p>
      <p style="margin:0 0 12px;color:#64748B;font-size:14px;">Message</p>
      <p style="margin:0;color:#334155;font-size:14px;">{{.alert_message}}</p>
    </div>
    <p style="margin:0 0 24px;color:#94A3B8;font-size:12px;">{{.timestamp}}</p>
    <a href="{{.dashboard_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">View Dashboard</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">Tesserix Platform Alerts</p>
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
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:#0F172A;padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix Platform" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">Tesserix Platform</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">Audit Event</h1>
    <div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Actor</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:16px;font-weight:600;">{{.actor_name}} ({{.actor_email}})</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Action</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:14px;font-weight:600;font-family:monospace;">{{.action}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Resource</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:14px;">{{.resource_type}} {{.resource_id}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Details</p>
      <p style="margin:0;color:#334155;font-size:14px;">{{.details}}</p>
    </div>
    <p style="margin:0;color:#94A3B8;font-size:12px;">{{.timestamp}}</p>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">Tesserix Platform Alerts</p>
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
    'Security Alert: {{.event_type}}',
    'Security Event Detected

User: {{.user_name}} ({{.user_email}})
Event: {{.event_type}}
IP: {{.ip_address}}
Location: {{.location}}
Time: {{.timestamp}}

Take action: {{.action_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:#0F172A;padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix Platform" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">Tesserix Platform</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#DC2626;font-size:24px;font-weight:700;">Security Alert</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">A security event has been detected on the platform.</p>
    <div style="background-color:#FEF2F2;border:1px solid #FECACA;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Event</p>
      <p style="margin:0 0 16px;color:#DC2626;font-size:16px;font-weight:600;">{{.event_type}}</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">User</p>
      <p style="margin:0 0 16px;color:#0F172A;font-size:14px;">{{.user_name}} ({{.user_email}})</p>
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">IP / Location</p>
      <p style="margin:0;color:#0F172A;font-size:14px;">{{.ip_address}} &mdash; {{.location}}</p>
    </div>
    <p style="margin:0 0 24px;color:#94A3B8;font-size:12px;">{{.timestamp}}</p>
    <a href="{{.action_url}}" style="display:inline-block;background-color:#DC2626;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">Review Event</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">Tesserix Platform Alerts</p>
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
    'New Tenant: {{.details}}',
    'New tenant signup on Tesserix.

Type: {{.notification_type}}
Details: {{.details}}
Time: {{.timestamp}}

Dashboard: {{.dashboard_url}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:''Source Sans 3'',''Inter'',''Segoe UI'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#F8FAFC;">
<tr><td style="padding:40px 20px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
  <tr><td style="background-color:#0F172A;padding:24px 32px;border-radius:10px 10px 0 0;">
    {{if .platform_logo_url}}<img src="{{.platform_logo_url}}" alt="Tesserix Platform" style="max-height:32px;">{{else}}<p style="margin:0;color:#FFFFFF;font-size:18px;font-weight:600;">Tesserix Platform</p>{{end}}
  </td></tr>
  <tr><td style="background-color:#FFFFFF;padding:32px;border:1px solid #E2E8F0;border-top:none;">
    <h1 style="margin:0 0 16px;color:#0F172A;font-size:24px;font-weight:700;">New Tenant Signup</h1>
    <p style="margin:0 0 24px;color:#334155;font-size:16px;line-height:1.6;">A new tenant has signed up on the platform.</p>
    <div style="background-color:#F0FDF4;border:1px solid #BBF7D0;border-radius:8px;padding:20px;margin:0 0 24px;">
      <p style="margin:0 0 8px;color:#64748B;font-size:14px;">Details</p>
      <p style="margin:0;color:#0F172A;font-size:16px;font-weight:600;">{{.details}}</p>
    </div>
    <p style="margin:0 0 24px;color:#94A3B8;font-size:12px;">{{.timestamp}}</p>
    <a href="{{.dashboard_url}}" style="display:inline-block;background-color:#0F172A;color:#FFFFFF;padding:12px 24px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;">View Dashboard</a>
  </td></tr>
  <tr><td style="padding:24px 32px;border:1px solid #E2E8F0;border-top:none;border-radius:0 0 10px 10px;background-color:#FFFFFF;">
    <p style="margin:0;color:#94A3B8;font-size:12px;text-align:center;">Tesserix Platform Alerts</p>
  </td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"admin_name":"","admin_email":"","notification_type":"","details":"","timestamp":"","dashboard_url":"","platform_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

COMMIT;
