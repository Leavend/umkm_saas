# UMKM SaaS Backend

Backend service for "Food Photography Naik Kelas" that exposes REST API with inline SQL, pgx, and worker pipeline.

## Prerequisites
- Go 1.22+
- PostgreSQL with `pgcrypto` extension
- `goose` for migrations (or use provided Makefile target)

## Setup
```bash
cp .env.example .env
# edit DATABASE_URL, JWT_SECRET, GOOGLE_CLIENT_ID, GOOGLE_ISSUER, STORAGE_BASE_URL
make migrate
```

## Run services
```bash
make run   # starts HTTP API on $PORT (default 8080)
make worker # starts background worker processing generation jobs
```

## Verification
```bash
make verify
```

## Key endpoints
```bash
# Health
curl -i http://localhost:8080/v1/healthz

# Google auth (id_token from Google)
curl -i -X POST http://localhost:8080/v1/auth/google/verify \
  -H 'Content-Type: application/json' \
  -d '{"id_token":"<GOOGLE_ID_TOKEN>"}'

# Current user
curl -i -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/me

# Generate images
curl -i -X POST http://localhost:8080/v1/images/generate \
  -H "Authorization: Bearer <JWT>" -H 'Content-Type: application/json' \
  -d '{
    "provider":"gemini",
    "quantity":1,
    "aspect_ratio":"1:1",
    "prompt":{
      "title":"Nasi goreng seafood premium",
      "product_type":"food",
      "style":"elegan",
      "background":"marble",
      "instructions":"Lighting lembut",
      "watermark":{"enabled":true,"text":"Warung Nasgor Bapak","position":"bottom-right"},
      "references":[],
      "extras":{"locale":"id","quality":"hd"}
    }
  }'

# Check job status
curl -i -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/images/<JOB_ID>/status

# Check job assets
curl -i -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/images/<JOB_ID>/assets

# Zip assets
curl -i -X POST -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/images/<JOB_ID>/zip

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

