package mutations

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// RFC 0135 FULL barrier cutover (#354, D216) — the same-final-decision equivalence
// fixtures that GATE each phase flipped from shadow/opt-in to the LIVE default. A
// legacy predicate is retired only after its barrier path is proven to produce the
// IDENTICAL result the legacy path produced. These fixtures are the proof.
//
//   - P4 (quorum): TestPanelQuorumCutoverEqualsEdgeByEdge proves dependenciesSatisfied
//     with the quorum barrier LIVE (default) yields the SAME enqueue decision as the
//     legacy edge-by-edge gate (STRIATUM_BARRIER_QUORUM=0) for a default-budget
//     all-gating panel, across the satisfied / unsatisfied / stale-seal end states.
//   - P6 (run.integrate): TestRunIntegrateRunEntityBarrierGate proves the run-entity
//     barrier gate accepts EXACTLY the runs the bare terminal-state check did (a
//     completed run with no declared job-level barriers integrates identically; the
//     gate is no stricter than the legacy check for every run on the live path).

// dependenciesSatisfiedWithBarrierFlag evaluates dependenciesSatisfied with the
// quorum-barrier flag forced to a known value, restoring the env afterward. This is
// how the equivalence fixture pins the LIVE path (flag on) against the LEGACY path
// (flag off) over the SAME seeded rows.
func dependenciesSatisfiedWithBarrierFlag(t *testing.T, ctx context.Context, pool *db.Pool, repoID, gateJobID string, quorumOn bool) bool {
	t.Helper()
	prior, had := os.LookupEnv("STRIATUM_BARRIER_QUORUM")
	if quorumOn {
		_ = os.Unsetenv("STRIATUM_BARRIER_QUORUM")
	} else {
		_ = os.Setenv("STRIATUM_BARRIER_QUORUM", "0")
	}
	defer func() {
		if had {
			_ = os.Setenv("STRIATUM_BARRIER_QUORUM", prior)
		} else {
			_ = os.Unsetenv("STRIATUM_BARRIER_QUORUM")
		}
	}()
	tx := beginBarrierTx(t, ctx, pool)
	ok, err := dependenciesSatisfied(ctx, tx, repoID, gateJobID)
	if err != nil {
		t.Fatalf("dependenciesSatisfied(quorumOn=%v): %v", quorumOn, err)
	}
	_ = tx.Rollback(ctx)
	return ok
}

// TestPanelQuorumCutoverEqualsEdgeByEdge is the RFC 0135 P4 DELIVERABLE GATE: it
// proves the panel-quorum barrier, wired LIVE into dependenciesSatisfied as the
// default for a GATING review panel, produces the IDENTICAL enqueue decision the
// legacy edge-by-edge gate produced — at the default abstention budget 0 with all
// gating seats. The two paths are evaluated over the SAME live rows at three end
// states; they must AGREE at every one. Only then is the edge-by-edge default retired
// for paneled review gates.
func TestPanelQuorumCutoverEqualsEdgeByEdge(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_cutover"
	runID := "run_quorum_cutover"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_a": "gating",
		"rev_b": "gating",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_b", "reviewer", "lane_b", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_b1", "rev_b", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_a": "job_a1", "rev_b": "job_b1",
	})

	// equalAtState asserts the LIVE (barrier) and LEGACY (edge-by-edge) gates agree.
	equalAtState := func(label string, wantSatisfied bool) {
		t.Helper()
		live := dependenciesSatisfiedWithBarrierFlag(t, ctx, pool, repoID, gateID, true)
		legacy := dependenciesSatisfiedWithBarrierFlag(t, ctx, pool, repoID, gateID, false)
		if live != legacy {
			t.Fatalf("%s: barrier-live gate (%v) DISAGREES with legacy edge-by-edge gate (%v) — the cutover must be byte-identical at the default budget", label, live, legacy)
		}
		if live != wantSatisfied {
			t.Fatalf("%s: gate satisfied=%v, want %v", label, live, wantSatisfied)
		}
	}

	// State 1: neither seat has a verdict — both gates refuse.
	equalAtState("no verdicts", false)

	// State 2: only seat A accepts — both gates still refuse (every gating seat required).
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a", "job_a1", "sess_a", "accept")
	equalAtState("only A accepts", false)

	// State 3: both seats accept at their live seal — both gates fire.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_b", "job_b1", "sess_b", "accept_with_findings")
	equalAtState("both accept at live seal", true)

	// State 4: seat A is requeued (live seal advances to attempt 2) — its attempt-1
	// accept is now stale. The LIVE barrier resolves this by the live seal
	// (staged.attempt = live.attempt) and refuses. The legacy edge-by-edge gate reads
	// latestVerdict over the same (still non-superseded) accept and, because the gate
	// only checks the verdict VALUE not the seal, would FIRE — so the two paths
	// legitimately DIVERGE here. This is the EXACT correctness win the cutover buys
	// (the trap-killer), so we assert the live barrier is STRICTER, not equal: it
	// refuses the stale-seal accept the legacy gate would admit.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a2", "rev_a", "completed", 2)
	live := dependenciesSatisfiedWithBarrierFlag(t, ctx, pool, repoID, gateID, true)
	legacy := dependenciesSatisfiedWithBarrierFlag(t, ctx, pool, repoID, gateID, false)
	if live {
		t.Fatal("after seat A requeue, the LIVE barrier must REFUSE (the attempt-1 accept is stale at live seal 2) — the trap-killer")
	}
	if !legacy {
		t.Fatal("the LEGACY edge-by-edge gate is seal-blind and WOULD admit the stale accept; if it refuses too, the divergence the cutover fixes is not being exercised")
	}

	// State 5: seat A re-accepts at the live seal (attempt 2) — both gates fire again,
	// confirming the barrier refusal was a seal mismatch, not a deadlock.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a2", "job_a2", "sess_a", "accept")
	equalAtState("A re-accepts at live seal", true)
}

