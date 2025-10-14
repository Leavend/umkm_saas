-- name: CreateImageJob :one
INSERT INTO image_jobs (user_id, provider, model, status, quantity, aspect_ratio, prompt, source_asset)
VALUES ($1, $2, $3, 'QUEUED', $4, $5, $6, $7)
RETURNING id;

-- name: StartImageJob :exec
UPDATE image_jobs SET status = 'RUNNING', updated_at = now() WHERE id = $1;

-- name: CompleteImageJob :exec
UPDATE image_jobs SET status = 'SUCCEEDED', output = $2, updated_at = now() WHERE id = $1;

-- name: FailImageJob :exec
UPDATE image_jobs SET status = 'FAILED', error = $2, updated_at = now() WHERE id = $1;

-- name: GetImageJob :one
SELECT * FROM image_jobs WHERE id = $1;

-- name: ListImageJobsByUser :many
SELECT * FROM image_jobs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: StatsSummary :one
WITH agg AS (
  SELECT
    count(*) AS total,
    count(*) FILTER (WHERE status = 'SUCCEEDED') AS succeeded,
    count(*) FILTER (WHERE status = 'FAILED') AS failed
  FROM image_jobs
)
SELECT total, succeeded, failed,
       ROUND(100.0 * succeeded / NULLIF(total, 0), 2) AS success_rate
FROM agg;
