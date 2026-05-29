package lanehealth

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

type mockProbe struct {
	result gosupervisor.LaneLiveness
}

func (m mockProbe) ProbeLane(ctx context.Context, meta gosupervisor.TmuxMeta, pid int, startToken string) gosupervisor.LaneLiveness {
	return m.result
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	repoID := "repo_test_lanehealth"
	sessionID := "sess_test_lanehealth"

	// 1. Set up DB fixtures
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,'ident_lh','/tmp/repo','/tmp/repo/.striatum','repo',$2,14,'active')`,
		repoID, now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_lh','wf','sha','{}'::jsonb,$2)`, repoID, now); err != nil {
		t.Fatalf("insert workflow snapshot: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,'run_lh','snap_lh','/tmp/repo','running',$2)`, repoID, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ($1,$2,'run_lh','implementer','codex','slug',1,'active',$3)`, repoID, sessionID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// 2. Insert process supervisor, pointer and daemon supervisor
	supID := "sup_lh_001"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
			repository_id, supervisor_id, session_id, run_id, pid, pid_start_time,
			stdin_pipe_path, state, started_at, heartbeat_at,
			adapter, command_json, cwd, scratch_path
		) VALUES ($1, $2, $3, 'run_lh', 4242, '', '/tmp/stdin', 'attached', $4, $4,
			'codex', '[]'::jsonb, '/tmp', '/tmp/scratch')`,
		repoID, supID, sessionID, now,
	); err != nil {
		t.Fatalf("insert process supervisor: %v", err)
	}

	dsupID := "dsup_lh_001"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
			repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id, pid, pid_start_time,
			state, metadata_json, updated_at
		) VALUES ($1, $2, $3, 'run_lh', $4, 4242, '', 'attached', '{}'::jsonb, $5)`,
		repoID, supID, dsupID, sessionID, now,
	); err != nil {
		t.Fatalf("insert pointer: %v", err)
	}

	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
			daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
			daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
			pid_start_time, state, started_at, heartbeat_at
		) VALUES ($1, $2, 'run_lh', $3, 'sup_lh_001', 'inst', 'codex', '[]'::jsonb, 'sha', '/tmp', 4242, '', 'attached', $4, $4)`,
		dsupID, repoID, sessionID, now,
	); err != nil {
		t.Fatalf("insert daemon supervisor: %v", err)
	}

	// 3. Run Check with mock Probe
	probe := mockProbe{
		result: gosupervisor.LaneLiveness{
			Alive: true,
			Class: "alive",
		},
	}
	checker := Checker{Probe: probe}

	health, err := checker.Check(ctx, pool.Runner, repoID, sessionID)
	if err != nil {
		t.Fatalf("checker.Check failed: %v", err)
	}

	if !health.Attested {
		t.Errorf("expected health.Attested to be true, got false")
	}
	if health.Reason != ReasonNone {
		t.Errorf("expected health.Reason to be ReasonNone, got %v", health.Reason)
	}
	if health.PID != 4242 {
		t.Errorf("expected health.PID to be 4242, got %v", health.PID)
	}
}

// TestAttestedDespiteNullToolsListWithMCPActivity guards #63 F4: a supervised
// agent-loop lane whose initial tools/list was issued before its session_id was
// bound (so last_tools_list_at is NULL) but which is actively driving the
// protocol over MCP (last_await_packet_at recently set) must remain attested
// even after the 60s discovery deadline. Previously the discovery stall fired,
// flipping health.Attested to false, which demoted the artifact byline to
// "author: operator" for every live agent-loop run (RFC 0026 / D149).
func TestAttestedDespiteNullToolsListWithMCPActivity(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	// registered well past the 60s discovery deadline, but recent MCP activity.
	registeredAt := now.Add(-5 * time.Minute)
	recentMCP := now.Add(-3 * time.Second)

	repoID := "repo_test_lh_nulltoolslist"
	sessionID := "sess_test_lh_nulltoolslist"

	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,'ident_lh_ntl','/tmp/repo','/tmp/repo/.striatum','repo',$2,14,'active')`,
		repoID, now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_lh_ntl','wf','sha','{}'::jsonb,$2)`, repoID, now); err != nil {
		t.Fatalf("insert workflow snapshot: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,'run_lh_ntl','snap_lh_ntl','/tmp/repo','running',$2)`, repoID, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	// last_tools_list_at deliberately left NULL; last_mcp_request_at and
	// last_await_packet_at recorded by an actual work.await_packet call.
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at, last_mcp_request_at, last_await_packet_at
		) VALUES ($1,$2,'run_lh_ntl','implementer','codex','slug',1,'active',$3,$4,$4)`,
		repoID, sessionID, registeredAt, recentMCP); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	supID := "sup_lh_ntl_001"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
			repository_id, supervisor_id, session_id, run_id, pid, pid_start_time,
			stdin_pipe_path, state, started_at, heartbeat_at,
			adapter, command_json, cwd, scratch_path
		) VALUES ($1, $2, $3, 'run_lh_ntl', 4242, '', '/tmp/stdin', 'attached', $4, $4,
			'codex', '[]'::jsonb, '/tmp', '/tmp/scratch')`,
		repoID, supID, sessionID, now,
	); err != nil {
		t.Fatalf("insert process supervisor: %v", err)
	}

	dsupID := "dsup_lh_ntl_001"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
			repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id, pid, pid_start_time,
			state, metadata_json, updated_at
		) VALUES ($1, $2, $3, 'run_lh_ntl', $4, 4242, '', 'attached', '{}'::jsonb, $5)`,
		repoID, supID, dsupID, sessionID, now,
	); err != nil {
		t.Fatalf("insert pointer: %v", err)
	}

	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
			daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
			daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
			pid_start_time, state, started_at, heartbeat_at
		) VALUES ($1, $2, 'run_lh_ntl', $3, 'sup_lh_ntl_001', 'inst', 'codex', '[]'::jsonb, 'sha', '/tmp', 4242, '', 'attached', $4, $4)`,
		dsupID, repoID, sessionID, now,
	); err != nil {
		t.Fatalf("insert daemon supervisor: %v", err)
	}

	probe := mockProbe{result: gosupervisor.LaneLiveness{Alive: true, Class: "alive"}}
	checker := Checker{Probe: probe}

	health, err := checker.Check(ctx, pool.Runner, repoID, sessionID)
	if err != nil {
		t.Fatalf("checker.Check failed: %v", err)
	}
	if health.Stall.StallClass != "" {
		t.Fatalf("expected no stall, got %q (since %v)", health.Stall.StallClass, health.Stall.StallSince)
	}
	if !health.Attested {
		t.Fatalf("expected health.Attested true (byline must not be demoted), got false; reason=%v", health.Reason)
	}
}

