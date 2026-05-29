package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
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

func TestRegisterSessionAutomatedSupersession(t *testing.T) {
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

	// 2. Register second session on the same lane/role/run
	res2, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"role":   "reviewer",
		"lane":   "codex",
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
