# fusion-index — Artifact Registry

Stores, indexes, and exposes versioned artifacts via REST API. Artifacts use Python-style namespaced names (`org.team.name`), semver versions (`major.minor.patch`), per-version configurations, file uploads, and mutable tags.

## Tech Stack
- **Language:** Go 1.25
- **HTTP framework:** Gin
- **Database:** PostgreSQL (pgx/v5 driver, sqlc-generated queries)
- **Migrations:** golang-migrate (SQL files in `migrations/`)
- **Storage backends:** Filesystem (default) or S3 (aws-sdk-go-v2)

## Structure
```
cmd/server/main.go              # entrypoint — config, pool, migrate, serve
internal/
├── config/config.go            # env var loading
├── semver/semver.go            # Parse("1.2.3") → Semver{Major,Minor,Patch}; used across handlers
├── db/
│   ├── queries/                # hand-written SQL (sqlc input)
│   │   ├── registry_artifacts.sql
│   │   ├── registry_versions.sql
│   │   ├── registry_files.sql
│   │   ├── registry_tags.sql
│   │   └── registry_metrics.sql  # aggregate queries for /q/metrics
│   └── sqlc/                   # generated Go — DO NOT EDIT
├── metrics/
│   └── cache.go                # Snapshot struct + TTL cache with singleflight for /q/metrics
├── storage/
│   ├── storage.go              # Storage interface (Store/Retrieve/Delete)
│   ├── filesystem.go
│   └── s3.go
└── api/
    ├── router.go               # gin setup, routes, CORS
    ├── openapi/
    │   ├── handler.go          # //go:embed openapi.yaml; serves spec + Swagger UI
    │   └── openapi.yaml        # hand-written OpenAPI 3.1 spec (all 18 ops)
    ├── middleware/
    │   └── auth.go             # Gin middleware: K8s SA token validation via TokenReview API
    ├── handlers/
    │   ├── artifacts.go        # CRUD on registry_artifact
    │   ├── versions.go         # CRUD on registry_artifact_version + tag-on-create
    │   ├── files.go            # multipart upload / download / delete
    │   ├── tags.go             # PUT/DELETE tags (upsert moves tag)
    │   ├── metrics.go          # GET /q/metrics — loads Snapshot via metrics.Cache
    │   └── helpers.go          # pathID, pathFileID, pathSemver, isUniqueViolation, isNotFound
    └── dto/
        ├── requests.go         # binding-tagged request structs
        └── responses.go        # response structs + mapper functions
migrations/                     # golang-migrate up-only SQL files
tests/integration/              # real-Postgres tests via testcontainers-go
sqlc.yaml                       # sqlc config
```

## DB Schema
| Table | Key columns |
|---|---|
| `registry_artifact` | `id`, `full_name VARCHAR(500) UNIQUE`, `description TEXT` |
| `registry_artifact_version` | `id`, `artifact_id FK`, `major/minor/patch INT`, `config TEXT`; UNIQUE `(artifact_id, major, minor, patch)` |
| `registry_artifact_file` | `id`, `version_id FK`, `name`, `content_type`, `size_bytes`, `storage_backend`, `storage_path`, `status`; UNIQUE `(version_id, name)` |
| `registry_artifact_tag` | `id`, `artifact_id FK`, `tag VARCHAR(255)`, `version_id FK`; UNIQUE `(artifact_id, tag)` |
| `registry_artifact_type` | `id`, `name VARCHAR(255) UNIQUE`, `description TEXT` |
| `registry_artifact_type_map` | `id`, `artifact_id FK`, `type_id FK`; UNIQUE `(artifact_id, type_id)` |

