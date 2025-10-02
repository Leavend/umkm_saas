Siap! Ini **SUPER-PROMPT** sekali tempel untuk Codex (paste di root repo `server/`, mis. di `README_Codex.md`). Tujuannya: **menyelesaikan seluruh backend** sesuai visimu — **Go sebagai wrapper**, DDD + Clean Code, **inline SQL + CTE**, **tanpa FK**, **properties JSONB**, **linter SQL-UUID (build blocker)**, semua **endpoint fitur** aktif, worker, migrator, logging, i18n, kuota, dan siap dipakai Nuxt.

> Cara pakai: buka proyek di VS Code, buka file ini, panggil Codex, lalu biarkan Codex menjalankan perubahan bertahap (commit per tahap).

---

# PROMPT untuk Codex — Selesaikan Backend SaaS “Food Photography Naik Kelas Pake AI” (Go Wrapper + Inline SQL)

## Peran & Prinsip

Kamu **Principal Backend Engineer**. Selesaikan proyek ini sampai **siap produksi** dengan prinsip berikut:

1. **Go hanya wrapper**: transport (HTTP), validasi, auth, logging, CORS, i18n, kuota. **Logic bisnis & agregasi data terutama di DB** via **inline SQL/CTE** dan fungsi SQL bila perlu.
2. **Tanpa ORM, tanpa FOREIGN KEY**. Semua relasi ditangani di inline SQL + **INDEX**.
3. Semua tabel memiliki **UUID v4 PK**, **properties JSONB**, **created_at/updated_at**.
4. **Setiap inline SQL WAJIB diawali `--sql <UUIDv4>`** sebagai marker untuk audit log. Buat **linter build-blocking** yang gagal jika ada SQL tanpa marker.
5. **pgx/v5 + sqlc (pgx mode)**. Koneksi: **pgxpool**.
6. **Clean Architecture / DDD** yang sudah ada dipertahankan, tapi implementasi repo/usecase diarahkan ke inline SQL.

---

## Target Akhir (Definition of Done)

* Semua **endpoint** fitur terhubung DB (bukan dummy) & terdokumentasi (Swagger).
* **Router tunggal** `internal/http/httpapi/router.go` dengan prefix `/v1`.
* **Linter SQL-UUID** aktif di `make verify` (gagal bila ada SQL tanpa `--sql UUID`).
* **Worker** memproses job `queued → running → succeeded/failed`.
* **Kuota**: user free **max 2** per hari untuk generate images (enforced di SQL atomik).
* **i18n**: auto ID/EN via `X-Locale` → `Accept-Language` → GeoIP.
* **CORS**: allow Nuxt (localhost:3000) & Apps Script.
* **Log** memasukkan `request_id`, `user_id`, `job_id`, `sql_uuid`, `duration_ms`.
* `Makefile` punya target: `run`, `worker`, `verify`, `test`.
* Tersedia **contoh curl** untuk semua endpoint inti.
* README diperbarui.

---

## Arsitektur & Folder (lengkapi/rapikan)

Pertahankan struktur, tambahkan bila perlu:

```
/cmd/api/              # HTTP server (sudah ada, pakai pgxpool)
/cmd/worker/           # worker job generator (isi sekarang)
/cmd/migrator/         # jalankan migrasi (goose atau runner kustom)

/internal/infra/       # config, logger, http server, db pool (pgxpool), jwt, geoip
/internal/http/httpapi/ router.go (SATU versi, prefix /v1)
/internal/http/handlers/ App container + handlers (method receiver *App)
/internal/middleware/   auth_jwt.go, cors.go, i18n.go, requestid.go, logger.go, ratelimit.go
/internal/db/migrations/ 0001..000N.sql (tambahkan migrasi baru)
/internal/db/sqlc/     # hasil generate sqlc (pgx/v5)
/internal/sqlinline/   # konstanta inline SQL (dengan --sql UUID)
/internal/providers/   image/nanobanana.go (stub), video/veo.go (stub), prompt/enhancer.go (stub)
/internal/domain/      # entity, enum, jsoncfg (struct utk JSON prompt, watermark, dll.)
/internal/tools/sqllint/ main.go (linter SQL-UUID)
/pkg/zip/              # util zip all assets
```

---

## DB & Migrasi (tambahkan migrasi baru)

* **Syarat kolom** (di semua tabel inti):
  `id UUID PK`, `created_at timestamptz default now()`, `updated_at timestamptz default now()`, `properties JSONB default '{}'::jsonb`.
* **Tidak ada FK**. Buat **INDEX** untuk `user_id`, `job_id`, `request_id`, dll.
* Pastikan tabel inti:
  `users`, `external_accounts`, `generation_requests`, `assets`, `watermarks`, `prompt_templates`, `usage_events`, `donations`.
* Tambah **fungsi SQL** bila perlu (disarankan):

  * `fn_consume_quota(user_id uuid, used int)` → sisa kuota.
  * `fn_queue_job_image(...)` → contoh pipeline insert job & usage.
