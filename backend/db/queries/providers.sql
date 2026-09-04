-- name: CreateProvider :one
INSERT INTO providers (
    type,
    authorized_person,
    name,
    password_hash,
    registration_no,
    govt_id,
    cin,
    email,
    ph_no,
    website,
    state,
    dist,
    location
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING id, type, authorized_person, name, registration_no, govt_id, cin, email, ph_no, website, state, dist, location, last_updated_at, created_at;

-- name: GetProviderByEmail :one
SELECT id, type, authorized_person, name, password_hash, registration_no, govt_id, cin, email, ph_no, website, state, dist, location, last_updated_at, created_at
FROM providers
WHERE email = $1;

-- name: GetProviderByPhone :one
SELECT id, type, authorized_person, name, password_hash, registration_no, govt_id, cin, email, ph_no, website, state, dist, location, last_updated_at, created_at
FROM providers
WHERE ph_no = $1;

-- name: GetProviderByID :one
SELECT id, type, authorized_person, name, registration_no, govt_id, cin, email, ph_no, website, state, dist, location, last_updated_at, created_at
FROM providers
WHERE id = $1;
