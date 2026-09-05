-- name: CreateAssistanceRequest :one
INSERT INTO assistance_requests (
    requester_id,
    tracking_code,
    category,
    quantity_needed,
    description,
    priority,
    status,
    location,
    address_text,
    contact_phone_encrypted,
    requester_name_encrypted,
    embedding
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetAssistanceRequestByID :one
SELECT * FROM assistance_requests
WHERE id = $1 LIMIT 1;

-- name: GetAssistanceRequestByTrackingCode :one
SELECT * FROM assistance_requests
WHERE tracking_code = $1 LIMIT 1;

-- name: ListAssistanceRequests :many
SELECT * FROM assistance_requests
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAssistanceRequestsByRequester :many
SELECT * FROM assistance_requests
WHERE requester_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAssistanceRequestStatus :one
UPDATE assistance_requests
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: AssignCoordinatorToAssistanceRequest :one
UPDATE assistance_requests
SET assigned_coordinator_id = $2, status = 'IN_PROGRESS', updated_at = NOW()
WHERE id = $1
RETURNING *;
