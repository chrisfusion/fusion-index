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
           (metadata store)       (artifact store)
```

**fusion-index** is a stateless REST service. All persistent state lives in PostgreSQL (metadata) and in S3 or the local filesystem (artifact bytes). Multiple replicas can run concurrently against the same database.

---

## Layered Architecture

```
┌─────────────────────────────────────────────────┐
│               HTTP (Gin router)                 │  router.go
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

Each layer has a single responsibility:

| Layer | Responsibility |
|-------|---------------|
| Router | Route registration, CORS, middleware |
| DTOs | Bind + validate request bodies; shape response JSON |
| Handlers | Business logic, transaction management, error mapping |
| sqlc | Type-safe SQL execution; zero raw queries outside this layer |
| Storage | Binary blob persistence, abstracted behind an interface |

---

## Request Lifecycle

### Read request (e.g., `GET /api/v1/jobs/{id}`)

```
Client
  │
  ▼
Gin router  →  matches route, extracts path param
  │
  ▼
Handler: GetJob
  ├── pathID(c)            — parse + validate {id} from URL
  ├── q.GetJobByID(ctx, id) — sqlc query → pgxpool
  ├── if pgx.ErrNoRows    → 404 {"error": "not found"}
  └── ToJobResponse(row)  → 200 {"id": ..., "name": ..., ...}
```

### Write request with transaction (e.g., `POST /api/v1/jobs`)

```
Client
  │
  ▼
Gin router
  │
  ▼
Handler: CreateJob
  ├── c.ShouldBindJSON(&req)       — validate body
  ├── q.GetJobByName(ctx, name)    — duplicate check → 409 if exists
  ├── pool.Begin(ctx)              — open transaction
  │     ├── q.WithTx(tx).CreateJob(...)         — insert row
  │     ├── q.WithTx(tx).IncrementJobVersion()  — bump counter
  │     └── q.WithTx(tx).CreateJobVersion(...)  — insert version 1
  ├── tx.Commit(ctx)
  └── ToJobResponse(row) → 201
```

### Artifact upload (`POST /api/v1/jobs/{jobId}/versions/{n}/artifacts`)

```
Client  ──multipart/form-data──►  Handler: UploadArtifact
  │
  ├── parse path params (jobId, versionNumber)
  ├── validate job version exists
  ├── c.Request.FormFile("file")          — read multipart header
  ├── q.CreateArtifact(..., "PENDING")    — register in DB first
  ├── storage.Store(path, reader, size)  — stream bytes to backend
  ├── if error → q.UpdateArtifactStatus(id, "ERROR")
  └── else    → q.UpdateArtifactStored(id, storagePath) → "AVAILABLE"
```

---

## Data Model

```
job_templates
  id          BIGINT  PK (sequence 1, increment 50)
  name        TEXT    UNIQUE NOT NULL
  description TEXT
  version     INT     NOT NULL DEFAULT 1
  created_at  TIMESTAMPTZ
  updated_at  TIMESTAMPTZ
       │
       │  1 : N
       ▼
job_template_versions
  id                  BIGINT  PK
  job_template_id     BIGINT  FK → job_templates.id
  version_number      INT     NOT NULL
  description         TEXT
  schema_definition   TEXT
  created_at          TIMESTAMPTZ
       │
       │  1 : N  (a job pins to a specific template version)
       ▼
jobs
  id                       BIGINT  PK
  name                     TEXT    UNIQUE NOT NULL
  description              TEXT
  job_template_version_id  BIGINT  FK → job_template_versions.id
  version                  INT     NOT NULL DEFAULT 1
  config                   TEXT
  created_at               TIMESTAMPTZ
  updated_at               TIMESTAMPTZ
       │
       │  1 : N
       ▼
job_versions
  id               BIGINT  PK
  job_id           BIGINT  FK → jobs.id
  version_number   INT     NOT NULL
  description      TEXT
  created_at       TIMESTAMPTZ
       │
       │  1 : N
       ▼
artifacts
  id               BIGINT  PK
  job_version_id   BIGINT  FK → job_versions.id
  filename         TEXT    NOT NULL
  content_type     TEXT
  size_bytes       BIGINT
  storage_path     TEXT               — logical key for storage backend
  status           TEXT               — PENDING | AVAILABLE | ERROR
  created_at       TIMESTAMPTZ
```

