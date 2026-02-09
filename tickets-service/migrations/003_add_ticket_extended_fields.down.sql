-- Rollback extended fields from tickets table

-- Drop indexes
DROP INDEX IF EXISTS idx_tickets_history;
DROP INDEX IF EXISTS idx_tickets_inline_comments;
DROP INDEX IF EXISTS idx_tickets_inline_attachments;
DROP INDEX IF EXISTS idx_tickets_assignees;
DROP INDEX IF EXISTS idx_tickets_parent_ticket_id;

-- Drop columns
ALTER TABLE tickets DROP COLUMN IF EXISTS updated_by;
ALTER TABLE tickets DROP COLUMN IF EXISTS history;
ALTER TABLE tickets DROP COLUMN IF EXISTS sla;
ALTER TABLE tickets DROP COLUMN IF EXISTS comments;
ALTER TABLE tickets DROP COLUMN IF EXISTS attachments;
ALTER TABLE tickets DROP COLUMN IF EXISTS assignees;
ALTER TABLE tickets DROP COLUMN IF EXISTS parent_ticket_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS actual_time;
ALTER TABLE tickets DROP COLUMN IF EXISTS estimated_time;
