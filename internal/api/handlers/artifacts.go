package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

type ArtifactHandler struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	storage        storage.Storage
	storageBackend string
}

func NewArtifactHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, backend string) *ArtifactHandler {
	return &ArtifactHandler{pool: pool, queries: q, storage: s, storageBackend: backend}
}

func (h *ArtifactHandler) ListForJobVersion(c *gin.Context) {
	jobID, ok := pathID(c)
	if !ok {
		return
	}
	vn, ok := pathVersionNumber(c)
	if !ok {
		return
	}

	jv, err := h.queries.GetJobVersionByJobAndNumber(c, db.GetJobVersionByJobAndNumberParams{
		JobID:         jobID,
		VersionNumber: int32(vn),
	})
	if err != nil {
		notFoundOrInternal(c, err, "Job version not found")
		return
	}

	artifacts, err := h.queries.ListArtifactsByJobVersion(c, jv.ID)
	if err != nil {
		internalError(c, err)
		return
	}
	resp := make([]dto.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		resp[i] = dto.ToArtifactResponse(a)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ArtifactHandler) Upload(c *gin.Context) {
	jobID, ok := pathID(c)
	if !ok {
		return
	}
	vn, ok := pathVersionNumber(c)
	if !ok {
		return
	}

	jv, err := h.queries.GetJobVersionByJobAndNumber(c, db.GetJobVersionByJobAndNumberParams{
		JobID:         jobID,
		VersionNumber: int32(vn),
	})
	if err != nil {
		notFoundOrInternal(c, err, "Job version not found")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}
	defer file.Close()

	contentType := c.Request.FormValue("contentType")
	if contentType == "" {
		contentType = header.Header.Get("Content-Type")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	storagePath := fmt.Sprintf("%d/%s", jv.ID, header.Filename)

	artifact, err := h.queries.CreateArtifact(c, db.CreateArtifactParams{
		JobVersionID:   jv.ID,
		Name:           header.Filename,
		ContentType:    &contentType,
		StorageBackend: h.storageBackend,
		StoragePath:    storagePath,
	})
	if err != nil {
		internalError(c, err)
		return
	}

	resolvedPath, err := h.storage.Store(storagePath, file, header.Size, contentType)
	if err != nil {
		_, _ = h.queries.UpdateArtifactStatus(c, db.UpdateArtifactStatusParams{
			ID:     artifact.ID,
			Status: "ERROR",
		})
		internalError(c, err)
		return
	}

	status := "AVAILABLE"
	updated, err := h.queries.UpdateArtifactStored(c, db.UpdateArtifactStoredParams{
		ID:          artifact.ID,
		StoragePath: resolvedPath,
		SizeBytes:   &header.Size,
		Status:      status,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToArtifactResponse(updated))
}

func (h *ArtifactHandler) ListAll(c *gin.Context) {
	page, pageSize := parsePagination(c)

	tx, err := h.pool.Begin(c)
	if err != nil {
		internalError(c, err)
		return
	}
	defer tx.Rollback(c)
	q := h.queries.WithTx(tx)

	artifacts, err := q.ListArtifacts(c, db.ListArtifactsParams{
		Limit:  int32(pageSize),
		Offset: int32(page * pageSize),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	total, err := q.CountArtifacts(c)
	if err != nil {
		internalError(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		internalError(c, err)
		return
	}

	items := make([]dto.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		items[i] = dto.ToArtifactResponse(a)
	}
	c.JSON(http.StatusOK, dto.PageResponse[dto.ArtifactResponse]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *ArtifactHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.queries.GetArtifact(c, id)
	if err != nil {
		notFoundOrInternal(c, err, fmt.Sprintf("Artifact not found: %d", id))
		return
	}
	c.JSON(http.StatusOK, dto.ToArtifactResponse(a))
}

func (h *ArtifactHandler) Download(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.queries.GetArtifact(c, id)
	if err != nil {
		notFoundOrInternal(c, err, fmt.Sprintf("Artifact not found: %d", id))
		return
	}
	if a.Status != "AVAILABLE" {
		c.JSON(http.StatusConflict, gin.H{"error": "Artifact is not available: status=" + a.Status})
		return
	}

	rc, err := h.storage.Retrieve(a.StoragePath)
	if err != nil {
		internalError(c, err)
		return
	}
	defer rc.Close()

	mime := "application/octet-stream"
	if a.ContentType != nil {
		mime = *a.ContentType
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, a.Name))
	c.DataFromReader(http.StatusOK, -1, mime, rc, nil)
}

func (h *ArtifactHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.queries.GetArtifact(c, id)
	if err != nil {
		notFoundOrInternal(c, err, fmt.Sprintf("Artifact not found: %d", id))
		return
	}
	// Best-effort storage cleanup
	_ = h.storage.Delete(a.StoragePath)

	if err := h.queries.DeleteArtifact(c, id); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
