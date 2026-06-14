package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// #273: run.retry_job bumps the attempt via reopenJobForAttempt, so retrying a
// job already at its max_attempts ceiling silently exceeds the configured
// attempt budget (during a worktree-durability recovery this minted a duplicate
// attempt + lane instead of completing the remediated one). It must refuse by
// default and require an explicit, audited --allow-exceed-max-attempts override.
func TestRetryJobRefusesExceedingMaxAttemptsUnlessOverridden(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "retry_budget", false)

	// Park the job at the attempt-budget ceiling, retriable (blocked, lease released).
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET attempt = 1, max_attempts = 1, state = 'blocked', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("park job at budget ceiling: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases SET state = 'released', released_at = NOW()
		 WHERE repository_id = $1 AND lease_id = $2`, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release lease: %v", err)
	}

	// Without the override: refuse, attempt untouched.
	_, err := HandleRunRetryJob(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id": ids.runID,
		"job_id": ids.jobID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" || !strings.Contains(rpcErr.Message, "exceeding max_attempts") {
		t.Fatalf("retry at budget ceiling without override = %v, want a max_attempts refusal", err)
	}
	if got := jobAttempt(t, ctx, runner, ids.repoID, ids.jobID); got != 1 {
		t.Fatalf("attempt = %d after refused retry, want 1 (no bump)", got)
	}

	// With the explicit override: succeeds, attempt bumped, recorded as an audited
	// operator override on job.retried.
	if _, err := HandleRunRetryJob(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":                    ids.runID,
		"job_id":                    ids.jobID,
		"allow_exceed_max_attempts": true,
	})); err != nil {
		t.Fatalf("retry with --allow-exceed-max-attempts: %v", err)
	}
	if got := jobAttempt(t, ctx, runner, ids.repoID, ids.jobID); got != 2 {
		t.Fatalf("attempt = %d after override retry, want 2", got)
	}
	n := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'job.retried'
		   AND payload_json->>'attempt_budget_override' = 'true'`,
		ids.repoID, ids.runID)
	if n != 1 {
		t.Fatalf("job.retried attempt_budget_override events = %d, want exactly 1", n)
	}
}
