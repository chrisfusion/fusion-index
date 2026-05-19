package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	"fusion-platform/fusion-index/internal/api/middleware"
	db "fusion-platform/fusion-index/internal/db/sqlc"
)

type ArtifactHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewArtifactHandler(pool *pgxpool.Pool, q *db.Queries) *ArtifactHandler {
	return &ArtifactHandler{pool: pool, queries: q}
}

func (h *ArtifactHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	name := c.Query("name")
	tag := c.Query("tag")
	types := c.QueryArray("type")

	var artifacts []db.RegistryArtifact
	var total int64
	var err error

	switch {
	case len(types) > 0:
		artifacts, err = h.queries.ListRegistryArtifactsByTypes(c, db.ListRegistryArtifactsByTypesParams{
			Column1: types,
			Limit:   int32(pageSize),
			Offset:  int32(page * pageSize),
		})
		if err != nil {
			internalError(c, err)
			return
		}
		total, err = h.queries.CountRegistryArtifactsByTypes(c, types)
	case name != "":
		pattern := name + "%"
		artifacts, err = h.queries.ListRegistryArtifactsByName(c, db.ListRegistryArtifactsByNameParams{
			FullName: pattern,
			Limit:    int32(pageSize),
			Offset:   int32(page * pageSize),
		})
		if err != nil {
			internalError(c, err)
			return
		}
		total, err = h.queries.CountRegistryArtifactsByName(c, pattern)
	case tag != "":
		artifacts, err = h.queries.ListRegistryArtifactsByTag(c, db.ListRegistryArtifactsByTagParams{
			Tag:    tag,
			Limit:  int32(pageSize),
			Offset: int32(page * pageSize),
		})
		if err != nil {
			internalError(c, err)
			return
		}
		total, err = h.queries.CountRegistryArtifactsByTag(c, tag)
	default:
		artifacts, err = h.queries.ListRegistryArtifacts(c, db.ListRegistryArtifactsParams{
			Limit:  int32(pageSize),
			Offset: int32(page * pageSize),
		})
		if err != nil {
			internalError(c, err)
			return
		}
		total, err = h.queries.CountRegistryArtifacts(c)
	}
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
	if err := h.queries.DeleteRegistryArtifact(c, id); err != nil {
		internalError(c, err)
		return
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
