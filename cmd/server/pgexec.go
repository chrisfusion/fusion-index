package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "fusion-platform/fusion-index/internal/config"
	"fusion-platform/fusion-index/internal/storage"
)

// pgConnArgs returns the connection flags shared by the pg_dump invocation in
// backup.go and the psql invocation in restore.go.
func pgConnArgs(cfg *appconfig.Config) []string {
	return []string{"-h", cfg.DBHost, "-p", cfg.DBPort, "-U", cfg.DBUser, "-d", cfg.DBName}
}

// pgEnv returns the environment for the pg_dump/psql subprocess, with PGPASSWORD set
// so the password never appears in argv (visible via `ps`/`/proc` to other users on
// the same node/container). PGSSLMODE mirrors DB_SSLMODE so pg_dump/psql enforce the
// same TLS posture as the app's own pgx connections (cfg.DBURL()) — without it, these
// subprocesses would silently fall back to libpq's default "prefer" (no server
// certificate verification), even when DB_SSLMODE=verify-full is configured.
func pgEnv(cfg *appconfig.Config) []string {
	return append(os.Environ(), "PGPASSWORD="+cfg.DBPassword, "PGSSLMODE="+cfg.DBSSLMode)
}

// requireS3Backend exits the process if the backup/restore commands are run with a
// non-S3 storage backend — both need an S3 bucket to read/write backups from,
// independent of where artifact files themselves are stored.
func requireS3Backend(cfg *appconfig.Config, cmdName string) {
	if cfg.StorageBackend != "S3" {
		slog.Error(cmdName + " requires STORAGE_BACKEND=S3")
		os.Exit(1)
	}
}

// mustS3Client builds an S3 client or exits the process — shared by backup.go,
// restore.go, and migrate.go, all of which treat a failure to build the client as
// fatal and unrecoverable.
func mustS3Client(ctx context.Context, cfg *appconfig.Config) *s3.Client {
	client, err := storage.NewS3Client(ctx, cfg.AWSRegion, cfg.S3EndpointOverride)
	if err != nil {
		slog.Error("build S3 client", "error", err)
		os.Exit(1)
	}
	return client
}
