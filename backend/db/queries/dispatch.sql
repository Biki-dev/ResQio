-- name: FindCandidateProvidersForRequest :many
SELECT 
    p.id AS provider_id,
    p.name AS provider_name,
    p.ph_no AS provider_phone,
    r.id AS resource_id,
    r.title AS resource_title,
    r.current_capacity AS resource_capacity,
    ROUND(ST_Distance(p.location::geography, ar.location::geography))::bigint AS distance_meters
FROM assistance_requests ar
CROSS JOIN providers p
JOIN LATERAL (
    SELECT id, title, current_capacity 
    FROM resources 
    WHERE provider_id = p.id 
      AND (category = ar.category OR ar.category = 'OTHER' OR category = 'OTHER')
      AND current_capacity >= ar.quantity_needed
      AND status != 'UNAVAILABLE'
    ORDER BY current_capacity DESC
    LIMIT 1
) r ON true
WHERE ar.id = $1
  AND p.is_available = TRUE
  AND p.current_active_tasks < p.max_active_tasks
  AND NOT EXISTS (
      SELECT 1 FROM dispatch_pings dp 
      WHERE dp.request_id = ar.id AND dp.provider_id = p.id
  )
ORDER BY ST_Distance(p.location::geography, ar.location::geography) ASC
LIMIT $2;

-- name: CreateDispatchPing :one
INSERT INTO dispatch_pings (
    request_id, provider_id, ping_order, status, expires_at
) VALUES ($1, $2, $3, 'PENDING', $4)
RETURNING *;

-- name: GetActivePingForProvider :one
SELECT 
    dp.id AS ping_id,
    dp.request_id,
    dp.provider_id,
    dp.ping_order,
    dp.status,
    dp.expires_at,
    dp.created_at,
    ar.tracking_code,
    ar.category,
    ar.quantity_needed,
    ar.description,
    ar.address_text,
    ROUND(ST_Distance(p.location::geography, ar.location::geography))::bigint AS distance_meters
FROM dispatch_pings dp
JOIN assistance_requests ar ON ar.id = dp.request_id
JOIN providers p ON p.id = dp.provider_id
WHERE dp.provider_id = $1 AND dp.status = 'PENDING' AND dp.expires_at > NOW()
ORDER BY dp.created_at DESC
LIMIT 1;

-- name: GetPingByID :one
SELECT * FROM dispatch_pings
WHERE id = $1;

-- name: GetExpiredPings :many
SELECT * FROM dispatch_pings
WHERE status = 'PENDING' AND expires_at <= NOW()
ORDER BY expires_at ASC;

-- name: UpdatePingStatus :one
UPDATE dispatch_pings
SET status = $2, responded_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateDispatchMatch :one
INSERT INTO dispatch_matches (
    request_id, provider_id, handshake_code, status
) VALUES ($1, $2, $3, 'ACTIVE')
RETURNING *;

-- name: GetMatchByRequestID :one
SELECT 
    dm.id,
    dm.request_id,
    dm.provider_id,
    dm.cluster_id,
    dm.handshake_code,
    dm.status,
    dm.completed_at,
    dm.created_at,
    p.name AS provider_name,
    p.ph_no AS provider_phone
FROM dispatch_matches dm
JOIN providers p ON p.id = dm.provider_id
WHERE dm.request_id = $1;

-- name: MarkRequestMatched :exec
UPDATE assistance_requests
SET dispatch_status = 'MATCHED',
    status = 'IN_PROGRESS',
    matched_provider_id = $2,
    matched_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkRequestDispatching :exec
UPDATE assistance_requests
SET dispatch_status = 'DISPATCHING',
    updated_at = NOW()
WHERE id = $1;

-- name: MarkRequestExhausted :exec
UPDATE assistance_requests
SET dispatch_status = 'EXHAUSTED',
    updated_at = NOW()
WHERE id = $1;

-- name: DeductResourceCapacity :exec
UPDATE resources
SET current_capacity = GREATEST(0, current_capacity - $2),
    last_updated_at = NOW()
WHERE id = $1;

-- name: IncrementProviderActiveTasks :exec
UPDATE providers
SET current_active_tasks = current_active_tasks + 1,
    last_updated_at = NOW()
WHERE id = $1;

-- name: GetExhaustedRequests :many
SELECT 
    id, tracking_code, category, quantity_needed, description, priority, 
    status, dispatch_status, address_text, created_at, updated_at
FROM assistance_requests
WHERE dispatch_status = 'EXHAUSTED'
ORDER BY priority_score DESC, created_at DESC;

-- name: GetNextPingOrderForRequest :one
SELECT COALESCE(MAX(ping_order), 0) + 1 AS next_order
FROM dispatch_pings
WHERE request_id = $1;

-- name: DeductProviderResourceCapacityByCategory :exec
UPDATE resources
SET current_capacity = GREATEST(0, resources.current_capacity - $3),
    last_updated_at = NOW()
WHERE id = (
    SELECT r2.id FROM resources r2
    WHERE r2.provider_id = $1 AND r2.category = $2 AND r2.current_capacity >= $3 AND r2.status != 'UNAVAILABLE'
    ORDER BY r2.current_capacity DESC
    LIMIT 1
);

-- name: HasActivePingForRequest :one
SELECT EXISTS (
    SELECT 1 FROM dispatch_pings
    WHERE request_id = $1 AND status = 'PENDING' AND expires_at > NOW()
);

