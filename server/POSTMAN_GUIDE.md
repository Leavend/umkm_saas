# Panduan Pengujian API dengan Postman

Panduan ini menjelaskan cara menyiapkan lingkungan lokal, membuat token JWT untuk akun pengujian, dan menjalankan seluruh rute REST yang sudah tersedia di service backend UMKM SaaS memakai Postman.

## 1. Ringkasan Progres Layanan

Saat ini backend sudah memiliki fitur-fitur utama berikut:

- **Autentikasi Google dan profil pengguna** melalui endpoint `/v1/auth/google/verify` dan `/v1/me` yang memakai JWT dengan middleware Chi.【F:server/internal/http/httpapi/router.go†L26-L45】【F:server/internal/http/handlers/auth.go†L19-L126】
- **Manajemen prompt** untuk generate ide/konten gambar (`/v1/prompts/*`) lengkap dengan pencatatan usage event ke database melalui SQL inline.【F:server/internal/http/handlers/prompts.go†L16-L97】
- **Pipeline job gambar & video** meliputi enqueue, status, daftar aset, dan unduhan arsip ZIP.【F:server/internal/http/handlers/images.go†L17-L124】【F:server/internal/http/handlers/videos.go†L13-L86】
- **Manajemen aset dan statistik** guna melihat arsip hasil generate dan metrik agregasi untuk dashboard publik.【F:server/internal/http/handlers/assets.go†L13-L79】【F:server/internal/http/handlers/stats.go†L9-L26】
- **Donasi dan testimoni** untuk mendukung inisiatif monetisasi sederhana.【F:server/internal/http/handlers/donations.go†L12-L70】

Semua rute terdaftar di `internal/http/httpapi/router.go` dan dilindungi oleh middleware standar (RequestID, logging, CORS, rate limiting, serta JWT pada rute privat).【F:server/internal/http/httpapi/router.go†L13-L53】

## 2. Prasyarat Lingkungan Lokal

1. **Siapkan variabel lingkungan**:

   ```bash
   cd server
   cp .env.example .env
   # edit nilai berikut sesuai kebutuhan lokal
   # DATABASE_URL=postgres://postgres:postgres@localhost:5432/umkm_saas?sslmode=disable
   # JWT_SECRET=supersecret
   # STORAGE_BASE_URL=http://localhost:8080/static
   # GOOGLE_CLIENT_ID=<opsional untuk integrasi real>
   ```

2. **Jalankan PostgreSQL & migrasi**:

   ```bash
   docker compose up -d db
   make migrate
   ```

3. **Start HTTP API** (port default 8080):

   ```bash
   make run
   ```

   Untuk mengetes worker, gunakan `make worker` di terminal terpisah.

## 3. Membuat Akun Pengujian & Token JWT

Endpoint privat mewajibkan JWT. Bila belum memiliki Google ID Token nyata, buat akun dummy langsung di database lalu tandatangani JWT secara manual.

1. **Insert user dummy** (sesuaikan `email` dan `google_sub`):

   ```sql
   INSERT INTO users (
     id, clerk_user_id, email, plan, locale_pref, google_sub, properties, created_at, updated_at
   ) VALUES (
     gen_random_uuid(),
     'test-google-sub',
     'tester@example.com',
     'free',
     'id',
     'test-google-sub',
     jsonb_build_object(
       'quota_daily', 10,
       'quota_used_today', 0,
       'preferred_locale', 'id'
     ),
     now(),
     now()
   )
   RETURNING id;
   ```

   Simpan nilai `id` yang dikembalikan, karena akan dipakai sebagai `sub` token.

2. **Generate JWT** memakai skrip Go sederhana:

   ```bash
   cat <<'GOEOF' > /tmp/sign_jwt.go
   package main

   import (
           "fmt"
           "log"
           "os"
           "time"

           "server/internal/middleware"
   )

   func main() {
           secret := os.Getenv("JWT_SECRET")
           userID := os.Args[1]
           token, err := middleware.SignJWT(secret, middleware.TokenClaims{
                   Sub:      userID,
                   Plan:     "free",
                   Locale:   "id",
                   Exp:      time.Now().Add(24 * time.Hour).Unix(),
                   Issuer:   "umkm-saas",
                   Audience: "umkm-clients",
           })
           if err != nil {
                   log.Fatal(err)
           }
           fmt.Println(token)
   }
   GOEOF

   JWT_SECRET=supersecret go run /tmp/sign_jwt.go <USER_ID_HASIL_INSERT>
   ```

   Simpan output token tersebut sebagai variabel `jwt_token` di Postman Environment.

## 4. Menyiapkan Postman

1. Buat **Environment** baru bernama `UMKM SaaS` dengan variabel:
   - `base_url` = `http://localhost:8080/v1`
   - `jwt_token` = hasil token di atas
   - `job_id` = kosong (akan diisi setelah menjalankan enqueue job)

2. Untuk setiap request privat, tambahkan header:

   ```
   Authorization: Bearer {{jwt_token}}
   Content-Type: application/json
   ```

3. Untuk request publik (health, docs, stats summary) header Authorization tidak diperlukan.

## 5. Daftar Request & Contoh Payload

