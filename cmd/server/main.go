package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"fusion-platform/fusion-index/internal/api"
	appconfig "fusion-platform/fusion-index/internal/config"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate-s3-prefix":
			runMigrateS3Prefix()
			return
		case "backup-db":
			runBackupDB()
			return
		case "restore-db":
			runRestoreDB()
			return
		}
	}
	runServer()
}

func runServer() {
	cfg := appconfig.Load()
	setupLogger(cfg)

	pool, err := pgxpool.New(context.Background(), cfg.DBURL())
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	runMigrations(cfg.DBURL())

	queries := db.New(pool)

	store, err := buildStorage(cfg)
	if err != nil {
		slog.Error("build storage", "error", err)
		os.Exit(1)
	}

	router := api.NewRouter(pool, queries, store, cfg.StorageBackend, cfg)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("starting fusion-index", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(cfg *appconfig.Config) {
	var level slog.Level
	unknownLevel := false
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info", "":
		level = slog.LevelInfo
	default:
		level = slog.LevelInfo
		unknownLevel = true
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))

	if unknownLevel {
		slog.Warn("unrecognised LOG_LEVEL, defaulting to info", "value", cfg.LogLevel)
	}
}

func runMigrations(dbURL string) {
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		slog.Error("create migrator", "error", err)
		os.Exit(1)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")
}

func buildStorage(cfg *appconfig.Config) (storage.Storage, error) {
	switch cfg.StorageBackend {
	case "S3":
		client, err := storage.NewS3Client(context.Background(), cfg.AWSRegion, cfg.S3EndpointOverride)
		if err != nil {
			return nil, err
		}
		return storage.NewS3Storage(client, cfg.S3Bucket, cfg.S3Prefix), nil
	default: // FILESYSTEM
		return storage.NewFilesystemStorage(cfg.StorageFSRoot), nil
	}
}
