package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func SweepRun(ctx context.Context, runner db.Runner, repositoryID string, runID string, author string) (map[string]any, error) {
	if author == "" {
		author = "striatumd-go"
	}
	result, err := HandleRecoveryAuto(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "daemon_sweep_" + runID,
		Method:        "recovery.sweep",
		Params: map[string]any{
			"repository_id": repositoryID,
			"run_id":        runID,
		},
	})
	if err != nil {
		return nil, err
	}
	_, err = withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first — this records a run-scoped sweep
		// event concurrently with claim/verdict-completion on the same run.
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		_, err := appendEvent(ctx, tx, repositoryID, runID, "daemon.recovery_sweep", nil, nil, nil, nil, nil, map[string]any{
			"author":        author,
			"repository_id": repositoryID,
			"result":        result,
		})
		return map[string]any{}, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// jobSalvageSearchRoots returns the ordered roots the auto-publish pass scans for
// a required artifact written-but-unsealed (#530). The main run repo_root is tried
// FIRST (the legacy behavior — a shared-checkout lane writes there). When a job ran
// on a PER-JOB worktree (worktree_isolation: per_job), the agent wrote its
// deliverable into the per-job worktree under .striatum/worktrees/, NOT the main
// checkout — so a transient unsealed exit (e.g. transient Anthropic-API
// unavailability during end-of-session wind-down) discarded the work because the
// auto-publish pass only ever scanned the main repo_root and found nothing
// (#530). We append the job's most-recent non-removed per-job worktree so its
// committed-but-unsealed deliverable can be adopted instead of discarded. The
// worktree path is validated to sit under repo_root/.striatum/worktrees/ (the
// read-side jail) so a forged worktree_path cannot redirect the read outside the
// scratch tree. A root is included only when it exists on disk.
func jobSalvageSearchRoots(ctx context.Context, runner any, repositoryID, repoRoot string, job map[string]any) []string {
	roots := []string{repoRoot}
	jobID := fmt.Sprint(job["job_id"])
	// Most-recent non-removed worktree for the job (active preferred, then abandoned).
	row, err := oneRow(ctx, runner, `
		SELECT worktree_path
		  FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND job_id = $2
		   AND state IN ('active','abandoned','released')
		 ORDER BY (state = 'active') DESC, created_at DESC, worktree_id DESC
		 LIMIT 1`, repositoryID, jobID)
	if err != nil || row == nil {
		return roots
	}
	worktreeRoot, jerr := salvageWorktreeTarget(repoRoot, fmt.Sprint(nullable(row["worktree_path"])))
	if jerr != nil {
		return roots
	}
	if info, serr := os.Stat(worktreeRoot); serr != nil || !info.IsDir() {
		return roots
	}
	return append(roots, worktreeRoot)
}

// salvageWorktreeTarget resolves a job_worktrees.worktree_path (relative or
// absolute) to an absolute path and jails it under repo_root/.striatum/worktrees/,
// mirroring the read-side reads.readWorktreeTarget so the salvage read is never
// looser than the worktree-refs reader. Returns an error if the path is empty or
// escapes the scratch jail.
func salvageWorktreeTarget(repoRoot, pathText string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("repo_root_invalid")
	}
	pathText = strings.TrimSpace(pathText)
	if pathText == "" || pathText == "<nil>" {
		return "", fmt.Errorf("worktree_path_missing")
	}
	var target string
	if filepath.IsAbs(pathText) {
		target = filepath.Clean(pathText)
	} else {
		target = filepath.Clean(filepath.Join(root, filepath.FromSlash(pathText)))
	}
	worktreesRoot := filepath.Join(root, ".striatum", "worktrees")
	rel, err := filepath.Rel(worktreesRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("worktree_path_outside_scratch")
	}
	return target, nil
}

