package mutations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestRunLockClaimVerdictSweepDeadlock is the RFC 0104 reproduction + regression.
//
// On ONE run it races, from a common start barrier and across many iterations:
//   - work.claim_next (HandleClaimNext), which locks sessions -> runs -> jobs;
//   - a review.verdict that COMPLETES the run (HandleRecordVerdict -> recordVerdict
//     -> maybeCompleteRun -> closeRemainingSessions), which locks jobs -> runs ->
//     sessions;
//   - a recovery SweepRun tick, the third concurrent party on the run's rows.
//
// The {sessions, runs} pair is acquired in OPPOSITE order by the claim and the
// verdict-completion paths, so before the per-run advisory lock (RFC 0104)
// Postgres resolves the cycle by aborting one transaction with SQLSTATE 40P01.
// The verdict path (HandleRecordVerdict) wraps in withTx with NO deadlock retry,
// so the abort surfaces RAW to the caller — a reliable RED before the fix. After
// lockRun is taken as the first statement of every per-run mutation tx, the cycle
// cannot form and the loop completes with NO deadlock surfaced to any caller.
//
// PG-gated: pgtest.Pool SKIPs when STRIATUM_PG_TEST_URL is unset.
func TestRunLockClaimVerdictSweepDeadlock(t *testing.T) {
	const (
		iterations = 80
		claimers   = 6
	)
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_runlock_deadlock"

	fx := seedRunLockRaceFixture(t, ctx, runner, repoID)

	for i := 0; i < iterations; i++ {
		resetRunLockRace(t, ctx, runner, fx)

		var wg sync.WaitGroup
		errs := make(chan error, claimers+2)
		start := make(chan struct{})

		// The verdict-completion half of the cycle: accept the running review job,
		// which completes the run (the upstream job is already completed) and runs
		// closeRemainingSessions -> sessions FOR UPDATE. No retry wrapper, so a
		// deadlock surfaces raw.
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
				"session_id": fx.reviewerSession,
				"job_id":     fx.reviewJob,
				"lease_id":   fx.reviewLease,
				"verdict":    "accept",
			})); err != nil {
				errs <- fmt.Errorf("verdict: %w", err)
			}
		}()

		// The claim half of the cycle: sibling sessions claiming on the same run
		// lock sessions -> runs. They find no claimable work (none is queued) but
		// still take both locks, which is the half of the cycle under test.
		for c := 0; c < claimers; c++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if _, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
					"session_id": fx.claimantSession,
				})); err != nil {
					errs <- fmt.Errorf("claim: %w", err)
				}
			}()
		}

		// The recovery sweep tick: a third concurrent party that locks
		// jobs/leases/sessions across the run. On a healthy fixture it takes no
		// recovery action, but it contends on the same rows.
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := SweepRun(ctx, runner, repoID, fx.runID, "runlock-deadlock-test"); err != nil {
				errs <- fmt.Errorf("sweep: %w", err)
			}
		}()

		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			msg := err.Error()
			if strings.Contains(msg, "40P01") ||
				strings.Contains(msg, "deadlock") ||
				strings.Contains(msg, "aborted by a database deadlock") {
				t.Fatalf("iteration %d: per-run deadlock surfaced — RFC 0104 lockRun is missing or ineffective: %v", i, err)
			}
			// Benign serialized outcome (and the proof the lock works): when the
			// verdict completes the run before a claimer runs, closeRemainingSessions
			// closes the run's active sessions, so a later claim on the now-closed
			// claimant session returns a clean "session is closed" invalid_transition
			// rather than deadlocking. Tolerate that ordering; fail on anything else.
			if strings.HasPrefix(msg, "claim: ") && strings.Contains(msg, "session is ") {
				continue
			}
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
	}
}

// runLockRaceFixture identifies the rows the deadlock race drives.
type runLockRaceFixture struct {
	repoID          string
	runID           string
	upstreamJob     string
	reviewJob       string
	reviewLease     string
	reviewerSession string // owns the review lease; records the completing verdict
	claimantSession string // races claim_next; closeRemainingSessions locks it
}

// seedRunLockRaceFixture seeds a run whose only non-terminal job is a running
// review job (lease held by the reviewer session) gated behind an already
// completed upstream job, plus a sibling claimant session with no lease. Recording
// an accept verdict on the review job therefore completes the whole run.
func seedRunLockRaceFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string) *runLockRaceFixture {
	t.Helper()
	fx := &runLockRaceFixture{
		repoID:          repoID,
		runID:           "run_" + repoID,
		upstreamJob:     "job_upstream",
		reviewJob:       "job_review",
		reviewLease:     "lease_review",
		reviewerSession: "sess_reviewer",
		claimantSession: "sess_claimant",
	}
	intgSeedRepo(t, ctx, runner, repoID)
	// The review job's workflow def omits require_attested_lane, so the verdict
	// path does not require an attached lane supervisor (kept hermetic).
	intgSeedRun(t, ctx, runner, repoID, fx.runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}, "builder": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
		"jobs": []any{
			map[string]any{"id": "upstream"},
			map[string]any{"id": "review_job"},
		},
	})
	intgSeedSession(t, ctx, runner, repoID, fx.runID, fx.reviewerSession, "reviewer", "claude", []string{"review"}, "active")
	intgSeedSession(t, ctx, runner, repoID, fx.runID, fx.claimantSession, "builder", "claude", nil, "active")
	resetRunLockRace(t, ctx, runner, fx)
	return fx
}

