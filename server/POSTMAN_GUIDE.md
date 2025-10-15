# Panduan Pengujian API dengan Postman

Panduan ini membantu menyiapkan lingkungan lokal, membuat token JWT untuk akun pengujian, serta menjalankan seluruh rute REST yang tersedia di backend UMKM SaaS melalui Postman.

## 1. Ringkasan Progres Layanan

- **Autentikasi Google & profil pengguna**: backend memverifikasi Google ID Token, melakukan upsert user, menandatangani JWT, lalu mengekspos profil lengkap dengan kuota harian dari kolom JSONB melalui `/v1/me`.【F:server/internal/http/httpapi/router.go†L33-L39】【F:server/internal/http/handlers/auth.go†L32-L125】【F:server/internal/sqlinline/users.go†L1-L120】
- **Manajemen prompt**: rute `/v1/prompts/*` menormalisasi input, memanggil enhancer (Gemini/OpenAI/static), serta mencatat usage event ke tabel `usage_events` beserta metadata latensi.【F:server/internal/http/httpapi/router.go†L41-L45】【F:server/internal/http/handlers/prompts.go†L27-L140】【F:server/internal/sqlinline/usage.go†L3-L5】
- **Pipeline gambar sinkron**: pengguna dapat mengunggah aset referensi, lalu memanggil `/v1/images/generate` yang selalu menggunakan DashScope `qwen-image-edit`, menyimpan jejak job ke tabel `image_jobs`, serta menyediakan status dan endpoint unduhan (single maupun ZIP) berbasis proxy.【F:server/internal/http/httpapi/router.go†L47-L53】【F:server/db/migrations/0007_create_image_jobs.sql†L1-L24】【F:server/internal/http/handlers/images.go†L305-L660】【F:server/internal/imagegen/instruction.go†L8-L42】
- **Pipeline video**: enqueue, status, dan daftar aset video tetap tersedia dan berbagi mekanisme validasi kepemilikan job yang sama dengan pipeline gambar.【F:server/internal/http/httpapi/router.go†L59-L63】【F:server/internal/http/handlers/videos.go†L27-L138】
- **Inventaris aset & unduhan**: `/v1/assets` melakukan paginasi aset milik user, sedangkan `/v1/assets/{id}/download` mengembalikan URL final (storage lokal atau absolut) setelah verifikasi kepemilikan.【F:server/internal/http/httpapi/router.go†L65-L68】【F:server/internal/http/handlers/assets.go†L14-L85】【F:server/internal/http/handlers/app.go†L200-L276】
- **Statistik publik & monetisasi**: `/v1/stats/summary` menarik data agregasi, sementara `/v1/donations` dan `/v1/donations/testimonials` menyimpan serta menampilkan testimoni donasi terbaru.【F:server/internal/http/httpapi/router.go†L70-L72】【F:server/internal/http/handlers/stats.go†L9-L24】【F:server/internal/http/handlers/donations.go†L18-L74】【F:server/internal/sqlinline/stats.go†L3-L11】【F:server/internal/sqlinline/donations.go†L3-L13】

> Catatan: router otomatis menyajikan statik `/static/*` ketika `STORAGE_PATH` disetel, sehingga hasil proxy atau salinan lokal dapat diakses melalui URL yang sama dengan respons assets.【F:server/internal/http/httpapi/router.go†L24-L31】【F:server/internal/http/handlers/app.go†L265-L276】

## 2. Prasyarat Lingkungan Lokal

1. **Konfigurasi environment**
   - Salin `.env.example`, kemudian isi minimal `DATABASE_URL` dan `JWT_SECRET`. Variabel lain seperti `STORAGE_BASE_URL`, `STORAGE_PATH`, serta kredensial provider (Gemini/Qwen/OpenAI) memiliki default di kode tetapi dapat dioverride sesuai kebutuhan.【F:server/README.md†L10-L25】【F:server/.env.example†L1-L16】【F:server/internal/infra/config.go†L41-L67】
2. **Menyiapkan database & dependensi**
   - Jalankan PostgreSQL lokal dengan ekstensi `pgcrypto`, unduh modul Go, lalu eksekusi migrasi Goose melalui Makefile.【F:server/README.md†L5-L31】
