package integration

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"fusion-platform/fusion-index/internal/api"
	"fusion-platform/fusion-index/internal/config"
	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/storage"
)

var testServer *httptest.Server

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		tcpostgres.WithDatabase("fusion_test"),
		tcpostgres.WithUsername("fusion"),
		tcpostgres.WithPassword("fusion"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Printf("terminate postgres container: %v", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	// Find migrations dir relative to this test file
	migrationsPath, err := filepath.Abs("../../migrations")
	if err != nil {
		log.Fatalf("resolve migrations path: %v", err)
	}
	migrateURL := fmt.Sprintf("file://%s", migrationsPath)

	mg, err := migrate.New(migrateURL, connStr)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	if err := mg.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("run migrations: %v", err)
	}
	mg.Close()

	artifactDir, err := os.MkdirTemp("", "fusion-index-test-*")
	if err != nil {
		log.Fatalf("create artifact dir: %v", err)
	}
	defer os.RemoveAll(artifactDir)

	queries := db.New(pool)
	store := storage.NewFilesystemStorage(artifactDir)

	router := api.NewRouter(pool, queries, store, "FILESYSTEM", &config.Config{})
	testServer = httptest.NewServer(router)
	defer testServer.Close()

	os.Exit(m.Run())
}
