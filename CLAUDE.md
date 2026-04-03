# fusion-index — Index (Job Registry)

Stores, indexes, and exposes Fusion job definitions via REST API.

## Tech Stack
- **Language:** Java 21
- **Framework:** Quarkus 3.x
- **Build:** Maven 3.9

## Structure
```
src/
├── main/java/fusion/index/
│   ├── registry/     # Job registration & lookup
│   ├── schemas/      # Job definition schemas (Molecule-based)
│   └── api/          # REST API endpoints (JAX-RS / RESTEasy Reactive)
└── test/java/fusion/index/
```

## Key Conventions
- Use Quarkus Panache for persistence (active record or repository pattern)
- REST endpoints follow RESTEasy Reactive (`@Path`, `@GET`, `@POST` etc.)
- Validate all job schema inputs at the API boundary

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
| GET | `/openapi`, `/swagger-ui` | API docs |

## Local Minikube Deployment

```bash
eval $(minikube docker-env)
docker build -t fusion-index:latest .
helm upgrade --install fusion-index deployment/ \
  --namespace fusion \
  -f deployment/values-dev.yaml \
  --wait --timeout 3m
```

- `values-dev.yaml`: `pullPolicy: Never`, `STORAGE_BACKEND: FILESYSTEM`, postgres persistence disabled, `createNamespace: false`
- After rebuilding the image: `kubectl rollout restart deployment/index-backend -n fusion`
- `eval $(minikube docker-env)` only affects the current shell — re-run in every new terminal before `docker build`
- Port-forward requires `--address 127.0.0.1` or it fails silently: `kubectl port-forward -n fusion service/index-backend 18080:8080 --address 127.0.0.1`

## Local Testing (no cluster needed)

```bash
mvn test
```

Uses H2 in-memory (test profile in `src/test/resources/application.properties`). No database or minikube required. Flyway disabled; Hibernate recreates schema via `drop-and-create`.

## Known Pitfalls

### `maven-compiler-plugin` must be pinned
`pom.xml` must specify `<version>3.13.0</version>` on `maven-compiler-plugin`. Maven 3.8.x defaults
to plugin 3.1 which ignores `<release>21</release>` and falls back to source/target 5,
causing a compile error: `"Quelloption 5 wird nicht mehr unterstützt"`.

### JAX-RS resource classes require a class-level `@Path`
Without a class-level `@Path`, JAX-RS path specificity routes requests to the more-specific
root resource class instead. Symptom: `"Unable to find matching target resource method"` even
though the endpoint appears in the OpenAPI spec.

Artifact endpoints are split across two classes for this reason:
- `ArtifactResource` → `@Path("/api/v1/jobs")` — list + upload (under job version path)
- `ArtifactByIdResource` → `@Path("/api/v1/artifacts")` — list all (paginated) / get / download / delete

### Pagination pattern for list endpoints
All paginated `list()` resource methods are annotated `@Transactional` so that `listAll` and `countAll` share a single transaction — keeping `total` consistent with the returned page. `page` is validated with `@Min(0)` and `pageSize` with `@Min(1)`; invalid values return 400.

## Branch Strategy
`main` → `develop` → `feature/*`

## Commit Style
Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`
