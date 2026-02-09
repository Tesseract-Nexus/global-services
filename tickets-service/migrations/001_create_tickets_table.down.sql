-- Drop triggers
DROP TRIGGER IF EXISTS update_ticket_comments_updated_at ON ticket_comments;
DROP TRIGGER IF EXISTS update_tickets_updated_at ON tickets;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_ticket_attachments_uploaded_by;
DROP INDEX IF EXISTS idx_ticket_attachments_ticket_id;
DROP INDEX IF EXISTS idx_ticket_comments_created_at;
DROP INDEX IF EXISTS idx_ticket_comments_user_id;
DROP INDEX IF EXISTS idx_ticket_comments_ticket_id;
DROP INDEX IF EXISTS idx_tickets_metadata;
DROP INDEX IF EXISTS idx_tickets_tags;
DROP INDEX IF EXISTS idx_tickets_tenant_assignee;
DROP INDEX IF EXISTS idx_tickets_tenant_status;
DROP INDEX IF EXISTS idx_tickets_due_date;
DROP INDEX IF EXISTS idx_tickets_updated_at;
DROP INDEX IF EXISTS idx_tickets_created_at;
DROP INDEX IF EXISTS idx_tickets_created_by;
DROP INDEX IF EXISTS idx_tickets_assignee_id;
DROP INDEX IF EXISTS idx_tickets_priority;
DROP INDEX IF EXISTS idx_tickets_type;
DROP INDEX IF EXISTS idx_tickets_status;
DROP INDEX IF EXISTS idx_tickets_tenant_id;

-- Drop tables
DROP TABLE IF EXISTS ticket_attachments;
DROP TABLE IF EXISTS ticket_comments;
DROP TABLE IF EXISTS tickets;

-- Drop enum types
DROP TYPE IF EXISTS ticket_priority;
DROP TYPE IF EXISTS ticket_type;
DROP TYPE IF EXISTS ticket_status;