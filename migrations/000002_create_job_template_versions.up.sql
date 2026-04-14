CREATE SEQUENCE job_template_version_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE job_template_version (
    id                 BIGINT       PRIMARY KEY DEFAULT nextval('job_template_version_seq'),
    template_id        BIGINT       NOT NULL REFERENCES job_template(id) ON DELETE CASCADE,
    version_number     INT          NOT NULL,
    docker_image       VARCHAR(500) NOT NULL,
    default_run_config TEXT,
    changelog          TEXT,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_template_version UNIQUE (template_id, version_number)
);

CREATE INDEX idx_jtv_template_id ON job_template_version (template_id);
