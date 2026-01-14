-- Seed system notification templates
-- These templates are used by the NATS subscriber for event-driven notifications

-- Order Confirmation Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-confirmation',
    'Email sent when a new order is placed',
    'EMAIL',
    'orders',
    'Order Confirmed - #{{.orderNumber}}',
    'Hi {{.customerName}},

Thank you for your order!

Order Number: {{.orderNumber}}
Total: {{.currency}} {{.totalAmount | currency}}

We will notify you when your order ships.

Thank you for shopping with us!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Order Confirmed!</h1>
<p>Hi {{.customerName}},</p>
<p>Thank you for your order!</p>
<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
<p><strong>Total:</strong> {{.currency}} {{.totalAmount | currency}}</p>
</div>
<p>We will notify you when your order ships.</p>
<p>Thank you for shopping with us!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Order Shipped Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-shipped',
    'Email sent when an order is shipped',
    'EMAIL',
    'orders',
    'Your Order #{{.orderNumber}} Has Shipped!',
    'Hi {{.customerName}},

Great news! Your order has shipped!

Order Number: {{.orderNumber}}
Carrier: {{.carrierName}}
{{if .trackingUrl}}Track your package: {{.trackingUrl}}{{end}}

Thank you for shopping with us!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Your Order Has Shipped!</h1>
<p>Hi {{.customerName}},</p>
<p>Great news! Your order has shipped!</p>
<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
<p><strong>Carrier:</strong> {{.carrierName}}</p>
{{if .trackingUrl}}<p><a href="{{.trackingUrl}}" style="color: #0066cc;">Track Your Package</a></p>{{end}}
</div>
<p>Thank you for shopping with us!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Order Shipped SMS
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-shipped-sms',
    'SMS sent when an order is shipped',
    'SMS',
    'orders',
    '',
    'Your order #{{.orderNumber}} has shipped! {{if .trackingUrl}}Track: {{.trackingUrl}}{{end}}',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Order Delivered Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-delivered',
    'Email sent when an order is delivered',
    'EMAIL',
    'orders',
    'Your Order #{{.orderNumber}} Has Been Delivered!',
    'Hi {{.customerName}},

Your order has been delivered!

Order Number: {{.orderNumber}}

We hope you love your purchase. If you have any questions, please contact us.

Thank you for shopping with us!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Your Order Has Been Delivered!</h1>
<p>Hi {{.customerName}},</p>
<p>Your order has been delivered!</p>
<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
</div>
<p>We hope you love your purchase. If you have any questions, please contact us.</p>
<p>Thank you for shopping with us!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Order Delivered SMS
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-delivered-sms',
    'SMS sent when an order is delivered',
    'SMS',
    'orders',
    '',
    'Your order #{{.orderNumber}} has been delivered! Thank you for shopping with us.',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Order Cancelled Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'order-cancelled',
    'Email sent when an order is cancelled',
    'EMAIL',
    'orders',
    'Order #{{.orderNumber}} Has Been Cancelled',
    'Hi {{.customerName}},

Your order has been cancelled.

Order Number: {{.orderNumber}}

If you did not request this cancellation, please contact us immediately.

Thank you.',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Order Cancelled</h1>
<p>Hi {{.customerName}},</p>
<p>Your order has been cancelled.</p>
<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
</div>
<p>If you did not request this cancellation, please contact us immediately.</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Payment Confirmation Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'payment-confirmation',
    'Email sent when payment is received',
    'EMAIL',
    'orders',
    'Payment Received for Order #{{.orderNumber}}',
    'Hi {{.customerName}},

We have received your payment!

Order Number: {{.orderNumber}}
Amount: {{.currency}} {{.amount | currency}}
Payment Method: {{.provider}}

Thank you!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Payment Received!</h1>
<p>Hi {{.customerName}},</p>
<p>We have received your payment!</p>
<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
<p><strong>Amount:</strong> {{.currency}} {{.amount | currency}}</p>
<p><strong>Payment Method:</strong> {{.provider}}</p>
</div>
<p>Thank you!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Payment Failed Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'payment-failed',
    'Email sent when payment fails',
    'EMAIL',
    'orders',
    'Payment Failed for Order #{{.orderNumber}}',
    'Hi {{.customerName}},

Unfortunately, your payment could not be processed.

Order Number: {{.orderNumber}}
Amount: {{.currency}} {{.amount | currency}}

Please update your payment method and try again.

If you need assistance, please contact our support team.',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #cc0000;">Payment Failed</h1>
<p>Hi {{.customerName}},</p>
<p>Unfortunately, your payment could not be processed.</p>
<div style="background: #fff5f5; padding: 20px; border-radius: 8px; margin: 20px 0; border: 1px solid #ffcccc;">
<p><strong>Order Number:</strong> {{.orderNumber}}</p>
<p><strong>Amount:</strong> {{.currency}} {{.amount | currency}}</p>
</div>
<p>Please update your payment method and try again.</p>
<p>If you need assistance, please contact our support team.</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Payment Failed SMS
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'payment-failed-sms',
    'SMS sent when payment fails',
    'SMS',
    'orders',
    '',
    'Payment failed for order #{{.orderNumber}}. Please update your payment method.',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Welcome Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'welcome-email',
    'Email sent to new customers',
    'EMAIL',
    'marketing',
    'Welcome to Our Store!',
    'Hi {{.customerName}},

Welcome! We are excited to have you join us.

Your account has been created successfully. You can now browse our products and make purchases.

Happy shopping!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Welcome!</h1>
<p>Hi {{.customerName}},</p>
<p>We are excited to have you join us.</p>
<p>Your account has been created successfully. You can now browse our products and make purchases.</p>
<p>Happy shopping!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Password Reset Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'password-reset',
    'Email sent for password reset',
    'EMAIL',
    'security',
    'Reset Your Password',
    'Hi,

We received a request to reset your password.

Click the link below to reset your password:
{{.resetUrl}}

If you did not request this, please ignore this email.

This link will expire in 1 hour.',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Reset Your Password</h1>
<p>Hi,</p>
<p>We received a request to reset your password.</p>
<div style="text-align: center; margin: 30px 0;">
<a href="{{.resetUrl}}" style="background: #0066cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px;">Reset Password</a>
</div>
<p>If you did not request this, please ignore this email.</p>
<p style="color: #666; font-size: 12px;">This link will expire in 1 hour.</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Verification Code Email
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'verification-code',
    'Email sent with verification code',
    'EMAIL',
    'security',
    'Your Verification Code',
    'Hi,

Your verification code is: {{.verificationCode}}

This code will expire in 10 minutes.

If you did not request this code, please ignore this email.',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
<h1 style="color: #333;">Your Verification Code</h1>
<p>Hi,</p>
<p>Your verification code is:</p>
<div style="text-align: center; margin: 30px 0;">
<span style="background: #f5f5f5; padding: 16px 32px; font-size: 32px; letter-spacing: 8px; font-weight: bold; border-radius: 8px;">{{.verificationCode}}</span>
</div>
<p>This code will expire in 10 minutes.</p>
<p style="color: #666; font-size: 12px;">If you did not request this code, please ignore this email.</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Verification Code SMS
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'system',
    'verification-code-sms',
    'SMS sent with verification code',
    'SMS',
    'security',
    '',
    'Your verification code is {{.verificationCode}}. It expires in 10 minutes.',
    true, true, 1
) ON CONFLICT DO NOTHING;
