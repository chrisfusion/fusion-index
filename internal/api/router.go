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
	"fusion-platform/fusion-index/internal/metrics"
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

	// Metrics — intentionally public (no auth) for Prometheus/dashboard scraping
	mh := handlers.NewMetricsHandler(pool, q, metrics.NewCache(cfg.MetricsCacheTTL))
	r.GET("/q/metrics", mh.Get)

	ah := handlers.NewArtifactHandler(pool, q, s)
	vh := handlers.NewVersionHandler(pool, q, s, cfg.AdminProtectedTag)
	fh := handlers.NewFileHandler(pool, q, s, storageBackend)
	th := handlers.NewTagHandler(pool, q)
	tyh := handlers.NewTypeHandler(pool, q)
	rh := handlers.NewRetentionHandler(q)
	adh := handlers.NewAdminHandler(pool, q, s, cfg.AdminProtectedTag)

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
	v1.DELETE("/artifacts/:id/versions", vh.BulkDelete)
	v1.GET("/artifacts/:id/versions/:semver", vh.Get)
	v1.DELETE("/artifacts/:id/versions/:semver", vh.Delete)

	// Tags
	v1.GET("/tags", th.ListAll)
	v1.PUT("/artifacts/:id/tags/:tag", th.Put)
	v1.DELETE("/artifacts/:id/tags/:tag", th.Delete)

	// Files
	v1.GET("/artifacts/:id/versions/:semver/files", fh.List)
	v1.POST("/artifacts/:id/versions/:semver/files", fh.Upload)
	v1.GET("/artifacts/:id/versions/:semver/files/:fileId", fh.Get)
	v1.GET("/artifacts/:id/versions/:semver/files/:fileId/download", fh.Download)
	v1.DELETE("/artifacts/:id/versions/:semver/files/:fileId", fh.Delete)

	// Versions across all artifacts (read-only; bulk delete is DELETE /admin/versions)
	v1.GET("/versions", vh.ListAll)

	// Types
	v1.GET("/types", tyh.List)
	v1.POST("/types", tyh.Create)
	v1.GET("/types/:typeId", tyh.Get)
	v1.PUT("/types/:typeId", tyh.Update)
	v1.DELETE("/types/:typeId", tyh.Delete)

	// Retention policies (stored and reported only — applied by an external runner)
	v1.GET("/retention-policies", rh.List)
	v1.GET("/artifacts/:id/retention", rh.GetForArtifact)
	v1.PUT("/artifacts/:id/retention", rh.PutForArtifact)
	v1.DELETE("/artifacts/:id/retention", rh.DeleteForArtifact)
	v1.GET("/types/:typeId/retention", rh.GetForType)
	v1.PUT("/types/:typeId/retention", rh.PutForType)
	v1.DELETE("/types/:typeId/retention", rh.DeleteForType)

	// Artifact type assignments
	v1.GET("/artifacts/:id/types", tyh.ListForArtifact)
	v1.PUT("/artifacts/:id/types/:typeId", tyh.Assign)
	v1.DELETE("/artifacts/:id/types/:typeId", tyh.Unassign)

	// Admin — maintenance endpoints
	admin := v1.Group("/admin")
	admin.GET("/artifacts/empty", adh.ListEmptyArtifacts)
	admin.DELETE("/artifacts/empty", adh.DeleteEmptyArtifacts)
	admin.DELETE("/versions", adh.BulkDeleteVersions)
	admin.GET("/versions/empty", adh.ListVersionsWithoutFiles)
	admin.DELETE("/versions/empty", adh.DeleteVersionsWithoutFiles)
	admin.GET("/artifacts/no-files", adh.ListArtifactsWithoutFiles)
	admin.DELETE("/artifacts/no-files", adh.DeleteArtifactsWithoutFiles)
	admin.GET("/files/incomplete", adh.ListIncompleteFiles)
	admin.DELETE("/files/incomplete", adh.DeleteIncompleteFiles)
	admin.GET("/types/unused", adh.ListUnusedTypes)
	admin.DELETE("/types/unused", adh.DeleteUnusedTypes)

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
