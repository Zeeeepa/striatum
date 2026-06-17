package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedFaninSeatJob inserts a jobs row for one fan-in seat at a given attempt and
// state — the live-seal source the barrier JOINs against. The jobs UNIQUE key
// (repository_id, run_id, workflow_job_id, attempt) means a later attempt is a
// distinct historical row, exactly as a requeue produces.
func seedFaninSeatJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, state string, attempt int) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, attempt, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'Build','build','implementer',$5,$6,$7,$8)`,
		repoID, jobID, runID, workflowJobID, state, attempt, "idem_"+jobID, now); err != nil {
		t.Fatalf("insert fan-in seat job %s (attempt %d): %v", jobID, attempt, err)
	}
}

// TestSealedBarrierJoinsOnLiveSeal is the RFC 0135 P1 trap-killer fence
// (seal=attempt, the generalized form of RFC 0133's
// TestFaninBarrierJoinsOnLiveAttempt): a contribution staged at attempt=1, then the
// seat's live seal advances to attempt=2 (a requeue) — the barrier must NOT fire on
// the attempt-1 staged ref, and must fire only once an attempt-2 ref exists. A
// stale-attempt staging row is STRUCTURALLY invisible because the predicate JOINs
// staged.attempt = live.attempt, never counts staged refs per seat.
func TestSealedBarrierJoinsOnLiveSeal(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_barrier_liveseal"
	runID := "run_barrier_liveseal"
	barrierID := "barrier_liveseal"
	seatA := "author_a"
	seatB := "author_b"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})

	// Both seats complete at attempt 1.
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a1", seatA, "completed", 1)
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_b1", seatB, "completed", 1)

	// Freeze the barrier over the two declared seats. (No git repo needed for the
	// readiness predicate — it runs entirely against PG.)
	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            "0000000000000000000000000000000000000000",
		DeclaredSiblingJobIDs:   []string{seatA, seatB},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}
	// Stage seat A at attempt 1, seat B not yet staged.
	stageRow(t, ctx, tx, repoID, barrierID, runID, seatA, "job_a1", 1)
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil {
		t.Fatalf("readiness (A staged only): %v", err)
	} else if ready {
		t.Fatal("barrier must NOT fire with seat B unstaged")
	}
	// Stage seat B at attempt 1 → both live-staged → ready.
	stageRow(t, ctx, tx, repoID, barrierID, runID, seatB, "job_b1", 1)
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil {
		t.Fatalf("readiness (both staged at attempt 1): %v", err)
	} else if !ready {
		t.Fatal("barrier must fire once every seat is live-staged at the live attempt")
	}

	// THE TRAP: seat B is requeued — its live attempt advances to 2 (a new completed
	// jobs row), while only the STALE attempt-1 staging row exists. The barrier must
	// drop back to NOT-ready: the attempt-1 ref no longer matches live attempt 2.
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_b2", seatB, "completed", 2)
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil {
		t.Fatalf("readiness (B requeued, only stale attempt-1 ref): %v", err)
	} else if ready {
		t.Fatal("barrier must NOT fire on a stale attempt-1 ref after the seat advanced to attempt 2 (RFC 0135 trap)")
	}

	// Once seat B stages at attempt 2 (its live seal), the barrier fires again.
	stageRow(t, ctx, tx, repoID, barrierID, runID, seatB, "job_b2", 2)
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil {
		t.Fatalf("readiness (B staged at attempt 2): %v", err)
	} else if !ready {
		t.Fatal("barrier must fire once seat B stages at its live attempt 2")
	}
	_ = tx.Rollback(ctx)
}

// TestSealedBarrierFreezePointIsImmutable is the RFC 0135 P1 immutability fence
// (RFC 0133's TestFaninFreezePointIsImmutable): an UPDATE or DELETE of a freeze
// record as the runtime role striatumd_rw is rejected by the BEFORE UPDATE/DELETE
// refuse-trigger AND the SELECT/INSERT-only grant. We exercise the unprivileged
// pool (which SETs ROLE to a login over striatumd_rw membership), so this asserts
// the production privilege boundary, not the superuser path that pgtest's
// privileged pool would mask.
func TestSealedBarrierFreezePointIsImmutable(t *testing.T) {
	ctx := context.Background()
	privileged, unprivileged := pgtest.Pools(t)

	repoID := "repo_freeze_immutable"
	runID := "run_freeze_immutable"
	barrierID := "barrier_freeze_immutable"

	intgSeedRepo(t, ctx, privileged.Runner, repoID)
	intgSeedRun(t, ctx, privileged.Runner, repoID, runID, map[string]any{"workflow_id": "wf"})

	// Insert the freeze record as the unprivileged runtime role (INSERT is granted).
	tx, err := unprivileged.Runner.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin unprivileged tx: %v", err)
	}
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            "0000000000000000000000000000000000000000",
		DeclaredSiblingJobIDs:   []string{"author_a"},
	}); err != nil {
		t.Fatalf("insert freeze record as striatumd_rw should be allowed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit freeze record: %v", err)
	}

	// UPDATE as striatumd_rw must be rejected (no UPDATE grant + refuse-trigger).
	err = unprivileged.Runner.Exec(ctx, `
		UPDATE striatumd.fanin_freeze_points SET frozen_tip_sha = 'tampered'
		 WHERE repository_id = $1 AND barrier_id = $2`, repoID, barrierID)
	if err == nil {
		t.Fatal("UPDATE of an immutable freeze record as striatumd_rw must be rejected")
	}
	// DELETE as striatumd_rw must be rejected.
	err = unprivileged.Runner.Exec(ctx, `
		DELETE FROM striatumd.fanin_freeze_points
		 WHERE repository_id = $1 AND barrier_id = $2`, repoID, barrierID)
	if err == nil {
		t.Fatal("DELETE of an immutable freeze record as striatumd_rw must be rejected")
	}

	// The record is unchanged (read via the privileged pool to confirm).
	row, err := oneRow(ctx, privileged.Runner, `
		SELECT frozen_tip_sha FROM striatumd.fanin_freeze_points
		 WHERE repository_id = $1 AND barrier_id = $2`, repoID, barrierID)
	if err != nil {
		t.Fatalf("read freeze record: %v", err)
	}
	if got := fmt.Sprint(row["frozen_tip_sha"]); got != "0000000000000000000000000000000000000000" {
		t.Fatalf("freeze record was mutated despite immutability: frozen_tip_sha = %q", got)
	}
}

// TestSealedBarrierRequeueTombstone is the RFC 0133 requeue-tombstone fence: when a
// seat is requeued, voidFaninContribution marks its prior staging rows 'voided' so
// they can never re-satisfy the barrier even at a colliding attempt — and the
// barrier reads only the live, non-voided contribution. A voided attempt-1 row,
// even if attempt 1 were to recur, is structurally excluded by the WHERE status =
// 'staged' clause in the staged_contrib relation.
func TestSealedBarrierRequeueTombstone(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_barrier_tombstone"
	runID := "run_barrier_tombstone"
	barrierID := "barrier_tombstone"
	seat := "author_a"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a1", seat, "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            "0000000000000000000000000000000000000000",
		DeclaredSiblingJobIDs:   []string{seat},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}
	stageRow(t, ctx, tx, repoID, barrierID, runID, seat, "job_a1", 1)
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil || !ready {
		t.Fatalf("barrier should fire at attempt 1: ready=%v err=%v", ready, err)
	}

	// Requeue: the seat advances to attempt 2 and its prior contribution is
	// tombstoned. voidFaninContribution touches no git (no repo here), so pass a
	// repoRoot whose refs do not resolve — the rename is best-effort and the PG
	// tombstone is the authoritative exclusion.
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_a2", seat, "completed", 2)
	if err := voidFaninContribution(ctx, tx, t.TempDir(), repoID, barrierID, runID, seat, 2, "requeue"); err != nil {
		t.Fatalf("void prior contribution: %v", err)
	}
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil {
		t.Fatalf("readiness after tombstone: %v", err)
	} else if ready {
		t.Fatal("barrier must NOT fire on a tombstoned attempt-1 contribution after requeue to attempt 2")
	}
	// The prior row is recorded as voided, so recovery can explain the exclusion.
	row, err := oneRow(ctx, tx, `
		SELECT status, void_reason FROM striatumd.barrier_staged_contributions
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = $3 AND attempt = 1`,
		repoID, barrierID, seat)
	if err != nil {
		t.Fatalf("read tombstoned row: %v", err)
	}
	if fmt.Sprint(row["status"]) != "voided" || fmt.Sprint(row["void_reason"]) != "requeue" {
		t.Fatalf("attempt-1 row should be voided with reason 'requeue', got status=%v reason=%v", row["status"], row["void_reason"])
	}
	_ = tx.Rollback(ctx)
}

// TestFaninBarrierSameFinalTreeAsPerCompletion is the RFC 0135 / RFC 0133 cutover
// discipline fence — the same-final-tree EQUIVALENCE FIXTURE. It proves the opt-in
// barrier assembly (assembleFaninBarrier: fold staged refs onto the frozen tip in
// canonical workflow_job_id order) produces a tree BYTE-IDENTICAL to the shipped
// D206 per-completion run-branch merge (fanInIntegrateRunBranch), for two
// disjoint-write-scope siblings completing in some order. This is the parity proof
// the default must keep before any workflow flips off D206 onto the barrier.
func TestFaninBarrierSameFinalTreeAsPerCompletion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	// --- Branch 1: the shipped D206 per-completion merge into a run branch. ---
	d206Root := t.TempDir()
	base := gitInit(t, d206Root)
	runBranch := "wf/equiv"
	gitRun(t, d206Root, "branch", runBranch, base)

	// Sibling A commits docs/a.txt off the frozen tip; integrate it per-completion.
	headA := commitSiblingOffBase(t, d206Root, base, "docs/a.txt", "A\n")
	if _, err := fanInIntegrateRunBranch(ctx, d206Root, "refs/heads/"+runBranch, base, headA, "job_a", 1); err != nil {
		t.Fatalf("D206 integrate sibling A: %v", err)
	}
	// Sibling B forked from the SAME frozen tip (a true fan-in fork) and now cannot
	// fast-forward; integrate it per-completion (the #290 content merge).
	headB := commitSiblingOffBase(t, d206Root, base, "docs/b.txt", "B\n")
	runTip := gitRevParse(t, d206Root, "refs/heads/"+runBranch)
	if _, err := fanInIntegrateRunBranch(ctx, d206Root, "refs/heads/"+runBranch, runTip, headB, "job_b", 1); err != nil {
		t.Fatalf("D206 integrate sibling B: %v", err)
	}
	d206Tree := gitRun(t, d206Root, "rev-parse", "refs/heads/"+runBranch+"^{tree}")

	// --- Branch 2: the opt-in barrier assembly over the SAME staged contributions. ---
	barrierRoot := t.TempDir()
	bbase := gitInit(t, barrierRoot)
	// Reproduce the same two sibling commits off the same base content. gitInit
	// seeds an identical tree (seed.txt), so the frozen tips have identical trees,
	// and the per-sibling commits add the same disjoint files.
	bHeadA := commitSiblingOffBase(t, barrierRoot, bbase, "docs/a.txt", "A\n")
	bHeadB := commitSiblingOffBase(t, barrierRoot, bbase, "docs/b.txt", "B\n")

	repoID := "repo_barrier_equiv"
	runID := "run_barrier_equiv"
	barrierID := "barrier_equiv"
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
	// Stage both siblings (co-transactional in production; here we stage directly,
	// which exercises the merge-base contamination check against the frozen base).
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_a", "job_a", bHeadA, 1); err != nil {
		t.Fatalf("stage sibling A: %v", err)
	}
	if _, err := stageFaninContribution(ctx, tx, barrierRoot, repoID, barrierID, runID, "author_b", "job_b", bHeadB, 1); err != nil {
		t.Fatalf("stage sibling B: %v", err)
	}
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil || !ready {
		t.Fatalf("barrier should be ready with both siblings staged: ready=%v err=%v", ready, err)
	}
	barrierTree, _, edges, err := assembleFaninBarrier(ctx, tx, barrierRoot, repoID, barrierID)
	if err != nil {
		t.Fatalf("assemble barrier: %v", err)
	}
	_ = tx.Rollback(ctx)

	// THE EQUIVALENCE: the barrier's deterministic assembly produces the SAME tree
	// as the D206 per-completion merge. Both contain seed.txt + docs/a.txt +
	// docs/b.txt with identical content, so their tree OIDs are byte-identical.
	if barrierTree != d206Tree {
		barrierFiles := gitRun(t, barrierRoot, "ls-tree", "-r", "--name-only", barrierTree)
		d206Files := gitRun(t, d206Root, "ls-tree", "-r", "--name-only", d206Tree)
		t.Fatalf("barrier assembly tree %s != D206 per-completion tree %s\nbarrier files:\n%s\nD206 files:\n%s",
			barrierTree, d206Tree, barrierFiles, d206Files)
	}

	// The manifest records both seats as staged_live at seal=1, in canonical order.
	if len(edges) != 2 {
		t.Fatalf("manifest should have 2 in-edges, got %d", len(edges))
	}
	if edges[0].EntityID != "author_a" || edges[1].EntityID != "author_b" {
		t.Fatalf("manifest in-edges not in canonical workflow_job_id order: %+v", edges)
	}
	for _, e := range edges {
		if e.Status != "staged_live" || e.Seal != 1 {
			t.Fatalf("manifest in-edge %s should be staged_live at seal 1, got %+v", e.EntityID, e)
		}
	}
}

// beginBarrierTx opens a real transaction runner for the barrier helpers (which
// take db.TxRunner). The caller rolls it back so the seeded rows do not persist.
func beginBarrierTx(t *testing.T, ctx context.Context, pool *db.Pool) db.TxRunner {
	t.Helper()
	tx, err := pool.Runner.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin barrier tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

// stageRow inserts a staging contribution row directly (no git ref / merge-base
// check), exercising the PG side of the barrier in isolation for the readiness and
// tombstone tests. The same row stageFaninContribution writes after its git +
// merge-base steps.
func stageRow(t *testing.T, ctx context.Context, tx db.TxRunner, repoID, barrierID, runID, workflowJobID, jobID string, attempt int) {
	t.Helper()
	ref := stagingRef(runID, workflowJobID, attempt)
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.barrier_staged_contributions
		  (repository_id, barrier_id, run_id, workflow_job_id, attempt, job_id, staging_ref, commit_sha, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'staged')
		ON CONFLICT (repository_id, barrier_id, workflow_job_id, attempt) DO UPDATE
		  SET status = 'staged', voided_at = NULL, void_reason = NULL`,
		repoID, barrierID, runID, workflowJobID, attempt, jobID, ref,
		"1111111111111111111111111111111111111111"); err != nil {
		t.Fatalf("stage row for seat %s attempt %d: %v", workflowJobID, attempt, err)
	}
}

// commitSiblingOffBase forks a detached worktree from base, writes path=content,
// commits, and returns the new commit sha — a fan-in sibling's disjoint-scope
// commit off the frozen tip. The worktree is removed afterward.
func commitSiblingOffBase(t *testing.T, repoRoot, base, path, content string) string {
	t.Helper()
	wtRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_"+strings.ReplaceAll(path, "/", "_")))
	wt := filepath.Join(repoRoot, filepath.FromSlash(wtRel))
	gitRun(t, repoRoot, "worktree", "add", "--detach", wt, base)
	full := filepath.Join(wt, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	gitRun(t, wt, "add", path)
	gitRun(t, wt, "commit", "-q", "-m", "sibling "+path)
	head := gitRevParse(t, wt, "HEAD")
	gitRun(t, repoRoot, "worktree", "remove", "--force", wt)
	return head
}
