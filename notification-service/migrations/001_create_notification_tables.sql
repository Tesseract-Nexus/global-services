-- Notification Service Database Schema

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    priority VARCHAR(20) DEFAULT 'NORMAL',

    -- Template information
    template_id UUID,
    template_name VARCHAR(255),

    -- Recipient information
    recipient_id UUID,
    recipient_email VARCHAR(255),
    recipient_phone VARCHAR(50),
    recipient_token TEXT,

    -- Message content
    subject VARCHAR(500),
    body TEXT,
    body_html TEXT,
    variables JSONB,
    metadata JSONB,

    -- Delivery tracking
    scheduled_for TIMESTAMP,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    failed_at TIMESTAMP,
    error_message TEXT,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,

    -- Provider information
    provider VARCHAR(100),
    provider_id VARCHAR(255),
    provider_data JSONB,

    -- Tracking
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    unsubscribed_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Notification templates table
CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    channel VARCHAR(20) NOT NULL,
    category VARCHAR(100),

    -- Template content
    subject VARCHAR(500),
    body_template TEXT,
    html_template TEXT,

    -- Template configuration
    variables JSONB,
    default_data JSONB,

    -- Versioning
    version INT DEFAULT 1,
    is_active BOOLEAN DEFAULT TRUE,
    is_system BOOLEAN DEFAULT FALSE,

    -- Metadata
    tags JSONB,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,

    UNIQUE(name, tenant_id, deleted_at)
);

-- Notification preferences table
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL,

    -- Channel preferences
    email_enabled BOOLEAN DEFAULT TRUE,
    sms_enabled BOOLEAN DEFAULT TRUE,
    push_enabled BOOLEAN DEFAULT TRUE,

    -- Category preferences
    marketing_enabled BOOLEAN DEFAULT TRUE,
    orders_enabled BOOLEAN DEFAULT TRUE,
    security_enabled BOOLEAN DEFAULT TRUE,

    -- Contact information
    email VARCHAR(255),
    phone VARCHAR(50),

    -- Push tokens
    push_tokens JSONB,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(user_id, tenant_id)
);

-- Notification logs table
CREATE TABLE IF NOT EXISTS notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL,
    event VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
    message TEXT,
    data JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Notification batches table
