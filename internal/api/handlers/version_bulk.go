package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	"fusion-platform/fusion-index/internal/api/middleware"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
	"fusion-platform/fusion-index/internal/storage"
)

// filteredVersions serves every endpoint that selects versions by the shared filters
// (ListVersionsFiltered): the version lists, which show them, and the bulk deletes,
// which remove them. Using one query for both keeps "show" and "delete" in sync.
type filteredVersions struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	storage      storage.Storage
	protectedTag string
}

// parseVersionFilter reads keepLast, olderThan, from, below, hasFiles,
// hasIncompleteFiles, minBytes, maxBytes and (per artifact only) the repeatable
// version parameter.
// anySet reports whether at least one filter was given.
func parseVersionFilter(c *gin.Context, perArtifact bool) (p db.ListVersionsFilteredParams, anySet, ok bool) {
	if p.KeepLast, ok = parseOptionalInt32(c, "keepLast", 1); !ok {
		return p, false, false
	}
	if p.OlderThan, ok = parseOptionalOlderThan(c); !ok {
		return p, false, false
	}
	if raw := c.Query("from"); raw != "" {
		sv, err := semver.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from: " + err.Error()})
			return p, false, false
		}
		p.FromMajor, p.FromMinor, p.FromPatch = &sv.Major, &sv.Minor, &sv.Patch
	}
	if raw := c.Query("below"); raw != "" {
		sv, err := semver.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "below: " + err.Error()})
			return p, false, false
		}
		p.BelowMajor, p.BelowMinor, p.BelowPatch = &sv.Major, &sv.Minor, &sv.Patch
	}
	if raws := c.QueryArray("version"); len(raws) > 0 {
		if !perArtifact {
			c.JSON(http.StatusBadRequest, gin.H{"error": "version is only supported per artifact"})
			return p, false, false
		}
		for _, raw := range raws {
			sv, err := semver.Parse(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "version: " + err.Error()})
				return p, false, false
			}
			p.Versions = append(p.Versions, sv.String()) // normalised, matches the SQL rendering
		}
	}
	if p.HasFiles, ok = parseOptionalBool(c, "hasFiles"); !ok {
		return p, false, false
	}
	if p.HasIncompleteFiles, ok = parseOptionalBool(c, "hasIncompleteFiles"); !ok {
		return p, false, false
	}
	if p.MinBytes, ok = parseOptionalInt64(c, "minBytes"); !ok {
		return p, false, false
	}
	if p.MaxBytes, ok = parseOptionalInt64(c, "maxBytes"); !ok {
		return p, false, false
	}

	anySet = p.KeepLast != nil || p.OlderThan.Valid || p.FromMajor != nil || p.BelowMajor != nil ||
		p.Versions != nil || p.HasFiles != nil || p.HasIncompleteFiles != nil ||
		p.MinBytes != nil || p.MaxBytes != nil
	return p, anySet, true
}

func versionFromFilteredRow(r db.ListVersionsFilteredRow) db.RegistryArtifactVersion {
	return db.RegistryArtifactVersion{
		ID: r.ID, ArtifactID: r.ArtifactID,
		Major: r.Major, Minor: r.Minor, Patch: r.Patch,
		Config: r.Config, CreatedAt: r.CreatedAt, TotalBytes: r.TotalBytes,
	}
}

// query parses the filters plus deletable=true (hide versions a bulk delete would
// skip) and runs ListVersionsFiltered; artifactID nil means every artifact.
func (f filteredVersions) query(c *gin.Context, artifactID *int64, limit *int32, offset int32) ([]db.ListVersionsFilteredRow, bool) {
	params, _, ok := parseVersionFilter(c, artifactID != nil)
	if !ok {
		return nil, false
	}
	deletable, ok := parseOptionalBool(c, "deletable")
	if !ok {
		return nil, false
	}
	params.DeletableOnly = deletable != nil && *deletable
	params.ArtifactID = artifactID
	params.ProtectedTag = f.protectedTag
	params.RowLimit = limit
	params.RowOffset = offset
	rows, err := f.queries.ListVersionsFiltered(c, params)
	if err != nil {
		internalError(c, err)
		return nil, false
	}
	return rows, true
}

