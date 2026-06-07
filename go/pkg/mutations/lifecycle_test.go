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

func registeredNonFreshReason(t *testing.T, ctx context.Context, runner any, repoID, sessionID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT non_fresh_reason
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("select non_fresh_reason: %v", err)
	}
	return fmt.Sprint(row["non_fresh_reason"])
}

func seedFreshReviewerPolicyRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string, freshReviewer bool) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	reviewJob := map[string]any{"id": "review", "type": "review"}
	if freshReviewer {
		reviewJob["reviewer_context_policy"] = "fresh"
	} else {
		reviewJob["reviewer_context_policy"] = "cross_round"
	}
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"author":   map[string]any{},
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"codex":    map[string]any{"capabilities": []any{"write"}},
			"reviewer": map[string]any{"capabilities": []any{"write"}},
		},
		"jobs": []any{reviewJob},
	})
}

func seedAuthorJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  created_at, completed_at
		)
		VALUES ($1,$2,$3,$4,1,$5,'author','{"lane_id":"codex"}'::jsonb,
		        $4,'draft','idem_'||$2,'[]'::jsonb,NOW(),
		        CASE WHEN $5 = 'completed' THEN NOW() ELSE NULL END)`,
		repoID, jobID, runID, jobID+"_wf", state); err != nil {
		t.Fatalf("insert author job %s: %v", jobID, err)
	}
}

func seedAuthorActiveWork(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, messageState string) {
	t.Helper()
	jobID := "job_work_" + sessionID
	leaseID := "lease_work_" + sessionID
	messageID := "msg_work_" + sessionID
	seedAuthorJob(t, ctx, runner, repoID, runID, jobID, "running")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '1 hour',NOW())`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert active author lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_session_id, target_role_id, target_lane_id, created_at, updated_at,
		  current_lease_id
		) VALUES ($1,$2,$3,$4,'work',$5,0,$6,'author','codex',NOW(),NOW(),$7)`,
		repoID, messageID, runID, jobID, messageState, sessionID, leaseID); err != nil {
		t.Fatalf("insert active author message: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET current_message_id = $1, current_lease_id = $2
		 WHERE repository_id = $3 AND job_id = $4`,
		messageID, leaseID, repoID, jobID); err != nil {
		t.Fatalf("stamp author job current ids: %v", err)
	}
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

	// #189: this test asserts the lease-transfer mechanics of --replace, not the
	// liveness gate. Backdate the first session past the heartbeat window so it
	// looks stranded and --replace proceeds without --force-live.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions SET last_heartbeat_at = NOW() - INTERVAL '2 hours', registered_at = NOW() - INTERVAL '2 hours'
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessID1); err != nil {
		t.Fatalf("backdate first session heartbeat: %v", err)
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

func TestFreshReviewerIgnoresDrainedIdleAuthor(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_fresh_drained_author"
	runID := "run_fresh_drained_author"
	authorSession := "sess_author_drained"

	seedFreshReviewerPolicyRun(t, ctx, runner, repoID, runID, true)
	intgSeedSession(t, ctx, runner, repoID, runID, authorSession, "author", "codex", []string{"write"}, "active")
	seedAuthorJob(t, ctx, runner, repoID, runID, "job_author_done", "completed")

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
	}))
	if err != nil {
		t.Fatalf("fresh reviewer should ignore drained idle author, got %v", err)
	}
	if fmt.Sprint(result["session_id"]) == "" {
		t.Fatalf("register result missing session_id: %#v", result)
	}
}

