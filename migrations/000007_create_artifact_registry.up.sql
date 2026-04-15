CREATE SEQUENCE registry_artifact_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact (
    id          BIGINT       PRIMARY KEY DEFAULT nextval('registry_artifact_seq'),
    full_name   VARCHAR(500) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_registry_artifact_full_name UNIQUE (full_name)
);

-- ---------------------------------------------------------------------------

CREATE SEQUENCE registry_artifact_version_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact_version (
    id          BIGINT      PRIMARY KEY DEFAULT nextval('registry_artifact_version_seq'),
    artifact_id BIGINT      NOT NULL REFERENCES registry_artifact(id) ON DELETE CASCADE,
    major       INT         NOT NULL,
    minor       INT         NOT NULL,
    patch       INT         NOT NULL,
    config      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifact_version UNIQUE (artifact_id, major, minor, patch)
);

CREATE INDEX idx_artifact_version_artifact_id ON registry_artifact_version (artifact_id);

-- ---------------------------------------------------------------------------

CREATE SEQUENCE registry_artifact_file_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact_file (
    id              BIGINT       PRIMARY KEY DEFAULT nextval('registry_artifact_file_seq'),
    version_id      BIGINT       NOT NULL REFERENCES registry_artifact_version(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    content_type    VARCHAR(255),
    size_bytes      BIGINT,
    storage_backend VARCHAR(20)  NOT NULL,
    storage_path    TEXT         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifact_file_version_id ON registry_artifact_file (version_id);
CREATE UNIQUE INDEX uq_artifact_file_version_name ON registry_artifact_file (version_id, name);

-- ---------------------------------------------------------------------------

CREATE SEQUENCE registry_artifact_tag_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact_tag (
    id          BIGINT       PRIMARY KEY DEFAULT nextval('registry_artifact_tag_seq'),
    artifact_id BIGINT       NOT NULL REFERENCES registry_artifact(id) ON DELETE CASCADE,
    tag         VARCHAR(255) NOT NULL,
    version_id  BIGINT       NOT NULL REFERENCES registry_artifact_version(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifact_tag UNIQUE (artifact_id, tag)
);

CREATE INDEX idx_artifact_tag_artifact_id ON registry_artifact_tag (artifact_id);
CREATE INDEX idx_artifact_tag_version_id  ON registry_artifact_tag (version_id);
