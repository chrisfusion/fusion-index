-- name: CreateTemplate :one
INSERT INTO job_template (name, description, docker_image, latest_version_number)
VALUES ($1, $2, $3, 0)
RETURNING *;

-- name: GetTemplateByID :one
SELECT * FROM job_template WHERE id = $1;

-- name: GetTemplateByName :one
SELECT * FROM job_template WHERE name = $1;

-- name: ListTemplates :many
SELECT * FROM job_template
ORDER BY id ASC
LIMIT $1 OFFSET $2;

-- name: CountTemplates :one
SELECT COUNT(*) FROM job_template;

-- name: UpdateTemplate :one
UPDATE job_template
SET description = $2,
    docker_image = $3,
    updated_at   = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTemplate :exec
DELETE FROM job_template WHERE id = $1;

-- name: IncrementTemplateVersion :one
UPDATE job_template
SET latest_version_number = latest_version_number + 1,
    updated_at             = NOW()
WHERE id = $1
RETURNING latest_version_number;

-- name: CountJobsForTemplate :one
SELECT COUNT(*)
FROM job j
JOIN job_template_version jtv ON j.template_version_id = jtv.id
WHERE jtv.template_id = $1;

-- name: CreateTemplateVersion :one
INSERT INTO job_template_version (template_id, version_number, docker_image, default_run_config, changelog)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTemplateVersionByID :one
SELECT * FROM job_template_version WHERE id = $1;

-- name: GetTemplateVersion :one
SELECT * FROM job_template_version
WHERE template_id = $1 AND version_number = $2;

-- name: ListTemplateVersions :many
SELECT * FROM job_template_version
WHERE template_id = $1
ORDER BY version_number ASC;
