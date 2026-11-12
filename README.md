# File Storage

Portal arsip dokumen kantor yang menyimpan berkas di object storage S3 (RustFS). Dibuat sebagai modular monolith: satu binary Go berisi API dan UI TypeScript yang sudah di-*build*, dijalankan berdampingan dengan RustFS lewat Docker Compose.

![Tampilan aplikasi](docs/screenshot.png)

## Arsitektur

```
Browser ──HTTP──> file_storage_app (Go, :8000) ──S3 API──> file_storage_rustfs (:9000)
                   ├─ /api/*   JSON API                     └─ volume rustfs_data
                   └─ /        UI (TypeScript, embedded)
```

Aplikasi tidak menyimpan berkas di disknya sendiri. Semua dokumen menjadi objek di bucket RustFS, jadi container aplikasi bisa dihapus dan dibuat ulang tanpa kehilangan data.

## Struktur

```
cmd/server/            entrypoint: baca config, siapkan storage, jalankan HTTP server
internal/config/       baca dan validasi environment variable (fail fast)
internal/storage/      client S3: list, put, get, delete, ensure bucket
internal/httpapi/      router, handler JSON API, middleware log, serve UI
internal/webui/        embed hasil build frontend ke binary
web/                   frontend TypeScript + Vite (Bun)
  src/api.ts           client API bertipe, upload dengan progress
  src/ui/              komponen: uploader, tabel dokumen, notifikasi, dialog
  src/icons.ts         ikon SVG
Dockerfile             3 tahap: build UI (Bun) → build Go → image runtime Alpine
docker-compose.yml     app + RustFS, semua nilai dari .env
```

## Konfigurasi

Semua konfigurasi lewat `.env` (salin dari `.env.example`). File `.env` tidak ikut ke Git.

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `RUSTFS_ACCESS_KEY` | ya | | Access key RustFS, dipakai juga oleh aplikasi |
| `RUSTFS_SECRET_KEY` | ya | | Secret key RustFS |
| `S3_BUCKET` | ya | | Nama bucket, dibuat otomatis saat start |
| `S3_REGION` | tidak | `us-east-1` | Region untuk tanda tangan SigV4 |
| `MAX_UPLOAD_MB` | tidak | `64` | Batas ukuran satu kali unggah |
| `APP_PORT` | tidak | `8000` | Port aplikasi di host |
| `S3_API_PORT` | tidak | `9000` | Port S3 API RustFS di host |
| `S3_CONSOLE_PORT` | tidak | `9001` | Port RustFS Console di host |
| `RUSTFS_IMAGE` | tidak | `rustfs/rustfs:latest` | Image RustFS |

Kalau variabel wajib kosong, `docker compose` berhenti dengan pesan yang menyebut variabelnya. Binary Go juga menolak start jika `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, atau `S3_SECRET_KEY` tidak di-set.

## Menjalankan di server (Ubuntu + Docker)

Prasyarat: Docker Engine dan Docker Compose v2. Port 8000, 9000, dan 9001 harus kosong.

```bash
git clone https://github.com/zhafrandzaky/file-storage.git
cd file-storage
cp .env.example .env
nano .env                       # ganti RUSTFS_ACCESS_KEY dan RUSTFS_SECRET_KEY
docker compose up -d --build
```

Cek:

```bash
docker compose ps               # app harus (healthy)
docker compose logs app         # "bucket created" lalu "listening"
curl -s localhost:8000/api/health
```

Akses dari browser:

- Aplikasi: `http://<IP_SERVER>:8000`
- RustFS Console: `http://<IP_SERVER>:9001` (login dengan access key dan secret key dari `.env`)

### Lewat Portainer

Portainer tidak bisa mem-*build* dari folder di host, jadi build image-nya dulu:

```bash
docker compose build
```

Lalu di Portainer: Stacks → Add stack → tempel isi `docker-compose.yml`, **hapus baris `build: .`**, dan isi variabel `.env` di bagian Environment variables. Deploy the stack.

### Persiapan sebelum lab tanpa internet

Jalankan sekali saat masih ada internet. Semua image dan dependency akan tersimpan di server:

```bash
docker compose pull rustfs
docker compose build
docker pull amazon/aws-cli:latest
```

## API

| Method | Path | Fungsi |
|---|---|---|
| `GET` | `/api/health` | Status aplikasi dan koneksi ke bucket |
| `GET` | `/api/info` | Nama bucket, endpoint, batas unggah |
| `GET` | `/api/files` | Daftar dokumen, terbaru di atas |
| `POST` | `/api/files` | Unggah (multipart, field `files`, bisa banyak) |
| `GET` | `/api/files/{nama}` | Buka dokumen; tambahkan `?download` untuk mengunduh |
| `DELETE` | `/api/files/{nama}` | Hapus dokumen |

Contoh:

```bash
curl -F "files=@laporan.pdf" localhost:8000/api/files
curl localhost:8000/api/files
curl -X DELETE localhost:8000/api/files/laporan.pdf
```

Error selalu berbentuk `{"error": {"code": "...", "message": "..."}}`.

### Verifikasi langsung ke RustFS dengan AWS CLI

```bash
set -a; . ./.env; set +a
docker run --rm --network container:file_storage_rustfs \
  -e AWS_ACCESS_KEY_ID=$RUSTFS_ACCESS_KEY -e AWS_SECRET_ACCESS_KEY=$RUSTFS_SECRET_KEY \
  amazon/aws-cli:latest --endpoint-url http://127.0.0.1:9000 s3 ls s3://$S3_BUCKET
```

## Pengembangan lokal

```bash
# backend (butuh RustFS atau S3 lain yang berjalan)
S3_ENDPOINT=http://localhost:9000 S3_BUCKET=office-documents \
S3_ACCESS_KEY=... S3_SECRET_KEY=... go run ./cmd/server

# frontend dengan hot reload, proxy /api ke :8000
cd web && bun install && bun run dev
```

Tes:

```bash
go test ./...
cd web && bun run typecheck
```

## Troubleshooting

| Gejala | Penyebab dan solusi |
|---|---|
| `required variable ... is missing` | `.env` belum dibuat atau variabel kosong. Jalankan `cp .env.example .env` lalu isi. |
| `port is already allocated` | Port dipakai container lain. Cek `docker ps`, atau ganti `APP_PORT` / `S3_API_PORT` di `.env`. |
| App restart terus, log `ensure bucket` | RustFS belum siap atau kredensial salah. Cek `docker compose logs rustfs` dan nilai di `.env`. |
| Halaman tidak terbuka dari host | Firewall: `sudo ufw allow 8000,9000,9001/tcp`. |
| Build gagal di `bun install` / `go mod download` | Server tidak punya internet saat build. Build sebelum masuk lab. |

## Menghentikan

```bash
docker compose stop          # hentikan, data tetap ada
docker compose down          # hapus container, data tetap ada di volume
docker compose down -v       # HAPUS SEMUA DATA di bucket
```
