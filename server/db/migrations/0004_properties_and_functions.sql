-- +goose Up
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS google_sub text;

ALTER TABLE external_accounts
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE generation_requests
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS prompt_json jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE watermarks
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE prompt_templates
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE usage_events
    ADD COLUMN IF NOT EXISTS properties jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS donations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid,
    amount_int bigint NOT NULL,
    note text,
    testimonial text,
    properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Drop foreign keys to follow soft relationship strategy
ALTER TABLE external_accounts DROP CONSTRAINT IF EXISTS external_accounts_user_id_fkey;
ALTER TABLE generation_requests DROP CONSTRAINT IF EXISTS generation_requests_user_id_fkey;
ALTER TABLE generation_requests DROP CONSTRAINT IF EXISTS generation_requests_source_asset_id_fkey;
ALTER TABLE assets DROP CONSTRAINT IF EXISTS assets_user_id_fkey;
ALTER TABLE assets DROP CONSTRAINT IF EXISTS assets_request_id_fkey;
ALTER TABLE assets DROP CONSTRAINT IF EXISTS assets_derived_from_asset_id_fkey;
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_user_id_fkey;
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_request_id_fkey;
ALTER TABLE watermarks DROP CONSTRAINT IF EXISTS watermarks_user_id_fkey;
ALTER TABLE prompt_templates DROP CONSTRAINT IF EXISTS prompt_templates_user_id_fkey;

CREATE INDEX IF NOT EXISTS ix_external_accounts_user_id ON external_accounts(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_google_sub ON users(google_sub);
CREATE INDEX IF NOT EXISTS ix_generation_requests_user_id ON generation_requests(user_id);
CREATE INDEX IF NOT EXISTS ix_generation_requests_status ON generation_requests(status);
CREATE INDEX IF NOT EXISTS ix_assets_user_id ON assets(user_id);
CREATE INDEX IF NOT EXISTS ix_assets_request_id ON assets(request_id);
CREATE INDEX IF NOT EXISTS ix_usage_events_request_id ON usage_events(request_id);
CREATE INDEX IF NOT EXISTS ix_usage_events_user_id ON usage_events(user_id);
CREATE INDEX IF NOT EXISTS ix_donations_created_at ON donations(created_at DESC);

CREATE OR REPLACE FUNCTION fn_consume_quota(p_user_id uuid, p_used int)
RETURNS TABLE (remaining int)
LANGUAGE plpgsql
AS $$
DECLARE
    quota_daily int;
    quota_used int;
BEGIN
    SELECT COALESCE((properties->>'quota_daily')::int, 2),
           COALESCE((properties->>'quota_used_today')::int, 0)
      INTO quota_daily, quota_used
      FROM users
     WHERE id = p_user_id
     FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'user not found';
    END IF;

    IF quota_used + p_used > quota_daily THEN
        RAISE EXCEPTION 'quota exceeded';
    END IF;

    UPDATE users
       SET properties = jsonb_set(
               jsonb_set(properties, '{quota_used_today}', to_jsonb(quota_used + p_used), true),
               '{quota_refreshed_at}', to_jsonb(now()), true
           ),
           updated_at = now()
     WHERE id = p_user_id;

    remaining := quota_daily - (quota_used + p_used);
    RETURN NEXT;
END;
$$;

CREATE OR REPLACE FUNCTION fn_insert_job_and_usage(
    p_user_id uuid,
    p_task text,
    p_status text,
    p_prompt jsonb,
    p_quantity int,
    p_aspect_ratio text,
    p_provider text,
    p_properties jsonb
)
RETURNS TABLE(job_id uuid)
LANGUAGE plpgsql
AS $$
DECLARE
    new_job uuid;
BEGIN
    INSERT INTO generation_requests(
        id, user_id, task_type, provider, model, prompt_json, aspect_ratio, quantity, status, created_at, updated_at, properties
    )
    VALUES (
        gen_random_uuid(),
        p_user_id,
        p_task,
        p_provider,
        p_provider,
        p_prompt,
        p_aspect_ratio,
        p_quantity,
        p_status,
        now(),
        now(),
        p_properties
    )
    RETURNING id INTO new_job;

    INSERT INTO usage_events(id, user_id, request_id, event_type, success, created_at, properties)
    VALUES (
        gen_random_uuid(),
        p_user_id,
        new_job,
        p_task,
        true,
        now(),
        jsonb_build_object('provider', p_provider)
    );

    job_id := new_job;
    RETURN NEXT;
END;
$$;

CREATE OR REPLACE VIEW vw_stats_summary AS
WITH totals AS (
    SELECT
        COUNT(DISTINCT user_id) AS total_users,
        COUNT(*) FILTER (WHERE task_type = 'IMAGE_GEN') AS image_generated,
        COUNT(*) FILTER (WHERE task_type = 'VIDEO_GEN') AS video_generated,
        COUNT(*) FILTER (WHERE status = 'SUCCEEDED') AS request_success,
        COUNT(*) FILTER (WHERE status = 'FAILED') AS request_fail
    FROM generation_requests
),
last24h AS (
    SELECT
        COUNT(*) FILTER (WHERE task_type = 'IMAGE_GEN') AS image_last24,
        COUNT(*) FILTER (WHERE task_type = 'VIDEO_GEN') AS video_last24
    FROM generation_requests
    WHERE created_at >= now() - interval '24 hours'
)
SELECT
    totals.total_users,
    totals.image_generated,
    totals.video_generated,
    totals.request_success,
    totals.request_fail,
    last24h.image_last24,
    last24h.video_last24
FROM totals, last24h;

-- +goose Down
DROP VIEW IF EXISTS vw_stats_summary;
DROP FUNCTION IF EXISTS fn_insert_job_and_usage;
DROP FUNCTION IF EXISTS fn_consume_quota;
DROP INDEX IF EXISTS ix_donations_created_at;
DROP INDEX IF EXISTS ix_usage_events_user_id;
DROP INDEX IF EXISTS ix_usage_events_request_id;
DROP INDEX IF EXISTS ix_assets_request_id;
DROP INDEX IF EXISTS ix_assets_user_id;
DROP INDEX IF EXISTS ix_generation_requests_status;
DROP INDEX IF EXISTS ix_generation_requests_user_id;
DROP INDEX IF EXISTS ux_users_google_sub;
DROP INDEX IF EXISTS ix_external_accounts_user_id;
DROP TABLE IF EXISTS donations;
ALTER TABLE prompt_templates DROP COLUMN IF EXISTS properties;
ALTER TABLE watermarks DROP COLUMN IF EXISTS properties;
ALTER TABLE usage_events DROP COLUMN IF EXISTS properties;
ALTER TABLE assets DROP COLUMN IF EXISTS properties;
ALTER TABLE assets DROP COLUMN IF EXISTS updated_at;
ALTER TABLE generation_requests DROP COLUMN IF EXISTS properties;
ALTER TABLE generation_requests DROP COLUMN IF EXISTS updated_at;
ALTER TABLE generation_requests DROP COLUMN IF EXISTS prompt_json;
ALTER TABLE external_accounts DROP COLUMN IF EXISTS properties;
ALTER TABLE users DROP COLUMN IF EXISTS properties;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS google_sub;
