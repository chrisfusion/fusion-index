CREATE SEQUENCE registry_artifact_type_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact_type (
    id          BIGINT       PRIMARY KEY DEFAULT nextval('registry_artifact_type_seq'),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_registry_artifact_type_name UNIQUE (name)
);

-- ---------------------------------------------------------------------------

CREATE SEQUENCE registry_artifact_type_map_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_artifact_type_map (
    id          BIGINT      PRIMARY KEY DEFAULT nextval('registry_artifact_type_map_seq'),
    artifact_id BIGINT      NOT NULL REFERENCES registry_artifact(id) ON DELETE CASCADE,
    type_id     BIGINT      NOT NULL REFERENCES registry_artifact_type(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_artifact_type_map UNIQUE (artifact_id, type_id)
);

CREATE INDEX idx_artifact_type_map_artifact_id ON registry_artifact_type_map (artifact_id);
CREATE INDEX idx_artifact_type_map_type_id     ON registry_artifact_type_map (type_id);