Key design decisions:

- **Sequence increment 50** — matches Hibernate allocationSize default, avoids conflicts if the DB is accessed by JPA-based tools in the future.
- **`version` counter on templates and jobs** — monotonically incrementing application-managed counter, separate from the versioned-row PK.
- **`status` on artifacts** — two-phase write (PENDING → AVAILABLE/ERROR) ensures the row exists in the DB before the storage call, making partial failures observable and recoverable.
- **No hard deletes on versions** — template and job versions are append-only. Only top-level templates, jobs, and artifacts support DELETE.
- **Referential integrity** — deleting a template is blocked if any job references one of its versions (enforced at the handler layer via `CountJobsForTemplate` before issuing DELETE).

---

## API Surface

### Base path: `/api/v1`

#### Templates

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/templates` | 200 | List (paginated) |
| `POST` | `/templates` | 201, 400, 409 | Create |
| `GET` | `/templates/{id}` | 200, 404 | Get by ID |
| `PUT` | `/templates/{id}` | 200, 404 | Update name/description |
| `DELETE` | `/templates/{id}` | 204, 404, 409 | Delete (blocked if jobs exist) |
| `GET` | `/templates/{id}/versions` | 200, 404 | List versions |
| `POST` | `/templates/{id}/versions` | 201, 404 | Publish new version |
| `GET` | `/templates/{id}/versions/{n}` | 200, 404 | Get specific version |

#### Jobs

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/jobs` | 200 | List (paginated) |
| `POST` | `/jobs` | 201, 400, 409 | Create (requires valid templateVersionId) |
| `GET` | `/jobs/{id}` | 200, 404 | Get by ID |
| `PUT` | `/jobs/{id}` | 200, 404 | Update name/description/config |
| `DELETE` | `/jobs/{id}` | 204, 404 | Delete |
| `GET` | `/jobs/{id}/versions` | 200, 404 | List versions |
| `POST` | `/jobs/{id}/versions` | 201, 404 | Publish new version |
| `GET` | `/jobs/{id}/versions/{n}` | 200, 404 | Get specific version |

#### Artifacts

| Method | Path | Status codes | Description |
|--------|------|-------------|-------------|
| `GET` | `/jobs/{jobId}/versions/{n}/artifacts` | 200, 404 | List artifacts for a job version |
| `POST` | `/jobs/{jobId}/versions/{n}/artifacts` | 201, 400, 404 | Upload artifact (multipart/form-data) |
| `GET` | `/artifacts` | 200 | List all artifacts (paginated, `createdAt` DESC) |
| `GET` | `/artifacts/{id}` | 200, 404 | Get artifact metadata |
| `GET` | `/artifacts/{id}/download` | 200, 404, 502 | Download artifact (streamed) |
| `DELETE` | `/artifacts/{id}` | 204, 404 | Delete (storage + DB) |

#### Pagination

All list endpoints accept `?page=1&pageSize=20`. Default: `page=1`, `pageSize=20`. Response envelope:

```json
{
  "items": [...],
  "total": 142,
  "page": 1,
  "pageSize": 20
}
```

#### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/q/health/live` | Liveness — always 200 if process is up |
| `GET` | `/q/health/ready` | Readiness — 200 if DB ping succeeds |

---

## Storage Abstraction

```go
type Storage interface {
    Store(suggestedPath string, data io.Reader, sizeHint int64, contentType string) (storagePath string, err error)
    Retrieve(storagePath string) (io.ReadCloser, error)
    Delete(storagePath string) error
}
```

