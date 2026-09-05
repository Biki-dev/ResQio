-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS location geometry;
CREATE INDEX IF NOT EXISTS idx_users_location ON users USING gist(location);

-- +goose Down
DROP INDEX IF EXISTS idx_users_location;
ALTER TABLE users DROP COLUMN IF EXISTS location;
