-- name: CreateMutualAidItem :one
INSERT INTO mutual_aid_items (
    user_id,
    item_name,
    quantity,
    description,
    location,
    is_claimed
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetMutualAidItemByID :one
SELECT * FROM mutual_aid_items
WHERE id = $1 LIMIT 1;

-- name: ListMutualAidItems :many
SELECT * FROM mutual_aid_items
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAvailableMutualAidItems :many
SELECT * FROM mutual_aid_items
WHERE is_claimed = FALSE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListMutualAidItemsByUser :many
SELECT * FROM mutual_aid_items
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ClaimMutualAidItem :one
UPDATE mutual_aid_items
SET is_claimed = TRUE, claimed_by_user_id = $2
WHERE id = $1 AND is_claimed = FALSE
RETURNING *;
