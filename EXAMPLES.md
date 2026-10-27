# fusion-index API Examples

All examples target `http://127.0.0.1:18080`. Start the port-forward first:

```bash
kubectl port-forward -n fusion service/fusion-index-backend 18080:8080 --address 127.0.0.1
```

Or against ingress after adding `$(minikube ip) index.fusion.local` to `/etc/hosts` — replace the base URL with `http://index.fusion.local`.

---

## OpenAPI

```bash
# OpenAPI 3.1 spec as JSON
curl -s http://127.0.0.1:18080/api/openapi.json | python3 -m json.tool

# Swagger UI — open in browser
open http://127.0.0.1:18080/swagger/
# Or: xdg-open http://127.0.0.1:18080/swagger/
```

---

## Health

```bash
curl -s http://127.0.0.1:18080/q/health/live  | python3 -m json.tool
curl -s http://127.0.0.1:18080/q/health/ready | python3 -m json.tool
```

---

## Artifacts

### Create

```bash
# Minimal
curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.fusion.mylib"}' \
  | python3 -m json.tool

# With description
curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.fusion.mylib","description":"A reusable library"}' \
  | python3 -m json.tool

# Nested namespace
curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.platform.data.pipeline","description":"ETL pipeline artifact"}' \
  | python3 -m json.tool
```

**Expected errors:**

```bash
# 400 — missing fullName
curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{}' \
  | python3 -m json.tool

# 409 — duplicate name
curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.fusion.mylib"}' \
  | python3 -m json.tool
```

### List (paginated)

```bash
# All artifacts
curl -s "http://127.0.0.1:18080/api/v1/artifacts" | python3 -m json.tool

# Filter by name prefix
curl -s "http://127.0.0.1:18080/api/v1/artifacts?name=org.fusion" | python3 -m json.tool

# Filter by tag
curl -s "http://127.0.0.1:18080/api/v1/artifacts?tag=latest" | python3 -m json.tool

# Filters combine with AND
curl -s "http://127.0.0.1:18080/api/v1/artifacts?name=org.fusion&tag=latest&type=model" | python3 -m json.tool

# Sort: most versions first / fewest first / largest version (bytes) first
curl -s "http://127.0.0.1:18080/api/v1/artifacts?sort=versionCount" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?sort=versionCount&order=asc" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?sort=largestVersion" | python3 -m json.tool
# Each item carries "versionCount" and "largestVersionBytes"

# Size: all versions together / at least 1 GiB / artifacts without a "latest" tag / with a retention rule
curl -s "http://127.0.0.1:18080/api/v1/artifacts?sort=totalSize" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?minTotalBytes=1073741824" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?missingTag=latest" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?hasRetention=true" | python3 -m json.tool

# Maintenance views (same sets the admin cleanups delete, readable by everyone)
curl -s "http://127.0.0.1:18080/api/v1/artifacts?maxVersions=0" | python3 -m json.tool                  # no versions
curl -s "http://127.0.0.1:18080/api/v1/artifacts?minVersions=1&hasFiles=false" | python3 -m json.tool  # versions, no files
curl -s "http://127.0.0.1:18080/api/v1/artifacts?hasIncompleteFiles=true" | python3 -m json.tool      # failed/pending uploads

# Pagination
curl -s "http://127.0.0.1:18080/api/v1/artifacts?page=0&pageSize=5" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts?page=1&pageSize=5" | python3 -m json.tool
```

### Get

```bash
curl -s http://127.0.0.1:18080/api/v1/artifacts/1 | python3 -m json.tool

# 404
curl -s http://127.0.0.1:18080/api/v1/artifacts/999999 | python3 -m json.tool
```

### Update description

```bash
curl -s -X PUT http://127.0.0.1:18080/api/v1/artifacts/1 \
  -H "Content-Type: application/json" \
  -d '{"description":"Updated description"}' \
  | python3 -m json.tool

# Clear description (set to null)
curl -s -X PUT http://127.0.0.1:18080/api/v1/artifacts/1 \
  -H "Content-Type: application/json" \
  -d '{"description":null}' \
  | python3 -m json.tool
```

### Delete

```bash
curl -s -X DELETE http://127.0.0.1:18080/api/v1/artifacts/1
# 204 No Content on success — versions, files, tags go with it, files also from storage

# Confirm deletion
curl -s http://127.0.0.1:18080/api/v1/artifacts/1 | python3 -m json.tool
# → 404
```

---

## Versions

All examples assume artifact id=1 (`org.fusion.mylib`). Recreate it if needed:

