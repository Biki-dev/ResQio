-- +goose Up
CREATE TYPE dispatch_status AS ENUM (
    'QUEUED',
    'DISPATCHING',
    'MATCHED',
    'FULFILLED',
    'EXHAUSTED',
    'CANCELLED'
);

CREATE TYPE dispatch_ping_status AS ENUM (
    'PENDING',
    'ACCEPTED',
    'REJECTED',
    'EXPIRED'
);

CREATE TYPE match_status AS ENUM (
    'ACTIVE',
    'COMPLETED',
    'ABORTED'
);

-- Alter providers with dispatch columns
ALTER TABLE providers 
    ADD COLUMN IF NOT EXISTS service_radius_km DOUBLE PRECISION NOT NULL DEFAULT 15.0,
    ADD COLUMN IF NOT EXISTS is_available BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS max_active_tasks INT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS current_active_tasks INT NOT NULL DEFAULT 0;

-- Alter assistance_requests with dispatch and AI scoring columns
ALTER TABLE assistance_requests
    ADD COLUMN IF NOT EXISTS priority_score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS yolo_score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS yolo_detections JSONB,
    ADD COLUMN IF NOT EXISTS cluster_id UUID,
    ADD COLUMN IF NOT EXISTS cluster_size INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS medical_emergency BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS water_food_urgency BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS vulnerable_individuals_present BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS property_damage_only BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_identity_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_location_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS dispatch_status dispatch_status NOT NULL DEFAULT 'QUEUED',
    ADD COLUMN IF NOT EXISTS matched_provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS matched_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_assistance_requests_dispatch_status ON assistance_requests(dispatch_status);

-- Create dispatch_pings table
CREATE TABLE IF NOT EXISTS dispatch_pings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES assistance_requests(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    ping_order INT NOT NULL,
    status dispatch_ping_status NOT NULL DEFAULT 'PENDING',
    expires_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_pings_provider ON dispatch_pings(provider_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_dispatch_pings_request ON dispatch_pings(request_id, status);

-- Create dispatch_matches table
CREATE TABLE IF NOT EXISTS dispatch_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL UNIQUE REFERENCES assistance_requests(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    cluster_id UUID,
    handshake_code VARCHAR(6) NOT NULL,
    status match_status NOT NULL DEFAULT 'ACTIVE',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_matches_provider ON dispatch_matches(provider_id, status);

-- +goose Down
DROP TABLE IF EXISTS dispatch_matches;
DROP TABLE IF EXISTS dispatch_pings;

ALTER TABLE assistance_requests
    DROP COLUMN IF EXISTS matched_at,
    DROP COLUMN IF EXISTS matched_provider_id,
    DROP COLUMN IF EXISTS dispatch_status,
    DROP COLUMN IF EXISTS is_location_verified,
    DROP COLUMN IF EXISTS is_identity_verified,
    DROP COLUMN IF EXISTS property_damage_only,
    DROP COLUMN IF EXISTS vulnerable_individuals_present,
    DROP COLUMN IF EXISTS water_food_urgency,
    DROP COLUMN IF EXISTS medical_emergency,
    DROP COLUMN IF EXISTS cluster_size,
    DROP COLUMN IF EXISTS cluster_id,
    DROP COLUMN IF EXISTS yolo_detections,
    DROP COLUMN IF EXISTS yolo_score,
    DROP COLUMN IF EXISTS priority_score;

ALTER TABLE providers
    DROP COLUMN IF EXISTS current_active_tasks,
    DROP COLUMN IF EXISTS max_active_tasks,
    DROP COLUMN IF EXISTS is_available,
    DROP COLUMN IF EXISTS service_radius_km;

DROP TYPE IF EXISTS match_status;
DROP TYPE IF EXISTS dispatch_ping_status;
DROP TYPE IF EXISTS dispatch_status;
