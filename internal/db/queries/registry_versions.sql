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
