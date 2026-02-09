-- Seed data for tickets service
INSERT INTO tickets (
    id,
    tenant_id,
    application_id,
    title,
    description,
    type,
    status,
    priority,
    assignee_id,
    assignee_name,
    created_by,
    created_by_name,
    tags,
    metadata,
    due_date,
    resolved_at,
    created_at,
    updated_at
) VALUES 
-- Bug Reports
(
    uuid_generate_v4(),
    'default-tenant',
    'ecommerce-app',
    'Login button not responding on mobile',
    'Users report that the login button on mobile devices (iOS and Android) sometimes does not respond to touches. This affects approximately 5% of mobile users according to our analytics.',
    'BUG',
    'IN_PROGRESS',
    'HIGH',
    'dev-team-lead',
    'Sarah Johnson',
    'user-mobile-1',
    'Mobile User',
    '["mobile", "login", "ui", "critical-path"]',
    '{"browser": "Mobile Safari", "os": "iOS 17", "app_version": "1.2.3", "reproduction_rate": "5%", "affected_users": 150}',
    NOW() + INTERVAL '2 days',
    NULL,
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '1 day'
),
(
    uuid_generate_v4(),
    'default-tenant',
    'ecommerce-app',
    'Shopping cart items disappearing',
    'Several users have reported that items in their shopping cart disappear when they navigate between pages. This seems to occur more frequently during high traffic periods.',
    'BUG',
    'OPEN',
    'CRITICAL',
    NULL,
    NULL,
    'user-shop-2',
    'John Smith',
    '["cart", "session", "high-priority"]',
    '{"sessions_affected": 45, "revenue_impact": "$2300", "first_reported": "2024-01-20", "browser_data": {"chrome": 60, "firefox": 20, "safari": 20}}',
    NOW() + INTERVAL '1 day',
    NULL,
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days'
),

-- Feature Requests
(
    uuid_generate_v4(),
    'default-tenant',
    'admin-app',
    'Add bulk export functionality for user data',
    'Admin users need the ability to export user data in bulk for compliance and reporting purposes. Should support CSV, JSON, and PDF formats with filtering capabilities.',
    'FEATURE',
    'OPEN',
    'MEDIUM',
    'backend-dev-1',
    'Mike Wilson',
    'admin-user-1',
    'Admin Manager',
    '["export", "admin", "compliance", "reporting"]',
    '{"requested_formats": ["CSV", "JSON", "PDF"], "estimated_effort": "5 days", "compliance_requirement": true}',
    NOW() + INTERVAL '2 weeks',
    NULL,
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '5 days'
),
(
    uuid_generate_v4(),
    'default-tenant',
    'reviews-app',
    'Implement review sentiment analysis',
    'Add automatic sentiment analysis to reviews to help merchants understand customer feedback trends. Should integrate with existing review moderation system.',
    'FEATURE',
    'IN_PROGRESS',
    'LOW',
    'ml-engineer-1',
    'Alice Chen',
    'product-manager-1',
    'Product Manager',
    '["ml", "sentiment", "reviews", "automation"]',
    '{"ml_model": "sentiment-analysis-v2", "integration_points": ["moderation", "analytics"], "expected_accuracy": "85%"}',
    NOW() + INTERVAL '3 weeks',
    NULL,
    NOW() - INTERVAL '1 week',
    NOW() - INTERVAL '3 days'
),

-- Support Requests
(
    uuid_generate_v4(),
    'default-tenant',
    'payment-service',
    'Customer unable to process payment',
    'Customer ID: CUST-12345 is unable to complete payment for order #ORD-98765. Payment gateway returns error code 4001. Customer has tried multiple cards.',
    'SUPPORT',
    'RESOLVED',
    'HIGH',
    'support-lead',
    'Emma Davis',
    'customer-support',
    'Support Agent',
    '["payment", "gateway", "customer-issue", "urgent"]',
    '{"customer_id": "CUST-12345", "order_id": "ORD-98765", "error_code": "4001", "payment_attempts": 3, "resolution": "Gateway configuration updated"}',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '3 hours'
),
(
    uuid_generate_v4(),
    'default-tenant',
    'user-service',
    'User account locked after password reset',
    'Multiple users are reporting that their accounts get locked after attempting to reset their password. The reset email is received but clicking the link results in account lockout.',
    'SUPPORT',
    'OPEN',
    'MEDIUM',
    'security-team',
    'Security Team',
    'customer-support',
    'Support Agent',
    '["password-reset", "account-lockout", "security"]',
    '{"affected_users": 8, "pattern_identified": true, "security_review_required": true}',
    NOW() + INTERVAL '3 days',
    NULL,
    NOW() - INTERVAL '6 hours',
    NOW() - INTERVAL '6 hours'
),