func TestFreshReviewerStillRefusesWorkingAuthor(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_fresh_working_author"
	runID := "run_fresh_working_author"
	authorSession := "sess_author_working"

	seedFreshReviewerPolicyRun(t, ctx, runner, repoID, runID, true)
	intgSeedSession(t, ctx, runner, repoID, runID, authorSession, "author", "codex", []string{"write"}, "active")
	seedAuthorActiveWork(t, ctx, runner, repoID, runID, authorSession, "acked")

	_, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error", err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition", rpcErr.Code)
	}
	if !strings.Contains(rpcErr.Message, authorSession) {
		t.Fatalf("message must name the active author session %q: %q", authorSession, rpcErr.Message)
	}
	if !strings.Contains(rpcErr.Message, "session close") {
		t.Fatalf("message must suggest releasing the author session via `session close`: %q", rpcErr.Message)
	}
	if !strings.Contains(rpcErr.Message, "--force-non-fresh") {
		t.Fatalf("message must still offer the --force-non-fresh escape hatch: %q", rpcErr.Message)
	}

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
		"force_non_fresh": true, "reason": "independent shell despite active author",
	}))
	if err != nil {
		t.Fatalf("--force-non-fresh with reason should override genuine refusal: %v", err)
	}
	reason := registeredNonFreshReason(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	if reason != "independent shell despite active author" {
		t.Fatalf("non_fresh_reason = %q", reason)
	}
}

func TestFreshReviewerStillRefusesAuthorWithPendingWork(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_fresh_pending_author"
	runID := "run_fresh_pending_author"
	authorSession := "sess_author_pending"

	seedFreshReviewerPolicyRun(t, ctx, runner, repoID, runID, true)
	intgSeedSession(t, ctx, runner, repoID, runID, authorSession, "author", "codex", []string{"write"}, "active")
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, "job_author_pending", "author_pending", "author", "codex")

	_, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("fresh reviewer with pending author work err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, authorSession) {
		t.Fatalf("message must name the active author session %q: %q", authorSession, rpcErr.Message)
	}
}

func TestForceNonFreshUnchangedSemantics(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_force_nonfresh_semantics"
	runID := "run_force_nonfresh_semantics"
	authorSession := "sess_author_force_nonfresh"

	seedFreshReviewerPolicyRun(t, ctx, runner, repoID, runID, true)
	intgSeedSession(t, ctx, runner, repoID, runID, authorSession, "author", "codex", []string{"write"}, "active")
	seedAuthorActiveWork(t, ctx, runner, repoID, runID, authorSession, "claimed")

	_, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer", "force_non_fresh": true,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("--force-non-fresh without reason err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, "requires a non-empty --reason") {
		t.Fatalf("message = %q, want reason requirement", rpcErr.Message)
	}

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
		"force_non_fresh": true, "reason": "operator accepted context contamination",
	}))
	if err != nil {
		t.Fatalf("--force-non-fresh with reason: %v", err)
	}
	reason := registeredNonFreshReason(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	if reason != "operator accepted context contamination" {
		t.Fatalf("non_fresh_reason = %q", reason)
	}
}