// bulkDelete deletes the matching versions with their files; artifactID nil means
// every artifact. At least one filter is required so a bare call never wipes every
// version. With dryRun=true it only reports what would be deleted. DB rows go first
// (in a transaction that re-checks protection), storage objects afterwards and
// best-effort — the DB never points at a deleted object, at worst storage keeps an orphan.
func (f filteredVersions) bulkDelete(c *gin.Context, artifactID *int64) {
	params, anySet, ok := parseVersionFilter(c, artifactID != nil)
	if !ok {
		return
	}
	if !anySet {
		msg := "at least one filter is required: keepLast, olderThan, from, below, hasFiles, hasIncompleteFiles, minBytes, maxBytes"
		if artifactID != nil {
			msg += ", version"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	params.ArtifactID = artifactID
	params.ProtectedTag = f.protectedTag
	dryRun := c.Query("dryRun") == "true"

	rows, err := f.queries.ListVersionsFiltered(c, params)
	if err != nil {
		internalError(c, err)
		return
	}
	var skippedCount int64
	candidates := make(map[int64]db.ListVersionsFilteredRow)
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.Protected {
			skippedCount++
			continue
		}
		candidates[r.ID] = r
		ids = append(ids, r.ID)
	}

	if dryRun || len(ids) == 0 {
		c.JSON(http.StatusOK, bulkVersionDeleteResponse(rows, ids, skippedCount, dryRun))
		return
	}

	tx, err := f.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c) //nolint:errcheck
	qtx := f.queries.WithTx(tx)
	files, err := qtx.ListFilesByVersionIDs(c, ids)
	if err != nil {
		internalError(c, err)
		return
	}
	deletedIDs, err := qtx.DeleteUnprotectedVersionsByIDs(c, db.DeleteUnprotectedVersionsByIDsParams{
		Ids:          ids,
		ProtectedTag: f.protectedTag,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	log := middleware.LoggerFromCtx(c)
	deleted := make(map[int64]bool, len(deletedIDs))
	for _, id := range deletedIDs {
		deleted[id] = true
	}
	for _, file := range files {
		if !deleted[file.VersionID] {
			continue
		}
		v := candidates[file.VersionID]
		path := fileObjectPath(file, v.ArtifactID, semver.Semver{Major: v.Major, Minor: v.Minor, Patch: v.Patch})
		if err := f.storage.Delete(path); err != nil { // best-effort
			log.Warn("delete file from storage on bulk version delete", "artifact_id", v.ArtifactID, "path", path, "error", err)
		}
	}

	// Versions tagged between listing and deleting were kept by the delete's re-check.
	skippedCount += int64(len(ids) - len(deletedIDs))
	log.Info("bulk version delete", "artifact_id", artifactID, "deleted", len(deletedIDs), "skipped", skippedCount, "files", len(files))
	c.JSON(http.StatusOK, bulkVersionDeleteResponse(rows, deletedIDs, skippedCount, false))
}

func bulkVersionDeleteResponse(rows []db.ListVersionsFilteredRow, ids []int64, skippedCount int64, dryRun bool) dto.BulkVersionDeleteResponse {
	include := make(map[int64]bool, len(ids))
	for _, id := range ids {
		include[id] = true
	}
	versions := make([]dto.VersionRef, 0, len(ids))
	for _, r := range rows {
		if include[r.ID] {
			sv := semver.Semver{Major: r.Major, Minor: r.Minor, Patch: r.Patch}
			versions = append(versions, dto.VersionRef{ArtifactID: r.ArtifactID, Version: sv.String()})
		}
	}
	return dto.BulkVersionDeleteResponse{
		Deleted:  int64(len(versions)),
		Skipped:  skippedCount,
		DryRun:   dryRun,
		Versions: versions,
	}
}
