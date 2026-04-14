-- name: CreateJob :one
INSERT INTO job (name, description, template_version_id, latest_version_number)
VALUES ($1, $2, $3, 0)
RETURNING *;

-- name: GetJobByID :one
SELECT * FROM job WHERE id = $1;

-- name: GetJobByName :one
SELECT * FROM job WHERE name = $1;

-- name: ListJobs :many
SELECT * FROM job
ORDER BY id ASC
LIMIT $1 OFFSET $2;

-- name: CountJobs :one
SELECT COUNT(*) FROM job;

-- name: UpdateJob :one
UPDATE job
SET description = $2,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteJob :exec
DELETE FROM job WHERE id = $1;

-- name: IncrementJobVersion :one
UPDATE job
SET latest_version_number = latest_version_number + 1,
    updated_at             = NOW()
WHERE id = $1
RETURNING latest_version_number;

-- name: CreateJobVersion :one
INSERT INTO job_version (job_id, version_number, docker_image, git_url, git_ref, git_subpath, run_config, template_version_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetJobVersionByJobAndNumber :one
SELECT * FROM job_version
WHERE job_id = $1 AND version_number = $2;

-- name: GetJobVersionByID :one
SELECT * FROM job_version WHERE id = $1;

-- name: ListJobVersions :many
SELECT * FROM job_version
WHERE job_id = $1
ORDER BY version_number ASC;

-- name: CountArtifactsForJobVersion :one
SELECT COUNT(*) FROM artifact WHERE job_version_id = $1;
