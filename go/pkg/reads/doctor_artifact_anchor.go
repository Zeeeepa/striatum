package reads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/db"
)

const (
	artifactAnchorHashMismatch   = "artifact_anchor_hash_mismatch"
	artifactAnchorMissingFile    = "artifact_anchor_missing_file"
	artifactBlobMetadataMissing  = "artifact_blob_metadata_missing"
	artifactBlobBodyVerifyFailed = "artifact_blob_body_verify_failed"

	// Legibility warning codes (D-doctor-integrity-legibility): reclassified,
	// non-ok-reddening counterparts of the integrity problems above.
	artifactLegacyUnverifiable = "artifact_legacy_unverifiable"
	artifactDebrisTerminalRun  = "artifact_debris_terminal_run"
)

func doctorArtifactAnchorIntegrity(ctx context.Context, runner db.Runner, repositoryID string, blobBlock map[string]any) (map[string]any, []string, []map[string]any, []string, []map[string]any) {
	block := map[string]any{
		"checked": false,
		"skipped": artifactAnchorSkipReason(repositoryID, blobBlock),
	}
	if block["skipped"] != "" {
		return block, nil, nil, nil, nil
	}

	rows, err := collectRows(ctx, runner, `
		SELECT a.repository_id, a.artifact_id, a.run_id, a.job_id, a.logical_name, a.repo_path,
		       a.content_sha256, a.artifact_kind, a.blob_key,
		       a.blob_sha256, a.blob_content_type`+artifactPlacementProjectionAny(ctx, runner, "a")+`,
		       j.workflow_job_id, j.attempt, j.write_scope_json,
		       r.repo_root, r.branch_name, r.state AS run_state
		  FROM striatumd.artifacts a
		  JOIN striatumd.jobs j
		    ON j.repository_id = a.repository_id
		   AND j.job_id = a.job_id
		  JOIN striatumd.runs r
		    ON r.repository_id = a.repository_id
		   AND r.run_id = a.run_id
		 WHERE a.repository_id = $1
		   AND j.state = 'completed'
		   AND COALESCE(j.write_scope_json->>'repo_write', 'false') = 'true'
		 ORDER BY a.run_id, a.job_id, a.created_at, a.artifact_id`,
		repositoryID,
	)
	if err != nil {
		block["error"] = err.Error()
		return block, nil, nil, nil, nil
	}

	block["checked"] = true
	block["skipped"] = nil
	block["artifact_count"] = len(rows)
	decorateArtifactPlacements(rows)
	problems := []string{}
	records := []map[string]any{}
	warnings := []string{}
	warningRecords := []map[string]any{}
	gitChecked := 0
	blobChecked := 0
	bucket := ""
	if packageBlobClient != nil {
		var err error
		bucket, err = lookupRepoBlobBucketRead(ctx, runner, repositoryID)
		if err != nil {
			block["error"] = err.Error()
			return block, nil, nil, nil, nil
		}
	}
	defaultRefByRoot := map[string]string{}
	for _, row := range rows {
		placement := artifactcontracts.ResolvePlacement(stringFrom(row, "artifact_kind"), row["placement"])
		row["placement"] = placement
		defaultRef := resolveDefaultRefCached(ctx, strings.TrimSpace(stringFrom(row, "repo_root")), defaultRefByRoot)
		var problem string
		var record map[string]any
		var warning string
		var warningRecord map[string]any
		if artifactcontracts.PlacementUsesBlob(placement) {
			blobChecked++
			problem, record, warning, warningRecord = checkBlobExhaustArtifact(ctx, row, bucket, defaultRef)
		} else if artifactcontracts.PlacementUsesGitAnchor(placement) {
			gitChecked++
			problem, record, warning, warningRecord = checkArtifactAnchor(ctx, row, defaultRef)
		}
		if problem != "" {
			problems = append(problems, problem)
			records = append(records, record)
		}
		if warning != "" {
			warnings = append(warnings, warning)
			warningRecords = append(warningRecords, warningRecord)
		}
	}
	block["git_anchor_count"] = gitChecked
	block["blob_exhaust_count"] = blobChecked
	block["problem_count"] = len(problems)
	block["warning_count"] = len(warnings)
	return block, problems, records, warnings, warningRecords
}

