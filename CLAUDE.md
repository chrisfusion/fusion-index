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
cmd/server/main.go              # entrypoint — subcommand dispatch, config, pool, migrate, serve
cmd/server/migrate.go           # `fusion-index migrate-s3-prefix` — pre-upgrade hook Job entrypoint
cmd/server/backup.go            # `fusion-index backup-db` — daily CronJob entrypoint
cmd/server/restore.go           # `fusion-index restore-db` — manual DR entrypoint, has its own safety guard
cmd/server/pgexec.go            # pgConnArgs/pgEnv/requireS3Backend/mustS3Client — shared by backup.go/restore.go/migrate.go
internal/
├── config/config.go            # env var loading
├── semver/semver.go            # Parse("1.2.3") → Semver{Major,Minor,Patch}; used across handlers
├── k8sclient/                  # minimal in-cluster K8s REST client (no client-go)
│   ├── incluster.go            # HTTP client (in-cluster CA) + own SA token read
│   └── configmap.go            # get/create/update ConfigMap — used by migrate-s3-prefix marker
├── db/
│   ├── queries/                # hand-written SQL (sqlc input)
│   │   ├── registry_artifacts.sql
│   │   ├── registry_versions.sql
│   │   ├── registry_files.sql   # includes ListAvailableS3FilePaths — drives S3 prefix migration
│   │   ├── registry_tags.sql
│   │   └── registry_metrics.sql  # aggregate queries for /q/metrics
│   └── sqlc/                   # generated Go — DO NOT EDIT
├── metrics/
│   └── cache.go                # Snapshot struct + TTL cache with singleflight for /q/metrics
├── storage/
│   ├── storage.go              # Storage interface (Store/Retrieve/Delete)
│   ├── filesystem.go
│   ├── s3.go                   # also NewS3Client() — shared by the server and migrate-s3-prefix/backup-db/restore-db
│   ├── migrate.go              # MigratePrefix() — DB-path-driven S3 CopyObject migration
│   └── backup.go               # UploadStream/DownloadStream/FindLatestBackupKey — used by backup-db/restore-db
└── api/
    ├── router.go               # gin setup, routes, CORS
    ├── openapi/
    │   ├── handler.go          # //go:embed openapi.yaml; serves spec + Swagger UI
    │   └── openapi.yaml        # hand-written OpenAPI 3.1 spec (all 18 ops)
    ├── middleware/
    │   └── auth.go             # Gin middleware: K8s SA token validation via TokenReview API, uses internal/k8sclient
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
- **`postgresql16-client` in the runtime image:** the Dockerfile's final stage installs it alongside `ca-certificates` so `pg_dump`/`psql` are available to `backup-db`/`restore-db` (`cmd/server/backup.go`/`restore.go`) — matches the Postgres 16 used elsewhere (`postgres:16-alpine` in `createDatabaseJob`).
- **`github.com/aws/aws-sdk-go-v2/feature/s3/manager` for streaming uploads:** used only by `internal/storage/backup.go`'s `UploadStream`, where the object size isn't known upfront (unlike artifact file uploads, which have `Content-Length` from the HTTP request and use plain `PutObject` via `S3Storage.Store`). This module is marked deprecated upstream in favor of `feature/s3/transfermanager`, but that replacement is still pre-1.0 (unstable API) — `feature/s3/manager` is still the actively-maintained, production-stable choice as of this writing. Revisit once `transfermanager` reaches v1.0.
- **Helm config-change restarts:** `backend-deployment.yaml`'s pod template carries `checksum/config`/`checksum/secrets` annotations (sha256 of the rendered `backend-configmap.yaml`/`secrets.yaml`). Without this, `envFrom`/`secretKeyRef` env vars are read once at container start and a `helm upgrade` that only changes ConfigMap/Secret content (e.g. `S3_PREFIX`, `LOG_LEVEL`) does **not** roll the Deployment — Kubernetes only restarts pods when the pod template itself changes. Verified against a real cluster: without the checksum annotations, a running backend pod kept using a stale `S3_PREFIX` indefinitely after `s3.prefix` changed, silently writing new uploads under the old prefix even after the migration Job (below) had already run.
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
- **Storage paths:** `{artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}` — including the DB file ID prevents collisions when the same filename is uploaded twice. This relative path is what's persisted as `storage_path` in the DB; it never includes the backend root/prefix (`STORAGE_FS_ROOT` or `S3_PREFIX`), which is applied only at the storage-backend layer at call time. This keeps DB rows portable if the root/prefix ever changes.
- **S3 multi-instance prefix:** `S3_PREFIX` (Helm: `s3.prefix`) namespaces every object key under a bucket so multiple fusion-index instances can share one bucket without colliding. Applied inside `S3Storage` (`internal/storage/s3.go`) via `path.Join(prefix, relativePath)` — never stored in the DB (see Storage paths above). Left empty, the Helm chart computes `<kubernetes-namespace>/index/data` (`fusion-index.s3Prefix` in `_helpers.tpl`) rather than a fixed literal, so every instance gets a collision-free default with zero configuration. The Go binary's own bare env var default (when run outside Helm, e.g. local dev) is the literal `index` — it has no way to know a "namespace" outside a cluster.
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
| `S3_PREFIX` | `index` (bare binary) / `<namespace>/index/data` (Helm, computed) | Key prefix (root folder) under the bucket; namespaces objects so multiple instances can share one bucket. See "S3 multi-instance prefix" below |
| `S3_BACKUP_PREFIX` | `backups` (bare binary) / `<namespace>/index/backups` (Helm, computed) | Key prefix for daily DB metadata backups (`backup-db`/`restore-db`). Independent of `S3_PREFIX` — see "Helm — PostgreSQL Backup" below |
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

