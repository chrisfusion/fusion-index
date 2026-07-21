package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	appconfig "fusion-platform/fusion-index/internal/config"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/k8sclient"
	"fusion-platform/fusion-index/internal/storage"
)

const s3MigrationPrefixKey = "prefix"

// runMigrateS3Prefix is invoked as `fusion-index migrate-s3-prefix` by the chart's
// pre-install/pre-upgrade hook Job (deployment/templates/s3-prefix-migration-job.yaml).
// It compares the currently configured S3_PREFIX against the last-applied prefix
// recorded in a marker ConfigMap and, if they differ, server-side-copies every known
// S3 object from the old prefix to the new one. See internal/storage/migrate.go for
// why this is DB-driven rather than an S3 bucket listing.
func runMigrateS3Prefix() {
	cfg := appconfig.Load()
	setupLogger(cfg)
	ctx := context.Background()

	if cfg.StorageBackend != "S3" {
		slog.Info("storage backend is not S3, nothing to migrate")
		return
	}

	namespace, err := k8sclient.ReadNamespace()
	if err != nil {
		slog.Error("read own namespace", "error", err)
		os.Exit(1)
	}
	markerName := os.Getenv("S3_MIGRATION_CONFIGMAP")
	if markerName == "" {
		slog.Error("S3_MIGRATION_CONFIGMAP is required for migrate-s3-prefix")
		os.Exit(1)
	}

	k8sHTTP, err := k8sclient.NewHTTPClient()
	if err != nil {
		slog.Error("build k8s client", "error", err)
		os.Exit(1)
	}
	token, err := k8sclient.ReadToken()
	if err != nil {
		slog.Error("read own SA token", "error", err)
		os.Exit(1)
	}

	data, resourceVersion, found, err := k8sclient.GetConfigMapData(k8sHTTP, token, namespace, markerName)
	if err != nil {
		slog.Error("read migration marker", "error", err)
		os.Exit(1)
	}
	if !found {
		if err := k8sclient.CreateConfigMapData(k8sHTTP, token, namespace, markerName, map[string]string{s3MigrationPrefixKey: cfg.S3Prefix}); err != nil {
			slog.Error("create migration marker", "error", err)
			os.Exit(1)
		}
		slog.Info("no prior migration marker found — recorded current prefix, nothing to migrate", "prefix", cfg.S3Prefix)
		return
	}

	oldPrefix := data[s3MigrationPrefixKey]
	if oldPrefix == cfg.S3Prefix {
		slog.Info("S3 prefix unchanged, nothing to migrate", "prefix", cfg.S3Prefix)
		return
	}

	slog.Info("S3 prefix changed, migrating objects", "old_prefix", oldPrefix, "new_prefix", cfg.S3Prefix)

	pool, err := pgxpool.New(ctx, cfg.DBURL())
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	queries := db.New(pool)

	paths, err := queries.ListAvailableS3FilePaths(ctx)
	if err != nil {
		slog.Error("list S3 file paths", "error", err)
		os.Exit(1)
	}

	s3Client := mustS3Client(ctx, cfg)

	copied, skipped, err := storage.MigratePrefix(ctx, s3Client, cfg.S3Bucket, oldPrefix, cfg.S3Prefix, paths, slog.Default())
	if err != nil {
		slog.Error("migrate S3 prefix — marker left unchanged, safe to retry (already-copied objects are skipped)",
			"old_prefix", oldPrefix, "new_prefix", cfg.S3Prefix, "copied", copied, "skipped", skipped, "error", err)
		os.Exit(1)
	}

	if err := k8sclient.UpdateConfigMapData(k8sHTTP, token, namespace, markerName, resourceVersion, map[string]string{s3MigrationPrefixKey: cfg.S3Prefix}); err != nil {
		slog.Error("update migration marker after successful copy — objects are now duplicated under both prefixes, retry will just skip already-copied ones", "error", err)
		os.Exit(1)
	}

	slog.Info("S3 prefix migration complete", "old_prefix", oldPrefix, "new_prefix", cfg.S3Prefix, "copied", copied, "skipped", skipped, "total", len(paths))
}