func checkBlobExhaustArtifact(ctx context.Context, row map[string]any, bucket, defaultRef string) (string, map[string]any, string, map[string]any) {
	blobKey := strings.TrimSpace(stringFrom(row, "blob_key"))
	expected := strings.TrimSpace(stringFrom(row, "blob_sha256"))
	if expected == "" {
		expected = strings.TrimSpace(stringFrom(row, "content_sha256"))
	}
	if blobKey == "" {
		// Legibility rule 3 (D-doctor-integrity-legibility): an empty blob_key is
		// the signature of a legacy artifact that predates RFC 0125 blob storage,
		// not a fresh durability gap. If its content is still verifiable on a
		// durable ref or the default branch, downgrade to a legacy warning; only
		// genuine loss (content absent everywhere) stays an ok-reddening problem.
		if artifactContentPreserved(ctx, row, defaultRef) {
			warning, record := artifactWarning(artifactLegacyUnverifiable, row, "blob_key_absent_predates_blob_storage", defaultRef)
			return "", nil, warning, record
		}
		if terminalDebrisRunState(stringFrom(row, "run_state")) {
			warning, record := artifactWarning(artifactDebrisTerminalRun, row, "blob_key_or_sha_missing", "")
			return "", nil, warning, record
		}
		problem, record := artifactBlobProblem(artifactBlobMetadataMissing, row, "blob_key_or_sha_missing")
		return problem, record, "", nil
	}
	if expected == "" {
		problem, record := artifactBlobProblem(artifactBlobMetadataMissing, row, "blob_key_or_sha_missing")
		return problem, record, "", nil
	}
	if packageBlobClient == nil {
		return "", nil, "", nil
	}
	if bucket == "" {
		return blobBodyVerifyResult(row, "repository_blob_bucket_missing")
	}
	if _, err := packageBlobClient.GetBytes(ctx, bucket, blobKey, expected); err != nil {
		return blobBodyVerifyResult(row, err.Error())
	}
	return "", nil, "", nil
}

// blobBodyVerifyResult classifies a blob-body verification failure. A failure on
// an abandoned (terminal-debris) run is leftover, not an active gap, so it is a
// warning; otherwise it stays an ok-reddening problem.
func blobBodyVerifyResult(row map[string]any, detail string) (string, map[string]any, string, map[string]any) {
	if terminalDebrisRunState(stringFrom(row, "run_state")) {
		warning, record := artifactWarning(artifactDebrisTerminalRun, row, detail, "")
		return "", nil, warning, record
	}
	problem, record := artifactBlobProblem(artifactBlobBodyVerifyFailed, row, detail)
	return problem, record, "", nil
}

func artifactAnchorSkipReason(repositoryID string, blobBlock map[string]any) string {
	if strings.TrimSpace(repositoryID) == "" {
		return "repository_id_missing"
	}
	if blobBlock["configured"] != true {
		return "blob_not_configured"
	}
	if blobBlock["reachable"] != true {
		return "blob_unreachable"
	}
	if stringFrom(blobBlock, "bucket_status") != "ok" {
		return "blob_bucket_not_ok"
	}
	return ""
}

func checkArtifactAnchor(ctx context.Context, row map[string]any, defaultRef string) (string, map[string]any, string, map[string]any) {
	repoPath, ok := cleanArtifactAnchorPath(stringFrom(row, "repo_path"))
	if !ok {
		problem, record := artifactAnchorProblem(artifactAnchorMissingFile, row, "", "", "invalid_repo_path")
		return problem, record, "", nil
	}
	expected := strings.TrimSpace(stringFrom(row, "content_sha256"))
	repoRoot := strings.TrimSpace(stringFrom(row, "repo_root"))
	refs := durableWorktreeProbeRefs(ctx, repoRoot, row)
	if repoRoot == "" || expected == "" || len(refs) == 0 {
		return "", nil, "", nil
	}

	checkedRefs := []string{}
	fileFound := false
	var mismatchAnchor artifactAnchorProbe
	var firstAnchor artifactAnchorProbe
	for _, ref := range refs {
		commit, err := readGitCommit(ctx, repoRoot, ref)
		if err != nil {
			continue
		}
		checkedRefs = appendUniqueString(checkedRefs, ref)
		if firstAnchor.Ref == "" {
			firstAnchor = artifactAnchorProbe{Ref: ref, Commit: commit}
		}
		probe, err := readGitBlobSHA256(ctx, repoRoot, commit, repoPath)
		if err != nil {
			problem, record := artifactAnchorProblem(artifactAnchorMissingFile, row, ref, commit, err.Error())
			return problem, record, "", nil
		}
		if !probe.Exists {
			continue
		}
		fileFound = true
		if probe.SHA256 == expected {
			return "", nil, "", nil
		}
		if mismatchAnchor.Ref == "" {
			mismatchAnchor = artifactAnchorProbe{Ref: ref, Commit: commit, SHA256: probe.SHA256}
		}
	}
	row["checked_refs"] = checkedRefs

	// Legibility rule 1 (D-doctor-integrity-legibility): content that matches the
	// artifact's content_sha256 at the default-branch tip is durably preserved
	// (the run branch was merged then deleted). That is not loss, so it is clean —
	// not even a warning, since there is nothing for the operator to anchor.
	if defaultRef != "" && artifactContentMatchesRef(ctx, repoRoot, defaultRef, repoPath, expected) {
		return "", nil, "", nil
	}

	// Legibility rule 2: a finding from an abandoned (terminal-debris) run is
	// archived leftover, not an active gap.
	if terminalDebrisRunState(stringFrom(row, "run_state")) {
		detail := "path_not_present_in_checked_anchors"
		if fileFound {
			detail = "anchor_content_sha_mismatch"
		}
		warning, record := artifactWarning(artifactDebrisTerminalRun, row, detail, "")
		return "", nil, warning, record
	}

	if fileFound {
		problem, record := artifactAnchorProblem(artifactAnchorHashMismatch, row, mismatchAnchor.Ref, mismatchAnchor.Commit, mismatchAnchor.SHA256)
		return problem, record, "", nil
	}
	problem, record := artifactAnchorProblem(artifactAnchorMissingFile, row, firstAnchor.Ref, firstAnchor.Commit, "path_not_present_in_checked_anchors")
	return problem, record, "", nil
}

