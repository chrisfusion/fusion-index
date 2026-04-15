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

type FileHandler struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	storage        storage.Storage
	storageBackend string
}

func NewFileHandler(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, backend string) *FileHandler {
	return &FileHandler{pool: pool, queries: q, storage: s, storageBackend: backend}
}

func (h *FileHandler) resolveVersion(c *gin.Context) (int64, semver.Semver, db.RegistryArtifactVersion, bool) {
	artifactID, ok := pathID(c)
	if !ok {
		return 0, semver.Semver{}, db.RegistryArtifactVersion{}, false
	}
	sv, ok := pathSemver(c)
	if !ok {
		return 0, semver.Semver{}, db.RegistryArtifactVersion{}, false
	}
	version, err := h.queries.GetArtifactVersion(c, db.GetArtifactVersionParams{
		ArtifactID: artifactID,
		Major:      sv.Major,
		Minor:      sv.Minor,
		Patch:      sv.Patch,
	})
	if err != nil {
		notFoundOrInternal(c, err, "version not found")
		return 0, semver.Semver{}, db.RegistryArtifactVersion{}, false
	}
	return artifactID, sv, version, true
}

func (h *FileHandler) List(c *gin.Context) {
	artifactID, sv, version, ok := h.resolveVersion(c)
	if !ok {
		return
	}
	files, err := h.queries.ListArtifactFiles(c, version.ID)
	if err != nil {
		internalError(c, err)
		return
	}
	resp := make([]dto.FileResponse, len(files))
	for i, f := range files {
		resp[i] = dto.ToFileResponse(f, artifactID, sv)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *FileHandler) Upload(c *gin.Context) {
	artifactID, sv, version, ok := h.resolveVersion(c)
	if !ok {
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

	// Include the DB-assigned record ID in the path to prevent collisions when the
	// same filename is uploaded again (the unique constraint on (version_id, name)
	// blocks duplicates at the DB level, but guards storage uniqueness too).
	record, err := h.queries.CreateArtifactFile(c, db.CreateArtifactFileParams{
		VersionID:      version.ID,
		Name:           header.Filename,
		ContentType:    &contentType,
		StorageBackend: h.storageBackend,
		StoragePath:    "pending", // overwritten after storage write
	})
	if err != nil {
		if isUniqueViolation(err) {
			conflictError(c, fmt.Sprintf("file %q already exists in this version", header.Filename))
			return
		}
		internalError(c, err)
		return
	}

	storagePath := fmt.Sprintf("%d/%d/%d/%d/%d/%s", artifactID, sv.Major, sv.Minor, sv.Patch, record.ID, header.Filename)

	resolvedPath, err := h.storage.Store(storagePath, file, header.Size, contentType)
	if err != nil {
		_ = h.queries.UpdateArtifactFileStatus(c, db.UpdateArtifactFileStatusParams{
			ID:     record.ID,
			Status: "ERROR",
		})
		internalError(c, err)
		return
	}

	updated, err := h.queries.UpdateArtifactFileStored(c, db.UpdateArtifactFileStoredParams{
		ID:          record.ID,
		StoragePath: resolvedPath,
		SizeBytes:   &header.Size,
		Status:      "AVAILABLE",
	})
	if err != nil {
		// Storage write succeeded but DB update failed: mark ERROR and clean up.
		_ = h.queries.UpdateArtifactFileStatus(c, db.UpdateArtifactFileStatusParams{
			ID:     record.ID,
			Status: "ERROR",
		})
		_ = h.storage.Delete(resolvedPath)
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToFileResponse(updated, artifactID, sv))
}

func (h *FileHandler) Get(c *gin.Context) {
	artifactID, sv, version, ok := h.resolveVersion(c)
	if !ok {
		return
	}
	fileID, ok := pathFileID(c)
	if !ok {
		return
	}
	f, err := h.queries.GetArtifactFile(c, fileID)
	if err != nil {
		notFoundOrInternal(c, err, "file not found")
		return
	}
	if f.VersionID != version.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, dto.ToFileResponse(f, artifactID, sv))
}

func (h *FileHandler) Download(c *gin.Context) {
	_, sv, version, ok := h.resolveVersion(c)
	if !ok {
		return
	}
	fileID, ok := pathFileID(c)
	if !ok {
		return
	}
	f, err := h.queries.GetArtifactFile(c, fileID)
	if err != nil {
		notFoundOrInternal(c, err, "file not found")
		return
	}
	if f.VersionID != version.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	if f.Status != "AVAILABLE" {
		c.JSON(http.StatusConflict, gin.H{"error": "file is not available: status=" + f.Status})
		return
	}

	rc, err := h.storage.Retrieve(f.StoragePath)
	if err != nil {
		internalError(c, err)
		return
	}
	defer rc.Close()

	mime := "application/octet-stream"
	if f.ContentType != nil {
		mime = *f.ContentType
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, f.Name))
	_ = sv // used in resolveVersion for 404 guard; sv not needed for streaming
	c.DataFromReader(http.StatusOK, -1, mime, rc, nil)
}

func (h *FileHandler) Delete(c *gin.Context) {
	_, _, version, ok := h.resolveVersion(c)
	if !ok {
		return
	}
	fileID, ok := pathFileID(c)
	if !ok {
		return
	}
	f, err := h.queries.GetArtifactFile(c, fileID)
	if err != nil {
		notFoundOrInternal(c, err, "file not found")
		return
	}
	if f.VersionID != version.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	_ = h.storage.Delete(f.StoragePath) // best-effort
	if err := h.queries.DeleteArtifactFile(c, fileID); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
