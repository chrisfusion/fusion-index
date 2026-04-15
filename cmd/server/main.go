package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	appconfig "fusion-platform/fusion-index/internal/config"
	"fusion-platform/fusion-index/internal/api"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

func main() {
	cfg := appconfig.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DBURL())
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	log.Println("database connected")

	runMigrations(cfg.DBURL())

	queries := db.New(pool)

	store, err := buildStorage(cfg)
	if err != nil {
		log.Fatalf("build storage: %v", err)
	}

	router := api.NewRouter(pool, queries, store, cfg.StorageBackend, cfg)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("starting fusion-index on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runMigrations(dbURL string) {
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("migrations applied")
}

func buildStorage(cfg *appconfig.Config) (storage.Storage, error) {
	switch cfg.StorageBackend {
	case "S3":
		awsCfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.AWSRegion),
		)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}
		opts := []func(*awss3.Options){}
		if cfg.S3EndpointOverride != "" {
			ep := cfg.S3EndpointOverride
			opts = append(opts, func(o *awss3.Options) {
				o.BaseEndpoint = &ep
				o.UsePathStyle = true
			})
		}
		client := awss3.NewFromConfig(awsCfg, opts...)
		return storage.NewS3Storage(client, cfg.S3Bucket), nil
	default: // FILESYSTEM
		return storage.NewFilesystemStorage(cfg.StorageFSRoot), nil
	}
}
