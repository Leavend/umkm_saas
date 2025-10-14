--sql 5d1b23a2-1d77-4df1-9c3a-9a1b2c3d4e5f
-- name: GetDashboard24h :one
SELECT
  COUNT(*) FILTER (WHERE event_type='IMAGE_GEN')      AS image_generated,
  COUNT(*) FILTER (WHERE event_type='VIDEO_GEN')      AS video_generated,
  COUNT(*) FILTER (WHERE success)                     AS request_success,
  COUNT(*) FILTER (WHERE NOT success)                 AS request_fail
FROM usage_events
WHERE created_at >= now() - interval '24 hours';
