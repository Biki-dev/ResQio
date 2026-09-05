-- name: CreateResource :one
INSERT INTO resources (
    provider_id,
    title,
    description,
    category,
    total_capacity,
    current_capacity,
    unit,
    status,
    location,
    contact_phone,
    embedding
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetResourceByID :one
SELECT * FROM resources
WHERE id = $1 LIMIT 1;

-- name: ListResources :many
SELECT * FROM resources
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListResourcesByProvider :many
SELECT * FROM resources
WHERE provider_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListResourcesByCategory :many
SELECT * FROM resources
WHERE category = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateResourceCapacity :one
UPDATE resources
SET current_capacity = $2, last_updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateResourceStatus :one
UPDATE resources
SET status = $2, last_updated_at = NOW()
WHERE id = $1
RETURNING *;
