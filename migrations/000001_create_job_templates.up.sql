CREATE SEQUENCE job_template_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE job_template (
    id                    BIGINT       PRIMARY KEY DEFAULT nextval('job_template_seq'),
    name                  VARCHAR(255) NOT NULL,
    description           TEXT,
    docker_image          VARCHAR(500) NOT NULL,
    latest_version_number INT          NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_job_template_name UNIQUE (name)
);
