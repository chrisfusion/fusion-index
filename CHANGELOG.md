# Changelog

All notable changes to fusion-index are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

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
