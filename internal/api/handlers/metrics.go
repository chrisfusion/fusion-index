package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/metrics"
)

type MetricsHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	cache   *metrics.Cache
}

func NewMetricsHandler(pool *pgxpool.Pool, q *db.Queries, c *metrics.Cache) *MetricsHandler {
	return &MetricsHandler{pool: pool, queries: q, cache: c}
}

func (h *MetricsHandler) Get(c *gin.Context) {
	snap, err := h.cache.Get(c.Request.Context(), h.load)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, snap)
}

func (h *MetricsHandler) load(ctx context.Context) (*metrics.Snapshot, error) {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := h.queries.WithTx(tx)

	totalArtifacts, err := q.CountRegistryArtifacts(ctx)
	if err != nil {
		return nil, err
	}
	totalVersions, err := q.CountRegistryVersions(ctx)
	if err != nil {
		return nil, err
	}
	totalTags, err := q.CountRegistryTags(ctx)
	if err != nil {
		return nil, err
	}
	fileRows, err := q.CountRegistryFilesByStatus(ctx)
	if err != nil {
		return nil, err
	}
	totalStorage, err := q.SumRegistryStorageBytes(ctx)
	if err != nil {
		return nil, err
	}
	withoutTags, err := q.CountArtifactsWithoutTags(ctx)
	if err != nil {
		return nil, err
	}
	withoutVersions, err := q.CountArtifactsWithoutVersions(ctx)
	if err != nil {
		return nil, err
	}
	typeRows, err := q.CountArtifactsByType(ctx)
	if err != nil {
		return nil, err
	}

	var filesAvailable, filesPending, filesError int64
	for _, r := range fileRows {
		switch r.Status {
		case "AVAILABLE":
			filesAvailable = r.Count
		case "PENDING":
			filesPending = r.Count
		case "ERROR":
			filesError = r.Count
		}
	}

	typeCounts := make([]metrics.TypeCount, len(typeRows))
	for i, r := range typeRows {
		typeCounts[i] = metrics.TypeCount{TypeName: r.TypeName, Count: r.ArtifactCount}
	}

	return &metrics.Snapshot{
		CachedAt:                 time.Now(),
		TotalArtifacts:           totalArtifacts,
		TotalVersions:            totalVersions,
		TotalTags:                totalTags,
		FilesAvailable:           filesAvailable,
		FilesPending:             filesPending,
		FilesError:               filesError,
		TotalStorageBytes:        totalStorage,
		ArtifactsWithoutTags:     withoutTags,
		ArtifactsWithoutVersions: withoutVersions,
		TypeCounts:               typeCounts,
	}, nil
}
