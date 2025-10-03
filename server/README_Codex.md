# Backend SaaS — Codex Playbook

**Go Wrapper · Inline SQL/CTE · JSONB Properties · No ORM/No FK · Worker · OpenAPI**

> **Goal:** Backend siap produksi untuk SaaS UMKM “Food Photography Naik Kelas Pake AI”.
> **Core prinsip:** Go hanya sebagai **wrapper** (HTTP, auth, validasi, logging, CORS, i18n, rate-limit, kuota). **Logic bisnis & agregasi data** dikerjakan di **PostgreSQL** via **inline SQL/CTE** + fungsi PL/pgSQL.
> **Build guard:** Semua inline SQL **wajib** diawali `--sql <UUIDv4>` → dilint oleh **sqllint** (build-blocking).

---

## 0) TL;DR — Cara Jalanin Cepat

```bash
# dari folder server/
go mod tidy
make migrate             # jalankan migrasi db (auto-load .env jika Makefile sudah include .env)
make run                 # start API di :1919
# terminal lain
make worker              # jalankan worker job (wajib untuk pipeline gambar)
```

Health check:

```bash
curl -i http://localhost:1919/v1/healthz
```

---

## 1) Prinsip Arsitektur (Wajib)

1. **Go = wrapper**: transport (HTTP), validasi, auth, logging, CORS, i18n (auto ID/EN), rate-limit, kuota.
2. **Tanpa ORM, tanpa FOREIGN KEY**. Hubungan antar entitas dihandle di **inline SQL** + **INDEX**.
3. Semua tabel inti memiliki **UUID v4 PK**, **`properties JSONB`**, **`created_at`/`updated_at`**.
4. **Setiap inline SQL wajib**: baris pertama `--sql <UUIDv4>` (unik). **sqllint** gagal → build gagal.
5. **pgx/v5 + sqlc (pgx mode)**, koneksi **pgxpool**.
6. **Clean Architecture / DDD** dipertahankan; implementasi repo/usecase diarahkan ke **inline SQL/CTE**.

---

## 2) Status Saat Ini (Rute Siap Dicoba)

### Tanpa autentikasi

* `GET  /v1/healthz` → health OK.
* `POST /v1/auth/google/verify` → verifikasi Google **ID token** (JWKS), **upsert user + external_account**, balas **JWT + profil**.
* `GET  /v1/stats/summary` → agregasi statistik langsung dari DB (view/CTE).
* `POST /v1/donations`, `GET /v1/donations/testimonials` → mencatat donasi & membaca testimoni via SQL.

### Perlu JWT (Authorization: Bearer …)

* **Profil**: `GET /v1/me` → profil + kuota harian (JSONB).
* **Prompts**: `POST /v1/prompts/enhance`, `POST /v1/prompts/random`, `POST /v1/prompts/clear`
  (saat ini enhancer **statis** — cocok uji E2E, belum AI nyata).
* **Images**:

  * `POST /v1/images/generate` *(enqueue & kurangi kuota)*
  * `GET  /v1/images/{job_id}/status`
  * `GET  /v1/images/{job_id}/assets`
  * `POST /v1/images/{job_id}/zip` *(ZIP bytes, worker harus hidup agar aset terisi)*

  > Catatan: ada endpoint **enhance**. Standarisasi final disarankan: `POST /v1/images/enhance`.
* **Videos**:

  * `POST /v1/videos/generate` *(saat ini sinkron stub; **target**: async via worker)*
  * `GET  /v1/videos/{job_id}/status|assets`
* **Ideas**: `POST /v1/ideas/from-image` → validasi & kembalikan 2 ide dummy.
* **Assets**:

  * `GET  /v1/assets` (paginasi milik user)
  * `GET  /v1/assets/{id}/download` (verifikasi kepemilikan + URL unduh)

### Infrastruktur yang sudah ada

