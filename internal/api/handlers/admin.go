package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fusion-platform/fusion-index/internal/api/dto"
	"fusion-platform/fusion-index/internal/api/middleware"
	db "fusion-platform/fusion-index/internal/db/sqlc"
)

type AdminHandler struct {
	queries      *db.Queries
	protectedTag string
}

func NewAdminHandler(q *db.Queries, protectedTag string) *AdminHandler {
	return &AdminHandler{queries: q, protectedTag: protectedTag}
}

func (h *AdminHandler) batchFetchTagsByVersionIDs(c *gin.Context, versions []db.RegistryArtifactVersion) map[int64][]db.RegistryArtifactTag {
	if len(versions) == 0 {
		return nil
	}
	ids := make([]int64, len(versions))
	for i, v := range versions {
		ids[i] = v.ID
	}
	rows, err := h.queries.ListTagsByVersionIDs(c, ids)
	if err != nil {
		middleware.LoggerFromCtx(c).Warn("fetch tags for admin version list", "error", err)
		return nil
	}
	result := make(map[int64][]db.RegistryArtifactTag)
	for _, t := range rows {
		result[t.VersionID] = append(result[t.VersionID], t)
	}
	return result
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

	tagsByVersion := h.batchFetchTagsByVersionIDs(c, versions)
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
