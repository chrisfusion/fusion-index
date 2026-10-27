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

-- name: ListTagsByVersionIDs :many
SELECT * FROM registry_artifact_tag
WHERE version_id = ANY($1::bigint[])
ORDER BY version_id ASC, tag ASC;

-- name: ListTagsFiltered :many
-- Registry-wide tag list (GET /api/v1/tags). broken = the tagged version has no
-- AVAILABLE file, i.e. resolving the tag gives nothing downloadable.
WITH t AS (
  SELECT rat.id, rat.artifact_id, ra.full_name, rat.tag, rat.version_id,
         v.major, v.minor, v.patch, rat.updated_at,
         NOT EXISTS (SELECT 1 FROM registry_artifact_file f
                     WHERE f.version_id = v.id AND f.status = 'AVAILABLE') AS broken
  FROM registry_artifact_tag rat
  JOIN registry_artifact ra ON ra.id = rat.artifact_id
  JOIN registry_artifact_version v ON v.id = rat.version_id
  WHERE sqlc.narg(tag)::text IS NULL OR rat.tag = sqlc.narg(tag)::text
)
SELECT t.id, t.artifact_id, t.full_name, t.tag, t.version_id, t.major, t.minor, t.patch,
       t.updated_at, t.broken::bool AS broken, COUNT(*) OVER () AS total_count
FROM t
WHERE sqlc.narg(broken)::bool IS NULL OR t.broken = sqlc.narg(broken)::bool
ORDER BY t.full_name, t.tag
LIMIT sqlc.arg(row_limit)::int OFFSET sqlc.arg(row_offset)::int;