// resetRunLockRace restores the fixture to its pre-race state before each
// iteration: the run is running, the upstream job completed, the review job
// running with a fresh active lease owned by the reviewer, both sessions active,
// and the review job's prior verdict cleared (the (job, session) verdict
// uniqueness constraint would otherwise reject the next accept).
func resetRunLockRace(t *testing.T, ctx context.Context, runner db.Runner, fx *runLockRaceFixture) {
	t.Helper()
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	exec := func(what, sql string, args ...any) {
		if err := runner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("reset %s: %v", what, err)
		}
	}
	exec("run", `UPDATE striatumd.runs SET state='running', completed_at=NULL, stop_reason=NULL
		 WHERE repository_id=$1 AND run_id=$2`, fx.repoID, fx.runID)
	// Upstream job: completed terminal (so the review accept completes the run).
	exec("upstream upsert", `
		INSERT INTO striatumd.jobs (repository_id, job_id, run_id, workflow_job_id, title,
		  job_type, role_id, state, idempotency_key, created_at, completed_at)
		VALUES ($1,$2,$3,'upstream','Upstream','build','builder','completed','idem_'||$2,$4,$4)
		ON CONFLICT (repository_id, job_id) DO UPDATE
		  SET state='completed', completed_at=$4`, fx.repoID, fx.upstreamJob, fx.runID, now)
	// Review job: running, verdict-capable.
	exec("review upsert", `
		INSERT INTO striatumd.jobs (repository_id, job_id, run_id, workflow_job_id, title,
		  job_type, role_id, state, idempotency_key, created_at)
		VALUES ($1,$2,$3,'review_job','Review','review','reviewer','running','idem_'||$2,$4)
		ON CONFLICT (repository_id, job_id) DO UPDATE
		  SET state='running', completed_at=NULL, current_lease_id=NULL`, fx.repoID, fx.reviewJob, fx.runID, now)
	// Fresh active lease on the review job, owned by the reviewer session. Upsert
	// rather than delete+insert: once the run completes, events reference the lease
	// via events.lease_id, so the lease row cannot be deleted between iterations.
	exec("review lease", `
		INSERT INTO striatumd.leases (repository_id, lease_id, run_id, resource_type,
		  resource_id, owner_session_id, state, acquired_at, expires_at, last_heartbeat_at)
		VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7,$6)
		ON CONFLICT (repository_id, lease_id) DO UPDATE
		  SET state='active', expires_at=$7, last_heartbeat_at=$6,
		      released_at=NULL, release_reason=NULL`,
		fx.repoID, fx.reviewLease, fx.runID, fx.reviewJob, fx.reviewerSession, now, future)
	// Both sessions active (closeRemainingSessions closes them when the run completes).
	exec("sessions active", `UPDATE striatumd.sessions SET state='active', closed_at=NULL, close_reason=NULL
		 WHERE repository_id=$1 AND session_id = ANY($2)`,
		fx.repoID, []string{fx.reviewerSession, fx.claimantSession})
	resetRunLockClaimantBackend(t, ctx, runner, fx, now)
	// Clear the prior round's verdict so the next accept can be recorded.
	exec("clear verdicts", `DELETE FROM striatumd.verdicts WHERE repository_id=$1 AND job_id=$2`, fx.repoID, fx.reviewJob)
}

func resetRunLockClaimantBackend(t *testing.T, ctx context.Context, runner db.Runner, fx *runLockRaceFixture, now time.Time) {
	t.Helper()
	pid := os.Getpid()
	supervisorID := "sup_" + fx.claimantSession
	daemonSupervisorID := "dsup_" + fx.claimantSession
	exec := func(what, sql string, args ...any) {
		if err := runner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("reset claimant backend %s: %v", what, err)
		}
	}
	exec("process supervisor", `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, pid_start_time, state, started_at, heartbeat_at, ended_at, stop_reason
		) VALUES ($1,$2,$3,$4,'claude','[]'::jsonb,'/tmp','/tmp/scratch',$5,'','attached',$6,$6,NULL,NULL)
		ON CONFLICT (repository_id, supervisor_id) DO UPDATE
		  SET pid = EXCLUDED.pid,
		      pid_start_time = '',
		      state = 'attached',
		      heartbeat_at = EXCLUDED.heartbeat_at,
		      ended_at = NULL,
		      stop_reason = NULL`,
		fx.repoID, supervisorID, fx.runID, fx.claimantSession, pid, now)
	exec("pointer", `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'','attached',$7,'{}'::jsonb)
		ON CONFLICT (repository_id, supervisor_id) DO UPDATE
		  SET daemon_supervisor_id = EXCLUDED.daemon_supervisor_id,
		      pid = EXCLUDED.pid,
		      pid_start_time = '',
		      state = 'attached',
		      updated_at = EXCLUDED.updated_at,
		      metadata_json = EXCLUDED.metadata_json`,
		fx.repoID, supervisorID, daemonSupervisorID, fx.runID, fx.claimantSession, pid, now)
	exec("daemon supervisor", `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
		  daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
		  pid_start_time, state, started_at, heartbeat_at, ended_at, stop_reason
		) VALUES ($1,$2,$3,$4,$5,'inst','claude','[]'::jsonb,'sha','/tmp',$6,'','attached',$7,$7,NULL,NULL)
		ON CONFLICT (daemon_supervisor_id) DO UPDATE
		  SET pid = EXCLUDED.pid,
		      pid_start_time = '',
		      state = 'attached',
		      heartbeat_at = EXCLUDED.heartbeat_at,
		      ended_at = NULL,
		      stop_reason = NULL`,
		daemonSupervisorID, fx.repoID, fx.runID, fx.claimantSession, supervisorID, pid, now)
}
