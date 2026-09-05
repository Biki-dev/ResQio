-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        BEGIN
            CREATE EXTENSION IF NOT EXISTS vector;
        EXCEPTION WHEN OTHERS THEN
            NULL;
        END;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
        CREATE DOMAIN vector AS text;
    END IF;
END$$;
-- +goose StatementEnd

CREATE TYPE resource_category AS ENUM (
    'FOOD',
    'WATER',
    'MEDICINE',
    'SHELTER',
    'EQUIPMENT',
    'VOLUNTEER',
    'OTHER'
);

CREATE TYPE verification_status AS ENUM (
    'UNVERIFIED',
    'VERIFIED',
    'UNAVAILABLE',
    'OUTDATED'
);

CREATE TYPE request_priority AS ENUM (
    'LOW',
    'MEDIUM',
    'HIGH',
    'CRITICAL'
);

CREATE TYPE request_status AS ENUM (
    'SUBMITTED',
    'ACKNOWLEDGED',
    'IN_PROGRESS',
    'FULFILLED',
    'CANCELLED'
);

-- 1. Disaster Report Table (Disaster Reporting Portal)
CREATE TABLE IF NOT EXISTS disaster_report (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID DEFAULT gen_random_uuid(),
    embedding vector NOT NULL,
    location geometry NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_disaster_report_created_at ON disaster_report(created_at DESC);

-- 2. Assistance Requests Table (Needs & Calls for Help)
CREATE TABLE IF NOT EXISTS assistance_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID REFERENCES users(id) ON DELETE SET NULL,
    tracking_code VARCHAR(12) NOT NULL UNIQUE,
    category resource_category NOT NULL,
    quantity_needed INT NOT NULL DEFAULT 1,
    description TEXT,
    priority request_priority NOT NULL DEFAULT 'MEDIUM',
    status request_status NOT NULL DEFAULT 'SUBMITTED',
    assigned_coordinator_id UUID REFERENCES users(id) ON DELETE SET NULL,
    location geometry NOT NULL,
    address_text TEXT,
    contact_phone_encrypted TEXT NOT NULL,
    requester_name_encrypted TEXT NOT NULL,
    embedding vector,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assistance_requests_tracking_code ON assistance_requests(tracking_code);
CREATE INDEX IF NOT EXISTS idx_assistance_requests_status ON assistance_requests(status);
CREATE INDEX IF NOT EXISTS idx_assistance_requests_requester_id ON assistance_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_assistance_requests_created_at ON assistance_requests(created_at DESC);

-- 3. Road Hazards Table (Issues Submission)
CREATE TABLE IF NOT EXISTS road_hazards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reporter_name VARCHAR(255),
    reporter_phone VARCHAR(20),
    hazard_type VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL DEFAULT 'MEDIUM',
    location geometry NOT NULL,
    description TEXT,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_road_hazards_reporter_id ON road_hazards(reporter_id);
CREATE INDEX IF NOT EXISTS idx_road_hazards_created_at ON road_hazards(created_at DESC);

-- 4. Mutual Aid Items Table
CREATE TABLE IF NOT EXISTS mutual_aid_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    description TEXT,
    location geometry NOT NULL,
    is_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    claimed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mutual_aid_items_user_id ON mutual_aid_items(user_id);
CREATE INDEX IF NOT EXISTS idx_mutual_aid_items_is_claimed ON mutual_aid_items(is_claimed);

-- 5. Resources Table (Provider Inventory & Capacities)
CREATE TABLE IF NOT EXISTS resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category resource_category NOT NULL,
    total_capacity INT NOT NULL,
    current_capacity INT NOT NULL,
    unit VARCHAR(50),
    status verification_status NOT NULL DEFAULT 'UNVERIFIED',
    location geometry NOT NULL,
    contact_phone VARCHAR(20),
    image_url TEXT,
    embedding vector,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_resources_provider_id ON resources(provider_id);
CREATE INDEX IF NOT EXISTS idx_resources_category ON resources(category);
CREATE INDEX IF NOT EXISTS idx_resources_status ON resources(status);

-- +goose Down
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS mutual_aid_items;
DROP TABLE IF EXISTS road_hazards;
DROP TABLE IF EXISTS assistance_requests;
DROP TABLE IF EXISTS disaster_report;

DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS request_priority;
DROP TYPE IF EXISTS verification_status;
DROP TYPE IF EXISTS resource_category;
