package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/handlers"
	"fusion-platform/fusion-index/internal/api/middleware"
	"fusion-platform/fusion-index/internal/api/openapi"
	"fusion-platform/fusion-index/internal/config"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

func NewRouter(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, storageBackend string, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.NewLoggingMiddleware())
	r.Use(corsMiddleware())

	// OpenAPI spec + Swagger UI
	r.GET("/api/openapi.json", openapi.ServeSpec)
	r.GET("/swagger/", openapi.ServeUI)

	// Health probes (same paths as Quarkus Smallrye Health)
	r.GET("/q/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})
	r.GET("/q/health/ready", func(c *gin.Context) {
		if err := pool.Ping(c); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	ah := handlers.NewArtifactHandler(pool, q)
	vh := handlers.NewVersionHandler(pool, q, s)
	fh := handlers.NewFileHandler(pool, q, s, storageBackend)
	th := handlers.NewTagHandler(pool, q)
	tyh := handlers.NewTypeHandler(pool, q)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.NewAuthMiddleware(cfg))

	// Artifacts
	v1.GET("/artifacts", ah.List)
	v1.POST("/artifacts", ah.Create)
	v1.GET("/artifacts/:id", ah.Get)
	v1.PUT("/artifacts/:id", ah.Update)
	v1.DELETE("/artifacts/:id", ah.Delete)

	// Versions
	v1.GET("/artifacts/:id/versions", vh.List)
	v1.POST("/artifacts/:id/versions", vh.Create)
	v1.GET("/artifacts/:id/versions/:semver", vh.Get)
	v1.DELETE("/artifacts/:id/versions/:semver", vh.Delete)

	// Tags
	v1.PUT("/artifacts/:id/tags/:tag", th.Put)
	v1.DELETE("/artifacts/:id/tags/:tag", th.Delete)

	// Files
	v1.GET("/artifacts/:id/versions/:semver/files", fh.List)
	v1.POST("/artifacts/:id/versions/:semver/files", fh.Upload)
	v1.GET("/artifacts/:id/versions/:semver/files/:fileId", fh.Get)
	v1.GET("/artifacts/:id/versions/:semver/files/:fileId/download", fh.Download)
	v1.DELETE("/artifacts/:id/versions/:semver/files/:fileId", fh.Delete)

	// Types
	v1.GET("/types", tyh.List)
	v1.POST("/types", tyh.Create)
	v1.GET("/types/:typeId", tyh.Get)
	v1.PUT("/types/:typeId", tyh.Update)
	v1.DELETE("/types/:typeId", tyh.Delete)

	// Artifact type assignments
	v1.GET("/artifacts/:id/types", tyh.ListForArtifact)
	v1.PUT("/artifacts/:id/types/:typeId", tyh.Assign)
	v1.DELETE("/artifacts/:id/types/:typeId", tyh.Unassign)

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "accept,authorization,content-type,x-requested-with")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
