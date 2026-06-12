package adapterconformance

// RFC 0108 Phase 1 — the multi-RUN composition gate. Where
// lifecycle_revision_test.go proves the FULL multi-LANE revision lifecycle
// self-recovers on one run, and run_lock_deadlock_test.go proves the claim /
// verdict / sweep cycle on one run does not deadlock (RFC 0104), this file
// proves the next axis: 2+ INDEPENDENT runs driven CONCURRENTLY on ONE
// repository compose without deadlock or corruption.
//
// RFC 0108 §"three real walls" #1: every mutation's appendEvent locks the
// singleton per-repository repo_event_chain_heads row FOR UPDATE, so ALL runs
// on a repo serialize at that one row — a brief row lock that yields one total,
// tamper-evident order of everything that happened on the repo. RFC 0104's
// per-(repo,run) advisory lock serializes WITHIN a run but never across runs.
// This gate proves those two compose: N concurrent run lifecycles
//
//   - all reach `completed` (the substrate carries each independent run home),
//   - surface NO 40P01 / deadlock to any driver (RFC 0104 + the single shared
//     chain-head lock cannot form a cross-run cycle), and
//   - leave the per-repo event hash chain LINEAR + VERIFYING (the head FOR
//     UPDATE keeps concurrent appenders from forking the chain — the
//     `daemon doctor` / archive deep-chain invariant).
//
// It turns RED exactly when the substrate it characterizes breaks: revert RFC
// 0104's lockRun and the intra-run claim/verdict cycle resurfaces as 40P01;
// drop the repo_event_chain_heads FOR UPDATE and concurrent appenders fork the
// chain so assertEventChainLinear fails.
//
// The branch + per-job worktree isolation half (RFC 0008 substrate) proves two
// concurrent repo-write runs each get their OWN confirmed branch and their OWN
// detached worktree (no shared checkout), and that DELIBERATELY removing
// isolation (worktree_isolation off) turns it red: the substrate refuses to
// hand a repo-write lane an isolated checkout, surfacing the shared-checkout
// hazard up front rather than letting two runs silently scribble one tree.
//
// PG-gated exactly like the rest of the conformance suite (NewHarness ->
// pgtest.Pool SKIPs without STRIATUM_PG_TEST_URL; each test gets a fresh
// database, so the event chain is isolated and contiguous per test).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// mrRun identifies the rows one concurrently-driven run owns. Every run lives on
// ONE shared repository but has its own run_id, branch, jobs, and sessions.
type mrRun struct {
	repoID        string
	runID         string
	branch        string
	implJob       string
	reviewJob     string
	implSession   string
	reviewSession string
}

const (
	mrImplementerRole = "implementer"
	mrImplementerLane = "claude_code"
	mrReviewerRole    = "reviewer"
	mrReviewerLane    = "reviewer_lane"
)

// --- Half A: concurrent run-lifecycle composition --------------------------

// TestMultiRunConcurrentComposeNoDeadlock is the RFC 0108 Phase 1 headline gate:
// across several iterations it seeds a batch of fresh independent runs on ONE
// repository and drives every one of their full review-gated lifecycles
// (implement -> complete -> review -> accept -> completed) CONCURRENTLY from a
// common start barrier. The assertions are the Phase 1 acceptance criteria:
// every run reaches `completed`, no driver surfaces a 40P01/deadlock, and the
// per-repo event chain stays linear + verifying as it grows under sustained
// concurrent appends.
func TestMultiRunConcurrentComposeNoDeadlock(t *testing.T) {
	const (
		iterations     = 4
		concurrentRuns = 4
	)
	repoID := "repo_multirun_compose"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	// Establish the repo's event-chain genesis with ONE serial warmup run before
	// any concurrent claiming — mirroring production, where repo registration +
	// run prepare/start append the first events serially through handlers long
	// before two runs ever claim at once. The per-repo repo_event_chain_heads FOR
	// UPDATE serializes appenders by locking that singleton row; a SELECT ... FOR
	// UPDATE over ZERO rows locks nothing, so genesis must be serial (it always is
	// in production). Past genesis, every concurrent appender serializes on the
	// head row, which is the linearity this gate proves.
	completed := 0
	warmup := mrSeedHappyRun(t, ctx, h, repoID, "warmup")
	if err := driveHappyRun(ctx, h, warmup); err != nil {
		t.Fatalf("warmup run: %v", err)
	}
	if got := chaosRunState(t, ctx, h, warmup.runID); got != "completed" {
		t.Fatalf("warmup run state = %q, want completed", got)
	}
	completed++

	for iter := 0; iter < iterations; iter++ {
		runs := make([]*mrRun, concurrentRuns)
		for i := range runs {
			runs[i] = mrSeedHappyRun(t, ctx, h, repoID, fmt.Sprintf("%d_%d", iter, i))
		}

		var wg sync.WaitGroup
		errs := make(chan error, concurrentRuns)
		start := make(chan struct{})
		for _, r := range runs {
			wg.Add(1)
			go func(r *mrRun) {
				defer wg.Done()
				<-start
				if err := driveHappyRun(ctx, h, r); err != nil {
					errs <- fmt.Errorf("run %s: %w", r.runID, err)
				}
			}(r)
		}
		close(start)
		wg.Wait()
		close(errs)

		for err := range errs {
			if isConcurrencyDeadlock(err) {
				t.Fatalf("iteration %d: cross-run deadlock surfaced — RFC 0104 lockRun or the repo_event_chain_heads serialization is missing/ineffective: %v", iter, err)
			}
			t.Fatalf("iteration %d: concurrent run drive failed: %v", iter, err)
		}

		// Every independent run carried itself home with no operator action.
		for _, r := range runs {
			if got := chaosRunState(t, ctx, h, r.runID); got != "completed" {
				t.Fatalf("iteration %d: run %s state = %q, want completed (a concurrent sibling starved it)", iter, r.runID, got)
			}
			completed++
		}

		// Distinct working branches per run (no shared branch across runs).
		assertDistinctBranches(t, ctx, h, runs)
	}

	if want := iterations*concurrentRuns + 1; completed != want {
		t.Fatalf("completed %d runs, want %d (incl. warmup)", completed, want)
	}

	// The single per-repo event chain stayed one linear, verifying chain across
	// every concurrent appender from every run.
	assertEventChainLinear(t, ctx, h, repoID)
}

