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
	"github.com/halbritt/striatum/go/pkg/rpc"
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
// discipline fence — the same-final-tree EQUIVALENCE FIXTURE. It proves the
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

	// --- Branch 2: the barrier assembly over the SAME staged contributions. ---
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

// TestFaninStagingRefusesSmuggledContent is the RFC 0133 Risks / #352 fence: the
// merge-base ancestry check (gitIsAncestor(frozen, staged)) proves the staged commit
// DESCENDS from the frozen tip, but a staged ref of `frozen_tip + linear commits`
// passes it even when an intermediate MERGE commit grafts in content the sibling
// never authored off the frozen base. The per-commit tree-provenance check
// (assertContributionTreeProvenance) closes that smuggled-content hole: a
// contribution whose chain pulls in an off-base lineage is REFUSED with
// barrier_smuggled_content, while a clean linear contribution still passes.
func TestFaninStagingRefusesSmuggledContent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	root := t.TempDir()
	base := gitInit(t, root) // the frozen tip
	baseTree := gitRevParse(t, root, base+"^{tree}")

	repoID := "repo_barrier_smuggle"
	runID := "run_barrier_smuggle"
	barrierID := "barrier_smuggle"
	cleanSeat := "author_clean"
	smuggleSeat := "author_smuggle"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_clean", cleanSeat, "completed", 1)
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_smuggle", smuggleSeat, "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	// Freeze the barrier over the real frozen tip AND its sealed tree sha, so the
	// per-commit tree-provenance check has its trusted tree anchor.
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            base,
		FrozenTipTreeSHA:        baseTree,
		DeclaredSiblingJobIDs:   []string{cleanSeat, smuggleSeat},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}

	// --- Clean case: a linear contribution authored on top of the frozen tip. ---
	cleanHead := commitSiblingOffBase(t, root, base, "docs/clean.txt", "clean\n")
	if _, err := stageFaninContribution(ctx, tx, root, repoID, barrierID, runID, cleanSeat, "job_clean", cleanHead, 1); err != nil {
		t.Fatalf("clean linear contribution must pass tree-provenance: %v", err)
	}

	// --- Smuggled case: a commit that DESCENDS from the frozen tip (so it passes the
	// merge-base ancestry check) but folds in an off-base side lineage via a merge,
	// smuggling content the sibling never authored off the frozen base. ---
	// (a) c1: a clean commit off the frozen tip (so the merge descends from frozen).
	c1 := commitOnTop(t, root, base, "docs/smuggle.txt", "legit\n", "c1 off frozen")
	// (b) offBase: an INDEPENDENT root commit — a foreign lineage that never
	//     descended from the frozen tip (the "evolved-tip / cherry-picked" content).
	//     A top-level path keeps the flat `git mktree` spec valid (no nested tree).
	offBase := commitDetachedRoot(t, root, "smuggled_evil.txt", "smuggled\n", "off-base root")
	// (c) merge c1 + offBase. The merge descends from the frozen tip via c1, so
	//     gitIsAncestor(frozen, merge) is TRUE — the ancestry check is satisfied — but
	//     its tree now contains the smuggled off-base content.
	smuggleHead := commitMerge(t, root, c1, offBase, "merge: smuggle off-base content")

	// Sanity: the smuggled head really does pass the ancestry check the old code
	// relied on, so this test exercises the gap the new check closes.
	if ok, err := gitIsAncestor(ctx, root, base, smuggleHead); err != nil || !ok {
		t.Fatalf("precondition: smuggled head must descend from frozen tip (ancestry must pass): ok=%v err=%v", ok, err)
	}

	_, err := stageFaninContribution(ctx, tx, root, repoID, barrierID, runID, smuggleSeat, "job_smuggle", smuggleHead, 1)
	if err == nil {
		t.Fatal("smuggled-content contribution (off-base merge) must be REFUSED by the per-commit tree-provenance check")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok {
		t.Fatalf("smuggled-content refusal must be an *rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != "barrier_smuggled_content" {
		t.Fatalf("smuggled-content refusal must carry code barrier_smuggled_content, got %q (err: %v)", rpcErr.Code, err)
	}
	_ = tx.Rollback(ctx)
}

// TestFaninStagingRecoversBaseDrift is the RFC 0133 #353 base-drift-as-a-recoverable-
// leg fence. A LEGITIMATE base drift — the run branch evolved under a sibling's feet,
// so the staged sibling forked off an evolved/rebased base that is a SIBLING of the
// frozen tip (both children of a common ancestor) rather than a descendant of it —
// must be RECOVERED, not refused: the staging row records the recoverable rebase leg
// (base_drift_onto_sha = the merge-base) and the assembly folds the contribution onto
// the frozen tip with an extra commit-tree parent, preserving BOTH the frozen-line
// content and the evolved-base content. (The old code refused any non-descendant with
// git_commit_apply_failed "base drift / contaminated base".)
func TestFaninStagingRecoversBaseDrift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	root := t.TempDir()
	common := gitInit(t, root) // the common ancestor C (carries seed.txt)
	// The frozen tip F is a child of C carrying f.txt (the run-branch tip at fan-out).
	frozen := commitOnTop(t, root, common, "docs/f.txt", "frozen line\n", "frozen tip")
	frozenTree := gitRevParse(t, root, frozen+"^{tree}")
	// The evolved base E is a SIBLING of F (also a child of C) carrying e.txt — the
	// run branch advanced/rebased onto a different line under the sibling's feet.
	evolved := commitOnTop(t, root, common, "docs/e.txt", "evolved line\n", "evolved base")
	// The staged sibling forks off the evolved base E and adds its own disjoint file.
	driftHead := commitOnTop(t, root, evolved, "docs/sib.txt", "sibling work\n", "sibling off evolved base")

	repoID := "repo_barrier_drift"
	runID := "run_barrier_drift"
	barrierID := "barrier_drift"
	seat := "author_drift"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_drift", seat, "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            frozen,
		FrozenTipTreeSHA:        frozenTree,
		DeclaredSiblingJobIDs:   []string{seat},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}

	// Precondition: the staged commit does NOT descend from the frozen tip (a true
	// drift, so the OLD refusal path would have fired), but it DOES share the common
	// ancestor C as a merge-base (the recoverable signal).
	if ok, err := gitIsAncestor(ctx, root, frozen, driftHead); err != nil || ok {
		t.Fatalf("precondition: drift head must NOT descend from frozen tip: ok=%v err=%v", ok, err)
	}
	mb, err := gitMergeBase(ctx, root, frozen, driftHead)
	if err != nil || mb != common {
		t.Fatalf("precondition: merge-base(frozen, drift) must be the common ancestor %s, got %s err=%v", common, mb, err)
	}

	// The recoverable drift must STAGE (not refuse) and record the rebase leg.
	if _, err := stageFaninContribution(ctx, tx, root, repoID, barrierID, runID, seat, "job_drift", driftHead, 1); err != nil {
		t.Fatalf("recoverable base drift must stage, not refuse: %v", err)
	}
	row, err := oneRow(ctx, tx, `
		SELECT COALESCE(base_drift_onto_sha,'') AS onto, COALESCE(base_drift_reason,'') AS reason
		  FROM striatumd.barrier_staged_contributions
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = $3 AND attempt = 1`,
		repoID, barrierID, seat)
	if err != nil {
		t.Fatalf("read drift-leg row: %v", err)
	}
	if got := fmt.Sprint(row["onto"]); got != common {
		t.Fatalf("base_drift_onto_sha should record the recoverable rebase leg (merge-base %s), got %q", common, got)
	}
	if got := fmt.Sprint(row["reason"]); got != "evolved_run_branch" {
		t.Fatalf("base_drift_reason should be evolved_run_branch, got %q", got)
	}

	// The barrier is ready (the single seat is live-staged) and assembles cleanly,
	// folding the drift onto the frozen tip and preserving BOTH lines' content.
	if ready, err := faninBarrierReady(ctx, tx, repoID, barrierID); err != nil || !ready {
		t.Fatalf("barrier should be ready with the recovered drift staged: ready=%v err=%v", ready, err)
	}
	assembledTree, assembledCommit, edges, err := assembleFaninBarrier(ctx, tx, root, repoID, barrierID)
	if err != nil {
		t.Fatalf("assemble recovered drift: %v", err)
	}
	// The manifest in-edge records the recovered drift leg.
	if len(edges) != 1 || edges[0].BaseDriftOnto != common {
		t.Fatalf("manifest in-edge should record base drift onto %s, got %+v", common, edges)
	}
	// The assembled tree contains the frozen line AND the evolved-base line AND the
	// sibling's own work — base drift preserves the intervening run-branch evolution
	// (the #299 invariant) rather than reverting it.
	files := gitRun(t, root, "ls-tree", "-r", "--name-only", assembledTree)
	for _, want := range []string{"seed.txt", "docs/f.txt", "docs/e.txt", "docs/sib.txt"} {
		if !strings.Contains(files, want) {
			t.Fatalf("assembled drift tree missing %q (base drift must preserve both lines); tree:\n%s", want, files)
		}
	}
	// The assembly commit folds the staged drift as an extra parent (the recoverable
	// extra-parent leg the RFC names) and frozen tip as the first parent.
	parents := strings.Fields(gitRun(t, root, "rev-list", "--parents", "-n", "1", assembledCommit))
	if len(parents) != 3 || parents[1] != frozen || parents[2] != driftHead {
		t.Fatalf("assembled commit must fold drift as an extra commit-tree parent (-p frozen -p drift), got parents %v", parents)
	}
	_ = tx.Rollback(ctx)
}

