--sql 4c2a1a9e-1b77-4b8e-a1d2-9a7b2c3d4e5f
-- name: InsertAsset :one
INSERT INTO assets (
  user_id, kind, request_id, derived_from_asset_id, storage_key,
  mime, bytes, width, height, aspect_ratio, hash_sha256, metadata
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING *;

--sql 9a1b2c3d-4e5f-6a7b-8c9d-0e1f2a3b4c5d
-- name: ListAssetsByUser :many
SELECT id, kind, request_id, storage_key, mime, bytes, created_at
FROM assets
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT COALESCE($2::int, 50);
