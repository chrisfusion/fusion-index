package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
)

type TagHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewTagHandler(pool *pgxpool.Pool, q *db.Queries) *TagHandler {
	return &TagHandler{pool: pool, queries: q}
}

func (h *TagHandler) Put(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")

	var req dto.AssignTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sv, err := semver.Parse(req.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.queries.GetRegistryArtifact(c, artifactID); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
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

	t, err := h.queries.UpsertArtifactTag(c, db.UpsertArtifactTagParams{
		ArtifactID: artifactID,
		Tag:        tag,
		VersionID:  version.ID,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTagResponse(t))
}

func (h *TagHandler) Delete(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")

	if _, err := h.queries.GetArtifactTag(c, db.GetArtifactTagParams{
		ArtifactID: artifactID,
		Tag:        tag,
	}); err != nil {
		notFoundOrInternal(c, err, "tag not found")
		return
	}

	if err := h.queries.DeleteArtifactTag(c, db.DeleteArtifactTagParams{
		ArtifactID: artifactID,
		Tag:        tag,
	}); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAll lists tags across all artifacts (paginated). Filters: tag (exact name) and
// broken (true = the tagged version has no AVAILABLE file).
func (h *TagHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePagination(c)
	broken, ok := parseOptionalBool(c, "broken")
	if !ok {
		return
	}
	params := db.ListTagsFilteredParams{
		Broken:    broken,
		RowLimit:  int32(pageSize),
		RowOffset: int32(page * pageSize),
	}
	if tag := c.Query("tag"); tag != "" {
		params.Tag = &tag
	}
	rows, err := h.queries.ListTagsFiltered(c, params)
	if err != nil {
		internalError(c, err)
		return
	}
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	} else if page > 0 {
		// Past the last page the window count has no row to ride on; ask page 0.
		params.RowLimit, params.RowOffset = 1, 0
		first, err := h.queries.ListTagsFiltered(c, params)
		if err != nil {
			internalError(c, err)
			return
		}
		if len(first) > 0 {
			total = first[0].TotalCount
		}
	}

	items := make([]dto.TagListItem, len(rows))
	for i, r := range rows {
		sv := semver.Semver{Major: r.Major, Minor: r.Minor, Patch: r.Patch}
		items[i] = dto.TagListItem{
			ID: r.ID, ArtifactID: r.ArtifactID, ArtifactName: r.FullName, Tag: r.Tag,
			VersionID: r.VersionID, Version: sv.String(), Broken: r.Broken, UpdatedAt: r.UpdatedAt.Time,
		}
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.TagListItem]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}
