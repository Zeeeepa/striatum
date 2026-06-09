package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// activeLeaseCount returns how many active leases are pinned to the given job.
// A re-open must leave zero so a fresh claim cannot fail the
// uq_active_resource_lease partial unique index with `duplicate active job lease`.
func activeLeaseCount(t *testing.T, ctx context.Context, runner any, repoID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.leases
		 WHERE repository_id = $1 AND resource_type = 'job' AND resource_id = $2
		   AND state = 'active'`, repoID, jobID)
	if err != nil {
		t.Fatalf("count active leases for %s: %v", jobID, err)
	}
	return intValue(row["n"])
}

// pendingWorkMessageCount returns how many pending work messages exist for a job.
func pendingWorkMessageCount(t *testing.T, ctx context.Context, runner any, repoID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND job_id = $2 AND kind = 'work' AND state = 'pending'`,
		repoID, jobID)
	if err != nil {
		t.Fatalf("count pending messages for %s: %v", jobID, err)
	}
	return intValue(row["n"])
}

func jobAttempt(t *testing.T, ctx context.Context, runner any, repoID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT attempt FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("select attempt for %s: %v", jobID, err)
	}
	return intValue(row["attempt"])
}

// TestRetryJobReopenReleasesLeaseAndAllowsFreshClaim is the RFC 0095 Phase 2
// (#65 P3) regression: re-opening a completed revision-cycle target through
// run.retry_job — even when the prior attempt still holds an active lease and a
// claimed/acked work message — must leave NO dangling active lease or stale
// message, so a fresh worker can claim the re-opened job without tripping
// `duplicate active job lease`.
func TestRetryJobReopenReleasesLeaseAndAllowsFreshClaim(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_reopen_retry_claim"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	runID, _, synthJobID, _, _ := revisionFixture(t, ctx, runner, repoID, cycles)

	// Pin a still-active lease + acked work message to the completed synth target,
	// modeling the #65 P3 wedge where a re-open left the prior attempt's lease
	// dangling. The synth target's role/lane must match a claimable worker.
	now := time.Now().UTC()
	staleLeaseID := "lease_stale_synth_" + repoID
	priorSessionID := "sess_synth_prior_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, priorSessionID, "synthesizer", "claude", []string{"synthesis"}, "active", 1)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '1 hour')`,
		repoID, staleLeaseID, runID, synthJobID, priorSessionID); err != nil {
		t.Fatalf("insert stale active lease: %v", err)
	}
	staleMsgID := "msg_stale_synth_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, current_lease_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'synthesizer','claude',$5,$6,$6)`,
		repoID, staleMsgID, runID, synthJobID, staleLeaseID, now); err != nil {
		t.Fatalf("insert stale acked message: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET current_lease_id = $3, current_message_id = $4, lane_selector_json = '{"lane_id":"claude"}'::jsonb
		 WHERE repository_id = $1 AND job_id = $2`,
		repoID, synthJobID, staleLeaseID, staleMsgID); err != nil {
		t.Fatalf("pin synth pointers: %v", err)
	}

	priorAttempt := jobAttempt(t, ctx, runner, repoID, synthJobID)

	// Re-open via run.retry_job (the operator F3 path).
	if _, err := HandleRunRetryJob(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"job_id": synthJobID,
	})); err != nil {
		t.Fatalf("retry_job re-open: %v", err)
	}

	// P1 invariants: no dangling active lease, exactly one fresh pending message,
	// the prior stale message retired, the attempt bumped.
	if n := activeLeaseCount(t, ctx, runner, repoID, synthJobID); n != 0 {
		t.Fatalf("active lease count after re-open = %d, want 0 (prior lease must be released)", n)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, synthJobID); n != 1 {
		t.Fatalf("pending message count after re-open = %d, want exactly 1", n)
	}
	staleState, err := oneRow(ctx, runner, `SELECT state FROM striatumd.queue_messages WHERE repository_id = $1 AND message_id = $2`, repoID, staleMsgID)
	if err != nil {
		t.Fatalf("select stale message state: %v", err)
	}
	if got := fmt.Sprint(staleState["state"]); got != "canceled" {
		t.Fatalf("stale message state = %q, want canceled", got)
	}
	if got := jobAttempt(t, ctx, runner, repoID, synthJobID); got != priorAttempt+1 {
		t.Fatalf("attempt = %d, want %d (bumped)", got, priorAttempt+1)
	}
	if got := jobState(t, ctx, runner, repoID, synthJobID); got != "queued" {
		t.Fatalf("synth state after re-open = %q, want queued", got)
	}

	// A fresh worker session in the target lane must claim the re-opened job
	// WITHOUT the re-claim failing the uq_active_resource_lease index.
	freshSessionID := "sess_synth_fresh_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSessionID, "synthesizer", "claude", []string{"synthesis"}, "active", 2)
	intgAttest(t, ctx, runner, repoID, runID, freshSessionID, "claude")
	res, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": freshSessionID,
	}))
	if err != nil {
		t.Fatalf("fresh claim of re-opened job must succeed, got: %v", err)
	}
	if fmt.Sprint(res["status"]) != "claimed" {
		t.Fatalf("fresh claim status = %v, want claimed; res=%#v", res["status"], res)
	}
	if got := fmt.Sprint(asMap(asMap(res["packet"])["job"])["job_id"]); got != synthJobID {
		t.Fatalf("fresh claim packet job_id = %v, want %s", got, synthJobID)
	}
	// Exactly one active lease now (the fresh one), proving no duplicate.
	if n := activeLeaseCount(t, ctx, runner, repoID, synthJobID); n != 1 {
		t.Fatalf("active lease count after fresh claim = %d, want 1", n)
	}
}

// TestCheckpointContinueReopenReleasesLease is the RFC 0095 Phase 2 companion
// covering the checkpoint.resolve continue re-open path: resolving a
// revision_routing human checkpoint with `continue` re-opens the review job
// atomically through the shared helper, leaving no dangling lease and a fresh
// pending message.
func TestCheckpointContinueReopenReleasesLease(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_reopen_checkpoint"
	// No cycle: a needs_revision verdict opens a human checkpoint.
	_, reviewJobID, _, sessionID, leaseID := revisionFixture(t, ctx, runner, repoID, []any{})

	res, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     reviewJobID,
		"lease_id":   leaseID,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record needs_revision: %v", err)
	}
	blockerID := fmt.Sprint(res["blocker_id"])
	if blockerID == "" || res["blocker_id"] == nil {
		t.Fatalf("expected a checkpoint blocker, got %#v", res)
	}
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "waiting_human" {
		t.Fatalf("review state = %q, want waiting_human", got)
	}
	priorAttempt := jobAttempt(t, ctx, runner, repoID, reviewJobID)

	// Resolve with continue -> the review job re-opens.
	if _, err := HandleCheckpointResolve(ctx, runner, intgEnv(repoID, map[string]any{
		"blocker_id": blockerID,
		"action":     "continue",
	})); err != nil {
		t.Fatalf("checkpoint continue: %v", err)
	}

	if n := activeLeaseCount(t, ctx, runner, repoID, reviewJobID); n != 0 {
		t.Fatalf("active lease count after continue = %d, want 0", n)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, reviewJobID); n != 1 {
		t.Fatalf("pending message count after continue = %d, want exactly 1", n)
	}
	if got := jobAttempt(t, ctx, runner, repoID, reviewJobID); got != priorAttempt+1 {
		t.Fatalf("attempt after continue = %d, want %d", got, priorAttempt+1)
	}
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "queued" {
		t.Fatalf("review state after continue = %q, want queued", got)
	}
}

// publishArtifactRow registers an artifact row directly (bypassing publishArtifact's
// disk checks) so a test can model a prior attempt's already-registered artifact.
// The row is recorded at the supplied attempt (RFC 0095 §1 / #84 attempt-scoped
// artifacts); a unique artifact_id per (logical_name, job, attempt) keeps the
// widened unique key happy when a test seeds several attempts.
func publishArtifactRow(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, sessionID, logicalName, kind, path, sha string, createdAt time.Time, attempt int) string {
	t.Helper()
	artifactID := fmt.Sprintf("art_%s_%s_a%d", logicalName, jobID, attempt)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at, attempt
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'create',$11,$12)`,
		repoID, artifactID, runID, jobID, sessionID, logicalName, kind, path, sha, 128, createdAt, attempt); err != nil {
		t.Fatalf("insert artifact row: %v", err)
	}
	return artifactID
}

