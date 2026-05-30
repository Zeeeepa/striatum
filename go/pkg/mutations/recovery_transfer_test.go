package mutations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedRepoWriteClaimedJob seeds a running repo-write job with an active lease
// owned by `wrongSession` and an acked work message — the live-but-wrong claim
// #82 is about. Returns the run/job/lease/message ids.
func seedRepoWriteClaimedJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, wrongSession string) (runID, jobID, leaseID, msgID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_synth_" + repoID
	leaseID = "lease_wrong_" + repoID
	msgID = "msg_synth_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"synthesizer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "forum_synthesis", "type": "synthesis", "role_id": "synthesizer"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, wrongSession, "synthesizer", "claude", []string{"write"}, "active")

	now := time.Now().UTC()
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"docs/"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  created_at, started_at
		) VALUES ($1,$2,$3,'forum_synthesis',1,'running','synthesizer','Synthesis','synthesis',
		          'idem_synth_'||$1,'[]'::jsonb,$4::jsonb,$5,$5)`,
		repoID, jobID, runID, wsArg, now); err != nil {
		t.Fatalf("insert repo-write job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'synthesizer','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '1 hour')`,
		repoID, leaseID, runID, jobID, wrongSession); err != nil {
		t.Fatalf("insert active lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_message_id = $3, current_lease_id = $4
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, msgID, leaseID); err != nil {
		t.Fatalf("link job pointers: %v", err)
	}
	return runID, jobID, leaseID, msgID
}

// #82: an operator-inspected `--force` requeue transfers a live-but-wrong
// repo-write claim to a fresh session, preserving the attempt counter (a lease
// ownership correction, NOT a content retry).
func TestRequeueStaleForceTransfersRepoWriteWithoutBumpingAttempt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lease_transfer"
	runID, jobID, leaseID, msgID := seedRepoWriteClaimedJob(t, ctx, runner, repoID, "sess_wrong_"+repoID)

	// Without --force, the live-claimant case guides the operator to the
	// transfer path instead of a bare "no stale lease" error.
	_, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID,
	}))
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected live-claimant guidance toward --force, got %v", err)
	}

	result, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID, "force": true,
		"justification": "lease ownership correction: stale convener wrongly claimed the revision",
	}))
	if err != nil {
		t.Fatalf("force requeue: %v", err)
	}
	if result["status"] != "requeued" {
		t.Fatalf("status = %v, want requeued; %#v", result["status"], result)
	}
	if result["operator_override"] != true {
		t.Fatalf("expected operator_override=true, got %#v", result)
	}

	// Attempt MUST be unchanged (1) — this is the whole point of #82.
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != 1 {
		t.Fatalf("#82: attempt = %d, want 1 (lease correction must not bump attempt)", got)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (reclaimable by a fresh session)", got)
	}
	// The same work message is reused, now pending.
	if got := messageState(t, ctx, runner, repoID, msgID); got != "pending" {
		t.Fatalf("message state = %q, want pending", got)
	}
	// The wrong claimant's active lease was force-expired.
	if got := leaseState(t, ctx, runner, repoID, leaseID); got != "expired" {
		t.Fatalf("wrong claimant lease state = %q, want expired", got)
	}
}

func messageState(t *testing.T, ctx context.Context, runner db.Runner, repoID, msgID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.queue_messages WHERE repository_id=$1 AND message_id=$2`, repoID, msgID)
	if err != nil {
		t.Fatalf("message state: %v", err)
	}
	return fmt.Sprint(row["state"])
}

func leaseState(t *testing.T, ctx context.Context, runner db.Runner, repoID, leaseID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.leases WHERE repository_id=$1 AND lease_id=$2`, repoID, leaseID)
	if err != nil {
		t.Fatalf("lease state: %v", err)
	}
	return fmt.Sprint(row["state"])
}
