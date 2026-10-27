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

type AdminHandler struct {
	queries      *db.Queries
	storage      storage.Storage
	protectedTag string
	filtered     filteredVersions
}

func NewAdminHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, protectedTag string) *AdminHandler {
	return &AdminHandler{
		queries: q, storage: s, protectedTag: protectedTag,
		filtered: filteredVersions{pool: pool, queries: q, storage: s, protectedTag: protectedTag},
	}
}

func skipped(total, deleted int64) int64 {
	if s := total - deleted; s > 0 {
		return s
	}
	return 0
}

func (h *AdminHandler) ListEmptyArtifacts(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)

	artifacts, err := h.queries.ListEmptyArtifacts(c, db.ListEmptyArtifactsParams{
		CreatedAt: olderThan,
		Limit:     int32(pageSize),
		Offset:    int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountEmptyArtifacts(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		items[i] = dto.ToArtifactResponse(a, nil)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.ArtifactResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) DeleteEmptyArtifacts(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	total, err := h.queries.CountEmptyArtifacts(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	deleted, err := h.queries.DeleteEmptyArtifacts(c, db.DeleteEmptyArtifactsParams{
		CreatedAt: olderThan,
		Tag:       h.protectedTag,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped(total, deleted)})
}

func (h *AdminHandler) ListVersionsWithoutFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)

	versions, err := h.queries.ListVersionsWithoutFiles(c, db.ListVersionsWithoutFilesParams{
		CreatedAt: olderThan,
		Limit:     int32(pageSize),
		Offset:    int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountVersionsWithoutFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}

	ids := make([]int64, len(versions))
	for i, v := range versions {
		ids[i] = v.ID
	}
	tagsByVersion := tagsByVersionIDs(c, h.queries, ids)
	items := make([]dto.VersionResponse, len(versions))
	for i, v := range versions {
		items[i] = dto.ToVersionResponse(v, tagsByVersion[v.ID])
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.VersionResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) DeleteVersionsWithoutFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	total, err := h.queries.CountVersionsWithoutFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	deleted, err := h.queries.DeleteVersionsWithoutFiles(c, db.DeleteVersionsWithoutFilesParams{
		CreatedAt: olderThan,
		Tag:       h.protectedTag,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped(total, deleted)})
}

func (h *AdminHandler) ListArtifactsWithoutFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)

	artifacts, err := h.queries.ListArtifactsWithoutFiles(c, db.ListArtifactsWithoutFilesParams{
		CreatedAt: olderThan,
		Limit:     int32(pageSize),
		Offset:    int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountArtifactsWithoutFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		items[i] = dto.ToArtifactResponse(a, nil)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.ArtifactResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) DeleteArtifactsWithoutFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	total, err := h.queries.CountArtifactsWithoutFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	deleted, err := h.queries.DeleteArtifactsWithoutFiles(c, db.DeleteArtifactsWithoutFilesParams{
		CreatedAt: olderThan,
		Tag:       h.protectedTag,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped(total, deleted)})
}

func (h *AdminHandler) ListIncompleteFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)

	rows, err := h.queries.ListIncompleteFiles(c, db.ListIncompleteFilesParams{
		CreatedAt: olderThan,
		Limit:     int32(pageSize),
		Offset:    int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountIncompleteFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.FileResponse, len(rows))
	for i, r := range rows {
		sv := semver.Semver{Major: r.Major, Minor: r.Minor, Patch: r.Patch}
		items[i] = dto.ToFileResponse(r.RegistryArtifactFile, r.ArtifactID, sv)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.FileResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

// DeleteIncompleteFiles removes PENDING/ERROR file rows and, best-effort, any
// object a failed upload may have left behind. storage_path still holds the
// placeholder for these rows, so the path is rebuilt from the version coordinates.
func (h *AdminHandler) DeleteIncompleteFiles(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	total, err := h.queries.CountIncompleteFiles(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	rows, err := h.queries.DeleteIncompleteFiles(c, db.DeleteIncompleteFilesParams{
		CreatedAt: olderThan,
		Tag:       h.protectedTag,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	for _, r := range rows {
		sv := semver.Semver{Major: r.Major, Minor: r.Minor, Patch: r.Patch}
		path := fileStoragePath(r.ArtifactID, sv, r.ID, r.Name)
		if err := h.storage.Delete(path); err != nil { // best-effort
			middleware.LoggerFromCtx(c).Warn("delete incomplete file from storage", "artifact_id", r.ArtifactID, "path", path, "error", err)
		}
	}
	deleted := int64(len(rows))
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped(total, deleted)})
}

// BulkDeleteVersions deletes the versions of every artifact matching the query filters.
func (h *AdminHandler) BulkDeleteVersions(c *gin.Context) {
	h.filtered.bulkDelete(c, nil)
}

func (h *AdminHandler) ListUnusedTypes(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)

	types, err := h.queries.ListUnusedArtifactTypes(c, db.ListUnusedArtifactTypesParams{
		CreatedAt: olderThan,
		Limit:     int32(pageSize),
		Offset:    int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := h.queries.CountUnusedArtifactTypes(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.TypeResponse, len(types))
	for i, t := range types {
		items[i] = dto.ToTypeResponse(t)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.TypeResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

// DeleteUnusedTypes deletes types assigned to no artifact. Types carry no tags, so
// nothing is protection-skipped; skipped counts types assigned in the meantime.
func (h *AdminHandler) DeleteUnusedTypes(c *gin.Context) {
	olderThan, ok := parseOlderThan(c)
	if !ok {
		return
	}
	total, err := h.queries.CountUnusedArtifactTypes(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	deleted, err := h.queries.DeleteUnusedArtifactTypes(c, olderThan)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped(total, deleted)})
}
