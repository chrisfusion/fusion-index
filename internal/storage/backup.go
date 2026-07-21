package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadStream uploads body to bucket/key via multipart upload, splitting into parts
// as needed. Used for DB backups (cmd/server/backup.go), where the gzipped pg_dump
// size isn't known upfront — unlike artifact file uploads, which have Content-Length
// from the HTTP request and use S3Storage.Store's plain PutObject instead.
//
// A failed upload leaves no partial object visible under key: S3 only makes a
// multipart upload's data visible on successful completion: manager.Uploader aborts
// automatically on error, so callers never see a corrupt/truncated backup.
func UploadStream(ctx context.Context, client *s3.Client, bucket, key string, body io.Reader) error {
	uploader := manager.NewUploader(client)
	if _, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}); err != nil {
		return fmt.Errorf("s3 upload stream: %w", err)
	}
	return nil
}

// DownloadStream returns a reader for bucket/key — used by restore-db to stream a
// backup object without buffering it to local disk first.
func DownloadStream(ctx context.Context, client *s3.Client, bucket, key string) (io.ReadCloser, error) {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, nil
}

// FindLatestBackupKey returns the most recent object key under prefix, chosen by
// lexicographic order — safe because backup keys embed a fixed-width, zero-padded
// UTC timestamp (see cmd/server/backup.go), which sorts the same lexicographically
// and chronologically.
func FindLatestBackupKey(ctx context.Context, client *s3.Client, bucket, prefix string) (string, error) {
	var latest string
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("list backups under %s: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && *obj.Key > latest {
				latest = *obj.Key
			}
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no backups found under %s", prefix)
	}
	return latest, nil
}
