# Delcom Blog — Backend (Golang Gin REST API)

REST API platform publikasi artikel multi-tier dengan **Clean Architecture & Feature-First**,
PostgreSQL 17, JWT, Swagger UI, serta **AI Blog Generator** yang terintegrasi Search Engine + OpenAI Chat Completions.

## Persiapan

```bash
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"
cp .env.example .env   # sesuaikan OPENAI_KEY / OPENAI_MODEL
```

Saat start, tabel `users` & `posts` dimigrasikan otomatis, akun admin (`ADMIN_USERNAME` / `ADMIN_PASSWORD`)
dan satu artikel sambutan di-seed jika belum ada.

## Menjalankan (port 8081)

```bash
go run cmd/server/main.go
# [GIN-debug] Listening and serving HTTP on :8081
```

- Swagger UI: http://localhost:8081/swagger/index.html
- Spesifikasi OpenAPI JSON: http://localhost:8081/api.json

`main.go` di root identik dengan `cmd/server/main.go` agar `go build -o server .` (pipeline Jenkins) menghasilkan biner server.

## Pengujian (100% statement coverage)

```bash
go test -v -cover ./internal/...
```

Pengujian memakai SQLite in-memory dan `httptest` (tanpa PostgreSQL / layanan eksternal).

## Regenerasi Swagger

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
swag init -g cmd/server/main.go -o docs --parseInternal && cp docs/swagger.json docs/api.json
```

## Endpoint

| Method | Endpoint | Akses |
| --- | --- | --- |
| GET | `/api/public/categories`, `/api/public/posts?q=&category=`, `/api/public/posts/:slug` | Publik |
| POST | `/api/auth/register`, `/api/auth/login` | Publik |
| POST/GET/PUT | `/api/auth/logout`, `/api/auth/profile` | Login |
| GET/POST | `/api/posts`, `/api/posts/stats` | Penulis |
| GET/PUT/DELETE | `/api/posts/:id` | Penulis |
| PATCH | `/api/posts/:id/toggle-publish` | Penulis |
| POST | `/api/posts/ai-generate` | Penulis |
| GET | `/api/admin/users`, `/api/admin/posts`, `/api/admin/users/:id/posts` | Admin |

Semua respons memakai format `{status, message, data?}`; rute tak dikenal → `404 Endpoint not found`.
