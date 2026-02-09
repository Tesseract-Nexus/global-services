-- Add extended fields to tickets table
-- These fields support advanced ticket management features

-- Time tracking fields
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS estimated_time INTEGER;
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS actual_time INTEGER;

-- Hierarchical ticket support (sub-tickets)
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS parent_ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL;

-- Multiple assignees support (JSONB array of assignee objects)
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS assignees JSONB DEFAULT '[]'::jsonb;

-- Inline attachments (JSONB array for quick access, complements ticket_attachments table)
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS attachments JSONB DEFAULT '[]'::jsonb;

-- Inline comments (JSONB array for quick access, complements ticket_comments table)
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS comments JSONB DEFAULT '[]'::jsonb;

-- SLA configuration and tracking
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS sla JSONB;

-- Ticket history/audit trail
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS history JSONB DEFAULT '[]'::jsonb;

-- Track who last updated the ticket
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS updated_by VARCHAR(255);

-- Create index for parent_ticket_id for efficient sub-ticket queries
CREATE INDEX IF NOT EXISTS idx_tickets_parent_ticket_id ON tickets(parent_ticket_id);

-- Create GIN indexes for JSONB columns for efficient querying
CREATE INDEX IF NOT EXISTS idx_tickets_assignees ON tickets USING GIN(assignees);
CREATE INDEX IF NOT EXISTS idx_tickets_inline_attachments ON tickets USING GIN(attachments);
CREATE INDEX IF NOT EXISTS idx_tickets_inline_comments ON tickets USING GIN(comments);
CREATE INDEX IF NOT EXISTS idx_tickets_history ON tickets USING GIN(history);
