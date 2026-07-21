package storage

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MigratePrefix copies every path in paths from oldPrefix to newPrefix within bucket
// using S3 server-side CopyObject (no object data flows through this process). paths
// is the exact set of DB-known storage_path values (see ListAvailableS3FilePaths) —
// migration never lists bucket contents, so it can't touch objects fusion-index
// doesn't own even when oldPrefix is empty (bucket root).
//
// A path already present under newPrefix is skipped, so an interrupted migration can
// be safely resumed by running it again. Objects under oldPrefix are left in place;
// this is a copy, not a move.
func MigratePrefix(ctx context.Context, client *s3.Client, bucket, oldPrefix, newPrefix string, paths []string, log *slog.Logger) (copied, skipped int, err error) {
	oldPrefix = strings.Trim(oldPrefix, "/")
	newPrefix = strings.Trim(newPrefix, "/")

	for _, relativePath := range paths {
		oldKey := joinPrefix(oldPrefix, relativePath)
		newKey := joinPrefix(newPrefix, relativePath)

		// Any HeadObject error (not just "not found") is treated as "needs copying" —
		// a real problem (permissions, network) surfaces loudly from CopyObject below
		// instead of being silently swallowed here.
		if _, headErr := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(newKey),
		}); headErr == nil {
			skipped++
			continue
		}

		if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(newKey),
			CopySource: aws.String(url.QueryEscape(bucket + "/" + oldKey)),
		}); err != nil {
			return copied, skipped, fmt.Errorf("copy %s to %s: %w", oldKey, newKey, err)
		}
		copied++
		log.Info("migrated object", "old_key", oldKey, "new_key", newKey)
	}
	return copied, skipped, nil
}

func joinPrefix(prefix, relativePath string) string {
	if prefix == "" {
		return relativePath
	}
	return path.Join(prefix, relativePath)
}
