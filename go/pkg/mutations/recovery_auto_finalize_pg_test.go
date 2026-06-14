package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// blockSeededWorktreeJob transitions a seeded worktree-required job into the
// blocked state the way work.report(block) does: it releases the active lease,
// clears current_lease_id, sets the job state to 'blocked', blocks the work
// message, and inserts an open blocker row. This reproduces the #274 starting
// condition — a job whose required artifact lives in the active per-job worktree
// but is no longer covered by a live lease.
func blockSeededWorktreeJob(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, blockerKind string) string {
	t.Helper()
	now := time.Now().UTC()
	blockerID := "blk_" + ids.jobID
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'blocked', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("block job: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'blocked'
		 WHERE repository_id = $2 AND lease_id = $3`, now, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'blocked', current_lease_id = NULL
		 WHERE repository_id = $1 AND message_id = $2`, ids.repoID, ids.messageID); err != nil {
		t.Fatalf("block message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		) VALUES ($1,$2,$3,$4,$5,'blocked',$6,'worktree durability','open',$7,'{}'::jsonb)`,
		ids.repoID, blockerID, ids.runID, ids.jobID, ids.sessionID, blockerKind, now); err != nil {
		t.Fatalf("insert blocker: %v", err)
	}
	return blockerID
}

func findSkipForJob(skipped []any, jobID string) (map[string]any, bool) {
	for _, item := range skipped {
		skip := asMap(item)
		if fmt.Sprint(skip["job_id"]) == jobID {
			return skip, true
		}
	}
	return nil, false
}

// TestAutoFinalizeExplainsBlockedJobWithUndurableArtifact is the #274
// (RFC 0125 P1-3) legibility regression: when a run has a blocked /
// lease-released job whose published artifact is not durable in the worktree
// HEAD, `recovery auto-finalize --dry-run` previously returned
// eligible_count:0 with an empty skipped[]. It must now name the job in
// skipped[] with the published_artifact_not_durable reason and a `recovery
// reseal` remediation hint — while leaving eligible_count at 0 (the blocked job
// is still not finalizable).
func TestAutoFinalizeExplainsBlockedJobWithUndurableArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "auto_finalize_blocked_undurable", true)

	// Publish an artifact whose body lives in the worktree but is NOT committed,
	// so it cannot be reconstructed from the worktree HEAD (the #274 hazard).
	payload := []byte("published but not committed\n")
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	seedPublishedArtifact(t, ctx, runner, ids, "art_blocked_undurable", "out", "docs/out.txt", payload, nil)

	blockSeededWorktreeJob(t, ctx, runner, ids, "worktree_durability")

	result, err := HandleRecoveryAutoFinalize(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":  ids.runID,
		"dry_run": true,
	}))
	if err != nil {
		t.Fatalf("recovery auto-finalize: %v", err)
	}

	if got := intValue(result["eligible_count"]); got != 0 {
		t.Fatalf("eligible_count = %d, want 0 (blocked job must not become a live candidate)", got)
	}
	if got := intValue(result["finalized_count"]); got != 0 {
		t.Fatalf("finalized_count = %d, want 0 in dry-run", got)
	}

	skipped := asList(result["skipped"])
	if got := intValue(result["skipped_count"]); got != len(skipped) {
		t.Fatalf("skipped_count = %d, but len(skipped) = %d", got, len(skipped))
	}
	skip, ok := findSkipForJob(skipped, ids.jobID)
	if !ok {
		t.Fatalf("skipped[] does not name blocked job %s: %#v", ids.jobID, skipped)
	}
	if got := fmt.Sprint(skip["reason"]); got != autoFinalizeReasonPublishedArtifactNotDurable {
		t.Fatalf("skip reason = %q, want %q", got, autoFinalizeReasonPublishedArtifactNotDurable)
	}
	if got := fmt.Sprint(skip["remediation"]); got != "recovery reseal" {
		t.Fatalf("skip remediation = %q, want \"recovery reseal\"", got)
	}
	explanation := fmt.Sprint(skip["explanation"])
	if !strings.Contains(explanation, "recovery reseal") {
		t.Fatalf("explanation must point at recovery reseal: %q", explanation)
	}
	if !strings.Contains(explanation, "not durable") {
		t.Fatalf("explanation must mention durability: %q", explanation)
	}
	artifacts := asList(skip["artifacts"])
	if len(artifacts) == 0 {
		t.Fatalf("skip must list the undurable artifact(s): %#v", skip)
	}
	foundArtifact := false
	for _, item := range artifacts {
		a := asMap(item)
		if fmt.Sprint(a["artifact_id"]) == "art_blocked_undurable" {
			foundArtifact = true
			if got := fmt.Sprint(a["reason"]); got != "missing_from_head" {
				t.Fatalf("artifact durability reason = %q, want missing_from_head", got)
			}
		}
	}
	if !foundArtifact {
		t.Fatalf("undurable artifact art_blocked_undurable not named in skip artifacts: %#v", artifacts)
	}
	if got := fmt.Sprint(skip["blocker_kind"]); got != "worktree_durability" {
		t.Fatalf("skip blocker_kind = %q, want worktree_durability", got)
	}
}

// TestAutoFinalizeExplainsBlockedJobWithoutUndurableArtifact covers the second
// #274 branch: a blocked job whose published artifact IS durable (committed in
// the worktree HEAD) falls back to the generic `blocked` reason with an
// inspect-blocker hint rather than the reseal hint.
func TestAutoFinalizeExplainsBlockedJobWithoutUndurableArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "auto_finalize_blocked_durable", true)

	// Commit the artifact body in the worktree so it IS reconstructable from HEAD.
	payload := []byte("published and committed\n")
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, ids.worktreeRoot, "add", "docs/out.txt")
	gitRun(t, ids.worktreeRoot, "commit", "-q", "-m", "artifact")
	seedPublishedArtifact(t, ctx, runner, ids, "art_blocked_durable", "out", "docs/out.txt", payload, nil)

	blockSeededWorktreeJob(t, ctx, runner, ids, "missing_input")

	result, err := HandleRecoveryAutoFinalize(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":  ids.runID,
		"dry_run": true,
	}))
	if err != nil {
		t.Fatalf("recovery auto-finalize: %v", err)
	}
	if got := intValue(result["eligible_count"]); got != 0 {
		t.Fatalf("eligible_count = %d, want 0", got)
	}
	skip, ok := findSkipForJob(asList(result["skipped"]), ids.jobID)
	if !ok {
		t.Fatalf("skipped[] does not name blocked job %s: %#v", ids.jobID, result["skipped"])
	}
	if got := fmt.Sprint(skip["reason"]); got != autoFinalizeReasonBlocked {
		t.Fatalf("skip reason = %q, want %q (durable artifact ⇒ no reseal path)", got, autoFinalizeReasonBlocked)
	}
	if got := fmt.Sprint(skip["remediation"]); got != "inspect_blocker" {
		t.Fatalf("skip remediation = %q, want inspect_blocker", got)
	}
	if _, hasArtifacts := skip["artifacts"]; hasArtifacts {
		t.Fatalf("durable-artifact skip must not carry durability problems: %#v", skip)
	}
	explanation := fmt.Sprint(skip["explanation"])
	if !strings.Contains(explanation, "missing_input") {
		t.Fatalf("explanation should cite the blocker kind: %q", explanation)
	}
}