// mrSeedHappyRun seeds one independent two-lane, review-gated run on an
// already-seeded shared repository: a document_only implement job (queued,
// non-required artifact so completion needs no publish) and a review job
// (blocked, depends on implement) on its own branch + own workflow snapshot,
// plus the implementer and reviewer sessions the driver claims with. suffix
// makes every id unique within the repo.
func mrSeedHappyRun(t *testing.T, ctx context.Context, h *Harness, repoID, suffix string) *mrRun {
	t.Helper()
	now := time.Now().UTC()
	r := &mrRun{
		repoID:        repoID,
		runID:         "run_mrc_" + suffix,
		branch:        fmt.Sprintf("striatum/mrc-%s", suffix),
		implJob:       "job_impl_" + suffix,
		reviewJob:     "job_review_" + suffix,
		implSession:   "sess_impl_" + suffix,
		reviewSession: "sess_review_" + suffix,
	}

	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles": map[string]any{
			mrImplementerRole: map[string]any{"summary": "implement a small change"},
			mrReviewerRole:    map[string]any{"summary": "review the change"},
		},
		"lanes": map[string]any{
			mrImplementerLane: map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}},
			mrReviewerLane:    map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "review"}},
		},
		"jobs": []any{
			map[string]any{"id": fixtureWorkflowJob, "interrogable": false},
			map[string]any{"id": "review_job", "type": "review", "role_id": mrReviewerRole},
		},
	}
	mrSeedRunRow(t, ctx, h.Runner, repoID, r.runID, r.branch, false, workflow)

	writeScope := map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}}
	expectedArtifacts := []any{map[string]any{
		"logical_name": "note", "kind": "progress_note", "path": "docs/seed.md", "required": false,
	}}
	seedFixtureJob(t, ctx, h.Runner, repoID, r.runID, r.implJob, mrImplementerRole, mrImplementerLane, writeScope, expectedArtifacts, now)

	mrSeedReviewJob(t, ctx, h.Runner, repoID, r.runID, r.reviewJob, r.implJob)

	seedFixtureSession(t, ctx, h.Runner, repoID, r.runID, r.implSession, mrImplementerRole, mrImplementerLane, []string{"claim", "write"}, 1)
	seedFixtureSession(t, ctx, h.Runner, repoID, r.runID, r.reviewSession, mrReviewerRole, mrReviewerLane, []string{"claim", "review"}, 1)
	return r
}

// driveHappyRun drives one run's full lifecycle through the PRODUCTION mutation
// handlers and returns an error (goroutine-safe — no t.Fatalf) so it can race
// sibling runs. implement: claim -> ack -> complete -> close; review: claim ->
// ack -> accept verdict, which must complete the run.
func driveHappyRun(ctx context.Context, h *Harness, r *mrRun) error {
	implLease, err := mrClaimAck(ctx, h, r.repoID, r.implSession)
	if err != nil {
		return fmt.Errorf("implementer claim/ack: %w", err)
	}
	if _, err := mutations.HandleCompleteWork(ctx, h.Runner, mrEnv(r.repoID, "work.complete", map[string]any{
		"session_id": r.implSession, "job_id": r.implJob, "lease_id": implLease, "summary": "implemented",
	})); err != nil {
		return fmt.Errorf("implementer complete: %w", err)
	}
	if _, err := mutations.HandleCloseSession(ctx, h.Runner, mrEnv(r.repoID, "session.close", map[string]any{
		"session_id": r.implSession, "reason": "lane_finished_job",
	})); err != nil {
		return fmt.Errorf("implementer close: %w", err)
	}

	reviewLease, err := mrClaimAck(ctx, h, r.repoID, r.reviewSession)
	if err != nil {
		return fmt.Errorf("reviewer claim/ack: %w", err)
	}
	result, err := mutations.HandleRecordVerdict(ctx, h.Runner, mrEnv(r.repoID, "review.verdict", map[string]any{
		"session_id": r.reviewSession, "job_id": r.reviewJob, "lease_id": reviewLease, "verdict": "accept",
	}))
	if err != nil {
		return fmt.Errorf("reviewer verdict: %w", err)
	}
	if got := fmt.Sprint(result["status"]); got != "completed" {
		return fmt.Errorf("accept status = %q, want completed", got)
	}
	return nil
}

// --- Half B: branch + per-job worktree isolation ---------------------------

// TestMultiRunPerJobWorktreesComposeIsolated proves the RFC 0008 worktree
// substrate composes across concurrent runs: two repo-write runs on ONE real
// git repo, each on its own confirmed branch with worktree_isolation: per_job,
// concurrently create their per-job worktrees — and each gets its OWN detached
// checkout (distinct path, real on-disk directory), never a shared tree, with
// no 40P01.
func TestMultiRunPerJobWorktreesComposeIsolated(t *testing.T) {
	repoID := "repo_multirun_worktrees"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)
	writeRepoFile(t, repoRoot, "docs/seed.md", fixtureArtifactBody())
	gitCommitAll(t, repoRoot, "seed")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	runA := mrSeedRepoWriteRun(t, ctx, h, repoID, repoRoot, "run_mrw_a", "striatum/mrw-a", "per_job")
	runB := mrSeedRepoWriteRun(t, ctx, h, repoID, repoRoot, "run_mrw_b", "striatum/mrw-b", "per_job")

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	paths := make([]string, 2)
	start := make(chan struct{})
	for i, r := range []*mrRun{runA, runB} {
		wg.Add(1)
		go func(i int, r *mrRun) {
			defer wg.Done()
			<-start
			path, err := mrCreateWorktree(ctx, h, r)
			if err != nil {
				errs <- fmt.Errorf("run %s worktree: %w", r.runID, err)
				return
			}
			paths[i] = path
		}(i, r)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if isConcurrencyDeadlock(err) {
			t.Fatalf("concurrent worktree create deadlocked: %v", err)
		}
		t.Fatalf("concurrent worktree create failed: %v", err)
	}

	if paths[0] == "" || paths[1] == "" {
		t.Fatalf("missing worktree paths: %#v", paths)
	}
	if paths[0] == paths[1] {
		t.Fatalf("both runs got the SAME worktree path %q — runs are sharing one checkout", paths[0])
	}
	for _, p := range paths {
		abs := filepath.Join(repoRoot, p)
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			t.Fatalf("worktree path %q is not a real on-disk directory: stat err=%v", p, err)
		}
	}

	rows := chaosQuery(t, ctx, h, `
		SELECT job_id, worktree_path FROM striatumd.job_worktrees
		 WHERE repository_id=$1 AND state='active' ORDER BY worktree_path`, repoID)
	if len(rows) != 2 {
		t.Fatalf("active job_worktrees = %d, want 2 (one isolated checkout per concurrent run)", len(rows))
	}
	if fmt.Sprint(rows[0]["worktree_path"]) == fmt.Sprint(rows[1]["worktree_path"]) {
		t.Fatalf("two active worktrees share a path: %v", rows)
	}
}

