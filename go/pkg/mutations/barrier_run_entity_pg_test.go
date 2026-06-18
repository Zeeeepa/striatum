package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedRunEntityRepoAndRun seeds a repository + a completed, branch-confirmed run
// with a disjoint run-branch change off main (and disjoint intervening main work),
// returning the run id, run branch, and the base sha. It mirrors the seeding in
// TestRunIntegratePreservesInterveningMainWork so the run-entity barrier is exercised
// against the same fixture shape HandleRunIntegrate is.
func seedRunEntityRepoAndRun(t *testing.T, ctx context.Context, runner db.Runner, repoRoot, repoID, runID, runBranch string) string {
	t.Helper()
	baseSHA := gitInit(t, repoRoot)
	now := time.Now().UTC()

	// The run branch forks from main at baseSHA and makes a DISJOINT change.
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	wt := filepath.Join(repoRoot, ".wt_re")
	gitRun(t, repoRoot, "worktree", "add", "--detach", wt, runBranch)
	if err := os.MkdirAll(filepath.Join(wt, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "src", "run.go"), []byte("run branch change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "src/run.go")
	gitRun(t, wt, "commit", "-q", "-m", "run branch change")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, gitRevParse(t, wt, "HEAD"))

	// main advances AFTER the fork with disjoint intervening work.
	if err := os.WriteFile(filepath.Join(repoRoot, "landed.md"), []byte("landed on main after the fork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "landed.md")
	gitRun(t, repoRoot, "commit", "-q", "-m", "intervening main work")

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+repoID, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	wfArg, err := db.JSONBArg(runner, map[string]any{"workflow_id": "wf_re"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_re','wf_re','sha',$2::jsonb,$3)`, repoID, wfArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at, completed_at
		) VALUES ($1,$2,'snap_re',$3,'completed',$4,$5,$6,'human',$6,$6)`,
		repoID, runID, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	return baseSHA
}

// TestRunIntegrateIsTheRunEntityBarrier is the RFC 0135 P6 DELIVERABLE GATE — the
// equivalence-fixture proof for the highest-risk fold. It asserts the run-entity
// sealed barrier (the shadow shadowRunEntityIntegrate path) produces:
//
//	(1) the SAME integrated tree OID as today's live HandleRunIntegrate, and
//	(2) the SAME idempotency outcome (an already-integrated run is a no-op naming
//	    the prior merge commit),
//
// asserted BEFORE any caller flips. The shadow reuses the EXACT shared assembly
// (assembleRunEntityIntegration) and the EXACT idempotency oracle (runIntegratedInto)
// the live path now uses, so this proves the structural recasting is non-breaking:
// run.integrate AS the run-entity barrier yields the identical observable result.
func TestRunIntegrateIsTheRunEntityBarrier(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoRoot := t.TempDir()
	repoID := "repo_run_entity"
	runID := "run_run_entity"
	runBranch := "striatum/run-entity"
	seedRunEntityRepoAndRun(t, ctx, runner, repoRoot, repoID, runID, runBranch)

	// --- Part A: the SHADOW run-entity barrier computes the integration BEFORE the
	// live path runs. Because the assembly is a pure computation (no ref writes), the
	// shadow leaves the repository observably unchanged. ---
	tx := beginBarrierTx(t, ctx, pool)
	shadow, err := shadowRunEntityIntegrate(ctx, tx, repoRoot, repoID, runID, runBranch, "main")
	if err != nil {
		t.Fatalf("shadow run-entity integrate: %v", err)
	}
	_ = tx.Rollback(ctx)
	if shadow.Status != "would_integrate" {
		t.Fatalf("shadow should report would_integrate for a not-yet-integrated run, got %q", shadow.Status)
	}
	if !isFullGitSHA(shadow.TreeOID) {
		t.Fatalf("shadow produced no merged tree OID: %#v", shadow)
	}
	// The shadow did NOT advance main (it is read-only w.r.t. ref state).
	mainBeforeLive := gitRevParse(t, repoRoot, "refs/heads/main")

	// --- Part B: the LIVE HandleRunIntegrate path advances main for real. ---
	res, err := HandleRunIntegrate(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "into": "main"}))
	if err != nil {
		t.Fatalf("live integrate: %v", err)
	}
	liveMergeCommit := fmt.Sprint(res["merge_commit"])
	if !isFullGitSHA(liveMergeCommit) {
		t.Fatalf("live integrate missing merge_commit: %#v", res)
	}
	mainAfterLive := gitRevParse(t, repoRoot, "refs/heads/main")
	if mainAfterLive == mainBeforeLive {
		t.Fatalf("live integrate did not advance main (shadow must not have advanced it either)")
	}
	liveTree := gitRun(t, repoRoot, "rev-parse", "refs/heads/main^{tree}")

	// THE EQUIVALENCE (1): the run-entity barrier's integrated tree OID is
	// BYTE-IDENTICAL to the live HandleRunIntegrate result. The tree is the
	// deterministic part of the merge (merge-tree output); the merge-commit OID can
	// differ only by commit timestamp, which is not part of the integrated tree the
	// gate asserts.
	if shadow.TreeOID != liveTree {
		t.Fatalf("run-entity barrier tree %s != live HandleRunIntegrate tree %s (the equivalence fixture must hold before any flip)", shadow.TreeOID, liveTree)
	}
	// The live merge commit's tree is exactly the assembled tree, confirming the
	// shared assembly is what advanced the live ref.
	liveMergeTree := gitRun(t, repoRoot, "rev-parse", liveMergeCommit+"^{tree}")
	if liveMergeTree != shadow.TreeOID {
		t.Fatalf("live merge commit tree %s != run-entity assembled tree %s", liveMergeTree, shadow.TreeOID)
	}

	// --- Part C: idempotency equivalence. After the live integration, the run is
	// already integrated into main; the run-entity barrier's idempotency oracle must
	// report a no-op naming the SAME prior merge commit the live path would. ---
	live2, err := HandleRunIntegrate(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "into": "main"}))
	if err != nil {
		t.Fatalf("live re-integrate (idempotency): %v", err)
	}
	if fmt.Sprint(live2["status"]) != "already_integrated" {
		t.Fatalf("live re-integrate should be already_integrated, got %q", live2["status"])
	}
	if fmt.Sprint(live2["merge_commit"]) != liveMergeCommit {
		t.Fatalf("live idempotent merge_commit %v != original %s", live2["merge_commit"], liveMergeCommit)
	}

	tx2 := beginBarrierTx(t, ctx, pool)
	shadow2, err := shadowRunEntityIntegrate(ctx, tx2, repoRoot, repoID, runID, runBranch, "main")
	if err != nil {
		t.Fatalf("shadow re-integrate (idempotency): %v", err)
	}
	_ = tx2.Rollback(ctx)

	// THE EQUIVALENCE (2): the run-entity barrier and the live path report the SAME
	// idempotency outcome — an already-integrated no-op naming the SAME merge commit.
	if shadow2.Status != "already_integrated" {
		t.Fatalf("run-entity barrier should report already_integrated after the live integration, got %q", shadow2.Status)
	}
	if shadow2.MergeCommit != liveMergeCommit {
		t.Fatalf("run-entity barrier idempotent merge_commit %s != live %s", shadow2.MergeCommit, liveMergeCommit)
	}
}

// TestRunEntityBarrierReadyComposesTerminalStateAndJobBarriers proves the run-entity
// barrier readiness predicate (RFC 0135 P6) composes (a) the run's terminal-acceptable
// state with (b) every declared job-level barrier having fired — expressed through
// P0's db.BarrierReadySQL shape (entity_kind='run'). A non-completed run is not ready;
// a completed run with NO declared job barriers is ready (the single-lane case,
// reducing to the terminal-state gate, matching HandleRunIntegrate); a completed run
// with a declared-but-unfired job barrier is NOT ready until that barrier commits.
func TestRunEntityBarrierReadyComposesTerminalStateAndJobBarriers(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_run_entity_ready"
	runID := "run_run_entity_ready"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})

	tx := beginBarrierTx(t, ctx, pool)

	// A non-completed (running) run is NOT ready, regardless of job barriers.
	if err := tx.Exec(ctx, `UPDATE striatumd.runs SET state = 'running' WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("set run running: %v", err)
	}
	if ready, err := runEntityBarrierReady(ctx, tx, repoID, runID); err != nil {
		t.Fatalf("readiness (running): %v", err)
	} else if ready {
		t.Fatal("run-entity barrier must NOT be ready for a non-completed run")
	}

	// A completed run with NO declared job-level barriers reduces to the terminal
	// state gate → ready (the single-lane case; HandleRunIntegrate integrates it).
	if err := tx.Exec(ctx, `UPDATE striatumd.runs SET state = 'completed', completed_at = now() WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("set run completed: %v", err)
	}
	if ready, err := runEntityBarrierReady(ctx, tx, repoID, runID); err != nil {
		t.Fatalf("readiness (completed, no job barriers): %v", err)
	} else if !ready {
		t.Fatal("run-entity barrier must be ready for a completed run with no declared job barriers")
	}

	// Declare a job-level barrier inside the run (a fanin_freeze_points row). Until it
	// FIRES (a committed barrier_state row), the run-entity barrier is NOT ready —
	// the job-level barrier composes into the run-level one (RFC 0135 P6).
	jobBarrierID := "job-barrier-1"
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               jobBarrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            "0000000000000000000000000000000000000000",
		DeclaredSiblingJobIDs:   []string{"author_a"},
	}); err != nil {
		t.Fatalf("record job-level freeze point: %v", err)
	}
	if ready, err := runEntityBarrierReady(ctx, tx, repoID, runID); err != nil {
		t.Fatalf("readiness (completed, unfired job barrier): %v", err)
	} else if ready {
		t.Fatal("run-entity barrier must NOT be ready while a declared job-level barrier has not fired")
	}

	// Fire the job-level barrier (a committed barrier_state row) → the run-entity
	// barrier composes to ready.
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.barrier_state
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id, state, committed_at)
		VALUES ($1,$2,$3,'synth','committed', now())`,
		repoID, jobBarrierID, runID); err != nil {
		t.Fatalf("commit job-level barrier_state: %v", err)
	}
	if ready, err := runEntityBarrierReady(ctx, tx, repoID, runID); err != nil {
		t.Fatalf("readiness (completed, fired job barrier): %v", err)
	} else if !ready {
		t.Fatal("run-entity barrier must be ready once every declared job-level barrier has fired")
	}
	_ = tx.Rollback(ctx)
}
