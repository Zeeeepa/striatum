package reads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
)

// doctorProbeKey is the fixed key the blob doctor round-trip writes
// and overwrites. Using a fixed key (instead of a timestamp suffix)
// keeps the bucket clean — repeated doctor calls overwrite a single
// probe object rather than accumulating.
const doctorProbeKey = "_striatum_doctor/last_probe.txt"

// blobDoctorBlock returns the diagnostics block reported by
// `daemon doctor` for the configured S3 backend. Three states:
//
//   - blob storage not configured (packageBlobClient nil):
//     {"configured": false}.
//   - configured but unreachable: {"configured": true,
//     "reachable": false, "errors": [...]}.
//   - configured and reachable: {"configured": true,
//     "reachable": true, ..., per-repo bucket round-trip when
//     repository_id is supplied}.
//
// The function never returns an error: every failure is captured in
// the report's "errors" slice so `daemon doctor` itself doesn't fail
// closed when blob storage is misconfigured.
func blobDoctorBlock(ctx context.Context, runner db.Runner, repositoryID string) map[string]any {
	if packageBlobClient == nil {
		return map[string]any{"configured": false}
	}

	report := map[string]any{
		"configured": true,
		"errors":     []string{},
	}
	errs := []string{}

	if err := packageBlobClient.Reachable(ctx); err != nil {
		report["reachable"] = false
		errs = append(errs, "reachable: "+err.Error())
		report["errors"] = errs
		return report
	}
	report["reachable"] = true

	if repositoryID == "" {
		return report
	}

	bucket, err := lookupRepoBlobBucketRead(ctx, runner, repositoryID)
	if err != nil {
		errs = append(errs, "lookup_repo_blob_bucket: "+err.Error())
		report["errors"] = errs
		return report
	}
	if bucket == "" {
		// Reachable daemon, configured backend, but the repo itself
		// has no blob_bucket. Not an error — operator may not have
		// run `striatum repo add <path> --apply-blob-creation` for this
		// repo yet (re-running it backfills the bucket on an
		// already-registered repo) — but worth surfacing.
		report["bucket"] = nil
		report["bucket_status"] = "not_provisioned"
		return report
	}
	report["bucket"] = bucket

	exists, err := packageBlobClient.BucketExists(ctx, bucket)
	if err != nil {
		errs = append(errs, "bucket_exists: "+err.Error())
		report["bucket_status"] = "head_failed"
		report["errors"] = errs
		return report
	}
	if !exists {
		report["bucket_status"] = "missing"
		errs = append(errs, fmt.Sprintf("bucket %q does not exist on the configured endpoint", bucket))
		report["errors"] = errs
		return report
	}
	report["bucket_exists"] = true

	probe, err := newProbePayload()
	if err != nil {
		errs = append(errs, "probe_payload: "+err.Error())
		report["errors"] = errs
		return report
	}
	started := time.Now()
	uploadedSha, err := packageBlobClient.PutBytes(ctx, bucket, doctorProbeKey, probe, "text/plain; charset=utf-8")
	if err != nil {
		errs = append(errs, "round_trip_put: "+err.Error())
		report["bucket_status"] = "put_failed"
		report["errors"] = errs
		return report
	}
	fetched, err := packageBlobClient.GetBytes(ctx, bucket, doctorProbeKey, uploadedSha)
	if err != nil {
		errs = append(errs, "round_trip_get: "+err.Error())
		report["bucket_status"] = "get_failed"
		report["errors"] = errs
		return report
	}
	if len(fetched) != len(probe) {
		errs = append(errs, fmt.Sprintf("round_trip_size_mismatch: put=%d get=%d", len(probe), len(fetched)))
		report["bucket_status"] = "round_trip_mismatch"
		report["errors"] = errs
		return report
	}
	report["bucket_status"] = "ok"
	report["round_trip_ms"] = time.Since(started).Milliseconds()
	report["round_trip_sha256"] = uploadedSha
	return report
}

// newProbePayload returns a small random payload for the doctor
// round-trip. Randomness defeats any back-end caching that might
// return a stale sha256.
func newProbePayload() ([]byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return []byte("striatum-doctor-probe " + hex.EncodeToString(buf) + "\n"), nil
}
