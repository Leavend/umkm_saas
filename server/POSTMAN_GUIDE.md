# Panduan Pengujian API dengan Postman

Panduan ini membantu menyiapkan lingkungan lokal, membuat token JWT untuk akun pengujian, serta menjalankan seluruh rute REST yang tersedia di backend UMKM SaaS melalui Postman.

## 1. Ringkasan Progres Layanan

- **Autentikasi Google & profil pengguna**: backend dapat memverifikasi Google ID Token, melakukan upsert user, menandatangani JWT, dan mengekspos profil lewat `/v1/me` dengan kuota harian dari kolom JSONB.【F:server/internal/http/httpapi/router.go†L33-L45】【F:server/internal/http/handlers/auth.go†L32-L166】【F:server/internal/sqlinline/users.go†L1-L120】
- **Manajemen prompt**: endpoint `/v1/prompts/*` melakukan normalisasi input, memanggil enhancer (Gemini/OpenAI/static), dan mencatat usage event ke tabel `usage_events`.【F:server/internal/http/httpapi/router.go†L41-L45】【F:server/internal/http/handlers/prompts.go†L27-L140】【F:server/internal/sqlinline/usage.go†L3-L5】
- **Pipeline media**: pengguna dapat mengunggah aset referensi, enqueue generate/enhance image, melihat status & daftar aset, serta mengunduh ZIP hasil job; worker memanfaatkan provider Qwen/Gemini yang sudah dipetakan pada App.【F:server/internal/http/httpapi/router.go†L47-L54】【F:server/internal/http/handlers/images.go†L44-L466】【F:server/internal/http/handlers/app.go†L195-L232】
- **Pipeline video**: enqueue, status, dan daftar aset video tersedia dan berbagi mekanisme validasi kepemilikan job dengan pipeline gambar.【F:server/internal/http/httpapi/router.go†L60-L64】【F:server/internal/http/handlers/videos.go†L20-L105】
- **Inventaris aset & unduhan**: `/v1/assets` melakukan paginasi aset milik user, dan `/v1/assets/{id}/download` mengembalikan signed URL lokal yang dihasilkan dari konfigurasi storage path/base URL.【F:server/internal/http/httpapi/router.go†L66-L69】【F:server/internal/http/handlers/assets.go†L14-L85】【F:server/internal/http/handlers/app.go†L199-L265】
- **Statistik publik & monetisasi**: `/v1/stats/summary` menarik data agregasi dari view `vw_stats_summary`, sementara `/v1/donations` dan `/v1/donations/testimonials` menyimpan serta menampilkan testimoni donasi.【F:server/internal/http/httpapi/router.go†L71-L73】【F:server/internal/http/handlers/stats.go†L9-L25】【F:server/internal/http/handlers/donations.go†L18-L74】【F:server/internal/sqlinline/stats.go†L3-L11】【F:server/internal/sqlinline/donations.go†L3-L13】

> Catatan: router otomatis menyajikan statik `/static/*` ketika `STORAGE_PATH` disetel, sehingga hasil worker dapat diakses langsung melalui URL yang sama dengan respons assets.【F:server/internal/http/httpapi/router.go†L28-L31】【F:server/internal/http/handlers/app.go†L253-L265】

## 2. Prasyarat Lingkungan Lokal

1. **Konfigurasi environment**
   - Salin `.env.example`, kemudian isi minimal `DATABASE_URL` dan `JWT_SECRET`. Variabel lain seperti `STORAGE_BASE_URL`, `STORAGE_PATH`, serta kredensial provider (Gemini/Qwen/OpenAI) memiliki default di kode tetapi dapat dioverride sesuai kebutuhan.【F:server/README.md†L10-L25】【F:server/.env.example†L1-L16】【F:server/internal/infra/config.go†L41-L67】
2. **Menyiapkan database & dependensi**
   - Jalankan PostgreSQL lokal dengan ekstensi `pgcrypto`, unduh modul Go, lalu eksekusi migrasi Goose melalui Makefile.【F:server/README.md†L5-L31】
