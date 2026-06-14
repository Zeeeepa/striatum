package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/jackc/pgx/v5"
)

// RFC 0125 P0-3 (GH #285): the body-reconstructability completion gate.
//
// RFC 0118 gates run completion on verdict attestation + artifact-ROW presence;
// RFC 0123 routes bodies to blob_exhaust / git_publication / git_pointer_manifest.
// Neither guarantees the artifact BODY is reconstructable at completion: a row +
// size_bytes can survive while the body is gone (the Hippo retrospective —
// `artifact.get_content` returns "body file does not exist on disk"). This gate
// re-reads + hash-verifies every required artifact body per its declared
// placement, dispatching on placement, and is wired ORTHOGONALLY to the
// verdict-attestation path so RFC 0118's legitimate unattested-neutral admission
// does not regress.
//
// The degrade ladder (RFC 0125 Design): hard-fail only on POSITIVE evidence the
// body is gone or corrupt — a recorded blob whose readback fails, or a
// `refs/striatum/` anchor that exists but lacks (or mismatches) the body. When
// the durable substrate is legitimately not present/verifiable yet (an anchor
// pending pre-integration, no blob client configured), record a ledger WARNING
// rather than blocking a run that will integrate after terminal state.

const (
	reconstructionVerified = "verified" // body re-read and hash-matched content_sha256
	reconstructionWarning  = "warning"  // substrate absent/unverifiable; recorded, not blocked
	reconstructionFailed   = "failed"   // positive evidence the body is gone/corrupt; blocks
)

// artifactReconstruction is the per-required-artifact reconstructability result.
// It doubles as the RFC 0125 P1-1 RUN_LEDGER per-gate artifacts[] entry (#286).
type artifactReconstruction struct {
	ArtifactID       string
	LogicalName      string
	RepoPath         string
	Kind             string
	Placement        string
	ContentSHA256    string
	BlobSHA256       string
	BlobKey          string
	GitAnchorRef     string
	GitAnchorCommit  string
	ReadbackVerified bool
	Status           string
	Reason           string
}