-- Tasks and Improvements
(
    uuid_generate_v4(),
    'default-tenant',
    'infrastructure',
    'Update SSL certificates for production servers',
    'SSL certificates for production servers expire next month. Need to coordinate with DevOps to update certificates and test all endpoints.',
    'TASK',
    'OPEN',
    'MEDIUM',
    'devops-lead',
    'DevOps Lead',
    'security-team',
    'Security Team',
    '["ssl", "certificates", "production", "maintenance"]',
    '{"expiry_date": "2024-02-28", "servers_affected": 12, "downtime_window": "2024-02-15 02:00-04:00 UTC"}',
    NOW() + INTERVAL '1 week',
    NULL,
    NOW() - INTERVAL '4 days',
    NOW() - INTERVAL '4 days'
),
(
    uuid_generate_v4(),
    'default-tenant',
    'performance',
    'Optimize database queries for review listing',
    'The review listing page is experiencing slow load times when displaying large numbers of reviews. Need to optimize database queries and consider implementing caching.',
    'IMPROVEMENT',
    'CLOSED',
    'LOW',
    'backend-dev-2',
    'Tom Anderson',
    'performance-team',
    'Performance Team',
    '["database", "optimization", "caching", "performance"]',
    '{"load_time_before": "3.2s", "load_time_after": "0.8s", "queries_optimized": 5, "caching_implemented": true}',
    NOW() - INTERVAL '2 weeks',
    NOW() - INTERVAL '1 week',
    NOW() - INTERVAL '2 weeks',
    NOW() - INTERVAL '1 week'
);

-- Insert ticket comments
INSERT INTO ticket_comments (
    id,
    ticket_id,
    user_id,
    user_name,
    content,
    is_internal,
    created_at,
    updated_at
) VALUES
-- Comments for mobile login bug
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Login button not responding on mobile' LIMIT 1),
    'dev-team-lead',
    'Sarah Johnson',
    'I''ve reproduced this issue on iOS Safari. The touch event handler seems to be conflicting with the CSS hover state. Working on a fix.',
    true,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
),
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Login button not responding on mobile' LIMIT 1),
    'qa-tester-1',
    'QA Tester',
    'Confirmed on Android Chrome as well. Issue is intermittent but reproducible with rapid tapping.',
    true,
    NOW() - INTERVAL '18 hours',
    NOW() - INTERVAL '18 hours'
),

-- Comments for payment issue (resolved)
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Customer unable to process payment' LIMIT 1),
    'support-lead',
    'Emma Davis',
    'Identified the issue - payment gateway configuration was outdated. Updated the API version and tested successfully.',
    false,
    NOW() - INTERVAL '4 hours',
    NOW() - INTERVAL '4 hours'
),
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Customer unable to process payment' LIMIT 1),
    'customer-support',
    'Support Agent',
    'Customer confirmed that payment now works. Order was successfully processed.',
    false,
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '3 hours'
);

-- Insert ticket attachments
INSERT INTO ticket_attachments (
    id,
    ticket_id,
    filename,
    original_filename,
    content_type,
    file_size,
    file_path,
    uploaded_by,
    created_at
) VALUES
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Login button not responding on mobile' LIMIT 1),
    'mobile_login_screenshot.png',
    'Screenshot 2024-01-23 at 2.30.45 PM.png',
    'image/png',
    245760,
    '/uploads/tickets/mobile_login_screenshot.png',
    'user-mobile-1',
    NOW() - INTERVAL '3 days'
),
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Shopping cart items disappearing' LIMIT 1),
    'cart_error_logs.txt',
    'cart_error_logs_20240123.txt',
    'text/plain',
    8192,
    '/uploads/tickets/cart_error_logs.txt',
    'user-shop-2',
    NOW() - INTERVAL '2 days'
),
(
    uuid_generate_v4(),
    (SELECT id FROM tickets WHERE title = 'Optimize database queries for review listing' LIMIT 1),
    'performance_report.pdf',
    'DB Performance Analysis Report.pdf',
    'application/pdf',
    1024000,
    '/uploads/tickets/performance_report.pdf',
    'performance-team',
    NOW() - INTERVAL '2 weeks'
);