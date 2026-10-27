-- name: UpsertArtifactRetentionPolicy :one
INSERT INTO registry_retention_policy (artifact_id, keep_last, max_age_days)
VALUES ($1, $2, $3)
ON CONFLICT (artifact_id) DO UPDATE
    SET keep_last    = EXCLUDED.keep_last,
        max_age_days = EXCLUDED.max_age_days,
        updated_at   = NOW()
RETURNING *;

-- name: UpsertTypeRetentionPolicy :one
INSERT INTO registry_retention_policy (type_id, keep_last, max_age_days)
VALUES ($1, $2, $3)
ON CONFLICT (type_id) DO UPDATE
    SET keep_last    = EXCLUDED.keep_last,
        max_age_days = EXCLUDED.max_age_days,
        updated_at   = NOW()
RETURNING *;

-- name: GetArtifactRetentionPolicy :one
SELECT * FROM registry_retention_policy WHERE artifact_id = $1;

-- name: GetTypeRetentionPolicy :one
SELECT * FROM registry_retention_policy WHERE type_id = $1;

-- name: DeleteArtifactRetentionPolicy :execrows
DELETE FROM registry_retention_policy WHERE artifact_id = $1;

-- name: DeleteTypeRetentionPolicy :execrows
DELETE FROM registry_retention_policy WHERE type_id = $1;

-- name: ListTypeRetentionPoliciesForArtifact :many
-- Policies of the types assigned to an artifact — merged into its effective rule
-- when the artifact has no policy of its own.
SELECT rp.* FROM registry_retention_policy rp
JOIN registry_artifact_type_map m ON m.type_id = rp.type_id
WHERE m.artifact_id = $1
ORDER BY rp.type_id;

-- name: ListRetentionPolicies :many
SELECT rp.*, ra.full_name AS artifact_name, rt.name AS type_name
FROM registry_retention_policy rp
LEFT JOIN registry_artifact ra ON ra.id = rp.artifact_id
LEFT JOIN registry_artifact_type rt ON rt.id = rp.type_id
ORDER BY rt.name NULLS LAST, ra.full_name;