// verifyRequiredArtifactReconstructable re-reads + hash-verifies the body of
// every REQUIRED expected artifact of the job, dispatching on the placement the
// contract declares. It returns the per-artifact reconstruction results so the
// caller can both record them into the RUN_LEDGER (#286) and route any
// reconstructionFailed result through the completion-gate failing[] path. It
// performs no mutation. verifyRequiredArtifacts has already proven the row
// exists; this checks that the BODY behind the row is durable.
func verifyRequiredArtifactReconstructable(ctx context.Context, runner any, repositoryID string, job map[string]any) ([]artifactReconstruction, error) {
	attempt := jobAttemptValue(job["attempt"])
	expected := resolveExpectedArtifactCycles(asList(job["expected_artifacts_json"]), attempt)
	if len(expected) == 0 {
		return nil, nil
	}
	jobID := fmt.Sprint(job["job_id"])
	runID := fmt.Sprint(job["run_id"])

	// Git context is resolved lazily and once — only git-placed artifacts need it.
	var repoRoot, runBranch string
	gitResolved := false
	resolveGit := func() (string, string, error) {
		if gitResolved {
			return repoRoot, runBranch, nil
		}
		root, err := activeRepositoryRoot(ctx, runner, repositoryID)
		if err != nil {
			return "", "", err
		}
		run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return "", "", err
		}
		repoRoot = root
		runBranch = reconstructionText(run["branch_name"])
		gitResolved = true
		return repoRoot, runBranch, nil
	}

	var results []artifactReconstruction
	for _, item := range expected {
		artifact := asMap(item)
		if artifact["required"] != true {
			continue
		}
		kind := fmt.Sprint(artifact["kind"])
		row, err := oneRow(ctx, runner, `
			SELECT artifact_id, content_sha256, blob_key, blob_sha256, repo_path
			  FROM striatumd.artifacts
			 WHERE repository_id = $1 AND job_id = $2 AND logical_name = $3
			   AND artifact_kind = $4 AND repo_path = $5 AND attempt = $6
			 ORDER BY created_at DESC, artifact_id DESC
			 LIMIT 1`,
			repositoryID, jobID, artifact["logical_name"], kind, artifact["path"], attempt)
		if errors.Is(err, pgx.ErrNoRows) {
			// verifyRequiredArtifacts ran first, so this should not happen; be
			// explicit rather than silently passing a missing body.
			results = append(results, artifactReconstruction{
				LogicalName: fmt.Sprint(artifact["logical_name"]),
				RepoPath:    fmt.Sprint(artifact["path"]),
				Kind:        kind,
				Status:      reconstructionFailed,
				Reason:      "artifact_row_missing",
			})
			continue
		}
		if err != nil {
			return nil, err
		}

		// Placement is declared by the contract (the durability surface the
		// workflow author chose), mirroring how artifact.publish resolves it; the
		// stored row's placement column is owner-bundle DDL and may be absent.
		placement := artifactcontracts.ResolvePlacement(kind, artifact["placement"])
		r := artifactReconstruction{
			ArtifactID:    fmt.Sprint(row["artifact_id"]),
			LogicalName:   fmt.Sprint(artifact["logical_name"]),
			RepoPath:      reconstructionText(row["repo_path"]),
			Kind:          kind,
			Placement:     placement,
			ContentSHA256: reconstructionText(row["content_sha256"]),
			BlobKey:       reconstructionText(row["blob_key"]),
			BlobSHA256:    reconstructionText(row["blob_sha256"]),
		}

		if placement == artifactcontracts.PlacementBlobExhaust && r.BlobKey != "" {
			if err := reconstructBlobExhaust(ctx, runner, repositoryID, &r); err != nil {
				return nil, err
			}
		} else {
			// git_publication and git_pointer_manifest: git is the body of record.
			// A blob_exhaust row with no blob_key (e.g. published with no blob
			// client) shares the same fallback surface artifact.get_content uses —
			// the repo_path / git anchor — so probe git rather than warn blindly.
			root, branch, err := resolveGit()
			if err != nil {
				return nil, err
			}
			if err := reconstructGitAnchored(ctx, runner, repositoryID, runID, jobID, attempt, root, branch, &r); err != nil {
				return nil, err
			}
		}
		results = append(results, r)
	}
	return results, nil
}

// reconstructBlobExhaust verifies a blob-placed body by re-reading it from blob
// storage with its content hash (GetBytes refuses a sha mismatch). A recorded
// blob key whose readback fails is positive evidence of loss (hard fail); a
// missing key or an unconfigured blob client cannot be verified and only warns.
func reconstructBlobExhaust(ctx context.Context, runner any, repositoryID string, r *artifactReconstruction) error {
	if r.BlobKey == "" {
		r.Status = reconstructionWarning
		r.Reason = "blob_exhaust_no_blob_key"
		return nil
	}
	if packageBlobClient == nil {
		r.Status = reconstructionWarning
		r.Reason = "blob_client_unavailable"
		return nil
	}
	bucket, err := lookupRepoBlobBucket(ctx, runner, repositoryID)
	if err != nil {
		return err
	}
	if bucket == "" {
		r.Status = reconstructionWarning
		r.Reason = "blob_bucket_unresolved"
		return nil
	}
	if _, err := packageBlobClient.GetBytes(ctx, bucket, r.BlobKey, r.ContentSHA256); err != nil {
		r.Status = reconstructionFailed
		r.Reason = "blob_readback_failed: " + err.Error()
		return nil
	}
	r.ReadbackVerified = true
	r.Status = reconstructionVerified
	return nil
}