## Key Conventions
- **OpenAPI spec:** hand-written `internal/api/openapi/openapi.yaml` (OpenAPI 3.1); embedded via `//go:embed` and served as JSON. swaggo/swag does NOT support 3.1 — don't use it.
- **`go:embed` rule:** `//go:embed` cannot use `../` paths — the file must be in the same directory or a subdirectory of the Go source file.
- **yaml.v3 → JSON:** `yaml.v3` may return `map[any]any` for non-string-keyed maps; always call `normaliseYAML()` (in `internal/api/openapi/handler.go`) before `json.Marshal` to avoid a panic.
- All DB access goes through `internal/db/sqlc` (generated). Never write raw pgx queries outside that layer.
- sqlc generates `pgtype.Timestamptz`. Always access `.Time` in response mappers — the sqlc.yaml `timestamptz → time.Time` override does NOT take effect with pgx/v5.
- **sqlc aggregate types:** `COALESCE(SUM(nullable_col), 0)` without an explicit cast generates `interface{}`. Always use `::bigint` + named alias: `COALESCE(SUM(size_bytes), 0)::bigint AS total_bytes`.
- **singleflight + context:** Inside `singleflight.Group.Do`, always use `context.Background()` for DB calls — using the caller's request context means a client disconnect aborts all concurrent waiters.
- **Read-only snapshot transactions:** Wrap multi-query aggregate reads in `pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})` + `q.WithTx(tx)` to prevent inconsistent snapshots.
- golang-migrate uses `lib/pq` internally (not pgx). Append `?sslmode=disable` to DBURL or migrations fail.
- Regenerate after SQL changes: `~/go/bin/sqlc generate`
- Transactions via `q.WithTx(tx)` for atomic operations (version create + tag upsert, artifact create + name check, etc.).
- Nullable columns from sqlc become `*string` / `*int64` (`emit_pointers_for_null_types: true`).
- Error responses always have shape `{"error": "..."}`.
- **409 on unique violation:** `errors.As(err, &pgErr) && pgErr.Code == "23505"` using `*pgconn.PgError` — used for duplicate version, duplicate artifact name, duplicate filename.
- **Storage paths:** `{artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}` — including the DB file ID prevents collisions when the same filename is uploaded twice.
- **Tag upsert:** `ON CONFLICT (artifact_id, tag) DO UPDATE SET version_id = EXCLUDED.version_id` — atomically moves a tag to a new version; no application-level conflict check needed.
- **Two-phase file upload:** create DB row (status=PENDING) → `storage.Store` → `UpdateArtifactFileStored` (status=AVAILABLE). On storage error: mark ERROR. On DB update error: mark ERROR + `storage.Delete` to avoid orphans.

## REST API (validated)

| Method | Path | Description |
|---|---|---|
| GET/POST | `/api/v1/artifacts` | List (filter `?name=` prefix, `?tag=`) / create |
| GET/PUT/DELETE | `/api/v1/artifacts/{id}` | Get / update description / delete artifact |
| GET/POST | `/api/v1/artifacts/{id}/versions` | List / create version (body: `version`, `config`, `tags[]`) |
| GET/DELETE | `/api/v1/artifacts/{id}/versions/{semver}` | Get / delete version |
| PUT/DELETE | `/api/v1/artifacts/{id}/tags/{tag}` | Assign (`{"version":"1.2.3"}`) / delete tag |
| GET/POST | `/api/v1/artifacts/{id}/versions/{semver}/files` | List / upload file (multipart) |
| GET | `/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}` | File metadata |
| GET | `/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}/download` | Download stream |
| DELETE | `/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}` | Delete file |
| GET | `/q/health/live`, `/q/health/ready` | Kubernetes health probes |
| GET | `/q/metrics` | Registry aggregate metrics (TTL-cached, always public) |
| GET | `/api/openapi.json` | OpenAPI 3.1 spec as JSON |
| GET | `/swagger/` | Swagger UI (assets from CDN, HTML embedded in binary) |

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | Listen port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `fusion_index` | Database name |
| `DB_USERNAME` | `fusion` | DB user |
| `DB_PASSWORD` | `fusion` | DB password |
| `DB_SSLMODE` | `disable` | `disable` / `require` / `verify-full` — must be set; golang-migrate uses lib/pq which defaults to require |
| `STORAGE_BACKEND` | `FILESYSTEM` | `FILESYSTEM` or `S3` |
| `STORAGE_FS_ROOT` | `~/.fusion-index/artifacts` | Root dir for filesystem storage |
| `S3_BUCKET` | `fusion-index-artifacts` | S3 bucket name |
| `AWS_REGION` | `us-east-1` | AWS region |
| `S3_ENDPOINT_OVERRIDE` | _(empty)_ | Custom S3 endpoint (MinIO etc.) |
| `AUTH_ENABLED` | `false` | `true` to enable K8s SA token validation |
| `AUTH_AUDIENCE` | _(empty)_ | If set, token audience is validated (recommended: `fusion-index`) |
| `AUTH_ALLOWED_SA` | _(empty)_ | Comma-separated `namespace/name` allowlist; empty = any valid SA |
| `METRICS_CACHE_TTL` | `60s` | How long `/q/metrics` results are cached; any `time.ParseDuration` value (e.g. `30s`, `5m`) |

