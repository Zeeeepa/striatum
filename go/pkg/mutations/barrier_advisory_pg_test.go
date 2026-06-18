package mutations

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// --- advisory guard test helpers (RFC 0132 Layer C / #341) -------------------

// advisoryTallyFor opens a tx and resolves the advisory tally for the gate.
func advisoryTallyFor(t *testing.T, ctx context.Context, pool *db.Pool, repoID, gateJobID string) advisoryTally {
	t.Helper()
	tx := beginBarrierTx(t, ctx, pool)
	tally, err := resolveAdvisoryTally(ctx, tx, repoID, gateJobID)
	if err != nil {
		t.Fatalf("resolveAdvisoryTally: %v", err)
	}
	_ = tx.Rollback(ctx)
	return tally
}

// runAdvisoryGuards opens a tx, runs applyAdvisoryGuards for the recorded seat, and
// commits, so the blocker rows persist for assertion.
func runAdvisoryGuards(t *testing.T, ctx context.Context, pool *db.Pool, repoID, recordedJobID string) {
	t.Helper()
	tx := beginBarrierTx(t, ctx, pool)
	if err := applyAdvisoryGuards(ctx, tx, repoID, recordedJobID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("applyAdvisoryGuards: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit advisory guards: %v", err)
	}
}

// openAdvisoryGuardBlockerCount returns how many open advisory-guard blockers of a
// kind sit on a gate job.
func openAdvisoryGuardBlockerCount(t *testing.T, ctx context.Context, runner db.Runner, repoID, gateJobID, kind string) int {
	t.Helper()
	rows, err := collectRowsMut(t, ctx, runner, `
		SELECT blocker_id FROM striatumd.blockers
		 WHERE repository_id = $1 AND job_id = $2 AND blocker_kind = $3 AND state = 'open'`,
		repoID, gateJobID, kind)
	if err != nil {
		t.Fatalf("count blockers: %v", err)
	}
	return len(rows)
}

func collectRowsMut(t *testing.T, ctx context.Context, runner db.Runner, sql string, args ...any) ([]map[string]any, error) {
	t.Helper()
	return queryRows(ctx, runner, sql, args...)
}

// --- tests -------------------------------------------------------------------

// TestAdvisorySilentAcceptFiresNoGuard is the silent-accept fence: a gating seat
// that accepts and an advisory seat that also accepts fire NO advisory guard — no
// blocker is opened (advisory: silent accept).
func TestAdvisorySilentAcceptFiresNoGuard(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_adv_silent"
	runID := "run_adv_silent"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_gate": "gating",
		"rev_adv":  "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_g", "reviewer", "lane_g", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_adv", "reviewer", "lane_adv", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g1", "rev_gate", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv1", "rev_adv", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_gate": "job_g1", "rev_adv": "job_adv1",
	})
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_g", "job_g1", "sess_g", "accept")
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_adv", "job_adv1", "sess_adv", "accept")

	runAdvisoryGuards(t, ctx, pool, repoID, "job_adv1")

	for _, kind := range []string{
		advisoryBlockerKindCorroborated, advisoryBlockerKindUnanimousReject, advisoryBlockerKindUngrounded,
	} {
		if n := openAdvisoryGuardBlockerCount(t, ctx, runner, repoID, gateID, kind); n != 0 {
			t.Fatalf("silent accept must fire no advisory guard; got %d %s blockers", n, kind)
		}
	}
}

// TestAdvisoryCorroboratedAbstention is guard 1: a gating abstention co-occurring
// with an advisory needs_revision reclassifies the abstention to must_escalate and
// opens an operator-resolvable blocker on the gate (the abstention does not silently
// fall into the budget).
func TestAdvisoryCorroboratedAbstention(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_adv_corrob"
	runID := "run_adv_corrob"
	gateWF := "adjudicate"
	gateID := "job_gate"

	// Two gating seats; one accepts, the other abstains (no verdict). One advisory
	// seat raises needs_revision — corroborating the gating gap.
	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_g1":  "gating",
		"rev_g2":  "gating",
		"rev_adv": "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_g1", "reviewer", "lane_g1", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_adv", "reviewer", "lane_adv", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g1", "rev_g1", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g2", "rev_g2", "running", 1) // abstaining
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv1", "rev_adv", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_g1": "job_g1", "rev_g2": "job_g2", "rev_adv": "job_adv1",
	})
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_g1", "job_g1", "sess_g1", "accept")
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_adv", "job_adv1", "sess_adv", "needs_revision")

	tally := advisoryTallyFor(t, ctx, pool, repoID, gateID)
	if !tally.HasGatingAbstention {
		t.Fatal("expected a gating abstention (one gating seat has no live-seal accept)")
	}
	decision := evaluateAdvisoryGuards(tally)
	if decision.GuardFired != advisoryBlockerKindCorroborated {
		t.Fatalf("expected advisory_corroborated_abstention; got %q", decision.GuardFired)
	}

	runAdvisoryGuards(t, ctx, pool, repoID, "job_adv1")
	if n := openAdvisoryGuardBlockerCount(t, ctx, runner, repoID, gateID, advisoryBlockerKindCorroborated); n != 1 {
		t.Fatalf("expected exactly one corroborated-abstention blocker; got %d", n)
	}
	// Idempotent: a second pass does not duplicate the blocker.
	runAdvisoryGuards(t, ctx, pool, repoID, "job_adv1")
	if n := openAdvisoryGuardBlockerCount(t, ctx, runner, repoID, gateID, advisoryBlockerKindCorroborated); n != 1 {
		t.Fatalf("advisory guard blocker must be idempotent; got %d", n)
	}
}

