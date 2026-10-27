package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"fusion-platform/fusion-index/internal/api/middleware"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
)

func internalError(c *gin.Context, err error) {
	middleware.LoggerFromCtx(c).Error("internal error", "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func notFoundOrInternal(c *gin.Context, err error, msg string) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
	} else {
		internalError(c, err)
	}
}

func conflictError(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, gin.H{"error": msg})
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func pathID(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + raw})
		return 0, false
	}
	return id, true
}

func pathFileID(c *gin.Context) (int64, bool) {
	raw := c.Param("fileId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fileId: " + raw})
		return 0, false
	}
	return id, true
}

func pathTypeID(c *gin.Context) (int64, bool) {
	raw := c.Param("typeId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid typeId: " + raw})
		return 0, false
	}
	return id, true
}

func pathSemver(c *gin.Context) (semver.Semver, bool) {
	raw := c.Param("semver")
	sv, err := semver.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return semver.Semver{}, false
	}
	return sv, true
}

func parseOlderThan(c *gin.Context) (pgtype.Timestamptz, bool) {
	raw := c.Query("olderThan")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "olderThan is required"})
		return pgtype.Timestamptz{}, false
	}
	return parseOlderThanValue(c, raw)
}

func parseOlderThanValue(c *gin.Context, raw string) (pgtype.Timestamptz, bool) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "olderThan must be RFC3339 (e.g. 2024-01-01T00:00:00Z)"})
		return pgtype.Timestamptz{}, false
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, true
}

func parsePagination(c *gin.Context) (page, pageSize int) {
	page = parseIntDefault(c.Query("page"), 0)
	pageSize = parseIntDefault(c.Query("pageSize"), 20)
	if page < 0 {
		page = 0
	}
	if pageSize < 1 {
		pageSize = 1
	}
	return
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// fileStoragePath is the backend-relative storage path of a file:
// {artifactID}/{major}/{minor}/{patch}/{fileID}/{filename}.
func fileStoragePath(artifactID int64, sv semver.Semver, fileID int64, name string) string {
	return fmt.Sprintf("%d/%d/%d/%d/%d/%s", artifactID, sv.Major, sv.Minor, sv.Patch, fileID, name)
}

// fileObjectPath is where a file's object lives in storage. Only AVAILABLE rows carry
// their real storage_path; PENDING/ERROR rows still hold the "pending" placeholder.
func fileObjectPath(f db.RegistryArtifactFile, artifactID int64, sv semver.Semver) string {
	if f.Status == "AVAILABLE" {
		return f.StoragePath
	}
	return fileStoragePath(artifactID, sv, f.ID, f.Name)
}

// parseOptionalInt32 reads an optional integer query parameter >= min; nil when absent.
func parseOptionalInt32(c *gin.Context, name string, min int) (*int32, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be an integer >= %d", name, min)})
		return nil, false
	}
	v := int32(n)
	return &v, true
}

// parseOptionalInt64 reads an optional non-negative integer query parameter (byte sizes).
func parseOptionalInt64(c *gin.Context, name string) (*int64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be an integer >= 0"})
		return nil, false
	}
	return &n, true
}

// parseOptionalBool reads an optional true/false query parameter; nil when absent.
func parseOptionalBool(c *gin.Context, name string) (*bool, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be true or false"})
		return nil, false
	}
	return &b, true
}

// parseOptionalOlderThan is parseOlderThan for endpoints where the filter is optional.
func parseOptionalOlderThan(c *gin.Context) (pgtype.Timestamptz, bool) {
	raw := c.Query("olderThan")
	if raw == "" {
		return pgtype.Timestamptz{}, true
	}
	return parseOlderThanValue(c, raw)
}

// tagsByVersionIDs fetches the tags of the given versions in one query, grouped by
// version ID. Tags are decoration on list responses, so a failure only logs.
func tagsByVersionIDs(c *gin.Context, q *db.Queries, ids []int64) map[int64][]db.RegistryArtifactTag {
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.ListTagsByVersionIDs(c, ids)
	if err != nil {
		middleware.LoggerFromCtx(c).Warn("fetch tags for version list", "error", err)
		return nil
	}
	result := make(map[int64][]db.RegistryArtifactTag)
	for _, t := range rows {
		result[t.VersionID] = append(result[t.VersionID], t)
	}
	return result
}