3. **Menjalankan layanan**
   - Jalankan `make run` untuk API (port default mengikuti `$PORT`, bawaan `8080`) dan `make worker` agar job async berubah status menjadi `SUCCEEDED/FAILED`. Pastikan direktori storage dapat ditulisi agar endpoint upload/ZIP berjalan lancar.【F:server/README.md†L27-L70】【F:server/internal/infra/config.go†L43-L48】【F:server/internal/http/handlers/images.go†L44-L154】

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
   - `base_url` = `http://localhost:8080/v1` (atau sesuaikan dengan `$PORT`).
   - `jwt_token` = token hasil langkah sebelumnya.
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
| `{{base_url}}/images/uploads` | POST | Ya (multipart) | Unggah gambar referensi (maks 12 MB, field `file`) dan simpan metadata tambahan seperti mode/theme/enhance level.【F:server/internal/http/handlers/images.go†L44-L154】 |
| `{{base_url}}/images/generate` | POST | Ya | Enqueue job gambar dengan validasi kuota & provider (default `qwen-image-plus`).【F:server/internal/http/handlers/images.go†L297-L347】 |
| `{{base_url}}/images/enhance` | POST | Ya | Alias ke `/images/generate` untuk skenario enhance prompt.<br>Gunakan payload yang sama.【F:server/internal/http/handlers/images.go†L489-L491】 |
| `{{base_url}}/images/{{job_id}}/status` | GET | Ya | Lihat status job, provider, quantity, dan metadata lain.【F:server/internal/http/handlers/images.go†L349-L369】 |
| `{{base_url}}/images/{{job_id}}/assets` | GET | Ya | Daftar aset (URL, dimensi, properties) milik job yang sama dan user terkait.【F:server/internal/http/handlers/images.go†L370-L424】 |
| `{{base_url}}/images/{{job_id}}/zip` | POST | Ya | Mengarsipkan semua aset job menjadi ZIP dan mengirim sebagai attachment.【F:server/internal/http/handlers/images.go†L426-L466】 |
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
  curl -X POST "http://localhost:8080/v1/images/uploads" \
    -H "Authorization: Bearer $JWT" \
    -F "file=@/path/to/reference.png" \
    -F "mode=product" -F "background_theme=marble" -F "enhance_level=medium"
  ```

- **Images Generate / Enhance**

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

## 7. Urutan Uji Coba yang Direkomendasikan

1. Jalankan request publik (`healthz`, `stats/summary`, `donations/testimonials`) untuk memastikan API dan koneksi database aktif.【F:server/internal/http/handlers/health.go†L7-L9】【F:server/internal/http/handlers/stats.go†L9-L25】【F:server/internal/http/handlers/donations.go†L42-L74】
2. Pastikan JWT valid dengan memanggil `/v1/me`; cek kuota harian sesuai nilai yang diinsert pada langkah persiapan.【F:server/internal/http/handlers/auth.go†L101-L125】
3. Uji seluruh grup **Prompts** dan verifikasi log usage di tabel `usage_events` untuk memastikan pipeline audit bekerja.【F:server/internal/http/handlers/prompts.go†L27-L140】【F:server/internal/sqlinline/usage.go†L3-L5】
4. Jalankan **Images Upload** diikuti **Images Generate/Enhance**, kemudian pantau status job hingga `SUCCEEDED` sembari memastikan file tersimpan di `$STORAGE_PATH` dan dapat diakses via `/static/...` maupun endpoint assets.【F:server/internal/http/handlers/images.go†L44-L466】【F:server/internal/http/handlers/app.go†L253-L265】
5. Setelah job sukses, gunakan `assets`, `download`, serta `zip` untuk memverifikasi akses file dan integritas metadata.【F:server/internal/http/handlers/assets.go†L14-L85】【F:server/internal/http/handlers/images.go†L426-L466】
6. Uji **Videos Generate** untuk memastikan provider video siap serta worker memproses antrean yang sama.【F:server/internal/http/handlers/videos.go†L20-L105】
7. Terakhir, jalankan alur donasi dan pastikan data muncul di `/donations/testimonials` sebagai sanity check monetisasi.【F:server/internal/http/handlers/donations.go†L18-L74】

Dengan alur di atas, seluruh permukaan API dapat divalidasi end-to-end memakai Postman tanpa ketergantungan kredensial produksi.
