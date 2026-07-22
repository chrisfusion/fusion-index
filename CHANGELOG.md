# Changelog

All notable changes to fusion-index are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [0.5.0] — 2026-07-21

### Added
- `S3_PREFIX` env var (Helm: `s3.prefix`) — namespaces S3 object keys under a configurable root folder, letting multiple fusion-index instances share a single bucket without colliding. Left unset, the Helm chart defaults it to `<kubernetes-namespace>/index/data` (`fusion-index.s3Prefix` helper) so every instance gets a collision-free default automatically; set `s3.prefix` explicitly to override. The prefix is applied only at the storage layer and is never persisted in `storage_path`, so it can be changed independently of existing DB rows.
- `fusion-index migrate-s3-prefix` subcommand + Helm `s3.migrationJob` (default `enabled: true`) — a `pre-install,pre-upgrade` hook Job that automatically copies existing S3 objects to a new `s3.prefix` on upgrade. DB-driven (uses `ListAvailableS3FilePaths`, not a bucket listing), copy-only (old objects are left in place), and resumable (already-copied objects are skipped on retry). No-ops (no DB/S3 calls) when the prefix hasn't changed since the last release. See CLAUDE.md "Helm — S3 Prefix Migration" for details.
- `internal/k8sclient` — shared in-cluster Kubernetes REST client (own SA token + in-cluster CA, no client-go), extracted from `auth.go` and reused by the new migration Job to read/write its marker ConfigMap.
- **Daily DB metadata backups to S3**, on the premise that S3 is the durable single point of truth and artifact files (already in S3) never need backing up — only the `registry_*` metadata tables (artifacts, versions, tags, configs) do:
  - `fusion-index backup-db` subcommand + Helm `postgresql.backupCronJob` (default `enabled: true`, daily at 02:00) — streams a `pg_dump --clean --if-exists`, gzipped, directly into S3 via multipart upload. No local temp file, no retention/pruning (rely on an S3 lifecycle rule if you want expiry). `pg_dump`/`psql` honor `DB_SSLMODE` via `PGSSLMODE` (`cmd/server/pgexec.go`), matching the app's own `pgx` connections.
  - `fusion-index restore-db` subcommand — manual, on-demand disaster recovery (not wired to any automatic Helm trigger). Finds the latest backup (or a specific one via `RESTORE_BACKUP_KEY`) and restores it via `psql`. Refuses to run against a database that already has data unless `RESTORE_FORCE=true`.
  - New `S3_BACKUP_PREFIX` env var (Helm: `s3.backupPrefix`, default `<namespace>/index/backups`) — independent of `S3_PREFIX`, not nested under it.
  - Dockerfile now installs `postgresql16-client` in the runtime image for `pg_dump`/`psql`.
  - See CLAUDE.md "Helm — PostgreSQL Backup" for the full design and the restore runbook.

### Fixed
- Backend Deployment pod template now carries `checksum/config`/`checksum/secrets` annotations, forcing a rolling restart whenever `backend-configmap.yaml`/`secrets.yaml`'s rendered content changes on `helm upgrade`. Previously a running pod kept using stale env vars (e.g. `S3_PREFIX`, `LOG_LEVEL`) indefinitely after a config-only upgrade, since Kubernetes only restarts pods when the pod template itself changes — found while end-to-end testing the S3 prefix migration above (a stale pod kept writing new uploads under the old prefix after the migration Job had already copied everything to the new one).

## [0.4.0] — 2026-07-07

### Added
- Helm `backend.persistence` block — when `storageBackend=FILESYSTEM`, a PVC is created and mounted at `backend.persistence.mountPath` (default `/data/artifacts`), with `STORAGE_FS_ROOT` wired automatically. Enabled by default in `values-dev.yaml` (5 Gi, minikube hostPath provisioner) so artifact files survive pod restarts in local development. Production (`storageBackend=S3`) is unaffected.
- Helm `postgresql.createDatabaseJob` — a `pre-install,pre-upgrade` hook Job (idempotent `CREATE DATABASE`) that provisions `postgresql.database` on the pre-installed PostgreSQL instance before the backend Deployment starts. Enabled by default; connects using separate `postgresql.admin.*` credentials (superuser/CREATEDB privilege), independent of the app's own `postgresql.username`/`password`. Set `postgresql.createDatabaseJob.enabled: false` if the database is provisioned by other tooling.

### Removed
- Bundled Bitnami `postgresql` Helm subchart dependency — fusion-index now always connects to a pre-installed PostgreSQL instance. The `postgresql.*` values are flattened to `host`/`port`/`database`/`username`/`password`/`existingSecret` (the old `postgresql.enabled` toggle and `postgresql.external.*` block are gone). Existing values files that set `postgresql.enabled` or `postgresql.external.*` need to be updated — see `INSTALL.md`.

