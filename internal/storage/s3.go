package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3Storage(client *s3.Client, bucket string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket}
}

func (s *S3Storage) Store(suggestedPath string, data io.Reader, sizeHint int64, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(suggestedPath),
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
		Key:    aws.String(storagePath),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, nil
}

func (s *S3Storage) Delete(storagePath string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storagePath),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}
