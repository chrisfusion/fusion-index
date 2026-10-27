-- name: CountRegistryVersions :one
SELECT COUNT(*) FROM registry_artifact_version;

-- name: CountRegistryTags :one
SELECT COUNT(*) FROM registry_artifact_tag;

-- name: CountRegistryFilesByStatus :many
SELECT status, COUNT(*) AS count
FROM registry_artifact_file
GROUP BY status;

-- name: SumRegistryStorageBytes :one
SELECT COALESCE(SUM(size_bytes), 0)::bigint AS total_storage_bytes
FROM registry_artifact_file
WHERE status = 'AVAILABLE';

-- name: CountArtifactsWithoutTags :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE NOT EXISTS (
    SELECT 1 FROM registry_artifact_tag rat WHERE rat.artifact_id = ra.id
);

-- name: CountArtifactsWithoutVersions :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE NOT EXISTS (
    SELECT 1 FROM registry_artifact_version rav WHERE rav.artifact_id = ra.id
);

-- name: CountArtifactsByType :many
SELECT rat.name AS type_name, COUNT(ratm.artifact_id) AS artifact_count
FROM registry_artifact_type rat
LEFT JOIN registry_artifact_type_map ratm ON ratm.type_id = rat.id
GROUP BY rat.name
ORDER BY rat.name;

-- name: CountVersionsWithoutAnyFiles :one
SELECT COUNT(*) FROM registry_artifact_version v
WHERE NOT EXISTS (SELECT 1 FROM registry_artifact_file f WHERE f.version_id = v.id);

-- name: CountArtifactsWithVersionsButNoFiles :one
SELECT COUNT(*) FROM registry_artifact ra
WHERE ra.version_count > 0
  AND NOT EXISTS (SELECT 1 FROM registry_artifact_version v
                  JOIN registry_artifact_file f ON f.version_id = v.id
                  WHERE v.artifact_id = ra.id);

-- name: CountBrokenTags :one
SELECT COUNT(*) FROM registry_artifact_tag rat
WHERE NOT EXISTS (SELECT 1 FROM registry_artifact_file f
                  WHERE f.version_id = rat.version_id AND f.status = 'AVAILABLE');

-- name: CountUnusedTypes :one
SELECT COUNT(*) FROM registry_artifact_type t
WHERE NOT EXISTS (SELECT 1 FROM registry_artifact_type_map m WHERE m.type_id = t.id);

-- name: CountRetentionPolicies :one
SELECT COUNT(*) FROM registry_retention_policy;

-- name: SumStorageBytesByBackend :many
SELECT storage_backend, COALESCE(SUM(size_bytes), 0)::bigint AS total_bytes
FROM registry_artifact_file
WHERE status = 'AVAILABLE'
GROUP BY storage_backend
ORDER BY storage_backend;