The `storagePath` returned by `Store` is an opaque string persisted in the `artifacts.storage_path` column. Handlers never construct storage paths themselves — they always receive the canonical path back from the backend.

### FilesystemBackend

- Files stored at `{STORAGE_FS_ROOT}/{uuid}`.
- `Retrieve` returns an `*os.File` which Gin streams to the client.
- `Delete` calls `os.Remove`.

### S3Backend

- Objects stored at `{uuid}` (bucket-root flat layout).
- Uses `aws-sdk-go-v2`. Endpoint override enables MinIO and Ceph.
- `ContentLength` is passed to S3 when `sizeHint > 0` (avoids chunked encoding).
- `Retrieve` returns the S3 `GetObject` body (an `io.ReadCloser`).

---

## Database Layer (sqlc)

All DB access goes through generated code in `internal/db/sqlc/`. The generation source is:

```
sqlc.yaml          ← config
internal/db/queries/*.sql  ← hand-written SQL
migrations/*.up.sql        ← schema (sqlc reads for type inference)
```

Regenerate after any SQL change:

```bash
~/go/bin/sqlc generate
```

Key sqlc settings:

| Setting | Value | Effect |
|---------|-------|--------|
| `sql_package` | `pgx/v5` | Uses pgx directly (no `database/sql` overhead) |
| `emit_pointers_for_null_types` | `true` | Nullable columns → `*string`, `*int64` |
| Timestamp type | `pgtype.Timestamptz` | Access `.Time` for `time.Time` in mappers |

---

## Transaction Strategy

Transactions are opened only where atomicity is required:

| Operation | Why |
|-----------|-----|
| `CreateTemplate` | Insert template + increment version counter + insert version 1 |
| `PublishTemplateVersion` | Increment counter + insert new version row |
| `CreateJob` | Insert job + increment version counter + insert version 1 |
| `PublishJobVersion` | Increment counter + insert new version row |

The `pgxpool.Pool` is passed to handlers; transactions are created with `pool.Begin(ctx)` and the query object is wrapped with `q.WithTx(tx)`. All other operations are single-query and run without explicit transactions.

---

## Error Handling

Every error response has the shape:

```json
{"error": "human-readable message"}
```

Handler helpers in `internal/api/handlers/helpers.go`:

| Helper | Behaviour |
|--------|-----------|
| `notFoundOrInternal(c, err)` | Returns 404 if `errors.Is(err, pgx.ErrNoRows)`, else 500 |
| `internalError(c, err)` | Always returns 500 with `{"error": "internal server error"}` |
| `pathID(c, param)` | Parses int64 path param; aborts with 400 on invalid input |
| `pathVersionNumber(c)` | Same, specifically for `{n}` version segments |
| `parsePagination(c)` | Returns page + pageSize with defaults and floor clamping |

409 Conflict is returned explicitly in handlers when a duplicate name is detected or when a referential delete is blocked.

---

## Configuration and Startup

`cmd/server/main.go` startup sequence:

```
1. config.Load()           — read all env vars
2. pgxpool.New(ctx, DBURL) — open connection pool
3. pool.Ping(ctx)          — verify connectivity (fail-fast)
4. runMigrations(DBURL)    — golang-migrate, file://migrations/
5. storage.New(cfg)        — build FilesystemBackend or S3Backend
6. api.NewRouter(pool, storage) — register all Gin routes
7. router.Run(":PORT")     — start serving
```

Migrations run on every startup. golang-migrate uses an advisory lock and a `schema_migrations` table to ensure idempotence.

---

## Health Probes

| Probe | Path | Logic |
|-------|------|-------|
| Liveness | `GET /q/health/live` | Always `{"status":"UP"}` — if the process responds, it is alive |
| Readiness | `GET /q/health/ready` | Pings the PostgreSQL pool; returns `{"status":"DOWN"}` + 503 if unreachable |

Kubernetes Deployment uses `initialDelaySeconds: 15` for readiness and `initialDelaySeconds: 30` for liveness to allow migration time on cold start.
