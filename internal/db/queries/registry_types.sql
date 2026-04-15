-- name: CreateArtifactType :one
INSERT INTO registry_artifact_type (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetArtifactType :one
SELECT * FROM registry_artifact_type WHERE id = $1;

-- name: GetArtifactTypeByName :one
SELECT * FROM registry_artifact_type WHERE name = $1;

-- name: ListArtifactTypes :many
SELECT * FROM registry_artifact_type
ORDER BY name ASC;

-- name: UpdateArtifactType :one
UPDATE registry_artifact_type
SET name        = $2,
    description = $3,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteArtifactType :exec
DELETE FROM registry_artifact_type WHERE id = $1;