// TestAutoFinalizeRefusesStaleAttemptArtifact is the RFC 0095 §1 / #84 (and the
// Phase 2 / #65 P2 created_at fallback) regression at the decision point:
// recovery auto-finalize must NOT re-complete a re-opened job from the PRIOR
// attempt's artifact. With the precise `attempt` column, a prior-attempt artifact
// does not even match the current-attempt lookup (status would_publish); the
// created_at boundary stays as defense-in-depth for a same-attempt row that
// predates the re-enqueue. A current-attempt artifact at/after the boundary is
// the fresh lane's own work and is accepted (status already_published).
func TestAutoFinalizeRefusesStaleAttemptArtifact(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_auto_finalize_stale"
	runID := "run_" + repoID
	jobID := "job_synth_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"synthesizer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "synth", "type": "synthesis", "role_id": "synthesizer"}},
	})
	now := time.Now().UTC().Truncate(time.Second)
	// The job is at attempt 2 (re-opened once).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at
		) VALUES ($1,$2,$3,'synth',2,'running','synthesizer','Synthesis','synthesis','idem_'||$1,'[]'::jsonb,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	sessionID := "sess_synth_" + repoID
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "synthesizer", "claude", []string{"synthesis"}, "active")

	logicalName := "synthesis_doc"
	path := "docs/synthesis.md"
	payload := []byte("synthesis-body")
	digest := sha256Hex(payload)
	priorArtifactCreatedAt := now.Add(-10 * time.Minute)
	// The PRIOR attempt's artifact (attempt 1), created before the current
	// attempt's re-enqueue boundary.
	publishArtifactRow(t, ctx, runner, repoID, runID, jobID, sessionID, logicalName, "synthesis", path, digest, priorArtifactCreatedAt, 1)

	statusFor := func(boundary any) map[string]any {
		t.Helper()
		status, err := autoFinalizeExistingArtifactStatus(ctx, runner, repoID, runID, jobID, logicalName, path, payload, boundary, 2)
		if err != nil {
			t.Fatalf("autoFinalizeExistingArtifactStatus: %v", err)
		}
		return status
	}

	// Only the attempt-1 artifact exists, but the job is at attempt 2: the
	// attempt-scoped lookup finds nothing, so auto-finalize reports would_publish
	// (it will NOT silently re-complete the re-opened job from the prior attempt).
	if got := fmt.Sprint(statusFor(now)["status"]); got != "would_publish" {
		t.Fatalf("status with only prior-attempt artifact = %q, want would_publish", got)
	}

	// Publish the fresh attempt-2 artifact (same content/path, attempt 2), created
	// at/after the boundary. Now the current attempt is satisfied and accepted.
	publishArtifactRow(t, ctx, runner, repoID, runID, jobID, sessionID, logicalName, "synthesis", path, digest, now, 2)
	if got := fmt.Sprint(statusFor(now.Add(-time.Second))["status"]); got != "already_published" {
		t.Fatalf("status with fresh attempt-2 artifact = %q, want already_published", got)
	}

	// Defense-in-depth: even at the current attempt, a row that predates the
	// re-enqueue boundary is refused as stale_attempt (the Phase 2 / #65 P2 guard
	// for the legacy/NULL-attempt path). Use a boundary AFTER the attempt-2 row.
	if got := fmt.Sprint(statusFor(now.Add(time.Minute))["status"]); got != "stale_attempt" {
		t.Fatalf("status with current-attempt artifact predating boundary = %q, want stale_attempt", got)
	}

	// A NULL boundary (no current message join) keeps the legacy behavior for the
	// matched current-attempt row.
	if got := fmt.Sprint(statusFor(nil)["status"]); got != "already_published" {
		t.Fatalf("status with NULL boundary = %q, want already_published (legacy)", got)
	}
}

