-- Onboarding Service Database Schema
-- Multi-tenant onboarding for all business applications

-- Create database (run separately)
-- CREATE DATABASE onboarding_db;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Onboarding templates (application type specific flows)
CREATE TABLE onboarding_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    application_type VARCHAR(100) NOT NULL, -- 'ecommerce', 'saas', 'marketplace', 'b2b'
    version INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    template_config JSONB NOT NULL DEFAULT '{}',
    steps JSONB NOT NULL DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT unique_default_per_app_type 
        EXCLUDE (application_type WITH =) WHERE (is_default = true)
);

-- Onboarding sessions (main tracking entity)
CREATE TABLE onboarding_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID, -- Will be null until tenant is created
    template_id UUID NOT NULL REFERENCES onboarding_templates(id),
    application_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) DEFAULT 'started', -- 'started', 'in_progress', 'completed', 'failed', 'abandoned'
    current_step VARCHAR(100),
    progress_percentage INTEGER DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '7 days'),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Business information
CREATE TABLE business_information (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    business_name VARCHAR(255) NOT NULL,
    business_type VARCHAR(100) NOT NULL, -- 'retail', 'wholesale', 'b2b', 'marketplace', 'services'
    industry VARCHAR(100) NOT NULL,
    business_description TEXT,
    website VARCHAR(500),
    registration_number VARCHAR(100),
    tax_id VARCHAR(100),
    incorporation_date DATE,
    employee_count VARCHAR(50), -- '1-10', '11-50', '51-200', '200+'
    annual_revenue VARCHAR(50), -- '<100k', '100k-1m', '1m-10m', '10m+'
    is_verified BOOLEAN DEFAULT false,
    verification_documents JSONB DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Contact information
CREATE TABLE contact_information (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    job_title VARCHAR(100),
    is_primary_contact BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Business address
CREATE TABLE business_addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    address_type VARCHAR(50) DEFAULT 'business', -- 'business', 'billing', 'shipping'
    street_address TEXT NOT NULL,
    city VARCHAR(100) NOT NULL,
    state_province VARCHAR(100) NOT NULL,
    postal_code VARCHAR(20) NOT NULL,
    country VARCHAR(100) NOT NULL,
    is_primary BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Verification records
CREATE TABLE verification_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    verification_type VARCHAR(50) NOT NULL, -- 'email', 'phone', 'business', 'identity'
    verification_method VARCHAR(50) NOT NULL, -- 'otp', 'link', 'document', 'manual'
    target_value VARCHAR(255) NOT NULL, -- email address, phone number, etc.
    verification_code VARCHAR(10),
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'verified', 'failed', 'expired'
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 5,
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '15 minutes'),
    verified_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Payment and subscription information
CREATE TABLE payment_information (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    subscription_plan VARCHAR(100), -- 'starter', 'professional', 'enterprise'
    billing_cycle VARCHAR(50), -- 'monthly', 'quarterly', 'yearly'
    payment_method VARCHAR(50), -- 'credit_card', 'bank_transfer', 'paypal'
    payment_provider VARCHAR(50), -- 'stripe', 'paypal', 'razorpay'
    payment_provider_customer_id VARCHAR(255),
    payment_provider_subscription_id VARCHAR(255),
    trial_end_date DATE,
    billing_address JSONB,
    payment_status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'active', 'failed', 'cancelled'
    setup_intent_id VARCHAR(255), -- Stripe setup intent
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Application-specific configuration
CREATE TABLE application_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    application_type VARCHAR(100) NOT NULL,
    configuration_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Task tracking
CREATE TABLE onboarding_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    task_id VARCHAR(100) NOT NULL, -- From template
    name VARCHAR(255) NOT NULL,
    description TEXT,
    task_type VARCHAR(100) NOT NULL, -- 'business_info', 'verification', 'payment', 'application_setup'
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'in_progress', 'completed', 'skipped', 'failed'
    is_required BOOLEAN DEFAULT true,
    order_index INTEGER NOT NULL,
    estimated_duration_minutes INTEGER,
    dependencies JSONB DEFAULT '[]', -- Array of task IDs
    completion_data JSONB DEFAULT '{}',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    skipped_at TIMESTAMP WITH TIME ZONE,
    skip_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT unique_task_per_session UNIQUE (onboarding_session_id, task_id)
);

-- Task execution logs
CREATE TABLE task_execution_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_task_id UUID NOT NULL REFERENCES onboarding_tasks(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL, -- 'started', 'completed', 'failed', 'retried'
    details JSONB DEFAULT '{}',
    error_message TEXT,
    performed_by VARCHAR(255), -- user_id or 'system'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Domain and subdomain tracking
CREATE TABLE domain_reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    domain_type VARCHAR(50) NOT NULL, -- 'subdomain', 'custom_domain'
    domain_value VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'reserved', -- 'reserved', 'verified', 'active', 'failed'
    verification_method VARCHAR(50), -- 'dns', 'file', 'meta_tag'
    verification_token VARCHAR(255),
    verified_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '24 hours'),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT unique_domain_value UNIQUE (domain_value)
);

