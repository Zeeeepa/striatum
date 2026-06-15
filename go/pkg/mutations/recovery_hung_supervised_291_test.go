package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// seedQueuedJobWithBoundSupervisedSession seeds the #291 wedge: a single-job run
// whose job is in 'queued' with a PENDING work message (never claimed, so NO
// lease at all), and an ACTIVE supervised session bound to the job by lane (the
// claim path's role+lane eligibility). This is the substrate both #291 telltales
// share — a hung supervised lane sitting on a claimable job with no lease — so a
// test can then make the session look dead-pane or honestly-stalled and assert
// the sweep recovers it (closes the hung owner so a fresh lane can claim).
//
// The session's activity timestamps are stamped `idleAgo` in the past so a test
// can place it either inside or outside the protocol-idle stall window.
func seedQueuedJobWithBoundSupervisedSession(t *testing.T, ctx context.Context, runner db.Runner, repoID string, idleAgo time.Duration) (runID, jobID, msgID, sessionID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_build_" + repoID
	msgID = "msg_build_" + repoID
	sessionID = "sess_hung_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "build", "type": "build", "role_id": "implementer"},
		},
	})
	// An ACTIVE supervised session in the codex lane — the lane the job is pinned
	// to — but it never claimed, so there is no lease.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "codex", []string{"write"}, "active")

	now := time.Now().UTC()
	idle := now.Add(-idleAgo)
	// Stamp the session's activity in the past: it discovered MCP (tools/list) and
	// produced a packet-delivery/await record, but recorded nothing since `idle`.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = $3, last_mcp_request_at = $3, last_tools_list_at = $3,
		       last_await_packet_at = $3, last_session_heartbeat_at = $3,
		       last_pty_activity_at = $3, last_heartbeat_at = $3
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, idle); err != nil {
		t.Fatalf("stamp idle session activity: %v", err)
	}

	// The job is QUEUED with a PENDING work message — never claimed, no lease.
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src/"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_lease_id, created_at, ready_at
		) VALUES ($1,$2,$3,'build',1,'queued','implementer','Build','build',
		          'idem_build_'||$1,'[]'::jsonb,$4::jsonb,'{"lane_id":"codex"}'::jsonb,NULL,$5,$5)`,
		repoID, jobID, runID, wsArg, now); err != nil {
		t.Fatalf("insert queued build job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,'implementer','codex',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert pending work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_message_id = $3
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, msgID); err != nil {
		t.Fatalf("link job message pointer: %v", err)
	}
	return runID, jobID, msgID, sessionID
}

// seedSupervisorPointer records a process_supervisor_pointer for a session so the
// confirmedDead probe (supervisedAgentConfirmedDead) has a recorded agent to
// judge. state is the pointer state ('attached' for a live-looking lane).
func seedSupervisorPointer(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, state string, pid int) {
	t.Helper()
	now := time.Now().UTC()
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'',$7,$8,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, pid, state, now); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}
}

// TestSweep291RecoversLeaselessHungSupervisedSession proves #291 case (b): a
// claimable ('queued') job whose bound supervised session is ACTIVE but holds no
// lease (it never claimed its packet) and has been idle past the protocol-idle
// deadline is recovered by the sweep — the hung owner session is CLOSED so a
// fresh lane can claim the already-pending job. Before the fix the decision tree
// never scanned 'queued' jobs and could not resolve a leaseless session, so the
// run sat wedged forever with supervisor_stalls.stalled_count:0.
func TestSweep291RecoversLeaselessHungSupervisedSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_291_leaseless"

	// Idle 20 minutes — well past the protocol-idle deadline (DefaultPolicy
	// ProtocolIdleSeconds) so Classify reports ProtocolStalled.
	runID, jobID, msgID, sessionID := seedQueuedJobWithBoundSupervisedSession(t, ctx, runner, repoID, 20*time.Minute)
	// An attached pointer whose agent probes ALIVE (the codex "tmux_ok, no_lease"
	// telltale): the lane is alive but stuck before claim. CASE 2 (honest stall)
	// must transfer it — not the confirmed-dead path.
	seedSupervisorPointer(t, ctx, runner, repoID, runID, sessionID, "attached", 4242)

	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: true, Class: "pid_ok"}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1 (the hung leaseless lane must be recovered); %#v", summary["acted_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 {
		t.Fatalf("expected exactly one action; got %#v", summary["actions"])
	}
	if acts[0]["acted"] != true {
		t.Fatalf("action acted=false, want true; %#v", acts[0])
	}
	if acts[0]["stalled_owner_closed"] != true {
		t.Fatalf("hung owner session was NOT closed; %#v (closing it is the #291 recovery)", acts[0])
	}

	// The hung session is now closed so it can never wake to double-claim.
	if got := sessionStateOf(t, ctx, runner, repoID, sessionID); got != "closed" {
		t.Fatalf("session state = %q, want closed", got)
	}
	// The job remains claimable: still queued with a pending work message and no lease.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (still claimable)", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
	if got := messageState(t, ctx, runner, repoID, msgID); got != "pending" {
		t.Fatalf("message state = %q, want pending", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}

	// Convergent: a second sweep is a no-op (the hung session is gone; the job is
	// just queued+pending awaiting a fresh lane).
	result2, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if got := intValue(recoveryActionsFromSweep(t, result2)["acted_count"]); got != 0 {
		t.Fatalf("second sweep acted_count = %d, want 0 (convergent)", got)
	}
}

// TestSweep291RecoversDeadPaneQueuedJob proves #291 case (a): a claimable
// ('queued') job whose bound supervised session is still marked ACTIVE but whose
// supervised agent process/pane is positively DEAD. The confirmed-dead probe
// (the unforgeable #147 Symptom B signal) fires from the default branch and the
// hung owner session is closed + the job left claimable on the same attempt.
func TestSweep291RecoversDeadPaneQueuedJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_291_dead_pane"

	// Fresh activity (1s ago) so neither CASE 1 (dead) nor CASE 2 (honest stall)
	// fires on the session signal — the masked-dead probe is the SOLE signal, just
	// like the #147 oracle test. The job is still 'queued' so this only recovers
	// once the #291 scan + bound-session resolution lets it through.
	runID, jobID, _, sessionID := seedQueuedJobWithBoundSupervisedSession(t, ctx, runner, repoID, 1*time.Second)
	seedSupervisorPointer(t, ctx, runner, repoID, runID, sessionID, "attached", 4242)

	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		// The pane/PID is positively dead (the claude_code tmux_pane_dead telltale).
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: false, Class: "pid_gone", ObservedPID: 4242}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1 (dead-pane queued job must be recovered); %#v", summary["acted_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["acted"] != true || acts[0]["stalled_owner_closed"] != true {
		t.Fatalf("expected one acted recovery that closed the hung owner; got %#v", summary["actions"])
	}
	if got := sessionStateOf(t, ctx, runner, repoID, sessionID); got != "closed" {
		t.Fatalf("session state = %q, want closed", got)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (still claimable)", got)
	}
}

// TestSweep291DoesNotFlagHealthyQueuedJobs is the load-bearing safety test: the
// newly-scanned 'queued' state must NOT cause recovery to act on normal queued
// jobs. Two healthy populations are checked:
//   - a freshly-queued job whose bound supervised session is active but WITHIN the
//     idle deadline (it just hasn't claimed yet), and
//   - a freshly-queued job with NO bound supervised session at all (waiting for a
//     lane to spawn).
//
// Neither may be touched by the sweep, or the fix would requeue/close healthy
// lanes on every sweep.
func TestSweep291DoesNotFlagHealthyQueuedJobs(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	t.Run("active_session_within_deadline", func(t *testing.T) {
		repoID := "repo_291_healthy_active"
		// Idle only 5 seconds — far inside the protocol-idle window, so a freshly
		// spawned lane that simply hasn't claimed yet is NOT a stall.
		runID, jobID, _, sessionID := seedQueuedJobWithBoundSupervisedSession(t, ctx, runner, repoID, 5*time.Second)
		seedSupervisorPointer(t, ctx, runner, repoID, runID, sessionID, "attached", 4242)

		restore := probeLaneLiveness
		probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
			return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: true, Class: "pid_ok"}
		}
		t.Cleanup(func() { probeLaneLiveness = restore })

		result, err := SweepRun(ctx, runner, repoID, runID, "")
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if got := intValue(recoveryActionsFromSweep(t, result)["acted_count"]); got != 0 {
			t.Fatalf("acted_count = %d, want 0 (a within-deadline queued lane must NOT be flagged)", got)
		}
		if got := sessionStateOf(t, ctx, runner, repoID, sessionID); got != "active" {
			t.Fatalf("session state = %q, want active (untouched)", got)
		}
		if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
			t.Fatalf("job state = %q, want queued (untouched)", got)
		}
	})

	t.Run("no_bound_session", func(t *testing.T) {
		repoID := "repo_291_healthy_nosession"
		// Same queued job, but DELETE the supervised session so the job is a plain
		// freshly-queued job with no bound lane yet. Idle window is irrelevant.
		runID, jobID, _, sessionID := seedQueuedJobWithBoundSupervisedSession(t, ctx, runner, repoID, 20*time.Minute)
		if err := runner.Exec(ctx, `DELETE FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID); err != nil {
			t.Fatalf("delete session: %v", err)
		}

		result, err := SweepRun(ctx, runner, repoID, runID, "")
		if err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if got := intValue(recoveryActionsFromSweep(t, result)["acted_count"]); got != 0 {
			t.Fatalf("acted_count = %d, want 0 (a queued job with no bound session must NOT be flagged)", got)
		}
		if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
			t.Fatalf("job state = %q, want queued (untouched)", got)
		}
	})
}
