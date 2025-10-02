-- +goose Up
CREATE TABLE watermarks (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid REFERENCES users(id) ON DELETE CASCADE,
  type        text NOT NULL CHECK (type IN ('TEXT','IMAGE')),
  config_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE prompt_templates (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid REFERENCES users(id) ON DELETE CASCADE,
  title         text NOT NULL,
  tags          text[] NOT NULL DEFAULT '{}',
  template_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE IF EXISTS prompt_templates;
DROP TABLE IF EXISTS watermarks;
