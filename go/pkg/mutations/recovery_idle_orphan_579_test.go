package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedSucceededJobForSession seeds a terminal ('completed') job in the run, owned
// (via a released lease) by sessionID. This is the #579 orphan's PRIOR slice: the
// builder finished it, so the session owns no UNFINISHED job — recoverStuckJobs'
// job-driven scan (which only scans queued/claimed/running/stale_lease) never
// resolves the session, leaving the idle orphan unreachable by the job-driven path.
func seedSucceededJobForSession(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, sessionID string) {
	t.Helper()
	now := time.Now().UTC()
	leaseID := "lease_done_" + jobID
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src/"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at, started_at, completed_at
		) VALUES ($1,$2,$3,$4,1,'completed','implementer','Prior slice','build',
		          'idem_'||$2,'[]'::jsonb,$5::jsonb,'{"lane_id":"claude"}'::jsonb,$6,$6,$6)`,
		repoID, jobID, runID, "wfj_"+jobID, wsArg, now); err != nil {
		t.Fatalf("insert completed prior-slice job: %v", err)
	}
	// A RELEASED lease the orphan once held — terminal, so it is NOT an active lease
	// and the orphan owns no live job. recoverStuckJobs resolves the latest lease on
	// a job, but only for jobs in non-terminal states, so this succeeded job (and its
	// owning session) is never scanned.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '2 hours',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '1 hour','work_complete')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
}

// seedQueuedDownstreamJob seeds a 'queued' downstream slice with a PENDING work
// message and NO lease, pinned to `lane`. With the orphan in a DIFFERENT lane (and
// holding no supervisor pointer), the #291 bound-session join cannot bind the
// orphan to this job, so the job-driven recoverStuckJobs path leaves both untouched.
func seedQueuedDownstreamJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, lane string) {
	t.Helper()
	now := time.Now().UTC()
	msgID := "msg_" + jobID
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src2/"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, created_at, ready_at
		) VALUES ($1,$2,$3,$4,1,'queued','implementer','Downstream slice','build',
		          'idem_'||$2,'[]'::jsonb,$5::jsonb,
		          ('{"lane_id":"'||$6||'"}')::jsonb,$7,$8,$8)`,
		repoID, jobID, runID, "wfj_"+jobID, wsArg, lane, msgID, now); err != nil {
		t.Fatalf("insert queued downstream job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,'implementer',$5,$6,$6)`,
		repoID, msgID, runID, jobID, lane, now); err != nil {
		t.Fatalf("insert pending work message: %v", err)
	}
}

// stampIdleStalled stamps a session's activity columns idleAgo in the past with
// NO fresh PTY/tool-call/heartbeat signal, so sessionliveness.Classify returns
// StallProtocolIdle (agent_protocol_idle_stall) once idleAgo exceeds the
// ProtocolIdleSeconds deadline. Mirrors the #579 evidence: still-discovered
// (an old last_mcp_request_at / heartbeat) but quiet for >ProtocolIdleSeconds.
func stampIdleStalled(t *testing.T, ctx context.Context, runner db.Runner, repoID, sessionID string, idleAgo time.Duration) {
	t.Helper()
	idle := time.Now().UTC().Add(-idleAgo)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = $3,
		       last_mcp_request_at = $3,
		       last_session_heartbeat_at = $3,
		       last_work_heartbeat_at = $3,
		       last_pty_activity_at = NULL,
		       last_tool_call_started_at = NULL,
		       last_tool_call_finished_at = NULL
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, idle); err != nil {
		t.Fatalf("stamp idle-stalled activity for %s: %v", sessionID, err)
	}
}

