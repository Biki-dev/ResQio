-- name: CreateUser :one
INSERT INTO users (
    phone,
    password_hash,
    role,
    full_name,
    location
) VALUES (
    $1, 
    $2, 
    $3, 
    $4,
    CASE 
        WHEN sqlc.arg('location')::text != '' AND sqlc.arg('location')::text IS NOT NULL THEN ST_GeomFromText(sqlc.arg('location')::text, 4326)
        ELSE NULL
    END
)
RETURNING id, phone, role, full_name, COALESCE(ST_AsText(location), '')::text as location, created_at;

-- name: GetUserByPhone :one
SELECT id, phone, password_hash, role, full_name, COALESCE(ST_AsText(location), '')::text as location, created_at
FROM users
WHERE phone = $1;

-- name: GetUserByID :one
SELECT id, phone, role, full_name, COALESCE(ST_AsText(location), '')::text as location, created_at
FROM users
WHERE id = $1;
