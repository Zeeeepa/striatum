package reads

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestEscalationResolveThroughWriteBoundary is the #176 regression: under
// pg_write_boundary=full, escalation.resolve appends its escalation.resolved
// event through the owner-owned SECURITY DEFINER append_event_row, which asserts
// daemon authority from the prelude-installed striatum.daemon_auth GUC. Before the
// fix, withResolveTx opened its transaction with a bare runner.BeginTx (no
// authority prelude), so the SD function raised SQLSTATE 28000 and the handler
// returned daemon_auth_lost while every other write verb (which all open through
// db.BeginAuthorizedMutation) succeeded. The fix routes withResolveTx through
// db.BeginAuthorizedMutation; this test exercises the resolve through the real
// boundary and proves it resolves the escalation and lands the event instead of
// raising daemon_auth_lost.
//
// The existing fake-tx unit test cannot catch this: its fake transaction does not
// implement the extended-protocol boundExecer (so the prelude is a no-op) and does
// not enforce the write boundary, so the SD path never runs.
func TestEscalationResolveThroughWriteBoundary(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)

	// Apply the production owner bundles so the SECURITY DEFINER
	// append_event_row + its authority gate exist on this test database.
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	// Register a per-instance daemon-authority secret and install it as the
	// process authority runtime so the prelude presents the matching secret.
	const secret = "s3cr3t-escalation-resolve"
	const salt = "per-instance-salt"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ('inst-test', 'striatumd_rw',
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

	now := time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC)
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_esc','ident_esc','/tmp/repo-esc','/tmp/repo-esc/.striatum','repo-esc',$1,23,'active')`,
		now); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256,
		  workflow_json, loaded_at
		) VALUES ('repo_esc','snap_esc','wf_esc','sha_esc','{}'::jsonb,$1)`,
		now); err != nil {
		t.Fatalf("seed workflow snapshot: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_esc','run_esc','snap_esc','/tmp/repo-esc','running',$1)`,
		now); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		) VALUES ('repo_esc','blk_esc','run_esc',NULL,NULL,'blocked',
		          'missing_authority','needs principal decision','open',$1,'{}'::jsonb)`,
		now); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.escalation_inbox (
		  repository_id, escalation_id, run_id, job_id, session_id,
		  blocker_id, blocker_kind, severity, state, created_at, payload_json
		) VALUES ('repo_esc','blk_esc','run_esc',NULL,NULL,
		          'blk_esc','missing_authority','blocked','pending',$1,'{}'::jsonb)`,
		now); err != nil {
		t.Fatalf("seed escalation inbox: %v", err)
	}

	response, err := HandleEscalationResolve(ctx, pool.Runner, rpc.Envelope{Params: map[string]any{
		"repository_id":   "repo_esc",
		"escalation_id":   "blk_esc",
		"resolution_note": "resolved through the write boundary",
	}})
	if err != nil {
		var rpcErr *rpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == "daemon_auth_lost" {
			t.Fatalf("#176: escalation.resolve raised daemon_auth_lost through pg_write_boundary=full; "+
				"withResolveTx must open its transaction via db.BeginAuthorizedMutation so the authority "+
				"prelude runs before the SECURITY DEFINER append_event_row: %v", err)
		}
		t.Fatalf("escalation.resolve failed unexpectedly: %v", err)
	}
	if response["status"] != "resolved" {
		t.Fatalf("response status = %v, want resolved", response["status"])
	}

	// The blocker must be resolved and the escalation.resolved event must have
	// landed via the SD path.
	blockerState, err := pool.Runner.QueryScalar(ctx,
		"SELECT state FROM striatumd.blockers WHERE repository_id = 'repo_esc' AND blocker_id = 'blk_esc'")
	if err != nil {
		t.Fatalf("read blocker state: %v", err)
	}
	if blockerState != "resolved" {
		t.Fatalf("blocker state = %q, want resolved", blockerState)
	}
	eventCount, err := pool.Runner.QueryScalar(ctx,
		"SELECT COUNT(*)::text FROM striatumd.events WHERE repository_id = 'repo_esc' AND event_type = 'escalation.resolved'")
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != "1" {
		t.Fatalf("escalation.resolved event count = %s, want 1 (the SD append must have landed)", eventCount)
	}
}
