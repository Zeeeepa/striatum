package reads

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// asRPCError is errors.As specialized to *rpc.Error for the barrier verify tests.
func asRPCError(err error, target **rpc.Error) bool {
	return errors.As(err, target)
}

// RFC 0135 P3 (#347) — PG-backed tests for the barrier doctor invariant, the
// BARRIER_BLOCKED named condition + blocked_manifest, the migration-0031
// striatumd.barrier_status view, and `striatum join verify` (HandleJoinVerify).
//
// These exercise the fires-on-bad / quiet-on-good discipline: the invariant must
// stay SILENT on a healthy barrier and FIRE on a corrupted / blocked one, and
// `join verify` must pass on a good barrier and fail on a tampered one.

func seedBarrierRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID, repoRoot string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+repoID, repoRoot, repoRoot+"/.striatum", now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha','{}'::jsonb,$3)`,
		repoID, "snap_"+repoID, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
}

func seedBarrierRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, repoRoot string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,$4,'running',$5)`,
		repoID, runID, "snap_"+repoID, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}

func setBarrierRunState(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = $3, completed_at = now()
		 WHERE repository_id = $1 AND run_id = $2`,
		repoID, runID, state); err != nil {
		t.Fatalf("set run state %s: %v", state, err)
	}
}

func seedBarrierJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, state string, attempt int) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, attempt, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'Build','build','implementer',$5,$6,$7,$8)`,
		repoID, jobID, runID, workflowJobID, state, attempt, "idem_"+jobID, now); err != nil {
		t.Fatalf("insert job %s: %v", jobID, err)
	}
}

func seedBarrierFreeze(t *testing.T, ctx context.Context, runner db.Runner, repoID, barrierID, runID, downstream string, seats []string, frozenTip string) {
	t.Helper()
	declared, err := db.JSONBArg(runner, seats)
	if err != nil {
		t.Fatalf("marshal seats: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.fanin_freeze_points
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id, frozen_tip_sha, declared_sibling_job_ids)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)`,
		repoID, barrierID, runID, downstream, frozenTip, declared); err != nil {
		t.Fatalf("insert freeze: %v", err)
	}
}

func seedBarrierStaged(t *testing.T, ctx context.Context, runner db.Runner, repoID, barrierID, runID, workflowJobID, jobID string, attempt int) {
	t.Helper()
	ref := "refs/striatum/staged/" + runID + "/" + workflowJobID + "/" + itoa(attempt)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.barrier_staged_contributions
		  (repository_id, barrier_id, run_id, workflow_job_id, attempt, job_id, staging_ref, commit_sha, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'staged')`,
		repoID, barrierID, runID, workflowJobID, attempt, jobID, ref,
		"1111111111111111111111111111111111111111"); err != nil {
		t.Fatalf("insert staged %s: %v", workflowJobID, err)
	}
}

func seedBarrierDependency(t *testing.T, ctx context.Context, runner db.Runner, repoID, jobID, dependsOnJobID string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`,
		repoID, jobID, dependsOnJobID); err != nil {
		t.Fatalf("insert dependency %s -> %s: %v", jobID, dependsOnJobID, err)
	}
}

