-- +goose Up
CREATE TABLE IF NOT EXISTS usage_events (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  request_id          uuid REFERENCES generation_requests(id) ON DELETE CASCADE,
  event_type          text NOT NULL CHECK (event_type IN ('IMAGE_GEN','VIDEO_GEN','UPSCALE')),
  success             boolean NOT NULL DEFAULT true,
  latency_ms          integer,
  provider_cost_cents integer,
  created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_usage_events_user_created ON usage_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_usage_events_success      ON usage_events(success);
CREATE INDEX IF NOT EXISTS ix_usage_events_event_type   ON usage_events(event_type);

-- +goose Down
DROP TABLE IF EXISTS usage_events;
