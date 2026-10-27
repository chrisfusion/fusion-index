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

-- name: ListVersionsFiltered :many
-- Shared by the version lists (GET /artifacts/{id}/versions, GET /versions) and the
-- bulk deletes (internal/api/handlers/version_bulk.go), so "show" and "delete" always
-- select the same versions. Every NULL filter is ignored, the rest combine with AND.
-- keep_last ranks all versions of an artifact by semver, tagged ones included.
-- protected = a tag points at the version or the artifact carries the protection
-- tag; bulk deletes never delete those, deletable_only hides them from lists.
-- row_limit NULL = no limit; total_count is the match count before LIMIT/OFFSET.
WITH ranked AS (
  SELECT v.*,
         row_number() OVER (PARTITION BY v.artifact_id ORDER BY v.major DESC, v.minor DESC, v.patch DESC) AS rn
  FROM registry_artifact_version v
  WHERE sqlc.narg(artifact_id)::bigint IS NULL OR v.artifact_id = sqlc.narg(artifact_id)::bigint
), matched AS (
  SELECT r.*,
         (EXISTS (SELECT 1 FROM registry_artifact_tag t WHERE t.version_id = r.id)
          OR EXISTS (SELECT 1 FROM registry_artifact_tag t WHERE t.artifact_id = r.artifact_id AND t.tag = sqlc.arg(protected_tag)::text)
         ) AS protected
  FROM ranked r
  WHERE (sqlc.narg(keep_last)::int IS NULL OR r.rn > sqlc.narg(keep_last)::int)
    AND (sqlc.narg(older_than)::timestamptz IS NULL OR r.created_at < sqlc.narg(older_than)::timestamptz)
    AND (sqlc.narg(from_major)::int IS NULL
         OR (r.major, r.minor, r.patch) >= (sqlc.narg(from_major)::int, sqlc.narg(from_minor)::int, sqlc.narg(from_patch)::int))
    AND (sqlc.narg(below_major)::int IS NULL
         OR (r.major, r.minor, r.patch) < (sqlc.narg(below_major)::int, sqlc.narg(below_minor)::int, sqlc.narg(below_patch)::int))
    AND (sqlc.narg(versions)::text[] IS NULL
         OR (r.major::text || '.' || r.minor::text || '.' || r.patch::text) = ANY(sqlc.narg(versions)::text[]))
    AND (sqlc.narg(has_files)::bool IS NULL
         OR EXISTS (SELECT 1 FROM registry_artifact_file f WHERE f.version_id = r.id) = sqlc.narg(has_files)::bool)
    AND (sqlc.narg(has_incomplete_files)::bool IS NULL
         OR EXISTS (SELECT 1 FROM registry_artifact_file f WHERE f.version_id = r.id AND f.status IN ('PENDING', 'ERROR')) = sqlc.narg(has_incomplete_files)::bool)
    AND (sqlc.narg(min_bytes)::bigint IS NULL OR r.total_bytes >= sqlc.narg(min_bytes)::bigint)
    AND (sqlc.narg(max_bytes)::bigint IS NULL OR r.total_bytes <= sqlc.narg(max_bytes)::bigint)
)
SELECT m.id, m.artifact_id, m.major, m.minor, m.patch, m.config, m.created_at, m.total_bytes,
       m.protected::bool AS protected,
       COUNT(*) OVER () AS total_count
FROM matched m
WHERE NOT (sqlc.arg(deletable_only)::bool AND m.protected)
ORDER BY m.artifact_id, m.major DESC, m.minor DESC, m.patch DESC
LIMIT sqlc.narg(row_limit)::int OFFSET sqlc.arg(row_offset)::int;

-- name: DeleteUnprotectedVersionsByIDs :many
-- Re-checks protection inside the delete so a tag assigned after the candidate
-- list was built still saves its version.
DELETE FROM registry_artifact_version v
WHERE v.id = ANY(sqlc.arg(ids)::bigint[])
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_tag t WHERE t.version_id = v.id)
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_tag t WHERE t.artifact_id = v.artifact_id AND t.tag = sqlc.arg(protected_tag)::text)
RETURNING v.id;
