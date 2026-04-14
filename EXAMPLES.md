# Usage Examples

Practical `curl` examples for every endpoint of **fusion-index**.

All examples assume the service is reachable at `http://localhost:8080`. When running via the Minikube port-forward use `http://127.0.0.1:18080` instead.

---

## Table of Contents

1. [Health Probes](#health-probes)
2. [Templates](#templates)
3. [Template Versions](#template-versions)
4. [Jobs](#jobs)
5. [Job Versions](#job-versions)
6. [Artifacts](#artifacts)
7. [Error Cases](#error-cases)
8. [End-to-End Workflow](#end-to-end-workflow)

---

## Health Probes

```bash
# Liveness — always UP if the process is running
curl -s http://localhost:8080/q/health/live | python3 -m json.tool
# { "status": "UP" }

# Readiness — UP only when the DB is reachable
curl -s http://localhost:8080/q/health/ready | python3 -m json.tool
# { "status": "UP" }
```

---

## Templates

Templates define the reusable blueprint for a class of jobs.

### Create a template

```bash
curl -s -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "spark-etl",
    "description": "Apache Spark ETL pipeline template"
  }' | python3 -m json.tool
```

```json
{
  "id": 1,
  "name": "spark-etl",
  "description": "Apache Spark ETL pipeline template",
  "version": 1,
  "createdAt": "2026-04-14T10:00:00Z",
  "updatedAt": "2026-04-14T10:00:00Z"
}
```

### Get a template

```bash
curl -s http://localhost:8080/api/v1/templates/1 | python3 -m json.tool
```

### Update a template

```bash
curl -s -X PUT http://localhost:8080/api/v1/templates/1 \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Apache Spark ETL pipeline — updated description"
  }' | python3 -m json.tool
```

### List templates (paginated)

```bash
# Default page 1, pageSize 20
curl -s "http://localhost:8080/api/v1/templates" | python3 -m json.tool

# Custom page
curl -s "http://localhost:8080/api/v1/templates?page=2&pageSize=5" | python3 -m json.tool
```

```json
{
  "items": [
    {
      "id": 1,
      "name": "spark-etl",
      "description": "Apache Spark ETL pipeline template",
      "version": 2,
      "createdAt": "2026-04-14T10:00:00Z",
      "updatedAt": "2026-04-14T10:05:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

### Delete a template

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE http://localhost:8080/api/v1/templates/1
# 204
```

---

## Template Versions

Versions are immutable snapshots of a template. Every template starts at version 1.

### Publish a new version

```bash
curl -s -X POST http://localhost:8080/api/v1/templates/1/versions \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Added spark.executor.memory parameter",
    "schemaDefinition": "{\"type\":\"object\",\"properties\":{\"inputPath\":{\"type\":\"string\"},\"outputPath\":{\"type\":\"string\"}}}"
  }' | python3 -m json.tool
```

```json
{
  "id": 51,
  "jobTemplateId": 1,
  "versionNumber": 2,
  "description": "Added spark.executor.memory parameter",
  "schemaDefinition": "{...}",
  "createdAt": "2026-04-14T10:05:00Z"
}
```

### List versions

```bash
curl -s http://localhost:8080/api/v1/templates/1/versions | python3 -m json.tool
```

### Get a specific version

```bash
# Get version 2 of template 1
curl -s http://localhost:8080/api/v1/templates/1/versions/2 | python3 -m json.tool
```

---

## Jobs

Jobs are instances of a template version with runtime-specific configuration.

### Create a job

The `templateVersionId` must reference an existing template version.

```bash
curl -s -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "customer-churn-etl",
    "description": "Monthly churn prediction ETL run",
    "templateVersionId": 51,
    "config": "{\"inputPath\":\"s3://raw/customers/\",\"outputPath\":\"s3://processed/churn/\"}"
  }' | python3 -m json.tool
```

```json
{
  "id": 1,
  "name": "customer-churn-etl",
  "description": "Monthly churn prediction ETL run",
  "templateVersionId": 51,
  "version": 1,
  "config": "{...}",
  "createdAt": "2026-04-14T10:10:00Z",
  "updatedAt": "2026-04-14T10:10:00Z"
}
```

### Get a job

```bash
curl -s http://localhost:8080/api/v1/jobs/1 | python3 -m json.tool
```

### Update a job

```bash
curl -s -X PUT http://localhost:8080/api/v1/jobs/1 \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Monthly churn prediction ETL run — Q2 2026",
    "config": "{\"inputPath\":\"s3://raw/customers/q2/\",\"outputPath\":\"s3://processed/churn/q2/\"}"
  }' | python3 -m json.tool
```

### List jobs

```bash
curl -s "http://localhost:8080/api/v1/jobs?page=1&pageSize=10" | python3 -m json.tool
```

### Delete a job

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE http://localhost:8080/api/v1/jobs/1
# 204
```

---

## Job Versions

Job versions capture a snapshot of the job configuration at a point in time.

### Get version 1 (auto-created with the job)

```bash
curl -s http://localhost:8080/api/v1/jobs/1/versions/1 | python3 -m json.tool
```

```json
{
  "id": 1,
  "jobId": 1,
  "versionNumber": 1,
  "description": null,
  "templateVersionId": 51,
  "artifactCount": 0,
  "createdAt": "2026-04-14T10:10:00Z"
}
```

### Publish a new version

```bash
curl -s -X POST http://localhost:8080/api/v1/jobs/1/versions \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated input path for Q2 data",
    "templateVersionId": 51
  }' | python3 -m json.tool
```

### List job versions

```bash
curl -s http://localhost:8080/api/v1/jobs/1/versions | python3 -m json.tool
```

---

## Artifacts

Artifacts are binary files (models, reports, output zips) attached to a specific job version.

### Upload an artifact

```bash
# Upload a JSON report
curl -s -X POST \
  "http://localhost:8080/api/v1/jobs/1/versions/1/artifacts" \
  -F "file=@/tmp/report.json;type=application/json" \
  | python3 -m json.tool
```

```json
{
  "id": 1,
  "jobVersionId": 1,
  "filename": "report.json",
  "contentType": "application/json",
  "sizeBytes": 2048,
  "status": "AVAILABLE",
  "downloadUrl": "/api/v1/artifacts/1/download",
  "createdAt": "2026-04-14T10:20:00Z"
}
```

```bash
# Upload a trained model
curl -s -X POST \
  "http://localhost:8080/api/v1/jobs/1/versions/2/artifacts" \
  -F "file=@/tmp/model.pkl;type=application/octet-stream" \
  | python3 -m json.tool
```

### List artifacts for a job version

```bash
curl -s "http://localhost:8080/api/v1/jobs/1/versions/1/artifacts" \
  | python3 -m json.tool
```

### Get artifact metadata

```bash
curl -s http://localhost:8080/api/v1/artifacts/1 | python3 -m json.tool
```

### Download an artifact

```bash
# Stream to stdout
curl -s "http://localhost:8080/api/v1/artifacts/1/download"

# Save to file (filename taken from Content-Disposition header)
curl -OJ "http://localhost:8080/api/v1/artifacts/1/download"

# Save with explicit filename
curl -s "http://localhost:8080/api/v1/artifacts/1/download" \
  -o /tmp/downloaded-report.json
```

The response includes:
- `Content-Type` matching the uploaded MIME type
- `Content-Disposition: attachment; filename="<original-filename>"`

### List all artifacts (paginated, newest first)

```bash
curl -s "http://localhost:8080/api/v1/artifacts?page=1&pageSize=10" \
  | python3 -m json.tool
```

### Delete an artifact

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE http://localhost:8080/api/v1/artifacts/1
# 204
```

---

## Error Cases

### 400 — Missing required field

```bash
curl -s -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{"description": "no name provided"}' \
  | python3 -m json.tool
# HTTP 400
# { "error": "Key: 'CreateTemplateRequest.Name' Error: ..." }
```

### 404 — Resource not found

```bash
curl -s http://localhost:8080/api/v1/templates/99999 | python3 -m json.tool
# HTTP 404
# { "error": "not found" }
```

### 409 — Duplicate name

```bash
# Create the same template twice
curl -s -X POST http://localhost:8080/api/v1/templates \
  -H "Content-Type: application/json" \
  -d '{"name": "spark-etl"}' | python3 -m json.tool
# HTTP 409
# { "error": "template with this name already exists" }
```

### 409 — Delete blocked by referencing jobs

```bash
# Attempt to delete a template that has jobs
curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE http://localhost:8080/api/v1/templates/1
# 409
# { "error": "cannot delete template: 3 job(s) reference it" }
```

---

## End-to-End Workflow

A complete walkthrough: define a template, create a job, upload a result artifact, then clean up.

```bash
BASE=http://localhost:8080/api/v1

# 1. Create template
TEMPLATE=$(curl -s -X POST $BASE/templates \
  -H "Content-Type: application/json" \
  -d '{"name":"fraud-detection","description":"Fraud detection pipeline"}')
TEMPLATE_ID=$(echo $TEMPLATE | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Template ID: $TEMPLATE_ID"

# 2. Publish a template version with a schema
TV=$(curl -s -X POST $BASE/templates/$TEMPLATE_ID/versions \
  -H "Content-Type: application/json" \
  -d '{
    "description": "v1 — initial schema",
    "schemaDefinition": "{\"type\":\"object\",\"properties\":{\"threshold\":{\"type\":\"number\"}}}"
  }')
TV_ID=$(echo $TV | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Template version ID: $TV_ID"

# 3. Create a job
JOB=$(curl -s -X POST $BASE/jobs \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"fraud-q2-2026\",
    \"description\": \"Q2 fraud detection run\",
    \"templateVersionId\": $TV_ID,
    \"config\": \"{\\\"threshold\\\": 0.85}\"
  }")
JOB_ID=$(echo $JOB | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Job ID: $JOB_ID"

# 4. Upload a result artifact against job version 1
echo '{"fraudulent": 1423, "reviewed": 52000}' > /tmp/result.json
ARTIFACT=$(curl -s -X POST \
  "$BASE/jobs/$JOB_ID/versions/1/artifacts" \
  -F "file=@/tmp/result.json;type=application/json")
ARTIFACT_ID=$(echo $ARTIFACT | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Artifact ID: $ARTIFACT_ID"

# 5. Download and verify the artifact
curl -s "$BASE/artifacts/$ARTIFACT_ID/download"
# {"fraudulent": 1423, "reviewed": 52000}

# 6. List all artifacts for this job version
curl -s "$BASE/jobs/$JOB_ID/versions/1/artifacts" | python3 -m json.tool

# 7. Clean up — order matters: artifact → job → template
curl -s -o /dev/null -X DELETE $BASE/artifacts/$ARTIFACT_ID
curl -s -o /dev/null -X DELETE $BASE/jobs/$JOB_ID
curl -s -o /dev/null -X DELETE $BASE/templates/$TEMPLATE_ID

echo "Done."
```

---

## Useful one-liners

```bash
# Count total artifacts in the registry
curl -s "http://localhost:8080/api/v1/artifacts?pageSize=1" \
  | python3 -c "import sys,json; print('total artifacts:', json.load(sys.stdin)['total'])"

# List all template names
curl -s "http://localhost:8080/api/v1/templates?pageSize=100" \
  | python3 -c "import sys,json; [print(t['name']) for t in json.load(sys.stdin)['items']]"

# Check if a specific job exists by name (exit 0 if found)
curl -s "http://localhost:8080/api/v1/jobs?pageSize=100" \
  | python3 -c "
import sys, json, os
jobs = json.load(sys.stdin)['items']
found = any(j['name'] == 'fraud-q2-2026' for j in jobs)
print('found' if found else 'not found')
sys.exit(0 if found else 1)
"
```