## Local Testing

```bash
go test ./tests/integration/... -timeout 120s
```

Uses testcontainers-go to spin up a real PostgreSQL container. Docker must be running.

## Local Minikube Deployment

```bash
eval $(minikube docker-env)
docker build -t fusion-index:latest .
helm upgrade --install fusion-index deployment/ \
  --namespace fusion \
  -f deployment/values-dev.yaml \
  --wait --timeout 3m
```

- After rebuilding: `kubectl rollout restart deployment/fusion-index-backend -n fusion`
- Port-forward: `kubectl port-forward -n fusion service/fusion-index-backend 18080:8080 --address 127.0.0.1`
- Smoke-test: `curl -s http://127.0.0.1:18080/api/v1/artifacts | python3 -m json.tool`

**Testing structural chart changes (new resources, removed dependencies, hook changes):** never `helm upgrade` the live releases in `fusion`/`fusion-dev-a`/`fusion-dev-b` — they hold real data. Instead build the image into minikube's docker daemon and `helm install` into a disposable namespace with a throwaway `postgres:16-alpine` Deployment+Service as the target, then delete the namespace when done.

## Authentication
- **K8s SA token auth:** `internal/api/middleware/auth.go` — calls `POST /apis/authentication.k8s.io/v1/tokenreviews` directly via `net/http` (no client-go). Uses in-cluster CA (`/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`) and own SA token.
- **SA token re-read per request** — kubelet rotates projected tokens; always `os.ReadFile(saTokenPath)` fresh, never cache.
- **Username format:** K8s returns `system:serviceaccount:<namespace>:<name>`; allowlist entries use `namespace/name` (converted by `saFromUsername`).
- **Protected scope:** auth middleware applied to `/api/v1` group only — `/q/health/*`, `/q/metrics`, `/api/openapi.json`, `/swagger/` are always public.
- **Disabled locally:** `AUTH_ENABLED=false` (default) makes middleware a no-op; safe for local dev outside cluster.

## Helm — Authentication
`auth.enabled` / `auth.audience` / `auth.allowedServiceAccounts` in `values.yaml`.
When `auth.enabled: true` a `ClusterRole` + `ClusterRoleBinding` granting `tokenreviews/create` are created automatically. The backend `ServiceAccount` is always created regardless of auth setting.

## Helm — Configurable Pod/Container Metadata
All knobs live under `backend.*` in `values.yaml`:
- `deploymentLabels`, `deploymentAnnotations` — Deployment object metadata (GitOps, ArgoCD sync-wave, etc.)
- `podLabels`, `podAnnotations` — Pod template (Prometheus scraping, cost labels, etc.)
- `serviceAnnotations` — Service object (e.g. cloud LB type, Linkerd opaque-ports)
- `podSecurityContext` — pod-level (`runAsNonRoot`, `fsGroup`)
- `containerSecurityContext` — container-level (`allowPrivilegeEscalation`, `readOnlyRootFilesystem`, `capabilities`)

