package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
	"fusion-platform/fusion-index/internal/storage"
)

type VersionHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	storage storage.Storage
}

func NewVersionHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage) *VersionHandler {
	return &VersionHandler{pool: pool, queries: q, storage: s}
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

	versions, err := h.queries.ListArtifactVersions(c, artifactID)
	if err != nil {
		internalError(c, err)
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

	resp := make([]dto.VersionResponse, len(versions))
	for i, v := range versions {
		resp[i] = dto.ToVersionResponse(v, tagsByVersion[v.ID])
	}
	c.JSON(http.StatusOK, resp)
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
		internalError(c, err)
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
			internalError(c, err)
			return
		}
		tagRows = append(tagRows, t)
	}

	if err := tx.Commit(c); err != nil {
		internalError(c, err)
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
			_ = h.storage.Delete(f.StoragePath)
		}
	}

	if err := h.queries.DeleteArtifactVersion(c, version.ID); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
