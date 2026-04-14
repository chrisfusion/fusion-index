package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
)

type TemplateHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewTemplateHandler(pool *pgxpool.Pool, q *db.Queries) *TemplateHandler {
	return &TemplateHandler{pool: pool, queries: q}
}

func (h *TemplateHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)

	tx, err := h.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c)
	q := h.queries.WithTx(tx)

	templates, err := q.ListTemplates(c, db.ListTemplatesParams{
		Limit:  int32(pageSize),
		Offset: int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := q.CountTemplates(c)
	if err != nil {
		internalError(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.TemplateResponse, len(templates))
	for i, t := range templates {
		items[i] = dto.ToTemplateResponse(t)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.TemplateResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req dto.CreateTemplateRequest
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

	if _, err := q.GetTemplateByName(c, req.Name); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A job template with name '" + req.Name + "' already exists."})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(c, err)
		return
	}

	t, err := q.CreateTemplate(c, db.CreateTemplateParams{
		Name:        req.Name,
		Description: req.Description,
		DockerImage: req.DockerImage,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	newVersionNum, err := q.IncrementTemplateVersion(c, t.ID)
	if err != nil {
		internalError(c, err)
		return
	}

	dockerImage := req.DockerImage
	if _, err := q.CreateTemplateVersion(c, db.CreateTemplateVersionParams{
		TemplateID:       t.ID,
		VersionNumber:    newVersionNum,
		DockerImage:      dockerImage,
		DefaultRunConfig: req.DefaultRunConfig,
		Changelog:        req.Changelog,
	}); err != nil {
		internalError(c, err)
		return
	}

	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	t.LatestVersionNumber = newVersionNum
	c.JSON(http.StatusCreated, dto.ToTemplateResponse(t))
}

func (h *TemplateHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	t, err := h.queries.GetTemplateByID(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "Job template not found: "+strconv.FormatInt(id, 10))
		return
	}
	c.JSON(http.StatusOK, dto.ToTemplateResponse(t))
}

func (h *TemplateHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := h.queries.GetTemplateByID(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "Job template not found: "+strconv.FormatInt(id, 10))
		return
	}

	description := current.Description
	if req.Description != nil {
		description = req.Description
	}
	dockerImage := current.DockerImage
	if req.DockerImage != nil {
		dockerImage = *req.DockerImage
	}

	updated, err := h.queries.UpdateTemplate(c, db.UpdateTemplateParams{
		ID:          id,
		Description: description,
		DockerImage: dockerImage,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTemplateResponse(updated))
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetTemplateByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job template not found: "+strconv.FormatInt(id, 10))
		return
	}

	count, err := h.queries.CountJobsForTemplate(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete template that is referenced by existing jobs."})
		return
	}

	if err := h.queries.DeleteTemplate(c, id); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TemplateHandler) ListVersions(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetTemplateByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job template not found: "+strconv.FormatInt(id, 10))
		return
	}
	versions, err := h.queries.ListTemplateVersions(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	resp := make([]dto.TemplateVersionResponse, len(versions))
	for i, v := range versions {
		resp[i] = dto.ToTemplateVersionResponse(v)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *TemplateHandler) PublishVersion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.PublishTemplateVersionRequest
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

	if _, err := q.GetTemplateByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job template not found: "+strconv.FormatInt(id, 10))
		return
	}

	newVersionNum, err := q.IncrementTemplateVersion(c, id)
	if err != nil {
		internalError(c, err)
		return
	}

	v, err := q.CreateTemplateVersion(c, db.CreateTemplateVersionParams{
		TemplateID:       id,
		VersionNumber:    newVersionNum,
		DockerImage:      req.DockerImage,
		DefaultRunConfig: req.DefaultRunConfig,
		Changelog:        req.Changelog,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToTemplateVersionResponse(v))
}

func (h *TemplateHandler) GetVersion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vn, ok := pathVersionNumber(c)
	if !ok {
		return
	}
	v, err := h.queries.GetTemplateVersion(c, db.GetTemplateVersionParams{
		TemplateID:    id,
		VersionNumber: int32(vn),
	})
	if err != nil {
		notFoundOrInternal(c, err, "Template version not found")
		return
	}
	c.JSON(http.StatusOK, dto.ToTemplateVersionResponse(v))
}