## Helm — Linkerd Integration
`linkerd.opaquePorts` in `values.yaml` — comma-separated list of ports to mark as opaque TCP in Linkerd (e.g. `"8080"`). When set, adds `config.linkerd.io/opaque-ports` to both the Service and Pod annotations so Linkerd bypasses its L7 HTTP/2 proxy and uses a raw mTLS TCP tunnel instead.

**Why this matters:** Linkerd's L7 proxy translates HTTP/1.1 → HTTP/2, which breaks large multipart uploads (`Content-Length` mismatch / flow-control stall). Setting opaque ports fixes silent upload failures for files >~2 MB without losing mTLS or Linkerd observability. Set to `""` (default) when Linkerd is not installed.

## Helm — Ingress Body Size
`ingress.proxyBodySize` in `values.yaml` — sets `nginx.ingress.kubernetes.io/proxy-body-size`. Defaults to `"100m"`. The Nginx default is `1m`, which silently rejects large artifact uploads with HTTP 413. Set to `"0"` for unlimited. Merged with `ingress.annotations`; explicit annotations take precedence for the same key.

## Helm — Filesystem Storage Persistence
- `storageBackend: FILESYSTEM` with no PVC = ephemeral — files vanish on pod restart.
- `backend.persistence.enabled: true` creates PVC `fusion-index-backend-artifacts` and wires `STORAGE_FS_ROOT` to the mount path (`/data/artifacts`). Enabled by default in `values-dev.yaml`.
- minikube uses the `standard` (hostPath) StorageClass — survives `rollout restart` and `minikube stop/start`, but NOT `minikube delete`.
- To verify persistence: upload a file → `kubectl rollout restart deployment/fusion-index-backend -n fusion` → download the file again.

## Helm — PostgreSQL
`postgresql.*` in `values.yaml` is external-only — fusion-index does not install/manage Postgres. App runtime creds: `host`/`port`/`database`/`username`/`password`/`existingSecret`. Separate `postgresql.admin.*` (superuser/CREATEDB) creds are used only by `postgresql.createDatabaseJob`, a `pre-install,pre-upgrade` hook Job that idempotently runs `CREATE DATABASE` before the backend Deployment starts.
- **Hook-ordering gotcha:** any Secret read by a hook Job must itself be annotated as a hook with an earlier `hook-weight` — ordinary (non-hook) resources apply *after* pre-install/pre-upgrade hooks. Otherwise the Job's pod sits in `CreateContainerConfigError` until `activeDeadlineSeconds` kills it (symptom: Job goes `InProgress` → `Failed`, no pod logs — pod's already deleted). See `postgresql-admin-secret.yaml` (weight `-6`) vs the create-db Job (weight `-5`).
- **psql gotcha:** `:'var'` substitution only works reading from a script/stdin, NOT via `-c "..."` (verified against postgres:16-alpine) — interpolate trusted, chart-controlled values directly into the SQL string instead.

## Logging

Platform-wide logging spec: `../logging_principles.md` (applies to this service).
Reference implementations: `../fusion-forge/internal/api/middleware/logging.go`, `../fusion-flux/internal/apiserver/middleware/logging.go`.

- `log/slog` only — no `import "log"` anywhere
- `LOG_LEVEL` / `LOG_FORMAT` env vars (wired through Helm ConfigMap)
- Per-request logger injected by `middleware.NewLoggingMiddleware()`; retrieve with `middleware.LoggerFromCtx(c)` in handlers
- `internalError` logs before writing the 500 response — don't also call `LoggerFromCtx` at the same call site (double-log)
- Add structured context fields (`name`, `artifact_id`, `version`, `filename`) at key mutation error paths; plain `internalError` is fine for simple lookups

## Changelog
Every feature addition and bugfix must be reflected in `CHANGELOG.md` before the work is considered done. Follow the existing format: add an entry under `## [Unreleased]` or create a new `## [x.y.z] — YYYY-MM-DD` section.

## Branch Strategy
`main` → `develop` → `feature/*`

## Commit Style
Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`
