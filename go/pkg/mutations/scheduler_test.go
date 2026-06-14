package mutations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// fakeSpawnExecutor stands in for the real register + supervise.start path so the
// scheduler's decision + attribution wiring is testable without launching a lane
// subprocess. It records the owner principal the scheduler attributes each spawn
// to (read off the ctx the scheduler hands it) and inserts an active session for
// the slot, so a subsequent sweep sees the lane as already spawned (the C4
// idempotent double-spawn guard).
type fakeSpawnExecutor struct {
	calls  []fakeSpawnCall
	nextID int
	failOn map[string]bool // role/lane keys to fail (no session inserted)
}

type fakeSpawnCall struct {
	ownerPrincipal string
	repositoryID   string
	runID          string
	role           string
	lane           string
}

func (f *fakeSpawnExecutor) Spawn(ctx context.Context, runner db.Runner, repositoryID, runID, role, lane string) (string, error) {
	f.calls = append(f.calls, fakeSpawnCall{
		ownerPrincipal: db.AuthorityFromContext(ctx).PrincipalID,
		repositoryID:   repositoryID,
		runID:          runID,
		role:           role,
		lane:           lane,
	})
	if f.failOn[role+"/"+lane] {
		return "", fmt.Errorf("injected spawn failure for %s/%s", role, lane)
	}
	f.nextID++
	sessionID := fmt.Sprintf("sched_sess_%d", f.nextID)
	// Insert an active session for the slot so a later sweep adopts (skips) it —
	// the production effect of register + supervise.start, sans subprocess.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'["write"]'::jsonb,'active',now())`,
		repositoryID, sessionID, runID, role, lane, sessionID+"-slug", f.nextID); err != nil {
		return "", err
	}
	return sessionID, nil
}

// startAutoSpawnRun seeds + starts an auto_spawn run under the given owner client,
// capturing a grant, and returns the run id. The root job is queued by run.start.
func startAutoSpawnRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, ownerClient string) {
	t.Helper()
	t.Setenv(supervisedLaneOSUserEnv, "") // daemon-user run-as resolves cleanly
	seedAutoSpawnReadyRun(t, ctx, runner, repoID, runID, true)
	authCtx, env := ownerAuthEnv(repoID, ownerClient, map[string]any{"run_id": runID})
	if _, err := HandleRunStart(authCtx, runner, env); err != nil {
		t.Fatalf("run.start: %v", err)
	}
}

// TestSchedulerSpawnAttributionUsesOwnerPrincipal (C1): the scheduler attributes
// each spawn to the captured run owner, so a scheduler-spawned lane is
// indistinguishable from one the owner spawned via supervise.start.
func TestSchedulerSpawnAttributionUsesOwnerPrincipal(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_attr", "run_sched_attr"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_attr")

	fake := &fakeSpawnExecutor{}
	result, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce: %v", err)
	}
	if result["spawned"] != 1 {
		t.Fatalf("spawned = %v, want 1; result=%#v", result["spawned"], result)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(fake.calls))
	}
	call := fake.calls[0]
	if call.ownerPrincipal != "client_owner_attr" {
		t.Fatalf("spawn attributed to %q, want client_owner_attr (C1)", call.ownerPrincipal)
	}
	if call.role != "author" || call.lane != "codex" {
		t.Fatalf("spawn role/lane = %s/%s, want author/codex", call.role, call.lane)
	}
}

// TestSchedulerNoDoubleSpawnAcrossSweeps (C4): a second sweep does not re-spawn a
// slot whose session is already active — the shared predicate adopts it instead.
func TestSchedulerNoDoubleSpawnAcrossSweeps(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_nodouble", "run_sched_nodouble"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_nd")

	fake := &fakeSpawnExecutor{}
	first, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if first["spawned"] != 1 {
		t.Fatalf("sweep 1 spawned = %v, want 1", first["spawned"])
	}
	second, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if second["spawned"] != 0 {
		t.Fatalf("sweep 2 spawned = %v, want 0 (slot already has an active session); result=%#v", second["spawned"], second)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("executor called %d times across two sweeps, want 1", len(fake.calls))
	}
}

// TestSchedulerNoDoubleSpawnExistingSession (C4): a slot already served by an
// active session is adopted, not re-spawned, on the very first sweep.
func TestSchedulerNoDoubleSpawnExistingSession(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_existing", "run_sched_existing"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_ex")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_preexisting", "author", "codex", []string{"write"}, "active")

	fake := &fakeSpawnExecutor{}
	result, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce: %v", err)
	}
	if result["spawned"] != 0 {
		t.Fatalf("spawned = %v, want 0 (existing session adopted)", result["spawned"])
	}
	if len(fake.calls) != 0 {
		t.Fatalf("executor called %d times, want 0", len(fake.calls))
	}
}

