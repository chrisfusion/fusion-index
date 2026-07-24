# API Architecture

This document describes the internal design of **fusion-index**: how an HTTP request flows through the system, how the layers are structured, and how the data model is organized.

---

## Table of Contents

1. [System Context](#system-context)
2. [Layered Architecture](#layered-architecture)
3. [Request Lifecycle](#request-lifecycle)
4. [Data Model](#data-model)
5. [API Surface](#api-surface)
6. [Storage Abstraction](#storage-abstraction)
7. [Database Layer (sqlc)](#database-layer-sqlc)
8. [Transaction Strategy](#transaction-strategy)
9. [Error Handling](#error-handling)
10. [Configuration and Startup](#configuration-and-startup)
11. [Health Probes](#health-probes)
12. [OpenAPI Spec](#openapi-spec)

---

## System Context

```
┌──────────────────────────────────────────────────────────────┐
│                      Fusion Platform                         │
│                                                              │
│   fusion-spectra ──► fusion-index  ◄── CLI / CI pipelines   │
│   (orchestrator)      (this svc)                             │
└──────────────────────────────────────────────────────────────┘
                              │
                   ┌──────────┴──────────┐
                   ▼                     ▼
             PostgreSQL            S3 / Filesystem
           (metadata store)       (artifact bytes)
```

**fusion-index** is a stateless REST service. All persistent state lives in PostgreSQL (metadata) and in S3 or the local filesystem (artifact bytes). Multiple replicas can run concurrently against the same database.

---

## Layered Architecture

```
┌─────────────────────────────────────────────────┐
│               HTTP (Gin router)                 │  router.go
├─────────────────────────────────────────────────┤
│     OpenAPI spec + Swagger UI (embedded)        │  openapi/handler.go
├─────────────────────────────────────────────────┤
│            Request / Response DTOs              │  dto/requests.go
│             (binding + validation)              │  dto/responses.go
├─────────────────────────────────────────────────┤
│                  Handlers                       │  handlers/*.go
│   (orchestrate queries, tx, storage calls)      │
├─────────────────────────────────────────────────┤
│            DB Access (sqlc)                     │  db/sqlc/*.go
│          pgxpool.Pool / Queries                 │
├─────────────────────────────────────────────────┤
│           Storage Interface                     │  storage/storage.go
│      FilesystemBackend | S3Backend              │  storage/filesystem.go
│                                                 │  storage/s3.go
├─────────────────────────────────────────────────┤
│                PostgreSQL                       │
└─────────────────────────────────────────────────┘
```

| Layer | Responsibility |
|-------|---------------|
| Router | Route registration, CORS, middleware |
| OpenAPI | Embedded spec + Swagger UI served from binary |
| DTOs | Bind + validate request bodies; shape response JSON |
| Handlers | Business logic, transaction management, error mapping |
| sqlc | Type-safe SQL execution; zero raw queries outside this layer |
| Storage | Binary blob persistence, abstracted behind an interface |

---

## Request Lifecycle

### Read request (`GET /api/v1/artifacts/{id}`)

```
Client
  │
  ▼
Gin router  →  match route, extract {id}
  │
  ▼
ArtifactHandler.Get
  ├── pathID(c)                       — parse + validate int64 path param
  ├── q.GetRegistryArtifact(ctx, id)  — sqlc query → pgxpool
  ├── pgx.ErrNoRows                  → 404 {"error": "artifact not found"}
  └── ToArtifactResponse(row)        → 200 {id, fullName, description, ...}
```

### Write request with transaction (`POST /api/v1/artifacts`)

```
Client
  │
  ▼
ArtifactHandler.Create
  ├── c.ShouldBindJSON(&req)               — validate body (fullName required)
  ├── pool.Begin(ctx)                      — open transaction
  │     ├── q.GetRegistryArtifactByName()  — duplicate check → 409 if exists
  │     └── q.CreateRegistryArtifact(...)  — insert row
  ├── tx.Commit(ctx)
  └── ToArtifactResponse(row) → 201
```

### Version create with tags (`POST /api/v1/artifacts/{id}/versions`)

```
Client
  │
  ▼
VersionHandler.Create
  ├── c.ShouldBindJSON(&req)               — validate (version required, semver format)
  ├── semver.Parse(req.Version)            — parse major.minor.patch
  ├── pool.Begin(ctx)
  │     ├── q.GetRegistryArtifact()        — 404 if artifact missing
  │     ├── q.CreateArtifactVersion(...)   — 409 on unique(artifact_id, major, minor, patch)
  │     └── q.UpsertArtifactTag(...)  ×N   — atomically assign each tag
  ├── tx.Commit(ctx)
  └── ToVersionResponse(version, tagRows) → 201
```

### File upload (`POST /api/v1/artifacts/{id}/versions/{semver}/files`)

```
Client  ──multipart/form-data──►  FileHandler.Upload
  │
  ├── resolveVersion(c)              — parse artifactID + semver, look up version row
  ├── c.Request.FormFile("file")     — read multipart header (no body buffering)
  ├── q.CreateArtifactFile("pending") — insert DB row, status=PENDING
  ├── unique violation (23505)       → 409 (duplicate filename for this version)
  ├── compute storagePath            — "{artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}"
  ├── storage.Store(path, reader)    — stream bytes to backend
  │     └── error → q.UpdateArtifactFileStatus(ERROR)  → 500
  ├── q.UpdateArtifactFileStored(path) — set status=AVAILABLE + real path
  │     └── error → q.UpdateArtifactFileStatus(ERROR) + storage.Delete(path) → 500
  └── ToFileResponse(record) → 201 {id, name, status: "AVAILABLE", downloadUrl, ...}
```

---

## Data Model

```
registry_artifact
  id           BIGINT   PK (sequence, increment 50)
  full_name    VARCHAR(500)  UNIQUE NOT NULL          ← "org.team.name"
  description  TEXT
  created_at   TIMESTAMPTZ
  updated_at   TIMESTAMPTZ
       │
       │  1 : N
       ▼
registry_artifact_version
  id           BIGINT   PK
  artifact_id  BIGINT   FK → registry_artifact.id  ON DELETE CASCADE
  major        INT      NOT NULL
  minor        INT      NOT NULL
  patch        INT      NOT NULL
  config       TEXT                                  ← raw JSON or YAML
  created_at   TIMESTAMPTZ
  UNIQUE (artifact_id, major, minor, patch)
       │
       │  1 : N
       ▼
registry_artifact_file
  id               BIGINT   PK
  version_id       BIGINT   FK → registry_artifact_version.id  ON DELETE CASCADE
  name             TEXT     NOT NULL
  content_type     TEXT
  size_bytes       BIGINT
  storage_backend  TEXT     NOT NULL                 ← "FILESYSTEM" or "S3"
  storage_path     TEXT     NOT NULL
  status           TEXT     NOT NULL  DEFAULT 'PENDING'  ← PENDING | AVAILABLE | ERROR
  created_at       TIMESTAMPTZ
  updated_at       TIMESTAMPTZ
  UNIQUE (version_id, name)

registry_artifact_tag
  id           BIGINT   PK
  artifact_id  BIGINT   FK → registry_artifact.id  ON DELETE CASCADE
  tag          VARCHAR(255)  NOT NULL
  version_id   BIGINT   FK → registry_artifact_version.id  ON DELETE CASCADE
  created_at   TIMESTAMPTZ
  updated_at   TIMESTAMPTZ
  UNIQUE (artifact_id, tag)                          ← tag is unique per artifact

registry_artifact_type
  id           BIGINT   PK
  name         VARCHAR(255)  UNIQUE NOT NULL          ← shared taxonomy, e.g. "model", "pipeline"
  description  TEXT
  created_at   TIMESTAMPTZ
  updated_at   TIMESTAMPTZ
       │
       │  M : N (via registry_artifact_type_map)
       ▼
registry_artifact_type_map
  id           BIGINT   PK
  artifact_id  BIGINT   FK → registry_artifact.id       ON DELETE CASCADE
  type_id      BIGINT   FK → registry_artifact_type.id  ON DELETE CASCADE
  created_at   TIMESTAMPTZ
  UNIQUE (artifact_id, type_id)
```

Key design decisions:

- **Sequence increment 50** — avoids collisions if JPA-based tools ever access the same DB.
- **`status` on files** — two-phase write (PENDING → AVAILABLE/ERROR) ensures the DB row exists before the storage call; partial failures are observable.
- **Storage path includes file ID** — `{artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}` guarantees storage-key uniqueness even if a file with the same name is re-uploaded after deletion.
- **Tag upsert** — `ON CONFLICT (artifact_id, tag) DO UPDATE SET version_id = EXCLUDED.version_id` atomically moves a tag with no application-side conflict check.
- **Cascade deletes** — deleting an artifact removes all versions, files, and tags automatically at the DB level; the version DELETE handler also performs best-effort storage cleanup before removing the version row.
- **Types are M:N via a join table** — `registry_artifact_type_map` lets one artifact carry multiple types and one type apply to many artifacts, unlike the artifact ↔ tag relationship which is per-artifact-unique.

---

## API Surface

### Base path: `/api/v1`

#### Artifacts

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/artifacts` | 200 | List (paginated); filter `?name=` prefix, `?tag=`, or `?type=` (repeatable) |
| `POST` | `/artifacts` | 201, 400, 409 | Create |
| `GET` | `/artifacts/{id}` | 200, 404 | Get by ID |
| `PUT` | `/artifacts/{id}` | 200, 400, 404 | Update description |
| `DELETE` | `/artifacts/{id}` | 204, 404 | Delete (cascades to versions, files, tags) |

#### Versions

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/artifacts/{id}/versions` | 200, 404 | List (newest first) |
| `POST` | `/artifacts/{id}/versions` | 201, 400, 404, 409 | Create; body: `version`, `config`, `tags[]` |
| `GET` | `/artifacts/{id}/versions/{semver}` | 200, 400, 404 | Get |
| `DELETE` | `/artifacts/{id}/versions/{semver}` | 204, 400, 404 | Delete (best-effort storage cleanup) |

#### Tags

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `PUT` | `/artifacts/{id}/tags/{tag}` | 200, 400, 404 | Assign tag (body: `{"version":"1.2.3"}`); moves if exists |
| `DELETE` | `/artifacts/{id}/tags/{tag}` | 204, 404 | Delete tag |

#### Files

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/artifacts/{id}/versions/{semver}/files` | 200, 400, 404 | List files |
| `POST` | `/artifacts/{id}/versions/{semver}/files` | 201, 400, 404, 409 | Upload (multipart `file` field) |
| `GET` | `/artifacts/{id}/versions/{semver}/files/{fileId}` | 200, 404 | File metadata |
| `GET` | `/artifacts/{id}/versions/{semver}/files/{fileId}/download` | 200, 404 | Download stream |
| `DELETE` | `/artifacts/{id}/versions/{semver}/files/{fileId}` | 204, 404 | Delete file + storage object |

#### Types

Shared artifact taxonomy (e.g. `model`, `pipeline`, `library`), independent of the per-artifact `tag` pointers. `registry_artifact_type_map` is M:N — one artifact can carry multiple types.

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/types` | 200 | List all types |
| `POST` | `/types` | 201, 400, 409 | Create |
| `GET` | `/types/{typeId}` | 200, 404 | Get |
| `PUT` | `/types/{typeId}` | 200, 400, 404, 409 | Update name/description |
| `DELETE` | `/types/{typeId}` | 204, 404 | Delete (cascades assignments) |
| `GET` | `/artifacts/{id}/types` | 200, 404 | List types assigned to an artifact |
| `PUT` | `/artifacts/{id}/types/{typeId}` | 200, 404 | Assign a type to an artifact |
| `DELETE` | `/artifacts/{id}/types/{typeId}` | 204, 404 | Unassign |

#### Admin

Maintenance endpoints for inspecting/cleaning up abandoned registry data (`internal/api/handlers/admin.go`). Every endpoint requires `?olderThan=` (RFC3339); delete endpoints skip items tagged with `ADMIN_PROTECTED_TAG` (default `protect`) and return `{"deleted": N, "skipped": M}`.

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/admin/artifacts/empty` | 200, 400 | List artifacts with no versions (paginated) |
| `DELETE` | `/admin/artifacts/empty` | 200, 400 | Bulk delete empty artifacts |
| `GET` | `/admin/versions/empty` | 200, 400 | List versions with no files (paginated) |
| `DELETE` | `/admin/versions/empty` | 200, 400 | Bulk delete file-less versions |
| `GET` | `/admin/artifacts/no-files` | 200, 400 | List artifacts whose versions have no files (paginated) |
| `DELETE` | `/admin/artifacts/no-files` | 200, 400 | Bulk delete such artifacts and their versions (cascade) |

#### Pagination

List artifacts accepts `?page=0&pageSize=20`. Default: `page=0`, `pageSize=20`. Response envelope:

```json
{
  "items": [...],
  "total": 142,
  "page": 0,
  "pageSize": 20
}
```

#### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/q/health/live` | Always 200 if process is up |
| `GET` | `/q/health/ready` | 200 if DB ping succeeds, 503 otherwise |

---

## Storage Abstraction

```go
type Storage interface {
    Store(path string, data io.Reader, size int64, contentType string) error
    Retrieve(path string) (io.ReadCloser, error)
    Delete(path string) error
}
```

Storage paths follow the scheme `{artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}`. The file's DB ID is included to ensure uniqueness even across re-uploads of the same filename.

### FilesystemBackend

- Root at `STORAGE_FS_ROOT` (default `~/.fusion-index/artifacts`).
- `Retrieve` returns an `*os.File` which Gin streams to the client.

### S3Backend

- Uses `aws-sdk-go-v2`. Endpoint override enables MinIO and Ceph.
- Authentication via standard AWS credential chain: env vars → IRSA → instance profile.
- `Retrieve` returns the S3 `GetObject` body (`io.ReadCloser`).

---

## Database Layer (sqlc)

All DB access goes through generated code in `internal/db/sqlc/`. The generation source is:

```
sqlc.yaml                        ← config
internal/db/queries/*.sql        ← hand-written SQL
migrations/*.up.sql              ← schema (sqlc reads for type inference)
```

Regenerate after any SQL change:

```bash
~/go/bin/sqlc generate
```

| Setting | Value | Effect |
|---------|-------|--------|
| `sql_package` | `pgx/v5` | Uses pgx directly |
| `emit_pointers_for_null_types` | `true` | Nullable columns → `*string`, `*int64` |
| Timestamp type | `pgtype.Timestamptz` | Access `.Time` in response mappers |

---

## Transaction Strategy

Transactions are opened only where atomicity is required:

| Operation | Why |
|-----------|-----|
| `CreateArtifact` | Duplicate-name check + insert must be atomic |
| `CreateVersion` | Version insert + N tag upserts must be atomic |

All other operations are single-query and run without explicit transactions. Read-only handlers (List, Get) use `h.queries` directly — no transaction needed.

---

## Error Handling

Every error response has the shape:

```json
{"error": "human-readable message"}
```

Handler helpers in `internal/api/handlers/helpers.go`:

| Helper | Behaviour |
|--------|-----------|
| `notFoundOrInternal(c, err, msg)` | 404 if `pgx.ErrNoRows`, else 500 |
| `internalError(c, err)` | Always 500 |
| `conflictError(c, msg)` | Always 409 |
| `pathID(c)` | Parses `{id}` as int64; 400 on failure |
| `pathFileID(c)` | Parses `{fileId}` as int64; 400 on failure |
| `pathSemver(c)` | Parses `{semver}` via `semver.Parse`; 400 on failure |
| `parsePagination(c)` | Returns page + pageSize with defaults (0, 20) and floor clamping |
| `isUniqueViolation(err)` | `pgconn.PgError.Code == "23505"` |
| `isNotFound(err)` | `errors.Is(err, pgx.ErrNoRows)` |

---

## Configuration and Startup

`cmd/server/main.go` startup sequence:

```
1. config.Load()            — read all env vars
2. pgxpool.New(ctx, DBURL)  — open connection pool
3. pool.Ping(ctx)           — verify connectivity (fail-fast)
4. runMigrations(DBURL)     — golang-migrate, file://migrations/
5. storage.New(cfg)         — build FilesystemBackend or S3Backend
6. api.NewRouter(...)       — register all Gin routes
7. router.Run(":PORT")      — start serving
```

Migrations run on every startup. golang-migrate uses an advisory lock and a `schema_migrations` table to ensure idempotence.

---

## Health Probes

| Probe | Path | Logic |
|-------|------|-------|
| Liveness | `GET /q/health/live` | Always `{"status":"UP"}` — if the process responds, it is alive |
| Readiness | `GET /q/health/ready` | Pings the PostgreSQL pool; returns `{"status":"DOWN"}` + 503 if unreachable |

Kubernetes Deployment uses `initialDelaySeconds: 15` for readiness and `initialDelaySeconds: 30` for liveness to allow migration time on cold start.

---

## OpenAPI Spec

The OpenAPI 3.1 spec is hand-written in `internal/api/openapi/openapi.yaml` and embedded into the binary at compile time via `//go:embed`. It is served at runtime as JSON:

| Path | Description |
|------|-------------|
| `GET /api/openapi.json` | OpenAPI 3.1 spec as JSON |
| `GET /swagger/` | Swagger UI (HTML embedded in binary; assets from CDN) |

The YAML→JSON conversion happens once in `init()`. A `normaliseYAML()` helper converts any `map[any]any` values produced by `gopkg.in/yaml.v3` before passing to `encoding/json`.
