-- +goose Up
CREATE TABLE IF NOT EXISTS image_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('QUEUED','RUNNING','SUCCEEDED','FAILED')),
  quantity INT NOT NULL DEFAULT 1,
  aspect_ratio TEXT,
  prompt JSONB NOT NULL,
  source_asset JSONB NOT NULL,
  output JSONB,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_image_jobs_created_at ON image_jobs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_image_jobs_user_id ON image_jobs (user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_image_jobs_user_id;
DROP INDEX IF EXISTS idx_image_jobs_created_at;
DROP TABLE IF EXISTS image_jobs;
