-- Remove seed data for tickets service
DELETE FROM ticket_attachments WHERE uploaded_by LIKE '%-1' OR uploaded_by LIKE '%-2' OR uploaded_by = 'performance-team';
DELETE FROM ticket_comments WHERE user_id LIKE '%-1' OR user_id LIKE '%-2' OR user_id IN ('customer-support', 'performance-team');
DELETE FROM tickets WHERE tenant_id = 'default-tenant';