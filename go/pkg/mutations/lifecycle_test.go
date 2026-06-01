package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestRegisterSessionDefaultsToWorkflowLaneCapabilities(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_caps"
	runID := "run_lifecycle_caps"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"claude_code": map[string]any{
				"capabilities": []any{"write", "interrogate", "write"},
			},
		},
	})

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"role":   "reviewer",
		"lane":   "claude_code",
	}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	got := registeredSessionCapabilities(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	want := []string{"write", "interrogate"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

// TestRegisterSessionMintsSessionBoundToken verifies RFC 0096 V2 / #135: the
// register-session handler mints a capability token BOUND to the new session,
// and the production authorizer resolves that token as session-bound (so the
// per-session enforcement will bite once lanes carry it).
func TestRegisterSessionMintsSessionBoundToken(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_token"
	runID := "run_lifecycle_token"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"capabilities": []any{"write", "claim"}}},
	})

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "claude",
	}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	sessionID := fmt.Sprint(result["session_id"])
	tokenInfo, ok := result["session_capability_token"].(map[string]any)
	if !ok {
		t.Fatalf("register did not return a session_capability_token: %#v", result)
	}
	token := fmt.Sprint(tokenInfo["token"])
	if token == "" || fmt.Sprint(tokenInfo["session_id"]) != sessionID {
		t.Fatalf("session_capability_token = %#v, want token bound to %s", tokenInfo, sessionID)
	}

	// The production authorizer must resolve this token as bound to the session.
	required := rpc.CapabilityWrite
	auth := rpc.PostgresAuthorizer{Runner: runner}
	resolved := auth.Authorize(&required, repoID, token)
	if resolved.Decision != "allowed" || !resolved.IsSessionBound() || resolved.SessionID != sessionID {
		t.Fatalf("minted token auth = %#v, want allowed bound to %s", resolved, sessionID)
	}
}

