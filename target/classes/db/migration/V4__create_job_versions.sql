CREATE SEQUENCE job_version_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE job_version (
    id                  BIGINT       PRIMARY KEY DEFAULT nextval('job_version_seq'),
    job_id              BIGINT       NOT NULL REFERENCES job(id) ON DELETE CASCADE,
    version_number      INT          NOT NULL,
    docker_image        VARCHAR(500) NOT NULL,
    git_url             VARCHAR(1000) NOT NULL,
    git_ref             VARCHAR(255) NOT NULL,
    git_subpath         VARCHAR(500),
    run_config          TEXT,
    template_version_id BIGINT       NOT NULL REFERENCES job_template_version(id),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_job_version UNIQUE (job_id, version_number)
);

CREATE INDEX idx_jv_job_id ON job_version (job_id);