CREATE TABLE IF NOT EXISTS notification_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    template_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL,

    -- Batch stats
    total_count INT DEFAULT 0,
    sent_count INT DEFAULT 0,
    failed_count INT DEFAULT 0,

    status VARCHAR(50) DEFAULT 'PENDING',
    scheduled_for TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id ON notifications(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notifications_channel ON notifications(channel);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_template_id ON notifications(template_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_id ON notifications(recipient_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_email ON notifications(recipient_email);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_scheduled_for ON notifications(scheduled_for);
CREATE INDEX IF NOT EXISTS idx_notifications_deleted_at ON notifications(deleted_at);

CREATE INDEX IF NOT EXISTS idx_templates_tenant_id ON notification_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_templates_category ON notification_templates(category);
CREATE INDEX IF NOT EXISTS idx_templates_channel ON notification_templates(channel);
CREATE INDEX IF NOT EXISTS idx_templates_deleted_at ON notification_templates(deleted_at);

CREATE INDEX IF NOT EXISTS idx_preferences_tenant_id ON notification_preferences(tenant_id);
CREATE INDEX IF NOT EXISTS idx_preferences_user_id ON notification_preferences(user_id);

CREATE INDEX IF NOT EXISTS idx_logs_notification_id ON notification_logs(notification_id);
CREATE INDEX IF NOT EXISTS idx_logs_created_at ON notification_logs(created_at);

CREATE INDEX IF NOT EXISTS idx_batches_tenant_id ON notification_batches(tenant_id);
CREATE INDEX IF NOT EXISTS idx_batches_status ON notification_batches(status);
CREATE INDEX IF NOT EXISTS idx_batches_scheduled_for ON notification_batches(scheduled_for);

-- Insert system notification templates

-- Email Templates
INSERT INTO notification_templates (tenant_id, name, description, channel, category, subject, body_template, html_template, variables, is_system) VALUES
-- Onboarding
('default-tenant', 'onboarding_welcome', 'Welcome email after account creation', 'EMAIL', 'onboarding',
'Welcome to {{.CompanyName}}!',
'Hi {{.FirstName}},

Welcome to {{.CompanyName}}! We''re excited to have you on board.

Your account has been successfully created. You can now start exploring our platform.

{{if .OnboardingLink}}
Complete your onboarding: {{.OnboardingLink}}
{{end}}

Best regards,
The {{.CompanyName}} Team',
'<html><body><h1>Welcome to {{.CompanyName}}!</h1><p>Hi {{.FirstName}},</p><p>Welcome to {{.CompanyName}}! We''re excited to have you on board.</p>{{if .OnboardingLink}}<p><a href="{{.OnboardingLink}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Complete Onboarding</a></p>{{end}}<p>Best regards,<br>The {{.CompanyName}} Team</p></body></html>',
'{"CompanyName": "Company name", "FirstName": "User first name", "OnboardingLink": "Onboarding completion link"}',
TRUE),

('default-tenant', 'onboarding_verification', 'Email verification for onboarding', 'EMAIL', 'onboarding',
'Verify your email address',
'Hi {{.FirstName}},

Please verify your email address by clicking the link below:

{{.VerificationLink}}

This link will expire in {{.ExpiryHours}} hours.

If you didn''t request this, please ignore this email.

Best regards,
The {{.CompanyName}} Team',
'<html><body><h1>Verify Your Email</h1><p>Hi {{.FirstName}},</p><p>Please verify your email address by clicking the button below:</p><p><a href="{{.VerificationLink}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Verify Email</a></p><p><small>This link will expire in {{.ExpiryHours}} hours.</small></p></body></html>',
'{"CompanyName": "Company name", "FirstName": "User first name", "VerificationLink": "Verification URL", "ExpiryHours": "Link expiry hours"}',
TRUE),

-- Order notifications
('default-tenant', 'order_confirmation', 'Order confirmation email', 'EMAIL', 'orders',
'Order Confirmation - {{.OrderNumber}}',
'Hi {{.CustomerName}},

Thank you for your order! Your order has been confirmed.

Order Number: {{.OrderNumber}}
Order Date: {{.OrderDate}}
Total: {{.Currency}}{{.Total}}

{{range .Items}}
- {{.ProductName}} x {{.Quantity}} - {{$.Currency}}{{.Price}}
{{end}}

Shipping Address:
{{.ShippingAddress}}

You can track your order at: {{.TrackingLink}}

Best regards,
{{.CompanyName}}',
'<html><body><h1>Order Confirmation</h1><p>Hi {{.CustomerName}},</p><p>Thank you for your order!</p><table><tr><th>Order Number</th><td>{{.OrderNumber}}</td></tr><tr><th>Date</th><td>{{.OrderDate}}</td></tr><tr><th>Total</th><td>{{.Currency}}{{.Total}}</td></tr></table><h3>Items:</h3><ul>{{range .Items}}<li>{{.ProductName}} x {{.Quantity}} - {{$.Currency}}{{.Price}}</li>{{end}}</ul><p><a href="{{.TrackingLink}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Track Order</a></p></body></html>',
'{"CustomerName": "Customer name", "OrderNumber": "Order number", "OrderDate": "Order date", "Total": "Total amount", "Currency": "Currency symbol", "Items": "Array of order items", "ShippingAddress": "Shipping address", "TrackingLink": "Order tracking link", "CompanyName": "Company name"}',
TRUE),

('default-tenant', 'order_shipped', 'Order shipped notification', 'EMAIL', 'orders',
'Your Order Has Shipped - {{.OrderNumber}}',
'Hi {{.CustomerName}},

Great news! Your order {{.OrderNumber}} has been shipped.

Tracking Number: {{.TrackingNumber}}
Carrier: {{.Carrier}}
Estimated Delivery: {{.EstimatedDelivery}}

Track your shipment: {{.TrackingLink}}

Best regards,
{{.CompanyName}}',
'<html><body><h1>Your Order Has Shipped!</h1><p>Hi {{.CustomerName}},</p><p>Great news! Your order <strong>{{.OrderNumber}}</strong> has been shipped.</p><table><tr><th>Tracking Number</th><td>{{.TrackingNumber}}</td></tr><tr><th>Carrier</th><td>{{.Carrier}}</td></tr><tr><th>Estimated Delivery</th><td>{{.EstimatedDelivery}}</td></tr></table><p><a href="{{.TrackingLink}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Track Shipment</a></p></body></html>',
'{"CustomerName": "Customer name", "OrderNumber": "Order number", "TrackingNumber": "Shipping tracking number", "Carrier": "Shipping carrier", "EstimatedDelivery": "Estimated delivery date", "TrackingLink": "Tracking URL", "CompanyName": "Company name"}',
TRUE),

-- Return notifications
('default-tenant', 'return_approved', 'Return request approved', 'EMAIL', 'returns',
'Return Approved - RMA {{.RMANumber}}',
'Hi {{.CustomerName}},

Your return request has been approved.

RMA Number: {{.RMANumber}}
Order Number: {{.OrderNumber}}
Refund Amount: {{.Currency}}{{.RefundAmount}}

{{if .ReturnLabel}}
Download return label: {{.ReturnLabel}}
{{end}}

Please ship the items back to us within 7 days.

Best regards,
{{.CompanyName}}',
'<html><body><h1>Return Approved</h1><p>Hi {{.CustomerName}},</p><p>Your return request has been approved.</p><table><tr><th>RMA Number</th><td>{{.RMANumber}}</td></tr><tr><th>Order Number</th><td>{{.OrderNumber}}</td></tr><tr><th>Refund Amount</th><td>{{.Currency}}{{.RefundAmount}}</td></tr></table>{{if .ReturnLabel}}<p><a href="{{.ReturnLabel}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Download Return Label</a></p>{{end}}</body></html>',
'{"CustomerName": "Customer name", "RMANumber": "RMA number", "OrderNumber": "Order number", "RefundAmount": "Refund amount", "Currency": "Currency symbol", "ReturnLabel": "Return shipping label URL", "CompanyName": "Company name"}',
TRUE),

('default-tenant', 'return_completed', 'Return completed and refunded', 'EMAIL', 'returns',
'Refund Processed - RMA {{.RMANumber}}',
'Hi {{.CustomerName}},

Your return has been processed and your refund has been issued.

RMA Number: {{.RMANumber}}
Refund Amount: {{.Currency}}{{.RefundAmount}}
Refund Method: {{.RefundMethod}}

You should see the refund in 3-5 business days.

Best regards,
{{.CompanyName}}',
'<html><body><h1>Refund Processed</h1><p>Hi {{.CustomerName}},</p><p>Your return has been processed and your refund has been issued.</p><table><tr><th>RMA Number</th><td>{{.RMANumber}}</td></tr><tr><th>Refund Amount</th><td>{{.Currency}}{{.RefundAmount}}</td></tr><tr><th>Refund Method</th><td>{{.RefundMethod}}</td></tr></table><p><small>You should see the refund in 3-5 business days.</small></p></body></html>',
'{"CustomerName": "Customer name", "RMANumber": "RMA number", "RefundAmount": "Refund amount", "Currency": "Currency symbol", "RefundMethod": "Refund method", "CompanyName": "Company name"}',
TRUE),

-- Auth notifications
('default-tenant', 'password_reset', 'Password reset request', 'EMAIL', 'auth',
'Reset Your Password',
'Hi {{.Name}},

We received a request to reset your password.

Click here to reset: {{.ResetLink}}

This link will expire in {{.ExpiryHours}} hours.

If you didn''t request this, please ignore this email.

Best regards,
{{.CompanyName}}',
'<html><body><h1>Reset Your Password</h1><p>Hi {{.Name}},</p><p>We received a request to reset your password.</p><p><a href="{{.ResetLink}}" style="background:#4F46E5;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;margin:20px 0;">Reset Password</a></p><p><small>This link will expire in {{.ExpiryHours}} hours.</small></p></body></html>',
'{"Name": "User name", "ResetLink": "Password reset URL", "ExpiryHours": "Link expiry hours", "CompanyName": "Company name"}',
TRUE)

ON CONFLICT (name, tenant_id, deleted_at) DO NOTHING;
