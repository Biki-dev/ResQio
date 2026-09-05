-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS location geometry;
-- +goose StatementBegin
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
		CREATE INDEX IF NOT EXISTS idx_users_location ON users USING gist(location);
	ELSE
		CREATE INDEX IF NOT EXISTS idx_users_location ON users(location);
	END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_users_location;
ALTER TABLE users DROP COLUMN IF EXISTS location;