```bash
ARTIFACT_ID=$(curl -s -X POST http://127.0.0.1:18080/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.fusion.mylib","description":"A reusable library"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Artifact ID: $ARTIFACT_ID"
```

### Create version

```bash
# Minimal semver
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0"}' \
  | python3 -m json.tool

# With JSON config and tags
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "2.0.0",
    "config": "{\"runtime\":\"python3.12\",\"entrypoint\":\"main.py\",\"memory\":\"512m\"}",
    "tags": ["latest", "stable"]
  }' \
  | python3 -m json.tool

# With YAML config
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "2.1.0",
    "config": "runtime: python3.12\nentrypoint: main.py\nmemory: 512m"
  }' \
  | python3 -m json.tool
```

**Expected errors:**

```bash
# 409 — duplicate version
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0"}' \
  | python3 -m json.tool

# 400 — invalid semver
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"not-a-semver"}' \
  | python3 -m json.tool

# 400 — missing version field
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" \
  -H "Content-Type: application/json" \
  -d '{}' \
  | python3 -m json.tool

# 404 — artifact not found
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/999999/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0"}' \
  | python3 -m json.tool
```

### List versions

```bash
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions" | python3 -m json.tool
# Returns sorted by semver descending (major, minor, patch); each version carries "totalBytes"

# Same filters as the bulk delete — show before you delete
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?keepLast=5" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?keepLast=5&deletable=true" | python3 -m json.tool  # exactly what DELETE would remove
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?hasFiles=false" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?from=1.0.0&below=2.0.0" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?minBytes=104857600" | python3 -m json.tool  # >= 100 MiB

# Across all artifacts (paginated)
curl -s "http://127.0.0.1:18080/api/v1/versions?hasIncompleteFiles=true" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/versions?keepLast=10&deletable=true&pageSize=50" | python3 -m json.tool

# Files of a version by status
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/1.0.0/files?status=PENDING&status=ERROR" | python3 -m json.tool
```

### Get specific version

```bash
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0" | python3 -m json.tool

# 404
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/9.9.9" | python3 -m json.tool
```

### Delete version (also cleans up storage files)

```bash
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/1.0.0"
# 204 No Content

# Confirm gone
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/1.0.0" | python3 -m json.tool
# → 404
```

### Bulk delete versions

Filters combine with AND, at least one is required. Tagged versions and protected artifacts (`ADMIN_PROTECTED_TAG`) are skipped. Files are removed from storage too.

```bash
# Preview: keep the newest 5 versions, delete the rest
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?keepLast=5&dryRun=true" \
  | python3 -m json.tool
# {"deleted": 12, "skipped": 1, "dryRun": true, "versions": [{"artifactId": 1, "version": "1.0.0"}, ...]}

# Everything below 2.0.0 that is older than 90 days
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?below=2.0.0&olderThan=2026-07-29T00:00:00Z"

# Range: 1.0.0 (inclusive) up to 1.5.0 (exclusive)
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?from=1.0.0&below=1.5.0"

# Exact versions
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?version=1.0.0&version=1.0.1"

# Versions without any file
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?hasFiles=false"

# Registry-wide (admin): keep the newest 10 versions of every artifact
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/versions?keepLast=10&dryRun=true" | python3 -m json.tool
```

---

## Tags

Tags are unique per artifact. Assigning a tag that already exists **moves** it to the new version.

```bash
# Across all artifacts: every "latest", and tags whose version has nothing downloadable
curl -s "http://127.0.0.1:18080/api/v1/tags?tag=latest" | python3 -m json.tool
curl -s "http://127.0.0.1:18080/api/v1/tags?broken=true" | python3 -m json.tool
```

### Assign tag

```bash
# Assign "latest" to 2.0.0
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/latest" \
  -H "Content-Type: application/json" \
  -d '{"version":"2.0.0"}' \
  | python3 -m json.tool

# Assign "stable" to 1.0.0
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/stable" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0"}' \
  | python3 -m json.tool
```

### Move a tag to a different version

```bash
# "latest" is on 2.0.0 — move it to 2.1.0
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/latest" \
  -H "Content-Type: application/json" \
  -d '{"version":"2.1.0"}' \
  | python3 -m json.tool

# Confirm: 2.0.0 no longer has "latest"
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0" \
  | python3 -c "import sys,json; v=json.load(sys.stdin); print('tags on 2.0.0:', [t['tag'] for t in v['tags']])"

# Confirm: 2.1.0 now has "latest"
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.1.0" \
  | python3 -c "import sys,json; v=json.load(sys.stdin); print('tags on 2.1.0:', [t['tag'] for t in v['tags']])"
```

