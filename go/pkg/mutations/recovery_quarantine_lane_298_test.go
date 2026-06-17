package mutations

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// cancelSeededRun drives the seeded run terminal (canceled) — the state #298
// quarantine-lane applies to — without touching the job/worktree rows. The lane
// worktree stays active so the verb can find a dirty worktree to quarantine.
func cancelSeededRun(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs) {
	t.Helper()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'canceled' WHERE repository_id = $1 AND run_id = $2`,
		ids.repoID, ids.runID); err != nil {
		t.Fatalf("cancel run: %v", err)
	}
}

// dirtyWorktreeFile writes an UNCOMMITTED file into the lane worktree so the
// quarantine snapshot has uncommitted work to preserve, returning its body.
func dirtyWorktreeFile(t *testing.T, worktreeRoot, relPath, body string) {
	t.Helper()
	target := filepath.Join(worktreeRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func quarantineEventCount(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT COUNT(*) AS c
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3 AND event_type = $4`,
		repoID, runID, jobID, LaneQuarantinedEventType)
	if err != nil {
		t.Fatalf("count quarantine events: %v", err)
	}
	switch v := row["c"].(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	default:
		t.Fatalf("count type = %T", row["c"])
		return 0
	}
}

func worktreeStateValue(t *testing.T, ctx context.Context, runner db.Runner, repoID, worktreeID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.job_worktrees WHERE repository_id = $1 AND worktree_id = $2`,
		repoID, worktreeID)
	if err != nil {
		t.Fatalf("read worktree state: %v", err)
	}
	return strings.TrimSpace(stringFromAny(row["state"]))
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// gitTreeContainsBlob reports whether the tree of commit-ish ref contains a file
// at relPath whose content equals body (used to assert the snapshot captured the
// uncommitted change).
func gitTreeContainsBlob(t *testing.T, repoRoot, ref, relPath, body string) bool {
	t.Helper()
	cmd := exec.Command("git", "show", ref+":"+relPath)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return string(out) == body
}

// TestQuarantineLaneSnapshotsDirtyCanceledWorktree is pgtest #1: a canceled run
// with a dirty lane worktree is quarantined — a refs/striatum/quarantine/... ref
// exists whose tree contains the uncommitted change, a recovery.lane_quarantined
// event records it, and the worktree is removed + its row retired.
func TestQuarantineLaneSnapshotsDirtyCanceledWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "ql_dirty", true)
	cancelSeededRun(t, ctx, runner, ids)
	body := "uncommitted lane work\n"
	dirtyWorktreeFile(t, ids.worktreeRoot, "docs/wip.md", body)

	result, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err != nil {
		t.Fatalf("quarantine-lane: %v", err)
	}
	if result["status"] != "quarantined" {
		t.Fatalf("status = %#v, want quarantined", result["status"])
	}
	if result["dirty"] != true {
		t.Fatalf("dirty = %#v, want true", result["dirty"])
	}
	quarantineRef, _ := result["quarantine_ref"].(string)
	if quarantineRef == "" || !strings.HasPrefix(quarantineRef, "refs/striatum/quarantine/") {
		t.Fatalf("quarantine_ref = %#v, want refs/striatum/quarantine/...", result["quarantine_ref"])
	}
	// The quarantine ref exists and its tree contains the uncommitted change.
	if !gitRefExists(ctx, repoRoot, quarantineRef) {
		t.Fatalf("quarantine ref %s does not exist", quarantineRef)
	}
	if !gitTreeContainsBlob(t, repoRoot, quarantineRef, "docs/wip.md", body) {
		t.Fatalf("quarantine ref %s tree missing the uncommitted docs/wip.md", quarantineRef)
	}
	// The event records the snapshot.
	if got := quarantineEventCount(t, ctx, runner, ids.repoID, ids.runID, ids.jobID); got != 1 {
		t.Fatalf("quarantine events = %d, want 1", got)
	}
	// The worktree is gone on disk and its row retired.
	if _, err := os.Stat(ids.worktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists or unexpected stat error: %v", err)
	}
	if state := worktreeStateValue(t, ctx, runner, ids.repoID, ids.worktreeID); state != "removed" {
		t.Fatalf("worktree state = %q, want removed", state)
	}
}

// TestQuarantineLaneDryRunWritesNothing is pgtest #2: --dry-run previews the
// changed paths + would-be quarantine ref and writes nothing (no ref, no event,
// the worktree stays on disk + active).
func TestQuarantineLaneDryRunWritesNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "ql_dry", true)
	cancelSeededRun(t, ctx, runner, ids)
	dirtyWorktreeFile(t, ids.worktreeRoot, "docs/wip.md", "uncommitted\n")

	result, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":  ids.runID,
		"job_id":  ids.jobID,
		"dry_run": true,
	}))
	if err != nil {
		t.Fatalf("dry-run quarantine-lane: %v", err)
	}
	if result["status"] != "would_quarantine" {
		t.Fatalf("status = %#v, want would_quarantine", result["status"])
	}
	changed, _ := result["changed_paths"].([]string)
	if len(changed) == 0 {
		t.Fatalf("changed_paths = %#v, want the uncommitted path previewed", result["changed_paths"])
	}
	quarantineRef, _ := result["quarantine_ref"].(string)
	if quarantineRef == "" {
		t.Fatalf("would-be quarantine_ref not previewed: %#v", result)
	}
	// Nothing was written: no ref, no event, worktree intact + active.
	if gitRefExists(ctx, repoRoot, quarantineRef) {
		t.Fatalf("dry-run created quarantine ref %s", quarantineRef)
	}
	if got := quarantineEventCount(t, ctx, runner, ids.repoID, ids.runID, ids.jobID); got != 0 {
		t.Fatalf("quarantine events = %d, want 0 (dry-run)", got)
	}
	if _, err := os.Stat(ids.worktreeRoot); err != nil {
		t.Fatalf("worktree path should remain after dry-run: %v", err)
	}
	if state := worktreeStateValue(t, ctx, runner, ids.repoID, ids.worktreeID); state != "active" {
		t.Fatalf("worktree state = %q, want active (dry-run)", state)
	}
}

// TestQuarantineLaneIsIdempotent is pgtest #3: a second quarantine of the same
// (run, job) is a no-op (the worktree row was already retired) and writes no
// duplicate event.
func TestQuarantineLaneIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "ql_idem", true)
	cancelSeededRun(t, ctx, runner, ids)
	dirtyWorktreeFile(t, ids.worktreeRoot, "docs/wip.md", "uncommitted\n")

	first, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err != nil {
		t.Fatalf("first quarantine: %v", err)
	}
	if first["status"] != "quarantined" {
		t.Fatalf("first status = %#v, want quarantined", first["status"])
	}

	second, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err != nil {
		t.Fatalf("second quarantine: %v", err)
	}
	if second["status"] != "already_quarantined" {
		t.Fatalf("second status = %#v, want already_quarantined", second["status"])
	}
	// No duplicate event: still exactly one.
	if got := quarantineEventCount(t, ctx, runner, ids.repoID, ids.runID, ids.jobID); got != 1 {
		t.Fatalf("quarantine events = %d, want 1 (no duplicate)", got)
	}
}

// TestQuarantineLaneRefusesNonTerminalRun is pgtest #4: a running (non-terminal)
// run is refused at the run-state gate with invalid_transition; nothing is
// snapshotted and the worktree is untouched.
func TestQuarantineLaneRefusesNonTerminalRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "ql_running", true)
	// Run stays 'running' (non-terminal): not quarantinable.
	dirtyWorktreeFile(t, ids.worktreeRoot, "docs/wip.md", "uncommitted\n")

	_, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("err = %v, want rpc error", err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("code = %q, want invalid_transition", rpcErr.Code)
	}
	// The worktree is untouched (never snapshotted/removed).
	if got := quarantineEventCount(t, ctx, runner, ids.repoID, ids.runID, ids.jobID); got != 0 {
		t.Fatalf("quarantine events = %d, want 0 (refused)", got)
	}
	if _, err := os.Stat(ids.worktreeRoot); err != nil {
		t.Fatalf("worktree path should remain after refusal: %v", err)
	}
	if state := worktreeStateValue(t, ctx, runner, ids.repoID, ids.worktreeID); state != "active" {
		t.Fatalf("worktree state = %q, want active (refused)", state)
	}
}

// TestQuarantineLaneCleanWorktreeIsReportedClean covers the clean-worktree path:
// a terminal run whose lane worktree has NO uncommitted work is reported clean
// (status clean, dirty=false), the worktree is gc'd, and no quarantine ref is
// created.
func TestQuarantineLaneCleanWorktreeIsReportedClean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "ql_clean", true)
	cancelSeededRun(t, ctx, runner, ids)
	// No dirt written: the worktree is clean.

	result, err := HandleRecoveryQuarantineLane(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	if err != nil {
		t.Fatalf("quarantine-lane clean: %v", err)
	}
	if result["status"] != "clean" {
		t.Fatalf("status = %#v, want clean", result["status"])
	}
	if result["dirty"] != false {
		t.Fatalf("dirty = %#v, want false", result["dirty"])
	}
	quarantineRef, _ := result["quarantine_ref"].(string)
	if quarantineRef != "" && gitRefExists(ctx, repoRoot, quarantineRef) {
		t.Fatalf("clean worktree created a quarantine ref %s", quarantineRef)
	}
	// The clean worktree is gc'd.
	if _, err := os.Stat(ids.worktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("clean worktree path should be removed: %v", err)
	}
	if state := worktreeStateValue(t, ctx, runner, ids.repoID, ids.worktreeID); state != "removed" {
		t.Fatalf("worktree state = %q, want removed", state)
	}
}

// TestWorktreeGCSkipsDirtyTerminalWorktree is pgtest #5: gc does NOT
// --force-discard a dirty terminal-run worktree — it skips with reason
// dirty_uncommitted_work, leaves the worktree on disk, and reports the deferred
// recovery verb (closing the silent-data-loss).
func TestWorktreeGCSkipsDirtyTerminalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "gc_dirty", true)
	// Terminal job (gc only considers terminal jobs) + a confirmed reachable HEAD
	// (HEAD == run branch tip from the --detach seed), then make it DIRTY.
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	dirtyWorktreeFile(t, ids.worktreeRoot, "docs/wip.md", "uncommitted gc dirt\n")

	result, err := HandleWorktreeGC(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	if err != nil {
		t.Fatalf("HandleWorktreeGC: %v", err)
	}
	if result["removed_count"] != 0 || result["skipped_count"] != 1 {
		t.Fatalf("gc result = %#v, want one dirty skip (no removal)", result)
	}
	skipped := result["skipped"].([]map[string]any)
	if len(skipped) != 1 || skipped[0]["reason"] != "dirty_uncommitted_work" {
		t.Fatalf("skipped = %#v, want dirty_uncommitted_work", skipped)
	}
	dirtyPaths, _ := skipped[0]["dirty_paths"].([]string)
	if len(dirtyPaths) == 0 {
		t.Fatalf("skipped dirty_paths = %#v, want the uncommitted path", skipped[0]["dirty_paths"])
	}
	if skipped[0]["next_action"] != "recovery quarantine-lane <run-id> <job-id>" {
		t.Fatalf("skipped next_action = %#v, want quarantine-lane deferral", skipped[0]["next_action"])
	}
	// The worktree was NOT discarded — its uncommitted work survives on disk.
	if _, err := os.Stat(ids.worktreeRoot); err != nil {
		t.Fatalf("dirty worktree path should remain (not --force-discarded): %v", err)
	}
	if state := worktreeStateValue(t, ctx, runner, ids.repoID, ids.worktreeID); state != "active" {
		t.Fatalf("worktree state = %q, want active (gc skipped the dirty worktree)", state)
	}
}
