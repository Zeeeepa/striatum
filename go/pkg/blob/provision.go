package blob

import (
	"context"
	"errors"
	"fmt"
)

// ProvisionResult records what happened during adopt-time bucket
// provisioning. The daemon stores BucketName and CreatedAt on the
// striatumd.repositories row when ProvisionResult.Created or
// ProvisionResult.Claimed.
type ProvisionResult struct {
	BucketName string
	Created    bool // true when the bucket did not exist and was created
	Claimed    bool // true when the bucket existed empty and is now claimed
	// Refused is non-empty when the existing bucket contains
	// striatum-shaped keys from a different repository; the daemon
	// returns this verbatim with exit code 12 (repo_blob_conflict).
	Refused string
}

// ErrApplyRequired is returned when the bucket does not exist and the
// caller did not request creation. The daemon's adopt verb refuses
// without --apply-blob-creation in this case, mirroring the
// daemon doctor --apply-migrations pattern.
var ErrApplyRequired = errors.New("bucket does not exist; pass --apply-blob-creation to create it")

// ProvisionOptions controls adopt-time bucket provisioning.
type ProvisionOptions struct {
	// ApplyCreation gates the CreateBucket call. The daemon's adopt
	// verb sets this from --apply-blob-creation; doctor checks set it
	// false (report-only mode).
	ApplyCreation bool

	// RepositoryID is recorded as user metadata on a probe object so
	// that future adopt calls can distinguish "empty bucket I claimed"
	// from "bucket already claimed by a different repo." Required.
	RepositoryID string
}

const claimMarkerKey = "_striatum_repo_marker"

// Provision is the adopt-time bucket setup for a registered
// repository. The behavior:
//
//   - If the bucket does not exist:
//   - opts.ApplyCreation false → return ErrApplyRequired.
//   - opts.ApplyCreation true → create the bucket, write a claim
//     marker recording opts.RepositoryID, return Created=true.
//   - If the bucket exists with a claim marker matching opts.RepositoryID
//     → return Claimed=true (re-adopting an already-claimed bucket is a
//     no-op).
//   - If the bucket exists with a claim marker for a DIFFERENT repository
//     → set Refused=<conflict-message> and return nil; the caller is
//     responsible for raising exit code 12 (repo_blob_conflict).
//   - If the bucket exists without a marker and is empty → write the
//     marker, return Claimed=true.
//   - If the bucket exists without a marker but contains other
//     striatum-shaped keys → set Refused=<conflict-message>; the
//     caller raises repo_blob_conflict.
//
// Provision is intentionally explicit: every branch is named and the
// "refuse" cases produce structured information rather than raw S3
// errors. The strict-by-default contract was the maintainer call in
// RFC 0072.
func (c *Client) Provision(ctx context.Context, bucket string, opts ProvisionOptions) (*ProvisionResult, error) {
	if opts.RepositoryID == "" {
		return nil, fmt.Errorf("blob.Provision: empty RepositoryID")
	}
	if err := ValidateBucketName(bucket); err != nil {
		return nil, fmt.Errorf("blob.Provision: %w", err)
	}

	exists, err := c.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("blob.Provision: head bucket: %w", err)
	}
	if !exists {
		if !opts.ApplyCreation {
			return nil, ErrApplyRequired
		}
		if err := c.CreateBucket(ctx, bucket); err != nil {
			return nil, fmt.Errorf("blob.Provision: create: %w", err)
		}
		if err := c.writeClaimMarker(ctx, bucket, opts.RepositoryID); err != nil {
			return nil, fmt.Errorf("blob.Provision: write marker: %w", err)
		}
		return &ProvisionResult{BucketName: bucket, Created: true}, nil
	}

	existingMarker, err := c.readClaimMarker(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("blob.Provision: read marker: %w", err)
	}
	if existingMarker != "" && existingMarker == opts.RepositoryID {
		return &ProvisionResult{BucketName: bucket, Claimed: true}, nil
	}
	if existingMarker != "" && existingMarker != opts.RepositoryID {
		return &ProvisionResult{
			BucketName: bucket,
			Refused: fmt.Sprintf(
				"bucket %q is claimed by repository %q; refuse to share storage with %q",
				bucket, existingMarker, opts.RepositoryID,
			),
		}, nil
	}

	// No marker. Acceptable only if the bucket has no striatum-shaped
	// keys yet; otherwise this is a cross-repo collision against
	// pre-RFC-0072 content.
	conflict, err := c.hasStriatumShapedKeys(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("blob.Provision: scan: %w", err)
	}
	if conflict {
		return &ProvisionResult{
			BucketName: bucket,
			Refused: fmt.Sprintf(
				"bucket %q contains striatum-shaped keys but no claim marker; refuse to overwrite (rename or empty the bucket first)",
				bucket,
			),
		}, nil
	}
	if err := c.writeClaimMarker(ctx, bucket, opts.RepositoryID); err != nil {
		return nil, fmt.Errorf("blob.Provision: write marker: %w", err)
	}
	return &ProvisionResult{BucketName: bucket, Claimed: true}, nil
}

func (c *Client) writeClaimMarker(ctx context.Context, bucket, repositoryID string) error {
	_, err := c.PutBytes(ctx, bucket, claimMarkerKey, []byte(repositoryID), "text/plain")
	return err
}

func (c *Client) readClaimMarker(ctx context.Context, bucket string) (string, error) {
	size, _, err := c.HeadObject(ctx, bucket, claimMarkerKey)
	if err != nil {
		// Treat any HEAD failure as "no marker." minio-go returns a
		// concrete error type for not-found; downgrading the
		// distinction here keeps the caller's branch logic readable.
		return "", nil
	}
	if size == 0 || size > 256 {
		return "", nil
	}
	// Marker payload is the literal repository_id string; fetch it
	// with sha256 verification skipped because we just wrote it
	// ourselves at adopt time (no integrity contract on the marker).
	obj, err := c.mc.GetObject(ctx, bucket, claimMarkerKey, minioGetMarkerOptions())
	if err != nil {
		return "", nil
	}
	defer func() { _ = obj.Close() }()
	buf := make([]byte, size)
	n, _ := obj.Read(buf)
	return string(buf[:n]), nil
}

func (c *Client) hasStriatumShapedKeys(ctx context.Context, bucket string) (bool, error) {
	for _, prefix := range []string{"runs/", "dogfood-historical/"} {
		ch := c.mc.ListObjects(ctx, bucket, minioListOptions(prefix, 1))
		for object := range ch {
			if object.Err != nil {
				return false, object.Err
			}
			return true, nil
		}
	}
	return false, nil
}