// TestMultiRunSharedCheckoutTurnsRed is the RFC 0108 Phase 1 "a deliberately
// induced shared checkout (isolation off) turns it red" boundary. With
// worktree_isolation OFF on a repo-write lane, the substrate REFUSES to hand the
// run an isolated worktree (so two concurrent runs could only share the main
// checkout). The gate asserts that refusal is surfaced up front — the
// shared-checkout hazard is detected, not silently allowed — which is exactly
// the precondition Phase 2 promotes to isolation-by-default under concurrency.
func TestMultiRunSharedCheckoutTurnsRed(t *testing.T) {
	repoID := "repo_multirun_sharedcheckout"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)
	writeRepoFile(t, repoRoot, "docs/seed.md", fixtureArtifactBody())
	gitCommitAll(t, repoRoot, "seed")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	// isolation "" => the lane declares no worktree_isolation: per_job.
	r := mrSeedRepoWriteRun(t, ctx, h, repoID, repoRoot, "run_mrs", "striatum/mrs", "")

	_, err := mrCreateWorktree(ctx, h, r)
	if err == nil {
		t.Fatal("worktree create SUCCEEDED for a lane without worktree_isolation: per_job — a shared-checkout run was silently allowed (RFC 0108 Phase 1 boundary)")
	}
	if !strings.Contains(err.Error(), "worktree_isolation: per_job") {
		t.Fatalf("shared-checkout refusal = %v, want the worktree_isolation: per_job precondition", err)
	}
}

// mrSeedRepoWriteRun seeds one repo-write run on an already-seeded real-git repo:
// a real working branch, a confirmed-branch run row, a repo_write implement job
// with worktree_isolation set per `isolation` ("" leaves it off), and the
// implementer session. The run's branch is a real ref so `git worktree add
// --detach <target> <branch>` succeeds.
func mrSeedRepoWriteRun(t *testing.T, ctx context.Context, h *Harness, repoID, repoRoot, runID, branch, isolation string) *mrRun {
	t.Helper()
	now := time.Now().UTC()
	runGit(t, repoRoot, "branch", branch)

	r := &mrRun{
		repoID:      repoID,
		runID:       runID,
		branch:      branch,
		implJob:     "job_" + runID,
		implSession: "sess_" + runID,
	}

	lane := map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
	if isolation != "" {
		lane["worktree_isolation"] = isolation
	}
	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles":       map[string]any{mrImplementerRole: map[string]any{"summary": "implement"}},
		"lanes":       map[string]any{mrImplementerLane: lane},
		"jobs":        []any{map[string]any{"id": fixtureWorkflowJob, "interrogable": false}},
	}
	mrSeedRunRow(t, ctx, h.Runner, repoID, runID, branch, true, workflow)

	writeScope := map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"docs"}}
	expectedArtifacts := []any{map[string]any{
		"logical_name": "note", "kind": "progress_note", "path": "docs/seed.md", "required": false,
	}}
	seedFixtureJob(t, ctx, h.Runner, repoID, runID, r.implJob, mrImplementerRole, mrImplementerLane, writeScope, expectedArtifacts, now)
	seedFixtureSession(t, ctx, h.Runner, repoID, runID, r.implSession, mrImplementerRole, mrImplementerLane, []string{"claim", "write"}, 1)
	return r
}

// mrCreateWorktree claims + acks the run's implement job, then creates its
// per-job worktree through the production handler, returning the worktree path.
// Goroutine-safe (returns errors).
func mrCreateWorktree(ctx context.Context, h *Harness, r *mrRun) (string, error) {
	lease, err := mrClaimAck(ctx, h, r.repoID, r.implSession)
	if err != nil {
		return "", fmt.Errorf("claim/ack: %w", err)
	}
	res, err := mutations.HandleWorktreeCreate(ctx, h.Runner, mrEnv(r.repoID, "work.worktree_create", map[string]any{
		"session_id": r.implSession, "job_id": r.implJob, "lease_id": lease,
	}))
	if err != nil {
		return "", err
	}
	return fmt.Sprint(res["worktree_path"]), nil
}

// --- shared helpers --------------------------------------------------------

// mrEnv builds a production-shaped envelope for a mutation handler call.
func mrEnv(repoID, method string, params map[string]any) rpc.Envelope {
	params["repository_id"] = repoID
	return rpc.Envelope{SchemaVersion: rpc.SupportedEnvelopeVersion, Method: method, Params: params}
}

// mrClaimAck claims the one queued job a session is eligible for and acks it,
// returning the lease id. Goroutine-safe (returns errors, never t.Fatalf, and
// never panics on a non-claimed result).
func mrClaimAck(ctx context.Context, h *Harness, repoID, sessionID string) (string, error) {
	claim, err := mutations.HandleClaimNext(ctx, h.Runner, mrEnv(repoID, "work.claim_next", map[string]any{
		"session_id": sessionID,
	}))
	if err != nil {
		return "", err
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		return "", fmt.Errorf("claim status = %v, want claimed", claim["status"])
	}
	packet, ok := claim["packet"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("claim returned no packet")
	}
	lease, ok := packet["lease"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("claim packet has no lease")
	}
	leaseID := fmt.Sprint(lease["lease_id"])
	messageID := fmt.Sprint(lease["message_id"])
	if _, err := mutations.HandleAckWork(ctx, h.Runner, mrEnv(repoID, "work.ack", map[string]any{
		"session_id": sessionID, "message_id": messageID, "lease_id": leaseID,
	})); err != nil {
		return "", err
	}
	return leaseID, nil
}

