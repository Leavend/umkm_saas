# UMKM SaaS Backend

Backend service for "Food Photography Naik Kelas" that exposes REST API with inline SQL, pgx, and worker pipeline.

## Prerequisites
- Go 1.22+
- PostgreSQL with `pgcrypto` extension
- `goose` for migrations (or use provided Makefile target)

## Setup
```bash
cp .env.example .env
# edit DATABASE_URL, JWT_SECRET, GOOGLE_CLIENT_ID, GOOGLE_ISSUER, STORAGE_BASE_URL, STORAGE_PATH
# download Go modules (requires internet access)
go mod tidy
# prepare database schema
make migrate
# store provider key centrally (optional, enables dynamic prompts for all users)
GEMINI_API_KEY=your-google-ai-key make set-gemini-key
# or switch to OpenAI by updating PROMPT_PROVIDER=openai and setting the key
OPENAI_API_KEY=your-openai-key make set-openai-key
# optional: override OPENAI_MODEL with a free tier model (defaults to gpt-4o-mini).
# aliases such as "gpt-5 thinking" map to gpt-4o-mini automatically, and any
# unsupported value also falls back to this free model tier.
```

## Run services
```bash
make run   # starts HTTP API on $PORT (default 8080)
make worker # starts background worker processing generation jobs
```

> **Important:** The application talks to a real PostgreSQL instance via
> `pgx/v5`. Ensure the database referenced in `DATABASE_URL` is reachable and
> already has the required extensions (`pgcrypto`) enabled before launching the
> services. When developing offline, make sure the dependencies are cached or
> vendored locally prior to running the commands above.

## Upgrading a user's plan

Use the dedicated CLI to switch a user from the free tier to pro (or any other
supported plan) and refresh their quota metadata. The command accepts either a
user ID or email address and, by default, bumps the daily quota to 50 while
resetting the usage counter.

```bash
# upgrade by email, set plan to pro with a 75 image/day quota
make user-plan ARGS="-email tiohadybayu@gmail.com -plan pro -quota 75"

# alternatively reference the UUID directly and keep the existing usage tally
make user-plan ARGS="-id ee717b5d-ae7e-42fa-a32b-d60b39afb943 -plan pro -keep-usage"
```

Flags:

- `-email` *(string)*: look up the user by email.
- `-id` *(UUID)*: look up the user by ID.
- `-plan` *(string, default `pro`)*: assign one of `free`, `pro`, or `supporter`.
- `-quota` *(int, default `50`)*: update the daily quota; pass `0` or a negative
  number to keep the current value.
- `-keep-usage` *(bool)*: when set, preserves the existing
  `quota_used_today` value instead of resetting it to zero.

The worker and HTTP layer both delegate image & video generation to the
Gemini **2.5 Flash** provider. When no `GEMINI_API_KEY` is configured the
provider emits deterministic synthetic assets so the end-to-end pipeline remains
testable without external calls. Synthetic results are written under
`$STORAGE_PATH` (default `./storage`) and exposed through `/static/...` URLs,
allowing clients to download the placeholder files returned by
`/v1/assets/{id}/download` immediately.

## Verification
```bash
make verify
```

## Key endpoints
```bash
# API documentation
curl -i http://localhost:8080/v1/openapi.json
curl -i http://localhost:8080/v1/docs

# Health
curl -i http://localhost:8080/v1/healthz

# Google auth (id_token from Google)
curl -i -X POST http://localhost:8080/v1/auth/google/verify \
  -H 'Content-Type: application/json' \
  -d '{"id_token":"<GOOGLE_ID_TOKEN>"}'

# Current user
curl -i -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/me

# Generate edited images synchronously (DashScope "qwen-image-edit")
curl -i -X POST http://localhost:8080/v1/images/generate 
  -H "Authorization: Bearer <JWT>" -H 'Content-Type: application/json' 
  -d '{
    "provider":"qwen-image-plus",
    "quantity":2,
    "aspect_ratio":"1:1",
    "prompt":{
      "title":"Nasi goreng seafood premium",
      "product_type":"food",
      "style":"elegan",
      "background":"marble",
      "instructions":"Lighting lembut",
      "watermark":{"enabled":false},
      "source_asset":{
        "asset_id":"upl_abc123",
        "url":"https://cdn.example.com/uploads/upl_abc123.png"
      },
      "extras":{"negative_prompt":"blurry"}
    }
  }'

# Inspect job payload, prompts, and output URLs
curl -i -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/images/jobs/<JOB_ID>

# Proxy-download the first generated image
curl -L -H "Authorization: Bearer <JWT>" 
  http://localhost:8080/v1/images/<JOB_ID>/download --output edited.png

# Download all generated images as a zip archive
curl -L -H "Authorization: Bearer <JWT>" 
  http://localhost:8080/v1/images/<JOB_ID>/download.zip --output edited.zip

# Generate videos (async via worker)
curl -i -X POST -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/videos/generate \
  -H 'Content-Type: application/json' \
  -d '{"provider":"gemini-2.5-flash","prompt":"Hero shot ramen"}'

# Ideas
curl -i -X POST -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/ideas/from-image \
  -H 'Content-Type: application/json' -d '{"image_base64":"..."}'

# Stats summary
curl -i http://localhost:8080/v1/stats/summary
```

## SQL Inline conventions
All SQL strings live in `internal/sqlinline/` and begin with `--sql <uuid>` marker. `make sqllint` (part of `make verify`) fails when the marker is missing.

## Adding new inline SQL
1. Add a constant in `internal/sqlinline/<domain>.go` using backtick literal.
2. Ensure first line is `--sql <uuid-v4>`.
3. Reference the constant through `infra.SQLRunner` helper to gain logging with `sql_uuid`.

## Extending features
- Extend JSON schemas under `internal/domain/jsoncfg/` for new configuration fields.
- Add properties to tables via new Goose migration (ensure JSONB defaults and updated_at columns).
- Register new endpoints in `internal/http/httpapi/router.go` and use `App` dependencies for DB, providers, and logging.

