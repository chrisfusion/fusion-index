CREATE SEQUENCE job_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE job (
    id                    BIGINT       PRIMARY KEY DEFAULT nextval('job_seq'),
    name                  VARCHAR(255) NOT NULL,
    description           TEXT,
    template_version_id   BIGINT       NOT NULL REFERENCES job_template_version(id),
    latest_version_number INT          NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_job_name UNIQUE (name)
);

CREATE INDEX idx_job_template_version_id ON job (template_version_id);
