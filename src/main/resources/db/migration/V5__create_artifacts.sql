CREATE SEQUENCE artifact_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE artifact (
    id              BIGINT       PRIMARY KEY DEFAULT nextval('artifact_seq'),
    job_version_id  BIGINT       NOT NULL REFERENCES job_version(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    content_type    VARCHAR(255),
    size_bytes      BIGINT,
    storage_backend VARCHAR(20)  NOT NULL,
    storage_path    TEXT         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifact_job_version_id ON artifact (job_version_id);
