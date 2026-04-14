package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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

func pathID(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + raw})
		return 0, false
	}
	return id, true
}

func pathVersionNumber(c *gin.Context) (int, bool) {
	raw := c.Param("versionNumber")
	n, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid versionNumber: " + raw})
		return 0, false
	}
	return n, true
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