---

## [0.3.0] — 2026-05-26

### Added
- `GET /q/metrics` endpoint exposing registry aggregate metrics: total artifacts, versions, tags, files by status (AVAILABLE / PENDING / ERROR), total storage bytes, artifacts without tags, artifacts without versions, and per-type artifact counts
- In-memory TTL cache for metrics queries with singleflight deduplication — prevents thundering-herd DB load when the cache expires under concurrent requests
- `METRICS_CACHE_TTL` env var (default `60s`) to configure cache lifetime; Helm value `backend.metricsCacheTtl` can be added to expose it

---

## [0.2.0] — 2026-05-21

### Added
- Admin maintenance API at `/api/v1/admin/**` — 6 endpoints for inspecting and cleaning up abandoned registry data:
  - `GET /api/v1/admin/artifacts/empty` — list artifacts with no versions (paginated, `olderThan` required)
  - `DELETE /api/v1/admin/artifacts/empty` — bulk delete empty artifacts
  - `GET /api/v1/admin/versions/empty` — list versions with no files (paginated)
  - `DELETE /api/v1/admin/versions/empty` — bulk delete file-less versions
  - `GET /api/v1/admin/artifacts/no-files` — list artifacts whose versions have no files (paginated)
  - `DELETE /api/v1/admin/artifacts/no-files` — bulk delete such artifacts and their versions (cascade)
- All bulk deletes return `{"deleted": N, "skipped": M}` where `skipped` counts items protected by a configurable tag
- Protection tag configurable via `ADMIN_PROTECTED_TAG` env var (default `protect`); Helm value `admin.protectedTag`
- All 6 endpoints documented in the OpenAPI spec under the `Admin` tag

---

## [0.1.2] - 2026-05-20

### Fixed
- Fixed readonly filesystem which can not write to tmp. This prevents multipart uploads

 
### Added
- Structured l-gging via `log/slog` (Go standard library) following fusion-platform logging principles
  - `LOG_LEVEL` env var (`debug` | `info` | `warn` | `error`, default `info`)
  - `LOG_FORMAT` env var (`json` | `text`, default `json` — JSON for k8s log collectors, text for local dev)
  - Per-request logging middleware (`internal/api/middleware/logging.go`): generates a `request_id`, attaches `{method, path, client_ip}` to every log line, emits one access log entry (status + latency_ms) after each handler
  - `LoggerFromCtx(c)` helper for propagating the per-request logger into handlers
  - `internalError` now logs `slog.Error("internal error", "error", err)` before writing the 500 response
  - Key mutation handlers (`Create artifact`, `Create version`, `Upload file`) attach structured context fields (`name`, `artifact_id`, `version`, `filename`) to error log entries
  - Best-effort storage cleanup failures (`Delete file`, `Delete version`) emit `slog.Warn`
  - Startup sequence uses `slog.Info`/`slog.Error`; `log.Fatal`/`log.Printf` removed
  - Helm: `backend.logLevel` and `backend.logFormat` in `values.yaml`; `LOG_LEVEL` and `LOG_FORMAT` added to the backend ConfigMap

---

## [0.1.1] — 2026-05-18

### Added
- `deployment/values.yaml`: `linkerd.opaquePorts` — comma-separated port list; when set (e.g. `"8080"`), adds `config.linkerd.io/opaque-ports` to both the Service and Pod annotations so Linkerd uses a raw mTLS TCP tunnel instead of its L7 HTTP/2 proxy; required when Linkerd is installed and large multipart uploads (venv archives from forge-builder, GUI uploads via BFF) fail silently or with truncated-body errors
- `deployment/values.yaml`: `backend.serviceAnnotations` — free-form annotation map added to the Service object (merged with Linkerd opaque-ports annotation when both are set; explicit `serviceAnnotations` take precedence)
- `deployment/values.yaml`: `ingress.proxyBodySize` — sets `nginx.ingress.kubernetes.io/proxy-body-size` on the Ingress; defaults to `"100m"` to allow large artifact uploads through Nginx (default Nginx limit is `1m`); merged with `ingress.annotations`, explicit annotations take precedence for the same key

---

## [0.1.0] — 2026-04-03

### Added
- Complete rewrite from Java/Quarkus to Go 1.25 with Gin, pgx/v5, sqlc, golang-migrate
- REST API: artifacts, versions, files (multipart upload/download), tags
- Storage backends: filesystem (default) and S3 (aws-sdk-go-v2)
- K8s SA TokenReview auth middleware
- Helm chart under `deployment/` with PostgreSQL, S3, auth, ingress, autoscaling values
- Integration tests via testcontainers-go