func TestLoadHelperProcessLiveness(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	repoID := "repo_test_helper_lh"
	sessionID := "sess_test_helper_lh"

	// 1. Set up DB fixtures
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,'ident_lh_helper','/tmp/repo','/tmp/repo/.striatum','repo',$2,14,'active')`,
		repoID, now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_lh_helper','wf','sha','{}'::jsonb,$2)`, repoID, now); err != nil {
		t.Fatalf("insert workflow snapshot: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,'run_lh_helper','snap_lh_helper','/tmp/repo','running',$2)`, repoID, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ($1,$2,'run_lh_helper','implementer','codex','slug',1,'active',$3)`, repoID, sessionID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// 2. Insert process supervisor, pointer (with helper process details) and daemon supervisor
	supID := "sup_lh_helper_001"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
			repository_id, supervisor_id, session_id, run_id, pid, pid_start_time,
			stdin_pipe_path, state, started_at, heartbeat_at,
			adapter, command_json, cwd, scratch_path
		) VALUES ($1, $2, $3, 'run_lh_helper', 4242, '', '/tmp/stdin', 'attached', $4, $4,
			'codex', '[]'::jsonb, '/tmp', '/tmp/scratch')`,
		repoID, supID, sessionID, now,
	); err != nil {
		t.Fatalf("insert process supervisor: %v", err)
	}

	dsupID := "dsup_lh_helper_001"
	// We seed a dead helper PID (e.g. 999999) inside metadata_json
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
			repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id, pid, pid_start_time,
			state, metadata_json, updated_at
		) VALUES ($1, $2, $3, 'run_lh_helper', $4, 4242, '', 'attached', '{"helper_pid": 999999, "helper_pid_start_time": "some-start-token"}'::jsonb, $5)`,
		repoID, supID, dsupID, sessionID, now,
	); err != nil {
		t.Fatalf("insert pointer: %v", err)
	}

	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
			daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
			daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
			pid_start_time, state, started_at, heartbeat_at
		) VALUES ($1, $2, 'run_lh_helper', $3, 'sup_lh_helper_001', 'inst', 'codex', '[]'::jsonb, 'sha', '/tmp', 4242, '', 'attached', $4, $4)`,
		dsupID, repoID, sessionID, now,
	); err != nil {
		t.Fatalf("insert daemon supervisor: %v", err)
	}

	// 3. Run Check with mock Probe
	probe := mockProbe{
		result: gosupervisor.LaneLiveness{
			Alive: true,
			Class: "alive",
		},
	}
	checker := Checker{Probe: probe}

	health, err := checker.Check(ctx, pool.Runner, repoID, sessionID)
	if err != nil {
		t.Fatalf("checker.Check failed: %v", err)
	}

	if health.Deliverable {
		t.Errorf("expected health.Deliverable to be false (degraded due to dead helper), got true")
	}
	if health.DeliveryReason != "helper_process_gone" {
		t.Errorf("expected health.DeliveryReason to be 'helper_process_gone', got %v", health.DeliveryReason)
	}
}
