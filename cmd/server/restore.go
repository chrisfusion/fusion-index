package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	appconfig "fusion-platform/fusion-index/internal/config"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

// pgUndefinedTable is the SQLSTATE Postgres returns when a queried table doesn't
// exist — used by targetHasData to tell "no schema yet" (safe to restore) apart from
// a real query failure.
const pgUndefinedTable = "42P01"

// runRestoreDB is invoked as `fusion-index restore-db` — a manual, on-demand disaster
// recovery operation, deliberately not wired to any automatic Helm trigger (see
// CLAUDE.md "Helm — PostgreSQL Backup"). Restores the backup named by
// RESTORE_BACKUP_KEY, or the most recent one under S3_BACKUP_PREFIX if unset, by
// streaming it through gunzip into psql. Refuses to run against a database that
// already has data unless RESTORE_FORCE=true.
func runRestoreDB() {
	cfg := appconfig.Load()
	setupLogger(cfg)
	ctx := context.Background()

	requireS3Backend(cfg, "restore-db")

	pool, err := pgxpool.New(ctx, cfg.DBURL())
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	queries := db.New(pool)

	hasData, err := targetHasData(ctx, queries)
	if err != nil {
		slog.Error("check target database state", "error", err)
		os.Exit(1)
	}
	if hasData && os.Getenv("RESTORE_FORCE") != "true" {
		slog.Error("target database already has data — refusing to restore; set RESTORE_FORCE=true to overwrite anyway")
		os.Exit(1)
	}

	s3Client := mustS3Client(ctx, cfg)

	key := os.Getenv("RESTORE_BACKUP_KEY")
	if key == "" {
		key, err = storage.FindLatestBackupKey(ctx, s3Client, cfg.S3Bucket, cfg.S3BackupPrefix)
		if err != nil {
			slog.Error("find latest backup", "error", err)
			os.Exit(1)
		}
	}

	rc, err := storage.DownloadStream(ctx, s3Client, cfg.S3Bucket, key)
	if err != nil {
		slog.Error("download backup", "key", key, "error", err)
		os.Exit(1)
	}
	defer rc.Close()

	gz, err := gzip.NewReader(rc)
	if err != nil {
		slog.Error("open gzip stream", "key", key, "error", err)
		os.Exit(1)
	}
	defer gz.Close()

	args := append(pgConnArgs(cfg), "-v", "ON_ERROR_STOP=1", "-f", "-")
	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = pgEnv(cfg)
	cmd.Stdin = gz
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	slog.Info("restoring database from backup", "bucket", cfg.S3Bucket, "key", key)
	if err := cmd.Run(); err != nil {
		slog.Error("psql restore failed", "key", key, "error", err, "stderr", stderr.String())
		os.Exit(1)
	}

	slog.Info("database restore complete", "key", key)
}

// targetHasData reports whether the target database already has artifacts — used to
// block an accidental restore over live data. A missing schema (fresh database, no
// migrations run yet) is treated as "no data", not an error, since that's exactly the
// expected pre-restore state for a from-scratch disaster recovery.
func targetHasData(ctx context.Context, queries *db.Queries) (bool, error) {
	count, err := queries.CountRegistryArtifacts(ctx)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
			return false, nil
		}
		return false, err
	}
	return count > 0, nil
}
