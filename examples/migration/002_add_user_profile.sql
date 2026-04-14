-- Migration 002: Add User Profile Fields
-- Demonstrates adding new columns (safe operation)

-- Add profile fields to users table
ALTER TABLE users
ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(500),
ADD COLUMN IF NOT EXISTS bio TEXT,
ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'UTC';

-- Create index for timezone filtering
CREATE INDEX IF NOT EXISTS idx_users_timezone ON users(timezone);

COMMENT ON COLUMN users.avatar_url IS 'User avatar image URL';
COMMENT ON COLUMN users.bio IS 'User biography';
COMMENT ON COLUMN users.timezone IS 'User timezone preference';
