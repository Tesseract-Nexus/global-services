-- Fix name column to allow NULL or have default
-- The Go code uses first_name/last_name, so name should be computed or nullable

-- Option 1: Make name nullable with default
ALTER TABLE users ALTER COLUMN name DROP NOT NULL;
ALTER TABLE users ALTER COLUMN name SET DEFAULT '';

-- Option 2: Create a trigger to auto-populate name from first_name and last_name
CREATE OR REPLACE FUNCTION update_user_name()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.name IS NULL OR NEW.name = '' THEN
        NEW.name := COALESCE(NEW.first_name, '') || ' ' || COALESCE(NEW.last_name, '');
        NEW.name := TRIM(NEW.name);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_user_name ON users;
CREATE TRIGGER trigger_update_user_name
    BEFORE INSERT OR UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_user_name();

-- Update existing rows that have empty name
UPDATE users
SET name = TRIM(COALESCE(first_name, '') || ' ' || COALESCE(last_name, ''))
WHERE name IS NULL OR name = '';