// TestSchedulerRespectsHoldPausedRun (C5): a paused run holds — the scheduler
// launches no new lanes.
func TestSchedulerRespectsHoldPausedRun(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_paused", "run_sched_paused"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_paused")
	if _, err := HandleRunPause(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.pause: %v", err)
	}

	fake := &fakeSpawnExecutor{}
	result, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce: %v", err)
	}
	if result["status"] != "paused" || result["spawned"] != 0 {
		t.Fatalf("result = %#v, want paused/0", result)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("paused run spawned %d lanes, want 0", len(fake.calls))
	}
}

// TestSchedulerRespectsHoldNonAutoSpawnLane (human-hold): a queued job on a lane
// that did not opt into auto_spawn is left for a human/operator — the scheduler
// spawns only auto_spawn lanes.
func TestSchedulerRespectsHoldNonAutoSpawnLane(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_hold_lane", "run_sched_hold_lane"
	t.Setenv(supervisedLaneOSUserEnv, "")
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}, "manual": map[string]any{}},
		"lanes": map[string]any{
			"codex":  map[string]any{"capabilities": []any{"write"}, "supervision": map[string]any{"auto_spawn": true}},
			"manual": map[string]any{"capabilities": []any{"write"}},
		},
		"jobs": []any{
			map[string]any{"id": "auto_job", "type": "draft", "role_id": "author"},
			map[string]any{"id": "held_job", "type": "draft", "role_id": "manual"},
		},
	})
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET state='ready' WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	// Two independent queued root jobs: one on the auto_spawn lane, one held.
	insertSchedulerJob(t, ctx, runner, repoID, runID, "job_auto", "auto_job", "author", "codex", "queued")
	insertSchedulerJob(t, ctx, runner, repoID, runID, "job_held", "held_job", "manual", "manual", "queued")
	// Run is already running so the queued jobs are visible; capture a grant.
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET state='running' WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("running: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.spawn_authorization_grants (
		  repository_id, grant_id, run_id, owner_principal_id, run_as_spec,
		  capability_envelope, created_at, expires_at
		) VALUES ($1,'sgrant_hold',$2,'client_owner_hold','{"mode":"daemon_user"}'::jsonb,
		  '{"capabilities":["claim"]}'::jsonb, now(), now() + interval '1 day')`,
		repoID, runID); err != nil {
		t.Fatalf("insert grant: %v", err)
	}

	fake := &fakeSpawnExecutor{}
	result, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce: %v", err)
	}
	if result["spawned"] != 1 {
		t.Fatalf("spawned = %v, want 1 (only the auto_spawn lane); result=%#v", result["spawned"], result)
	}
	if len(fake.calls) != 1 || fake.calls[0].lane != "codex" {
		t.Fatalf("calls = %#v, want a single codex spawn (manual lane held)", fake.calls)
	}
}

// TestSchedulerRefusesExpiredGrant (C2): an expired grant is a loud refusal, never
// a silent spawn.
func TestSchedulerRefusesExpiredGrant(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_expired", "run_sched_expired"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_exp")
	if err := runner.Exec(ctx, `
		UPDATE striatumd.spawn_authorization_grants
		   SET expires_at = now() - interval '1 hour'
		 WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("expire grant: %v", err)
	}

	fake := &fakeSpawnExecutor{}
	_, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "spawn_grant_expired" {
		t.Fatalf("err = %v, want spawn_grant_expired", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("expired grant spawned %d lanes, want 0", len(fake.calls))
	}
}

// TestSchedulerRefusesMissingGrant (C2): auto_spawn work with no active grant
// (revoked) is a loud refusal.
func TestSchedulerRefusesMissingGrant(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_nogrant", "run_sched_nogrant"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_ng")
	if err := runner.Exec(ctx, `
		UPDATE striatumd.spawn_authorization_grants
		   SET revoked_at = now(), revoke_reason = 'test'
		 WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}

	fake := &fakeSpawnExecutor{}
	_, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "spawn_grant_missing" {
		t.Fatalf("err = %v, want spawn_grant_missing", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("missing grant spawned %d lanes, want 0", len(fake.calls))
	}
}

// TestSchedulerTerminalRunNoSpawn: a terminal run is not drivable.
func TestSchedulerTerminalRunNoSpawn(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_terminal", "run_sched_terminal"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_term")
	if _, err := HandleRunCancel(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "reason": "test"})); err != nil {
		t.Fatalf("run.cancel: %v", err)
	}

	fake := &fakeSpawnExecutor{}
	result, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", fake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce: %v", err)
	}
	if result["spawned"] != 0 {
		t.Fatalf("terminal run spawned %v, want 0", result["spawned"])
	}
	if len(fake.calls) != 0 {
		t.Fatalf("terminal run called executor %d times, want 0", len(fake.calls))
	}
}

func insertSchedulerJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, role, lane, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  created_at
		) VALUES ($1,$2,$3,$4,1,$5,$6,$7::jsonb,'Job','draft','idem_'||$2,'[]'::jsonb,NOW())`,
		repoID, jobID, runID, workflowJobID, state, role,
		fmt.Sprintf(`{"lane_id":%q}`, lane)); err != nil {
		t.Fatalf("insert scheduler job %s: %v", jobID, err)
	}
}
