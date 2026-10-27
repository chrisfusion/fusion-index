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

-- name: ListAvailableS3FilePaths :many
-- storage_path values for every available S3-backed file, used to drive the S3
-- prefix migration Job (internal/storage/migrate.go): the DB is the exact manifest
-- of keys this instance owns, so migration never has to list bucket contents.
SELECT storage_path FROM registry_artifact_file
WHERE storage_backend = 'S3' AND status = 'AVAILABLE'
ORDER BY id;

-- name: ListIncompleteFiles :many
-- PENDING/ERROR files left behind by interrupted or failed uploads (admin cleanup).
-- The version coordinates are returned so callers can rebuild the storage path:
-- storage_path is still the 'pending' placeholder for these rows.
SELECT sqlc.embed(f), v.artifact_id, v.major, v.minor, v.patch
FROM registry_artifact_file f
JOIN registry_artifact_version v ON v.id = f.version_id
WHERE f.status IN ('PENDING', 'ERROR')
  AND f.created_at < $1
ORDER BY f.id
LIMIT $2 OFFSET $3;

-- name: CountIncompleteFiles :one
SELECT COUNT(*) FROM registry_artifact_file
WHERE status IN ('PENDING', 'ERROR')
  AND created_at < $1;

-- name: DeleteIncompleteFiles :many
WITH deleted AS (
  DELETE FROM registry_artifact_file f
  WHERE f.status IN ('PENDING', 'ERROR')
    AND f.created_at < $1
    AND NOT EXISTS (
      SELECT 1 FROM registry_artifact_version v
      JOIN registry_artifact_tag rat ON rat.artifact_id = v.artifact_id
      WHERE v.id = f.version_id AND rat.tag = $2
    )
  RETURNING f.id, f.name, f.version_id
)
SELECT d.id, d.name, v.artifact_id, v.major, v.minor, v.patch
FROM deleted d
JOIN registry_artifact_version v ON v.id = d.version_id;

-- name: ListFilesByVersionIDs :many
SELECT * FROM registry_artifact_file
WHERE version_id = ANY(sqlc.arg(version_ids)::bigint[])
ORDER BY id;

-- name: ListFilesByArtifactID :many
-- All files of an artifact with their version coordinates, so the artifact delete
-- can remove the storage objects after the cascade has dropped the rows.
SELECT sqlc.embed(f), v.major, v.minor, v.patch
FROM registry_artifact_file f
JOIN registry_artifact_version v ON v.id = f.version_id
WHERE v.artifact_id = $1
ORDER BY f.id;