| Endpoint | Method | Auth | Deskripsi |
| --- | --- | --- | --- |
| `{{base_url}}/healthz` | GET | Tidak | Cek status layanan.【F:server/internal/http/handlers/health.go†L6-L9】 |
| `{{base_url}}/openapi.json` | GET | Tidak | Dump OpenAPI JSON (berguna untuk import otomatis Postman).【F:server/internal/http/httpapi/router.go†L28-L29】 |
| `{{base_url}}/docs` | GET | Tidak | Viewer dokumentasi Swagger UI yang di-serve backend.【F:server/internal/http/httpapi/router.go†L28-L29】 |
| `{{base_url}}/auth/google/verify` | POST | Tidak | Verifikasi Google ID Token dan menerima JWT baru.【F:server/internal/http/handlers/auth.go†L19-L94】 |
| `{{base_url}}/me` | GET | Ya | Mendapatkan profil user saat ini.【F:server/internal/http/handlers/auth.go†L96-L126】 |
| `{{base_url}}/prompts/enhance` | POST | Ya | Menyempurnakan prompt dan menghasilkan ide tambahan.【F:server/internal/http/handlers/prompts.go†L16-L97】 |
| `{{base_url}}/prompts/random` | POST | Ya | Mendapatkan daftar prompt random berdasarkan locale.【F:server/internal/http/handlers/prompts.go†L52-L90】 |
| `{{base_url}}/prompts/clear` | POST | Ya | Logging event clear prompt, respon HTTP 204.【F:server/internal/http/handlers/auth.go†L128-L136】 |
| `{{base_url}}/images/generate` | POST | Ya | Enqueue job generate gambar (async).【F:server/internal/http/handlers/images.go†L17-L74】 |
| `{{base_url}}/images/{{job_id}}/status` | GET | Ya | Melihat status job gambar.【F:server/internal/http/handlers/images.go†L76-L110】 |
| `{{base_url}}/images/{{job_id}}/assets` | GET | Ya | Mendapatkan daftar aset untuk job tertentu.【F:server/internal/http/handlers/images.go†L112-L152】 |
| `{{base_url}}/images/{{job_id}}/zip` | POST | Ya | Mengunduh arsip ZIP aset job.【F:server/internal/http/handlers/images.go†L154-L197】 |
| `{{base_url}}/ideas/from-image` | POST | Ya | Menghasilkan ide berdasarkan gambar base64.【F:server/internal/http/handlers/ideas.go†L12-L36】 |
| `{{base_url}}/videos/generate` | POST | Ya | Enqueue job video (async).【F:server/internal/http/handlers/videos.go†L13-L56】 |
| `{{base_url}}/videos/{{job_id}}/status` | GET | Ya | Status job video (reuse handler gambar).【F:server/internal/http/handlers/videos.go†L58-L61】 |
| `{{base_url}}/videos/{{job_id}}/assets` | GET | Ya | Daftar aset video.【F:server/internal/http/handlers/videos.go†L63-L86】 |
| `{{base_url}}/assets?limit=20&offset=0` | GET | Ya | Daftar seluruh aset milik user saat ini.【F:server/internal/http/handlers/assets.go†L13-L53】 |
| `{{base_url}}/assets/{{asset_id}}/download` | GET | Ya | Mendapatkan signed URL untuk unduh aset.【F:server/internal/http/handlers/assets.go†L55-L79】 |
| `{{base_url}}/stats/summary` | GET | Tidak | Statistik agregasi publik.【F:server/internal/http/handlers/stats.go†L9-L26】 |
| `{{base_url}}/donations` | POST | Opsional | Mencatat donasi pengguna jika login (user ID opsional).【F:server/internal/http/handlers/donations.go†L15-L47】 |
| `{{base_url}}/donations/testimonials` | GET | Tidak | Menampilkan testimoni dan daftar donasi terbaru.【F:server/internal/http/handlers/donations.go†L49-L70】 |

### Contoh Payload JSON

- **Prompts Enhance**

  ```json
  {
    "prompt": {
      "title": "Nasi goreng seafood premium",
      "product_type": "food",
      "style": "elegan",
      "background": "marble",
      "instructions": "Lighting lembut",
      "watermark": {
        "enabled": true,
        "text": "Warung Nasgor Bapak",
        "position": "bottom-right"
      },
      "references": [],
      "extras": {
        "locale": "id",
        "quality": "hd"
      }
    }
  }
  ```

- **Images Generate**

  ```json
  {
    "provider": "gemini",
    "quantity": 1,
    "aspect_ratio": "1:1",
    "prompt": {
      "title": "Nasi goreng seafood premium",
      "product_type": "food",
      "style": "elegan",
      "background": "marble",
      "instructions": "Lighting lembut",
      "watermark": {
        "enabled": true,
        "text": "Warung Nasgor Bapak",
        "position": "bottom-right"
      },
      "references": [],
      "extras": {
        "locale": "id",
        "quality": "hd"
      }
    }
  }
  ```

- **Videos Generate**

  ```json
  {
    "provider": "veo2",
    "prompt": "Hero shot ramen",
    "locale": "id"
  }
  ```

- **Ideas From Image**

  ```json
  {
    "image_base64": "<base64-encoded-string>"
  }
  ```

- **Donations Create**

  ```json
  {
    "amount": 25000,
    "note": "Dukungan pengembangan fitur baru",
    "testimonial": "Fitur AI-nya membantu banget!"
  }
  ```

## 6. Urutan Uji Coba yang Direkomendasikan

1. Jalankan request publik (`healthz`, `stats/summary`, `donations/testimonials`) untuk memastikan server hidup.
2. Gunakan JWT dummy untuk memanggil `/v1/me` dan memverifikasi setup akun.
3. Uji grup **Prompts** lalu cek tabel `usage_events` di database untuk memastikan logging terjadi.
4. Jalankan **Images Generate** dan **Videos Generate** kemudian pantau tabel `generation_requests` & `assets` untuk melihat status berubah dari `QUEUED` → `SUCCEEDED` (butuh worker aktif dan provider mock).
5. Setelah job berhasil, pakai endpoint assets dan ZIP untuk memvalidasi akses file.
6. Tutup dengan mengetes alur donasi.

Dengan langkah di atas, seluruh rute API dapat diverifikasi secara menyeluruh menggunakan Postman tanpa tergantung kredensial eksternal produksi.
