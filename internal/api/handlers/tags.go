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