// artifactContentPreserved reports whether the artifact's content_sha256 is
// present at its repo_path on any durable ref (run branch / refs/striatum pins)
// or at the default-branch tip. It degrades safely: a missing repo root, path,
// sha, or ref is treated as "not preserved here".
func artifactContentPreserved(ctx context.Context, row map[string]any, defaultRef string) bool {
	repoRoot := strings.TrimSpace(stringFrom(row, "repo_root"))
	repoPath, ok := cleanArtifactAnchorPath(stringFrom(row, "repo_path"))
	expected := strings.TrimSpace(stringFrom(row, "content_sha256"))
	if repoRoot == "" || !ok || expected == "" {
		return false
	}
	refs := durableWorktreeProbeRefs(ctx, repoRoot, row)
	if defaultRef != "" {
		refs = appendUniqueString(refs, defaultRef)
	}
	return artifactContentMatchesAnyRef(ctx, repoRoot, repoPath, expected, refs)
}

func artifactContentMatchesAnyRef(ctx context.Context, repoRoot, repoPath, expectedSHA string, refs []string) bool {
	for _, ref := range refs {
		if artifactContentMatchesRef(ctx, repoRoot, ref, repoPath, expectedSHA) {
			return true
		}
	}
	return false
}

func artifactContentMatchesRef(ctx context.Context, repoRoot, ref, repoPath, expectedSHA string) bool {
	commit, err := readGitCommit(ctx, repoRoot, ref)
	if err != nil {
		return false
	}
	probe, err := readGitBlobSHA256(ctx, repoRoot, commit, repoPath)
	if err != nil || !probe.Exists {
		return false
	}
	return probe.SHA256 == expectedSHA
}

type artifactAnchorProbe struct {
	Ref    string
	Commit string
	SHA256 string
	Exists bool
}

func readGitBlobSHA256(ctx context.Context, repoRoot, commit, repoPath string) (artifactAnchorProbe, error) {
	body, present, err := readGitFileBytes(ctx, repoRoot, commit, repoPath)
	if err != nil {
		return artifactAnchorProbe{}, err
	}
	if !present {
		return artifactAnchorProbe{Commit: commit, Exists: false}, nil
	}
	sum := sha256.Sum256(body)
	return artifactAnchorProbe{Commit: commit, SHA256: hex.EncodeToString(sum[:]), Exists: true}, nil
}

// readGitFileBytes returns the raw bytes of repoPath at commit (git show
// commit:repoPath, stdout only so the body is byte-exact for sha verification).
// present=false means the path is absent in that commit/tree.
func readGitFileBytes(ctx context.Context, repoRoot, commit, repoPath string) ([]byte, bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", commit+":"+repoPath)
	body, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, false, nil
		}
		return nil, false, err
	}
	return body, true, nil
}

