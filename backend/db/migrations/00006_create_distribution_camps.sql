-- +goose Up
CREATE TABLE IF NOT EXISTS distribution_camps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    camp_name VARCHAR(255) NOT NULL,
    address_text TEXT NOT NULL,
    location geometry NOT NULL,
    items_available TEXT NOT NULL,
    distribution_start TIME NOT NULL,
    distribution_end TIME NOT NULL,
    contact_phone VARCHAR(20),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_distribution_camps_provider ON distribution_camps(provider_id);
CREATE INDEX IF NOT EXISTS idx_distribution_camps_active ON distribution_camps(is_active);
CREATE INDEX IF NOT EXISTS idx_distribution_camps_location ON distribution_camps USING GIST(location);

-- +goose Down
DROP TABLE IF EXISTS distribution_camps;