func TestRegisterSessionExplicitCapabilitiesOverrideLaneDefault(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_caps_explicit"
	runID := "run_lifecycle_caps_explicit"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"codex": map[string]any{
				"capabilities": []any{"write", "interrogate"},
			},
		},
	})

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id":     runID,
		"role":       "reviewer",
		"lane":       "codex",
		"capability": []any{"review", "review", " "},
	}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	got := registeredSessionCapabilities(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	want := []string{"review"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func registeredSessionCapabilities(t *testing.T, ctx context.Context, runner any, repoID, sessionID string) []string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT capabilities_json
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	result := []string{}
	for _, item := range asList(row["capabilities_json"]) {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

// TestRegisterSessionReplaceSupersedesPrior verifies RFC 0095 §5 (#60):
// register-session --replace atomically closes the prior active session on the
// same (run, role, lane) slot, transfers its leases, and registers a new active
// session. (Without --replace this supersession no longer happens — see
// TestRegisterSessionWithoutReplaceRefusesDuplicate.)
func TestRegisterSessionReplaceSupersedesPrior(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_supersession"
	runID := "run_lifecycle_supersession"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"codex": map[string]any{
				"capabilities": []any{"write", "interrogate"},
			},
		},
	})

	// 1. Register first session
	res1, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"role":   "reviewer",
		"lane":   "codex",
	}))
	if err != nil {
		t.Fatalf("register first session: %v", err)
	}
	sessID1 := fmt.Sprint(res1["session_id"])

	// Assert first session is active
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row["state"]) != "active" {
		t.Fatalf("first session should be active, got state: %v, err: %v", row["state"], err)
	}

	// Seed a job and active lease for the first session
	jobID := "job_1"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, idempotency_key, created_at
		)
		VALUES ($1, $2, $3, 'reviewer_job', 1, 'running', 'reviewer', 'Review Job', 'review', 'idem_1', NOW())`,
		repoID, jobID, runID)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	leaseID := "lease_1"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.leases (repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id, state, acquired_at, expires_at)
		VALUES ($1, $2, $3, 'job', $4, $5, 'active', NOW(), NOW() + INTERVAL '1 hour')`, repoID, leaseID, runID, jobID, sessID1)
	if err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	// Seed a claimed queue message for that job
	msgID := "msg_1"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (repository_id, message_id, run_id, job_id, kind, state, priority, target_session_id, target_role_id, target_lane_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'work', 'claimed', 0, $5, 'reviewer', 'codex', NOW(), NOW())`, repoID, msgID, runID, jobID, sessID1)
	if err != nil {
		t.Fatalf("insert queue message: %v", err)
	}

	// 2. Register second session on the same lane/role/run with --replace, which
	// opts in to closing+superseding the prior session and transferring its lease.
	res2, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id":  runID,
		"role":    "reviewer",
		"lane":    "codex",
		"replace": true,
	}))
	if err != nil {
		t.Fatalf("register second session: %v", err)
	}
	sessID2 := fmt.Sprint(res2["session_id"])

	// Assert first session is closed automatically
	row1, err := oneRow(ctx, runner, `SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row1["state"]) != "closed" || fmt.Sprint(row1["close_reason"]) != "superseded" {
		t.Fatalf("first session should be closed and superseded, got state: %v, reason: %v, err: %v", row1["state"], row1["close_reason"], err)
	}

	// Assert second session is active
	row2, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID2)
	if err != nil || fmt.Sprint(row2["state"]) != "active" {
		t.Fatalf("second session should be active, got state: %v, err: %v", row2["state"], err)
	}

	// Assert lease is released
	leaseRow, err := oneRow(ctx, runner, `SELECT state FROM striatumd.leases WHERE repository_id = $1 AND lease_id = $2`, repoID, leaseID)
	if err != nil || fmt.Sprint(leaseRow["state"]) != "released" {
		t.Fatalf("lease should be released, got state: %v, err: %v", leaseRow["state"], err)
	}

	// Assert job is queued
	jobRow, err := oneRow(ctx, runner, `SELECT state FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil || fmt.Sprint(jobRow["state"]) != "queued" {
		t.Fatalf("job should be queued, got state: %v, err: %v", jobRow["state"], err)
	}

	// Assert queue message is reset to pending
	msgRow, err := oneRow(ctx, runner, `SELECT state FROM striatumd.queue_messages WHERE repository_id = $1 AND message_id = $2`, repoID, msgID)
	if err != nil || fmt.Sprint(msgRow["state"]) != "pending" {
		t.Fatalf("queue message should be pending, got state: %v, err: %v", msgRow["state"], err)
	}
}

// TestRegisterSessionWithoutReplaceRefusesDuplicate verifies RFC 0095 §5 (#60):
// registering a second session on the same (run, role, lane) WITHOUT --replace
// is refused with the exact remediation (which session id to close), and the
// prior session is NOT implicitly superseded.
func TestRegisterSessionWithoutReplaceRefusesDuplicate(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_dup_refuse"
	runID := "run_lifecycle_dup_refuse"

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

	// Second registration without --replace must be refused with remediation.
	_, err = HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("duplicate register err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, sessID1) || !strings.Contains(rpcErr.Message, "--replace") {
		t.Fatalf("remediation message = %q, want it to name %s and --replace", rpcErr.Message, sessID1)
	}

	// The first session must still be active (NOT superseded).
	row1, err := oneRow(ctx, runner, `SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1)
	if err != nil || fmt.Sprint(row1["state"]) != "active" {
		t.Fatalf("first session should still be active, got state: %v, reason: %v, err: %v", row1["state"], row1["close_reason"], err)
	}
}

