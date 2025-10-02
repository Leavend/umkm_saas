-- +goose Up
CREATE TYPE user_role AS ENUM ('user', 'admin');
CREATE TYPE user_plan AS ENUM ('free', 'pro');
CREATE TYPE job_type AS ENUM ('image_generate', 'image_enhance', 'video_generate');
CREATE TYPE job_status AS ENUM ('queued', 'running', 'succeeded', 'failed');
CREATE TYPE asset_kind AS ENUM ('image', 'video');

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    google_sub TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    picture TEXT,
    locale TEXT DEFAULT 'id',
    role user_role NOT NULL DEFAULT 'user',
    plan user_plan NOT NULL DEFAULT 'free',
    quota_daily INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type job_type NOT NULL,
    status job_status NOT NULL,
    prompt_json JSONB NOT NULL,
    result_json JSONB,
    quantity INT NOT NULL DEFAULT 1,
    aspect_ratio TEXT NOT NULL DEFAULT '1:1',
    provider TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);

CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    kind asset_kind NOT NULL,
    url TEXT NOT NULL,
    width INT,
    height INT,
    checksum TEXT,
    bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assets_job_id ON assets(job_id);

CREATE TABLE IF NOT EXISTS prompt_templates (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    json_template JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS donations (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    amount_int BIGINT NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_donations_created_at ON donations(created_at DESC);

CREATE TABLE IF NOT EXISTS analytics_daily (
    day DATE PRIMARY KEY,
    visitors INT NOT NULL DEFAULT 0,
    online_users INT NOT NULL DEFAULT 0,
    ai_requests INT NOT NULL DEFAULT 0,
    last24h INT NOT NULL DEFAULT 0,
    videos_generated INT NOT NULL DEFAULT 0,
    images_generated INT NOT NULL DEFAULT 0,
    request_success INT NOT NULL DEFAULT 0,
    request_fail INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();

CREATE TRIGGER trg_jobs_updated
BEFORE UPDATE ON jobs
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();

CREATE TRIGGER trg_analytics_daily_updated
BEFORE UPDATE ON analytics_daily
FOR EACH ROW
EXECUTE PROCEDURE update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS trg_analytics_daily_updated ON analytics_daily;
DROP TRIGGER IF EXISTS trg_jobs_updated ON jobs;
DROP TRIGGER IF EXISTS trg_users_updated ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS analytics_daily;
DROP TABLE IF EXISTS donations;
DROP TABLE IF EXISTS prompt_templates;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS asset_kind;
DROP TYPE IF EXISTS job_status;
DROP TYPE IF EXISTS job_type;
DROP TYPE IF EXISTS user_plan;
DROP TYPE IF EXISTS user_role;
