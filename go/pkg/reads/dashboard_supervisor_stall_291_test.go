package reads

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// ss291Seed seeds the #291 dashboard substrate: a running run with a single
// 'queued' job (pending work message, NO lease) and an ACTIVE supervised session
// bound to the job by lane, with an 'attached' process_supervisors row + pointer.
// idleAgo places the session's last heartbeat in the past so a test can put it
// inside or outside the supervisor stall window (defaultSupervisorStallAfterSeconds).
func ss291Seed(t *testing.T, ctx context.Context, runner db.Runner, repoID string, idleAgo time.Duration) (runID, sessionID string) {
	t.Helper()
	now := time.Now().UTC()
	idle := now.Add(-idleAgo)
	runID = "run_" + repoID
	sessionID = "sess_hung_" + repoID
	crSeedRepo(t, ctx, runner, repoID, now)
	crSeedRunState(t, ctx, runner, repoID, runID, "striatum/291", now, "running")

	// The job is QUEUED with a PENDING work message — never claimed, no lease.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, current_message_id, created_at, ready_at
		) VALUES ($1,'job_291',$2,'build',1,'queued','author','Build','draft','idem_291',
		         '{"mode":"repo_write","repo_write":true,"allowed_paths":["src"]}'::jsonb,'[]'::jsonb,
		         '{"lane_id":"claude"}'::jsonb,'msg_291',$3,$3)`,
		repoID, runID, now); err != nil {
		t.Fatalf("seed queued job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,'msg_291',$2,'job_291','work','pending',0,'author','claude',$3,$3)`,
		repoID, runID, now); err != nil {
		t.Fatalf("seed pending message: %v", err)
	}
	// An ACTIVE supervised session in the matching lane, idle since `idle`.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, operator_label, registered_at, last_heartbeat_at,
		  last_tools_list_at
		) VALUES ($1,$2,$3,'author','claude',$2||'-slug',1,'["claim","write"]'::jsonb,
		         'active','codex',$4,$5,$5)`,
		repoID, sessionID, runID, idle, idle); err != nil {
		t.Fatalf("seed active session: %v", err)
	}
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at, heartbeat_at
		) VALUES ($1,$2,$3,$4,'claude','[]'::jsonb,'/tmp','/tmp/s',4242,'attached',$5,$6)`,
		repoID, supID, runID, sessionID, now.Add(-time.Hour), idle); err != nil {
		t.Fatalf("seed process_supervisor: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, idle); err != nil {
		t.Fatalf("seed pointer: %v", err)
	}
	return runID, sessionID
}

// TestDashboardSupervisorStalls291SurfacesLeaselessHungSession proves the
// dashboard projection half of #291: an active-but-leaseless bound supervised
// session whose job is 'queued' and idle past the stall threshold surfaces as a
// stall (stalled_count >= 1, leaseless_count == 1) instead of the silent
// stalled_count:0 the run wedged on for ~80 minutes in the report.
func TestDashboardSupervisorStalls291SurfacesLeaselessHungSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_ss291_hung"

	runID, sessionID := ss291Seed(t, ctx, runner, repoID, 20*time.Minute)

	res, err := dashboardAllSupervisorStalls(ctx, runner, repoID, runID)
	if err != nil {
		t.Fatalf("dashboardAllSupervisorStalls: %v", err)
	}
	if got := res["stalled_count"]; got != 1 {
		t.Fatalf("stalled_count = %v, want 1; %#v", got, res)
	}
	if got := res["leaseless_count"]; got != 1 {
		t.Fatalf("leaseless_count = %v, want 1; %#v", got, res)
	}
	sups := res["supervisors"].([]map[string]any)
	if len(sups) != 1 {
		t.Fatalf("supervisors = %#v", sups)
	}
	if sups[0]["leaseless"] != true {
		t.Fatalf("surfaced supervisor not marked leaseless: %#v", sups[0])
	}
	if sups[0]["session_id"] != sessionID {
		t.Fatalf("surfaced session_id = %v, want %s", sups[0]["session_id"], sessionID)
	}
}

// TestDashboardSupervisorStalls291IgnoresWithinDeadline is the safety half: a
// freshly-spawned active supervised session that simply has not claimed yet
// (within the stall window) must NOT be surfaced, so the dashboard does not
// false-flag a healthy queued lane.
func TestDashboardSupervisorStalls291IgnoresWithinDeadline(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_ss291_healthy"

	runID, _ := ss291Seed(t, ctx, runner, repoID, 5*time.Second)

	res, err := dashboardAllSupervisorStalls(ctx, runner, repoID, runID)
	if err != nil {
		t.Fatalf("dashboardAllSupervisorStalls: %v", err)
	}
	if got := res["stalled_count"]; got != 0 {
		t.Fatalf("stalled_count = %v, want 0 (within-deadline lane must NOT be flagged); %#v", got, res)
	}
	if got := res["leaseless_count"]; got != 0 {
		t.Fatalf("leaseless_count = %v, want 0; %#v", got, res)
	}
}