**Expected errors:**

```bash
# 404 — version does not exist
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/latest" \
  -H "Content-Type: application/json" \
  -d '{"version":"9.9.9"}' \
  | python3 -m json.tool

# 400 — invalid semver in body
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/latest" \
  -H "Content-Type: application/json" \
  -d '{"version":"bad"}' \
  | python3 -m json.tool
```

### Delete tag

```bash
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/stable"
# 204 No Content

# 404 — tag doesn't exist
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/tags/nonexistent" \
  | python3 -m json.tool
```

---

## Files

### Upload a file

```bash
# Upload any local file to a version
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -F "file=@go.mod" \
  | python3 -m json.tool

# Upload with explicit content type
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -F "file=@README.md" \
  -F "contentType=text/markdown" \
  | python3 -m json.tool

# Upload a second file to the same version
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -F "file=@go.sum" \
  | python3 -m json.tool
```

Store the file id for later:

```bash
FILE_ID=$(curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -F "file=@Dockerfile" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "File ID: $FILE_ID"
```

**Expected errors:**

```bash
# 400 — no file field
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -H "Content-Type: application/json" -d '{}' \
  | python3 -m json.tool

# 404 — version not found
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/9.9.9/files" \
  -F "file=@go.mod" \
  | python3 -m json.tool

# 409 — same filename already uploaded to this version
curl -s -X POST "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  -F "file=@go.mod" \
  | python3 -m json.tool
```

### List files for a version

```bash
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files" \
  | python3 -m json.tool
```

### Get file metadata

```bash
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID" \
  | python3 -m json.tool

# 404
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/999999" \
  | python3 -m json.tool
```

### Download a file

```bash
# Stream to stdout
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID/download"

# Save to disk
curl -OJ "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID/download"
# -O saves with the server-provided filename from Content-Disposition

# Check Content-Disposition header
curl -I "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID/download"
```

### Delete a file

```bash
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID"
# 204 No Content

# Confirm gone
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions/2.0.0/files/$FILE_ID" \
  | python3 -m json.tool
# → 404
```

---

## Types

Artifact types are a shared taxonomy (e.g. `model`, `pipeline`, `library`) that artifacts can be tagged with, independent of the per-artifact `tag` pointers.

### Create a type

```bash
curl -s -X POST http://127.0.0.1:18080/api/v1/types \
  -H "Content-Type: application/json" \
  -d '{"name":"model","description":"Trained model checkpoints"}' \
  | python3 -m json.tool
```

**Expected errors:**

```bash
# 409 — duplicate type name
curl -s -X POST http://127.0.0.1:18080/api/v1/types \
  -H "Content-Type: application/json" \
  -d '{"name":"model"}' \
  | python3 -m json.tool
```

### List / get / update / delete types

```bash
curl -s http://127.0.0.1:18080/api/v1/types | python3 -m json.tool

curl -s http://127.0.0.1:18080/api/v1/types/1 | python3 -m json.tool

curl -s -X PUT http://127.0.0.1:18080/api/v1/types/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"model","description":"Updated description"}' \
  | python3 -m json.tool

curl -s -X DELETE http://127.0.0.1:18080/api/v1/types/1
# 204 No Content

# Types no artifact uses
curl -s "http://127.0.0.1:18080/api/v1/types?unused=true" | python3 -m json.tool
```

### Retention policies

Stored and reported only — an external runner applies them via the bulk version delete.

```bash
# Every artifact of type 1: keep the newest 10 versions, delete older ones only once they are 90 days old
curl -s -X PUT http://127.0.0.1:18080/api/v1/types/1/retention \
  -H "Content-Type: application/json" -d '{"keepLast":10,"maxAgeDays":90}' | python3 -m json.tool

# Artifact-specific override
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/retention" \
  -H "Content-Type: application/json" -d '{"keepLast":3}' | python3 -m json.tool

# Effective rule + ready-made filter
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/retention" | python3 -m json.tool
# {"source":"artifact","rule":{"keepLast":3,"maxAgeDays":null},"own":{...},"fromTypes":[...],"versionFilter":"keepLast=3"}

# What the rule would delete, then delete it
FILTER=$(curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/retention" | python3 -c 'import json,sys; print(json.load(sys.stdin)["versionFilter"])')
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?$FILTER&deletable=true" | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/versions?$FILTER" | python3 -m json.tool

curl -s http://127.0.0.1:18080/api/v1/retention-policies | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/retention"   # 204
```

