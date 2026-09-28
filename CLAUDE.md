# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`geo-post-api` — a single Go service backing a hyper-local social media app ("geo-post"). Built incrementally, step by step:

1. Project skeleton + DB/cache connections
2. Zero-Trust JWT auth middleware
3. Event creation — writes Postgres + Redis `GEOADD`
4. Proximity feed — reads via Redis `GEOSEARCH`, hydrated from Postgres
5. Media upload — presigned MinIO PUT URLs, attached to events via `media_url`

Layered, flat-at-root layout (not the Go-standard `cmd/`/`internal/` split): `main.go` at the repo root is the only composition root, and business code is organized by layer — `controllers` (HTTP) → `service` (business logic) → `repo` (data access) — plus `middlewares`, `models`, `config`, `db`.

Stack: Go, [Gin](https://github.com/gin-gonic/gin) (HTTP), [GORM](https://gorm.io) with the Postgres driver, [go-redis/v9](https://github.com/redis/go-redis), [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt), [google/uuid](https://github.com/google/uuid), [joho/godotenv](https://github.com/joho/godotenv), [minio-go/v7](https://github.com/minio/minio-go).

## Commands

```bash
go build ./...                                          # compile everything
go vet ./...                                              # static checks
go run .                                                   # run the service
go test ./...                                              # run tests
go test ./middlewares/... -v -run TestRequireAuth   # just the auth middleware tests
```

Copy `.env.example` to `.env` and fill in real values — `main.go`'s `init()` loads `.env` automatically (see Architecture below), so plain `go run .` just works; no manual `export`/`source` needed. Anything already exported in the shell still takes priority over `.env`. `JWT_PUBLIC_KEY_PATH` is optional — it defaults to `cert/public_key.pem`, resolved via `runtime.Caller` against `config/config.go`'s own file location rather than the process's working directory, so it resolves the same way no matter what directory `go run .` is invoked from.

## Architecture

One binary, one `main.go` at the repo root, layered top-level packages instead of `cmd/`/`internal/`:

```
main.go            composition root: wiring only
config/            env-var config, incl. JWT key default-path resolution
db/                Postgres + Redis + MinIO constructors, AutoMigrate
models/            GORM model structs
repo/              data access — Postgres (GORM) + Redis (geo index)
service/           business logic, sits between controllers and repo
controllers/       Gin handlers (HTTP layer)
middlewares/        Zero-Trust JWT auth
cert/              RSA public key loaded at boot
postman/           API collection — keep in sync with routes (see below)
```

- `main.go` — the single composition root, split into `init()` and `main()`:
  - `init()` runs before `main()` and does all bootstrap work: loads `.env` via [`github.com/joho/godotenv`](https://github.com/joho/godotenv) (`loadEnvFile()` tries `godotenv.Load()` against cwd first, then falls back to this file's own directory; `godotenv` never overrides a key already set in the real environment), calls `config.Load()`, loads the JWT public key, and opens + pings Postgres and Redis, storing the results in package-level vars (`cfg`, `pg`, `rdb`, `sqlDB`). Any failure here is `log.Fatalf` — every external connection is made and verified before the server can accept a single request.
  - `main()` assumes everything from `init()` is already live; it just builds repos/services from `pg`/`rdb`, wires the Gin engine (routes, middleware), and starts listening. Nothing else should own startup/wiring logic.
- `config/config.go` — `Config` struct + `Load()`, reading `PORT`, `DB_URL`, `REDIS_URL`, `JWT_PUBLIC_KEY_PATH`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL` via `os.Getenv`. `PORT` defaults to `8080`. `JWT_PUBLIC_KEY_PATH` defaults to `cert/public_key.pem` (see Commands above for how that default resolves). MinIO vars default to `localhost:9000` / `minioadmin` / `minioadmin` / `false` (a local dev instance). `DB_URL`/`REDIS_URL` are required and not validated here (callers get connection errors if they're wrong/empty).
- `db/` — one constructor per dependency, each independently testable/swappable:
  - `postgres.go`: `NewPostgres(dsn) (*gorm.DB, error)` — opens via `gorm.io/driver/postgres`, sets pool limits (`SetMaxOpenConns(25)`, `SetMaxIdleConns(10)`, `SetConnMaxLifetime(30m)`) on the underlying `*sql.DB`, pings to fail fast on bad connections, then `AutoMigrate`s `models.LocalEvent`.
  - `redis.go`: `NewRedis(ctx, url) (*redis.Client, error)` — parses the URL with `redis.ParseURL` (so `redis://user:pass@host:port/db` works directly) and pings with a 5s timeout.
  - `minio.go`: `NewMinio(cfg) (*minio.Client, error)` — connects to an **existing** MinIO instance (never spins one up), ensures `MediaBucket` (`"pulse-media"`, exported so `repo`/`service` reference the same constant) exists, and sets a public-read bucket policy on it so uploaded media is viewable without auth. Writes still require a presigned URL — only `s3:GetObject` is public.
- `models/event.go` — `LocalEvent` GORM model: `ID` (UUID), `UserID` (a plain `string`, not `uuid.UUID` — it stores the JWT `sub` claim verbatim, since real-world issuers hand out non-UUID subs, e.g. this project's auth-server uses numeric ids), `Title`, `Description`, `Category`, `Latitude`, `Longitude`, `CreatedAt`, `ExpiresAt`, `MediaURL` (`*string` — a pointer so an absent value is a real SQL `NULL` and is omitted from JSON via `omitempty`, not serialized as `""`). `BeforeCreate` fills in a client-side UUID and defaults `ExpiresAt` to now+24h when unset.
- `repo/` — one file per aggregate:
  - `event_repo.go`: `EventRepository.CreateEvent` — writes to Postgres via GORM, then `GEOADD`s the event's UUID into `events:geo`. If the Redis step fails after a successful Postgres insert, the error is returned as-is (no outbox pattern — kept simple deliberately).
  - `feed_repo.go`: `FeedRepository.GetNearbyEvents(ctx, lat, lng, radiusKm)` — `GEOSEARCH`es (not the deprecated `GEORADIUS`) `events:geo` for ids within `radiusKm`, then hydrates matching rows from Postgres via `WHERE id IN (...)`, ordered `created_at DESC`. Returns `[]models.LocalEvent{}` (not `nil`) when nothing matches.
  - `upload_repo.go`: `UploadRepository.PresignedPutURL(ctx, objectName, expiry)` — thin wrapper around `minio.Client.PresignedPutObject` against `db.MediaBucket`.
- `service/` — one file per aggregate, sitting between controllers and repo. `EventService`/`FeedService` are currently thin pass-throughs to the matching repo method — this is where business logic grows as the app does; controllers should never call `repo` directly. `upload_service.go`'s `UploadService.GenerateUploadURL(ctx, ext)` is the one exception with real logic: it builds a unique `uuid.ext` object name, gets a presigned PUT URL from the repo (valid 15 min, `uploadURLExpiry`), and separately builds the public `media_url` from `cfg.MinioEndpoint`/`MinioUseSSL` — the presigned URL and the public URL are constructed independently, so don't assume they're always derived from each other.
- `controllers/` — one file per route, calling `service`, never `repo`:
  - `event_controller.go`: `CreateEventHandler(svc)` binds a DTO (lat/lng validated via `min`/`max` binding tags, deliberately *not* `binding:"required"` since `0` is a valid coordinate; `media_url` is optional), injects `user_id` from the Gin context, calls the service, returns `201`/`400`/`500`.
  - `feed_controller.go`: `GetFeedHandler(svc)` parses `lat`/`lng`/`radius` (default `5.0`) from query params, calls the service, returns `200` with the JSON array (or `400` on bad params).
  - `upload_controller.go`: `GenerateUploadURL(svc)` reads `?ext=`, validates it against `allowedUploadExtensions` (`jpg`/`jpeg`/`png`/`gif`/`webp` — this exists because `ext` otherwise flows straight into a storage object key from an unvalidated query param), calls the service, returns `{"presigned_url", "media_url"}` or `400`/`500`.
- `middlewares/auth.go` — Zero-Trust JWT verification (no calls to an external auth service at request time):
  - `LoadPublicKey(path string) error` parses an RSA public key PEM into a package-level `*rsa.PublicKey`. Call once at boot, before the server accepts traffic — the middleware assumes it's already set and does not lazily load or lock around it.
  - `RequireAuth() gin.HandlerFunc` reads `Authorization: Bearer <token>`, parses/verifies via `golang-jwt/jwt/v5` against that key, and rejects (401) unless the signing method is actually RSA (checked by asserting `*jwt.SigningMethodRSA` in the keyfunc, not just the `alg` string — this is what blocks `alg=none`/algorithm-confusion tokens) and the token has a non-empty `sub` claim. On success it sets `user_id` in the Gin context via `c.Set`.
  - `middlewares/auth_test.go` covers valid tokens, missing header, wrong signing key, `alg=none`, and expired tokens — run this file's tests after touching auth logic.
- `cert/public_key.pem` — the RSA public key loaded at boot (see `config/config.go` for the default path resolution).

`/health` in `main.go` pings both Postgres and Redis and returns 503 if either is down — treat it as a real liveness check, not a static 200, when adding infra that should also gate readiness.

`/api/v1` is the protected route group (`router.Group` + `middlewares.RequireAuth()`); add new authenticated endpoints under it. Unauthenticated endpoints go directly on `router`, alongside `/health`.

## Postman

`postman/geo-post-api.postman_collection.json` — a single self-contained collection file (variables `base_url`/`jwt_token` live in its own `variable` array, no separate environment file) — documents every route, with example success and error responses. **Whenever a route is added, removed, or its request/response shape changes, update this collection in the same change** — folders are organized to mirror the router (`Health` = unauthenticated root routes, `Protected (api/v1)` = the `/api/v1` group, which carries folder-level Bearer auth using `{{jwt_token}}`).

## Conventions to follow as the service grows

- Keep `main.go` as pure wiring — new business logic belongs in `service/`, HTTP concerns in `controllers/`, data access in `repo/`.
- Controllers call service, service calls repo — never skip a layer (e.g. a controller calling repo directly), even when a service method is currently just a pass-through.
- Each `db/` constructor returns `(client, error)` and pings before returning, so `main.go` can `log.Fatalf` on startup rather than fail silently at first request.
- Config stays a flat struct of env-derived strings in `config/`; don't reach for a config library/YAML unless the env var list grows unwieldy.
- Auth stays Zero-Trust: verification must only ever depend on the locally-loaded RSA public key, never a network round-trip to an auth service during request handling.
