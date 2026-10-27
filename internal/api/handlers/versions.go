package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	"fusion-platform/fusion-index/internal/api/middleware"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
	"fusion-platform/fusion-index/internal/storage"
)

type VersionHandler struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	storage  storage.Storage
	filtered filteredVersions
}

func NewVersionHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, protectedTag string) *VersionHandler {
	return &VersionHandler{
		pool: pool, queries: q, storage: s,
		filtered: filteredVersions{pool: pool, queries: q, storage: s, protectedTag: protectedTag},
	}
}

func (h *VersionHandler) List(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, artifactID); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}

	rows, ok := h.filtered.query(c, &artifactID, nil, 0)
	if !ok {
		return
	}

	// Fetch all tags for the artifact once, then group by version_id.
	allTags, err := h.queries.ListArtifactTags(c, artifactID)
	if err != nil {
		internalError(c, err)
		return
	}
	tagsByVersion := make(map[int64][]db.RegistryArtifactTag)
	for _, t := range allTags {
		tagsByVersion[t.VersionID] = append(tagsByVersion[t.VersionID], t)
	}

	resp := make([]dto.VersionResponse, len(rows))
	for i, r := range rows {
		resp[i] = dto.ToVersionResponse(versionFromFilteredRow(r), tagsByVersion[r.ID])
	}
	c.JSON(http.StatusOK, resp)
}

// ListAll lists versions across all artifacts (paginated) with the same filters as the
// per-artifact list — the read-only counterpart of DELETE /admin/versions.
func (h *VersionHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePagination(c)
	limit := int32(pageSize)
	rows, ok := h.filtered.query(c, nil, &limit, int32(page*pageSize))
	if !ok {
		return
	}
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	} else if page > 0 {
		// Past the last page the window count has no row to ride on; ask page 0.
		one := int32(1)
		first, ok := h.filtered.query(c, nil, &one, 0)
		if !ok {
			return
		}
		if len(first) > 0 {
			total = first[0].TotalCount
		}
	}

	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	tagsByVersion := tagsByVersionIDs(c, h.queries, ids)
	items := make([]dto.VersionResponse, len(rows))
	for i, r := range rows {
		items[i] = dto.ToVersionResponse(versionFromFilteredRow(r), tagsByVersion[r.ID])
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.VersionResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *VersionHandler) Create(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sv, err := semver.Parse(req.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c)
	q := h.queries.WithTx(tx)

	if _, err := q.GetRegistryArtifact(c, artifactID); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}

	version, err := q.CreateArtifactVersion(c, db.CreateArtifactVersionParams{
		ArtifactID: artifactID,
		Major:      sv.Major,
		Minor:      sv.Minor,
		Patch:      sv.Patch,
		Config:     req.Config,
	})
	if err != nil {
		if isUniqueViolation(err) {
			conflictError(c, fmt.Sprintf("version %s already exists for this artifact", sv.String()))
			return
		}
		middleware.LoggerFromCtx(c).Error("create version", "artifact_id", artifactID, "version", sv.String(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tagRows := make([]db.RegistryArtifactTag, 0, len(req.Tags))
	for _, tag := range req.Tags {
		t, err := q.UpsertArtifactTag(c, db.UpsertArtifactTagParams{
			ArtifactID: artifactID,
			Tag:        tag,
			VersionID:  version.ID,
		})
		if err != nil {
			middleware.LoggerFromCtx(c).Error("upsert tag on create version", "artifact_id", artifactID, "version", sv.String(), "tag", tag, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tagRows = append(tagRows, t)
	}

	if err := tx.Commit(c); err != nil {
		middleware.LoggerFromCtx(c).Error("commit create version", "artifact_id", artifactID, "version", sv.String(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.ToVersionResponse(version, tagRows))
}

func (h *VersionHandler) Get(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	sv, ok := pathSemver(c)
	if !ok {
		return
	}

	version, err := h.queries.GetArtifactVersion(c, db.GetArtifactVersionParams{
		ArtifactID: artifactID,
		Major:      sv.Major,
		Minor:      sv.Minor,
		Patch:      sv.Patch,
	})
	if err != nil {
		notFoundOrInternal(c, err, "version not found")
		return
	}

	tags, err := h.queries.ListArtifactTagsByVersionID(c, version.ID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToVersionResponse(version, tags))
}

func (h *VersionHandler) Delete(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	sv, ok := pathSemver(c)
	if !ok {
		return
	}

	version, err := h.queries.GetArtifactVersion(c, db.GetArtifactVersionParams{
		ArtifactID: artifactID,
		Major:      sv.Major,
		Minor:      sv.Minor,
		Patch:      sv.Patch,
	})
	if err != nil {
		notFoundOrInternal(c, err, "version not found")
		return
	}

	// Best-effort storage cleanup before removing DB rows.
	files, err := h.queries.ListArtifactFiles(c, version.ID)
	if err == nil {
		for _, f := range files {
			path := fileObjectPath(f, artifactID, sv)
			if err := h.storage.Delete(path); err != nil {
				middleware.LoggerFromCtx(c).Warn("delete file from storage on version delete", "path", path, "error", err)
			}
		}
	}

	if err := h.queries.DeleteArtifactVersion(c, version.ID); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// BulkDelete deletes the versions of one artifact matching the query filters.
func (h *VersionHandler) BulkDelete(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, artifactID); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	h.filtered.bulkDelete(c, &artifactID)
}
