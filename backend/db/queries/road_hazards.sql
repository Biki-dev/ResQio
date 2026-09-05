-- name: CreateRoadHazard :one
INSERT INTO road_hazards (
    reporter_id,
    reporter_name,
    reporter_phone,
    hazard_type,
    severity,
    location,
    description,
    is_verified,
    image_url,
    predicted_class,
    confidence_score,
    priority_score,
    cluster_id,
    cluster_size
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetRoadHazardByID :one
SELECT * FROM road_hazards
WHERE id = $1 LIMIT 1;

-- name: ListRoadHazards :many
SELECT * FROM road_hazards
ORDER BY priority_score DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListRoadHazardsByReporter :many
SELECT * FROM road_hazards
WHERE reporter_id = $1
ORDER BY priority_score DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: VerifyRoadHazard :one
UPDATE road_hazards
SET is_verified = $2
WHERE id = $1
RETURNING *;