-- Notification tracking
CREATE TABLE onboarding_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    notification_type VARCHAR(100) NOT NULL, -- 'email', 'sms', 'in_app'
    template_name VARCHAR(100) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    content TEXT,
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'sent', 'delivered', 'failed'
    provider VARCHAR(50), -- 'smtp', 'twilio', 'sendgrid'
    provider_message_id VARCHAR(255),
    sent_at TIMESTAMP WITH TIME ZONE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Webhook events for integration
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    onboarding_session_id UUID NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL, -- 'step_completed', 'onboarding_completed', 'payment_success'
    payload JSONB NOT NULL DEFAULT '{}',
    webhook_url VARCHAR(500),
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'sent', 'failed'
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    response_status INTEGER,
    response_body TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_onboarding_sessions_tenant_id ON onboarding_sessions(tenant_id);
CREATE INDEX idx_onboarding_sessions_status ON onboarding_sessions(status);
CREATE INDEX idx_onboarding_sessions_application_type ON onboarding_sessions(application_type);
CREATE INDEX idx_onboarding_sessions_expires_at ON onboarding_sessions(expires_at);

CREATE INDEX idx_business_information_session_id ON business_information(onboarding_session_id);
CREATE INDEX idx_contact_information_session_id ON contact_information(onboarding_session_id);
CREATE INDEX idx_contact_information_email ON contact_information(email);

CREATE INDEX idx_verification_records_session_id ON verification_records(onboarding_session_id);
CREATE INDEX idx_verification_records_type_status ON verification_records(verification_type, status);
CREATE INDEX idx_verification_records_expires_at ON verification_records(expires_at);

CREATE INDEX idx_payment_information_session_id ON payment_information(onboarding_session_id);
CREATE INDEX idx_payment_information_status ON payment_information(payment_status);

CREATE INDEX idx_onboarding_tasks_session_id ON onboarding_tasks(onboarding_session_id);
CREATE INDEX idx_onboarding_tasks_status ON onboarding_tasks(status);
CREATE INDEX idx_onboarding_tasks_order ON onboarding_tasks(onboarding_session_id, order_index);

CREATE INDEX idx_domain_reservations_domain_value ON domain_reservations(domain_value);
CREATE INDEX idx_domain_reservations_status ON domain_reservations(status);

CREATE INDEX idx_onboarding_notifications_session_id ON onboarding_notifications(onboarding_session_id);
CREATE INDEX idx_onboarding_notifications_status ON onboarding_notifications(status);

CREATE INDEX idx_webhook_events_session_id ON webhook_events(onboarding_session_id);
CREATE INDEX idx_webhook_events_status ON webhook_events(status);
CREATE INDEX idx_webhook_events_retry ON webhook_events(next_retry_at) WHERE status = 'failed';

