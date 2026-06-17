package reads

import (
	"context"
	"strings"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/db"
)

// DebrisVerdict is the per-artifact eligibility verdict for
// `recovery prune-debris`. It is produced by ArtifactDebrisVerdicts, which
// reuses the exact same per-row classifiers the doctor artifact-anchor pass
// runs, so an artifact is Eligible here if and only if doctor would have
// reported it as artifact_debris_terminal_run (archived debris from an
// abandoned run whose files are gone and that no preservation guard rescued).
type DebrisVerdict struct {
	ArtifactID    string
	RunID         string
	JobID         string
	RepoPath      string
	ContentSHA256 string
	// Code is the doctor classifier verdict for the row: the debris warning
	// code (artifact_debris_terminal_run) when Eligible, otherwise the
	// preservation warning code, the active integrity problem code, or "" when
	// the row is clean (preserved on a durable ref / default branch).
	Code string
	// Eligible is true only when the row classified to the terminal-debris
	// warning: (a) the run is in a terminal-debris state, (b) the underlying
	// integrity finding is a debris code, and (c) no preservation guard
	// (file present on default branch, anchored on a durable ref, in history,
	// acknowledged-loss baseline) matched. This is byte-identical to the
	// doctor eligibility because it goes through the same classifiers.
	Eligible bool
	// Reason names why a non-eligible row is retained, in the vocabulary the
	// prune handler surfaces: file_present_on_default_branch,
	// anchored_on_durable_ref, not_terminal, not_a_debris_code.
	Reason string
}

// debrisEligibleCodes is the set of doctor codes the prune verb treats as
// prunable terminal-run debris. Only artifact_debris_terminal_run is emitted by
// the per-row classifiers' terminal branch — the underlying integrity codes
// (hash_mismatch / missing_file / blob_metadata_missing /
// legacy_unverifiable) are reclassified into it on a terminal-debris run — so
// it is the single eligibility code. The constants are listed here so the
// eligibility set stays anchored to the classifier's code vocabulary.
var debrisEligibleCodes = map[string]bool{
	artifactDebrisTerminalRun: true,
}

// ArtifactDebrisVerdicts returns the per-artifact prune-eligibility verdicts for
// a single run, reusing the exact per-row classifiers the doctor artifact-anchor
// pass uses (checkBlobExhaustArtifact / checkArtifactAnchor). Eligibility is
// therefore byte-identical to what doctor reports: a row is Eligible only when
// the classifier reclassifies it to the artifact_debris_terminal_run warning,
// which already folds in (a) terminalDebrisRunState(run_state), (b) a debris
// integrity code, and (c) "no preservation guard matched".
//
// It does not require blob storage to be configured: when packageBlobClient is
// nil the blob-exhaust classifier still partitions metadata-missing rows (it
// only skips the live blob-body fetch), which is exactly the offline behavior
// the prune verb needs. The scan is scoped to the one run, so it issues bounded
// git work like the doctor pass.
func ArtifactDebrisVerdicts(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]DebrisVerdict, error) {
	repositoryID = strings.TrimSpace(repositoryID)
	runID = strings.TrimSpace(runID)
	if repositoryID == "" || runID == "" {
		return nil, nil
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
		   AND a.run_id = $2
		   AND j.state = 'completed'
		   AND COALESCE(j.write_scope_json->>'repo_write', 'false') = 'true'
		 ORDER BY a.run_id, a.job_id, a.created_at, a.artifact_id`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	decorateArtifactPlacements(rows)

	bucket := ""
	if packageBlobClient != nil {
		// A missing/unreachable bucket is non-fatal here: the classifier returns a
		// blob-body-verify result that, on a terminal run, still folds into the
		// debris warning, so the eligibility verdict is unchanged. Resolve it
		// best-effort and degrade to "" on error.
		if resolved, lookupErr := lookupRepoBlobBucketRead(ctx, runner, repositoryID); lookupErr == nil {
			bucket = resolved
		}
	}

	pass := newArtifactAnchorPass()
	verdicts := make([]DebrisVerdict, 0, len(rows))
	for _, row := range rows {
		placement := artifactcontracts.ResolvePlacement(stringFrom(row, "artifact_kind"), row["placement"])
		row["placement"] = placement
		defaultRef := resolveDefaultRefCached(ctx, strings.TrimSpace(stringFrom(row, "repo_root")), pass.defaultRefByRoot)

		var problem, warning string
		if artifactcontracts.PlacementUsesBlob(placement) {
			problem, _, warning, _ = checkBlobExhaustArtifact(ctx, row, bucket, defaultRef, pass)
		} else if artifactcontracts.PlacementUsesGitAnchor(placement) {
			problem, _, warning, _ = checkArtifactAnchor(ctx, row, defaultRef, pass)
		}

		verdict := DebrisVerdict{
			ArtifactID:    stringFrom(row, "artifact_id"),
			RunID:         stringFrom(row, "run_id"),
			JobID:         stringFrom(row, "job_id"),
			RepoPath:      stringFrom(row, "repo_path"),
			ContentSHA256: stringFrom(row, "content_sha256"),
		}
		switch {
		case warning != "" && debrisEligibleCodes[verdictCheckCode(warning)]:
			verdict.Code = artifactDebrisTerminalRun
			verdict.Eligible = true
		case warning != "":
			verdict.Code = verdictCheckCode(warning)
			verdict.Reason = retainReasonForWarning(verdict.Code)
		case problem != "":
			verdict.Code = verdictCheckCode(problem)
			// A problem on a non-terminal run is an active gap, not debris; the
			// classifier only reaches a problem verdict when the terminal/preservation
			// guards did NOT fire, so the run is non-terminal here.
			verdict.Reason = retainReasonNotTerminal
		default:
			// Clean: content is preserved on a durable ref / default branch.
			verdict.Reason = retainReasonAnchored
		}
		verdicts = append(verdicts, verdict)
	}
	return verdicts, nil
}

// Retention reason vocabulary surfaced to the prune handler / operator.
const (
	retainReasonFilePresentOnDefault = "file_present_on_default_branch"
	retainReasonAnchored             = "anchored_on_durable_ref"
	retainReasonNotTerminal          = "not_terminal"
	retainReasonNotDebrisCode        = "not_a_debris_code"
)

// verdictCheckCode extracts the doctor check code from a classifier message of
// the shape "<check>.<artifact_id>: ...". The classifiers always prefix their
// message with the check code, so this never needs the full record map.
func verdictCheckCode(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	head := message
	if idx := strings.IndexByte(head, ':'); idx >= 0 {
		head = head[:idx]
	}
	if idx := strings.IndexByte(head, '.'); idx >= 0 {
		head = head[:idx]
	}
	return strings.TrimSpace(head)
}

// retainReasonForWarning maps a non-debris preservation warning code to the
// retention reason the prune handler reports. The superseded-on-default-branch
// warning means the deliverable path is still live on the default branch; the
// other preservation warnings mean the content is durably anchored.
func retainReasonForWarning(code string) string {
	switch code {
	case artifactSupersededOnDefaultBranch:
		return retainReasonFilePresentOnDefault
	case artifactLegacyUnverifiable, artifactAcknowledgedLoss:
		return retainReasonAnchored
	default:
		return retainReasonNotDebrisCode
	}
}

// ArtifactDebrisRunStateTerminal reports whether a run state is a terminal
// debris state (canceled/failed) — the prune verb's run-state gate. It re-exports
// the doctor predicate so the prune handler refuses non-terminal runs with the
// identical classification doctor uses, without duplicating the state list.
func ArtifactDebrisRunStateTerminal(state string) bool {
	return terminalDebrisRunState(state)
}