// reconstructGitAnchored verifies a git-placed body by reading it back from the
// durable refs and hash-matching content_sha256. It probes every
// `refs/striatum/<run>/<job>/*` attempt pin (matching the read-side resolver in
// reads/worktree_refs.go, so the gate is never stricter than artifact.get_content),
// the legacy job pin, then the run branch HEAD (RFC 0117).
//
// The degrade ladder reserves a hard FAIL for POSITIVE evidence of loss; a
// transient inability to probe (git could not run) only WARNS, so an
// infrastructure fault never wedges a completion:
//   - found + sha-match anywhere ........................ verified
//   - found-but-mismatch anywhere ....................... failed (corruption)
//   - definitively absent from an existing anchor pin ... failed (the Hippo case)
//   - could not probe an existing anchor pin ............ warning (unconfirmed)
//   - no anchor pin yet, durable blob mirror present .... warning (pending)
//   - no anchor pin yet, nothing else ................... warning (pending)
//
// It returns an error only for a genuine DB fault; git-process faults degrade.
func reconstructGitAnchored(ctx context.Context, runner any, repositoryID, runID, jobID string, attempt int, repoRoot, runBranch string, r *artifactReconstruction) error {
	repoPath, ok := cleanDurableArtifactPath(r.RepoPath)
	if !ok {
		r.Status = reconstructionFailed
		r.Reason = "invalid_repo_path"
		return nil
	}
	// Candidate refs, most-specific first: the job's own attempt pins (all of
	// them, not just the current attempt — a revision-cycle body may be anchored
	// under a sibling attempt), the legacy job pin, then the run branch HEAD.
	siblingPins := gitJobAnchorRefs(ctx, repoRoot, runID, jobID)
	legacyPin := legacyPinRef(runID, jobID)
	candidates := dedupRefs(append(append([]string{attemptPinRef(runID, jobID, attempt)}, siblingPins...), legacyPin))
	if runBranch != "" {
		candidates = append(candidates, "refs/heads/"+runBranch)
	}
	anchorPinExists := len(siblingPins) > 0 || gitRefExists(ctx, repoRoot, legacyPin)

	sawMismatch := false
	probeErr := false
	for _, ref := range candidates {
		if !gitRefExists(ctx, repoRoot, ref) {
			continue
		}
		sha, found, err := gitRefFileSHA256(ctx, repoRoot, ref, repoPath)
		if err != nil {
			// git could not be run against this ref — an environment fault, not
			// evidence the body is gone. Record it and keep probing.
			probeErr = true
			continue
		}
		if !found {
			continue
		}
		if sha == r.ContentSHA256 {
			r.GitAnchorRef = ref
			r.GitAnchorCommit = gitRefCommit(ctx, repoRoot, ref)
			r.ReadbackVerified = true
			r.Status = reconstructionVerified
			return nil
		}
		sawMismatch = true
	}

	if sawMismatch {
		// A ref carries the path with a different sha — positive corruption.
		r.Status = reconstructionFailed
		if anchorPinExists {
			r.Reason = "git_anchor_content_mismatch"
		} else {
			r.Reason = "git_content_mismatch"
		}
		return nil
	}
	if anchorPinExists && !probeErr {
		// The body was anchored but is definitively not in any anchor pin and we
		// probed them all without fault — positive evidence of loss (Hippo).
		r.Status = reconstructionFailed
		r.Reason = "missing_from_git_anchor"
		return nil
	}
	// No anchor pin yet (legitimately pending pre-integration), or we could not
	// confirm absence. A durable blob mirror keeps the body reconstructable in
	// the interim; otherwise there is no positive evidence of loss, so warn.
	if r.BlobKey != "" && packageBlobClient != nil {
		bucket, err := lookupRepoBlobBucket(ctx, runner, repositoryID)
		if err != nil {
			return err
		}
		if bucket != "" {
			if _, err := packageBlobClient.GetBytes(ctx, bucket, r.BlobKey, r.ContentSHA256); err == nil {
				r.ReadbackVerified = true
				r.Status = reconstructionWarning
				r.Reason = "git_anchor_pending_blob_resident"
				return nil
			}
		}
	}
	r.Status = reconstructionWarning
	if probeErr {
		r.Reason = "git_probe_error"
	} else {
		r.Reason = "git_anchor_pending_unverifiable"
	}
	return nil
}

