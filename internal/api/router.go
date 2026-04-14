package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api/handlers"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

func NewRouter(pool *pgxpool.Pool, q *db.Queries, s storage.Storage, storageBackend string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

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

	th := handlers.NewTemplateHandler(pool, q)
	jh := handlers.NewJobHandler(pool, q)
	ah := handlers.NewArtifactHandler(pool, q, s, storageBackend)

	v1 := r.Group("/api/v1")

	// Templates
	v1.GET("/templates", th.List)
	v1.POST("/templates", th.Create)
	v1.GET("/templates/:id", th.Get)
	v1.PUT("/templates/:id", th.Update)
	v1.DELETE("/templates/:id", th.Delete)
	v1.GET("/templates/:id/versions", th.ListVersions)
	v1.POST("/templates/:id/versions", th.PublishVersion)
	v1.GET("/templates/:id/versions/:versionNumber", th.GetVersion)

	// Jobs
	v1.GET("/jobs", jh.List)
	v1.POST("/jobs", jh.Create)
	v1.GET("/jobs/:id", jh.Get)
	v1.PUT("/jobs/:id", jh.Update)
	v1.DELETE("/jobs/:id", jh.Delete)
	v1.GET("/jobs/:id/versions", jh.ListVersions)
	v1.POST("/jobs/:id/versions", jh.PublishVersion)
	v1.GET("/jobs/:id/versions/:versionNumber", jh.GetVersion)

	// Artifacts scoped to job version
	v1.GET("/jobs/:id/versions/:versionNumber/artifacts", ah.ListForJobVersion)
	v1.POST("/jobs/:id/versions/:versionNumber/artifacts", ah.Upload)

	// Artifacts by ID
	v1.GET("/artifacts", ah.ListAll)
	v1.GET("/artifacts/:id", ah.Get)
	v1.GET("/artifacts/:id/download", ah.Download)
	v1.DELETE("/artifacts/:id", ah.Delete)

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