// mrSeedRunRow seeds one run's workflow snapshot + run row on an already-seeded
// repository. content_sha256 is unique per run so two runs on one repo never
// collide on the snapshot. branch_name is always set; branch_confirmed_at is set
// when confirmed (required before worktree create).
func mrSeedRunRow(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, branch string, confirmed bool, workflow map[string]any) {
	t.Helper()
	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatalf("encode workflow: %v", err)
	}
	snapID := "snap_" + runID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,$4,$5::jsonb,$6)`,
		repoID, snapID, fixtureWorkflowID, "sha_"+runID, workflowArg, now); err != nil {
		t.Fatalf("seed snapshot %s: %v", runID, err)
	}
	var confirmedAt any
	if confirmed {
		confirmedAt = now
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  branch_name, branch_confirmed_at
		) VALUES ($1,$2,$3,(SELECT repo_root FROM striatumd.repositories WHERE repository_id=$1),'running',$4,$5,$6)`,
		repoID, runID, snapID, now, branch, confirmedAt); err != nil {
		t.Fatalf("seed run %s: %v", runID, err)
	}
}

// mrSeedReviewJob seeds a blocked, verdict-capable review job that depends on
// the implement job, so the implementer's work.complete -> maybeEnqueueDownstream
// enqueues it for the reviewer.
func mrSeedReviewJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, reviewJobID, implJobID string) {
	t.Helper()
	now := time.Now().UTC()
	laneSel, err := db.JSONBArg(runner, map[string]any{"lane_id": mrReviewerLane})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,'review_job',1,'blocked',$4,'Review','review','idem_'||$2,'{}'::jsonb,'[]'::jsonb,$5::jsonb,$6)`,
		repoID, reviewJobID, runID, mrReviewerRole, laneSel, now); err != nil {
		t.Fatalf("seed review job %s: %v", reviewJobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewJobID, implJobID); err != nil {
		t.Fatalf("seed review dependency %s: %v", reviewJobID, err)
	}
}

// mrSeedRepoRow seeds the shared repository row once for a multi-run repo.
func mrSeedRepoRow(t *testing.T, ctx context.Context, runner db.Runner, repoID, repoRoot string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'multirun',$5,16,'active')`,
		repoID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
}

// assertDistinctBranches asserts every run carries a distinct, non-empty working
// branch (no two concurrent runs share a branch).
func assertDistinctBranches(t *testing.T, ctx context.Context, h *Harness, runs []*mrRun) {
	t.Helper()
	seen := map[string]string{}
	for _, r := range runs {
		rows := chaosQuery(t, ctx, h, `SELECT branch_name FROM striatumd.runs WHERE repository_id=$1 AND run_id=$2`,
			r.repoID, r.runID)
		if len(rows) != 1 {
			t.Fatalf("run %s: expected 1 row, got %d", r.runID, len(rows))
		}
		branch := fmt.Sprint(rows[0]["branch_name"])
		if branch == "" || branch == "<nil>" {
			t.Fatalf("run %s has no working branch", r.runID)
		}
		if prior, ok := seen[branch]; ok {
			t.Fatalf("runs %s and %s share branch %q (no branch-per-run isolation)", prior, r.runID, branch)
		}
		seen[branch] = r.runID
	}
}

