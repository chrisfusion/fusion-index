package config

import (
	"fmt"
	"os"
	"path/filepath"
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
