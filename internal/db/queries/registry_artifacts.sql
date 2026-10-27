-- name: CreateRegistryArtifact :one
INSERT INTO registry_artifact (full_name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetRegistryArtifact :one
SELECT * FROM registry_artifact WHERE id = $1;

-- name: GetRegistryArtifactByName :one
SELECT * FROM registry_artifact WHERE full_name = $1;

-- name: CountRegistryArtifacts :one
SELECT COUNT(*) FROM registry_artifact;

-- name: ListRegistryArtifactsFiltered :many
-- Artifact list for GET /api/v1/artifacts: every NULL filter is ignored, the rest
-- combine with AND. Keep the WHERE clause identical to CountRegistryArtifactsFiltered.
-- version_count / largest_version_bytes / total_bytes are trigger-maintained (migration 000009).
SELECT ra.* FROM registry_artifact ra
WHERE (sqlc.narg(name_prefix)::text IS NULL OR ra.full_name ILIKE sqlc.narg(name_prefix)::text || '%')
  AND (sqlc.narg(tag)::text IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = ra.id AND rat.tag = sqlc.narg(tag)::text))
  AND (sqlc.narg(types)::text[] IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_type_map ratm
                  JOIN registry_artifact_type rty ON rty.id = ratm.type_id
                  WHERE ratm.artifact_id = ra.id AND rty.name = ANY(sqlc.narg(types)::text[])))
  AND (sqlc.narg(older_than)::timestamptz IS NULL OR ra.created_at < sqlc.narg(older_than)::timestamptz)
  AND (sqlc.narg(min_versions)::int IS NULL OR ra.version_count >= sqlc.narg(min_versions)::int)
  AND (sqlc.narg(max_versions)::int IS NULL OR ra.version_count <= sqlc.narg(max_versions)::int)
  AND (sqlc.narg(has_files)::bool IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_version rav
                  JOIN registry_artifact_file raf ON raf.version_id = rav.id
                  WHERE rav.artifact_id = ra.id) = sqlc.narg(has_files)::bool)
  AND (sqlc.narg(has_incomplete_files)::bool IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_version rav
                  JOIN registry_artifact_file raf ON raf.version_id = rav.id
                  WHERE rav.artifact_id = ra.id AND raf.status IN ('PENDING', 'ERROR')) = sqlc.narg(has_incomplete_files)::bool)
  AND (sqlc.narg(missing_tag)::text IS NULL
       OR NOT EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = ra.id AND rat.tag = sqlc.narg(missing_tag)::text))
  AND (sqlc.narg(min_total_bytes)::bigint IS NULL OR ra.total_bytes >= sqlc.narg(min_total_bytes)::bigint)
  AND (sqlc.narg(has_retention)::bool IS NULL
       OR (EXISTS (SELECT 1 FROM registry_retention_policy rp WHERE rp.artifact_id = ra.id)
           OR EXISTS (SELECT 1 FROM registry_artifact_type_map ratm
                      JOIN registry_retention_policy rp ON rp.type_id = ratm.type_id
                      WHERE ratm.artifact_id = ra.id)) = sqlc.narg(has_retention)::bool)
ORDER BY
  CASE WHEN sqlc.arg(sort)::text = 'name'           AND sqlc.arg(sort_desc)::bool     THEN ra.full_name END DESC,
  CASE WHEN sqlc.arg(sort)::text = 'name'           AND NOT sqlc.arg(sort_desc)::bool THEN ra.full_name END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'versionCount'   AND sqlc.arg(sort_desc)::bool     THEN ra.version_count END DESC,
  CASE WHEN sqlc.arg(sort)::text = 'versionCount'   AND NOT sqlc.arg(sort_desc)::bool THEN ra.version_count END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'largestVersion' AND sqlc.arg(sort_desc)::bool     THEN ra.largest_version_bytes END DESC,
  CASE WHEN sqlc.arg(sort)::text = 'largestVersion' AND NOT sqlc.arg(sort_desc)::bool THEN ra.largest_version_bytes END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'totalSize'      AND sqlc.arg(sort_desc)::bool     THEN ra.total_bytes END DESC,
  CASE WHEN sqlc.arg(sort)::text = 'totalSize'      AND NOT sqlc.arg(sort_desc)::bool THEN ra.total_bytes END ASC,
  CASE WHEN sqlc.arg(sort_desc)::bool     THEN ra.created_at END DESC,
  CASE WHEN NOT sqlc.arg(sort_desc)::bool THEN ra.created_at END ASC,
  ra.id
