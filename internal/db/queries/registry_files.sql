-- name: CreateArtifactFile :one
INSERT INTO registry_artifact_file (version_id, name, content_type, storage_backend, storage_path, status)
VALUES ($1, $2, $3, $4, $5, 'PENDING')
RETURNING *;

-- name: GetArtifactFile :one
SELECT * FROM registry_artifact_file WHERE id = $1;

-- name: ListArtifactFiles :many
SELECT * FROM registry_artifact_file
WHERE version_id = $1
ORDER BY created_at ASC;

-- name: UpdateArtifactFileStored :one
UPDATE registry_artifact_file
SET storage_path = $2,
    size_bytes   = $3,
    status       = $4,
    updated_at   = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateArtifactFileStatus :exec
UPDATE registry_artifact_file
SET status     = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteArtifactFile :exec
DELETE FROM registry_artifact_file WHERE id = $1;