// TestSweepReapsIdleOrphanSessionWithNoJob is the #579 regression: a builder lane
// that COMPLETED its slice and then went agent_protocol_idle_stall with job=None
// (no active lease, not bound to any unfinished job) must be reaped by the sweep
// so a fresh lane can claim the queued downstream slice. The job-driven
// recoverStuckJobs cannot reach this orphan; the session-driven
// reapIdleOrphanSessions pass must close it. A healthy idle session that is still
// inside the protocol-idle deadline is the negative control: it must NOT be reaped.
func TestSweepReapsIdleOrphanSessionWithNoJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_579_idle_orphan"
	runID := "run_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}, "codex": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "prior", "type": "build", "role_id": "implementer"},
			map[string]any{"id": "downstream", "type": "build", "role_id": "implementer"},
		},
	})

	// The #579 orphan: an ACTIVE implementer session in the claude lane that
	// finished its prior slice (owns a released lease on a succeeded job), holds no
	// active lease, has no supervisor pointer, and has been idle 30 minutes — well
	// past ProtocolIdleSeconds (300s) — so Classify returns StallProtocolIdle.
	orphan := "sess_orphan_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, orphan, "implementer", "claude", []string{"write"}, "active", 1)
	stampIdleStalled(t, ctx, runner, repoID, orphan, 30*time.Minute)
	seedSucceededJobForSession(t, ctx, runner, repoID, runID, "job_prior_"+repoID, orphan)

	// The queued downstream slice, pinned to the codex lane (a DIFFERENT lane than
	// the orphan), with a pending work message and no lease. The lane mismatch (and
	// the orphan's absent supervisor pointer) means the #291 bound-session join
	// never binds the orphan to it, so recoverStuckJobs leaves it untouched.
	downstream := "job_downstream_" + repoID
	seedQueuedDownstreamJob(t, ctx, runner, repoID, runID, downstream, "codex")

	// Negative control: a healthy ACTIVE implementer session, idle only 5 seconds —
	// far inside the protocol-idle window, so it classifies quiet/working, NOT
	// idle-stalled, and must survive the reap (a session momentarily between claims).
	healthy := "sess_healthy_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, healthy, "implementer", "claude", []string{"write"}, "active", 2)
	stampIdleStalled(t, ctx, runner, repoID, healthy, 5*time.Second)

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	summary := recoveryActionsFromSweep(t, result)
	acts, _ := summary["actions"].([]map[string]any)
	var reap map[string]any
	for _, a := range acts {
		if a["action"] == "reap_idle_orphan_session" {
			reap = a
		}
	}
	if reap == nil {
		t.Fatalf("expected a reap_idle_orphan_session action; got %#v", summary["actions"])
	}
	if reap["session_id"] != orphan {
		t.Fatalf("reap targeted session %v, want the orphan %s", reap["session_id"], orphan)
	}
	if reap["acted"] != true || reap["stalled_owner_closed"] != true {
		t.Fatalf("reap action not acted/closed: %#v", reap)
	}
	if reap["stall_class"] != "agent_protocol_idle_stall" {
		t.Fatalf("reap stall_class = %v, want agent_protocol_idle_stall", reap["stall_class"])
	}

	// The orphan is now closed with the recovery reason so it cannot keep its lane
	// "occupied" in run-reconcile — a fresh lane can spawn for the queued downstream.
	if got := sessionStateOf(t, ctx, runner, repoID, orphan); got != "closed" {
		t.Fatalf("orphan session state = %q, want closed", got)
	}
	if got := sessionCloseReasonOf(t, ctx, runner, repoID, orphan); got != "recovery_stalled_transfer" {
		t.Fatalf("orphan close_reason = %q, want recovery_stalled_transfer", got)
	}

	// NEGATIVE CONTROL: the healthy within-deadline session is untouched.
	if got := sessionStateOf(t, ctx, runner, repoID, healthy); got != "active" {
		t.Fatalf("healthy session state = %q, want active (within-deadline lane must NOT be reaped)", got)
	}

	// The queued downstream slice stays claimable: still queued with a pending work
	// message and no lease (the reap frees the lane, it does not touch the job).
	if got := jobState(t, ctx, runner, repoID, downstream); got != "queued" {
		t.Fatalf("downstream job state = %q, want queued (still claimable)", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, downstream); n != 1 {
		t.Fatalf("downstream pending work message count = %d, want 1", n)
	}

	// Convergent: a second sweep is a no-op for the orphan (already closed).
	result2, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	acts2, _ := recoveryActionsFromSweep(t, result2)["actions"].([]map[string]any)
	for _, a := range acts2 {
		if a["action"] == "reap_idle_orphan_session" {
			t.Fatalf("second sweep re-reaped the orphan (not convergent): %#v", a)
		}
	}
}
