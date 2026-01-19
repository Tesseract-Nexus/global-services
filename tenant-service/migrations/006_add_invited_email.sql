-- Add invited_email to user_tenant_memberships for invitation workflows
ALTER TABLE user_tenant_memberships
    ADD COLUMN IF NOT EXISTS invited_email VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_utm_invited_email
    ON user_tenant_memberships(invited_email)
    WHERE invited_email IS NOT NULL;