3. **Menjalankan layanan**
   - Jalankan `make run` untuk API (port default mengikuti `$PORT`, bawaan `8080`). Pipeline gambar bersifat sinkron, namun `make worker` tetap diperlukan jika Anda ingin menguji job video yang diproses async. Pastikan direktori storage dapat ditulisi agar endpoint upload dan unduhan berbasis proxy berjalan lancar.【F:server/README.md†L27-L74】【F:server/internal/infra/config.go†L43-L48】【F:server/internal/http/handlers/images.go†L44-L660】

## 3. Membuat Akun Pengujian & Token JWT

Endpoint privat memerlukan JWT. Jika belum memiliki Google ID Token valid:

1. **Insert user dummy** (ubah email dan `google_sub` sesuai kebutuhan). Contoh berikut sudah menambahkan metadata kuota harian.

   ```sql
   INSERT INTO users (
     id, clerk_user_id, email, name, plan, locale_pref,
     google_sub, properties, created_at, updated_at
   ) VALUES (
     gen_random_uuid(),
     'test-google-sub',
     'tester@example.com',
     'Tester QA',
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

   Tabel `users` mendukung kolom `google_sub`, `properties`, dan `updated_at` sesuai migrasi terbaru.【F:server/db/migrations/0001_init.sql†L4-L14】【F:server/db/migrations/0004_properties_and_functions.sql†L1-L23】

2. **Generate JWT** memakai helper `middleware.SignJWT`:

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

   Simpan output token sebagai variabel `jwt_token` di Postman.【F:server/internal/middleware/auth_jwt.go†L18-L75】

## 4. Menyiapkan Postman

1. Buat **Environment** `UMKM SaaS` berisi:
  - `base_url` = `http://localhost:${PORT:-8080}/v1` (sesuaikan dengan nilai `$PORT`).
   - `jwt_token` = token hasil langkah sebelumnya.
   - `upload_asset_id` = akan diisi dari respons `/images/uploads` jika Anda menguji workflow enhance.
   - `job_id` = kosong; akan diisi setelah enqueue job.
2. Tambahkan header berikut untuk setiap request privat:

   ```
   Authorization: Bearer {{jwt_token}}
   Content-Type: application/json
   ```

3. Request publik seperti health, docs, stats, dan testimonials tidak membutuhkan header Authorization.

## 5. Daftar Request & Contoh Penggunaan

