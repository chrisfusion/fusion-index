-- name: AssignArtifactType :exec
INSERT INTO registry_artifact_type_map (artifact_id, type_id)
VALUES ($1, $2)
ON CONFLICT (artifact_id, type_id) DO NOTHING;

-- name: RemoveArtifactType :exec
DELETE FROM registry_artifact_type_map
WHERE artifact_id = $1 AND type_id = $2;

-- name: GetArtifactTypeAssignment :one
SELECT * FROM registry_artifact_type_map
WHERE artifact_id = $1 AND type_id = $2;

-- name: ListArtifactTypesByArtifactID :many
SELECT rat.* FROM registry_artifact_type rat
JOIN registry_artifact_type_map ratm ON ratm.type_id = rat.id
WHERE ratm.artifact_id = $1
ORDER BY rat.name ASC;

-- name: ListArtifactTypesByArtifactIDs :many
SELECT ratm.artifact_id, rat.id, rat.name, rat.description, rat.created_at, rat.updated_at
FROM registry_artifact_type rat
JOIN registry_artifact_type_map ratm ON ratm.type_id = rat.id
WHERE ratm.artifact_id = ANY($1::bigint[])
ORDER BY ratm.artifact_id, rat.name ASC;
