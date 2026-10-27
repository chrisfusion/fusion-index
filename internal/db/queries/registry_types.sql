-- name: CreateArtifactType :one
INSERT INTO registry_artifact_type (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetArtifactType :one
SELECT * FROM registry_artifact_type WHERE id = $1;

-- name: GetArtifactTypeByName :one
SELECT * FROM registry_artifact_type WHERE name = $1;

-- name: ListArtifactTypes :many
-- unused = NULL lists all types; true only types assigned to no artifact.
SELECT t.* FROM registry_artifact_type t
WHERE sqlc.narg(unused)::bool IS NULL
   OR NOT EXISTS (SELECT 1 FROM registry_artifact_type_map m WHERE m.type_id = t.id) = sqlc.narg(unused)::bool
ORDER BY t.name ASC;

-- name: ListUnusedArtifactTypes :many
SELECT t.* FROM registry_artifact_type t
WHERE t.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_type_map m WHERE m.type_id = t.id)
ORDER BY t.name ASC
LIMIT $2 OFFSET $3;

-- name: CountUnusedArtifactTypes :one
SELECT COUNT(*) FROM registry_artifact_type t
WHERE t.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_type_map m WHERE m.type_id = t.id);

-- name: DeleteUnusedArtifactTypes :execrows
-- Retention policies on these types go with them (ON DELETE CASCADE).
DELETE FROM registry_artifact_type t
WHERE t.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_type_map m WHERE m.type_id = t.id);

-- name: UpdateArtifactType :one
UPDATE registry_artifact_type
SET name        = $2,
    description = $3,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteArtifactType :exec
DELETE FROM registry_artifact_type WHERE id = $1;
