package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedOpenBlocker inserts an open blocker row attached to a job.
func seedOpenBlocker(t *testing.T, ctx context.Context, runner interface {
	Exec(context.Context, string, ...any) error
}, repoID, runID, jobID, sessionID, blockerID, severity, kind string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'stale blocker','open',$8,'{}'::jsonb)`,
		repoID, blockerID, runID, jobID, sessionID, severity, kind, now); err != nil {
		t.Fatalf("insert blocker %s: %v", blockerID, err)
	}
}

func blockerState(t *testing.T, ctx context.Context, runner interface {
	// satisfied by db.Runner
	Exec(context.Context, string, ...any) error
}, repoID, blockerID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.blockers WHERE repository_id = $1 AND blocker_id = $2`, repoID, blockerID)
	if err != nil {
		t.Fatalf("read blocker %s: %v", blockerID, err)
	}
	return fmt.Sprint(row["state"])
}

// TestCompleteResolvesOpenAutonomousBlockers verifies #175: when a job
// completes, its open autonomous (non-human, non-escalation) blockers — e.g. a
// stale write_scope_guard_conflict — are resolved inside the work.complete
// transaction with a blocker.resolved_on_completion event per resolution, so a
// completed run does not present them as active open_blockers. A
// human_checkpoint blocker must NOT be auto-resolved.
func TestCompleteResolvesOpenAutonomousBlockers(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_complete_blockers"
	runID := "run_complete_blockers"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}, "reviewer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
	})

	builder := "sess_builder_blk"
	intgSeedSession(t, ctx, runner, repoID, runID, builder, "implementer", "claude", nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, builder, "claude")

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, idempotency_key, created_at, current_lease_id
		) VALUES ($1,'job_blk',$2,'build','Build','build','implementer','running','idem_blk',$3,'lease_blk')`,
		repoID, runID, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	// a downstream review job keeps the run from auto-completing on build finish.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, idempotency_key, created_at
		) VALUES ($1,'job_blk_review',$2,'review','Review','review','reviewer','blocked','idem_blk_review',$3)`,
		repoID, runID, now); err != nil {
		t.Fatalf("insert review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,'lease_blk',$2,'job','job_blk',$3,'active',$4,$5)`,
		repoID, runID, builder, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	// Seed an open autonomous blocker (write_scope_guard_conflict, severity
	// 'blocked') and an open human_checkpoint blocker on the same job.
	seedOpenBlocker(t, ctx, runner, repoID, runID, "job_blk", builder, "blk_autonomous", "blocked", "write_scope_guard_conflict")
	seedOpenBlocker(t, ctx, runner, repoID, runID, "job_blk", builder, "blk_human", "human_checkpoint", "revision_routing")

	if _, err := HandleCompleteWork(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": builder, "job_id": "job_blk", "lease_id": "lease_blk", "summary": "done",
	})); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// The autonomous blocker is resolved...
	if got := blockerState(t, ctx, runner, repoID, "blk_autonomous"); got != "resolved" {
		t.Fatalf("autonomous blocker should be resolved on completion, got %q", got)
	}
	// ...the human_checkpoint blocker stays open.
	if got := blockerState(t, ctx, runner, repoID, "blk_human"); got != "open" {
		t.Fatalf("human_checkpoint blocker must NOT be auto-resolved, got %q", got)
	}

	// A blocker.resolved_on_completion event was emitted for the autonomous one.
	evRow, err := oneRow(ctx, runner, `
		SELECT count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2
		   AND event_type = 'blocker.resolved_on_completion'
		   AND payload_json->>'blocker_id' = 'blk_autonomous'`, repoID, runID)
	if err != nil {
		t.Fatalf("count resolution events: %v", err)
	}
	if intValue(evRow["n"]) != 1 {
		t.Fatalf("want exactly one blocker.resolved_on_completion for blk_autonomous, got %v", evRow["n"])
	}
	// And NONE for the human_checkpoint blocker.
	humanEv, err := oneRow(ctx, runner, `
		SELECT count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2
		   AND event_type = 'blocker.resolved_on_completion'
		   AND payload_json->>'blocker_id' = 'blk_human'`, repoID, runID)
	if err != nil {
		t.Fatalf("count human resolution events: %v", err)
	}
	if intValue(humanEv["n"]) != 0 {
		t.Fatalf("human_checkpoint blocker must not emit a resolution event, got %v", humanEv["n"])
	}
}
