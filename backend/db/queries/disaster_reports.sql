-- name: CreateDisasterReport :one
INSERT INTO disaster_report (
    reporter_id,
    embedding,
    location
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetDisasterReportByID :one
SELECT * FROM disaster_report
WHERE id = $1 LIMIT 1;

-- name: ListDisasterReports :many
SELECT * FROM disaster_report
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListDisasterReportsByReporter :many
SELECT * FROM disaster_report
WHERE reporter_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
