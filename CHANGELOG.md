# Changelog

All notable changes to fusion-index are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

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