| Endpoint | Method | Auth | Deskripsi |
| --- | --- | --- | --- |
| `{{base_url}}/healthz` | GET | Tidak | Cek status layanan (mengembalikan `{"status":"ok"}`).【F:server/internal/http/handlers/health.go†L7-L9】 |
| `{{base_url}}/openapi.json` | GET | Tidak | Mengambil spesifikasi OpenAPI statis untuk import koleksi otomatis.【F:server/internal/http/httpapi/router.go†L33-L36】 |
| `{{base_url}}/docs` | GET | Tidak | Viewer dokumentasi Redoc yang di-serve backend.【F:server/internal/http/httpapi/router.go†L33-L36】 |
| `{{base_url}}/auth/google/verify` | POST | Tidak | Verifikasi Google ID Token, upsert user, lalu balas JWT + profil.【F:server/internal/http/handlers/auth.go†L32-L99】 |
| `{{base_url}}/me` | GET | Ya | Profil user saat ini beserta kuota harian dari JSONB properties.【F:server/internal/http/handlers/auth.go†L101-L125】 |
| `{{base_url}}/prompts/enhance` | POST | Ya | Normalisasi prompt, panggil enhancer sesuai konfigurasi, dan catat usage event.【F:server/internal/http/handlers/prompts.go†L27-L87】 |
| `{{base_url}}/prompts/random` | POST | Ya | Ambil kumpulan prompt acak per locale dan log provider yang dipakai.【F:server/internal/http/handlers/prompts.go†L89-L120】 |
| `{{base_url}}/prompts/clear` | POST | Ya | Mencatat event pembersihan prompt; respon 204 tanpa body.【F:server/internal/http/handlers/auth.go†L158-L166】 |
| `{{base_url}}/images/uploads` | POST | Ya (multipart) | Unggah gambar referensi (maks 12 MB, field `file`); backend memvalidasi format, menyimpan file ke `$STORAGE_PATH`, lalu menuliskan entri aset yang dapat direferensikan ulang.【F:server/internal/http/handlers/images.go†L44-L153】 |
| `{{base_url}}/images/generate` | POST | Ya | Mengedit gambar secara sinkron menggunakan DashScope `qwen-image-edit`, menyimpan log job ke Postgres, serta mengembalikan `job_id` dan URL hasil dalam respons yang sama. Pastikan host upload lokal ditambahkan ke `IMAGE_SOURCE_HOST_ALLOWLIST` bila ingin menggunakan URL `localhost`.【F:server/internal/http/handlers/images.go†L305-L457】【F:server/internal/infra/config.go†L41-L87】 |
| `{{base_url}}/images/jobs/{{id}}` | GET | Ya | Mengambil status, payload prompt, dan output job yang tersimpan di tabel `image_jobs`.【F:server/internal/http/handlers/images.go†L460-L524】 |
| `{{base_url}}/images/{{job_id}}/download` | GET | Ya | Proxy-download file pertama dari hasil job yang telah `SUCCEEDED`.【F:server/internal/http/handlers/images.go†L526-L590】 |
| `{{base_url}}/images/{{job_id}}/download.zip` | GET | Ya | Mengunduh seluruh URL hasil job dalam bentuk arsip ZIP yang di-stream langsung dari DashScope.【F:server/internal/http/handlers/images.go†L592-L660】 |
| `{{base_url}}/ideas/from-image` | POST | Ya | Validasi base64 image dan mengembalikan dua ide dummy untuk demo UX.【F:server/internal/http/handlers/ideas.go†L15-L35】 |
| `{{base_url}}/videos/generate` | POST | Ya | Enqueue video job; pastikan provider salah satu kunci Gemini yang didukung App.【F:server/internal/http/handlers/videos.go†L20-L54】【F:server/internal/http/handlers/app.go†L215-L232】 |
| `{{base_url}}/videos/{{job_id}}/status` | GET | Ya | Menggunakan handler status gambar untuk job video.【F:server/internal/http/handlers/videos.go†L56-L58】 |
| `{{base_url}}/videos/{{job_id}}/assets` | GET | Ya | Daftar aset video dengan metadata lengkap.【F:server/internal/http/handlers/videos.go†L60-L105】 |
| `{{base_url}}/assets?limit=20&offset=0` | GET | Ya | Paginasi seluruh aset milik user berdasarkan `created_at` desc.【F:server/internal/http/handlers/assets.go†L14-L55】 |
| `{{base_url}}/assets/{{asset_id}}/download` | GET | Ya | Menghasilkan URL unduhan (lokal atau absolute) setelah verifikasi kepemilikan.【F:server/internal/http/handlers/assets.go†L58-L85】 |
| `{{base_url}}/stats/summary` | GET | Tidak | Statistik agregasi publik (total user, jumlah job, sukses/gagal, 24 jam terakhir).【F:server/internal/http/handlers/stats.go†L9-L25】 |
| `{{base_url}}/donations` | POST | Opsional | Mencatat donasi; `user_id` otomatis diisi bila JWT tersedia.【F:server/internal/http/handlers/donations.go†L18-L40】 |
| `{{base_url}}/donations/testimonials` | GET | Tidak | Daftar testimoni dan donasi terbaru (limit 10).【F:server/internal/http/handlers/donations.go†L42-L74】 |

## 6. Contoh Payload & Request

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

- **Images Upload (multipart)**

  Gunakan form-data dengan key `file` (type File) dan opsional `mode`, `background_theme`, `enhance_level`.

  ```bash
  curl -X POST "http://localhost:${PORT:-8080}/v1/images/uploads" \
    -H "Authorization: Bearer $JWT" \
    -F "file=@/path/to/reference.png" \
    -F "mode=product" -F "background_theme=marble" -F "enhance_level=medium"
  ```

- **Contoh Respons Upload** — simpan `asset_id` dan `url` sebagai variabel Postman (`upload_asset_id`, `upload_asset_url`).
- **Catatan penting**: URL yang dikembalikan perlu dapat diakses publik (bukan `localhost` atau jaringan privat) agar DashScope
  dapat mengunduhnya saat proses edit. Pada lingkungan lokal, unggah file ke storage publik atau gunakan URL eksternal yang Anda
  kontrol untuk pengujian endpoint generate.

  ```json
  {
    "asset_id": "a3f1f4f2-52a5-4d3b-8d2e-6db6a85f8d2b",
    "storage_key": "uploads/USER_ID/1709898888123456.png",
    "mime": "image/png",
    "bytes": 123456,
    "width": 1024,
    "height": 1024,
    "aspect_ratio": "1:1",
    "url": "http://localhost:${PORT:-8080}/static/uploads/USER_ID/1709898888123456.png"
  }
  ```

