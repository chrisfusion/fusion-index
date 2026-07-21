package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// NewS3Client builds an *s3.Client from the same region/endpoint-override rules used
// by the server (main.go's buildStorage) — shared so the S3 prefix migration command
// (cmd/server/migrate.go) talks to S3 identically.
func NewS3Client(ctx context.Context, region, endpointOverride string) (*s3.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	var opts []func(*s3.Options)
	if endpointOverride != "" {
		ep := endpointOverride
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = &ep
			o.UsePathStyle = true
		})
	}
	return s3.NewFromConfig(awsCfg, opts...), nil
}

type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string // key prefix, e.g. "index"; empty means objects live at the bucket root
}

// NewS3Storage creates an S3-backed Storage. prefix namespaces every object key under
// this bucket (e.g. "index"), letting multiple fusion-index instances share one bucket
// without colliding. An empty prefix keeps objects at the bucket root.
func NewS3Storage(client *s3.Client, bucket, prefix string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket, prefix: strings.Trim(prefix, "/")}
}

// key returns the DB-stored relative path resolved against the configured prefix.
// The prefix is never stored in the DB, mirroring how STORAGE_FS_ROOT is applied to
// filesystem paths at read-time — this keeps DB rows portable across prefix changes.
func (s *S3Storage) key(relativePath string) string {
	return joinPrefix(s.prefix, relativePath)
}

func (s *S3Storage) Store(suggestedPath string, data io.Reader, sizeHint int64, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(suggestedPath)),
		Body:   data,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if sizeHint > 0 {
		input.ContentLength = aws.Int64(sizeHint)
	}
	if _, err := s.client.PutObject(context.Background(), input); err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}
	return suggestedPath, nil
}

func (s *S3Storage) Retrieve(storagePath string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(storagePath)),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, nil
}

func (s *S3Storage) Delete(storagePath string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(storagePath)),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}
