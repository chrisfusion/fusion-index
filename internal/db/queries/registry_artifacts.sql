-- name: CreateRegistryArtifact :one
INSERT INTO registry_artifact (full_name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetRegistryArtifact :one
SELECT * FROM registry_artifact WHERE id = $1;

-- name: GetRegistryArtifactByName :one
SELECT * FROM registry_artifact WHERE full_name = $1;

-- name: ListRegistryArtifacts :many
SELECT * FROM registry_artifact
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListRegistryArtifactsByName :many
SELECT * FROM registry_artifact
WHERE full_name ILIKE $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRegistryArtifactsByTag :many
SELECT ra.* FROM registry_artifact ra
JOIN registry_artifact_tag rat ON rat.artifact_id = ra.id
WHERE rat.tag = $1
ORDER BY ra.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRegistryArtifacts :one
SELECT COUNT(*) FROM registry_artifact;

-- name: CountRegistryArtifactsByName :one
SELECT COUNT(*) FROM registry_artifact
WHERE full_name ILIKE $1;

-- name: CountRegistryArtifactsByTag :one
SELECT COUNT(*) FROM registry_artifact ra
JOIN registry_artifact_tag rat ON rat.artifact_id = ra.id
WHERE rat.tag = $1;

-- name: ListRegistryArtifactsByTypes :many
SELECT DISTINCT ra.* FROM registry_artifact ra
JOIN registry_artifact_type_map ratm ON ratm.artifact_id = ra.id
JOIN registry_artifact_type rat ON rat.id = ratm.type_id
WHERE rat.name = ANY($1::text[])
ORDER BY ra.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRegistryArtifactsByTypes :one
SELECT COUNT(DISTINCT ra.id) FROM registry_artifact ra
JOIN registry_artifact_type_map ratm ON ratm.artifact_id = ra.id
JOIN registry_artifact_type rat ON rat.id = ratm.type_id
WHERE rat.name = ANY($1::text[]);

-- name: UpdateRegistryArtifact :one
UPDATE registry_artifact
SET description = $2,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteRegistryArtifact :exec
DELETE FROM registry_artifact WHERE id = $1;
