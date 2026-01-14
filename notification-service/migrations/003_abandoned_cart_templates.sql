-- Abandoned Cart Reminder Templates

-- Abandoned Cart Reminder 1 (First reminder, gentle)
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'default-tenant',
    'abandoned_cart_reminder_1',
    'First abandoned cart reminder email',
    'EMAIL',
    'marketing',
    'You left something behind!',
    'Hi {{.customerName}},

You left some items in your shopping cart. Don''t let them get away!

Your Cart:
{{range .cartItems}}- {{.name}} ({{.quantity}}) - ${{.price}}
{{end}}
Total: ${{.cartTotal}}

Complete your purchase now: {{.cartRecoveryUrl}}

See you soon!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
<h1 style="color: #333;">You left something behind!</h1>
<p>Hi {{.customerName}},</p>
<p>You left some items in your shopping cart. Don''t let them get away!</p>

<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<h3 style="margin-top: 0;">Your Cart:</h3>
{{range .cartItems}}
<div style="display: flex; padding: 10px 0; border-bottom: 1px solid #ddd;">
{{if .image}}<img src="{{.image}}" alt="{{.name}}" style="width: 60px; height: 60px; object-fit: cover; border-radius: 4px; margin-right: 15px;">{{end}}
<div>
<p style="margin: 0; font-weight: bold;">{{.name}}</p>
<p style="margin: 5px 0; color: #666;">Qty: {{.quantity}} - ${{.price}}</p>
</div>
</div>
{{end}}
<p style="text-align: right; font-size: 18px; font-weight: bold; margin-top: 15px;">Total: ${{.cartTotal}}</p>
</div>

<div style="text-align: center; margin: 30px 0;">
<a href="{{.cartRecoveryUrl}}" style="background: #4CAF50; color: white; padding: 15px 30px; text-decoration: none; border-radius: 4px; font-size: 16px;">Complete Your Purchase</a>
</div>

<p>See you soon!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Abandoned Cart Reminder 2 (Second reminder, add urgency)
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'default-tenant',
    'abandoned_cart_reminder_2',
    'Second abandoned cart reminder email',
    'EMAIL',
    'marketing',
    'Still thinking about it?',
    'Hi {{.customerName}},

We noticed you haven''t completed your purchase yet. Your items are still waiting for you!

Your Cart:
{{range .cartItems}}- {{.name}} ({{.quantity}}) - ${{.price}}
{{end}}
Total: ${{.cartTotal}}

{{if .discountCode}}
Use code {{.discountCode}} for a special discount!
{{end}}

Complete your purchase now: {{.cartRecoveryUrl}}

Don''t miss out!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
<h1 style="color: #333;">Still thinking about it?</h1>
<p>Hi {{.customerName}},</p>
<p>We noticed you haven''t completed your purchase yet. Your items are still waiting for you!</p>

{{if .discountCode}}
<div style="background: #fff3cd; border: 2px dashed #ffc107; padding: 20px; border-radius: 8px; margin: 20px 0; text-align: center;">
<p style="margin: 0; font-size: 14px; color: #856404;">Special Offer!</p>
<p style="margin: 10px 0; font-size: 24px; font-weight: bold; color: #856404;">Use code: {{.discountCode}}</p>
</div>
{{end}}

<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<h3 style="margin-top: 0;">Your Cart:</h3>
{{range .cartItems}}
<div style="display: flex; padding: 10px 0; border-bottom: 1px solid #ddd;">
{{if .image}}<img src="{{.image}}" alt="{{.name}}" style="width: 60px; height: 60px; object-fit: cover; border-radius: 4px; margin-right: 15px;">{{end}}
<div>
<p style="margin: 0; font-weight: bold;">{{.name}}</p>
<p style="margin: 5px 0; color: #666;">Qty: {{.quantity}} - ${{.price}}</p>
</div>
</div>
{{end}}
<p style="text-align: right; font-size: 18px; font-weight: bold; margin-top: 15px;">Total: ${{.cartTotal}}</p>
</div>

<div style="text-align: center; margin: 30px 0;">
<a href="{{.cartRecoveryUrl}}" style="background: #ff9800; color: white; padding: 15px 30px; text-decoration: none; border-radius: 4px; font-size: 16px;">Complete Your Purchase</a>
</div>

<p>Don''t miss out!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;

-- Abandoned Cart Reminder 3 (Final reminder, last chance)
INSERT INTO notification_templates (
    id, tenant_id, name, description, channel, category,
    subject, body_template, html_template, is_active, is_system, version
) VALUES (
    gen_random_uuid(),
    'default-tenant',
    'abandoned_cart_reminder_3',
    'Final abandoned cart reminder email',
    'EMAIL',
    'marketing',
    'Last chance to complete your order!',
    'Hi {{.customerName}},

This is your last reminder! Your cart items won''t be saved forever.

Your Cart:
{{range .cartItems}}- {{.name}} ({{.quantity}}) - ${{.price}}
{{end}}
Total: ${{.cartTotal}}

{{if .discountCode}}
Use code {{.discountCode}} for a special discount - this is your last chance!
{{end}}

Complete your purchase now: {{.cartRecoveryUrl}}

We hope to see you soon!',
    '<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
<h1 style="color: #d32f2f;">Last chance to complete your order!</h1>
<p>Hi {{.customerName}},</p>
<p>This is your last reminder! Your cart items won''t be saved forever.</p>

{{if .discountCode}}
<div style="background: #ffebee; border: 2px solid #d32f2f; padding: 20px; border-radius: 8px; margin: 20px 0; text-align: center;">
<p style="margin: 0; font-size: 14px; color: #d32f2f;">FINAL OFFER!</p>
<p style="margin: 10px 0; font-size: 24px; font-weight: bold; color: #d32f2f;">Use code: {{.discountCode}}</p>
<p style="margin: 0; font-size: 12px; color: #666;">This is your last chance!</p>
</div>
{{end}}

<div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0;">
<h3 style="margin-top: 0;">Your Cart:</h3>
{{range .cartItems}}
<div style="display: flex; padding: 10px 0; border-bottom: 1px solid #ddd;">
{{if .image}}<img src="{{.image}}" alt="{{.name}}" style="width: 60px; height: 60px; object-fit: cover; border-radius: 4px; margin-right: 15px;">{{end}}
<div>
<p style="margin: 0; font-weight: bold;">{{.name}}</p>
<p style="margin: 5px 0; color: #666;">Qty: {{.quantity}} - ${{.price}}</p>
</div>
</div>
{{end}}
<p style="text-align: right; font-size: 18px; font-weight: bold; margin-top: 15px;">Total: ${{.cartTotal}}</p>
</div>

<div style="text-align: center; margin: 30px 0;">
<a href="{{.cartRecoveryUrl}}" style="background: #d32f2f; color: white; padding: 15px 30px; text-decoration: none; border-radius: 4px; font-size: 16px;">Complete Your Purchase Now</a>
</div>

<p>We hope to see you soon!</p>
</body>
</html>',
    true, true, 1
) ON CONFLICT DO NOTHING;