-- Insert default onboarding templates
INSERT INTO onboarding_templates (name, description, application_type, is_default, template_config, steps) VALUES 
-- E-commerce template
(
    'E-commerce Store Setup',
    'Complete setup flow for online stores and e-commerce platforms',
    'ecommerce',
    true,
    '{"requires_payment": true, "requires_domain": true, "trial_period_days": 14}',
    '[
        {
            "id": "business-registration",
            "name": "Business Registration",
            "description": "Provide your business information and contact details",
            "type": "business_info",
            "required": true,
            "order": 1,
            "estimated_duration_minutes": 5,
            "fields": ["business_name", "business_type", "industry", "contact_info", "address"]
        },
        {
            "id": "email-verification",
            "name": "Email Verification",
            "description": "Verify your email address",
            "type": "verification",
            "required": true,
            "order": 2,
            "estimated_duration_minutes": 2,
            "dependencies": ["business-registration"]
        },
        {
            "id": "store-setup",
            "name": "Store Configuration",
            "description": "Configure your store settings",
            "type": "application_setup",
            "required": true,
            "order": 3,
            "estimated_duration_minutes": 8,
            "dependencies": ["email-verification"],
            "config": {
                "fields": ["store_name", "subdomain", "currency", "timezone", "categories", "shipping_zones"]
            }
        },
        {
            "id": "payment-setup",
            "name": "Payment & Billing",
            "description": "Set up your subscription and payment method",
            "type": "payment",
            "required": true,
            "order": 4,
            "estimated_duration_minutes": 5,
            "dependencies": ["store-setup"]
        },
        {
            "id": "theme-selection",
            "name": "Design & Theme",
            "description": "Choose and customize your store theme",
            "type": "application_setup",
            "required": false,
            "order": 5,
            "estimated_duration_minutes": 10,
            "dependencies": ["payment-setup"],
            "config": {
                "type": "theme_selection",
                "customization_options": ["colors", "fonts", "layout"]
            }
        },
        {
            "id": "launch-preparation",
            "name": "Launch Preparation",
            "description": "Final review and store activation",
            "type": "application_setup",
            "required": true,
            "order": 6,
            "estimated_duration_minutes": 3,
            "dependencies": ["payment-setup"],
            "config": {
                "type": "launch_review",
                "auto_activate": false
            }
        }
    ]'
),
-- SaaS template
(
    'SaaS Platform Setup',
    'Setup flow for SaaS applications and platforms',
    'saas',
    true,
    '{"requires_payment": true, "requires_domain": false, "trial_period_days": 30}',
    '[
        {
            "id": "business-registration",
            "name": "Business Registration",
            "description": "Provide your business information",
            "type": "business_info",
            "required": true,
            "order": 1,
            "estimated_duration_minutes": 5
        },
        {
            "id": "email-verification",
            "name": "Email Verification",
            "description": "Verify your email address",
            "type": "verification",
            "required": true,
            "order": 2,
            "estimated_duration_minutes": 2,
            "dependencies": ["business-registration"]
        },
        {
            "id": "workspace-setup",
            "name": "Workspace Setup",
            "description": "Configure your workspace and subdomain",
            "type": "application_setup",
            "required": true,
            "order": 3,
            "estimated_duration_minutes": 5,
            "dependencies": ["email-verification"],
            "config": {
                "fields": ["workspace_name", "subdomain", "timezone"]
            }
        },
        {
            "id": "team-setup",
            "name": "Team Configuration",
            "description": "Set up user roles and team structure",
            "type": "application_setup",
            "required": false,
            "order": 4,
            "estimated_duration_minutes": 8,
            "dependencies": ["workspace-setup"],
            "config": {
                "type": "team_management",
                "max_users": 10
            }
        },
        {
            "id": "payment-setup",
            "name": "Subscription Setup",
            "description": "Choose your plan and payment method",
            "type": "payment",
            "required": true,
            "order": 5,
            "estimated_duration_minutes": 5,
            "dependencies": ["workspace-setup"]
        }
    ]'
);

-- Add some sample data for development
-- This would typically be populated by the application
INSERT INTO onboarding_sessions (id, template_id, application_type, status, current_step) VALUES 
(
    '550e8400-e29b-41d4-a716-446655440000',
    (SELECT id FROM onboarding_templates WHERE application_type = 'ecommerce' AND is_default = true LIMIT 1),
    'ecommerce',
    'in_progress',
    'business-registration'
);