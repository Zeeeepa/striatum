package mutations

import (
	"context"
	"os/exec"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// applyBarrierAssemblyOwnerBundle widens the jobs job_type CHECK to permit
// 'barrier_assembly' (owner bundle 0013). The mutations pgtests run against a
// runtime-migrated DB that does NOT apply owner bundles, so the RFC 0135 P2 path
// needs the owner-table DDL applied first. pgtest.Pool returns the privileged
// (owner=current user) runner, which ApplyOwnerBundles requires. Applying all
// bundles is idempotent and reaches LatestOwnerBundleVersion (>= 13).
func applyBarrierAssemblyOwnerBundle(t *testing.T, ctx context.Context, runner db.Runner) {
	t.Helper()
	if _, _, err := db.ApplyOwnerBundles(ctx, runner, "test"); err != nil {
		t.Fatalf("apply owner bundles (0013 barrier_assembly job_type): %v", err)
	}
}

// execer is the minimal write surface shared by db.Runner and db.TxRunner, so the
// barrier_assembly INSERT probe can run on either the pool runner or a tx.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// insertBarrierAssemblyJob inserts a jobs row with job_type='barrier_assembly'.
// It is the deployment-gate probe: it succeeds only when the live
// jobs_job_type_check permits the value (owner bundle 0013 applied), and raises a
// CHECK violation otherwise — exactly the behavior the in-Go deployment-tolerance
// guard (jobBarrierAssemblyTypePermitted) avoids triggering by probing first.
func insertBarrierAssemblyJob(ctx context.Context, runner execer, repoID, runID, jobID, workflowJobID string) error {
	return runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, attempt, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'Assemble','barrier_assembly','assembler','queued',1,$5,now())`,
		repoID, jobID, runID, workflowJobID, "idem_"+jobID)
}

// TestBarrierAssemblyJobTypePermittedTracksOwnerBundle is the RFC 0135 P2
// deployment-tolerance fence: the in-Go probe jobBarrierAssemblyTypePermitted
// reports FALSE on a runtime-migrated-only database (owner bundle 0013 unapplied)
// and TRUE once the bundle is applied — without itself raising a CHECK violation.
// A behind-deployment must fall back to D206 (the opt-in is gated on this probe),
// never persist a barrier_assembly job and crash the CHECK.
func TestBarrierAssemblyJobTypePermittedTracksOwnerBundle(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_ba_gate"
	runID := "run_ba_gate"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})

	// BEHIND-DEPLOYMENT (bundle < 13): the probe reports NOT permitted, and a
	// barrier_assembly INSERT would CHECK-fail. The opt-in is gated on the probe, so
	// D206 stays the path; nothing crashes.
	tx := beginBarrierTx(t, ctx, pool)
	permitted, err := jobBarrierAssemblyTypePermitted(ctx, tx)
	if err != nil {
		t.Fatalf("probe (bundle unapplied): %v", err)
	}
	if permitted {
		t.Fatal("jobBarrierAssemblyTypePermitted must be FALSE before owner bundle 0013 is applied")
	}
	// Confirm the gate is real: a barrier_assembly INSERT does CHECK-fail pre-bundle.
	if err := insertBarrierAssemblyJob(ctx, tx, repoID, runID, "job_ba_pre", "asm"); err == nil {
		t.Fatal("a barrier_assembly job INSERT must CHECK-fail before owner bundle 0013")
	}
	_ = tx.Rollback(ctx)

	// Apply owner bundle 0013 (privileged owner runner).
	applyBarrierAssemblyOwnerBundle(t, ctx, runner)

	// POST-BUNDLE: the probe reports permitted, and the job_type is accepted.
	tx2 := beginBarrierTx(t, ctx, pool)
	permitted, err = jobBarrierAssemblyTypePermitted(ctx, tx2)
	if err != nil {
		t.Fatalf("probe (bundle applied): %v", err)
	}
	if !permitted {
		t.Fatal("jobBarrierAssemblyTypePermitted must be TRUE after owner bundle 0013 is applied")
	}
	_ = tx2.Rollback(ctx)
	if err := insertBarrierAssemblyJob(ctx, runner, repoID, runID, "job_ba_post", "asm"); err != nil {
		t.Fatalf("a barrier_assembly job INSERT must be accepted after owner bundle 0013: %v", err)
	}
}

// TestBarrierAssemblyTwoPhaseJournalAndN1 exercises the recoverable assembly end to
// end through the UNIFIED path with a SINGLE sibling (N=1): the assembly journals
// its intent (state='assembling') BEFORE the git CAS, advances the run branch, and
// flips to 'committed'. The assembled run-branch tree equals the D206
// single-completion tree (the N=1 unification: no special-case branch, the same
// assembleFaninBarrier fold).
func TestBarrierAssemblyTwoPhaseJournalAndN1(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	applyBarrierAssemblyOwnerBundle(t, ctx, runner)

	// --- D206 reference: a single sibling integrated per-completion. ---
	d206Root := t.TempDir()
	base := gitInit(t, d206Root)
	runBranch := "wf/n1"
	gitRun(t, d206Root, "branch", runBranch, base)
	headA := commitSiblingOffBase(t, d206Root, base, "docs/a.txt", "A\n")
	if _, err := fanInIntegrateRunBranch(ctx, d206Root, "refs/heads/"+runBranch, base, headA, "job_a", 1); err != nil {
		t.Fatalf("D206 integrate single sibling: %v", err)
	}
	d206Tree := gitRun(t, d206Root, "rev-parse", "refs/heads/"+runBranch+"^{tree}")

	// --- Barrier assembly over the SAME single staged contribution. ---
	barrierRoot := t.TempDir()
	bbase := gitInit(t, barrierRoot)
	gitRun(t, barrierRoot, "branch", runBranch, bbase)
	bHeadA := commitSiblingOffBase(t, barrierRoot, bbase, "docs/a.txt", "A\n")

	repoID := "repo_ba_n1"
	runID := "run_ba_n1"
	barrierID := "barrier_ba_n1"
	runRef := "refs/heads/" + runBranch
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a", "author_a", "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            bbase,
		DeclaredSiblingJobIDs:   []string{"author_a"}, // N=1
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_a", "job_a", bHeadA, 1); err != nil {
		t.Fatalf("stage single sibling: %v", err)
	}
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil || !ready {
		t.Fatalf("N=1 barrier should be ready with the one sibling staged: ready=%v err=%v", ready, err)
	}

	res, err := runBarrierAssembly(ctx, tx, barrierRoot, runRef, repoID, barrierID, "asm", "job_asm")
	if err != nil {
		t.Fatalf("run barrier assembly (N=1): %v", err)
	}
	if res.Mode != "assembled" {
		t.Fatalf("first assembly mode = %q; want assembled", res.Mode)
	}

	// The run branch advanced to the assembled commit, and its tree equals the D206
	// single-completion tree (N=1 unification: identical final tree).
	runTip := gitRevParse(t, barrierRoot, runRef)
	if runTip != res.CommitSHA {
		t.Fatalf("run branch tip %s != assembled commit %s", runTip, res.CommitSHA)
	}
	barrierTree := gitRun(t, barrierRoot, "rev-parse", runRef+"^{tree}")
	if barrierTree != d206Tree {
		t.Fatalf("N=1 barrier assembly tree %s != D206 single-completion tree %s", barrierTree, d206Tree)
	}

	// The journal walked sealed/absent -> assembling -> committed.
	st, err := loadBarrierAssemblyState(ctx, tx, repoID, barrierID)
	if err != nil {
		t.Fatalf("load barrier state: %v", err)
	}
	if !st.Present || st.State != barrierStateCommitted {
		t.Fatalf("barrier_state should be 'committed', got present=%v state=%q", st.Present, st.State)
	}
	if st.TargetCommitSHA != res.CommitSHA || st.TreeSHA != res.TreeSHA {
		t.Fatalf("journaled target/tree (%s / %s) != result (%s / %s)", st.TargetCommitSHA, st.TreeSHA, res.CommitSHA, res.TreeSHA)
	}
	_ = tx.Rollback(ctx)
}

// TestBarrierAssemblyCrashMidAssemblyResumes is the RFC 0133/0135 crash-recovery
// fence: a barrier_state row is journaled in state='assembling' with a target
// intent but the run branch was NEVER advanced (the assembler crashed AFTER the PG
// journal write but BEFORE/DURING the git CAS). A re-run recognizes its OWN
// journaled intent, resumes Phase 2 (advances the run branch to the same commit),
// and flips to 'committed' — never wedging, never double-committing.
func TestBarrierAssemblyCrashMidAssemblyResumes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	applyBarrierAssemblyOwnerBundle(t, ctx, runner)

	barrierRoot := t.TempDir()
	bbase := gitInit(t, barrierRoot)
	runBranch := "wf/crash"
	runRef := "refs/heads/" + runBranch
	gitRun(t, barrierRoot, "branch", runBranch, bbase)
	bHeadA := commitSiblingOffBase(t, barrierRoot, bbase, "docs/a.txt", "A\n")
	bHeadB := commitSiblingOffBase(t, barrierRoot, bbase, "docs/b.txt", "B\n")

	repoID := "repo_ba_crash"
	runID := "run_ba_crash"
	barrierID := "barrier_ba_crash"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a", "author_a", "completed", 1)
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_b", "author_b", "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            bbase,
		DeclaredSiblingJobIDs:   []string{"author_a", "author_b"},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_a", "job_a", bHeadA, 1); err != nil {
		t.Fatalf("stage sibling A: %v", err)
	}
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_b", "job_b", bHeadB, 1); err != nil {
		t.Fatalf("stage sibling B: %v", err)
	}

	// Simulate a crash MID-ASSEMBLY: compute the deterministic assembled commit and
	// journal the intent (state='assembling'), but do NOT advance the run branch (the
	// crash point). This is exactly the durable state runBarrierAssembly's recovery
	// branch must resume from.
	treeSHA, commitSHA, _, err := assembleFaninBarrier(ctx, tx, barrierRoot, repoID, barrierID)
	if err != nil {
		t.Fatalf("pre-crash assemble: %v", err)
	}
	if err := journalBarrierAssemblyIntent(ctx, tx, repoID, barrierID, runID, "synth", "asm", "job_asm", bbase, commitSHA, treeSHA); err != nil {
		t.Fatalf("journal crash intent: %v", err)
	}
	// The run branch is still at the frozen base — the CAS never happened.
	if got := gitRevParse(t, barrierRoot, runRef); got != bbase {
		t.Fatalf("precondition: run branch should still be at the frozen base %s, got %s", bbase, got)
	}

	// RECOVERY: re-run the assembly. It must recognize its own journaled intent,
	// resume Phase 2, advance the run branch to the SAME commit, and commit.
	res, err := runBarrierAssembly(ctx, tx, barrierRoot, runRef, repoID, barrierID, "asm", "job_asm")
	if err != nil {
		t.Fatalf("crash-recovery re-run: %v", err)
	}
	if res.Mode != "resumed_recovery" {
		t.Fatalf("recovery mode = %q; want resumed_recovery", res.Mode)
	}
	if res.CommitSHA != commitSHA {
		t.Fatalf("recovery produced a DIFFERENT commit %s than the journaled intent %s (double-commit / wedge)", res.CommitSHA, commitSHA)
	}
	if got := gitRevParse(t, barrierRoot, runRef); got != commitSHA {
		t.Fatalf("recovery did not advance the run branch to the journaled commit: got %s want %s", got, commitSHA)
	}
	st, err := loadBarrierAssemblyState(ctx, tx, repoID, barrierID)
	if err != nil {
		t.Fatalf("load state after recovery: %v", err)
	}
	if st.State != barrierStateCommitted {
		t.Fatalf("post-recovery state = %q; want committed", st.State)
	}

	// IDEMPOTENT re-run after success: a second run must be a no-op (already_committed),
	// never double-commit.
	res2, err := runBarrierAssembly(ctx, tx, barrierRoot, runRef, repoID, barrierID, "asm", "job_asm")
	if err != nil {
		t.Fatalf("post-commit idempotent re-run: %v", err)
	}
	if res2.Mode != "already_committed" {
		t.Fatalf("post-commit re-run mode = %q; want already_committed", res2.Mode)
	}
	if got := gitRevParse(t, barrierRoot, runRef); got != commitSHA {
		t.Fatalf("idempotent re-run moved the run branch: got %s want %s", got, commitSHA)
	}
	_ = tx.Rollback(ctx)
}

// TestBarrierAssemblyAlreadyAppliedCommitIsIdempotent covers the other crash
// window: the assembler advanced the run branch (the git CAS landed) but crashed
// BEFORE flipping the journal to 'committed' (still 'assembling'). A re-run sees the
// run branch already at its own journaled commit and must recognize it, NOT
// double-commit, and finalize the journal.
func TestBarrierAssemblyAlreadyAppliedCommitIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	applyBarrierAssemblyOwnerBundle(t, ctx, runner)

	barrierRoot := t.TempDir()
	bbase := gitInit(t, barrierRoot)
	runBranch := "wf/applied"
	runRef := "refs/heads/" + runBranch
	gitRun(t, barrierRoot, "branch", runBranch, bbase)
	bHeadA := commitSiblingOffBase(t, barrierRoot, bbase, "docs/a.txt", "A\n")

	repoID := "repo_ba_applied"
	runID := "run_ba_applied"
	barrierID := "barrier_ba_applied"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a", "author_a", "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            bbase,
		DeclaredSiblingJobIDs:   []string{"author_a"},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_a", "job_a", bHeadA, 1); err != nil {
		t.Fatalf("stage sibling A: %v", err)
	}
	treeSHA, commitSHA, _, err := assembleFaninBarrier(ctx, tx, barrierRoot, repoID, barrierID)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	// Journal the intent AND advance the run branch (the CAS landed) — but leave the
	// journal in 'assembling' (crashed before the 'committed' flip).
	if err := journalBarrierAssemblyIntent(ctx, tx, repoID, barrierID, runID, "synth", "asm", "job_asm", bbase, commitSHA, treeSHA); err != nil {
		t.Fatalf("journal intent: %v", err)
	}
	if err := advanceRunBranchToAssembled(ctx, barrierRoot, runRef, bbase, commitSHA, barrierID); err != nil {
		t.Fatalf("apply CAS: %v", err)
	}

	// RECOVERY: re-run. The run branch is already at the journaled commit; recovery
	// must recognize it (no double-commit) and finalize the journal.
	res, err := runBarrierAssembly(ctx, tx, barrierRoot, runRef, repoID, barrierID, "asm", "job_asm")
	if err != nil {
		t.Fatalf("recovery re-run (commit already applied): %v", err)
	}
	if res.Mode != "resumed_recovery" {
		t.Fatalf("recovery mode = %q; want resumed_recovery", res.Mode)
	}
	if got := gitRevParse(t, barrierRoot, runRef); got != commitSHA {
		t.Fatalf("recovery double-committed / moved the run branch: got %s want %s", got, commitSHA)
	}
	st, err := loadBarrierAssemblyState(ctx, tx, repoID, barrierID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.State != barrierStateCommitted {
		t.Fatalf("post-recovery state = %q; want committed", st.State)
	}
	_ = tx.Rollback(ctx)
}
