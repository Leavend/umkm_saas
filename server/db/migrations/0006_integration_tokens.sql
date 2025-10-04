-- +goose Up
create table if not exists integration_tokens (
    id uuid primary key default gen_random_uuid(),
    provider text not null,
    token text not null,
    properties jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists integration_tokens_provider_key on integration_tokens (provider);

-- +goose Down
drop table if exists integration_tokens;