func seedCommittedManifestMismatch(t *testing.T, ctx context.Context, runner db.Runner, repoID, barrierID string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.barrier_state
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id, state,
		   target_commit_sha, tree_sha, committed_at, updated_at)
		SELECT repository_id, barrier_id, run_id, downstream_workflow_job_id,
		       'committed',
		       '2222222222222222222222222222222222222222',
		       '3333333333333333333333333333333333333333',
		       now(), now()
		  FROM striatumd.fanin_freeze_points
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repoID, barrierID); err != nil {
		t.Fatalf("insert committed barrier_state: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.barrier_staged_contributions
		   SET status = 'voided', voided_at = now(), void_reason = 'tamper'
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = 'author_b'`,
		repoID, barrierID); err != nil {
		t.Fatalf("void author_b staged row: %v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// TestBarrierStatusViewReturnsExpectedRows exercises the migration-0031 view: a
// fully-staged barrier projects condition READY with the right seat counts; an
// outstanding seat keeps it PENDING.
func TestBarrierStatusViewReturnsExpectedRows(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bstatus_view"
	runID := "run_bstatus_view"
	barrierID := "barrier_bstatus_view"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "running", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)

	rows, err := collectRows(ctx, runner, `
		SELECT declared_seats, staged_seats, outstanding_seats, blocking_seats, condition
		  FROM striatumd.barrier_status
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repoID, barrierID)
	if err != nil {
		t.Fatalf("query barrier_status: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 barrier_status row, got %d", len(rows))
	}
	r := rows[0]
	if got := intFromAny(r["declared_seats"]); got != 2 {
		t.Fatalf("declared_seats = %d, want 2", got)
	}
	if got := intFromAny(r["staged_seats"]); got != 1 {
		t.Fatalf("staged_seats = %d, want 1 (only author_a staged at live seal)", got)
	}
	if got := intFromAny(r["outstanding_seats"]); got != 1 {
		t.Fatalf("outstanding_seats = %d, want 1 (author_b running, unstaged)", got)
	}
	if got := stringFrom(r, "condition"); got != "PENDING" {
		t.Fatalf("condition = %q, want PENDING (author_b outstanding)", got)
	}

	// Complete author_b (its running job finishes) + stage it → READY.
	if err := runner.Exec(ctx, `UPDATE striatumd.jobs SET state='completed'
		WHERE repository_id=$1 AND run_id=$2 AND workflow_job_id='author_b'`, repoID, runID); err != nil {
		t.Fatalf("complete author_b: %v", err)
	}
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)
	rows, err = collectRows(ctx, runner, `
		SELECT staged_seats, outstanding_seats, condition
		  FROM striatumd.barrier_status WHERE repository_id = $1 AND barrier_id = $2`,
		repoID, barrierID)
	if err != nil {
		t.Fatalf("re-query barrier_status: %v", err)
	}
	if got := intFromAny(rows[0]["staged_seats"]); got != 2 {
		t.Fatalf("staged_seats = %d, want 2 after staging both", got)
	}
	if got := stringFrom(rows[0], "condition"); got != "READY" {
		t.Fatalf("condition = %q, want READY", got)
	}
}

// TestDoctorBarrierQuietOnHealthyBarrier asserts the invariant stays SILENT (no
// barrier_* problems, ok stays true for the barrier dimension) on a fully-staged,
// in-flight barrier.
func TestDoctorBarrierQuietOnHealthyBarrier(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bdoc_quiet"
	runID := "run_bdoc_quiet"
	barrierID := "barrier_bdoc_quiet"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "completed", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)

	_, problems, _, _, _ := doctorBarrierIntegrity(ctx, runner, repoID)
	for _, p := range problems {
		if strings.HasPrefix(p, "barrier_") {
			t.Fatalf("healthy barrier produced a problem: %q", p)
		}
	}
}

// TestDoctorBarrierFiresOnBlockedBarrier asserts BARRIER_BLOCKED fires with a
// blocked_manifest when an in-edge seat's job is in a blocking state.
func TestDoctorBarrierFiresOnBlockedBarrier(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bdoc_blocked"
	runID := "run_bdoc_blocked"
	barrierID := "barrier_bdoc_blocked"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	// author_b is BLOCKED — a live blocking in-edge, not a clean terminal gap.
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "blocked", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)

	block, problems, records, _, _ := doctorBarrierIntegrity(ctx, runner, repoID)
	foundBlocked := false
	for _, p := range problems {
		if strings.HasPrefix(p, "barrier_blocked.") {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Fatalf("expected barrier_blocked problem, got: %v", problems)
	}
	// The verbose record must carry a blocked_manifest naming author_b.
	foundManifest := false
	for _, rec := range records {
		if rec["check"] != "barrier_blocked" {
			continue
		}
		ctxMap, _ := rec["context"].(map[string]any)
		manifest, _ := ctxMap["blocked_manifest"].([]map[string]any)
		for _, e := range manifest {
			if stringFrom(e, "workflow_job_id") == "author_b" {
				foundManifest = true
				if stringFrom(e, "reason") == "" {
					t.Fatalf("blocked in-edge author_b has empty reason")
				}
			}
		}
	}
	if !foundManifest {
		t.Fatalf("blocked_manifest did not name author_b; records=%v", records)
	}
	// The barrier_status condition must be BARRIER_BLOCKED.
	rows, _ := collectRows(ctx, runner, `SELECT condition FROM striatumd.barrier_status WHERE repository_id=$1 AND barrier_id=$2`, repoID, barrierID)
	if len(rows) == 0 || stringFrom(rows[0], "condition") != "BARRIER_BLOCKED" {
		t.Fatalf("view condition not BARRIER_BLOCKED: %v", rows)
	}
	_ = block
}