// TestCycleScopedRepublishAvoidsLogicalNameCollision is the RFC 0095 Phase 2
// (#84, migration-free disposition) regression: a re-opened attempt that bumps
// the job `attempt` resolves its ${cycle}-templated expected artifact to a
// DISTINCT cycle_<attempt> logical_name + path, so republishing the revised
// artifact does not collide with the prior attempt's row on the
// (run_id, job_id, logical_name) unique constraint — the #84
// `artifact logical name already exists with different content` wedge. This is
// the migration-free half of #84; a workflow with a FIXED (non-${cycle})
// logical_name still collides and is deferred to the Phase 2.5 artifact-attempt
// schema column (the artifacts table is append-only and cannot widen its
// uniqueness key without a daemon-owned migration).
func TestCycleScopedRepublishAvoidsLogicalNameCollision(t *testing.T) {
	cycleArtifact := []any{
		map[string]any{
			"required":     true,
			"kind":         "collaboration_ledger",
			"logical_name": "ledger_${cycle}",
			"path":         "docs/ledgers/ledger_${cycle}.md",
		},
	}
	a1 := asMap(resolveExpectedArtifactCycles(cycleArtifact, 1)[0])
	a2 := asMap(resolveExpectedArtifactCycles(cycleArtifact, 2)[0])
	if a1["logical_name"] != "ledger_cycle_1" || a1["path"] != "docs/ledgers/ledger_cycle_1.md" {
		t.Fatalf("attempt 1 resolution = %#v, want cycle_1 logical_name + path", a1)
	}
	if a2["logical_name"] != "ledger_cycle_2" || a2["path"] != "docs/ledgers/ledger_cycle_2.md" {
		t.Fatalf("attempt 2 resolution = %#v, want cycle_2 logical_name + path", a2)
	}
	if a1["logical_name"] == a2["logical_name"] || a1["path"] == a2["path"] {
		t.Fatalf("re-opened attempt must get a distinct logical_name + path; got identical %#v / %#v", a1, a2)
	}

	// A fixed (non-${cycle}) artifact is unchanged across attempts: the same
	// logical_name on every attempt is the deferred #84 case that needs the
	// Phase 2.5 schema column.
	fixedArtifact := []any{
		map[string]any{"required": true, "kind": "synthesis", "logical_name": "synthesis", "path": "docs/synthesis.md"},
	}
	f1 := asMap(resolveExpectedArtifactCycles(fixedArtifact, 1)[0])
	f2 := asMap(resolveExpectedArtifactCycles(fixedArtifact, 2)[0])
	if f1["logical_name"] != f2["logical_name"] || f1["path"] != f2["path"] {
		t.Fatalf("fixed artifact must be unchanged across attempts; got %#v / %#v", f1, f2)
	}
}