// TestFaninStagingRefusesContaminatedBaseDrift is the contamination fence that the
// recoverable-drift relaxation (#353) must NOT widen: a staged commit that does not
// descend from the frozen tip AND folds in an off-base FOREIGN root lineage (the #352
// smuggled-content shape, but reached via a non-descendant base) is still REFUSED with
// barrier_smuggled_content — it shares no legitimate lineage the assembly could rebase
// onto. This keeps the #352 guarantee intact while #353 recovers legit drift.
func TestFaninStagingRefusesContaminatedBaseDrift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	root := t.TempDir()
	common := gitInit(t, root)
	frozen := commitOnTop(t, root, common, "docs/f.txt", "frozen line\n", "frozen tip")
	frozenTree := gitRevParse(t, root, frozen+"^{tree}")

	repoID := "repo_barrier_contam"
	runID := "run_barrier_contam"
	barrierID := "barrier_contam"
	driftSeat := "author_legit_drift"
	contamSeat := "author_contaminated"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_legit", driftSeat, "completed", 1)
	seedFaninSeatJob(t, ctx, runner, repoID, runID, "job_contam", contamSeat, "completed", 1)

	tx := beginBarrierTx(t, ctx, pool)
	if err := recordFaninFreezePoint(ctx, tx, repoID, faninFreezePoint{
		BarrierID:               barrierID,
		RunID:                   runID,
		DownstreamWorkflowJobID: "synth",
		FrozenTipSHA:            frozen,
		FrozenTipTreeSHA:        frozenTree,
		DeclaredSiblingJobIDs:   []string{driftSeat, contamSeat},
	}); err != nil {
		t.Fatalf("record freeze point: %v", err)
	}

	// A legitimate evolved-base drift (sibling of the frozen tip) must STILL stage —
	// the contamination tightening must not refuse a real drift.
	evolved := commitOnTop(t, root, common, "docs/e.txt", "evolved\n", "evolved base")
	legitHead := commitOnTop(t, root, evolved, "docs/legit.txt", "legit\n", "legit drift")
	if _, err := stageFaninContribution(ctx, tx, root, repoID, barrierID, runID, driftSeat, "job_legit", legitHead, 1); err != nil {
		t.Fatalf("legitimate evolved-base drift must still stage: %v", err)
	}

	// A contaminated base: a commit that does NOT descend from the frozen tip and
	// folds in an INDEPENDENT off-base root via a merge (foreign lineage). It shares
	// no legitimate base with the frozen tip's lineage — its history carries a root
	// the frozen tip does not — so it must be REFUSED, not recovered.
	c1 := commitOnTop(t, root, common, "docs/sidebase.txt", "side\n", "off-frozen sibling base")
	offBase := commitDetachedRoot(t, root, "smuggled_evil.txt", "smuggled\n", "off-base root")
	contamHead := commitMerge(t, root, c1, offBase, "merge: smuggle off-base content")

	// Precondition: the contaminated head does NOT descend from the frozen tip (so it
	// reaches the new drift classifier, not the direct #352 path).
	if ok, err := gitIsAncestor(ctx, root, frozen, contamHead); err != nil || ok {
		t.Fatalf("precondition: contaminated head must NOT descend from frozen tip: ok=%v err=%v", ok, err)
	}

	_, err := stageFaninContribution(ctx, tx, root, repoID, barrierID, runID, contamSeat, "job_contam", contamHead, 1)
	if err == nil {
		t.Fatal("a contaminated base (off-base foreign root, non-descendant) must be REFUSED, not recovered as a drift")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok {
		t.Fatalf("contamination refusal must be an *rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != "barrier_smuggled_content" {
		t.Fatalf("contamination refusal must carry code barrier_smuggled_content, got %q (err: %v)", rpcErr.Code, err)
	}
	_ = tx.Rollback(ctx)
}

// commitOnTop commits path=content on top of parent (a single-parent commit) and
// returns the new commit sha, without touching any branch ref.
func commitOnTop(t *testing.T, repoRoot, parent, path, content, msg string) string {
	t.Helper()
	wt := filepath.Join(repoRoot, ".striatum", "worktrees", "wt_ontop_"+strings.ReplaceAll(path, "/", "_"))
	gitRun(t, repoRoot, "worktree", "add", "--detach", wt, parent)
	full := filepath.Join(wt, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	gitRun(t, wt, "add", path)
	gitRun(t, wt, "commit", "-q", "-m", msg)
	head := gitRevParse(t, wt, "HEAD")
	gitRun(t, repoRoot, "worktree", "remove", "--force", wt)
	return head
}

// commitDetachedRoot creates an INDEPENDENT root commit (no parent, an orphan
// lineage that does not descend from any existing commit) carrying a single file at
// path=content, via git plumbing, and returns its sha. This is the "foreign /
// off-base" content a smuggling merge folds into an otherwise-linear chain. Using
// plumbing (hash-object / mktree / commit-tree) avoids orphan-worktree fragility.
func commitDetachedRoot(t *testing.T, repoRoot, path, content, msg string) string {
	t.Helper()
	// Write the blob via a temp file (gitRun does not pipe stdin).
	tmp := filepath.Join(t.TempDir(), "blob")
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("write blob temp: %v", err)
	}
	blob := gitRun(t, repoRoot, "hash-object", "-w", tmp)
	treeSpec := fmt.Sprintf("100644 blob %s\t%s\n", blob, path)
	tmpTree := filepath.Join(t.TempDir(), "treespec")
	if err := os.WriteFile(tmpTree, []byte(treeSpec), 0o644); err != nil {
		t.Fatalf("write tree spec: %v", err)
	}
	tree := mktreeFromFile(t, repoRoot, tmpTree)
	return gitRun(t, repoRoot, "commit-tree", tree, "-m", msg)
}

// mktreeFromFile runs `git mktree` reading the tree spec from a file (gitRun does
// not pipe stdin), returning the written tree oid.
func mktreeFromFile(t *testing.T, repoRoot, specPath string) string {
	t.Helper()
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read tree spec: %v", err)
	}
	cmd := exec.Command("git", "mktree")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(string(spec))
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git mktree: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitMerge creates a merge commit with parents p1 (first parent) and p2, using
// a detached worktree off p1, and returns the merge commit sha. The resulting commit
// descends from p1 (and thus from anything p1 descends from) while folding p2's
// content into its tree — the smuggling shape.
func commitMerge(t *testing.T, repoRoot, p1, p2, msg string) string {
	t.Helper()
	wt := filepath.Join(repoRoot, ".striatum", "worktrees", "wt_merge")
	gitRun(t, repoRoot, "worktree", "add", "--detach", wt, p1)
	// Allow unrelated histories so the orphan off-base lineage can be merged in.
	gitRun(t, wt, "merge", "--no-ff", "--no-edit", "--allow-unrelated-histories", "-m", msg, p2)
	head := gitRevParse(t, wt, "HEAD")
	gitRun(t, repoRoot, "worktree", "remove", "--force", wt)
	return head
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
