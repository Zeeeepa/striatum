package mutations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	readspkg "github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

type supervisionControlPGFixture struct {
	repoID    string
	runID     string
	sessionID string
	jobID     string
	leaseID   string
	repoRoot  string
}

func TestSupervisionControlPGStartProgressStatusAndStopPersistRowsAndEvents(t *testing.T) {
	pool := pgtest.Pool(t)
	runner := pool.Runner
	ctx := context.Background()

	fx := seedSupervisionControlPGFixture(t, ctx, runner)

	pid := os.Getpid()
	pidStartTime, ok := processStartToken(pid)
	if !ok || pidStartTime == "" {
		t.Skipf("current process start token unavailable for pid %d", pid)
	}

	origMkfifo := supervisionMkfifo
	origLaunch := supervisionLaunch
	t.Cleanup(func() {
		supervisionMkfifo = origMkfifo
		supervisionLaunch = origLaunch
	})
	supervisionMkfifo = func(path string) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return os.WriteFile(path, []byte{}, 0o600)
	}
	supervisionLaunch = func(_ context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath, eventPath string) (supervisionLaunchResult, error) {
		if config.CapabilityToken == "" {
			t.Fatal("supervise.start did not mint a session-bound token")
		}
		if config.RepositoryID != fx.repoID || config.SessionID != fx.sessionID || config.RunID != fx.runID {
			t.Fatalf("launch config ids = (%q,%q,%q), want (%q,%q,%q)",
				config.RepositoryID, config.SessionID, config.RunID, fx.repoID, fx.sessionID, fx.runID)
		}
		if config.AgentLoopMode != agentLoopModeSelfDriving {
			t.Fatalf("agent loop mode = %q, want %q", config.AgentLoopMode, agentLoopModeSelfDriving)
		}
		if scratch == "" || pipePath == "" || eventPath == "" || supervisorID == "" {
			t.Fatalf("launch paths were not populated: supervisor=%q scratch=%q pipe=%q event=%q",
				supervisorID, scratch, pipePath, eventPath)
		}
		return supervisionLaunchResult{
			PID:          pid,
			PIDStartTime: pidStartTime,
			Metadata: map[string]any{
				"source": "supervision_control_pg_test",
			},
		}, nil
	}

	start, err := HandleSuperviseStart(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervision_control_pg_start",
		Method:        "supervise.start",
		Params: map[string]any{
			"repository_id":      fx.repoID,
			"session_id":         fx.sessionID,
			"provider_auth_gate": "off",
		},
	})
	if err != nil {
		t.Fatalf("supervise.start: %v", err)
	}
	supervisorID := fmt.Sprint(start["supervisor_id"])
	daemonSupervisorID := fmt.Sprint(start["daemon_supervisor_id"])
	if start["state"] != "attached" || supervisorID == "" || daemonSupervisorID == "" {
		t.Fatalf("start result = %#v, want attached supervisor ids", start)
	}

	assertSupervisorAttachedRows(t, ctx, runner, fx, supervisorID, daemonSupervisorID, pid, pidStartTime)
	assertEventCount(t, ctx, runner, fx, "supervisor.starting", 1)
	assertEventCount(t, ctx, runner, fx, "supervisor.started", 1)

	report, err := HandleSuperviseReport(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervision_control_pg_progress",
		Method:        "supervise.report",
		Params: map[string]any{
			"repository_id": fx.repoID,
			"session_id":    fx.sessionID,
			"supervisor_id": supervisorID,
			"event_type":    "progress",
			"payload": map[string]any{
				"meaningful":  true,
				"bytes":       float64(4096),
				"total_bytes": float64(4096),
			},
		},
	})
	if err != nil {
		t.Fatalf("supervise.report progress: %v", err)
	}
	if report["state"] != "attached" || fmt.Sprint(report["supervisor_id"]) != supervisorID {
		t.Fatalf("progress report = %#v, want attached supervisor %s", report, supervisorID)
	}
	assertProgressDurability(t, ctx, runner, fx)

	status, err := readspkg.HandleSuperviseStatus(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervision_control_pg_status",
		Method:        "supervise.status",
		Params: map[string]any{
			"repository_id": fx.repoID,
			"session_id":    fx.sessionID,
		},
	})
	if err != nil {
		t.Fatalf("supervise.status: %v", err)
	}
	if fmt.Sprint(status["supervisor_id"]) != supervisorID {
		t.Fatalf("status supervisor_id = %v, want %s", status["supervisor_id"], supervisorID)
	}
	if status["supervisor_state"] != "attached" || status["agent_pid_alive"] != true {
		t.Fatalf("status = %#v, want attached live process", status)
	}
	if status["active_lease_id"] != fx.leaseID || status["active_lease_last_heartbeat_at"] == nil {
		t.Fatalf("status active lease = %#v, want lease %s with heartbeat", status, fx.leaseID)
	}

	poisonSupervisorStartTokenForStop(t, ctx, runner, fx.repoID, supervisorID, daemonSupervisorID)
	stop, err := HandleSuperviseStop(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervision_control_pg_stop",
		Method:        "supervise.stop",
		Params: map[string]any{
			"repository_id": fx.repoID,
			"session_id":    fx.sessionID,
			"reason":        "operator_requested",
		},
	})
	if err != nil {
		t.Fatalf("supervise.stop: %v", err)
	}
	if stop["state"] != "stopped" || fmt.Sprint(stop["supervisor_id"]) != supervisorID {
		t.Fatalf("stop result = %#v, want stopped supervisor %s", stop, supervisorID)
	}
	assertSupervisorStoppedRows(t, ctx, runner, fx.repoID, supervisorID, daemonSupervisorID)
	assertEventCount(t, ctx, runner, fx, "supervisor.stopped", 1)
	assertStoppedEventPayload(t, ctx, runner, fx, supervisorID)
}

