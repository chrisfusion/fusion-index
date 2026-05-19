package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port               string
	DBHost             string
	DBPort             string
	DBName             string
	DBUser             string
	DBPassword         string
	DBSSLMode          string
	StorageBackend     string
	StorageFSRoot      string
	S3Bucket           string
	AWSRegion          string
	S3EndpointOverride string

	// Kubernetes SA token authentication
	AuthEnabled    bool
	AuthAudience   string   // if non-empty, validated against token audience claim
	AuthAllowedSAs []string // "namespace/name" pairs; empty = allow any valid SA token

	LogLevel  string // "debug" | "info" | "warn" | "error"
	LogFormat string // "json" | "text"
}

func Load() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Port:               getEnv("HTTP_PORT", "8080"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBName:             getEnv("DB_NAME", "fusion_index"),
		DBUser:             getEnv("DB_USERNAME", "fusion"),
		DBPassword:         getEnv("DB_PASSWORD", "fusion"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		StorageBackend:     getEnv("STORAGE_BACKEND", "FILESYSTEM"),
		StorageFSRoot:      getEnv("STORAGE_FS_ROOT", filepath.Join(home, ".fusion-index", "artifacts")),
		S3Bucket:           getEnv("S3_BUCKET", "fusion-index-artifacts"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		S3EndpointOverride: getEnv("S3_ENDPOINT_OVERRIDE", ""),
		AuthEnabled:        getEnv("AUTH_ENABLED", "false") == "true",
		AuthAudience:       getEnv("AUTH_AUDIENCE", ""),
		AuthAllowedSAs:     splitCSV(getEnv("AUTH_ALLOWED_SA", "")),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		LogFormat:          getEnv("LOG_FORMAT", "json"),
	}
}

func (c *Config) DBURL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
