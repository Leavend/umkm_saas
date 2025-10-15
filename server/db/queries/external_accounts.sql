--sql 7a8b9c0d-1e2f-3a4b-5c6d-7e8f9a0b1c2d
-- name: GetGoogleAccountByUser :one
SELECT id, access_token, refresh_token, expires_at, scopes
FROM external_accounts
WHERE user_id = $1 AND provider = 'google'
LIMIT 1;
