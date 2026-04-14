# fusion-index — Index (Job Registry)

Stores, indexes, and exposes Fusion job definitions via REST API.

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
├── db/
│   ├── queries/                # hand-written SQL (sqlc input)
│   └── sqlc/                   # generated Go — DO NOT EDIT
├── storage/
│   ├── storage.go              # Storage interface
│   ├── filesystem.go
│   └── s3.go
└── api/
    ├── router.go               # gin setup, routes, CORS
    ├── handlers/
    │   ├── templates.go
    │   ├── jobs.go
    │   ├── artifacts.go
    │   └── helpers.go          # shared path parsing, pagination, error helpers
    └── dto/
        ├── requests.go         # binding-tagged request structs
        └── responses.go        # response structs + mapper functions
migrations/                     # golang-migrate up-only SQL files
tests/integration/              # real-Postgres tests via testcontainers-go
sqlc.yaml                       # sqlc config
```

## Key Conventions
- All DB access goes through `internal/db/sqlc` (generated). Never write raw pgx queries outside that layer.
- sqlc timestamp override for `pg_catalog.timestamptz → time.Time` does NOT apply; sqlc generates `pgtype.Timestamptz`. Always access `.Time` in response mappers.
- golang-migrate uses `lib/pq` internally (not pgx). Append `?sslmode=disable` to DBURL or migrations fail against Bitnami Postgres (no TLS in dev).
- Regenerate after SQL changes: `~/go/bin/sqlc generate`
- Transactions are opened in handlers that need atomicity (create + version bump). Pass `q.WithTx(tx)` to queries.
- Nullable columns from sqlc become `*string` / `*int64` (`emit_pointers_for_null_types: true`). Timestamps are `pgtype.Timestamptz`; access `.Time` for `time.Time`.
- Error responses always have shape `{"error": "..."}`.

## REST API (validated)

| Method | Path | Description |
|---|---|---|
| GET/POST | `/api/v1/templates` | List / create job templates |
| GET/PUT/DELETE | `/api/v1/templates/{id}` | Get / update / delete template |
| GET/POST | `/api/v1/templates/{id}/versions` | List / publish template version |
| GET | `/api/v1/templates/{id}/versions/{n}` | Get specific template version |
| GET/POST | `/api/v1/jobs` | List / create jobs |
| GET/PUT/DELETE | `/api/v1/jobs/{id}` | Get / update / delete job |
| GET/POST | `/api/v1/jobs/{id}/versions` | List / publish job version |
| GET | `/api/v1/jobs/{id}/versions/{n}` | Get specific job version |
| GET/POST | `/api/v1/jobs/{jobId}/versions/{n}/artifacts` | List / upload artifact |
| GET | `/api/v1/artifacts` | List all artifacts (paginated, sorted by createdAt DESC) |
| GET/DELETE | `/api/v1/artifacts/{id}` | Get metadata / delete artifact |
| GET | `/api/v1/artifacts/{id}/download` | Download artifact stream |
| GET | `/q/health/live`, `/q/health/ready` | Kubernetes health probes |

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

## Branch Strategy
`main` → `develop` → `feature/*`

## Commit Style
Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`
