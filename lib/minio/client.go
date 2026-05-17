package minio

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	UseSSL          bool
	PublicBucket    string
	PrivateBucket   string
	PublicBaseURL   string // base URL for constructing public object URLs (e.g. http://minio:9000)
}

type Client struct {
	mc            *minio.Client
	publicBucket  string
	privateBucket string
	publicBaseURL string
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: failed to create client: %w", err)
	}

	c := &Client{
		mc:            mc,
		publicBucket:  cfg.PublicBucket,
		privateBucket: cfg.PrivateBucket,
		publicBaseURL: cfg.PublicBaseURL,
	}

	if err := c.ensureBuckets(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) ensureBuckets(ctx context.Context) error {
	for _, bucket := range []string{c.publicBucket, c.privateBucket} {
		exists, err := c.mc.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("minio: checking bucket %q: %w", bucket, err)
		}
		if !exists {
			if err := c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("minio: creating bucket %q: %w", bucket, err)
			}
		}
	}

	// Set public-read policy on the public bucket
	policy := fmt.Sprintf(`{
		"Version":"2012-10-17",
		"Statement":[{
			"Effect":"Allow",
			"Principal":{"AWS":["*"]},
			"Action":["s3:GetObject"],
			"Resource":["arn:aws:s3:::%s/*"]
		}]
	}`, c.publicBucket)

	if err := c.mc.SetBucketPolicy(ctx, c.publicBucket, policy); err != nil {
		return fmt.Errorf("minio: setting public policy on %q: %w", c.publicBucket, err)
	}

	return nil
}

// UploadPublic uploads a file to the public bucket and returns a permanent URL.
func (c *Client) UploadPublic(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	_, err := c.mc.PutObject(ctx, c.publicBucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio: upload to public bucket: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s", c.publicBaseURL, c.publicBucket, objectKey), nil
}

// UploadPrivate uploads a file to the private bucket and returns the object key (not a URL).
func (c *Client) UploadPrivate(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	_, err := c.mc.PutObject(ctx, c.privateBucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio: upload to private bucket: %w", err)
	}

	return objectKey, nil
}

// GetPresignedURL generates a time-limited presigned URL for a private object.
func (c *Client) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.privateBucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("minio: presigning %q: %w", objectKey, err)
	}
	return u.String(), nil
}

// DeleteObject removes an object from the specified bucket.
func (c *Client) DeleteObject(ctx context.Context, bucket, objectKey string) error {
	return c.mc.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

func (c *Client) PublicBucket() string  { return c.publicBucket }
func (c *Client) PrivateBucket() string { return c.privateBucket }