## Helm — PostgreSQL Backup
`postgresql.backupCronJob` in `values.yaml` (default `enabled: true`, daily at 02:00) runs `fusion-index backup-db` (`cmd/server/backup.go`) as an ordinary `CronJob` — same image as the backend, only rendered when `backend.storageBackend == "S3"`. This backs up **metadata only** (artifacts/versions/tags/config — the `registry_*` tables) — never the artifact files themselves, which are assumed durable in S3 already (the premise of this whole feature: S3 is the single point of truth, a day of DB-metadata loss is acceptable).
- **Why a CronJob and not a hook Job:** a `CronJob`'s scheduled runs always use whatever `jobTemplate` is currently applied by the most recent `helm upgrade` — unlike `s3-prefix-migration-job.yaml`, there's no hook-ordering problem, so it references `db-secret`/`s3-secret`/`backend-config` (ordinary resources) directly, no duplicated hook-scoped secret needed.
- **Format:** `pg_dump --format=plain --clean --if-exists --no-owner --no-privileges`, gzip'd, streamed straight into S3 via a multipart upload (`github.com/aws/aws-sdk-go-v2/feature/s3/manager`) — no local temp file, so backup size isn't bounded by the Job's ephemeral disk, and a failed upload never leaves a partial object visible (S3 only exposes a multipart upload's data on successful completion). Named `backup-<UTC timestamp>.sql.gz` — not a literal `.tar.gz`, since it's just a gzipped SQL script, not a tar archive.
- **`pgEnv()` (`cmd/server/pgexec.go`) sets both `PGPASSWORD` and `PGSSLMODE`** — the latter mirrors `DB_SSLMODE` so `pg_dump`/`psql` enforce the same TLS posture as the app's own `pgx` connections. Easy to miss: `DB_SSLMODE` alone means nothing to libpq subprocesses (only `PGSSLMODE` does), so without this they'd silently connect with libpq's default `prefer` — no server certificate verification — even when `DB_SSLMODE=verify-full` is configured. Local/dev testing with `sslmode=disable` won't surface this gap.
- **`--clean --if-exists` is what makes ordering safe:** the dump includes `DROP TABLE IF EXISTS` before every `CREATE TABLE`, so `restore-db` (below) works identically whether the target database is completely fresh (no schema) or already has fusion-index's migrated-but-empty schema (e.g. because a normal `helm install`/backend startup already ran `golang-migrate` against it before anyone got around to restoring) — no need to carefully sequence "restore before first backend start" in a DR runbook.
- **No retention/pruning** — by design, given "S3 is always safe": if you want old backups expired, configure an S3 lifecycle rule outside this chart. Keeps the Job simple and avoids a bug in pruning logic ever being the thing that deletes a backup you needed.
- **`S3_BACKUP_PREFIX` is independent of `S3_PREFIX`, not nested under it** (`fusion-index.s3BackupPrefix` in `_helpers.tpl`, defaults to `<namespace>/index/backups`, a sibling of `<namespace>/index/data`) — deriving it from `S3_PREFIX` by string manipulation would break if `s3.prefix` is overridden to something that doesn't end in `/data`.
- **Restore (`fusion-index restore-db`, `cmd/server/restore.go`) is a manual, on-demand operation — deliberately not wired to any automatic Helm trigger.** It finds the latest backup under `S3_BACKUP_PREFIX` (or a specific one via `RESTORE_BACKUP_KEY`), downloads it, and streams it through `gunzip` into `psql -v ON_ERROR_STOP=1 -f -`.
  - **Safety guard:** refuses to run if the target database already has rows in `registry_artifact` (`CountRegistryArtifacts`), unless `RESTORE_FORCE=true` is set. A target with no schema at all (fresh DR instance, migrations never run) is treated as safe-to-restore, not an error — `targetHasData` specifically checks for Postgres SQLSTATE `42P01` (undefined_table) to tell the two cases apart.
  - **Cross-namespace/cross-cluster DR:** if restoring into a different namespace than the one that produced the backup, `s3.backupPrefix`/`S3_BACKUP_PREFIX` must be set explicitly to the *source's* prefix — the computed default is namespace-scoped, so a fresh target namespace would otherwise compute a different (empty) prefix and find no backups.
  - **Manual invocation** (no chart-templated Job, by design — see below): `kubectl run restore-db --rm -i --image=<same image> --env=DB_HOST=... --env=S3_BACKUP_PREFIX=... --env=RESTORE_FORCE=true --command -- /app/fusion-index restore-db`, reusing the same DB/S3 env vars the backend Deployment uses (pull them from `db-secret`/`s3-secret`/`backend-config` in the source or target release).
  - **Why no Helm-templated restore Job at all, not even a disabled-by-default one:** a destructive DR operation should never be one `helm upgrade --set` away from accidentally auto-triggering — keeping it entirely out of the chart's hook/resource graph is the safest option.
