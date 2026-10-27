-- Denormalised size/count statistics for sorting and filtering the artifact list
-- without aggregating over every file row per request (live aggregation measured
-- at ~380 ms per page for 1M files; the stored columns keep it a column sort).
--
--   registry_artifact_version.total_bytes        sum(size_bytes) of its AVAILABLE files
--   registry_artifact.version_count              number of versions
--   registry_artifact.largest_version_bytes      max(total_bytes) over its versions
--   registry_artifact.total_bytes                sum(total_bytes) over its versions
--
-- Maintained by triggers so every write path (uploads, deletes, cascades, admin
-- cleanups, restore-db) keeps them consistent.

ALTER TABLE registry_artifact_version
    ADD COLUMN total_bytes BIGINT NOT NULL DEFAULT 0;

ALTER TABLE registry_artifact
    ADD COLUMN version_count         INT    NOT NULL DEFAULT 0,
    ADD COLUMN largest_version_bytes BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN total_bytes           BIGINT NOT NULL DEFAULT 0;

-- Backfill before the triggers exist, so it doesn't fire them row by row.
-- The ALTER TABLEs above hold ACCESS EXCLUSIVE locks until commit, so no write
-- can slip in between backfill and trigger creation.
UPDATE registry_artifact_version v
SET total_bytes = s.bytes
FROM (
    SELECT version_id, SUM(size_bytes) AS bytes
    FROM registry_artifact_file
    WHERE status = 'AVAILABLE' AND size_bytes IS NOT NULL
    GROUP BY version_id
) s
WHERE s.version_id = v.id;

UPDATE registry_artifact a
SET version_count         = s.cnt,
    largest_version_bytes = s.largest,
    total_bytes           = s.total
FROM (
    SELECT artifact_id, COUNT(*) AS cnt, COALESCE(MAX(total_bytes), 0) AS largest,
           COALESCE(SUM(total_bytes), 0) AS total
    FROM registry_artifact_version
    GROUP BY artifact_id
) s
WHERE s.artifact_id = a.id;

-- hasIncompleteFiles filters: PENDING/ERROR rows are a tiny fraction of all files,
-- a partial index turns their EXISTS probes from a status scan into an index lookup.
CREATE INDEX idx_artifact_file_incomplete ON registry_artifact_file (version_id)
    WHERE status IN ('PENDING', 'ERROR');

-- File → version: apply the byte delta of AVAILABLE files.
CREATE FUNCTION registry_file_sync_version_bytes() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    delta BIGINT := 0;
    vid   BIGINT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        vid := OLD.version_id;
        IF OLD.status = 'AVAILABLE' THEN
            delta := delta - COALESCE(OLD.size_bytes, 0);
        END IF;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        vid := NEW.version_id;
        IF NEW.status = 'AVAILABLE' THEN
            delta := delta + COALESCE(NEW.size_bytes, 0);
        END IF;
    END IF;
    IF delta <> 0 THEN
        UPDATE registry_artifact_version SET total_bytes = total_bytes + delta WHERE id = vid;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER registry_file_sync_version_bytes
AFTER INSERT OR UPDATE OF status, size_bytes OR DELETE ON registry_artifact_file
FOR EACH ROW EXECUTE FUNCTION registry_file_sync_version_bytes();

-- Version → artifact: recompute count and max. The artifact row is locked first in
-- its own statement, so the recompute runs on a snapshot taken after every earlier
-- writer of this artifact committed (READ COMMITTED takes one per statement) —
-- concurrent uploads/deletes can't leave stale values behind.
CREATE FUNCTION registry_version_sync_artifact_stats() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    aid BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        aid := OLD.artifact_id;
    ELSE
        aid := NEW.artifact_id;
    END IF;
    PERFORM 1 FROM registry_artifact WHERE id = aid FOR UPDATE;
    UPDATE registry_artifact a
    SET version_count         = s.cnt,
        largest_version_bytes = s.largest,
        total_bytes           = s.total
    FROM (
        SELECT COUNT(*) AS cnt, COALESCE(MAX(total_bytes), 0) AS largest,
               COALESCE(SUM(total_bytes), 0) AS total
        FROM registry_artifact_version
        WHERE artifact_id = aid
    ) s
    WHERE a.id = aid;
    RETURN NULL;
END;
$$;

CREATE TRIGGER registry_version_sync_artifact_stats
AFTER INSERT OR UPDATE OF total_bytes OR DELETE ON registry_artifact_version
FOR EACH ROW EXECUTE FUNCTION registry_version_sync_artifact_stats();
