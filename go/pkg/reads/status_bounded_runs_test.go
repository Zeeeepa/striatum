package reads

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestStatusBoundsRepoWideRunsAndExcludesTerminalWork is the #193 regression.
//
// Repo-wide status used to assemble EVERY historical run (a 353KB payload) and
// enumerate claimable/blocked jobs across all of them — including pending queue
// messages orphaned on canceled runs (run.cancel cancels the jobs but not the
// messages). This seeds one running run plus many terminal (canceled) runs, each
// carrying an orphan pending message and a blocked job, and asserts:
//
//   - the repo-wide default returns the running run + the most-recent N terminal
//     runs (bounded), NOT all of them;
//   - --all-runs returns every run (the legacy behavior, available on request);
//   - --run-limit overrides N;
//   - claimable_jobs / blocked_downstream_jobs exclude terminal runs entirely,
//     regardless of the recent-runs window.
func TestStatusBoundsRepoWideRunsAndExcludesTerminalWork(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_status_bounded"
	now := time.Now().UTC()

	crSeedRepo(t, ctx, runner, repoID, now)

	// One live (running) run with a genuinely-claimable pending message + a blocked
	// downstream job.
	liveRun := "run_live_" + repoID
	crSeedRunState(t, ctx, runner, repoID, liveRun, "main", now, "running")
	seedStatusClaimable(t, ctx, runner, repoID, liveRun, now)
	seedStatusBlocked(t, ctx, runner, repoID, liveRun, "blocked_live", now)

	// Many terminal (canceled) runs, each with an ORPHAN pending message and a
	// blocked job — the historical clutter #193 complains about.
	const terminalRuns = 30
	for i := 0; i < terminalRuns; i++ {
		runID := fmt.Sprintf("run_term_%02d_%s", i, repoID)
		// Stagger created_at so "most recent N" is well-defined.
		created := now.Add(time.Duration(i) * time.Minute)
		crSeedRunState(t, ctx, runner, repoID, runID, "main", created, "canceled")
		seedStatusClaimable(t, ctx, runner, repoID, runID, created)
		seedStatusBlocked(t, ctx, runner, repoID, runID, fmt.Sprintf("blocked_term_%02d", i), created)
	}

	repoWide := func(extra map[string]any) statusRunScope {
		p := map[string]any{"repository_id": repoID}
		for k, v := range extra {
			p[k] = v
		}
		return resolveStatusRunScope(rpc.Envelope{Params: p}, "")
	}

	// Default repo-wide: running + 20 most-recent terminal = 21.
	defScope := repoWide(nil)
	runs, err := statusRuns(ctx, runner, repoID, defScope)
	if err != nil {
		t.Fatalf("statusRuns default: %v", err)
	}
	wantDefault := 1 + defaultStatusTerminalRunLimit
	if len(runs) != wantDefault {
		t.Fatalf("default repo-wide runs = %d, want %d (running + %d recent terminal)", len(runs), wantDefault, defaultStatusTerminalRunLimit)
	}
	// The most recent terminal run (highest index) must be present; the oldest
	// must be dropped by the window.
	ids := runIDSet(runs)
	if !ids[fmt.Sprintf("run_term_%02d_%s", terminalRuns-1, repoID)] {
		t.Fatalf("default window must include the most recent terminal run")
	}
	if ids[fmt.Sprintf("run_term_%02d_%s", 0, repoID)] {
		t.Fatalf("default window must drop the oldest terminal run beyond the limit")
	}
	if !ids[liveRun] {
		t.Fatalf("default window must always include the running run")
	}

	// --all-runs: every run.
	allScope := repoWide(map[string]any{"all_runs": true})
	allRuns, err := statusRuns(ctx, runner, repoID, allScope)
	if err != nil {
		t.Fatalf("statusRuns all: %v", err)
	}
	if len(allRuns) != 1+terminalRuns {
		t.Fatalf("--all-runs runs = %d, want %d", len(allRuns), 1+terminalRuns)
	}

	// --run-limit overrides N.
	limScope := repoWide(map[string]any{"run_limit": 5})
	limRuns, err := statusRuns(ctx, runner, repoID, limScope)
	if err != nil {
		t.Fatalf("statusRuns limit: %v", err)
	}
	if len(limRuns) != 1+5 {
		t.Fatalf("--run-limit=5 runs = %d, want 6 (running + 5 terminal)", len(limRuns))
	}

	// claimable_jobs: only the live run's message, never a terminal run's orphan.
	claimable, err := statusClaimableJobs(ctx, runner, repoID, defScope)
	if err != nil {
		t.Fatalf("statusClaimableJobs: %v", err)
	}
	if len(claimable) != 1 {
		t.Fatalf("claimable jobs = %d, want 1 (only the live run; terminal-run orphans excluded)", len(claimable))
	}
	if got := fmt.Sprint(claimable[0]["run_id"]); got != liveRun {
		t.Fatalf("claimable run_id = %q, want the live run %q", got, liveRun)
	}

	// blocked_downstream_jobs: only the live run's blocked job.
	blocked, err := statusBlockedDownstreamJobs(ctx, runner, repoID, defScope)
	if err != nil {
		t.Fatalf("statusBlockedDownstreamJobs: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("blocked downstream jobs = %d, want 1 (only the live run; terminal runs excluded)", len(blocked))
	}
	if got := fmt.Sprint(blocked[0]["run_id"]); got != liveRun {
		t.Fatalf("blocked run_id = %q, want the live run %q", got, liveRun)
	}

	// Even with --all-runs, terminal-run claimable/blocked stay excluded for the
	// repo-wide enumeration (the override widens the run LISTING, not the
	// actionable-work surface), so the actionable set is unchanged.
	claimableAll, err := statusClaimableJobs(ctx, runner, repoID, allScope)
	if err != nil {
		t.Fatalf("statusClaimableJobs all: %v", err)
	}
	if len(claimableAll) != 1 {
		t.Fatalf("claimable jobs (--all-runs) = %d, want 1 (terminal-run orphans never actionable)", len(claimableAll))
	}
}

func runIDSet(rows []map[string]any) map[string]bool {
	set := map[string]bool{}
	for _, r := range rows {
		set[fmt.Sprint(r["run_id"])] = true
	}
	return set
}

// seedStatusClaimable inserts a queued job + a pending work message for a run, so
// the run has one claimable job in statusClaimableJobs.
func seedStatusClaimable(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string, now time.Time) {
	t.Helper()
	jobID := "cjob_" + runID
	msgID := "cmsg_" + runID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,'claimable',1,'queued','author','Build','draft',$4,
		         '{}'::jsonb,'[]'::jsonb,'{"lane_id":"claude"}'::jsonb,$5)`,
		repoID, jobID, runID, "idem_c_"+runID, now); err != nil {
		t.Fatalf("seed claimable job %s: %v", runID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,'author','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("seed pending message %s: %v", runID, err)
	}
}

// seedStatusBlocked inserts a blocked job for a run, so the run has one entry in
// statusBlockedDownstreamJobs.
func seedStatusBlocked(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, label string, now time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,$4,1,'blocked','author','Build','draft',$5,
		         '{}'::jsonb,'[]'::jsonb,'{"lane_id":"claude"}'::jsonb,$6)`,
		repoID, "bjob_"+runID, runID, label, "idem_b_"+runID, now); err != nil {
		t.Fatalf("seed blocked job %s: %v", runID, err)
	}
}