- Verified end-to-end against a real cluster (minikube + MinIO): backup → artifact created → CronJob triggered manually → object lands at the expected key → restored onto a separate, fresh Postgres instance → data round-trips correctly; safety guard confirmed to block a populated target and to allow it through with `RESTORE_FORCE=true`.

## Helm — S3 Prefix Migration
`s3.migrationJob` in `values.yaml` (default `enabled: true`) runs `fusion-index migrate-s3-prefix` (`cmd/server/migrate.go`) as a `pre-install,pre-upgrade` hook Job — same image as the backend, only rendered when `backend.storageBackend == "S3"`. It compares the resolved `s3.prefix` (see `fusion-index.s3Prefix` — `<namespace>/index/data` when `s3.prefix` is left empty) against the last-applied prefix recorded in a marker ConfigMap (`<release>-s3-migration-state`); if unchanged it's a fast no-op (no DB or S3 calls), if it differs it copies every `AVAILABLE` S3 file from the old prefix to the new one.
- **Both `backend-configmap.yaml` and `s3-prefix-migration-job.yaml` must render `S3_PREFIX` via the same `fusion-index.s3Prefix` helper** — if they ever diverge, the migration Job would compare against a prefix the backend isn't actually using.
- **DB-driven, not bucket-listing:** the set of keys to copy comes from `ListAvailableS3FilePaths` (`internal/db/queries/registry_files.sql`), not `ListObjectsV2`. This is deliberate — listing under an empty old prefix (bucket root) would sweep up any unrelated objects in a shared bucket, not just fusion-index's own files.
- **Copy, not move:** old objects are left in place after a successful migration (see `internal/storage/migrate.go`). No automatic cleanup — that's a manual/future step.
- **Resumable:** each key is skipped via `HeadObject` if it already exists at the new prefix, so a failed/interrupted migration (Job failure leaves the marker unchanged) can simply be retried by re-running `helm upgrade`.
- **RBAC:** a dedicated `ServiceAccount`/`Role`/`RoleBinding` (`s3-migration-rbac.yaml`) scoped to get/create/update on just the marker ConfigMap — intentionally not the backend's own ServiceAccount, so the running backend Deployment doesn't inherit ConfigMap write access it never needs. Note K8s RBAC can't `resourceNames`-scope the `create` verb (the object doesn't exist yet at authorization time), so `create` is granted chart-wide on `configmaps` while `get`/`update` are scoped to the one marker name.
- **IAM:** the S3 credentials/IAM policy need `s3:GetObject` + `s3:PutObject` across the whole bucket (both old and new prefix), since `CopyObject` requires both — not just the currently-active prefix.
- **K8s API access:** `internal/k8sclient` — a from-scratch in-cluster REST client (own SA token + in-cluster CA, no client-go), shared with `auth.go`'s TokenReview calls. Also reads its own namespace via `k8sclient.ReadNamespace()` from the projected SA volume (`.../serviceaccount/namespace`, mounted automatically alongside `token`/`ca.crt` whenever `automountServiceAccountToken` isn't disabled) — no `POD_NAMESPACE` downward-API env var needed.
- **Hook-ordering gotcha (verified against a real cluster):** the Job's own `ServiceAccount`/`Role`/`RoleBinding` (`s3-migration-rbac.yaml`) and its DB/S3 credentials (`s3-migration-secret.yaml`) all carry `helm.sh/hook: pre-install,pre-upgrade` at an earlier weight (`-4`) than the Job itself (`-3`) — same class of issue as `postgresql-admin-secret.yaml`. Non-secret env vars (`DB_HOST`, `S3_BUCKET`, `S3_PREFIX`, etc.) are inlined as literal `value:` fields in the Job spec rather than `envFrom: configMapRef: backend-config`, because that ConfigMap is an ordinary resource and doesn't exist yet at hook time either.
- **Why a separate `s3-migration-secret.yaml` instead of hook-ifying `db-secret`/`s3-secret`:** verified empirically that Helm hook resources annotated only with `before-hook-creation` are **not** deleted by `helm uninstall` (only replaced right before the *next* release's hook phase) — turning the app's own runtime credential Secrets into hooks would leak them after every uninstall. The migration Job instead gets its own narrowly-scoped secret duplicating just the DB password / S3 static credentials (same accepted trade-off `postgresql-admin-secret.yaml` already makes), and only when there's no `existingSecret` to reference directly (existingSecret values are pre-existing and unaffected by this chart's hook ordering).

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
