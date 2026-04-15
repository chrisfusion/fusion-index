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
# 204 No Content on success

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
# Returns sorted by semver descending (major, minor, patch)
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

---

## Tags

Tags are unique per artifact. Assigning a tag that already exists **moves** it to the new version.

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
