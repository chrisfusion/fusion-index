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

type ArtifactHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	storage storage.Storage
}

func NewArtifactHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage) *ArtifactHandler {
	return &ArtifactHandler{pool: pool, queries: q, storage: s}
}

// artifactSorts maps the sort query values to their default direction (true = desc).
var artifactSorts = map[string]bool{
	"createdAt":      true,
	"name":           false,
	"versionCount":   true,
	"largestVersion": true,
	"totalSize":      true,
}

// parseArtifactListFilter reads the list filters (combined with AND) and the sort.
// The maintenance views are plain filters here, so normal users get them too:
// maxVersions=0 (no versions), minVersions=1&hasFiles=false (versions but no files),
// hasIncompleteFiles=true (PENDING/ERROR uploads). missingTag=latest finds artifacts
// without that tag, hasRetention whether a retention rule applies (own or via a type).
func parseArtifactListFilter(c *gin.Context) (db.ListRegistryArtifactsFilteredParams, bool) {
	var p db.ListRegistryArtifactsFilteredParams
	var ok bool
	if name := c.Query("name"); name != "" {
		p.NamePrefix = &name
	}
	if tag := c.Query("tag"); tag != "" {
		p.Tag = &tag
	}
	p.Types = c.QueryArray("type")
	if p.OlderThan, ok = parseOptionalOlderThan(c); !ok {
		return p, false
	}
	if p.MinVersions, ok = parseOptionalInt32(c, "minVersions", 0); !ok {
		return p, false
	}
	if p.MaxVersions, ok = parseOptionalInt32(c, "maxVersions", 0); !ok {
		return p, false
	}
	if p.HasFiles, ok = parseOptionalBool(c, "hasFiles"); !ok {
		return p, false
	}
	if p.HasIncompleteFiles, ok = parseOptionalBool(c, "hasIncompleteFiles"); !ok {
		return p, false
	}
	if tag := c.Query("missingTag"); tag != "" {
		p.MissingTag = &tag
	}
	if p.MinTotalBytes, ok = parseOptionalInt64(c, "minTotalBytes"); !ok {
		return p, false
	}
	if p.HasRetention, ok = parseOptionalBool(c, "hasRetention"); !ok {
		return p, false
	}

	p.Sort = c.DefaultQuery("sort", "createdAt")
	desc, known := artifactSorts[p.Sort]
	if !known {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be one of createdAt, name, versionCount, largestVersion, totalSize"})
		return p, false
	}
	switch c.Query("order") {
	case "":
	case "asc":
		desc = false
	case "desc":
		desc = true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "order must be asc or desc"})
		return p, false
	}
	p.SortDesc = desc
	return p, true
}

func (h *ArtifactHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	params, ok := parseArtifactListFilter(c)
	if !ok {
		return
	}
	params.RowLimit = int32(pageSize)
	params.RowOffset = int32(page * pageSize)

	artifacts, err := h.queries.ListRegistryArtifactsFiltered(c, params)
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountRegistryArtifactsFiltered(c, db.CountRegistryArtifactsFilteredParams{
		NamePrefix:         params.NamePrefix,
		Tag:                params.Tag,
		Types:              params.Types,
		OlderThan:          params.OlderThan,
		MinVersions:        params.MinVersions,
		MaxVersions:        params.MaxVersions,
		HasFiles:           params.HasFiles,
		HasIncompleteFiles: params.HasIncompleteFiles,
		MissingTag:         params.MissingTag,
		MinTotalBytes:      params.MinTotalBytes,
		HasRetention:       params.HasRetention,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	typesByArtifact := h.batchFetchTypes(c, artifacts)
	items := make([]dto.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		items[i] = dto.ToArtifactResponse(a, typesByArtifact[a.ID])
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.ArtifactResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *ArtifactHandler) Create(c *gin.Context) {
	var req dto.CreateArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

	_, err = q.GetRegistryArtifactByName(c, req.FullName)
	if err == nil {
		conflictError(c, "artifact with this name already exists")
		return
	}
	if !isNotFound(err) {
		middleware.LoggerFromCtx(c).Error("check artifact name", "name", req.FullName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	a, err := q.CreateRegistryArtifact(c, db.CreateRegistryArtifactParams{
		FullName:    req.FullName,
		Description: req.Description,
	})
	if err != nil {
		middleware.LoggerFromCtx(c).Error("create artifact", "name", req.FullName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(c); err != nil {
		middleware.LoggerFromCtx(c).Error("commit create artifact", "name", req.FullName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto.ToArtifactResponse(a, nil))
}

func (h *ArtifactHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.queries.GetRegistryArtifact(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	types, err := h.queries.ListArtifactTypesByArtifactID(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToArtifactResponse(a, types))
}

func (h *ArtifactHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := h.queries.UpdateRegistryArtifact(c, db.UpdateRegistryArtifactParams{
		ID:          id,
		Description: req.Description,
	})
	if err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	types, err := h.queries.ListArtifactTypesByArtifactID(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToArtifactResponse(a, types))
}

func (h *ArtifactHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, id); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}

	// DB first (the cascade drops versions, files and tags), storage after commit and
	// best-effort — same order as the bulk version delete.
	tx, err := h.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c) //nolint:errcheck
	qtx := h.queries.WithTx(tx)
	files, err := qtx.ListFilesByArtifactID(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	if err := qtx.DeleteRegistryArtifact(c, id); err != nil {
		internalError(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}
	for _, r := range files {
		path := fileObjectPath(r.RegistryArtifactFile, id, semver.Semver{Major: r.Major, Minor: r.Minor, Patch: r.Patch})
		if err := h.storage.Delete(path); err != nil { // best-effort
			middleware.LoggerFromCtx(c).Warn("delete file from storage on artifact delete", "artifact_id", id, "path", path, "error", err)
		}
	}
	c.Status(http.StatusNoContent)
}

// batchFetchTypes fetches all types for the given artifacts in one query and
// groups them by artifact ID.
func (h *ArtifactHandler) batchFetchTypes(c *gin.Context, artifacts []db.RegistryArtifact) map[int64][]db.RegistryArtifactType {
	if len(artifacts) == 0 {
		return nil
	}
	ids := make([]int64, len(artifacts))
	for i, a := range artifacts {
		ids[i] = a.ID
	}
	rows, err := h.queries.ListArtifactTypesByArtifactIDs(c, ids)
	if err != nil {
		middleware.LoggerFromCtx(c).Warn("fetch types for artifacts", "error", err)
		return nil
	}
	result := make(map[int64][]db.RegistryArtifactType)
	for _, row := range rows {
		t := db.RegistryArtifactType{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
		result[row.ArtifactID] = append(result[row.ArtifactID], t)
	}
	return result
}
