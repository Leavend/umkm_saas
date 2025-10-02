-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_user_id    text NOT NULL UNIQUE,
  email            text NOT NULL UNIQUE,
  name             text,
  avatar_url       text,
  plan             text NOT NULL DEFAULT 'free' CHECK (plan IN ('free','pro','supporter')),
  locale_pref      text,
  feature_flags    jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_ip_country  text,
  last_seen_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE external_accounts (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider           text NOT NULL CHECK (provider IN ('google')),
  external_user_id   text NOT NULL,
  clerk_external_id  text,
  scopes             text[] NOT NULL DEFAULT '{}',
  access_token       text,
  refresh_token      text,
  expires_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_external_accounts_user_provider ON external_accounts(user_id, provider);
CREATE UNIQUE INDEX ux_external_accounts_provider_sub  ON external_accounts(provider, external_user_id);

CREATE TABLE generation_requests (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_type      text NOT NULL CHECK (task_type IN ('IMAGE_GEN','VIDEO_GEN','UPSCALE','PROMPT_ENHANCE','IDEA')),
  provider       text NOT NULL,
  model          text NOT NULL,
  source_asset_id uuid,
  prompt_json    jsonb NOT NULL DEFAULT '{}'::jsonb,
  aspect_ratio   text NOT NULL CHECK (aspect_ratio IN ('1:1','4:3','3:4','16:9','9:16')),
  quantity       int  NOT NULL DEFAULT 1 CHECK (quantity BETWEEN 1 AND 8),
  status         text NOT NULL CHECK (status IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELED')),
  progress       int  NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
  error_code     text,
  error_message  text,
  started_at     timestamptz,
  finished_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT prompt_has_version CHECK (prompt_json ? 'version')
);
CREATE INDEX ix_gr_status_created ON generation_requests(status, created_at DESC);
CREATE INDEX ix_gr_prompt_gin     ON generation_requests USING GIN (prompt_json);
CREATE INDEX ix_gr_style_expr     ON generation_requests ((prompt_json->>'style'));

CREATE TABLE assets (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind                   text NOT NULL CHECK (kind IN ('ORIGINAL','GENERATED','UPSCALED','THUMBNAIL')),
  request_id             uuid REFERENCES generation_requests(id) ON DELETE CASCADE,
  derived_from_asset_id  uuid REFERENCES assets(id) ON DELETE SET NULL,
  storage_key            text NOT NULL,
  mime                   text NOT NULL,
  bytes                  bigint NOT NULL,
  width                  int,
  height                 int,
  aspect_ratio           text,
  hash_sha256            text,
  metadata               jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_assets_hash       ON assets(hash_sha256);
CREATE INDEX ix_assets_user_kind_created ON assets(user_id, kind, created_at DESC);

ALTER TABLE generation_requests
  ADD CONSTRAINT fk_gr_source_asset
  FOREIGN KEY (source_asset_id) REFERENCES assets(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE generation_requests DROP CONSTRAINT IF EXISTS fk_gr_source_asset;
DROP TABLE IF EXISTS assets;
DROP INDEX IF EXISTS ix_gr_style_expr;
DROP INDEX IF EXISTS ix_gr_prompt_gin;
DROP INDEX IF EXISTS ix_gr_status_created;
DROP TABLE IF EXISTS generation_requests;
DROP INDEX IF EXISTS ux_external_accounts_provider_sub;
DROP INDEX IF EXISTS ux_external_accounts_user_provider;
DROP TABLE IF EXISTS external_accounts;
DROP TABLE IF EXISTS users;
