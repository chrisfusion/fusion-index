-- Retention rules per artifact or per artifact type. fusion-index only stores and
-- reports them (see GET /api/v1/artifacts/{id}/retention for the effective rule);
-- applying them is left to an external runner that calls the bulk version delete.
--
-- A rule deletes versions that are BOTH outside the newest keep_last (by semver)
-- AND older than max_age_days — the same AND semantics as the bulk delete filters.
-- Either field may be NULL (= no constraint from that side), not both.

CREATE SEQUENCE registry_retention_policy_seq START WITH 1 INCREMENT BY 50;

CREATE TABLE registry_retention_policy (
    id           BIGINT      PRIMARY KEY DEFAULT nextval('registry_retention_policy_seq'),
    artifact_id  BIGINT      UNIQUE REFERENCES registry_artifact(id) ON DELETE CASCADE,
    type_id      BIGINT      UNIQUE REFERENCES registry_artifact_type(id) ON DELETE CASCADE,
    keep_last    INT         CHECK (keep_last >= 1),
    max_age_days INT         CHECK (max_age_days >= 1),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_retention_target CHECK ((artifact_id IS NULL) <> (type_id IS NULL)),
    CONSTRAINT chk_retention_rule   CHECK (keep_last IS NOT NULL OR max_age_days IS NOT NULL)
);