* Tambah **VIEW/MAT VIEW** untuk ringkasan 24h (dashboard).

---

## Konfigurasi sqlc (WAJIB pgx)

Ubah `sqlc.yaml` agar:

```
gen:
  go:
    sql_package: "pgx/v5"
    package: sqlc
    out: internal/db/sqlc
    emit_json_tags: true
    emit_interface: true
```

Jalankan `sqlc generate`.

---

## Linter SQL-UUID (build blocker)

Buat `/internal/tools/sqllint/main.go`:

* Scan semua `.go`.
* Temukan **const** SQL (heuristik: backtick + keyword `select|insert|update|delete|with`).
* **Baris pertama SQL harus `--sql <uuid-v4>`**. Validasi regex v4.
* Cetak daftar pelanggaran dan **exit non-zero** kalau ada.

Integrasikan di `Makefile`:

```
verify: fmt vet lint sqllint test
sqllint:
go run ./internal/tools/sqllint
```

Tambahkan **pre-commit hook** untuk `make verify`.

---

## Inline SQL (CTE, tanpa FK)

* Semua konstanta SQL diletakkan di `/internal/sqlinline/*.go`, **baris pertama** selalu:

  ```
  --sql 8a9c9d3e-6a2f-4c86-9aab-bb2b2a1d2f77
  ```
* Contoh **enqueue image job** (buat nyata):

  ```go
  const QEnqueueImageJob = `--sql 2caa5b21-4c2b-4b72-8a36-7d3d0f9b77a1
  with
  input as (
    select
      $1::uuid  as user_id,
      $2::jsonb as prompt_json,
      $3::int   as quantity,
      $4::text  as aspect_ratio,
      $5::text  as provider
  ),
  quota as (
    -- kurangi kuota di users.properties (tanpa FK, atomic in TX)
    update users u
    set properties = jsonb_set(
      u.properties,
      '{quota_used_today}',
      (coalesce((u.properties->>'quota_used_today')::int,0) + (select quantity from input))::text::jsonb,
      true
    ),
    updated_at = now()
    where u.id = (select user_id from input)
    returning u.id
  ),
  ins_job as (
    insert into generation_requests
      (id, user_id, type, status, prompt_json, quantity, aspect_ratio, provider, properties)
    values (gen_random_uuid(), (select user_id from input), 'image_generate', 'running',
            (select prompt_json from input), (select quantity from input),
            (select aspect_ratio from input), (select provider from input), '{}'::jsonb)
    returning id
  )
  select id from ins_job;
  `
  ```
* Semua eksekusi SQL **dibungkus** helper yang **mengekstrak `sql_uuid`** dari komentar pertama dan **log** sebelum/sesudah.

---

## Middleware & Keamanan

* **CORS**: allow `http://localhost:3000` (Nuxt) + Apps Script origin.
* **Auth Google**:

  * `POST /v1/auth/google/verify { id_token }` → verifikasi JWKS Google, audience, issuer.
  * Upsert user (email, sub, name, picture, locale) tanpa FK.
  * Terbitkan **JWT internal** (HS256) → `{sub, plan, locale}`.
* **Auth JWT**: middleware `Authorization: Bearer <token>`.
* **i18n**: `X-Locale` → `Accept-Language` → GeoIP(ID→`id`, else `en`), inject ke context.
* **Rate limit**: 30 req/menit per IP (chi/httprate).
* **RequestID**: simpan ke context & log; propagate ke SQL logging.

---

## JSON Contracts (type-safe)

Buat `internal/domain/jsoncfg/`:

* `PromptJSON` (title, product_type, style, background, instructions, watermark{enabled,text,position}, aspect_ratio, quantity, references[], extras{locale,quality})
* `WatermarkConfig`, `EnhanceOptions`, `PromptVariant`, `IdeaSuggestion`, `UsageEventPayload`, dsb sesuai kebutuhan.
  Validasi pakai `validator/v10`. Simpan mentah ke `generation_requests.prompt_json` (JSONB).

---

## Endpoints (aktifkan semua di satu router `/v1`)

Ubah **semua handler jadi method receiver** `func (a *App) …` agar akses `a.DB/a.Q`.
`internal/http/httpapi/router.go` (hapus versi duplikat lama; gunakan satu file):

