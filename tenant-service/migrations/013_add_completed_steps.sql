-- Migration: Add completed_steps field to track onboarding step completion
-- Purpose: Enable server-side validation to prevent step-skipping attacks
-- Created: 2026-02-12

-- Add completed_steps JSONB column to onboarding_sessions
ALTER TABLE onboarding_sessions
ADD COLUMN completed_steps JSONB DEFAULT '[]'::jsonb;

-- Add index on completed_steps for efficient queries
CREATE INDEX idx_onboarding_sessions_completed_steps ON onboarding_sessions USING gin(completed_steps);

-- Add comment
COMMENT ON COLUMN onboarding_sessions.completed_steps IS 'Array of completed step indices (e.g., [0, 1, 2]) - used for server-side validation to prevent step-skipping';