- **Images Generate (DashScope Edit)**

  ```json
  {
    "provider": "qwen-image-plus",
    "quantity": 1,
    "aspect_ratio": "1:1",
    "prompt": {
      "title": "Nasi goreng seafood premium",
      "product_type": "food",
      "style": "elegan",
      "background": "marble",
      "instructions": "Lighting lembut",
      "watermark": { "enabled": false },
      "references": [],
      "source_asset": {
        "asset_id": "{{upload_asset_id}}",
        "url": "{{upload_asset_url}}"
      },
      "extras": {
        "locale": "id",
        "quality": "hd",
        "negative_prompt": "blurry"
      }
    }
  }
  ```

## 7. Alur Uji Workflow Gambar

Workflow gambar kini sinkron dan mengembalikan URL hasil secara langsung, namun jejak job tetap tersimpan di database untuk audit dan unduhan berikutnya:

1. Lakukan upload aset referensi, lalu simpan `asset_id` dan `url` yang dikembalikan Postman sebagai variabel lingkungan.【F:server/internal/http/handlers/images.go†L44-L153】
2. Panggil `/v1/images/generate` menggunakan `prompt.source_asset` yang sama; backend otomatis memetakan provider kompatibel ke `qwen-image-edit`, membuat entri `image_jobs`, dan menjalankan maksimum dua permintaan paralel ke DashScope.【F:server/internal/http/handlers/images.go†L305-L457】
3. Respons 201 sudah menyertakan `job_id` dan array `images`. Jika perlu metadata lengkap atau ingin mengunduh ulang setelah URL DashScope kedaluwarsa, akses `GET /v1/images/jobs/{id}` atau endpoint unduhan yang tersedia.【F:server/internal/http/handlers/images.go†L460-L660】
4. Router tetap mengekspos folder storage lokal via `/static/*` apabila Anda menyimpan salinan manual; namun untuk permintaan default backend mem-proxy URL dari DashScope ketika klien memanggil endpoint download.【F:server/internal/http/httpapi/router.go†L24-L31】【F:server/internal/http/handlers/images.go†L526-L660】

- **Videos Generate**

  ```json
  {
    "provider": "gemini-2.5-flash",
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

## 8. Urutan Uji Coba yang Direkomendasikan

1. Jalankan request publik (`healthz`, `stats/summary`, `donations/testimonials`) untuk memastikan API dan koneksi database aktif.【F:server/internal/http/handlers/health.go†L7-L9】【F:server/internal/http/handlers/stats.go†L9-L25】【F:server/internal/http/handlers/donations.go†L42-L74】
2. Pastikan JWT valid dengan memanggil `/v1/me`; cek kuota harian sesuai nilai yang diinsert pada langkah persiapan.【F:server/internal/http/handlers/auth.go†L101-L125】
3. Uji seluruh grup **Prompts** dan verifikasi log usage di tabel `usage_events` untuk memastikan pipeline audit bekerja.【F:server/internal/http/handlers/prompts.go†L27-L140】【F:server/internal/sqlinline/usage.go†L3-L5】
4. Jalankan **Images Upload** lalu **Images Generate (DashScope Edit)** untuk memastikan respons sinkron berisi `job_id` dan URL hasil; lanjutkan dengan `GET /images/jobs/{id}` bila ingin memeriksa payload yang tersimpan.【F:server/internal/http/handlers/images.go†L44-L524】
5. Gunakan endpoint `download` maupun `download.zip` guna memverifikasi alur proxy dari DashScope serta integritas metadata aset yang sama di `/v1/assets`.【F:server/internal/http/handlers/images.go†L526-L660】【F:server/internal/http/handlers/assets.go†L14-L85】【F:server/internal/http/handlers/app.go†L265-L276】
6. Uji **Videos Generate** untuk memastikan provider video siap serta worker memproses antrean yang sama.【F:server/internal/http/handlers/videos.go†L27-L138】
7. Terakhir, jalankan alur donasi dan pastikan data muncul di `/donations/testimonials` sebagai sanity check monetisasi.【F:server/internal/http/handlers/donations.go†L18-L74】

Dengan alur di atas, seluruh permukaan API dapat divalidasi end-to-end memakai Postman tanpa ketergantungan kredensial produksi.
