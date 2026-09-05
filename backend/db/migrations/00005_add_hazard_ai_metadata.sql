-- +goose Up
ALTER TABLE road_hazards
    ADD COLUMN IF NOT EXISTS image_url TEXT,
    ADD COLUMN IF NOT EXISTS predicted_class VARCHAR(120),
    ADD COLUMN IF NOT EXISTS confidence_score DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS priority_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cluster_id UUID,
    ADD COLUMN IF NOT EXISTS cluster_size INT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_road_hazards_priority ON road_hazards(priority_score DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_road_hazards_cluster ON road_hazards(cluster_id);
CREATE INDEX IF NOT EXISTS idx_road_hazards_predicted_class ON road_hazards(predicted_class);

-- +goose Down
DROP INDEX IF EXISTS idx_road_hazards_predicted_class;
DROP INDEX IF EXISTS idx_road_hazards_cluster;
DROP INDEX IF EXISTS idx_road_hazards_priority;
ALTER TABLE road_hazards
    DROP COLUMN IF EXISTS cluster_size,
    DROP COLUMN IF EXISTS cluster_id,
    DROP COLUMN IF EXISTS priority_score,
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS predicted_class,
    DROP COLUMN IF EXISTS image_url;