### Assign / unassign a type on an artifact

```bash
# Assign type 1 to artifact $ARTIFACT_ID
curl -s -X PUT "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/types/1" \
  | python3 -m json.tool

# List types on an artifact
curl -s "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/types" | python3 -m json.tool

# Filter artifact list by type
curl -s "http://127.0.0.1:18080/api/v1/artifacts?type=model" | python3 -m json.tool

# Unassign
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/artifacts/$ARTIFACT_ID/types/1"
# 204 No Content
```

---

## Admin (maintenance)

Bulk inspection/cleanup of abandoned registry data. Every endpoint requires `olderThan` (RFC3339 timestamp); artifacts/versions carrying the tag configured via `ADMIN_PROTECTED_TAG` (default `protect`) are skipped by delete operations.

```bash
# Artifacts with no versions
curl -s "http://127.0.0.1:18080/api/v1/admin/artifacts/empty?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/artifacts/empty?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
# {"deleted": N, "skipped": M}

# Versions with no files
curl -s "http://127.0.0.1:18080/api/v1/admin/versions/empty?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/versions/empty?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool

# Files stuck in PENDING/ERROR (interrupted or failed uploads) — run this first,
# they otherwise keep their version from counting as file-less
curl -s "http://127.0.0.1:18080/api/v1/admin/files/incomplete?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/files/incomplete?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool

# Types assigned to no artifact
curl -s "http://127.0.0.1:18080/api/v1/admin/types/unused?olderThan=2026-01-01T00:00:00Z" | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/types/unused?olderThan=2026-01-01T00:00:00Z" | python3 -m json.tool

# Artifacts whose versions all have no files (cascades)
curl -s "http://127.0.0.1:18080/api/v1/admin/artifacts/no-files?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
curl -s -X DELETE "http://127.0.0.1:18080/api/v1/admin/artifacts/no-files?olderThan=2026-01-01T00:00:00Z" \
  | python3 -m json.tool
```

**Expected errors:**

```bash
# 400 — missing olderThan
curl -s "http://127.0.0.1:18080/api/v1/admin/artifacts/empty" | python3 -m json.tool
```

---

## End-to-end workflow

Create an artifact, publish two versions with tags, upload a file, then resolve by tag.

```bash
BASE=http://127.0.0.1:18080

# 1. Create artifact
AID=$(curl -s -X POST $BASE/api/v1/artifacts \
  -H "Content-Type: application/json" \
  -d '{"fullName":"org.example.processor","description":"Data processor"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Artifact: $AID"

# 2. Publish v1.0.0 as "stable"
curl -s -X POST "$BASE/api/v1/artifacts/$AID/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0","config":"{\"workers\":2}","tags":["stable"]}' \
  | python3 -m json.tool

# 3. Upload file to v1.0.0
curl -s -X POST "$BASE/api/v1/artifacts/$AID/versions/1.0.0/files" \
  -F "file=@go.mod" \
  | python3 -m json.tool

# 4. Publish v2.0.0 as "latest"
curl -s -X POST "$BASE/api/v1/artifacts/$AID/versions" \
  -H "Content-Type: application/json" \
  -d '{"version":"2.0.0","config":"{\"workers\":4}","tags":["latest"]}' \
  | python3 -m json.tool

# 5. Upload file to v2.0.0
curl -s -X POST "$BASE/api/v1/artifacts/$AID/versions/2.0.0/files" \
  -F "file=@go.sum" \
  | python3 -m json.tool

# 6. Find artifact by tag=latest
curl -s "$BASE/api/v1/artifacts?tag=latest" | python3 -m json.tool

# 7. Inspect all versions (sorted newest first)
curl -s "$BASE/api/v1/artifacts/$AID/versions" | python3 -m json.tool

# 8. Promote v2.0.0 to stable as well
curl -s -X PUT "$BASE/api/v1/artifacts/$AID/tags/stable" \
  -H "Content-Type: application/json" \
  -d '{"version":"2.0.0"}' \
  | python3 -m json.tool

# 9. v1.0.0 now has no tags
curl -s "$BASE/api/v1/artifacts/$AID/versions/1.0.0" \
  | python3 -c "import sys,json; v=json.load(sys.stdin); print('v1.0.0 tags:', v['tags'])"

# 10. v2.0.0 has both tags
curl -s "$BASE/api/v1/artifacts/$AID/versions/2.0.0" \
  | python3 -c "import sys,json; v=json.load(sys.stdin); print('v2.0.0 tags:', [t['tag'] for t in v['tags']])"
```
