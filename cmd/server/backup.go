package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"time"

	appconfig "fusion-platform/fusion-index/internal/config"
	"fusion-platform/fusion-index/internal/storage"
)

// runBackupDB is invoked as `fusion-index backup-db` by the chart's daily CronJob
// (deployment/templates/postgresql-backup-cronjob.yaml). It runs pg_dump against the
// configured database, gzips the output, and streams it directly into S3 as it's
// produced — no local temp file, so backup size isn't bounded by the Job's ephemeral
// disk.
//
// pg_dump runs with --clean --if-exists so the resulting dump is restorable into
// either a freshly created (empty, no schema) database or one that already has
// fusion-index's migrated-but-empty schema — DROP ... IF EXISTS makes both cases
// idempotent. See restore.go for the corresponding restore path and its safety guard.
func runBackupDB() {
	cfg := appconfig.Load()
	setupLogger(cfg)
	ctx := context.Background()

	requireS3Backend(cfg, "backup-db")

	filename := fmt.Sprintf("backup-%s.sql.gz", time.Now().UTC().Format("20060102T150405Z"))
	key := path.Join(cfg.S3BackupPrefix, filename)

	pr, pw := io.Pipe()
	gz := gzip.NewWriter(pw)

	args := append(pgConnArgs(cfg), "--format=plain", "--clean", "--if-exists", "--no-owner", "--no-privileges")
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = pgEnv(cfg)
	cmd.Stdout = gz
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// dumpErr carries the pg_dump/gzip-specific failure (if any), sent before the
	// pipe is closed either way — so it's always ready by the time UploadStream
	// below returns, letting us log the *actual* cause (pg_dump vs. S3) instead of
	// a generic upload error when pg_dump is what really failed.
	dumpErr := make(chan error, 1)
	go func() {
		if runErr := cmd.Run(); runErr != nil {
			err := fmt.Errorf("pg_dump: %w: %s", runErr, stderr.String())
			dumpErr <- err
			pw.CloseWithError(err)
			return
		}
		if closeErr := gz.Close(); closeErr != nil {
			err := fmt.Errorf("gzip close: %w", closeErr)
			dumpErr <- err
			pw.CloseWithError(err)
			return
		}
		dumpErr <- nil
		pw.Close()
	}()

	counting := &countingReader{r: pr}
	s3Client := mustS3Client(ctx, cfg)
	uploadErr := storage.UploadStream(ctx, s3Client, cfg.S3Bucket, key, counting)

	if err := <-dumpErr; err != nil {
		slog.Error("pg_dump failed", "key", key, "error", err)
		os.Exit(1)
	}
	if uploadErr != nil {
		slog.Error("upload backup", "key", key, "error", uploadErr)
		os.Exit(1)
	}

	slog.Info("database backup complete", "bucket", cfg.S3Bucket, "key", key, "bytes", counting.n)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