// TestDoctorBarrierIgnoresDependencyBlockedSeats covers the live RFC 0170 v2
// shape: downstream fan-in seats start in jobs.state='blocked' while they are only
// waiting for upstream dependencies. That scheduler-blocked state is not an
// intervention-needed BARRIER_BLOCKED in-edge.
func TestDoctorBarrierIgnoresDependencyBlockedSeats(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bdoc_dependency_blocked"
	runID := "run_bdoc_dependency_blocked"
	barrierID := "barrier_bdoc_dependency_blocked"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_holder", "holder", "running", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_falsifier_1", "falsifier_1", "blocked", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_falsifier_2", "falsifier_2", "blocked", 1)
	seedBarrierDependency(t, ctx, runner, repoID, "job_falsifier_1", "job_holder")
	seedBarrierDependency(t, ctx, runner, repoID, "job_falsifier_2", "job_falsifier_1")
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "adjudicate", []string{"holder", "falsifier_1", "falsifier_2"}, "0000000000000000000000000000000000000000")

	_, problems, records, _, _ := doctorBarrierIntegrity(ctx, runner, repoID)
	for _, p := range problems {
		if strings.HasPrefix(p, "barrier_blocked.") {
			t.Fatalf("dependency-blocked seat must not red doctor as barrier_blocked: problems=%v records=%v", problems, records)
		}
	}
	rows, _ := collectRows(ctx, runner, `SELECT blocking_seats, condition FROM striatumd.barrier_status WHERE repository_id=$1 AND barrier_id=$2`, repoID, barrierID)
	if len(rows) != 1 {
		t.Fatalf("expected one barrier_status row, got %d", len(rows))
	}
	if got := intFromAny(rows[0]["blocking_seats"]); got != 0 {
		t.Fatalf("blocking_seats = %d, want 0 for dependency-blocked seats", got)
	}
	if got := stringFrom(rows[0], "condition"); got != "PENDING" {
		t.Fatalf("condition = %q, want PENDING", got)
	}
}

func TestDoctorBarrierCommittedMismatchActiveRunStillReds(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bdoc_mismatch_active"
	runID := "run_bdoc_mismatch_active"
	barrierID := "barrier_bdoc_mismatch_active"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "completed", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)
	seedCommittedManifestMismatch(t, ctx, runner, repoID, barrierID)

	_, problems, _, warnings, _ := doctorBarrierIntegrity(ctx, runner, repoID)
	found := false
	for _, p := range problems {
		if strings.HasPrefix(p, "barrier_committed_manifest_mismatch."+barrierID) {
			found = true
		}
	}
	if !found {
		t.Fatalf("active run mismatch must red doctor; problems=%v warnings=%v", problems, warnings)
	}
	for _, w := range warnings {
		if strings.HasPrefix(w, "barrier_debris_terminal_run."+barrierID) {
			t.Fatalf("active run mismatch must not be terminal debris: %q", w)
		}
	}
}