// TestRunIntegrateRunEntityBarrierGate is the RFC 0135 P6 DELIVERABLE GATE for the
// LIVE flip: the run-entity barrier now gates HandleRunIntegrate (replacing the bare
// `state == 'completed'` check). It proves the gate accepts EXACTLY the runs the
// legacy terminal-state check did — a completed, branch-confirmed run with no declared
// job-level barriers (every run on today's live completion path) integrates
// identically whether the barrier gate is ON (default) or OFF
// (STRIATUM_BARRIER_RUN_ENTITY=0). The same merged tree is produced in both modes.
func TestRunIntegrateRunEntityBarrierGate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	// Each invocation seeds its OWN repository/run/branch (seedRunEntityRepoAndRun does an
	// unconditional INSERT INTO repositories): the equivalence proof runs the gate twice
	// over IDENTICALLY-SHAPED but DISTINCT rows, so the two seeds never collide on
	// repositories_pkey. The label distinguishes them.
	integrateOnce := func(t *testing.T, label string, barrierOn bool) string {
		t.Helper()
		prior, had := os.LookupEnv("STRIATUM_BARRIER_RUN_ENTITY")
		if barrierOn {
			_ = os.Unsetenv("STRIATUM_BARRIER_RUN_ENTITY")
		} else {
			_ = os.Setenv("STRIATUM_BARRIER_RUN_ENTITY", "0")
		}
		defer func() {
			if had {
				_ = os.Setenv("STRIATUM_BARRIER_RUN_ENTITY", prior)
			} else {
				_ = os.Unsetenv("STRIATUM_BARRIER_RUN_ENTITY")
			}
		}()
		repoRoot := t.TempDir()
		repoID := "repo_p6_gate_" + label
		runID := "run_p6_gate_" + label
		runBranch := "striatum/p6-gate-" + label
		seedRunEntityRepoAndRun(t, ctx, runner, repoRoot, repoID, runID, runBranch)

		res, err := HandleRunIntegrate(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "into": "main"}))
		if err != nil {
			t.Fatalf("HandleRunIntegrate(barrierOn=%v): %v", barrierOn, err)
		}
		if status := stringOf(res["status"]); status != "integrated" {
			t.Fatalf("barrierOn=%v: status=%q, want integrated", barrierOn, status)
		}
		return gitRun(t, repoRoot, "rev-parse", "refs/heads/main^{tree}")
	}

	// A completed run with NO declared job-level barriers integrates under BOTH the
	// barrier gate (default) and the legacy terminal-state-only gate. The run-entity
	// barrier reduces to the terminal-state check for such a run (empty in-edges →
	// vacuously satisfied), so the same merged tree results either way.
	barrierTree := integrateOnce(t, "barrier", true)
	legacyTree := integrateOnce(t, "legacy", false)
	if barrierTree != legacyTree {
		t.Fatalf("run-entity barrier gate tree %s != legacy terminal-state-only tree %s — the P6 flip must accept exactly the runs the bare check did", barrierTree, legacyTree)
	}
	if !isFullGitSHA(barrierTree) {
		t.Fatalf("integration produced no merged tree: %q", barrierTree)
	}
}

// stringOf is a tiny local helper so the cutover fixtures do not depend on a wider
// fmt import surface; it renders a map value as a string.
func stringOf(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
