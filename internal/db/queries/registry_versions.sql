-- name: CreateArtifactVersion :one
INSERT INTO registry_artifact_version (artifact_id, major, minor, patch, config)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetArtifactVersion :one
SELECT * FROM registry_artifact_version
WHERE artifact_id = $1
  AND major = $2
  AND minor = $3
  AND patch = $4;

-- name: GetArtifactVersionByID :one
SELECT * FROM registry_artifact_version WHERE id = $1;

-- name: ListArtifactVersions :many
SELECT * FROM registry_artifact_version
WHERE artifact_id = $1
ORDER BY major DESC, minor DESC, patch DESC;

-- name: DeleteArtifactVersion :exec
DELETE FROM registry_artifact_version WHERE id = $1;

-- name: ListVersionsWithoutFiles :many
SELECT rav.* FROM registry_artifact_version rav
WHERE rav.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_file raf WHERE raf.version_id = rav.id)
ORDER BY rav.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountVersionsWithoutFiles :one
SELECT COUNT(*) FROM registry_artifact_version rav
WHERE rav.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_file raf WHERE raf.version_id = rav.id);

-- name: DeleteVersionsWithoutFiles :execrows
DELETE FROM registry_artifact_version
WHERE registry_artifact_version.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_file raf WHERE raf.version_id = registry_artifact_version.id)
  AND NOT EXISTS (
    SELECT 1 FROM registry_artifact_tag rat
    WHERE rat.artifact_id = registry_artifact_version.artifact_id AND rat.tag = $2
  );
