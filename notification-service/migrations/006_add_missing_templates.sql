-- Add missing templates: customer-goodbye, customer-otp, draft-reminder
-- These were previously rendered as hardcoded HTML in tenant-service and verification-service.

BEGIN;

-- ─── 1. customer-goodbye ────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Customer Goodbye',
    'customer-goodbye',
    'Sent when a customer deactivates their account',
    'EMAIL', 'customer',
    'We''re sorry to see you go from {{.store_name}}',
    'Hi {{.first_name}},

Your account at {{.store_name}} has been deactivated as requested.

Your data will be safely retained for 90 days. You can reactivate your account anytime before {{.purge_date}}.

After 90 days, your data will be permanently deleted.

Changed your mind? Simply log back in to reactivate: {{.reactivation_url}}

{{.store_name}}',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:{{if .brand_primary_color}}{{.brand_primary_color}}{{else}}#64748b{{end}};border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      {{if .brand_logo_url}}<img src="{{.brand_logo_url}}" alt="{{.store_name}}" height="28" style="height:28px;max-width:140px;">{{else}}<span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">{{.store_name}}</span>{{end}}
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 20px;font-size:20px;font-weight:600;color:#18181b;">We''re sorry to see you go</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.first_name}}, your account at <strong>{{.store_name}}</strong> has been deactivated as requested.</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 20px;"><tr><td style="padding:16px 20px;background:#fffbeb;border-left:3px solid #f59e0b;border-radius:0 8px 8px 0;">
        <p style="margin:0 0 8px;font-size:14px;font-weight:600;color:#92400e;">What happens next?</p>
        <p style="margin:0;font-size:14px;line-height:1.7;color:#92400e;">Your data will be safely retained for <strong>90 days</strong>. You can reactivate anytime before <strong>{{.purge_date}}</strong>. After that, your data will be permanently deleted.</p>
      </td></tr></table>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;"><tr><td style="padding:16px 20px;background:#ecfdf5;border-left:3px solid #10b981;border-radius:0 8px 8px 0;">
        <p style="margin:0 0 4px;font-size:14px;font-weight:600;color:#166534;">Changed your mind?</p>
        <p style="margin:0;font-size:14px;color:#166534;">Simply log back in to reactivate your account. All your data will be restored instantly.</p>
      </td></tr></table>
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;"><tr><td style="background:#10b981;border-radius:6px;">
        <a href="{{.reactivation_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Reactivate My Account</a>
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
    '{"first_name":"","store_name":"","purge_date":"","reactivation_url":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 2. customer-otp ────────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Customer OTP',
    'customer-otp',
    'OTP code sent to storefront customers for email verification',
    'EMAIL', 'customer',
    'Your verification code for {{.store_name}}',
    'Hi,

Your verification code for {{.store_name}} is: {{.verification_code}}

This code will expire in {{.expiry_minutes}} minutes.

If you did not request this code, please ignore this email.',
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
      <h1 style="margin:0 0 20px;font-size:20px;font-weight:600;color:#18181b;">Verify your email address</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#3f3f46;">Enter the code below to verify your email for <strong>{{.store_name}}</strong>.</p>
      <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto 24px;"><tr><td style="padding:16px 32px;background:#fafafa;border-radius:8px;border:1px solid #e4e4e7;">
        <span style="font-size:32px;font-weight:700;letter-spacing:8px;color:#18181b;font-family:''SF Mono'',''Fira Code'',''Fira Mono'',''Roboto Mono'',monospace;">{{.verification_code}}</span>
      </td></tr></table>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 20px;"><tr><td style="padding:12px 16px;background:#fffbeb;border-left:3px solid #f59e0b;border-radius:0 8px 8px 0;">
        <p style="margin:0;font-size:13px;color:#92400e;">This code expires in <strong>{{.expiry_minutes}} minutes</strong>.</p>
      </td></tr></table>
      <p style="margin:0;font-size:13px;color:#a1a1aa;">If you did not request this code, please ignore this email.</p>
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
    '{"verification_code":"","store_name":"","expiry_minutes":"10","email":"","brand_primary_color":"","brand_logo_url":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 3. draft-reminder ──────────────────────────────────────────────────────

INSERT INTO notification_templates (
    id, tenant_id, name, slug, description, channel, category,
    subject, body_template, html_template,
    variables, is_active, is_system, version
) VALUES (
    gen_random_uuid(), 'default-tenant',
    'Draft Reminder',
    'draft-reminder',
    'Reminder email for incomplete onboarding sessions',
    'EMAIL', 'tenant_onboarding',
    'Continue setting up your store',
    'Hi {{.first_name}},

It looks like you started setting up your store but haven''t finished yet.

Pick up right where you left off: {{.continue_url}}

If you need help, reply to this email and we''ll be happy to assist.',
    '<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;"><tr><td style="padding:48px 24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;margin:0 auto;">
<tr><td>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;border:1px solid #e4e4e7;">
    <tr><td style="height:3px;background:#18181b;border-radius:12px 12px 0 0;font-size:0;line-height:0;">&nbsp;</td></tr>
    <tr><td style="padding:32px 32px 0;text-align:center;">
      <span style="font-size:15px;font-weight:600;color:#18181b;letter-spacing:-.2px;">mark8ly</span>
    </td></tr>
    <tr><td style="padding:28px 32px 36px;">
      <h1 style="margin:0 0 20px;font-size:20px;font-weight:600;color:#18181b;">Continue setting up your store</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#3f3f46;">Hi {{.first_name}}, it looks like you started setting up your store but haven''t finished yet. Your progress has been saved — pick up right where you left off.</p>
      <table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#18181b;border-radius:6px;">
        <a href="{{.continue_url}}" style="display:inline-block;padding:11px 24px;font-size:14px;font-weight:500;color:#fff;text-decoration:none;">Continue Setup &rarr;</a>
      </td></tr></table>
      <p style="margin:20px 0 0;font-size:13px;color:#a1a1aa;">Need help? Reply to this email and we''ll be happy to assist.</p>
    </td></tr>
  </table>
</td></tr>
<tr><td style="padding:20px 0 0;text-align:center;">
  <p style="margin:0;font-size:12px;color:#a1a1aa;">mark8ly</p>
  <p style="margin:6px 0 0;font-size:11px;color:#d4d4d8;">Powered by <a href="https://mark8ly.com" style="color:#a1a1aa;text-decoration:none;">mark8ly</a></p>
</td></tr>
</table>
</td></tr></table>
</body></html>',
    '{"first_name":"","continue_url":"","session_id":""}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- ─── 4. verification-link (add slug to existing template) ───────────────────
-- The verification-link template was added in 005 but may not have a slug in
-- older environments. Ensure it exists by upserting.

UPDATE notification_templates
SET slug = 'verification-link'
WHERE name = 'Verification Link' AND slug IS NULL;

COMMIT;