// TestRegisterSessionParallelSameRoleLaneBothActive verifies RFC 0095 §5
// (F-K/#75): two distinct active sessions on the same (role, lane) are allowed
// to coexist. Registration no longer implicitly supersedes an existing active
// session, so two parallel disjoint-scope jobs can each hold their own active
// session and `supervise start` finds an active session for each.
func TestRegisterSessionParallelSameRoleLaneBothActive(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_parallel"
	runID := "run_lifecycle_parallel"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})

	// First fresh session.
	res1, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "fresh": true,
	}))
	if err != nil {
		t.Fatalf("register first session: %v", err)
	}
	sessID1 := fmt.Sprint(res1["session_id"])

	// A second active session on the SAME (role, lane) is seeded directly, as a
	// launcher would for the second parallel disjoint-scope job. Under the old
	// behavior, registering the second would have superseded the first; the
	// invariant under #75 is that two distinct active sessions can coexist.
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, "sess_parallel_2", "reviewer", "codex", []string{"write"}, "active", 2)
	sessID2 := "sess_parallel_2"

	for _, id := range []string{sessID1, sessID2} {
		row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, id)
		if err != nil || fmt.Sprint(row["state"]) != "active" {
			t.Fatalf("session %s should be active, got state: %v, err: %v", id, row["state"], err)
		}
	}

	// And a *registration* of a third session without --replace must refuse,
	// listing BOTH active sessions as the ones to close — proving registration no
	// longer implicitly closes either.
	_, err = HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "codex", "fresh": true,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("third register err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, sessID1) || !strings.Contains(rpcErr.Message, sessID2) {
		t.Fatalf("remediation should list both active sessions, got %q", rpcErr.Message)
	}

	// Both originally-active sessions remain active after the refused registration.
	for _, id := range []string{sessID1, sessID2} {
		row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, id)
		if err != nil || fmt.Sprint(row["state"]) != "active" {
			t.Fatalf("session %s should remain active after refusal, got state: %v, err: %v", id, row["state"], err)
		}
	}
}

// #100: when the (role, lane) slot has more live parallel work than active
// sessions, a second registration SUCCEEDS (the documented disjoint-scope
// fanout) instead of being refused; a registration beyond the available work is
// still refused as an accidental duplicate.
func TestRegisterSessionAllowsParallelWhenWorkRemains(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_parallel_work"
	runID := "run_lifecycle_parallel_work"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"proposer": map[string]any{}},
		"lanes":       map[string]any{"claude_code": map[string]any{"capabilities": []any{"write"}}},
	})

	// Two parallel queued jobs on (proposer, claude_code), each with a pending
	// work message — the documented disjoint-scope fanout shape.
	for i := 1; i <= 2; i++ {
		jobID := fmt.Sprintf("job_p%d_%s", i, repoID)
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, expected_artifacts_json, created_at
			) VALUES ($1,$2,$3,$4,1,'queued','proposer','P','draft','idem_'||$2,'[]'::jsonb,NOW())`,
			repoID, jobID, runID, fmt.Sprintf("proposal_%d", i)); err != nil {
			t.Fatalf("insert job %d: %v", i, err)
		}
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.queue_messages (
			  repository_id, message_id, run_id, job_id, kind, state, priority,
			  target_role_id, target_lane_id, created_at, updated_at
			) VALUES ($1,$2,$3,$4,'work','pending',0,'proposer','claude_code',NOW(),NOW())`,
			repoID, fmt.Sprintf("msg_p%d_%s", i, repoID), runID, jobID); err != nil {
			t.Fatalf("insert message %d: %v", i, err)
		}
	}

	// First and second registrations both succeed (2 jobs > active sessions).
	for i := 1; i <= 2; i++ {
		res, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
			"run_id": runID, "role": "proposer", "lane": "claude_code", "fresh": true,
		}))
		if err != nil {
			t.Fatalf("#100: parallel registration %d should succeed, got %v", i, err)
		}
		if fmt.Sprint(res["session_id"]) == "" {
			t.Fatalf("registration %d returned no session_id: %#v", i, res)
		}
	}

	// A third registration has no remaining parallel work -> refused.
	_, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "proposer", "lane": "claude_code", "fresh": true,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("third register (no remaining work) err = %v, want invalid_transition", err)
	}

	// Both registered sessions are active and have distinct ordinals.
	rows, err := queryRows(ctx, runner, `SELECT ordinal FROM striatumd.sessions WHERE repository_id=$1 AND run_id=$2 AND role_id='proposer' AND lane_id='claude_code' AND state='active' ORDER BY ordinal`, repoID, runID)
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if len(rows) != 2 || intValue(rows[0]["ordinal"]) != 1 || intValue(rows[1]["ordinal"]) != 2 {
		t.Fatalf("#100: expected two active sessions with ordinals 1,2, got %#v", rows)
	}
}