func cleanArtifactAnchorPath(pathText string) (string, bool) {
	pathText = strings.TrimSpace(filepath.ToSlash(pathText))
	if pathText == "" || strings.HasPrefix(pathText, "/") {
		return "", false
	}
	clean := filepath.ToSlash(filepath.Clean(pathText))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func artifactAnchorProblem(check string, row map[string]any, anchorRef, anchorCommit, detail string) (string, map[string]any) {
	artifactID := stringFrom(row, "artifact_id")
	repoPath := stringFrom(row, "repo_path")
	message := fmt.Sprintf("%s.%s: artifact %s at %s does not match its durable git anchor", check, artifactID, artifactID, repoPath)
	if check == artifactAnchorMissingFile {
		message = fmt.Sprintf("%s.%s: artifact %s missing at %s in durable git anchor", check, artifactID, artifactID, repoPath)
	}
	contextMap := map[string]any{
		"repository_id":  row["repository_id"],
		"run_id":         row["run_id"],
		"job_id":         row["job_id"],
		"artifact_id":    row["artifact_id"],
		"logical_name":   row["logical_name"],
		"repo_path":      row["repo_path"],
		"content_sha256": row["content_sha256"],
		"placement":      row["placement"],
		"anchor_kind":    artifactAnchorKind(anchorRef),
		"anchor_ref":     nullableString(anchorRef),
		"anchor_commit":  nullableString(anchorCommit),
		"checked_refs":   row["checked_refs"],
	}
	if check == artifactAnchorHashMismatch {
		contextMap["anchor_content_sha256"] = detail
	} else {
		contextMap["reason"] = detail
	}
	record := map[string]any{
		"check":   check,
		"id":      artifactID,
		"context": contextMap,
	}
	return message, record
}

func artifactBlobProblem(check string, row map[string]any, detail string) (string, map[string]any) {
	artifactID := stringFrom(row, "artifact_id")
	message := fmt.Sprintf("%s.%s: blob-exhaust artifact %s does not have a verified blob body", check, artifactID, artifactID)
	if check == artifactBlobMetadataMissing {
		message = fmt.Sprintf("%s.%s: blob-exhaust artifact %s is missing blob metadata", check, artifactID, artifactID)
	}
	record := map[string]any{
		"check": check,
		"id":    artifactID,
		"context": map[string]any{
			"repository_id":     row["repository_id"],
			"run_id":            row["run_id"],
			"job_id":            row["job_id"],
			"artifact_id":       row["artifact_id"],
			"logical_name":      row["logical_name"],
			"repo_path":         row["repo_path"],
			"content_sha256":    row["content_sha256"],
			"placement":         row["placement"],
			"blob_key":          row["blob_key"],
			"blob_sha256":       row["blob_sha256"],
			"blob_content_type": row["blob_content_type"],
			"reason":            detail,
		},
	}
	return message, record
}

// artifactWarning builds the message + verbose record for a reclassified
// (warning, not problem) artifact finding. Records stay parallel to
// artifactBlobProblem / artifactAnchorProblem so verbose consumers can read both
// channels with the same shape.
func artifactWarning(check string, row map[string]any, detail, preservedRef string) (string, map[string]any) {
	artifactID := stringFrom(row, "artifact_id")
	repoPath := stringFrom(row, "repo_path")
	var message string
	switch check {
	case artifactLegacyUnverifiable:
		message = fmt.Sprintf("%s.%s: legacy artifact %s at %s predates blob storage; content is preserved on a durable ref or the default branch", check, artifactID, artifactID, repoPath)
	case artifactDebrisTerminalRun:
		message = fmt.Sprintf("%s.%s: artifact %s at %s belongs to an abandoned run; archived debris, not an active durability gap", check, artifactID, artifactID, repoPath)
	default:
		message = fmt.Sprintf("%s.%s: artifact %s at %s reclassified to warning", check, artifactID, artifactID, repoPath)
	}
	contextMap := map[string]any{
		"repository_id":  row["repository_id"],
		"run_id":         row["run_id"],
		"job_id":         row["job_id"],
		"artifact_id":    row["artifact_id"],
		"logical_name":   row["logical_name"],
		"repo_path":      row["repo_path"],
		"content_sha256": row["content_sha256"],
		"placement":      row["placement"],
		"blob_key":       row["blob_key"],
		"run_state":      row["run_state"],
		"reason":         detail,
	}
	if preservedRef != "" {
		contextMap["preserved_ref"] = preservedRef
	}
	record := map[string]any{
		"check":   check,
		"id":      artifactID,
		"context": contextMap,
	}
	return message, record
}

func artifactAnchorKind(ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		return worktreeAnchorRunBranch
	}
	return worktreeAnchorJobPin
}
