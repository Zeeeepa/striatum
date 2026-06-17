package mutations

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// #356: HandleHeartbeat (work.heartbeat / heartbeat) is the input the entire
// stall-classification / recovery decision tree keys off (lease extension,
// session liveness, the lease.heartbeat event) yet had ZERO behavioral test
// coverage. These tests cover lease renewal, the session+lease heartbeat
// timestamps, the emitted event, and the schema/lease error paths.

// seedHeartbeatLease seeds a repo + run + active session that holds an active
// lease on a running job, and returns the lease/session ids. It mirrors the
// post-claim state every supervised lane heartbeats from.
func seedHeartbeatLease(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, sessionID, leaseID string) {
	t.Helper()
	runID = "run_" + repoID
	sessionID = "sess_hb_" + repoID
	leaseID = "lease_hb_" + repoID
	jobID := "job_hb_" + repoID
	role := "implementer"
	lane := "claude"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, []string{"write"}, "active")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, created_at
		) VALUES ($1,$2,$3,'hb',1,'running',$4,'HB','draft','idem_'||$2,NOW())`,
		repoID, jobID, runID, role); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	// A short-dated lease so the renewal is observable as a strict extension.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '60 seconds',NOW() - INTERVAL '5 seconds')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	return runID, sessionID, leaseID
}

func TestHeartbeatRenewsLeaseAndEmitsEvent(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_heartbeat_renew"
	runID, sessionID, leaseID := seedHeartbeatLease(t, ctx, runner, repoID)

	priorLeaseHB := scalarOrEmpty(t, ctx, runner,
		`SELECT last_heartbeat_at::text FROM striatumd.leases WHERE repository_id=$1 AND lease_id=$2`, repoID, leaseID)

	result, err := HandleHeartbeat(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID, "lease_id": leaseID, "extend_seconds": 1800,
	}))
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if result["status"] != "heartbeat" {
		t.Fatalf("status = %v, want heartbeat", result["status"])
	}
	expiresAt := fmt.Sprint(result["expires_at"])
	if expiresAt == "" || expiresAt == "<nil>" {
		t.Fatalf("heartbeat must return the new expires_at, got %#v", result)
	}

	// The returned expires_at parses to a time ~extend_seconds in the future.
	returnedExpiry, ok := asTime(expiresAt)
	if !ok {
		t.Fatalf("could not parse returned expires_at %q", expiresAt)
	}

	// The lease's expires_at is extended to ~now+extend_seconds, far past the
	// original 60s window, and matches the returned value (compared as parsed
	// instants, since the ::text rendering differs from the RFC3339 response).
	row, err := oneRow(ctx, runner, `
		SELECT expires_at, last_heartbeat_at::text AS last_heartbeat_at
		  FROM striatumd.leases WHERE repository_id=$1 AND lease_id=$2`, repoID, leaseID)
	if err != nil {
		t.Fatalf("read lease: %v", err)
	}
	newExpiry, ok := asTime(row["expires_at"])
	if !ok {
		t.Fatalf("could not parse lease expires_at %v", row["expires_at"])
	}
	if !newExpiry.Equal(returnedExpiry) {
		t.Fatalf("lease expires_at = %v, want the returned %v", newExpiry, returnedExpiry)
	}
	if !newExpiry.After(time.Now().UTC().Add(20 * time.Minute)) {
		t.Fatalf("lease expires_at = %v, want ~now+30m (extended past the 60s window)", newExpiry)
	}
	if fmt.Sprint(row["last_heartbeat_at"]) == priorLeaseHB {
		t.Fatalf("lease last_heartbeat_at was not advanced (still %s)", priorLeaseHB)
	}

	// The session's last_heartbeat_at advances too (session liveness).
	sessHB := scalarOrEmpty(t, ctx, runner,
		`SELECT last_heartbeat_at::text FROM striatumd.sessions WHERE repository_id=$1 AND session_id=$2`, repoID, sessionID)
	if sessHB == "" {
		t.Fatalf("session last_heartbeat_at was not set by heartbeat")
	}

	// A lease.heartbeat event landed on the run, carrying the new expires_at.
	evt, err := oneRow(ctx, runner, `
		SELECT actor_session_id, lease_id, payload_json
		  FROM striatumd.events
		 WHERE repository_id=$1 AND run_id=$2 AND event_type='lease.heartbeat'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read heartbeat event: %v", err)
	}
	if fmt.Sprint(evt["actor_session_id"]) != sessionID || fmt.Sprint(evt["lease_id"]) != leaseID {
		t.Fatalf("lease.heartbeat event actor/lease = %v/%v, want %s/%s",
			evt["actor_session_id"], evt["lease_id"], sessionID, leaseID)
	}
	payload := asMap(evt["payload_json"])
	if fmt.Sprint(payload["expires_at"]) != expiresAt {
		t.Fatalf("lease.heartbeat event payload expires_at = %v, want %s", payload["expires_at"], expiresAt)
	}
}

func TestHeartbeatRequiresSessionAndLease(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_heartbeat_schema"
	_, sessionID, leaseID := seedHeartbeatLease(t, ctx, runner, repoID)

	for _, tc := range []struct {
		name   string
		params map[string]any
	}{
		{"missing lease_id", map[string]any{"session_id": sessionID}},
		{"missing session_id", map[string]any{"lease_id": leaseID}},
		{"both missing", map[string]any{}},
	} {
		_, err := HandleHeartbeat(ctx, runner, intgEnv(repoID, tc.params))
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
			t.Fatalf("%s: err = %v, want schema_invalid", tc.name, err)
		}
	}
}

func TestHeartbeatRejectsForeignAndInactiveLease(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_heartbeat_lease_err"
	runID, sessionID, leaseID := seedHeartbeatLease(t, ctx, runner, repoID)

	// A second active session on the run that does NOT own the lease.
	otherSession := "sess_hb_other_" + repoID
	intgSeedSession(t, ctx, runner, repoID, runID, otherSession, "reviewer", "claude", []string{"write"}, "active")

	// Foreign owner: the lease belongs to sessionID, not otherSession.
	_, err := HandleHeartbeat(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": otherSession, "lease_id": leaseID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lease_error" {
		t.Fatalf("foreign-owner heartbeat err = %v, want lease_error", err)
	}

	// Released lease: heartbeat must refuse a non-active lease.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases SET state='released', released_at=NOW()
		 WHERE repository_id=$1 AND lease_id=$2`, repoID, leaseID); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	_, err = HandleHeartbeat(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID, "lease_id": leaseID,
	}))
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lease_error" {
		t.Fatalf("released-lease heartbeat err = %v, want lease_error", err)
	}
}

func scalarOrEmpty(t *testing.T, ctx context.Context, runner db.Runner, sql string, args ...any) string {
	t.Helper()
	value, err := runner.QueryScalar(ctx, sql, args...)
	if err != nil {
		t.Fatalf("query scalar: %v", err)
	}
	return value
}
