package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
)

type TypeHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewTypeHandler(pool *pgxpool.Pool, q *db.Queries) *TypeHandler {
	return &TypeHandler{pool: pool, queries: q}
}

func (h *TypeHandler) List(c *gin.Context) {
	types, err := h.queries.ListArtifactTypes(c)
	if err != nil {
		internalError(c, err)
		return
	}
	items := make([]dto.TypeResponse, len(types))
	for i, t := range types {
		items[i] = dto.ToTypeResponse(t)
	}
	c.JSON(http.StatusOK, items)
}

func (h *TypeHandler) Create(c *gin.Context) {
	var req dto.CreateTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.queries.CreateArtifactType(c, db.CreateArtifactTypeParams{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		if isUniqueViolation(err) {
			conflictError(c, "type with this name already exists")
			return
		}
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToTypeResponse(t))
}

func (h *TypeHandler) Get(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	t, err := h.queries.GetArtifactType(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "type not found")
		return
	}
	c.JSON(http.StatusOK, dto.ToTypeResponse(t))
}

func (h *TypeHandler) Update(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	var req dto.UpdateTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.queries.UpdateArtifactType(c, db.UpdateArtifactTypeParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		if isUniqueViolation(err) {
			conflictError(c, "type with this name already exists")
			return
		}
		notFoundOrInternal(c, err, "type not found")
		return
	}
	c.JSON(http.StatusOK, dto.ToTypeResponse(t))
}

func (h *TypeHandler) Delete(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetArtifactType(c, id); err != nil {
		notFoundOrInternal(c, err, "type not found")
		return
	}
	if err := h.queries.DeleteArtifactType(c, id); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TypeHandler) ListForArtifact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, id); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	types, err := h.queries.ListArtifactTypesByArtifactID(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	items := make([]dto.TypeResponse, len(types))
	for i, t := range types {
		items[i] = dto.ToTypeResponse(t)
	}
	c.JSON(http.StatusOK, items)
}

func (h *TypeHandler) Assign(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	typeID, ok := pathTypeID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, artifactID); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	t, err := h.queries.GetArtifactType(c, typeID)
	if err != nil {
		notFoundOrInternal(c, err, "type not found")
		return
	}
	if err := h.queries.AssignArtifactType(c, db.AssignArtifactTypeParams{
		ArtifactID: artifactID,
		TypeID:     typeID,
	}); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTypeResponse(t))
}

func (h *TypeHandler) Unassign(c *gin.Context) {
	artifactID, ok := pathID(c)
	if !ok {
		return
	}
	typeID, ok := pathTypeID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetArtifactTypeAssignment(c, db.GetArtifactTypeAssignmentParams{
		ArtifactID: artifactID,
		TypeID:     typeID,
	}); err != nil {
		notFoundOrInternal(c, err, "type not assigned to this artifact")
		return
	}
	if err := h.queries.RemoveArtifactType(c, db.RemoveArtifactTypeParams{
		ArtifactID: artifactID,
		TypeID:     typeID,
	}); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
