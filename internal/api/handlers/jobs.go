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

type JobHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewJobHandler(pool *pgxpool.Pool, q *db.Queries) *JobHandler {
	return &JobHandler{pool: pool, queries: q}
}

func (h *JobHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)

	tx, err := h.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c)
	q := h.queries.WithTx(tx)

	jobs, err := q.ListJobs(c, db.ListJobsParams{
		Limit:  int32(pageSize),
		Offset: int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := q.CountJobs(c)
	if err != nil {
		internalError(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.JobResponse, len(jobs))
	for i, j := range jobs {
		items[i] = dto.ToJobResponse(j)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.JobResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *JobHandler) Create(c *gin.Context) {
	var req dto.CreateJobRequest
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

	if _, err := q.GetJobByName(c, req.Name); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A job with name '" + req.Name + "' already exists."})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(c, err)
		return
	}

	if _, err := q.GetTemplateVersionByID(c, req.TemplateVersionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job template version not found: " + strconv.FormatInt(req.TemplateVersionID, 10)})
		} else {
			internalError(c, err)
		}
		return
	}

	j, err := q.CreateJob(c, db.CreateJobParams{
		Name:              req.Name,
		Description:       req.Description,
		TemplateVersionID: req.TemplateVersionID,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	newVersionNum, err := q.IncrementJobVersion(c, j.ID)
	if err != nil {
		internalError(c, err)
		return
	}

	if _, err := q.CreateJobVersion(c, db.CreateJobVersionParams{
		JobID:             j.ID,
		VersionNumber:     newVersionNum,
		DockerImage:       req.DockerImage,
		GitUrl:            req.GitURL,
		GitRef:            req.GitRef,
		GitSubpath:        req.GitSubpath,
		RunConfig:         req.RunConfig,
		TemplateVersionID: req.TemplateVersionID,
	}); err != nil {
		internalError(c, err)
		return
	}

	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	j.LatestVersionNumber = newVersionNum
	c.JSON(http.StatusCreated, dto.ToJobResponse(j))
}

func (h *JobHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	j, err := h.queries.GetJobByID(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "Job not found: "+strconv.FormatInt(id, 10))
		return
	}
	c.JSON(http.StatusOK, dto.ToJobResponse(j))
}

func (h *JobHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.UpdateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := h.queries.GetJobByID(c, id)
	if err != nil {
		notFoundOrInternal(c, err, "Job not found: "+strconv.FormatInt(id, 10))
		return
	}

	description := current.Description
	if req.Description != nil {
		description = req.Description
	}

	updated, err := h.queries.UpdateJob(c, db.UpdateJobParams{
		ID:          id,
		Description: description,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToJobResponse(updated))
}

func (h *JobHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetJobByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job not found: "+strconv.FormatInt(id, 10))
		return
	}
	if err := h.queries.DeleteJob(c, id); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *JobHandler) ListVersions(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetJobByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job not found: "+strconv.FormatInt(id, 10))
		return
	}
	versions, err := h.queries.ListJobVersions(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	resp := make([]dto.JobVersionResponse, len(versions))
	for i, v := range versions {
		count, _ := h.queries.CountArtifactsForJobVersion(c, v.ID)
		resp[i] = dto.ToJobVersionResponse(v, count)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *JobHandler) PublishVersion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.PublishJobVersionRequest
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

	if _, err := q.GetJobByID(c, id); err != nil {
		notFoundOrInternal(c, err, "Job not found: "+strconv.FormatInt(id, 10))
		return
	}
	if _, err := q.GetTemplateVersionByID(c, req.TemplateVersionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job template version not found: " + strconv.FormatInt(req.TemplateVersionID, 10)})
		} else {
			internalError(c, err)
		}
		return
	}

	newVersionNum, err := q.IncrementJobVersion(c, id)
	if err != nil {
		internalError(c, err)
		return
	}

	v, err := q.CreateJobVersion(c, db.CreateJobVersionParams{
		JobID:             id,
		VersionNumber:     newVersionNum,
		DockerImage:       req.DockerImage,
		GitUrl:            req.GitURL,
		GitRef:            req.GitRef,
		GitSubpath:        req.GitSubpath,
		RunConfig:         req.RunConfig,
		TemplateVersionID: req.TemplateVersionID,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToJobVersionResponse(v, 0))
}

func (h *JobHandler) GetVersion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vn, ok := pathVersionNumber(c)
	if !ok {
		return
	}
	v, err := h.queries.GetJobVersionByJobAndNumber(c, db.GetJobVersionByJobAndNumberParams{
		JobID:         id,
		VersionNumber: int32(vn),
	})
	if err != nil {
		notFoundOrInternal(c, err, "Job version not found")
		return
	}
	count, _ := h.queries.CountArtifactsForJobVersion(c, v.ID)
	c.JSON(http.StatusOK, dto.ToJobVersionResponse(v, count))
}
