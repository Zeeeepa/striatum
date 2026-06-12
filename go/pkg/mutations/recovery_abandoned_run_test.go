package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestSweepAutoCancelsAbandonedRun(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_abandoned_auto_cancel"
	runID := "run_abandoned_auto_cancel"
	jobID := "job_abandoned_auto_cancel"
	old := time.Now().UTC().Add(-25 * time.Hour)

	seedAbandonedRunCandidate(t, ctx, runner, repoID, runID, jobID, old)
	seedRunEventAt(t, ctx, runner, repoID, runID, "run.started", old)
	seedRunEventAt(t, ctx, runner, repoID, runID, "daemon.recovery_sweep", time.Now().UTC())

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	abandoned := asMap(result["abandoned_run"])
	if abandoned["status"] != "auto_canceled" {
		t.Fatalf("abandoned_run = %#v, want auto_canceled; result=%#v", abandoned, result)
	}
	run, err := oneRow(ctx, runner, `
		SELECT state, stop_reason, completion_record_json
		  FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if run["state"] != "canceled" || run["stop_reason"] != abandonedRunAutoCancelReason {
		t.Fatalf("run row = %#v, want canceled/%s", run, abandonedRunAutoCancelReason)
	}
	if len(asMap(run["completion_record_json"])) == 0 {
		t.Fatalf("completion_record_json = %#v, want frozen record", run["completion_record_json"])
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "canceled" {
		t.Fatalf("job state = %q, want canceled", got)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "run.canceled"); n != 1 {
		t.Fatalf("run.canceled count = %d, want 1", n)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "recovery.abandoned_run_auto_canceled"); n != 1 {
		t.Fatalf("recovery.abandoned_run_auto_canceled count = %d, want 1", n)
	}
}

func TestSweepDoesNotAutoCancelRunWithLiveWorktreeEvidence(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_abandoned_worktree"
	runID := "run_abandoned_worktree"
	jobID := "job_abandoned_worktree"
	old := time.Now().UTC().Add(-25 * time.Hour)

	seedAbandonedRunCandidate(t, ctx, runner, repoID, runID, jobID, old)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at, closed_at, close_reason
		) VALUES ($1,'sess_historical',$2,'author','codex','author-codex-1',1,
		          'closed',$3,$3,'historical')`, repoID, runID, old); err != nil {
		t.Fatalf("insert historical session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,'lease_historical',$2,'job',$3,'sess_historical',
		          'released',$4,$4,$4,'historical')`, repoID, runID, jobID, old); err != nil {
		t.Fatalf("insert historical lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ($1,'wt_abandoned',$2,$3,'lease_historical','main','.striatum/worktrees/wt_abandoned','active',$4)`,
		repoID, runID, jobID, old); err != nil {
		t.Fatalf("insert active worktree: %v", err)
	}

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	abandoned := asMap(result["abandoned_run"])
	if abandoned["reason"] != "live_worktree" {
		t.Fatalf("abandoned_run = %#v, want live_worktree block", abandoned)
	}
	run, err := oneRow(ctx, runner, `
		SELECT state, stop_reason
		  FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if run["state"] != "running" || run["stop_reason"] != nil {
		t.Fatalf("run row = %#v, want still running", run)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "run.canceled"); n != 0 {
		t.Fatalf("run.canceled count = %d, want 0", n)
	}
}

func seedAbandonedRunCandidate(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID string, old time.Time) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs
		   SET created_at = $3, started_at = $3
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID, old); err != nil {
		t.Fatalf("age run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  created_at
		)
		VALUES ($1,$2,$3,'author',1,'queued','author','{"lane_id":"codex"}'::jsonb,
		        'Author','draft','idem_'||$2,'[]'::jsonb,$4)`,
		repoID, jobID, runID, old); err != nil {
		t.Fatalf("insert abandoned job: %v", err)
	}
}

func seedRunEventAt(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, eventType string, at time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (
		  repository_id, run_id, event_type, payload_json, created_at
		) VALUES ($1,$2,$3,'{}'::jsonb,$4)`, repoID, runID, eventType, at); err != nil {
		t.Fatalf("insert event %s: %v", eventType, err)
	}
}

func eventCount(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, eventType string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT COUNT(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = $3`, repoID, runID, eventType)
	if err != nil {
		t.Fatalf("count event %s: %v", eventType, err)
	}
	return intValue(row["n"])
}
