package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client is the daemon's gateway to S3-compatible blob storage.
//
// Concurrent-safe: the underlying minio-go client carries its own
// connection pool; Client embeds it without extra locking. Methods
// accept a context.Context so callers can deadline reads/writes
// per-RPC.
type Client struct {
	cfg  *Config
	mc   *minio.Client
}

// New returns a new Client backed by minio-go. Returns an error if the
// configuration is invalid or the underlying client cannot be
// constructed; does not attempt any S3 round-trip — call Reachable for
// that.
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("blob.New: nil config")
	}
	if cfg.Endpoint == "" {
		return nil, errors.New("blob.New: empty endpoint")
	}
	opts := &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		Region:       cfg.Region,
		BucketLookup: minio.BucketLookupAuto,
	}
	if cfg.PathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}
	mc, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("blob.New: %w", err)
	}
	return &Client{cfg: cfg, mc: mc}, nil
}

// Reachable returns nil if the S3 endpoint accepts the configured
// credentials and the bucket-list call succeeds. Errors are
// authentication, network, or endpoint mismatches. Does not require
// any specific bucket to exist.
func (c *Client) Reachable(ctx context.Context) error {
	_, err := c.mc.ListBuckets(ctx)
	return err
}

// BucketExists reports whether the named bucket exists on the
// configured endpoint. Distinct from Reachable: a configured endpoint
// can be reachable without the per-repo bucket having been provisioned
// yet.
func (c *Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return c.mc.BucketExists(ctx, bucket)
}

// CreateBucket creates the named bucket on the configured endpoint
// using the configured region. The bucket is created with default
// (private) ACL; versioning is not enabled. Returns nil if the bucket
// already exists.
//
// CreateBucket is intended for adopt-time provisioning behind an
// explicit --apply-blob-creation flag, mirroring
// `daemon doctor --apply-migrations`. Production publish paths do not
// create buckets implicitly.
func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	if err := ValidateBucketName(bucket); err != nil {
		return fmt.Errorf("blob.CreateBucket: %w", err)
	}
	exists, err := c.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("blob.CreateBucket: head: %w", err)
	}
	if exists {
		return nil
	}
	return c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: c.cfg.Region})
}

// PutBytes uploads body to bucket at key with the given content type.
// The sha256 of body is computed before upload and verified against
// the post-upload object via a HEAD round-trip. Returns the sha256 hex
// digest on success.
//
// PutBytes is the canonical publish path: callers (the daemon's
// artifact.publish handler) pass the already-validated artifact body
// in; PutBytes does not validate front-matter, byline, or any other
// artifact-shape contract — that runs upstream.
func (c *Client) PutBytes(ctx context.Context, bucket, key string, body []byte, contentType string) (string, error) {
	if bucket == "" || key == "" {
		return "", fmt.Errorf("blob.PutBytes: empty bucket=%q or key=%q", bucket, key)
	}
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	reader := bytes.NewReader(body)
	_, err := c.mc.PutObject(ctx, bucket, key, reader, int64(len(body)), minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: map[string]string{"X-Striatum-Sha256": hexSum},
	})
	if err != nil {
		return "", fmt.Errorf("blob.PutBytes: put: %w", err)
	}
	stat, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("blob.PutBytes: stat: %w", err)
	}
	if stat.Size != int64(len(body)) {
		return "", fmt.Errorf("blob.PutBytes: size mismatch: put=%d stat=%d", len(body), stat.Size)
	}
	return hexSum, nil
}

// GetBytes fetches the object at bucket/key, verifies its sha256
// against the expected value, and returns the body. A non-empty
// expectedSha256 is required: a blob fetch without an integrity anchor
// is not part of the artifact contract.
//
// Errors are returned for not-found, network failures, sha256
// mismatches, and read failures. Callers may log mismatches as audit
// events; this layer does not.
func (c *Client) GetBytes(ctx context.Context, bucket, key, expectedSha256 string) ([]byte, error) {
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("blob.GetBytes: empty bucket=%q or key=%q", bucket, key)
	}
	if expectedSha256 == "" {
		return nil, fmt.Errorf("blob.GetBytes: empty expectedSha256 for %s/%s", bucket, key)
	}
	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("blob.GetBytes: get: %w", err)
	}
	defer obj.Close()
	body, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("blob.GetBytes: read: %w", err)
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != expectedSha256 {
		return nil, fmt.Errorf("blob.GetBytes: sha256 mismatch: expected=%s got=%s", expectedSha256, got)
	}
	return body, nil
}

// HeadObject returns the size and content type of bucket/key without
// fetching the body. Used by the daemon doctor round-trip check and
// the artifact listing flows.
func (c *Client) HeadObject(ctx context.Context, bucket, key string) (size int64, contentType string, err error) {
	stat, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, "", err
	}
	return stat.Size, stat.ContentType, nil
}