// assertEventChainLinear verifies the per-repository event hash chain is a single
// linear, verifying chain: ordered by event_id, each row's previous_hash links to
// exactly the prior row's row_hash (the first is NULL), and the singleton
// repo_event_chain_heads row points at the true tail. A concurrent appender race
// on the head (e.g. a dropped FOR UPDATE) forks the chain and breaks the linkage.
func assertEventChainLinear(t *testing.T, ctx context.Context, h *Harness, repoID string) {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT event_id, previous_hash, row_hash
		  FROM striatumd.events WHERE repository_id=$1 ORDER BY event_id`, repoID)
	if len(rows) == 0 {
		t.Fatal("event chain is empty — concurrent runs appended no events")
	}
	var prevHash any
	for i, row := range rows {
		if fmt.Sprint(row["previous_hash"]) != fmt.Sprint(prevHash) {
			t.Fatalf("event chain broken at index %d (event_id=%v): previous_hash=%v, want %v (the chain forked under concurrency)",
				i, row["event_id"], row["previous_hash"], prevHash)
		}
		if rh := fmt.Sprint(row["row_hash"]); rh == "" || rh == "<nil>" {
			t.Fatalf("event %v has empty row_hash", row["event_id"])
		}
		prevHash = row["row_hash"]
	}
	last := rows[len(rows)-1]
	head := chaosQuery(t, ctx, h, `
		SELECT last_event_id, last_hash FROM striatumd.repo_event_chain_heads WHERE repository_id=$1`, repoID)
	if len(head) != 1 {
		t.Fatalf("expected 1 chain-head row, got %d", len(head))
	}
	if fmt.Sprint(head[0]["last_hash"]) != fmt.Sprint(last["row_hash"]) {
		t.Fatalf("chain head last_hash=%v does not point at the chain tail row_hash=%v", head[0]["last_hash"], last["row_hash"])
	}
	if fmt.Sprint(head[0]["last_event_id"]) != fmt.Sprint(last["event_id"]) {
		t.Fatalf("chain head last_event_id=%v != tail event_id=%v", head[0]["last_event_id"], last["event_id"])
	}
}

// --- Phase 2: isolation by default under concurrency -----------------------

// TestMultiRunIsolationRequiredWhenSiblingActive is the RFC 0108 Phase 2 gate:
// once a run is active on a repo, a SECOND run that would write the SHARED main
// checkout (a repo-write job on a lane without worktree_isolation: per_job) is
// refused at run.start — the repo_write_without_worktree_isolation lint warning
// promoted to an enforced precondition, so no two concurrent runs ever scribble
// one working tree. A run whose repo-write work is per_job-isolated (its own
// worktree) or that does not write the repo starts freely beside the active
// sibling, and the single-run case is untouched.
func TestMultiRunIsolationRequiredWhenSiblingActive(t *testing.T) {
	repoID := "repo_multirun_p2_isolation"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	// First repo-write run starts ALONE with isolation off: no sibling is active,
	// so the single-run main checkout is its to use — allowed (the common case is
	// untouched).
	runA := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p2_a", "striatum/p2-a", "")
	if err := mrStartRun(ctx, h, runA); err != nil {
		t.Fatalf("first unisolated repo-write run refused with no sibling active: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runA.runID); got != "running" {
		t.Fatalf("run A state = %q, want running", got)
	}

	// A SECOND unisolated repo-write run, started while A is active, would share
	// the main checkout — refused with the Phase 2 precondition code.
	runB := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p2_b", "striatum/p2-b", "")
	err := mrStartRun(ctx, h, runB)
	if err == nil {
		t.Fatal("second unisolated repo-write run started while a sibling run is active — two runs share the main checkout (RFC 0108 Phase 2 boundary)")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "concurrent_run_isolation_required" {
		t.Fatalf("refusal = %v, want code concurrent_run_isolation_required", err)
	}
	if got := chaosRunState(t, ctx, h, runB.runID); got != "ready" {
		t.Fatalf("refused run B state = %q, want ready (a refused start must not advance the run)", got)
	}

	// A per-job-isolated repo-write run starts fine beside the active sibling: its
	// repo-write work lives in its OWN detached worktree, never the shared checkout.
	runC := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p2_c", "striatum/p2-c", "per_job")
	if err := mrStartRun(ctx, h, runC); err != nil {
		t.Fatalf("per_job-isolated repo-write run refused beside an active sibling: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runC.runID); got != "running" {
		t.Fatalf("run C state = %q, want running", got)
	}

	// A document-only run (no repo-write job) also starts fine — the precondition
	// guards only repo-write work that would touch the shared tree.
	runD := mrSeedReadyDocumentRun(t, ctx, h, repoID, "run_p2_d", "striatum/p2-d")
	if err := mrStartRun(ctx, h, runD); err != nil {
		t.Fatalf("document-only run refused beside an active sibling: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runD.runID); got != "running" {
		t.Fatalf("run D state = %q, want running", got)
	}
}

func TestRunStartRefusesAutonomousRepoWriteSharedCheckout(t *testing.T) {
	repoID := "repo_multirun_p2_autonomous_isolation"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	runA := mrSeedReadyAutonomousRepoWriteRun(t, ctx, h, repoID, "run_auto_a", "striatum/auto-a", "", false)
	err := mrStartRun(ctx, h, runA)
	if err == nil {
		t.Fatal("agent-loop repo-write run started in the shared checkout without the compatibility override")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "autonomous_worktree_isolation_required" {
		t.Fatalf("refusal = %v, want code autonomous_worktree_isolation_required", err)
	}
	if got := chaosRunState(t, ctx, h, runA.runID); got != "ready" {
		t.Fatalf("refused run A state = %q, want ready", got)
	}

	runB := mrSeedReadyAutonomousRepoWriteRun(t, ctx, h, repoID, "run_auto_b", "striatum/auto-b", "", true)
	if err := mrStartRun(ctx, h, runB); err != nil {
		t.Fatalf("shared-checkout compatibility override was refused: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runB.runID); got != "running" {
		t.Fatalf("run B state = %q, want running", got)
	}

	runC := mrSeedReadyAutonomousRepoWriteRun(t, ctx, h, repoID, "run_auto_c", "striatum/auto-c", "per_job", false)
	if err := mrStartRun(ctx, h, runC); err != nil {
		t.Fatalf("per-job isolated agent-loop repo-write run was refused: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runC.runID); got != "running" {
		t.Fatalf("run C state = %q, want running", got)
	}
}

// TestMultiRunConcurrentUnisolatedStartsResolveToOne proves the Phase 2
// precondition is race-free: when several unisolated repo-write runs call
// run.start CONCURRENTLY on one repo, EXACTLY ONE wins the shared main checkout
// and every other is refused with concurrent_run_isolation_required — never two
// both reaching `running` (which would share the checkout). The per-repo
// run.start serialization (lockRepo) closes the check-then-act window; no driver
// surfaces a 40P01, and the audit chain stays linear under the concurrent starts.
func TestMultiRunConcurrentUnisolatedStartsResolveToOne(t *testing.T) {
	const concurrentRuns = 6
	repoID := "repo_multirun_p2_race"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	runs := make([]*mrRun, concurrentRuns)
	for i := range runs {
		runs[i] = mrSeedReadyRepoWriteRun(t, ctx, h, repoID, fmt.Sprintf("run_p2race_%d", i), fmt.Sprintf("striatum/p2race-%d", i), "")
	}

	var wg sync.WaitGroup
	results := make([]error, concurrentRuns)
	start := make(chan struct{})
	for i, r := range runs {
		wg.Add(1)
		go func(i int, r *mrRun) {
			defer wg.Done()
			<-start
			results[i] = mrStartRun(ctx, h, r)
		}(i, r)
	}
	close(start)
	wg.Wait()

	started, refused := 0, 0
	for i, err := range results {
		if err == nil {
			started++
			continue
		}
		if isConcurrencyDeadlock(err) {
			t.Fatalf("run %s start surfaced a deadlock: %v", runs[i].runID, err)
		}
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != "concurrent_run_isolation_required" {
			t.Fatalf("run %s refusal = %v, want concurrent_run_isolation_required", runs[i].runID, err)
		}
		refused++
	}
	if started != 1 {
		t.Fatalf("%d unisolated repo-write runs reached running concurrently, want exactly 1 (the rest must be refused so no two share the main checkout)", started)
	}
	if refused != concurrentRuns-1 {
		t.Fatalf("refused %d runs, want %d", refused, concurrentRuns-1)
	}

	runningCount := 0
	for _, r := range runs {
		if chaosRunState(t, ctx, h, r.runID) == "running" {
			runningCount++
		}
	}
	if runningCount != 1 {
		t.Fatalf("%d runs in running state, want 1", runningCount)
	}
	assertEventChainLinear(t, ctx, h, repoID)
}

// mrSeedReadyRepoWriteRun seeds a repo-write run in the READY state (branch
// confirmed, not yet started) whose lane carries worktree_isolation per
// `isolation` ("" leaves it off).
func mrSeedReadyRepoWriteRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch, isolation string) *mrRun {
	t.Helper()
	lane := map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
	if isolation != "" {
		lane["worktree_isolation"] = isolation
	}
	return mrSeedReadyRunWithLane(t, ctx, h, repoID, runID, branch, lane,
		map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"docs"}})
}

func mrSeedReadyAutonomousRepoWriteRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch, isolation string, override bool) *mrRun {
	t.Helper()
	lane := map[string]any{
		"display_model":        "Codex",
		"capabilities":         []any{"claim", "write"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	if isolation != "" {
		lane["worktree_isolation"] = isolation
	}
	if override {
		lane["allow_shared_checkout_repo_write"] = true
		lane["shared_checkout_repo_write_rationale"] = "interactive-human compatibility fixture"
	}
	return mrSeedReadyRunWithLane(t, ctx, h, repoID, runID, branch, lane,
		map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"docs"}})
}

// mrSeedReadyDocumentRun seeds a document_only run in the READY state (no
// repo-write job, so the Phase 2 isolation precondition never bites it).
func mrSeedReadyDocumentRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch string) *mrRun {
	t.Helper()
	return mrSeedReadyRun(t, ctx, h, repoID, runID, branch, "",
		map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}})
}

// mrSeedReadyRun seeds one run in the READY state (branch confirmed, not yet
// started) on an already-seeded repository, with one BLOCKED root job mirroring
// run.prepare's output (no queue message — run.start enqueues it) and an
// implementer session, so HandleRunStart drives the ready->running transition the
// Phase 2 precondition guards. No real git is needed: run.start touches only DB
// state.
func mrSeedReadyRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch, isolation string, writeScope map[string]any) *mrRun {
	t.Helper()
	lane := map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
	if isolation != "" {
		lane["worktree_isolation"] = isolation
	}
	return mrSeedReadyRunWithLane(t, ctx, h, repoID, runID, branch, lane, writeScope)
}

func mrSeedReadyRunWithLane(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch string, lane map[string]any, writeScope map[string]any) *mrRun {
	t.Helper()
	now := time.Now().UTC()
	r := &mrRun{repoID: repoID, runID: runID, branch: branch, implJob: "job_" + runID, implSession: "sess_" + runID}

	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles":       map[string]any{mrImplementerRole: map[string]any{"summary": "implement"}},
		"lanes":       map[string]any{mrImplementerLane: lane},
		"jobs":        []any{map[string]any{"id": fixtureWorkflowJob, "interrogable": false}},
	}
	mrSeedReadyRunRow(t, ctx, h.Runner, repoID, runID, branch, workflow)
	mrSeedBlockedJob(t, ctx, h.Runner, repoID, runID, r.implJob, mrImplementerRole, mrImplementerLane, writeScope, now)
	seedFixtureSession(t, ctx, h.Runner, repoID, runID, r.implSession, mrImplementerRole, mrImplementerLane, []string{"claim", "write"}, 1)
	return r
}

// mrSeedReadyRunRow seeds a run row in the READY state with a confirmed branch
// (the state HandleRunStart drives ready->running) plus its own workflow snapshot.
func mrSeedReadyRunRow(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, branch string, workflow map[string]any) {
	t.Helper()
	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatalf("encode workflow: %v", err)
	}
	snapID := "snap_" + runID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,$4,$5::jsonb,$6)`,
		repoID, snapID, fixtureWorkflowID, "sha_"+runID, workflowArg, now); err != nil {
		t.Fatalf("seed snapshot %s: %v", runID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  branch_name, branch_confirmed_at, branch_confirmed_by
		) VALUES ($1,$2,$3,(SELECT repo_root FROM striatumd.repositories WHERE repository_id=$1),'ready',$4,$5,$4,'human')`,
		repoID, runID, snapID, now, branch); err != nil {
		t.Fatalf("seed ready run %s: %v", runID, err)
	}
}

// mrSeedBlockedJob seeds one BLOCKED root job (no queue message) mirroring
// run.prepare's output, so HandleRunStart's enqueue path (blocked->queued) runs.
func mrSeedBlockedJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, role, lane string, writeScope map[string]any, now time.Time) {
	t.Helper()
	writeScopeArg, err := db.JSONBArg(runner, writeScope)
	if err != nil {
		t.Fatalf("encode write_scope: %v", err)
	}
	laneSelArg, err := db.JSONBArg(runner, map[string]any{"lane_id": lane})
	if err != nil {
		t.Fatalf("encode lane_selector: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, max_attempts, idempotency_key, write_scope_json,
		  expected_artifacts_json, lane_selector_json, created_at
		) VALUES ($1,$2,$3,$4,1,'blocked',$5,'P2 job','draft',1,$6,$7::jsonb,'[]'::jsonb,$8::jsonb,$9)`,
		repoID, jobID, runID, fixtureWorkflowJob, role, "idem_"+jobID,
		writeScopeArg, laneSelArg, now); err != nil {
		t.Fatalf("seed blocked job %s: %v", jobID, err)
	}
}

