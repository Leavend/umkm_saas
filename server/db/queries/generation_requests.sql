--sql 6e7e8d2a-8d3f-4a1b-9a55-44f6c6f6f2f1
-- name: EnqueueGeneration :one
INSERT INTO generation_requests (
  user_id, task_type, provider, model, source_asset_id,
  prompt_json, aspect_ratio, quantity, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, LEAST($8, 2), 'QUEUED')
RETURNING *;

--sql 2fa8b4d9-7c2f-4b90-b2a3-1d6f5b2c2b9a
-- name: GetRequestStatus :one
SELECT id, status, progress, error_code, error_message,
       prompt_json->>'style' AS style,
       created_at, started_at, finished_at
FROM generation_requests
WHERE id = $1;

--sql 8c01c6e6-1f8e-4f61-9e2d-6040d7cf24af
-- name: ClaimNextQueuedJob :one
WITH next AS (
  SELECT id
  FROM generation_requests
  WHERE status = 'QUEUED'
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE generation_requests gr
SET status='RUNNING', started_at=now()
FROM next
WHERE gr.id = next.id
RETURNING gr.id, gr.user_id, gr.provider, gr.model, gr.prompt_json, gr.source_asset_id;