// gitJobAnchorRefs lists every refs/striatum/<run>/<job>/* attempt pin that
// exists, matching the read-side durable-ref resolver. Errors degrade to an
// empty list (the caller treats that as "no anchor pin yet").
func gitJobAnchorRefs(ctx context.Context, repoRoot, runID, jobID string) []string {
	result, err := runGitWorktreeCommand(ctx, repoRoot, "for-each-ref",
		"--format=%(refname)", "refs/striatum/"+runID+"/"+jobID+"/")
	if err != nil || result.ExitCode != 0 {
		return nil
	}
	var refs []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if ref := strings.TrimSpace(line); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

// dedupRefs preserves order and drops empties/duplicates.
func dedupRefs(refs []string) []string {
	seen := map[string]bool{}
	out := refs[:0:0]
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// failedReconstructions returns the entries whose body is positively gone/corrupt.
func failedReconstructions(results []artifactReconstruction) []artifactReconstruction {
	var failed []artifactReconstruction
	for _, r := range results {
		if r.Status == reconstructionFailed {
			failed = append(failed, r)
		}
	}
	return failed
}

// reconstructionLedgerEntries projects reconstruction results into the
// JSON-friendly per-gate artifacts[] block recorded in the completion ledger
// and the RUN_LEDGER (#286).
func reconstructionLedgerEntries(results []artifactReconstruction) []map[string]any {
	entries := make([]map[string]any, 0, len(results))
	for _, r := range results {
		entry := map[string]any{
			"artifact_id":       r.ArtifactID,
			"logical_name":      r.LogicalName,
			"repo_path":         r.RepoPath,
			"kind":              r.Kind,
			"placement":         r.Placement,
			"content_sha256":    r.ContentSHA256,
			"readback_verified": r.ReadbackVerified,
			"status":            r.Status,
		}
		if r.Reason != "" {
			entry["reason"] = r.Reason
		}
		if r.BlobSHA256 != "" {
			entry["blob_sha256"] = r.BlobSHA256
		}
		if r.BlobKey != "" {
			entry["blob_key"] = r.BlobKey
		}
		if r.GitAnchorRef != "" {
			entry["git_anchor_ref"] = r.GitAnchorRef
		}
		if r.GitAnchorCommit != "" {
			entry["git_anchor_commit"] = r.GitAnchorCommit
		}
		entries = append(entries, entry)
	}
	return entries
}

// gitRefExists reports whether ref resolves to a commit in the repo at repoRoot.
func gitRefExists(ctx context.Context, repoRoot, ref string) bool {
	result, err := runGitWorktreeCommand(ctx, repoRoot, "rev-parse", "--verify", "--quiet", ref)
	return err == nil && result.ExitCode == 0
}

// gitRefCommit resolves the commit a ref points at (best-effort, for the ledger).
func gitRefCommit(ctx context.Context, repoRoot, ref string) string {
	result, err := runGitWorktreeCommand(ctx, repoRoot, "rev-parse", "--verify", ref)
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// gitRefFileSHA256 returns the sha256 of the file content at ref:repoPath, and
// whether the path exists under that ref. It hashes stdout (the file bytes)
// exactly like gitCommittedFileSHA256, generalized to an arbitrary ref.
func gitRefFileSHA256(ctx context.Context, repoRoot, ref, repoPath string) (string, bool, error) {
	result, err := runGitWorktreeCommand(ctx, repoRoot, "show", ref+":"+repoPath)
	if err != nil {
		return "", false, err
	}
	if result.ExitCode != 0 {
		return "", false, nil
	}
	sum := sha256.Sum256([]byte(result.Stdout))
	return hex.EncodeToString(sum[:]), true, nil
}

// reconstructionText normalizes a possibly-NULL column to a trimmed string,
// collapsing the nil/"<nil>" sentinel to empty.
func reconstructionText(v any) string {
	s := strings.TrimSpace(fmt.Sprint(nullable(v)))
	if s == "<nil>" {
		return ""
	}
	return s
}
