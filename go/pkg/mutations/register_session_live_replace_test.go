package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// seedReplaceFixture seeds a job, active lease, and claimed queue message owned
// by sessID1 on the (run, reviewer, codex) slot, mirroring
// TestRegisterSessionReplaceSupersedesPrior but returning the ids so a test can
// drive --replace under different liveness.
func seedReplaceFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessID1 string) (jobID, leaseID, msgID string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, idempotency_key, created_at
		)
		VALUES ($1, $2, $3, 'reviewer_job', 1, 'running', 'reviewer', 'Review Job', 'review', $4, NOW())`,
		repoID, "job_live_"+runID, runID, "idem_"+runID); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	jobID = "job_live_" + runID
	leaseID = "lease_live_" + runID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id, state, acquired_at, expires_at)
		VALUES ($1, $2, $3, 'job', $4, $5, 'active', NOW(), NOW() + INTERVAL '1 hour')`,
		repoID, leaseID, runID, jobID, sessID1); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	msgID = "msg_live_" + runID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (repository_id, message_id, run_id, job_id, kind, state, priority, target_session_id, target_role_id, target_lane_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'work', 'claimed', 0, $5, 'reviewer', 'codex', NOW(), NOW())`,
		repoID, msgID, runID, jobID, sessID1); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}
	return jobID, leaseID, msgID
}

// TestRegisterSessionReplaceRefusesLiveHeartbeatingSession verifies #189/#174:
// register-session --replace MUST refuse when the displaced session has
// heartbeated within the lease's advertised heartbeat window, naming the
// displaced session id, its last-heartbeat age, and operator label, rather than
// silently yanking a live, actively-working lane's leases.
func TestRegisterSessionReplaceRefusesLiveHeartbeatingSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_replace_live_refuse"
	runID := "run_replace_live_refuse"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})

	res1, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "operator_label": "alpha-driver",
	}))
	if err != nil {
		t.Fatalf("register first session: %v", err)
	}
	sessID1 := fmt.Sprint(res1["session_id"])
	seedReplaceFixture(t, ctx, runner, repoID, runID, sessID1)
	// The first session registered just now: its last_heartbeat_at is recent
	// (set to registration time), so it is within the heartbeat window.

	_, err = HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "replace": true,
	}))
	if err == nil {
		t.Fatalf("expected --replace to refuse displacing a live-heartbeating session")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != "displaced_session_live" {
		t.Fatalf("error code = %q, want displaced_session_live", rpcErr.Code)
	}
	if !strings.Contains(rpcErr.Message, sessID1) {
		t.Fatalf("error message %q should name displaced session %s", rpcErr.Message, sessID1)
	}
	if !strings.Contains(rpcErr.Message, "alpha-driver") {
		t.Fatalf("error message %q should name operator label", rpcErr.Message)
	}
	if !strings.Contains(rpcErr.Message, "--force-live") {
		t.Fatalf("error message %q should name the --force-live escape hatch", rpcErr.Message)
	}

	// The first session must remain active and keep its lease.
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row["state"]) != "active" {
		t.Fatalf("displaced session should still be active, got %v err %v", row["state"], err)
	}
}

// TestRegisterSessionReplaceForceLiveRecordsReason verifies #189: --force-live
// --reason "..." permits the displacement of a live-heartbeating session and
// records the justification on the session.closed event for audit.
func TestRegisterSessionReplaceForceLiveRecordsReason(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_replace_force_live"
	runID := "run_replace_force_live"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})

	res1, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex",
	}))
	if err != nil {
		t.Fatalf("register first session: %v", err)
	}
	sessID1 := fmt.Sprint(res1["session_id"])
	_, leaseID, _ := seedReplaceFixture(t, ctx, runner, repoID, runID, sessID1)

	// --force-live without a reason must still be refused.
	if _, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "replace": true, "force_live": true,
	})); err == nil {
		t.Fatalf("--force-live without --reason should be refused")
	}

	res2, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex",
		"replace": true, "force_live": true, "reason": "lane wedged but still heartbeating",
	}))
	if err != nil {
		t.Fatalf("register with --force-live --reason: %v", err)
	}
	if fmt.Sprint(res2["session_id"]) == sessID1 {
		t.Fatalf("expected a new session id")
	}

	// First session superseded, lease released.
	row1, err := oneRow(ctx, runner, `SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row1["state"]) != "closed" || fmt.Sprint(row1["close_reason"]) != "superseded" {
		t.Fatalf("displaced session should be closed+superseded, got %v/%v err %v", row1["state"], row1["close_reason"], err)
	}
	leaseRow, err := oneRow(ctx, runner, `SELECT state FROM striatumd.leases WHERE repository_id = $1 AND lease_id = $2`, repoID, leaseID)
	if err != nil || fmt.Sprint(leaseRow["state"]) != "released" {
		t.Fatalf("lease should be released, got %v err %v", leaseRow["state"], err)
	}

	// The forced-live reason is recorded on the session.closed event payload.
	evRow, err := oneRow(ctx, runner, `
		SELECT payload_json->>'force_live_reason' AS reason
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'session.closed'
		   AND payload_json->>'session_id' = $3
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID, sessID1)
	if err != nil {
		t.Fatalf("read session.closed event: %v", err)
	}
	if got := fmt.Sprint(evRow["reason"]); got != "lane wedged but still heartbeating" {
		t.Fatalf("force_live_reason on event = %q, want the recorded justification", got)
	}
}

// TestRegisterSessionReplaceStillReplacesStaleSession verifies #189: a session
// whose last heartbeat is OLDER than the heartbeat window is still replaceable
// without --force-live (the genuinely-stranded case the operator wants).
func TestRegisterSessionReplaceStillReplacesStaleSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_replace_stale_ok"
	runID := "run_replace_stale_ok"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})

	res1, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex",
	}))
	if err != nil {
		t.Fatalf("register first session: %v", err)
	}
	sessID1 := fmt.Sprint(res1["session_id"])
	_, leaseID, _ := seedReplaceFixture(t, ctx, runner, repoID, runID, sessID1)

	// Backdate both the session and its lease heartbeats well beyond the window
	// so the session looks genuinely stranded.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions SET last_heartbeat_at = NOW() - INTERVAL '2 hours', registered_at = NOW() - INTERVAL '2 hours'
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases SET last_heartbeat_at = NOW() - INTERVAL '2 hours'
		 WHERE repository_id = $1 AND lease_id = $2`, repoID, leaseID); err != nil {
		t.Fatalf("backdate lease: %v", err)
	}

	res2, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "replace": true,
	}))
	if err != nil {
		t.Fatalf("stale session should replace without --force-live: %v", err)
	}
	if fmt.Sprint(res2["session_id"]) == sessID1 {
		t.Fatalf("expected a new session id")
	}

	row1, err := oneRow(ctx, runner, `SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row1["state"]) != "closed" || fmt.Sprint(row1["close_reason"]) != "superseded" {
		t.Fatalf("stale session should be closed+superseded, got %v/%v err %v", row1["state"], row1["close_reason"], err)
	}
}
