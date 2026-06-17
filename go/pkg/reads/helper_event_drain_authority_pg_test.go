package reads

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestReadSideHelperEventDrainsUseAuthorizedTx is the #329 regression: status,
// dashboard, and supervise read projections opportunistically drain helper
// events. Those helper events append through the same SECURITY DEFINER
// append_event_row path as mutations, so the read-side drain transaction must
// install the daemon-authority prelude before calling DrainHelperEventsHook.
func TestReadSideHelperEventDrainsUseAuthorizedTx(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)

	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}
	const secret = "s3cr3t-helper-drain"
	const salt = "helper-drain-salt"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ('inst-helper-drain', 'striatumd_rw',
		        encode(striatumd.digest(convert_to($1 || $2, 'UTF8'), 'sha256'), 'hex'), $2)`,
		secret, salt); err != nil {
		t.Fatalf("register secret: %v", err)
	}
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV3, "", false)
	db.SetActiveWriteBoundary(db.PhaseFull)
	t.Cleanup(func() {
		db.SetActiveWriteBoundary(db.PhaseNone)
		db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false)
	})

	seedHelperDrainFixture(t, ctx, pool.Runner)
	prevHook := DrainHelperEventsHook
	hookCalls := 0
	DrainHelperEventsHook = func(ctx context.Context, runner db.TxRunner, repositoryID string, supervisorID string) error {
		hookCalls++
		_, err := db.AppendEventRowSD(ctx, runner, db.EventRow{
			RepositoryID:   repositoryID,
			RunID:          "run_helper",
			EventType:      "supervisor.progress",
			ActorSessionID: "sess_helper",
			Payload: map[string]any{
				"supervisor_id": supervisorID,
				"source":        "helper_drain_pgtest",
				"call":          hookCalls,
			},
		})
		return err
	}
	t.Cleanup(func() { DrainHelperEventsHook = prevHook })

	countEvents := func() int {
		t.Helper()
		eventCount, err := pool.Runner.QueryScalar(ctx, `
			SELECT COUNT(*)::text
			  FROM striatumd.events
			 WHERE repository_id = 'repo_helper'
			   AND event_type = 'supervisor.progress'
			   AND payload_json->>'source' = 'helper_drain_pgtest'`)
		if err != nil {
			t.Fatalf("count helper drain events: %v", err)
		}
		count, err := strconv.Atoi(eventCount)
		if err != nil {
			t.Fatalf("parse helper drain event count %q: %v", eventCount, err)
		}
		return count
	}

	assertDrainAppends := func(label string, call func() error) {
		t.Helper()
		beforeCalls := hookCalls
		beforeEvents := countEvents()
		if err := call(); err != nil {
			t.Fatalf("%s helper drain: %v", label, err)
		}
		callDelta := hookCalls - beforeCalls
		eventDelta := countEvents() - beforeEvents
		if callDelta <= 0 {
			t.Fatalf("%s helper drain hook calls did not increase", label)
		}
		if eventDelta != callDelta {
			t.Fatalf("%s helper drain appended %d events, want %d", label, eventDelta, callDelta)
		}
	}

	assertDrainAppends("status", func() error {
		drainStatusHelperEvents(ctx, pool.Runner, "repo_helper", "sup_helper", map[string]any{"session_id": "sess_helper"})
		return nil
	})
	assertDrainAppends("dashboard", func() error {
		_, err := HandleDashboard(ctx, pool.Runner, rpc.Envelope{Params: map[string]any{
			"repository_id": "repo_helper",
			"run_id":        "run_helper",
		}})
		return err
	})
	assertDrainAppends("dashboard.all", func() error {
		_, err := HandleDashboardAll(ctx, pool.Runner, rpc.Envelope{})
		return err
	})
	assertDrainAppends("supervise.status", func() error {
		_, err := HandleSuperviseStatus(ctx, pool.Runner, rpc.Envelope{Params: map[string]any{
			"repository_id": "repo_helper",
			"session_id":    "sess_helper",
		}})
		return err
	})
	assertDrainAppends("supervise.reattach_status", func() error {
		_, err := HandleSuperviseReattachStatus(ctx, pool.Runner, rpc.Envelope{Params: map[string]any{
			"repository_id": "repo_helper",
			"run_id":        "run_helper",
		}})
		return err
	})

	if hookCalls < 5 {
		t.Fatalf("helper drain hook calls = %d, want at least 5", hookCalls)
	}
	if eventCount := countEvents(); eventCount != hookCalls {
		t.Fatalf("helper drain event count = %d, want hook call count %d", eventCount, hookCalls)
	}
}

func seedHelperDrainFixture(t *testing.T, ctx context.Context, runner db.Runner) {
	t.Helper()
	now := time.Date(2026, 6, 17, 1, 0, 0, 0, time.UTC)
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "repository",
			sql: `INSERT INTO striatumd.repositories (
				repository_id, repo_identity, repo_root, state_db_path, display_name,
				registered_at, last_schema_version, state
			) VALUES ('repo_helper','ident_helper','/tmp/repo-helper','/tmp/repo-helper/.striatum','repo-helper',$1,23,'active')`,
			args: []any{now},
		},
		{
			name: "workflow snapshot",
			sql: `INSERT INTO striatumd.workflow_snapshots (
				repository_id, workflow_snapshot_id, workflow_id, content_sha256,
				workflow_json, loaded_at
			) VALUES ('repo_helper','snap_helper','wf_helper','sha_helper','{}'::jsonb,$1)`,
			args: []any{now},
		},
		{
			name: "run",
			sql: `INSERT INTO striatumd.runs (
				repository_id, run_id, workflow_snapshot_id, repo_root, state, branch_name, created_at, started_at
			) VALUES ('repo_helper','run_helper','snap_helper','/tmp/repo-helper','running','main',$1,$1)`,
			args: []any{now},
		},
		{
			name: "session",
			sql: `INSERT INTO striatumd.sessions (
				repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
				capabilities_json, state, registered_at
			) VALUES ('repo_helper','sess_helper','run_helper','worker','codex','worker-codex',1,'[]'::jsonb,'active',$1)`,
			args: []any{now},
		},
		{
			name: "supervisor",
			sql: `INSERT INTO striatumd.process_supervisors (
				repository_id, supervisor_id, run_id, session_id, adapter, command_json,
				cwd, scratch_path, pid, pid_start_time, state, started_at, heartbeat_at
			) VALUES (
				'repo_helper','sup_helper','run_helper','sess_helper','codex','[]'::jsonb,
				'/tmp/repo-helper','/tmp/repo-helper/.striatum/scratch/sup_helper',
				$1,'helper-start','attached',$2,$2
			)`,
			args: []any{1, now},
		},
		{
			name: "supervisor pointer",
			sql: `INSERT INTO striatumd.process_supervisor_pointers (
				repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
				pid, pid_start_time, state, updated_at, metadata_json
			) VALUES (
				'repo_helper','sup_helper','daemon_sup_helper','run_helper','sess_helper',
				$1,'helper-start','attached',$2,'{}'::jsonb
			)`,
			args: []any{1, now},
		},
	}
	for _, stmt := range statements {
		if err := runner.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("seed %s: %v\n%s", stmt.name, err, stmt.sql)
		}
	}
}
