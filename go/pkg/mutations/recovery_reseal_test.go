package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestRecoveryResealRequeuesSameAttemptWhenBodyDurable is the RFC 0125 P1-2
// (#271/#273) happy path: a repo-write job whose required artifact body was made
// durable (committed to the worktree HEAD) is, after recovery.reseal, requeued
// on the SAME attempt — jobs.attempt is unchanged, no duplicate attempt rows are
// minted, the artifact row stays single-per-logical-name at that attempt, and a
// fresh session can claim the requeued (pending) work message.
func TestRecoveryResealRequeuesSameAttemptWhenBodyDurable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "reseal_durable", true)

	payload := []byte("resealed artifact body\n")
	// Make the published body durable: commit docs/out.txt into the per-job
	// worktree HEAD with the exact bytes the artifact row records.
	commitWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", string(payload))
	seedPublishedArtifact(t, ctx, runner, ids, "art_reseal_durable", "out", "docs/out.txt", payload, nil)

	// Precondition snapshot: the live attempt.
	attemptBefore := intField(t, ctx, runner, ids, "attempt")

	result, err := HandleRecoveryReseal(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err != nil {
		t.Fatalf("HandleRecoveryReseal (durable body): %v", err)
	}
	if result["status"] != "resealed" {
		t.Fatalf("reseal status = %#v, want resealed", result["status"])
	}
	if result["new_state"] != "queued" {
		t.Fatalf("reseal new_state = %#v, want queued", result["new_state"])
	}

	// attempt UNCHANGED (no bump, contrast run.retry_job / reopenJobForAttempt).
	attemptAfter := intField(t, ctx, runner, ids, "attempt")
	if attemptAfter != attemptBefore {
		t.Fatalf("attempt bumped %d -> %d; reseal must not bump the attempt", attemptBefore, attemptAfter)
	}
	if got := fmt.Sprint(result["attempt"]); got != fmt.Sprint(attemptBefore) {
		t.Fatalf("result attempt = %s, want unchanged %d", got, attemptBefore)
	}

	// Exactly one artifacts row per logical_name at that attempt (no duplicate
	// provenance).
	count := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND logical_name = 'out' AND attempt = $4`,
		ids.repoID, ids.runID, ids.jobID, attemptBefore)
	if count != 1 {
		t.Fatalf("artifacts rows for logical_name=out attempt=%d = %d, want exactly 1", attemptBefore, count)
	}

	// The job is requeued and claimable: state=queued with a live pending work
	// message.
	jobRow, err := oneRow(ctx, runner, `
		SELECT state, current_message_id FROM striatumd.jobs
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("read job row: %v", err)
	}
	if jobRow["state"] != "queued" {
		t.Fatalf("job state = %#v, want queued", jobRow["state"])
	}
	pending := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND job_id = $2 AND kind = 'work' AND state = 'pending'`,
		ids.repoID, ids.jobID)
	if pending != 1 {
		t.Fatalf("pending work messages = %d, want exactly 1 (claimable)", pending)
	}

	// A durable recovery.resealed event was emitted on the same attempt.
	evtAttempt := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND event_type = 'recovery.resealed'`,
		ids.repoID, ids.runID, ids.jobID)
	if evtAttempt != 1 {
		t.Fatalf("recovery.resealed events = %d, want exactly 1", evtAttempt)
	}
}

// TestRecoveryResealRefusesWhenBodyStillNotDurable asserts the gate: when the
// durability probe still reports problems (the published body is NOT committed
// to HEAD), recovery.reseal REFUSES with an invalid_transition listing the
// undurable artifacts rather than silently completing, and it leaves the job's
// attempt untouched.
func TestRecoveryResealRefusesWhenBodyStillNotDurable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "reseal_undurable", true)

	payload := []byte("uncommitted artifact body\n")
	// Write the file into the worktree but do NOT commit it: the body is not
	// reconstructable from HEAD, so the durability probe must still flag it.
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	seedPublishedArtifact(t, ctx, runner, ids, "art_reseal_undurable", "out", "docs/out.txt", payload, nil)

	attemptBefore := intField(t, ctx, runner, ids, "attempt")

	_, err := HandleRecoveryReseal(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err == nil {
		t.Fatalf("reseal succeeded with an undurable body; want refusal")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok {
		t.Fatalf("reseal error = %T (%v), want *rpc.Error", err, err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("reseal error code = %q, want invalid_transition", rpcErr.Code)
	}
	if rpcErr.Details == nil || rpcErr.Details["artifacts"] == nil {
		t.Fatalf("reseal refusal details = %#v, want an artifacts list naming the undurable bodies", rpcErr.Details)
	}

	// The attempt is untouched after a refusal.
	attemptAfter := intField(t, ctx, runner, ids, "attempt")
	if attemptAfter != attemptBefore {
		t.Fatalf("attempt changed %d -> %d on a refused reseal", attemptBefore, attemptAfter)
	}
	// No reseal event was emitted.
	evts := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND event_type = 'recovery.resealed'`,
		ids.repoID, ids.runID, ids.jobID)
	if evts != 0 {
		t.Fatalf("recovery.resealed events on a refused reseal = %d, want 0", evts)
	}
}

func intField(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, column string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, fmt.Sprintf(`
		SELECT %s AS v FROM striatumd.jobs
		 WHERE repository_id = $1 AND job_id = $2`, column), ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("read job %s: %v", column, err)
	}
	return intValue(row["v"])
}

func scalarInt(t *testing.T, ctx context.Context, runner db.Runner, query string, args ...any) int {
	t.Helper()
	row, err := oneRow(ctx, runner, query, args...)
	if err != nil {
		t.Fatalf("scalar query: %v", err)
	}
	for _, v := range row {
		return intValue(v)
	}
	t.Fatalf("scalar query returned no columns")
	return 0
}
