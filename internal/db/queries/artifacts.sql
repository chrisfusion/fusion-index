-- name: CreateArtifact :one
INSERT INTO artifact (job_version_id, name, content_type, storage_backend, storage_path, status)
VALUES ($1, $2, $3, $4, $5, 'PENDING')
RETURNING *;

-- name: GetArtifact :one
SELECT * FROM artifact WHERE id = $1;

-- name: UpdateArtifactStored :one
UPDATE artifact
SET storage_path = $2,
    size_bytes   = $3,
    status       = $4,
    updated_at   = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateArtifactStatus :one
UPDATE artifact
SET status     = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListArtifacts :many
SELECT * FROM artifact
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountArtifacts :one
SELECT COUNT(*) FROM artifact;

-- name: ListArtifactsByJobVersion :many
SELECT * FROM artifact
WHERE job_version_id = $1
ORDER BY created_at ASC;

-- name: DeleteArtifact :exec
DELETE FROM artifact WHERE id = $1;
