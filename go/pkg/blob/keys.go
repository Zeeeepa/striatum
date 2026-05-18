// Package blob provides S3-compatible blob storage for striatum
// artifact bodies, per RFC 0072.
//
// The split between this package and the daemon's other storage code:
//
//   - Daemon-owned PostgreSQL holds the authoritative reference (artifact
//     id, kind, byline, run/job linkage, content sha256 anchor) — the
//     "what was published, by whom, with what hash."
//   - S3-compatible blob storage holds the artifact body — the bytes
//     themselves, at the canonical key for the run/job/logical-name.
//
// Each registered target repository gets its own bucket; per-repo
// bucket-level isolation matches the existing repository_id-scoped data
// boundary in PG. The bucket name is recorded on
// striatumd.repositories.blob_bucket at adopt time.
package blob

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxBucketNameLength is the AWS S3 / MinIO bucket name length cap.
const MaxBucketNameLength = 63

// MinBucketNameLength is the AWS S3 / MinIO bucket name length floor.
const MinBucketNameLength = 3

// DefaultBucketPrefix is the prefix striatum uses when auto-generating
// a bucket name for a registered repository. Operators can override via
// STRIATUM_BLOB_BUCKET_PREFIX.
const DefaultBucketPrefix = "striatum-"

// bucketNameRegex enforces AWS S3 / MinIO bucket naming rules:
//   - lowercase letters, digits, hyphens
//   - cannot start or end with a hyphen
//   - cannot have consecutive hyphens
//   - 3-63 chars
var bucketNameRegex = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|-(?:[a-z0-9]))*$`)

// ValidateBucketName returns nil when name is a valid S3 / MinIO bucket
// name. Errors are returned for length, character, and shape violations.
//
// The function does not check name uniqueness against the configured S3
// endpoint; that is a runtime check, not a syntactic one.
func ValidateBucketName(name string) error {
	n := len(name)
	if n < MinBucketNameLength || n > MaxBucketNameLength {
		return fmt.Errorf("bucket name length %d must be %d-%d chars", n, MinBucketNameLength, MaxBucketNameLength)
	}
	if !bucketNameRegex.MatchString(name) {
		return fmt.Errorf("bucket name %q is not valid (lowercase, digits, hyphens; no leading/trailing/double hyphens)", name)
	}
	return nil
}

// DefaultBucketName returns the bucket name striatum will use for the
// given repositoryID when --blob-bucket is not supplied. The format is
// "<prefix><sanitized-repository-id>", clipped to 63 chars.
//
// Sanitization replaces every character that is not [a-z0-9-] with a
// hyphen and collapses runs of hyphens to a single hyphen.
func DefaultBucketName(prefix string, repositoryID string) string {
	if prefix == "" {
		prefix = DefaultBucketPrefix
	}
	sanitized := sanitizeForBucket(strings.ToLower(repositoryID))
	if sanitized == "" {
		sanitized = "repo"
	}
	candidate := prefix + sanitized
	if len(candidate) > MaxBucketNameLength {
		candidate = candidate[:MaxBucketNameLength]
	}
	return strings.Trim(candidate, "-")
}

func sanitizeForBucket(input string) string {
	var b strings.Builder
	previousHyphen := false
	for _, r := range input {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !valid {
			r = '-'
		}
		if r == '-' {
			if previousHyphen || b.Len() == 0 {
				continue
			}
			previousHyphen = true
		} else {
			previousHyphen = false
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), "-")
}

// ArtifactKey returns the canonical blob key for a published artifact.
// The key is path-shaped, browsable in the bucket console, and does not
// encode the sha256 — integrity is verified via the PG row on read.
//
// runID, jobID, and logicalName must be non-empty; ArtifactKey panics
// if any is empty because every active publish path has them set.
func ArtifactKey(runID, jobID, logicalName string) string {
	if runID == "" || jobID == "" || logicalName == "" {
		panic(fmt.Sprintf("blob.ArtifactKey requires non-empty parts (run=%q job=%q name=%q)", runID, jobID, logicalName))
	}
	return fmt.Sprintf("runs/%s/jobs/%s/artifacts/%s", runID, jobID, logicalName)
}

// HistoricalDogfoodKey returns the canonical blob key for a file
// migrated from the docs/dogfood/<id>/ tree. relPath is the file's
// path relative to the dogfood run's root directory.
//
// dogfoodID is the run directory name (e.g., "001-v2", "054b"); it is
// preserved verbatim because the existing docs/dogfood/HISTORICAL.md
// references those identifiers.
func HistoricalDogfoodKey(dogfoodID, relPath string) string {
	if dogfoodID == "" || relPath == "" {
		panic(fmt.Sprintf("blob.HistoricalDogfoodKey requires non-empty parts (id=%q rel=%q)", dogfoodID, relPath))
	}
	clean := strings.TrimPrefix(strings.TrimPrefix(relPath, "/"), "./")
	return fmt.Sprintf("dogfood-historical/%s/%s", dogfoodID, clean)
}