func TestDoctorBarrierCommittedMismatchTerminalRunIsWarning(t *testing.T) {
	for _, terminalState := range []string{"canceled", "completed"} {
		t.Run(terminalState, func(t *testing.T) {
			ctx := context.Background()
			pool := pgtest.Pool(t)
			runner := pool.Runner

			repoID := "repo_bdoc_mismatch_" + terminalState
			runID := "run_bdoc_mismatch_" + terminalState
			barrierID := "barrier_bdoc_mismatch_" + terminalState
			repoRoot := t.TempDir()

			seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
			seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
			seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
			seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "completed", 1)
			seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
			seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)
			seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)
			seedCommittedManifestMismatch(t, ctx, runner, repoID, barrierID)
			setBarrierRunState(t, ctx, runner, repoID, runID, terminalState)

			_, problems, _, warnings, warningRecords := doctorBarrierIntegrity(ctx, runner, repoID)
			for _, p := range problems {
				if strings.HasPrefix(p, "barrier_committed_manifest_mismatch."+barrierID) {
					t.Fatalf("terminal %s run mismatch must not red doctor: %v", terminalState, problems)
				}
			}
			foundWarning := false
			foundRecord := false
			for _, w := range warnings {
				if strings.HasPrefix(w, "barrier_debris_terminal_run."+barrierID) &&
					strings.Contains(w, "barrier_committed_manifest_mismatch") {
					foundWarning = true
				}
			}
			for _, rec := range warningRecords {
				if rec["check"] != "barrier_debris_terminal_run" {
					continue
				}
				ctxMap, _ := rec["context"].(map[string]any)
				if stringFrom(ctxMap, "original_check") == "barrier_committed_manifest_mismatch" &&
					stringFrom(ctxMap, "run_state") == terminalState {
					foundRecord = true
				}
			}
			if !foundWarning || !foundRecord {
				t.Fatalf("terminal mismatch must be warning debris; warnings=%v warningRecords=%v", warnings, warningRecords)
			}
		})
	}
}

// TestDoctorBarrierFiresOnOrphanedStagingRef asserts a staged contribution with no
// freeze record is reported as orphaned debris.
func TestDoctorBarrierFiresOnOrphanedStagingRef(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_bdoc_orphan"
	runID := "run_bdoc_orphan"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	// A staged contribution for a barrier that has NO freeze record (orphan).
	seedBarrierStaged(t, ctx, runner, repoID, "barrier_orphan", runID, "author_x", "job_x", 1)

	_, problems, _, _, _ := doctorBarrierIntegrity(ctx, runner, repoID)
	found := false
	for _, p := range problems {
		if strings.HasPrefix(p, "barrier_orphaned_staging_ref.barrier_orphan") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected barrier_orphaned_staging_ref problem, got: %v", problems)
	}
}

// TestJoinVerifyPassesOnGoodBarrier asserts HandleJoinVerify returns valid=true on
// a fully-staged barrier.
func TestJoinVerifyPassesOnGoodBarrier(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_jv_good"
	runID := "run_jv_good"
	barrierID := "barrier_jv_good"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "completed", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)

	result, err := HandleJoinVerify(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID, "barrier_id": barrierID,
	}})
	if err != nil {
		t.Fatalf("HandleJoinVerify on good barrier: %v", err)
	}
	if result["valid"] != true {
		t.Fatalf("valid = %v, want true; problems=%v", result["valid"], result["problems"])
	}
	if got := intFromAny(result["staged_seats"]); got != 2 {
		t.Fatalf("staged_seats = %d, want 2", got)
	}
}

