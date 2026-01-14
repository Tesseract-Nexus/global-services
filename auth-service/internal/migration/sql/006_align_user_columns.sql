-- Align users table with Go models
-- Add first_name, last_name columns that the Go model expects

-- Add first_name and last_name columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR(255);

-- Add role and status columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(50) DEFAULT 'customer';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active';

-- Add password column (plain text for legacy compatibility - actual storage uses password_hash)
ALTER TABLE users ADD COLUMN IF NOT EXISTS password VARCHAR(255);

-- Add phone column
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20);

-- Migrate existing name data to first_name/last_name
UPDATE users
SET first_name = SPLIT_PART(name, ' ', 1),
    last_name = COALESCE(NULLIF(SUBSTRING(name FROM POSITION(' ' IN name) + 1), ''), '')
WHERE first_name IS NULL AND name IS NOT NULL AND name != '';

-- Set defaults for any NULL values
UPDATE users SET first_name = '' WHERE first_name IS NULL;
UPDATE users SET last_name = '' WHERE last_name IS NULL;

-- Add NOT NULL constraint to first_name (after migration)
-- ALTER TABLE users ALTER COLUMN first_name SET NOT NULL;
-- ALTER TABLE users ALTER COLUMN last_name SET NOT NULL;

-- Create indexes for new columns
CREATE INDEX IF NOT EXISTS idx_users_first_name ON users(first_name);
CREATE INDEX IF NOT EXISTS idx_users_last_name ON users(last_name);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