func TestFreshReviewerPolicyParityNonFreshWorkflow(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_nonfresh_policy_parity"
	runID := "run_nonfresh_policy_parity"
	authorSession := "sess_author_nonfresh_policy"

	seedFreshReviewerPolicyRun(t, ctx, runner, repoID, runID, false)
	intgSeedSession(t, ctx, runner, repoID, runID, authorSession, "author", "codex", []string{"write"}, "active")
	seedAuthorActiveWork(t, ctx, runner, repoID, runID, authorSession, "acked")

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "role": "reviewer", "lane": "reviewer",
	}))
	if err != nil {
		t.Fatalf("workflow without fresh reviewer policy should not run the author blocker predicate: %v", err)
	}
	if fmt.Sprint(result["session_id"]) == "" {
		t.Fatalf("register result missing session_id: %#v", result)
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

// TestSessionReportFlagsUnackedPacket is the W3 (#125) regression: a lane that
// reports readiness via session.report while it holds a claimed-but-unacked work
// packet must be told it still owes a work.ack — session.report is NOT a
// substitute for work.ack. Without this, a codex lane that reports ready instead
// of acking stalls silently (the job stays 'claimed', never 'running'). The report
// is still recorded (liveness is preserved) but the response flags the outstanding
// packet so the control plane and the pane agree.
func TestSessionReportFlagsUnackedPacket(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_session_report_ack"
	runID := "run_" + repoID
	sessionID := "sess_codex"
	role := "implementer"
	lane := "codex"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Codex"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionID, lane)

	jobID := "job_impl"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, created_at
		) VALUES ($1,$2,$3,'impl',1,'queued',$4,'Impl','draft','idem_impl',NOW())`,
		repoID, jobID, runID, role); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, target_role_id,
		  target_lane_id, priority, created_at, updated_at
		) VALUES ($1,'msg_impl',$2,$3,'work','pending',$4,$5,10,NOW(),NOW())`,
		repoID, runID, jobID, role, lane); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}

	claim, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claim["status"] != "claimed" {
		t.Fatalf("claim status = %v, want claimed", claim["status"])
	}
	lease := claim["packet"].(map[string]any)["lease"].(map[string]any)
	messageID := fmt.Sprint(lease["message_id"])
	leaseID := fmt.Sprint(lease["lease_id"])

	// The lane reports readiness INSTEAD of acking — pre-fix this silently masks
	// the stall (the job stays 'claimed'). The report must flag the unacked packet.
	report, err := HandleSessionReport(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID, "report_kind": "ready", "phase": "await_packet",
	}))
	if err != nil {
		t.Fatalf("session.report: %v", err)
	}
	unacked, ok := report["unacked_packet"].(map[string]any)
	if !ok {
		t.Fatalf("session.report with a claimed-unacked packet must flag unacked_packet, got %#v", report)
	}
	if fmt.Sprint(unacked["message_id"]) != messageID {
		t.Fatalf("unacked_packet.message_id = %v, want %s", unacked["message_id"], messageID)
	}
	if strings.TrimSpace(fmt.Sprint(unacked["guidance"])) == "" {
		t.Fatalf("unacked_packet must carry work.ack guidance, got %#v", unacked)
	}

	// After a real work.ack, a subsequent report carries no flag (the pane and the
	// control plane now agree).
	if _, err := HandleAckWork(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID, "message_id": messageID, "lease_id": leaseID,
	})); err != nil {
		t.Fatalf("work.ack: %v", err)
	}
	report2, err := HandleSessionReport(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID, "report_kind": "ready", "phase": "lease_held",
	}))
	if err != nil {
		t.Fatalf("session.report after ack: %v", err)
	}
	if _, flagged := report2["unacked_packet"]; flagged {
		t.Fatalf("session.report after work.ack must NOT flag an unacked packet, got %#v", report2)
	}
}

func TestCompletionRequeuesReadyBlockedSibling(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_ready_blocked_sibling"
	runID := "run_ready_blocked_sibling"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"cross_examiner": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{}},
	})

	insertLifecycleJob(t, ctx, runner, repoID, runID, "job_lane_done", "lane_done", "completed")
	insertLifecycleJob(t, ctx, runner, repoID, runID, "job_gate", "gate", "completed")
	insertLifecycleJob(t, ctx, runner, repoID, runID, "job_ready", "ready_sibling", "blocked")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id, gate_json)
		VALUES ($1,'job_ready','job_gate','{"on":"completed"}'::jsonb)`, repoID); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return nil, maybeEnqueueDownstream(ctx, tx, repoID, "job_lane_done")
	}); err != nil {
		t.Fatalf("maybeEnqueueDownstream: %v", err)
	}

	if got := jobState(t, ctx, runner, repoID, "job_ready"); got != "queued" {
		t.Fatalf("job_ready state = %q, want queued", got)
	}
	row, err := oneRow(ctx, runner, `
		SELECT target_lane_id, state
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND job_id = 'job_ready'
		 ORDER BY created_at DESC
		 LIMIT 1`, repoID)
	if err != nil {
		t.Fatalf("select queued message: %v", err)
	}
	if fmt.Sprint(row["state"]) != "pending" || fmt.Sprint(row["target_lane_id"]) != "codex" {
		t.Fatalf("queued message = %#v, want pending codex message", row)
	}
}

func insertLifecycleJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  created_at, completed_at
		)
		VALUES ($1,$2,$3,$4,1,$5,'cross_examiner','{"lane_id":"codex"}'::jsonb,
		        $4,'review','idem_'||$2,'[]'::jsonb,NOW(),
		        CASE WHEN $5 = 'completed' THEN NOW() ELSE NULL END)`,
		repoID, jobID, runID, workflowJobID, state); err != nil {
		t.Fatalf("insert job %s: %v", jobID, err)
	}
}