LIMIT sqlc.arg(row_limit)::int OFFSET sqlc.arg(row_offset)::int;

-- name: CountRegistryArtifactsFiltered :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE (sqlc.narg(name_prefix)::text IS NULL OR ra.full_name ILIKE sqlc.narg(name_prefix)::text || '%')
  AND (sqlc.narg(tag)::text IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = ra.id AND rat.tag = sqlc.narg(tag)::text))
  AND (sqlc.narg(types)::text[] IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_type_map ratm
                  JOIN registry_artifact_type rty ON rty.id = ratm.type_id
                  WHERE ratm.artifact_id = ra.id AND rty.name = ANY(sqlc.narg(types)::text[])))
  AND (sqlc.narg(older_than)::timestamptz IS NULL OR ra.created_at < sqlc.narg(older_than)::timestamptz)
  AND (sqlc.narg(min_versions)::int IS NULL OR ra.version_count >= sqlc.narg(min_versions)::int)
  AND (sqlc.narg(max_versions)::int IS NULL OR ra.version_count <= sqlc.narg(max_versions)::int)
  AND (sqlc.narg(has_files)::bool IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_version rav
                  JOIN registry_artifact_file raf ON raf.version_id = rav.id
                  WHERE rav.artifact_id = ra.id) = sqlc.narg(has_files)::bool)
  AND (sqlc.narg(has_incomplete_files)::bool IS NULL
       OR EXISTS (SELECT 1 FROM registry_artifact_version rav
                  JOIN registry_artifact_file raf ON raf.version_id = rav.id
                  WHERE rav.artifact_id = ra.id AND raf.status IN ('PENDING', 'ERROR')) = sqlc.narg(has_incomplete_files)::bool)
  AND (sqlc.narg(missing_tag)::text IS NULL
       OR NOT EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = ra.id AND rat.tag = sqlc.narg(missing_tag)::text))
  AND (sqlc.narg(min_total_bytes)::bigint IS NULL OR ra.total_bytes >= sqlc.narg(min_total_bytes)::bigint)
  AND (sqlc.narg(has_retention)::bool IS NULL
       OR (EXISTS (SELECT 1 FROM registry_retention_policy rp WHERE rp.artifact_id = ra.id)
           OR EXISTS (SELECT 1 FROM registry_artifact_type_map ratm
                      JOIN registry_retention_policy rp ON rp.type_id = ratm.type_id
                      WHERE ratm.artifact_id = ra.id)) = sqlc.narg(has_retention)::bool);

-- name: UpdateRegistryArtifact :one
UPDATE registry_artifact
SET description = $2,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteRegistryArtifact :exec
DELETE FROM registry_artifact WHERE id = $1;

-- name: ListEmptyArtifacts :many
SELECT ra.* FROM registry_artifact ra
WHERE ra.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = ra.id)
ORDER BY ra.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountEmptyArtifacts :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE ra.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = ra.id);

-- name: DeleteEmptyArtifacts :execrows
DELETE FROM registry_artifact
WHERE registry_artifact.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = registry_artifact.id)
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = registry_artifact.id AND rat.tag = $2);

-- name: ListArtifactsWithoutFiles :many
SELECT ra.* FROM registry_artifact ra
WHERE ra.created_at < $1
  AND EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = ra.id)
  AND NOT EXISTS (
    SELECT 1 FROM registry_artifact_version rav2
    JOIN registry_artifact_file raf ON raf.version_id = rav2.id
    WHERE rav2.artifact_id = ra.id
  )
ORDER BY ra.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountArtifactsWithoutFiles :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE ra.created_at < $1
  AND EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = ra.id)
  AND NOT EXISTS (
    SELECT 1 FROM registry_artifact_version rav2
    JOIN registry_artifact_file raf ON raf.version_id = rav2.id
    WHERE rav2.artifact_id = ra.id
  );

-- name: DeleteArtifactsWithoutFiles :execrows
DELETE FROM registry_artifact
WHERE registry_artifact.created_at < $1
  AND EXISTS (SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = registry_artifact.id)
  AND NOT EXISTS (
    SELECT 1 FROM registry_artifact_version rav2
    JOIN registry_artifact_file raf ON raf.version_id = rav2.id
    WHERE rav2.artifact_id = registry_artifact.id
  )
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = registry_artifact.id AND rat.tag = $2);
