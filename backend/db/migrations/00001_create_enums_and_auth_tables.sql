-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
        BEGIN
            CREATE EXTENSION IF NOT EXISTS postgis;
        EXCEPTION WHEN OTHERS THEN
            NULL;
        END;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'geometry') THEN
        CREATE DOMAIN geometry AS text;
    END IF;
END$$;
-- +goose StatementEnd

CREATE TYPE user_role AS ENUM (
    'GUEST',
    'VICTIM',
    'PUBLIC',
    'PROVIDER',
    'COORDINATOR',
    'ADMIN'
);

CREATE TYPE provider_type AS ENUM (
    'ORGANISATION',
    'INDIVIDUAL'
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(20) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role user_role NOT NULL DEFAULT 'PUBLIC',
    full_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type provider_type NOT NULL,
    authorized_person VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    registration_no VARCHAR(50),
    govt_id VARCHAR(50) NOT NULL,
    cin VARCHAR(50),
    email VARCHAR(255) NOT NULL UNIQUE,
    ph_no VARCHAR(20) NOT NULL UNIQUE,
    website VARCHAR(255),
    state VARCHAR(255) NOT NULL,
    dist VARCHAR(255) NOT NULL,
    location geometry NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_providers_email ON providers(email);
CREATE INDEX IF NOT EXISTS idx_providers_ph_no ON providers(ph_no);

-- +goose Down
DROP TABLE IF EXISTS providers;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS provider_type;
DROP TYPE IF EXISTS user_role;