func autoPublishableArtifacts(ctx context.Context, runner any, repositoryID string, repoRoot string, job map[string]any, sessionID string, expectedByline string) (publishable []map[string]any, skipped []map[string]any, err error) {
	publishable = []map[string]any{}
	skipped = []map[string]any{}
	currentAttempt := jobAttemptValue(job["attempt"])
	workflowJobID := fmt.Sprint(job["workflow_job_id"])
	// #530: scan the main repo_root first, then the job's per-job worktree, so a
	// deliverable an unsealed-exiting per-job lane wrote into its isolated worktree
	// is salvaged rather than discarded.
	searchRoots := jobSalvageSearchRoots(ctx, runner, repositoryID, repoRoot, job)
	for _, item := range asList(job["expected_artifacts_json"]) {
		declared := asMap(item)
		if declared["required"] == false {
			continue
		}
		pathText, _ := declared["path"].(string)
		kind, _ := declared["kind"].(string)
		logicalName, _ := declared["logical_name"].(string)
		if pathText == "" || kind == "" || logicalName == "" {
			continue
		}
		// Try each search root in priority order; the first that holds a readable,
		// valid-UTF-8 file wins and its root is recorded so the recovered artifact is
		// published FROM the same root (the body read for the artifact row matches).
		var payload []byte
		var sourceRoot string
		for _, root := range searchRoots {
			path, perr := repoRelativePath(root, pathText, false)
			if perr != nil {
				continue
			}
			info, serr := os.Stat(path)
			if serr != nil || info.IsDir() {
				continue
			}
			body, rerr := os.ReadFile(path)
			if rerr != nil || !utf8.Valid(body) {
				continue
			}
			payload = body
			sourceRoot = root
			break
		}
		if sourceRoot == "" {
			continue
		}
		matched := false
		for _, line := range markdownTitleBlockAuthorLines(string(payload)) {
			if canonicalBylineForm(line) == expectedByline {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		// RFC 0095 Goal 2 / #203: recovery auto-publish must only complete a job
		// from artifacts produced by the CURRENT attempt. A re-opened (revision-
		// cycle) job whose on-disk file is byte-identical to a PRIOR attempt's
		// published artifact is the pre-revision document the reviewer rejected —
		// crediting it converts the needs_revision verdict into a silent no-op.
		// Detect it by content_sha256: a row for THIS job at a lower attempt with
		// the same content means the on-disk file is stale prior-attempt output.
		sum := sha256.Sum256(payload)
		digest := hex.EncodeToString(sum[:])
		priorAttempt, found, perr := priorAttemptArtifactByContent(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), fmt.Sprint(job["job_id"]), digest, currentAttempt)
		if perr != nil {
			return nil, nil, perr
		}
		if found {
			skipped = append(skipped, map[string]any{
				"workflow_job_id": workflowJobID,
				"path":            pathText,
				"reason":          "stale_prior_attempt_artifact",
				"detail": fmt.Sprintf(
					"on-disk %s is byte-identical (content_sha256=%s) to this job's attempt-%d artifact while the job is at attempt %d; a re-opened (revision-cycle) attempt is never satisfied by a prior attempt's output (RFC 0095 Goal 2 / #203). A fresh lane must perform the revision.",
					pathText, digest, priorAttempt, currentAttempt,
				),
			})
			continue
		}
		publishable = append(publishable, map[string]any{
			"path":         pathText,
			"kind":         kind,
			"logical_name": logicalName,
			// #530: the root the body was found in (main repo_root or the per-job
			// worktree). The publish step reads from this root so the artifact row's
			// content matches the file that satisfied the byline/attempt gates.
			"source_root": sourceRoot,
		})
	}
	return publishable, skipped, nil
}

// priorAttemptArtifactByContent reports whether THIS job already published an
// artifact with the given content_sha256 at an attempt strictly lower than the
// job's current attempt. Used by the recovery auto-publish attempt-gate
// (RFC 0095 Goal 2 / #203) to refuse crediting a re-opened job from its
// pre-revision on-disk document. Returns the matched prior attempt for a legible
// skip reason.
func priorAttemptArtifactByContent(ctx context.Context, runner any, repositoryID, runID, jobID, contentSha256 string, currentAttempt int) (priorAttempt int, found bool, err error) {
	row, err := oneRow(ctx, runner, `
		SELECT attempt FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND content_sha256 = $4 AND attempt < $5
		 ORDER BY attempt DESC
		 LIMIT 1`, repositoryID, runID, jobID, contentSha256, currentAttempt)
	if err != nil {
		if errorsIsNoRows(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return jobAttemptValue(row["attempt"]), true, nil
}

// recoveredReviewVerdict extracts a verdict-bearing job's verdict from the
// just-published finding artifact's verdict_intent front matter, pairing it with
// that finding's published artifact_id (publishable[i] <-> artifacts[i]). #144:
// the stale-lease auto-publish path uses this to record the review's ACTUAL
// verdict on recovery (honoring what the reviewer decided on disk), not a blanket
// accept. Returns found=false when no finding with a verdict_intent is among the
// published artifacts, so the caller falls back to a plain completion.
func recoveredReviewVerdict(repoRoot string, publishable, artifacts []map[string]any) (verdict string, findingArtifactID any, found bool, err error) {
	for i, declared := range publishable {
		kind := fmt.Sprint(declared["kind"])
		if _, ok := frontMatterSchemas[kind]; !ok {
			continue
		}
		pathText := fmt.Sprint(declared["path"])
		// #530: read the finding from the root it was salvaged from (main repo_root
		// or the per-job worktree), so a worktree-salvaged review's verdict_intent is
		// honored rather than missed (which would default to a plain completion and
		// silently drop a needs_revision).
		root := repoRoot
		if r, ok := declared["source_root"].(string); ok && r != "" {
			root = r
		}
		path, perr := repoRelativePath(root, pathText, false)
		if perr != nil {
			continue
		}
		payload, rerr := os.ReadFile(filepath.Clean(path))
		if rerr != nil {
			continue
		}
		fm, ferr := autoFinalizeRequiredFrontMatter(kind, path, payload)
		if ferr != nil {
			return "", nil, false, ferr
		}
		v, ok := fm["verdict_intent"].(string)
		if !ok || v == "" {
			continue
		}
		if i < len(artifacts) {
			findingArtifactID = artifacts[i]["artifact_id"]
		}
		return v, findingArtifactID, true, nil
	}
	return "", nil, false, nil
}

// salvagePublishMissingRequiredArtifacts is the #530 salvage path for the
// operator complete-stalled verb: when a required artifact ROW is missing (the
// lane wrote its deliverable but exited unsealed before artifact.publish — e.g. a
// transient Anthropic-API outage during end-of-session wind-down), this scans the
// job's main-checkout and per-job-worktree roots for the declared file and, if it
// finds a matching-byline body, publishes the missing artifact ROW(s) so the
// reconstructability gate (which then resolves the body via the worktree's
// git anchor) can pass and the job can be finalized from durable provenance. It
// is a no-op when the rows already exist (the legacy #292 case) or when no
// matching file is on disk. It does NOT complete the job or record a verdict — the
// caller's finalizeStalledJob does that. Returns the count of artifact rows
// published. A reviewer/verdict-capable job is salvaged the same way (the row is
// what complete-stalled needs); complete-stalled already refuses verdict-capable
// jobs at a prior gate, so this only runs for non-verdict jobs.
func salvagePublishMissingRequiredArtifacts(ctx context.Context, tx db.TxRunner, repositoryID string, job map[string]any) (int, error) {
	jobID := fmt.Sprint(job["job_id"])
	// Already satisfied? Then there is nothing to salvage (the #292 case).
	if err := verifyRequiredArtifacts(ctx, tx, repositoryID, jobID); err == nil {
		return 0, nil
	}
	repoRoot, err := activeRepositoryRoot(ctx, tx, repositoryID)
	if err != nil {
		// Cannot resolve the checkout — leave the missing-row failure to the gate.
		return 0, nil
	}
	// Resolve the dead lane's owning session for the byline check. Prefer the job's
	// current lease, then the most-recent lease on the job resource.
	sessionID := salvageOwnerSessionForJob(ctx, tx, repositoryID, job)
	if sessionID == "" {
		return 0, nil
	}
	expectedByline, err := expectedAuthorLine(ctx, tx, repositoryID, job, sessionID)
	if err != nil {
		return 0, nil
	}
	publishable, _, err := autoPublishableArtifacts(ctx, tx, repositoryID, repoRoot, job, sessionID, expectedByline)
	if err != nil {
		return 0, err
	}
	if len(publishable) == 0 {
		return 0, nil
	}
	leaseID := fmt.Sprint(nullable(job["current_lease_id"]))
	published := 0
	for _, declared := range publishable {
		sourceRoot := repoRoot
		if r, ok := declared["source_root"].(string); ok && r != "" {
			sourceRoot = r
		}
		if _, perr := publishRecoveredArtifact(ctx, tx, repositoryID, job, sessionID, leaseID, sourceRoot, declared); perr != nil {
			return published, perr
		}
		published++
	}
	return published, nil
}

// salvageOwnerSessionForJob resolves the dead lane's owning session for a job
// whose artifact must be salvaged (#530). It prefers the job's current lease, then
// the most-recent lease on the job resource (the dead lane's lease may already be
// released/expired with the job pointer cleared). Returns "" when no owner can be
// resolved.
func salvageOwnerSessionForJob(ctx context.Context, runner any, repositoryID string, job map[string]any) string {
	if leaseID := nullable(job["current_lease_id"]); leaseID != nil {
		row, err := oneRow(ctx, runner, `
			SELECT owner_session_id FROM striatumd.leases
			 WHERE repository_id = $1 AND lease_id = $2 LIMIT 1`, repositoryID, leaseID)
		if err == nil && row != nil {
			if s := nullable(row["owner_session_id"]); s != nil {
				return fmt.Sprint(s)
			}
		}
	}
	row, err := oneRow(ctx, runner, `
		SELECT owner_session_id FROM striatumd.leases
		 WHERE repository_id = $1 AND resource_id = $2 AND owner_session_id IS NOT NULL
		 ORDER BY (state = 'active') DESC, acquired_at DESC, lease_id DESC
		 LIMIT 1`, repositoryID, fmt.Sprint(job["job_id"]))
	if err != nil || row == nil {
		return ""
	}
	if s := nullable(row["owner_session_id"]); s != nil {
		return fmt.Sprint(s)
	}
	return ""
}

// autonomouslyApplicableVerdict reports whether the autonomous stale-lease
// recovery path can cleanly apply a recovered verdict. accept / accept_with_findings
// complete the gate and needs_revision routes the bounded cycle (or opens a
// checkpoint) — none of which error. reject is excluded: its revision-cycle
// self-correction guard returns an error that would roll back the whole sweep, so
// a recovered reject falls back to plain completion instead.
func autonomouslyApplicableVerdict(verdict string) bool {
	switch verdict {
	case "accept", "accept_with_findings", "needs_revision":
		return true
	default:
		return false
	}
}

func publishRecoveredArtifact(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string, leaseID string, repoRoot string, declared map[string]any) (map[string]any, error) {
	kind := fmt.Sprint(declared["kind"])
	logicalName := fmt.Sprint(declared["logical_name"])
	pathText := fmt.Sprint(declared["path"])
	if kind == "transcript" {
		return nil, rpc.NewError("artifact_error", "transcript artifacts are not allowed by default", nil)
	}
	if !allowedArtifactKinds[kind] {
		return nil, rpc.NewError("artifact_error", fmt.Sprintf("artifact kind %q is not in the allowed kinds list", kind), nil)
	}
	applyFrozenAttemptWriteScope(ctx, runner, repositoryID, job, leaseID)
	writeScope := asMap(job["write_scope_json"])
	if !pathAllowed(repoRoot, pathText, writeScope) {
		return nil, writeScopePathError(job, pathText, stringListFromAny(writeScope["allowed_paths"]), stringListFromAny(writeScope["forbidden_paths"]))
	}
	path, err := repoRelativePath(repoRoot, pathText, false)
	if err != nil {
		return nil, rpc.NewError("artifact_error", err.Error(), nil)
	}
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, rpc.NewError("artifact_error", "artifact file does not exist", nil)
	}
	payload, err = ensureRequiredFrontMatter(kind, path, payload)
	if err != nil {
		return nil, err
	}
	if err := validateMarkdownAuthorLine(ctx, runner, repositoryID, job, sessionID, path, payload); err != nil {
		return nil, err
	}
	if err := validateArtifactFrontMatter(kind, path, payload); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	// RFC 0095 §1 / #84: artifacts are attempt-scoped. Surgical recovery must key
	// its collision check (and the INSERT below) on the job's CURRENT attempt, so
	// a re-opened job's prior-attempt row neither mis-fires the conflict nor
	// mis-attributes the recovered artifact to attempt 1.
	attempt := jobAttemptValue(job["attempt"])
	existing, err := oneRow(ctx, runner, `
		SELECT * FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3 AND logical_name = $4
		   AND attempt = $5
		 LIMIT 1`, repositoryID, job["run_id"], job["job_id"], logicalName, attempt)
	if err == nil {
		if fmt.Sprint(existing["content_sha256"]) == digest && fmt.Sprint(existing["repo_path"]) == pathText {
			return map[string]any{"status": "already_published", "artifact_id": existing["artifact_id"]}, nil
		}
		return nil, rpc.NewError("artifact_error", artifactLogicalNameConflictMessage, nil)
	}
	if !errorsIsNoRows(err) {
		return nil, err
	}
	artifactID, err := newID("art")
	if err != nil {
		return nil, err
	}
	now := nowString()
	tx, ok := runner.(db.TxRunner)
	if !ok {
		return nil, fmt.Errorf("runner does not support transactional artifact append")
	}
	// RFC 0110 §7: SD-routed at phase audit_artifacts, direct INSERT before P1.
	if err := db.AppendArtifactInTx(ctx, tx, db.ArtifactRow{
		RepositoryID:  repositoryID,
		ArtifactID:    artifactID,
		RunID:         job["run_id"],
		JobID:         job["job_id"],
		SessionID:     sessionID,
		LogicalName:   logicalName,
		ArtifactKind:  kind,
		RepoPath:      pathText,
		ContentSHA256: digest,
		SizeBytes:     len(payload),
		CreatedAt:     now,
		AuthorLine:    nullable(firstAuthorLine(payload)),
		Attempt:       attempt,
	}); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "artifact.published", sessionID, job["job_id"], nil, artifactID, leaseID, map[string]any{
		"logical_name": logicalName,
		"path":         pathText,
		"sha256":       digest,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"status": "published", "artifact_id": artifactID, "sha256": digest}, nil
}

func completeAutoRecoveredJob(ctx context.Context, runner any, repositoryID, jobID, sessionID, leaseID, messageID string) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(job["state"]) != "stale_lease" && fmt.Sprint(job["state"]) != "running" && fmt.Sprint(job["state"]) != "claimed" {
		return nil, rpc.NewError("invalid_transition", "stale job is no longer auto-recoverable", nil)
	}
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
		return nil, err
	}
	if messageID != "" && messageID != "<nil>" {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $2,
			       current_lease_id = NULL
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return nil, err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET released_at = COALESCE(released_at, $1),
		       release_reason = COALESCE(release_reason, 'auto_published')
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.completed", sessionID, jobID, nullable(messageID), nil, leaseID, map[string]any{
		"summary": "auto-published on stale lease",
	}); err != nil {
		return nil, err
	}
	// #304: a blocked-severity blocker raised on an earlier attempt of this job
	// must not dangle once the auto-recovery path completes it. Resolve the
	// completing job's open autonomous blockers exactly as the normal
	// HandleCompleteWork path does.
	if err := resolveAutonomousBlockersOnCompletion(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), jobID, sessionID, now); err != nil {
		return nil, err
	}
	if err := markJobTerminal(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), jobID); err != nil {
		return nil, err
	}
	if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "job_id": jobID}, nil
}