// TestAdvisoryUnanimousRejectBlocksUnderGatingAccept is guard 2: every submitted
// advisory seat rejecting blocks finalize even under full gating accept.
func TestAdvisoryUnanimousRejectBlocksUnderGatingAccept(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_adv_unanimous"
	runID := "run_adv_unanimous"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_gate": "gating",
		"rev_a1":   "advisory",
		"rev_a2":   "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_g", "reviewer", "lane_g", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a1", "reviewer", "lane_a1", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a2", "reviewer", "lane_a2", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g1", "rev_gate", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a1", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a2", "rev_a2", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_gate": "job_g1", "rev_a1": "job_a1", "rev_a2": "job_a2",
	})
	// The gating seat accepts; both advisory seats reject.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_g", "job_g1", "sess_g", "accept")
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a1", "job_a1", "sess_a1", "reject")
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a2", "job_a2", "sess_a2", "reject")

	// The gating quorum is satisfied (the gating seat accepted) — but the advisory
	// guard must still block finalize.
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("precondition: the gating quorum should be satisfied under the gating accept")
	}
	tally := advisoryTallyFor(t, ctx, pool, repoID, gateID)
	if !tally.unanimousAdvisoryReject() {
		t.Fatal("expected unanimous advisory reject")
	}
	if evaluateAdvisoryGuards(tally).GuardFired != advisoryBlockerKindUnanimousReject {
		t.Fatal("expected unanimous_advisory_reject guard")
	}

	runAdvisoryGuards(t, ctx, pool, repoID, "job_a2")
	if n := openAdvisoryGuardBlockerCount(t, ctx, runner, repoID, gateID, advisoryBlockerKindUnanimousReject); n != 1 {
		t.Fatalf("expected one unanimous-advisory-reject blocker; got %d", n)
	}
	// The gate is now held: dependenciesSatisfied must refuse to enqueue it despite
	// the gating quorum being satisfied.
	tx := beginBarrierTx(t, ctx, pool)
	satisfied, err := dependenciesSatisfied(ctx, tx, repoID, gateID)
	if err != nil {
		t.Fatalf("dependenciesSatisfied: %v", err)
	}
	_ = tx.Rollback(ctx)
	if satisfied {
		t.Fatal("a gate held by an open advisory guard must NOT be dependency-satisfied (advisory stops the line)")
	}
}

// TestAdvisoryOnlyPanelUngrounded is guard 3: a gate whose panel has zero gating
// seats but at least one advisory seat blocks unconditionally.
func TestAdvisoryOnlyPanelUngrounded(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_adv_ungrounded"
	runID := "run_adv_ungrounded"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_adv": "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_adv", "reviewer", "lane_adv", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv1", "rev_adv", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_adv": "job_adv1",
	})
	// Even an advisory ACCEPT is ungrounded: there is no gating ground at all.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_adv", "job_adv1", "sess_adv", "accept")

	tally := advisoryTallyFor(t, ctx, pool, repoID, gateID)
	if tally.GatingSeatCount != 0 {
		t.Fatalf("expected zero gating seats; got %d", tally.GatingSeatCount)
	}
	if evaluateAdvisoryGuards(tally).GuardFired != advisoryBlockerKindUngrounded {
		t.Fatal("expected advisory_only_panel_ungrounded guard")
	}

	runAdvisoryGuards(t, ctx, pool, repoID, "job_adv1")
	if n := openAdvisoryGuardBlockerCount(t, ctx, runner, repoID, gateID, advisoryBlockerKindUngrounded); n != 1 {
		t.Fatalf("expected one ungrounded blocker; got %d", n)
	}
}

// TestAdvisoryStaleSealVerdictExcludedFromTally is the seal trap-killer applied to
// the advisory tally: an advisory seat's prior-round reject (at a superseded attempt)
// does not count once the seat advances; only its current-seal verdict tallies.
func TestAdvisoryStaleSealVerdictExcludedFromTally(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_adv_staleseal"
	runID := "run_adv_staleseal"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_gate": "gating",
		"rev_adv":  "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_g", "reviewer", "lane_g", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_adv", "reviewer", "lane_adv", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g1", "rev_gate", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv1", "rev_adv", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_gate": "job_g1", "rev_adv": "job_adv1",
	})
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_g", "job_g1", "sess_g", "accept")
	// The advisory seat rejected at attempt 1 ...
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_adv1", "job_adv1", "sess_adv", "reject")
	if !advisoryTallyFor(t, ctx, pool, repoID, gateID).unanimousAdvisoryReject() {
		t.Fatal("precondition: a single rejecting advisory seat is a unanimous reject at attempt 1")
	}
	// ... but the seat advances to attempt 2 (a revision round). The attempt-1 reject
	// is now at a superseded seal and must NOT tally.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv2", "rev_adv", "running", 2)
	tally := advisoryTallyFor(t, ctx, pool, repoID, gateID)
	if tally.hasAdvisoryDissent() {
		t.Fatal("a superseded-seal advisory reject must NOT tally as a current advisory dissent")
	}
	if tally.unanimousAdvisoryReject() {
		t.Fatal("the advisory seat has no current-seal verdict; unanimous-reject must not fire on a stale-seal reject")
	}
}
