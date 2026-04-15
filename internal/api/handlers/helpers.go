package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"fusion-platform/fusion-index/internal/semver"
)

func internalError(c *gin.Context, err error) {
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
