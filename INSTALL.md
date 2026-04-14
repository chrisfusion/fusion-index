# Installation Guide

This guide covers every deployment scenario for **fusion-index**, from a local development shell to a production Kubernetes cluster.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development (bare Go)](#local-development)
3. [Docker (single container)](#docker)
4. [Minikube (local Kubernetes)](#minikube)
5. [Production Kubernetes via Helm](#production-kubernetes)
6. [Storage Backends](#storage-backends)
7. [Database: external PostgreSQL](#external-postgresql)
8. [Upgrading](#upgrading)

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.25+ | `go version` |
| PostgreSQL | 15 or 16 | Bitnami subchart or external |
| Docker | 20+ | Required for integration tests and image builds |
| kubectl | 1.28+ | For Kubernetes deployments |
| Helm | 3.14+ | For chart installs |
| Minikube | 1.32+ | Local K8s only |
| sqlc | 1.30.0 | Only when regenerating queries: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0` |

---

## Local Development

### 1. Start PostgreSQL

```bash
docker run -d --name fusion-pg \
  -e POSTGRES_USER=fusion \
  -e POSTGRES_PASSWORD=fusion \
  -e POSTGRES_DB=fusion_index \
  -p 5432:5432 \
  postgres:16-alpine
```

### 2. Build and run

```bash
go build -o fusion-index ./cmd/server

export DB_HOST=localhost
export DB_PASSWORD=fusion
export STORAGE_BACKEND=FILESYSTEM

./fusion-index
```

Migrations run automatically at startup. The server is ready when you see:

```
[GIN-debug] Listening and serving HTTP on :8080
```

### 3. Health check

```bash
curl -s http://localhost:8080/q/health/ready
# {"status":"UP"}
```

### 4. Run integration tests

Docker must be running — testcontainers-go starts its own PostgreSQL container.

```bash
go test ./tests/integration/... -v -timeout 120s
```

### 5. Regenerate sqlc queries (after SQL changes)

```bash
~/go/bin/sqlc generate
```

---

## Docker

### Build the image

```bash
docker build -t fusion-index:latest .
```

### Run with filesystem storage

```bash
docker run -d --name fusion-index \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e DB_PASSWORD=fusion \
  -e STORAGE_BACKEND=FILESYSTEM \
  -v /tmp/fusion-artifacts:/data/artifacts \
  fusion-index:latest
```

### Run with MinIO (S3-compatible)

```bash
# Start MinIO first
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  quay.io/minio/minio server /data --console-address ":9001"

# Start fusion-index pointing at MinIO
docker run -d --name fusion-index \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e DB_PASSWORD=fusion \
  -e STORAGE_BACKEND=S3 \
  -e S3_BUCKET=fusion-index-artifacts \
  -e AWS_REGION=us-east-1 \
  -e S3_ENDPOINT_OVERRIDE=http://host.docker.internal:9000 \
  -e AWS_ACCESS_KEY_ID=minioadmin \
  -e AWS_SECRET_ACCESS_KEY=minioadmin \
  fusion-index:latest
```

---

## Minikube

### One-time setup

```bash
minikube start --cpus=4 --memory=8g
# Point Docker CLI at Minikube's daemon so images are available in-cluster
eval $(minikube docker-env)
```

### Build the image inside Minikube

```bash
docker build -t fusion-index:latest .
```

### Create a dev values override

`deployment/values-dev.yaml` (already present in the repo):

```yaml
backend:
  image:
    tag: latest
    pullPolicy: Never   # use the locally-built image
  ginMode: debug
  replicas: 1

postgresql:
  auth:
    password: "dev-password"

s3:
  credentialsType: static
  accessKeyId: minioadmin
  secretAccessKey: minioadmin
  endpointOverride: ""   # set to MinIO ClusterIP if using S3 in dev
```

### Install (or upgrade)

```bash
# Remove an existing release first if needed
helm uninstall fusion-index -n fusion 2>/dev/null || true

helm upgrade --install fusion-index deployment/ \
  --namespace fusion \
  --create-namespace \
  -f deployment/values-dev.yaml \
  --wait --timeout 3m
```

### Verify pods

```bash
kubectl get pods -n fusion
# NAME                                    READY   STATUS    RESTARTS
# fusion-index-backend-<hash>             1/1     Running   0
# fusion-index-postgresql-0              1/1     Running   0
```

### Access the API

```bash
kubectl port-forward -n fusion service/fusion-index-backend 18080:8080 --address 127.0.0.1 &
curl -s http://127.0.0.1:18080/q/health/ready
```

### After rebuilding the image

```bash
eval $(minikube docker-env)
docker build -t fusion-index:latest .
kubectl rollout restart deployment/fusion-index-backend -n fusion
```

---

## Production Kubernetes

### 1. Push the image to your registry

```bash
docker build -t registry.example.com/fusion/fusion-index:0.1.0 .
docker push registry.example.com/fusion/fusion-index:0.1.0
```

### 2. Create a production values file

`values-prod.yaml` (do **not** commit to git):

```yaml
namespace: fusion

backend:
  image:
    repository: registry.example.com/fusion/fusion-index
    tag: "0.1.0"
    pullPolicy: Always
  replicas: 2
  dbSSLMode: require
  storageBackend: S3
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: "2"
      memory: 1Gi
  podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
  podSecurityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 2000
  containerSecurityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop: [ALL]
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 8
    targetCPUUtilizationPercentage: 70

postgresql:
  enabled: false
  external:
    host: pg.prod.internal
    port: 5432
    database: fusion_index
    username: fusion
    existingSecret: fusion-index-db-creds   # key: password

s3:
  bucket: fusion-index-artifacts-prod
  region: eu-central-1
  credentialsType: default   # IRSA / Workload Identity

ingress:
  enabled: true
  className: nginx
  host: index.fusion.example.com
  tls:
    enabled: true
    secretName: fusion-index-tls
```

### 3. Deploy

```bash
helm upgrade --install fusion-index deployment/ \
  --namespace fusion \
  --create-namespace \
  -f values-prod.yaml \
  --wait --timeout 5m
```

---

## Storage Backends

### Filesystem (default for dev)

```
STORAGE_BACKEND=FILESYSTEM
STORAGE_FS_ROOT=/data/artifacts   # must be on a persistent volume in K8s
```

Files are stored under `{STORAGE_FS_ROOT}/{uuid}`. In Kubernetes you need a `PersistentVolumeClaim` mounted at that path.

### S3 / S3-compatible

```
STORAGE_BACKEND=S3
S3_BUCKET=my-bucket
AWS_REGION=us-east-1
S3_ENDPOINT_OVERRIDE=       # empty for AWS; set for MinIO/Ceph
```

Authentication is via standard AWS credential chain: env vars → IRSA → instance profile. For static credentials set `credentialsType: static` in Helm values (or inject `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` directly).

---

## External PostgreSQL

Set `postgresql.enabled: false` and fill in the `postgresql.external.*` block:

```yaml
postgresql:
  enabled: false
  external:
    host: pg.internal
    port: 5432
    database: fusion_index
    username: fusion
    existingSecret: my-pg-secret   # Kubernetes Secret with key: password
```

The chart will create no internal PostgreSQL and will reference your secret directly.

---

## Upgrading

1. Build and push the new image.
2. Update `backend.image.tag` in your values file.
3. Run `helm upgrade` — migrations run automatically at startup.

```bash
helm upgrade fusion-index deployment/ \
  --namespace fusion \
  -f values-prod.yaml \
  --wait --timeout 5m
```

To check rollout status:

```bash
kubectl rollout status deployment/fusion-index-backend -n fusion
```
