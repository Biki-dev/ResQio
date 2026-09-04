-- name: CreateUser :one
INSERT INTO users (
    phone,
    password_hash,
    role,
    full_name
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, phone, role, full_name, created_at;

-- name: GetUserByPhone :one
SELECT id, phone, password_hash, role, full_name, created_at
FROM users
WHERE phone = $1;

-- name: GetUserByID :one
SELECT id, phone, role, full_name, created_at
FROM users
WHERE id = $1;