* **Kuota harian**: di `users.properties` + **`fn_consume_quota`** (atomik saat enqueue).
* **Worker**: ambil job `queued` (`FOR UPDATE SKIP LOCKED`) → set `running` → panggil provider gambar **stub** → simpan aset → set `succeeded/failed`.
* **Video**: saat ini **sync** (di handler) → **TODO** pindah ke **worker**.
* **sqllint**: build-blocking; inline SQL wajib ber-UUID.
* **Makefile**: target `run`, `worker`, `migrate`, `lint`, `verify`.

---

## 3) Definition of Done (DoD)

* Semua endpoint **terhubung DB (bukan dummy)**, dokumentasi **OpenAPI** tersedia di `/v1/openapi.json` + `/v1/docs` (Redoc).
* **Router tunggal** `internal/http/httpapi/router.go` (prefix `/v1`), tanpa duplikasi.
* **sqllint** aktif di `make verify`.
* **Worker** memproses **images & videos** (async) → `queued → running → succeeded/failed`.
* **Kuota free**: max **2** generate/hari (ditegakkan atomik di SQL).
* **i18n**: `X-Locale` → `Accept-Language` → GeoIP(ID→`id`, selainnya `en`).
* **CORS**: allow Nuxt ([http://localhost:3000](http://localhost:3000)) & Apps Script.
* **Logs**: `request_id`, `user_id`, `job_id`, `sql_uuid`, `duration_ms`, `route`, `provider`.
* `Makefile`: `run`, `worker`, `verify`, `test`.
* README diperbarui + contoh `curl` E2E.

---

## 4) Struktur Direktori

```
/cmd/api/                 # HTTP server (pgxpool)
/cmd/worker/              # worker job
/cmd/migrator/            # (opsional) goose wrapper

/internal/infra/          # config, logger, http server, db pool, jwt, geoip
/internal/http/httpapi/   # router.go (SATU file, prefix /v1)
/internal/http/handlers/  # App + handlers (method receiver *App)
/internal/middleware/     # auth_jwt, cors, i18n, requestid, logger, ratelimit
/db/migrations/           # 0001..000N.sql (dipakai goose)
/internal/db/sqlc/        # output sqlc (pgx/v5)
/internal/sqlinline/      # konstanta SQL (baris 1: --sql <UUIDv4>)
/internal/providers/      # image/nanobanana.go, video/veo.go, prompt/enhancer.go
/internal/domain/         # entity, enum, jsoncfg (PromptJSON, Watermark, dll.)
/internal/tools/sqllint/  # linter SQL-UUID
/pkg/zip/                 # util zip ([]byte, error)
```

---

## 5) DB & Migrasi

* Semua tabel inti: `id uuid pk default gen_random_uuid()`, `created_at/updated_at timestamptz default now()`, `properties jsonb default '{}'::jsonb`.
* **Tanpa FK**. Gunakan **INDEX** untuk `user_id`, `request_id`, `status`, `created_at`, dst.
* Fungsi PL/pgSQL (gunakan dollar-quoting konsisten, mis. `$fn$ ... $fn$;`):

  * `fn_consume_quota(user_id uuid, used int) returns table(remaining int)`
  * `fn_insert_job_and_usage(...) returns table(job_id uuid)`
* `vw_stats_summary` atau CTE fallback (kompatibel `task_type` **atau** `type`).
* Pastikan extension: `CREATE EXTENSION IF NOT EXISTS pgcrypto;`

---

## 6) sqlc (WAJIB pgx)

`sqlc.yaml` diselaraskan:

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: db/migrations
    queries: internal/db/queries # jika ada; jika full inline, bisa dikosongkan/abaikan
    gen:
      go:
        package: sqlc
        out: internal/db/sqlc
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
```

Jalankan:

```bash
sqlc generate
go mod tidy
```

---

## 7) Linter SQL-UUID (build blocker)

* `internal/tools/sqllint/main.go`:

  * Scan semua `.go`.
  * Deteksi konstanta SQL (heuristik: backtick + keyword `select|insert|update|delete|with`).
  * **Baris pertama** harus **`--sql <uuid-v4>`** (regex valid).
  * Ada pelanggaran → **exit non-zero**.

Integrasi di Makefile:

```make
verify: fmt vet lint sqllint test

sqllint:
	go run ./internal/tools/sqllint
```

**Pre-commit hook** (opsional):

```bash
printf '#!/bin/sh\nmake verify\n' > .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
```

---

## 8) Inline SQL (CTE, tanpa FK)

* Semua SQL ditempatkan di `/internal/sqlinline/*.go`.
* **WAJIB**: baris pertama SQL = `--sql <UUIDv4>` unik.
* Contoh enqueue image (skema ilustrasi, sesuaikan kolom real):

```sql
--sql 2caa5b21-4c2b-4b72-8a36-7d3d0f9b77a1
with input as (
  select
    $1::uuid  as user_id,
    $2::jsonb as prompt_json,
    $3::int   as quantity,
    $4::text  as aspect_ratio,
    $5::text  as provider
),
q as (
  select (fn_consume_quota((select user_id from input), (select quantity from input))).remaining
),
ins as (
  insert into generation_requests
    (id, user_id, type, status, prompt_json, quantity, aspect_ratio, provider, properties, created_at, updated_at)
  values
    (gen_random_uuid(), (select user_id from input), 'image_generate', 'queued',
     (select prompt_json from input), (select quantity from input),
     (select aspect_ratio from input), (select provider from input),
     '{}'::jsonb, now(), now())
  returning id
)
select id from ins;
```

> **Logging SQL:** bungkus eksekusi dengan helper yang mengekstrak `sql_uuid` dari baris pertama & logging sebelum/sesudah.

---

## 9) Middleware & Keamanan

* **CORS**: allow `http://localhost:3000` (Nuxt) & Apps Script origin.
* **Auth Google** (`/v1/auth/google/verify`):

  * Verifikasi JWKS, issuer, audience.
  * Upsert user (email, sub, name, picture, locale) tanpa FK.
  * Terbitkan **JWT internal** (HS256) → `{sub, plan, locale}`.
* **Auth JWT**: header `Authorization: Bearer <token>`.
* **i18n**: `X-Locale` → `Accept-Language` → **GeoIP** (ID→`id`, else `en`).
* **Rate-limit**: ≥ 30 req/menit per IP (chi/httprate).
* **RequestID**: log & propagate ke SQL logging.

---

## 10) JSON Contracts (type-safe)

`internal/domain/jsoncfg/`:

* `PromptJSON`: `title`, `product_type`, `style`, `background`, `instructions`,
  `watermark{enabled,text,position}`, `aspect_ratio`, `quantity`, `references[]`, `extras{locale,quality}`.
* `WatermarkConfig`, `EnhanceOptions`, `IdeaSuggestion`, `UsageEventPayload`, dsb.
* Validasi dengan `validator/v10`. Simpan mentah ke `generation_requests.prompt_json` (JSONB).
* Enforce **free plan**: `quantity <= 2` (validasi + SQL kuota atomik).

---

## 11) Endpoints & Router (SATU file)

`internal/http/httpapi/router.go` (prefix **/v1**):

```
GET  /healthz
POST /auth/google/verify
GET  /me

POST /prompts/enhance
POST /prompts/random
POST /prompts/clear

POST /images/generate
POST /images/enhance               # disarankan endpoint terpisah utk upscale/HD
GET  /images/{job_id}/status
GET  /images/{job_id}/assets
POST /images/{job_id}/zip

POST /ideas/from-image

POST /videos/generate              # target: async (enqueue) → worker
GET  /videos/{job_id}/status
GET  /videos/{job_id}/assets

GET  /assets
GET  /assets/{id}/download

GET  /stats/summary
POST /donations
GET  /donations/testimonials
```

> **Catatan:** Saat ini videos **sync** (stub), **TODO** pindah ke worker agar konsisten dengan images.

---

## 12) Worker & Providers

**Worker**

* Ambil job `queued` (`FOR UPDATE SKIP LOCKED`) → set `running` → panggil **provider** → simpan **assets** → update status.
* Log wajib: `job_id`, `provider`, `sql_uuid`, `duration_ms`.

**Providers**

* `internal/providers/image/nanobanana.go` (stub)
* `internal/providers/video/veo.go` (stub)
* `internal/providers/prompt/enhancer.go` (rule-based; gunakan locale)

> Stub boleh mengembalikan URL dummy. **Struktur** harus siap diganti provider AI nyata (Gemini/Nano Banana, Veo2/Veo3, dst).

---

## 13) ZIP Util

* `pkg/zip`: signature **`([]byte, error)`**, **cek semua error I/O**.
* Handler `POST /images/{job_id}/zip`:

  * Header:

    ```
    Content-Type: application/zip
    Content-Disposition: attachment; filename="<job_id>.zip"
    ```

---

## 14) OpenAPI/Swagger

* Sediakan **`/v1/openapi.json`** (boleh statik JSON) & **`/v1/docs`** (Redoc).
* Schemas: `PromptJSON`, `WatermarkConfig`, `EnqueueResponse`, `StatusResponse`, `Asset`, `Donation`, dsb.
* Update setiap menambah field baru di JSON contracts.

---

## 15) Testing (Minimal Wajib)

**Unit**

* Validasi `PromptJSON` (aspect ratio, quantity ≤ 2).
* i18n middleware (resolusi `id` vs `en`).
* JWT (sign/verify).

**Integration**

* DB test (goose up), `/v1/images/generate` → job **queued** + kuota berkurang.
* Worker jalan → status **succeeded** + assets tersedia.

`make verify` harus **hijau**.

---

## 16) Makefile & .env

Contoh Makefile (auto-load `.env`):

```make
GO ?= go
GOOSE ?= goose
MIGRATION_DIR := db/migrations

# auto load .env
ifneq (,$(wildcard ./.env))
include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env)
endif

.PHONY: run worker migrate fmt vet lint test sqllint verify

run:
	@set -a; . ./.env 2>/dev/null || true; set +a; \
	$(GO) run ./cmd/api

worker:
	@set -a; . ./.env 2>/dev/null || true; set +a; \
	$(GO) run ./cmd/worker

migrate:
	$(GOOSE) -dir $(MIGRATION_DIR) postgres "$(DATABASE_URL)" up

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required"; exit 1; }
	golangci-lint run

sqllint:
	$(GO) run ./internal/tools/sqllint

test:
	$(GO) test ./...

verify: fmt vet lint sqllint test
```

`.env` contoh:

```env
APP_ENV=development
PORT=1919
JWT_SECRET=awesome_secret_key
DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
CLERK_JWKS_URL=https://YOUR-CLERK-DOMAIN/.well-known/jwks.json
```

---

## 17) Contoh `curl` End-to-End

### Auth & Profile

```bash
# Login (Google ID token) → JWT
curl -i -X POST http://localhost:1919/v1/auth/google/verify \
  -H 'Content-Type: application/json' \
  -d '{"id_token":"<GOOGLE_ID_TOKEN>"}'

# Me
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/me
```

### Prompts

```bash
curl -i -H "Authorization: Bearer <JWT>" \
  -X POST http://localhost:1919/v1/prompts/enhance \
  -H 'Content-Type: application/json' \
  -d '{"prompt":{"title":"Nasi goreng premium","product_type":"food"}}'

curl -i -H "Authorization: Bearer <JWT>" -X POST http://localhost:1919/v1/prompts/random
curl -i -H "Authorization: Bearer <JWT>" -X POST http://localhost:1919/v1/prompts/clear
```

### Images (butuh worker)

```bash
# Enqueue generate
curl -i -H "Authorization: Bearer <JWT>" \
  -X POST http://localhost:1919/v1/images/generate \
  -H 'Content-Type: application/json' \
  -d '{
    "provider":"nanobanana",
    "quantity":1,
    "aspect_ratio":"1:1",
    "prompt":{
      "title":"Nasi goreng seafood premium",
      "product_type":"food",
      "style":"elegan",
      "background":"marble",
      "instructions":"Lighting lembut",
      "watermark":{"enabled":true,"text":"Brand Kamu","position":"bottom-right"},
      "references":[],
      "extras":{"locale":"id","quality":"hd"}
    }
  }'

# Status & assets
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/images/<JOB_ID>/status
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/images/<JOB_ID>/assets

# ZIP
curl -L -H "Authorization: Bearer <JWT>" \
  -X POST http://localhost:1919/v1/images/<JOB_ID>/zip -o images_<JOB_ID>.zip
```

### Videos (target async)

```bash
curl -i -H "Authorization: Bearer <JWT>" \
  -X POST http://localhost:1919/v1/videos/generate \
  -H 'Content-Type: application/json' \
  -d '{"provider":"veo2","prompt":{"title":"Hero shot ramen"}}'

curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/videos/<JOB_ID>/status
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/videos/<JOB_ID>/assets
```

### Ideas

```bash
curl -i -H "Authorization: Bearer <JWT)" \
  -X POST http://localhost:1919/v1/ideas/from-image \
  -H 'Content-Type: application/json' \
  -d '{"image_base64":"<base64>"}'
```

### Assets, Stats, Donations

```bash
curl -i -H "Authorization: Bearer <JWT>" "http://localhost:1919/v1/assets?limit=10&offset=0"
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/assets/<ASSET_ID>/download
curl -i http://localhost:1919/v1/stats/summary

curl -i -X POST http://localhost:1919/v1/donations \
  -H 'Content-Type: application/json' \
  -d '{"amount_int":50000,"note":"Keep going!"}'

curl -i http://localhost:1919/v1/donations/testimonials
```

---

## 18) Logging

* Zerolog + field wajib: `request_id`, `user_id`, `job_id`, `sql_uuid`, `duration_ms`, `route`, `provider`.
* Wrapper DB: ekstrak `sql_uuid` dari komentar SQL pertama → log sebelum/sesudah eksekusi.

---

## 19) Testing

* **Unit**: PromptJSON validation, i18n resolution, JWT sign/verify.
* **Integration**: `/images/generate` men-queue job & mengurangi kuota; worker menyelesaikan → status & assets valid.
* `make verify` harus **hijau**.

---

## 20) Troubleshooting

* **Migrate error `DATABASE_URL is required`**: pastikan Makefile include `.env` atau jalankan:

  ```bash
  set -a; . ./.env; set +a
  make migrate
  ```
* **`unterminated dollar-quoted string`**: fungsi PL/pgSQL harus `AS $fn$ ... $fn$;` (pembuka/penutup identik) + `LANGUAGE plpgsql`.
* **`errcheck`/lint**: jangan abaikan error I/O (selalu periksa).
* **sqllint fail**: pastikan **baris pertama** setiap konstanta SQL = `--sql <UUIDv4>`.

---

## 21) Rencana Commit (untuk Codex)

1. Router `/v1` tunggal & handler method-receiver; hapus duplikasi.
2. `sqlc.yaml` sinkron (schema → `db/migrations`) + `sqlc generate`.
3. SQL inline ber-UUID: enqueue, status, assets, stats, zip.
4. Worker: **videos → async** (pindah dari sync).
5. Providers stub (image/video/prompt) ber-interface; baca locale.
6. ZIP bytes + header download.
7. Stats SQL (view/CTE) final.
8. OpenAPI JSON + `/v1/docs`.
9. Tests (unit + integration).
10. Update README (file ini).

**Output terakhir** (wajib ditulis oleh Codex di PR):

* Daftar file dibuat/diubah (ringkas per commit).
* Perintah jalan:

  ```bash
  go mod tidy
  make migrate
  make run
  make worker
  ```
* Contoh curl end-to-end.
* Catatan singkat: cara kerja **sqllint**, cara menambah SQL baru (wajib `--sql <UUIDv4>`), dan pola rilis fitur lewat **`properties` JSONB + struct** (tanpa redesign schema).