```
/v1
  GET  /healthz                          -> app.Health
  POST /auth/google/verify               -> app.AuthGoogleVerify
  GET  /me                               -> app.Me (JWT)

  POST /prompts/enhance                  -> app.PromptEnhance
  POST /prompts/random                   -> app.PromptRandom
  POST /prompts/clear                    -> app.PromptClear (204 + usage_event)

  POST /images/generate                  -> app.ImagesGenerate (JWT, quota ≤ 2)
  POST /images/enhance                   -> app.ImagesEnhance
  GET  /images/{job_id}/status           -> app.ImageStatus
  GET  /images/{job_id}/assets           -> app.ImageAssets
  POST /images/{job_id}/zip              -> app.ImageZip

  POST /ideas/from-image                 -> app.IdeasFromImage

  POST /videos/generate                  -> app.VideosGenerate
  GET  /videos/{job_id}/status           -> app.VideoStatus
  GET  /videos/{job_id}/assets           -> app.VideoAssets

  GET  /assets                           -> app.ListAssets
  GET  /assets/{id}/download             -> app.DownloadAsset

  GET  /stats/summary                    -> app.StatsSummary
  POST /donations                        -> app.DonationsCreate
  GET  /donations/testimonials           -> app.DonationsTestimonials
```

**Catatan:**

* **Images** `provider`: `gemini` (default) & **`nanobanana`** (stub).
* **Videos** `provider`: **`veo2` / `veo3`** (stub).
* **Quantity** enforce ≤2 (plan free) di SQL atomik (CTE quota).
* **Aspect ratio**: `1:1|4:3|3:4|16:9|9:16`.

---

## Worker

* `cmd/worker`: loop ambil job `queued` (SELECT … FOR UPDATE SKIP LOCKED / CTE), set `running`, panggil provider **stub** (return URL dummy), insert assets, set `succeeded/failed`.
* Tulis inline SQL ber-marker untuk semua tahap.
* Logging menyertakan `job_id` & `sql_uuid`.

---

## Logging

* Zerolog: **sempurnakan** field: `request_id`, `user_id`, `job_id`, `sql_uuid`, `duration_ms`, `route`, `provider`.
* Helper `dbexec/dbquery`: ekstrak `sql_uuid` dari komentar pertama SQL, masukkan ke log.

---

## Analytics & Dashboard 24H

* Ringkas metrik: `Total Visitor, Active Online User, AI Request, Last 24H, Videos Generated, Images Generated, Request Success, Request Fail`.
* Implementasi via **VIEW atau CTE** dari `usage_events`, `generation_requests`, `assets`.
* Endpoint `GET /v1/stats/summary`.

---

## Makefile, Lint, Test

Buat/rapikan:

```
run:
go run ./cmd/api
worker:
go run ./cmd/worker
migrate:
goose -dir internal/db/migrations postgres "$(DATABASE_URL)" up
fmt:
gofmt -w .
vet:
go vet ./...
lint:
golangci-lint run
sqllint:
go run ./internal/tools/sqllint
test:
go test ./...
verify: fmt vet lint sqllint test
```

---

## Contoh CURL (siapkan di README)

```
# Health
curl -i http://localhost:1919/v1/healthz

# Auth (verifikasi id_token Google)
curl -i -X POST http://localhost:1919/v1/auth/google/verify \
  -H 'Content-Type: application/json' \
  -d '{"id_token":"<GOOGLE_ID_TOKEN>"}'

# Me (pakai JWT internal)
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/me

# Generate image
curl -i -X POST http://localhost:1919/v1/images/generate \
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

# Cek status & assets
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/images/<JOB_ID>/status
curl -i -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/images/<JOB_ID>/assets

# Zip semua hasil
curl -i -X POST -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/images/<JOB_ID>/zip

# Ideas dari image
curl -i -X POST -H "Authorization: Bearer <JWT>" http://localhost:1919/v1/ideas/from-image \
  -H 'Content-Type: application/json' -d '{"image_base64":"..."}'

# Stats summary
curl -i http://localhost:1919/v1/stats/summary
```

---

## Gaya Kode & Error Shape

* Response JSON, header `application/json; charset=utf-8`.
* Error konsisten:

```
{"error":{"code":"bad_request","message":"invalid prompt"}}
```

* Gunakan **method receiver `*handlers.App`** untuk semua handler yang butuh DB.
* Pastikan **router hanya satu** (hapus file router duplikat lama).

---

## Output yang harus kamu berikan (di akhir pengerjaan)

1. **Daftar file yang dibuat/diubah** per commit (ringkas).
2. Perintah untuk menjalankan:

   * `go mod tidy`
   * `make migrate`
   * `make run` dan `make worker`
   * contoh `curl` (sudah di README).
3. Catatan teknis singkat: bagaimana linter SQL-UUID bekerja, cara menambah SQL baru, dan cara menambah fitur (cukup tambah kolom `properties` & JSON struct tanpa redesign schema).

> **Jangan** memasukkan kredensial. Gunakan variabel `.env`: `APP_ENV`, `PORT`, `DATABASE_URL`, `JWT_SECRET`, `GEOIP_DB_PATH`, `STORAGE_BASE_URL`, dll.

---

Selesai. Jalankan ini bertahap, commit per langkah (infra → migrasi → linter → router → auth → images → videos → prompts → ideas → assets → stats → worker → docs).

Pastikan tetap clean code & best practice DDD
