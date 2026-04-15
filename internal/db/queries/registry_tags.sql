-- name: UpsertArtifactTag :one
INSERT INTO registry_artifact_tag (artifact_id, tag, version_id)
VALUES ($1, $2, $3)
ON CONFLICT (artifact_id, tag) DO UPDATE
    SET version_id = EXCLUDED.version_id,
        updated_at = NOW()
RETURNING *;

-- name: GetArtifactTag :one
SELECT * FROM registry_artifact_tag
WHERE artifact_id = $1 AND tag = $2;

-- name: ListArtifactTags :many
SELECT * FROM registry_artifact_tag
WHERE artifact_id = $1
ORDER BY tag ASC;

-- name: ListArtifactTagsByVersionID :many
SELECT * FROM registry_artifact_tag
WHERE version_id = $1
ORDER BY tag ASC;

-- name: DeleteArtifactTag :exec
DELETE FROM registry_artifact_tag
WHERE artifact_id = $1 AND tag = $2;
