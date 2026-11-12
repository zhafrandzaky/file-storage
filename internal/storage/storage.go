// Package storage wraps an S3-compatible object store (RustFS, MinIO, AWS S3)
// behind a small API used by the HTTP layer.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/zhafrandzaky/file-storage/internal/config"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("object not found")

// Object describes a stored file.
type Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
}

// Download is an open object stream. The caller must close Body.
type Download struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// S3 is an S3-backed store bound to a single bucket.
type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 builds a client from configuration. It does not contact the server.
func NewS3(cfg config.S3) *S3 {
	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.UsePathStyle,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		// Checksums only when the operation requires them; some
		// S3-compatible servers reject the SDK's default trailing checksums.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &S3{client: client, bucket: cfg.Bucket}
}

// Bucket returns the bucket name this store writes to.
func (s *S3) Bucket() string { return s.bucket }

// EnsureBucket creates the bucket if needed, retrying while the storage
// server is still starting.
func (s *S3) EnsureBucket(ctx context.Context, attempts int, wait time.Duration) error {
	var last error
	for i := 0; i < attempts; i++ {
		if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket}); err == nil {
			return nil
		}
		_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket})
		if err == nil {
			slog.Info("bucket created", "bucket", s.bucket)
			return nil
		}
		last = err
		slog.Warn("storage not ready, retrying", "attempt", i+1, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return fmt.Errorf("ensure bucket %q: %w", s.bucket, last)
}

// Ping checks that the bucket is reachable.
func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	return err
}

// List returns all objects, newest first.
func (s *S3) List(ctx context.Context) ([]Object, error) {
	objects := []Object{}
	p := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: &s.bucket})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.Contents {
			objects = append(objects, Object{
				Key:          aws.ToString(o.Key),
				Size:         aws.ToInt64(o.Size),
				LastModified: aws.ToTime(o.LastModified),
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].LastModified.After(objects[j].LastModified) })
	return objects, nil
}

// Put stores r under key, replacing any existing object.
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

// Get opens an object for reading.
func (s *S3) Get(ctx context.Context, key string) (*Download, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, mapErr(err)
	}
	return &Download{Body: out.Body, ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}

// Delete removes an object. It returns ErrNotFound if the key does not exist.
func (s *S3) Delete(ctx context.Context, key string) error {
	if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key}); err != nil {
		return mapErr(err)
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}

func mapErr(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return ErrNotFound
		}
	}
	return err
}