func seedSupervisionControlPGFixture(t *testing.T, ctx context.Context, runner db.Runner) supervisionControlPGFixture {
	t.Helper()
	now := time.Now().UTC().Add(-time.Minute)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".striatum"), 0o700); err != nil {
		t.Fatalf("mkdir repo scratch: %v", err)
	}

	fx := supervisionControlPGFixture{
		repoID:    "repo_supervision_control_pg",
		runID:     "run_supervision_control_pg",
		sessionID: "sess_supervision_control_pg",
		jobID:     "job_supervision_control_pg",
		leaseID:   "lease_supervision_control_pg",
		repoRoot:  repoRoot,
	}

	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id": "wf_supervision_control_pg",
		"lanes": map[string]any{
			"codex": map[string]any{
				"adapter":              "process",
				"command":              []string{"codex"},
				"adapter_capabilities": map[string]any{"agent_loop": true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "repo_write",
		"repo_write":      true,
		"allowed_paths":   []string{"docs/"},
		"forbidden_paths": []string{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedArg, err := db.JSONBArg(runner, []any{})
	if err != nil {
		t.Fatal(err)
	}
	capReqArg, err := db.JSONBArg(runner, map[string]any{"process_execution": true})
	if err != nil {
		t.Fatal(err)
	}

	mustExec(t, ctx, runner, "insert repository", `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo-supervision-control-pg',$5,23,'active')`,
		fx.repoID, "ident_supervision_control_pg", repoRoot, filepath.Join(repoRoot, ".striatum"), now)
	mustExec(t, ctx, runner, "insert workflow snapshot", `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_supervision_control_pg','wf_supervision_control_pg','sha_supervision_control_pg',$2::jsonb,$3)`,
		fx.repoID, workflowArg, now)
	mustExec(t, ctx, runner, "insert run", `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, branch_name,
		  created_at, started_at
		) VALUES ($1,$2,'snap_supervision_control_pg',$3,'running','main',$4,$4)`,
		fx.repoID, fx.runID, repoRoot, now)
	mustExec(t, ctx, runner, "insert session", `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ($1,$2,$3,'author','codex','author-codex-supervision-pg',1,'[]'::jsonb,'active',$4)`,
		fx.repoID, fx.sessionID, fx.runID, now)
	mustExec(t, ctx, runner, "insert job", `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key,
		  expected_artifacts_json, capability_requirements_json, write_scope_json,
		  current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'author_draft',1,'running','author',
		  $4::jsonb,'Author draft','build','idem_supervision_control_pg',
		  $5::jsonb,$6::jsonb,$7::jsonb,$8,$9,$9)`,
		fx.repoID, fx.jobID, fx.runID, laneSelectorArg, expectedArg, capReqArg, writeScopeArg, fx.leaseID, now)
	mustExec(t, ctx, runner, "insert lease", `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		fx.repoID, fx.leaseID, fx.runID, fx.jobID, fx.sessionID, now, now.Add(time.Hour))
	return fx
}

func assertSupervisorAttachedRows(t *testing.T, ctx context.Context, runner db.Runner, fx supervisionControlPGFixture, supervisorID, daemonSupervisorID string, pid int, pidStartTime string) {
	t.Helper()
	process := mustOneRow(t, ctx, runner, `
		SELECT state, pid::text AS pid, COALESCE(pid_start_time, '') AS pid_start_time,
		       heartbeat_at IS NOT NULL AS has_heartbeat
		  FROM striatumd.process_supervisors
		 WHERE repository_id = $1 AND supervisor_id = $2`,
		fx.repoID, supervisorID)
	if process["state"] != "attached" || process["pid"] != fmt.Sprint(pid) || process["pid_start_time"] != pidStartTime || process["has_heartbeat"] != true {
		t.Fatalf("process supervisor row = %#v, want attached pid %d start %s with heartbeat", process, pid, pidStartTime)
	}

	pointer := mustOneRow(t, ctx, runner, `
		SELECT state, daemon_supervisor_id, pid::text AS pid,
		       COALESCE(pid_start_time, '') AS pid_start_time,
		       metadata_json->>'source' AS source,
		       metadata_json->>'agent_loop_mode' AS agent_loop_mode
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND supervisor_id = $2`,
		fx.repoID, supervisorID)
	if pointer["state"] != "attached" || pointer["daemon_supervisor_id"] != daemonSupervisorID ||
		pointer["pid"] != fmt.Sprint(pid) || pointer["pid_start_time"] != pidStartTime ||
		pointer["source"] != "supervision_control_pg_test" || pointer["agent_loop_mode"] != agentLoopModeSelfDriving {
		t.Fatalf("supervisor pointer row = %#v, want attached daemon id %s and launch metadata", pointer, daemonSupervisorID)
	}

	daemon := mustOneRow(t, ctx, runner, `
		SELECT state, repo_supervisor_id, pid::text AS pid, COALESCE(pid_start_time, '') AS pid_start_time,
		       heartbeat_at IS NOT NULL AS has_heartbeat
		  FROM striatumd.daemon_supervisors
		 WHERE daemon_supervisor_id = $1`,
		daemonSupervisorID)
	if daemon["state"] != "attached" || daemon["repo_supervisor_id"] != supervisorID ||
		daemon["pid"] != fmt.Sprint(pid) || daemon["pid_start_time"] != pidStartTime || daemon["has_heartbeat"] != true {
		t.Fatalf("daemon supervisor row = %#v, want attached supervisor %s", daemon, supervisorID)
	}
}

func assertProgressDurability(t *testing.T, ctx context.Context, runner db.Runner, fx supervisionControlPGFixture) {
	t.Helper()
	lease := mustOneRow(t, ctx, runner, `
		SELECT last_heartbeat_at IS NOT NULL AS lease_heartbeat,
		       expires_at > now() AS lease_extended
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND lease_id = $2`,
		fx.repoID, fx.leaseID)
	if lease["lease_heartbeat"] != true || lease["lease_extended"] != true {
		t.Fatalf("lease row after progress = %#v, want heartbeat and fresh expiry", lease)
	}
	session := mustOneRow(t, ctx, runner, `
		SELECT last_heartbeat_at IS NOT NULL AS session_heartbeat,
		       last_work_heartbeat_at IS NOT NULL AS work_heartbeat,
		       last_pty_activity_at IS NOT NULL AS pty_activity
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`,
		fx.repoID, fx.sessionID)
	if session["session_heartbeat"] != true || session["work_heartbeat"] != true || session["pty_activity"] != true {
		t.Fatalf("session liveness row after progress = %#v, want heartbeat/work/pty timestamps", session)
	}
	assertEventCount(t, ctx, runner, fx, "lease.heartbeat", 1)
	source := mustScalar(t, ctx, runner, `
		SELECT COALESCE(payload_json->>'source', '')
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'lease.heartbeat'
		 ORDER BY event_id DESC
		 LIMIT 1`,
		fx.repoID, fx.runID)
	if source != "supervisor_pty_progress" {
		t.Fatalf("lease.heartbeat source = %q, want supervisor_pty_progress", source)
	}
	assertEventCount(t, ctx, runner, fx, "supervisor.progress", 0)
}

func poisonSupervisorStartTokenForStop(t *testing.T, ctx context.Context, runner db.Runner, repoID, supervisorID, daemonSupervisorID string) {
	t.Helper()
	const stale = "stale-before-stop"
	mustExec(t, ctx, runner, "poison process supervisor start token", `
		UPDATE striatumd.process_supervisors
		   SET pid_start_time = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		stale, repoID, supervisorID)
	mustExec(t, ctx, runner, "poison supervisor pointer start token", `
		UPDATE striatumd.process_supervisor_pointers
		   SET pid_start_time = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		stale, repoID, supervisorID)
	mustExec(t, ctx, runner, "poison daemon supervisor start token", `
		UPDATE striatumd.daemon_supervisors
		   SET pid_start_time = $1
		 WHERE daemon_supervisor_id = $2`,
		stale, daemonSupervisorID)
}

func assertSupervisorStoppedRows(t *testing.T, ctx context.Context, runner db.Runner, repoID, supervisorID, daemonSupervisorID string) {
	t.Helper()
	for _, check := range []struct {
		name             string
		sql              string
		args             []any
		wantReasonColumn bool
	}{
		{
			name: "process supervisor",
			sql: `SELECT state, ended_at IS NOT NULL AS ended, stop_reason FROM striatumd.process_supervisors
			       WHERE repository_id = $1 AND supervisor_id = $2`,
			args:             []any{repoID, supervisorID},
			wantReasonColumn: true,
		},
		{
			name: "supervisor pointer",
			sql: `SELECT state, TRUE AS ended, NULL::text AS stop_reason
			        FROM striatumd.process_supervisor_pointers
			       WHERE repository_id = $1 AND supervisor_id = $2`,
			args: []any{repoID, supervisorID},
		},
		{
			name: "daemon supervisor",
			sql: `SELECT state, ended_at IS NOT NULL AS ended, stop_reason FROM striatumd.daemon_supervisors
			       WHERE daemon_supervisor_id = $1`,
			args:             []any{daemonSupervisorID},
			wantReasonColumn: true,
		},
	} {
		row := mustOneRow(t, ctx, runner, check.sql, check.args...)
		if row["state"] != "stopped" || row["ended"] != true {
			t.Fatalf("%s row after stop = %#v, want stopped", check.name, row)
		}
		if check.wantReasonColumn && row["stop_reason"] != "operator_requested" {
			t.Fatalf("%s stop_reason = %v, want operator_requested", check.name, row["stop_reason"])
		}
	}
}

func assertStoppedEventPayload(t *testing.T, ctx context.Context, runner db.Runner, fx supervisionControlPGFixture, supervisorID string) {
	t.Helper()
	row := mustOneRow(t, ctx, runner, `
		SELECT payload_json->>'supervisor_id' AS supervisor_id,
		       payload_json->>'reason' AS reason,
		       payload_json ? 'pid_cleanup_skipped_reason' AS cleanup_skipped
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'supervisor.stopped'
		 ORDER BY event_id DESC
		 LIMIT 1`,
		fx.repoID, fx.runID)
	if row["supervisor_id"] != supervisorID || row["reason"] != "operator_requested" || row["cleanup_skipped"] != true {
		t.Fatalf("supervisor.stopped payload = %#v, want reason and guarded cleanup skip for %s", row, supervisorID)
	}
}

func assertEventCount(t *testing.T, ctx context.Context, runner db.Runner, fx supervisionControlPGFixture, eventType string, want int) {
	t.Helper()
	got := mustScalar(t, ctx, runner, `
		SELECT count(*)::text
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = $3`,
		fx.repoID, fx.runID, eventType)
	if got != fmt.Sprint(want) {
		t.Fatalf("event count %s = %s, want %d", eventType, got, want)
	}
}

func mustExec(t *testing.T, ctx context.Context, runner db.Runner, name string, sql string, args ...any) {
	t.Helper()
	if err := runner.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, sql)
	}
}

func mustOneRow(t *testing.T, ctx context.Context, runner db.Runner, sql string, args ...any) map[string]any {
	t.Helper()
	row, err := oneRow(ctx, runner, sql, args...)
	if err != nil {
		t.Fatalf("query one row: %v\n%s", err, sql)
	}
	return row
}

func mustScalar(t *testing.T, ctx context.Context, runner db.Runner, sql string, args ...any) string {
	t.Helper()
	value, err := runner.QueryScalar(ctx, sql, args...)
	if err != nil {
		t.Fatalf("query scalar: %v\n%s", err, sql)
	}
	return value
}