// mrStartRun drives the run's ready->running transition through the production
// HandleRunStart handler (goroutine-safe — returns the error).
func mrStartRun(ctx context.Context, h *Harness, r *mrRun) error {
	_, err := mrStartRunResult(ctx, h, r, false)
	return err
}

// mrStartRunResult drives run.start and returns the full result map (for asserting
// the Phase 3 cross-run warnings), passing allow_overlap when set.
func mrStartRunResult(ctx context.Context, h *Harness, r *mrRun, allowOverlap bool) (map[string]any, error) {
	params := map[string]any{"run_id": r.runID}
	if allowOverlap {
		params["allow_overlap"] = true
	}
	return mutations.HandleRunStart(ctx, h.Runner, mrEnv(r.repoID, "run.start", params))
}

// mrWarnings extracts the run.start result's Phase 3 cross-run warnings.
func mrWarnings(res map[string]any) []string {
	raw, ok := res["warnings"].([]any)
	if !ok {
		if s, ok := res["warnings"].([]string); ok {
			return s
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// mrSeedReadyScopedRun seeds a per_job-isolated repo-write run in the READY state
// with explicit allowed_paths (for the Phase 3 write-scope-overlap gate). per_job
// isolation keeps the Phase 2 shared-checkout precondition from masking Phase 3.
func mrSeedReadyScopedRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch string, allowed []any) *mrRun {
	t.Helper()
	return mrSeedReadyRun(t, ctx, h, repoID, runID, branch, "per_job",
		map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": allowed})
}

// --- Phase 3: cross-run collision detection --------------------------------

// TestMultiRunSameBranchRefusedWhileSiblingActive is the RFC 0108 Phase 3 branch
// collision gate: two runs may not target the SAME git branch concurrently (they
// would clobber one branch and collide at integration). The second start is
// refused with cross_run_collision unless the operator passes allow_overlap; a
// run on a DISTINCT branch starts freely. Both runs are per_job-isolated so the
// Phase 2 shared-checkout precondition does not mask the Phase 3 branch check.
func TestMultiRunSameBranchRefusedWhileSiblingActive(t *testing.T) {
	repoID := "repo_multirun_p3_branch"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	runA := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p3b_a", "striatum/shared", "per_job")
	if err := mrStartRun(ctx, h, runA); err != nil {
		t.Fatalf("first run on a branch refused with no sibling: %v", err)
	}

	// A second run targeting the SAME branch while A is active — refused.
	runB := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p3b_b", "striatum/shared", "per_job")
	err := mrStartRun(ctx, h, runB)
	if err == nil {
		t.Fatal("second run on the same branch started while a sibling on that branch is active (RFC 0108 Phase 3 branch collision)")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "cross_run_collision" {
		t.Fatalf("refusal = %v, want code cross_run_collision", err)
	}
	if got := chaosRunState(t, ctx, h, runB.runID); got != "ready" {
		t.Fatalf("refused run B state = %q, want ready", got)
	}

	// allow_overlap lets the operator override the branch collision.
	if _, err := mrStartRunResult(ctx, h, runB, true); err != nil {
		t.Fatalf("same-branch start with allow_overlap refused: %v", err)
	}
	if got := chaosRunState(t, ctx, h, runB.runID); got != "running" {
		t.Fatalf("run B with allow_overlap state = %q, want running", got)
	}

	// A run on a DISTINCT branch starts freely beside the active runs.
	runC := mrSeedReadyRepoWriteRun(t, ctx, h, repoID, "run_p3b_c", "striatum/distinct", "per_job")
	if err := mrStartRun(ctx, h, runC); err != nil {
		t.Fatalf("run on a distinct branch refused: %v", err)
	}
}

// TestMultiRunWriteScopeOverlapWarns is the RFC 0108 Phase 3 write-scope-overlap
// gate: two active runs whose repo-write allowed_paths prefix-overlap can both
// START (on distinct branches + per_job worktrees they do not collide at write
// time) but the second start carries a WARNING that names the overlapping run, so
// the operator knows to expect a merge conflict at integration (P4). A run whose
// write scope is disjoint starts with no warning.
func TestMultiRunWriteScopeOverlapWarns(t *testing.T) {
	repoID := "repo_multirun_p3_scope"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "docs/seed.md", "seed\n")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	runA := mrSeedReadyScopedRun(t, ctx, h, repoID, "run_p3s_a", "striatum/p3s-a", []any{"docs"})
	if err := mrStartRun(ctx, h, runA); err != nil {
		t.Fatalf("run A start: %v", err)
	}

	// run B's allowed_paths (docs/rfcs) is a prefix-overlap of run A's (docs).
	runB := mrSeedReadyScopedRun(t, ctx, h, repoID, "run_p3s_b", "striatum/p3s-b", []any{"docs/rfcs"})
	res, err := mrStartRunResult(ctx, h, runB, false)
	if err != nil {
		t.Fatalf("run B with an overlapping write scope was refused (Phase 3 overlap warns, not blocks): %v", err)
	}
	warnings := mrWarnings(res)
	if len(warnings) == 0 {
		t.Fatal("run B started with an overlapping write scope but carried NO warning (RFC 0108 Phase 3)")
	}
	if joined := strings.Join(warnings, " "); !strings.Contains(joined, runA.runID) {
		t.Fatalf("overlap warning %q does not name the colliding run %s", joined, runA.runID)
	}
	if got := chaosRunState(t, ctx, h, runB.runID); got != "running" {
		t.Fatalf("run B state = %q, want running (overlap is a warning, not a refusal)", got)
	}

	// run C's scope (src) is disjoint from docs — starts with no warning.
	runC := mrSeedReadyScopedRun(t, ctx, h, repoID, "run_p3s_c", "striatum/p3s-c", []any{"src"})
	res, err = mrStartRunResult(ctx, h, runC, false)
	if err != nil {
		t.Fatalf("run C start: %v", err)
	}
	if w := mrWarnings(res); len(w) != 0 {
		t.Fatalf("run C with a disjoint write scope carried warnings: %v", w)
	}
}

// --- Phase 4: serialized gated integration ---------------------------------

// TestMultiRunSerializedIntegrationMergesCleanAndSurfacesConflict is the RFC 0108
// Phase 4 gate: two completed runs on their own branches integrate into one
// mainline through the serialized gate (one at a time, mainline advances for
// each), and a pair that conflicts surfaces merge_conflict rather than corrupting
// mainline. The merge is pure git plumbing (merge-tree -> commit-tree ->
// update-ref), so no working tree is ever mutated.
func TestMultiRunSerializedIntegrationMergesCleanAndSurfacesConflict(t *testing.T) {
	repoID := "repo_multirun_p4"
	ctx := context.Background()
	h := NewHarness(t, repoID)

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)
	writeRepoFile(t, repoRoot, "base.txt", "base\n")
	gitCommitAll(t, repoRoot, "base")
	runGit(t, repoRoot, "branch", "-M", "mainline")
	mrSeedRepoRow(t, ctx, h.Runner, repoID, repoRoot)

	// Two runs change DIFFERENT files off mainline (no conflict between them).
	mrSeedBranchChange(t, repoRoot, "mainline", "striatum/p4-a", "a.txt", "from A\n")
	mrSeedBranchChange(t, repoRoot, "mainline", "striatum/p4-b", "b.txt", "from B\n")
	mrSeedCompletedRun(t, ctx, h, repoID, "run_p4_a", "striatum/p4-a")
	mrSeedCompletedRun(t, ctx, h, repoID, "run_p4_b", "striatum/p4-b")

	resA, err := mrIntegrate(ctx, h, repoID, "run_p4_a", "mainline")
	if err != nil {
		t.Fatalf("integrate A: %v", err)
	}
	if resA["integrated"] != true || fmt.Sprint(resA["status"]) != "integrated" {
		t.Fatalf("A not integrated: %#v", resA)
	}
	resB, err := mrIntegrate(ctx, h, repoID, "run_p4_b", "mainline")
	if err != nil {
		t.Fatalf("integrate B: %v", err)
	}
	if resB["integrated"] != true {
		t.Fatalf("B not integrated: %#v", resB)
	}

	// Mainline now carries BOTH runs' changes — the serialized merges composed.
	assertBranchFileEquals(t, repoRoot, "mainline", "a.txt", "from A\n")
	assertBranchFileEquals(t, repoRoot, "mainline", "b.txt", "from B\n")

	// Re-integrating an already-integrated run is an idempotent no-op.
	resA2, err := mrIntegrate(ctx, h, repoID, "run_p4_a", "mainline")
	if err != nil {
		t.Fatalf("re-integrate A: %v", err)
	}
	if fmt.Sprint(resA2["status"]) != "already_integrated" {
		t.Fatalf("re-integrate A status = %v, want already_integrated", resA2["status"])
	}

	// A conflicting pair: both edit the SAME path; the second surfaces the conflict
	// and mainline is left untouched (no corruption).
	mrSeedBranchChange(t, repoRoot, "mainline", "striatum/p4-c", "shared.txt", "C version\n")
	mrSeedBranchChange(t, repoRoot, "mainline", "striatum/p4-d", "shared.txt", "D version\n")
	mrSeedCompletedRun(t, ctx, h, repoID, "run_p4_c", "striatum/p4-c")
	mrSeedCompletedRun(t, ctx, h, repoID, "run_p4_d", "striatum/p4-d")

	if _, err := mrIntegrate(ctx, h, repoID, "run_p4_c", "mainline"); err != nil {
		t.Fatalf("integrate C: %v", err)
	}
	mainlineBefore := gitCapture(t, repoRoot, "rev-parse", "mainline")
	_, err = mrIntegrate(ctx, h, repoID, "run_p4_d", "mainline")
	if err == nil {
		t.Fatal("integrating a conflicting run D succeeded — mainline corruption (RFC 0108 Phase 4 must surface the conflict)")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "merge_conflict" {
		t.Fatalf("conflict refusal = %v, want code merge_conflict", err)
	}
	if !strings.Contains(rpcErr.Error(), "shared.txt") {
		t.Fatalf("merge_conflict did not name the conflicting path: %v", err)
	}
	if got := gitCapture(t, repoRoot, "rev-parse", "mainline"); got != mainlineBefore {
		t.Fatalf("mainline moved after a conflicting integration (corruption): %s -> %s", mainlineBefore, got)
	}
}

// mrSeedBranchChange creates `branch` off `base`, writes `content` to `file`,
// commits it, and restores the working tree to `base` (the integration path is
// pure plumbing, so the checked-out branch never matters).
func mrSeedBranchChange(t *testing.T, repoRoot, base, branch, file, content string) {
	t.Helper()
	runGit(t, repoRoot, "checkout", "-q", base)
	runGit(t, repoRoot, "checkout", "-q", "-b", branch)
	writeRepoFile(t, repoRoot, file, content)
	gitCommitAll(t, repoRoot, "change on "+branch)
	runGit(t, repoRoot, "checkout", "-q", base)
}

// mrSeedCompletedRun seeds a completed run with a confirmed branch (the state
// HandleRunIntegrate requires), plus its own workflow snapshot.
func mrSeedCompletedRun(t *testing.T, ctx context.Context, h *Harness, repoID, runID, branch string) {
	t.Helper()
	now := time.Now().UTC()
	snapID := "snap_" + runID
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,$4,'{}'::jsonb,$5)`,
		repoID, snapID, fixtureWorkflowID, "sha_"+runID, now); err != nil {
		t.Fatalf("seed snapshot %s: %v", runID, err)
	}
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  completed_at, branch_name, branch_confirmed_at, branch_confirmed_by
		) VALUES ($1,$2,$3,(SELECT repo_root FROM striatumd.repositories WHERE repository_id=$1),
		         'completed',$4,$4,$5,$4,'human')`,
		repoID, runID, snapID, now, branch); err != nil {
		t.Fatalf("seed completed run %s: %v", runID, err)
	}
}

// mrIntegrate drives run.integrate through the production handler.
func mrIntegrate(ctx context.Context, h *Harness, repoID, runID, into string) (map[string]any, error) {
	return mutations.HandleRunIntegrate(ctx, h.Runner, mrEnv(repoID, "run.integrate", map[string]any{
		"run_id": runID, "into": into,
	}))
}

// gitCapture runs a git command and returns its trimmed stdout.
func gitCapture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// assertBranchFileEquals asserts `file` at `branch`'s tree equals `want`.
func assertBranchFileEquals(t *testing.T, repoRoot, branch, file, want string) {
	t.Helper()
	got := gitCapture(t, repoRoot, "show", branch+":"+file)
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("%s:%s = %q, want %q", branch, file, got, want)
	}
}

// isConcurrencyDeadlock reports whether err is a Postgres serialization/deadlock
// abort (SQLSTATE 40P01) surfaced raw to a caller — the RFC 0104 failure shape.
func isConcurrencyDeadlock(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "40P01") ||
		strings.Contains(msg, "deadlock") ||
		strings.Contains(msg, "aborted by a database deadlock")
}