// TestJoinVerifyFailsOnTamperedBarrier asserts HandleJoinVerify returns a
// barrier_integrity_failed error when a staged ref is voided out from under a
// committed barrier (manifest no longer matches the staged refs at the live seal).
func TestJoinVerifyFailsOnTamperedBarrier(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_jv_tampered"
	runID := "run_jv_tampered"
	barrierID := "barrier_jv_tampered"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_b1", "author_b", "completed", 1)
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_a", "job_a1", 1)
	seedBarrierStaged(t, ctx, runner, repoID, barrierID, runID, "author_b", "job_b1", 1)
	// Mark the barrier committed, then TAMPER: void author_b's staged ref out from
	// under the committed commit. The manifest no longer matches the staged refs.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.barrier_state
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id, state,
		   target_commit_sha, tree_sha, committed_at, updated_at)
		VALUES ($1,$2,$3,'synth','committed',
		  '2222222222222222222222222222222222222222',
		  '3333333333333333333333333333333333333333', now(), now())`,
		repoID, barrierID, runID); err != nil {
		t.Fatalf("insert committed barrier_state: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.barrier_staged_contributions
		   SET status = 'voided', voided_at = now(), void_reason = 'tamper'
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = 'author_b'`,
		repoID, barrierID); err != nil {
		t.Fatalf("void author_b staged row: %v", err)
	}

	_, err := HandleJoinVerify(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID, "barrier_id": barrierID,
	}})
	if err == nil {
		t.Fatal("expected HandleJoinVerify to FAIL on a tampered committed barrier, got nil error")
	}
	var rpcErr *rpc.Error
	if !asRPCError(err, &rpcErr) {
		t.Fatalf("expected an rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != "barrier_integrity_failed" {
		t.Fatalf("error code = %q, want barrier_integrity_failed", rpcErr.Code)
	}

	// And on a BLOCKED barrier the verb must fail with barrier_blocked.
	repoID2 := "repo_jv_blocked"
	runID2 := "run_jv_blocked"
	barrierID2 := "barrier_jv_blocked"
	repoRoot2 := t.TempDir()
	seedBarrierRepo(t, ctx, runner, repoID2, repoRoot2)
	seedBarrierRun(t, ctx, runner, repoID2, runID2, repoRoot2)
	seedBarrierJob(t, ctx, runner, repoID2, runID2, "job_a1", "author_a", "completed", 1)
	seedBarrierJob(t, ctx, runner, repoID2, runID2, "job_b1", "author_b", "waiting_human", 1)
	seedBarrierFreeze(t, ctx, runner, repoID2, barrierID2, runID2, "synth", []string{"author_a", "author_b"}, "0000000000000000000000000000000000000000")
	seedBarrierStaged(t, ctx, runner, repoID2, barrierID2, runID2, "author_a", "job_a1", 1)

	_, err2 := HandleJoinVerify(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID2, "barrier_id": barrierID2,
	}})
	if err2 == nil {
		t.Fatal("expected HandleJoinVerify to FAIL on a blocked barrier")
	}
	var rpcErr2 *rpc.Error
	if !asRPCError(err2, &rpcErr2) {
		t.Fatalf("expected an rpc.Error, got %T: %v", err2, err2)
	}
	if rpcErr2.Code != "barrier_blocked" {
		t.Fatalf("blocked error code = %q, want barrier_blocked", rpcErr2.Code)
	}
	manifest, _ := rpcErr2.Details["blocked_manifest"].([]map[string]any)
	if len(manifest) == 0 {
		t.Fatalf("barrier_blocked error missing blocked_manifest detail")
	}
}

func TestJoinVerifyDependencyBlockedBarrierIsIncompleteNotBlocked(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_jv_dependency_blocked"
	runID := "run_jv_dependency_blocked"
	barrierID := "barrier_jv_dependency_blocked"
	repoRoot := t.TempDir()

	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)
	seedBarrierRun(t, ctx, runner, repoID, runID, repoRoot)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_holder", "holder", "running", 1)
	seedBarrierJob(t, ctx, runner, repoID, runID, "job_falsifier", "falsifier", "blocked", 1)
	seedBarrierDependency(t, ctx, runner, repoID, "job_falsifier", "job_holder")
	seedBarrierFreeze(t, ctx, runner, repoID, barrierID, runID, "adjudicate", []string{"holder", "falsifier"}, "0000000000000000000000000000000000000000")

	_, err := HandleJoinVerify(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID, "barrier_id": barrierID,
	}})
	if err == nil {
		t.Fatal("expected HandleJoinVerify to fail while the barrier is incomplete")
	}
	var rpcErr *rpc.Error
	if !asRPCError(err, &rpcErr) {
		t.Fatalf("expected an rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != "barrier_integrity_failed" {
		t.Fatalf("error code = %q, want barrier_integrity_failed", rpcErr.Code)
	}
	manifest, _ := rpcErr.Details["blocked_manifest"].([]map[string]any)
	if len(manifest) != 0 {
		t.Fatalf("dependency-blocked barrier returned blocked_manifest=%v, want empty", manifest)
	}
}

// TestJoinVerifyMissingBarrierIsNotFound asserts a non-existent barrier returns
// not_found.
func TestJoinVerifyMissingBarrierIsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_jv_404"
	repoRoot := t.TempDir()
	seedBarrierRepo(t, ctx, runner, repoID, repoRoot)

	_, err := HandleJoinVerify(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID, "barrier_id": "barrier_does_not_exist",
	}})
	if err == nil {
		t.Fatal("expected not_found error for a missing barrier")
	}
	var rpcErr *rpc.Error
	if !asRPCError(err, &rpcErr) || rpcErr.Code != "not_found" {
		t.Fatalf("expected not_found, got %v", err)
	}
}
