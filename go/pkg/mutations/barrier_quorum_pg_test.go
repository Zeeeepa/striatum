package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// --- quorum test seeding helpers -------------------------------------------

// seedQuorumRun seeds a repo + run whose workflow snapshot carries the panel: one
// downstream gate job that depends on N review seats. Each review seat's job
// definition carries its panel_role; the gate job carries max_gating_abstentions.
// The denominator (the gate's review-seat dependencies) is FROZEN in the snapshot.
func seedQuorumRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, gateJobWF string, seatRoles map[string]string, maxGatingAbstentions int) {
	t.Helper()
	jobs := []map[string]any{}
	for seatWF, role := range seatRoles {
		def := map[string]any{"id": seatWF, "type": "review"}
		if role != "" {
			def["panel_role"] = role
		}
		jobs = append(jobs, def)
	}
	gateDef := map[string]any{"id": gateJobWF, "type": "synthesis"}
	if maxGatingAbstentions != 0 {
		gateDef["max_gating_abstentions"] = maxGatingAbstentions
	}
	jobs = append(jobs, gateDef)
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"jobs":        jobs,
	})
}

// seedReviewSeatJob inserts a review job row for a seat at an attempt/state — the
// live-seal source the quorum JOINs against. The jobs UNIQUE key
// (repository_id, run_id, workflow_job_id, attempt) means a requeue is a distinct row.
func seedReviewSeatJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, seatWF, state string, attempt int) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, attempt, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'Review','review','reviewer',$5,$6,$7,$8)`,
		repoID, jobID, runID, seatWF, state, attempt, "idem_"+jobID, now); err != nil {
		t.Fatalf("insert review seat job %s (attempt %d): %v", jobID, attempt, err)
	}
}

// seedSeatVerdict inserts a verdict row for a seat's job. session_id must exist.
func seedSeatVerdict(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, verdictID, jobID, sessionID, verdict string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict, created_at, posture
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'neutral')`,
		repoID, verdictID, runID, jobID, sessionID, verdict, now); err != nil {
		t.Fatalf("insert verdict %s: %v", verdictID, err)
	}
}

// quorumGateJobID seeds the downstream gate job row (a phase_synthesis) and its
// dependency edges onto each seat's job, returning the gate job_id.
func seedQuorumGateAndDeps(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, gateJobID, gateJobWF string, seatJobIDs map[string]string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, attempt, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'Gate','synthesis','adjudicator','blocked',1,$5,$6)`,
		repoID, gateJobID, runID, gateJobWF, "idem_"+gateJobID, now); err != nil {
		t.Fatalf("insert gate job: %v", err)
	}
	for _, seatJobID := range seatJobIDs {
		seedDependency(t, ctx, runner, repoID, gateJobID, seatJobID, map[string]any{
			"requires_verdict": []string{"accept", "accept_with_findings"},
		})
	}
}

// quorumReady opens a tx and evaluates panelQuorumSatisfied for the gate job.
func quorumReady(t *testing.T, ctx context.Context, pool *db.Pool, repoID, gateJobID string) bool {
	t.Helper()
	tx := beginBarrierTx(t, ctx, pool)
	ready, err := panelQuorumSatisfied(ctx, tx, repoID, gateJobID)
	if err != nil {
		t.Fatalf("panelQuorumSatisfied: %v", err)
	}
	_ = tx.Rollback(ctx)
	return ready
}

// --- tests -----------------------------------------------------------------

// TestPanelQuorumOverFrozenDenominatorBudgetZero is the default-behavior fence:
// with max_gating_abstentions=0 (the default), EVERY declared gating seat must carry a
// live-seal accepting verdict for the barrier to fire — exactly the edge-by-edge
// behavior, so the default is unchanged. The barrier must NOT fire while any gating
// seat is unsatisfied, and must fire only once all gating seats accept at their live
// attempt.
func TestPanelQuorumOverFrozenDenominatorBudgetZero(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_budget0"
	runID := "run_quorum_budget0"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_a": "gating",
		"rev_b": "gating",
	}, 0)
	// Both seats complete at attempt 1; both have an active session for the verdict FK.
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_b", "reviewer", "lane_b", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_b1", "rev_b", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_a": "job_a1", "rev_b": "job_b1",
	})

	// Seat A accepts; seat B has no verdict yet → quorum NOT satisfied (budget 0).
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a", "job_a1", "sess_a", "accept")
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must NOT fire with seat B unsatisfied at budget 0 (every gating seat required)")
	}

	// Seat B accepts at its live attempt → all gating seats satisfied → quorum fires.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_b", "job_b1", "sess_b", "accept_with_findings")
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must fire once every gating seat has a live-seal accepting verdict")
	}
}

// TestPanelQuorumStaleSealAcceptDoesNotSatisfy is the trap-killer fence for the quorum
// caller: a seat's accepting verdict at a SUPERSEDED attempt does not satisfy the
// barrier once the seat's live seal advances (a requeue). staged.attempt = live.attempt
// makes the stale-seal accept structurally invisible — never satisfied by recency.
func TestPanelQuorumStaleSealAcceptDoesNotSatisfy(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_staleseal"
	runID := "run_quorum_staleseal"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{"rev_a": "gating"}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{"rev_a": "job_a1"})

	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a1", "job_a1", "sess_a", "accept")
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must fire with the only gating seat accepting at its live attempt")
	}

	// THE TRAP: seat is requeued (its live attempt advances to 2) but the only accepting
	// verdict is the STALE attempt-1 one. The barrier must drop back to NOT-ready.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a2", "rev_a", "completed", 2)
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must NOT fire on a stale attempt-1 accept after the seat advanced to attempt 2 (RFC 0135 trap)")
	}

	// Once the seat accepts at its live attempt 2, the barrier fires again.
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, "sess_a2", "reviewer", "lane_a", nil, "active", 2)
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a2", "job_a2", "sess_a2", "accept")
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must fire once the seat accepts at its live attempt 2")
	}
}

// TestQuorumStubHoldsSeatWithoutVerdict is D214(a): a daemon-authored abstention stub
// holds a seat / raises the frozen denominator but carries NO verdict value — it is
// classified `abstain`, never accept/reject, and cannot satisfy staged.seal = live.seal.
// We model the stub as a declared gating seat with a live job row but NO verdict (the
// verdict-LESS seat-occupancy). It raises the denominator (the gate now waits on it)
// but is never satisfied, so the quorum at budget 0 must NOT fire while it is held.
func TestQuorumStubHoldsSeatWithoutVerdict(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_stub"
	runID := "run_quorum_stub"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_a": "gating",
		"rev_stub": "gating",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	// The abstention stub: a declared gating seat with a held job row (raising the
	// denominator) but NO verdict value — verdict-less seat occupancy.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_stub1", "rev_stub", "running", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_a": "job_a1", "rev_stub": "job_stub1",
	})

	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a", "job_a1", "sess_a", "accept")

	// The stub holds a seat in the denominator but carries no satisfying contribution,
	// so the quorum at budget 0 must NOT fire — the stub is `abstain`, not `accept`.
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("D214a: a verdict-less abstention stub must NOT satisfy the quorum — it holds a seat, it does not vote")
	}

	// Cross-check the never-fabricate-a-vote axiom at the resolver level: the stub seat
	// resolves with NO accepting verdict and is NOT satisfied.
	decl, err := resolveQuorumDeclaration(ctx, beginBarrierTx(t, ctx, pool), repoID, gateID)
	if err != nil {
		t.Fatalf("resolve declaration: %v", err)
	}
	if len(decl.Seats) != 2 {
		t.Fatalf("denominator should include the stub seat (frozen denominator raised): got %v", decl.Seats)
	}
	tx := beginBarrierTx(t, ctx, pool)
	seats, err := resolvePanelSeats(ctx, tx, repoID, gateID, decl)
	if err != nil {
		t.Fatalf("resolve seats: %v", err)
	}
	_ = tx.Rollback(ctx)
	for _, s := range seats {
		if s.WorkflowJobID == "rev_stub" {
			if s.AcceptingAttempt != -1 {
				t.Fatalf("D214a: the stub seat must carry NO accepting verdict value (AcceptingAttempt=-1), got %d", s.AcceptingAttempt)
			}
			if s.satisfied() {
				t.Fatal("D214a: the verdict-less stub seat must NOT be satisfied")
			}
		}
	}
}

// TestQuorumSkipsOnlyProvablyDeadSeat is D214(b): a structurally_unrecoverable
// (provably-dead) gating seat is quorum-skippable WITHIN the abstention budget, but a
// LIVE gating seat blocks the barrier — quorum may skip a DEAD seat, never a SLOW one
// (silence from a live lane is not consent).
func TestQuorumSkipsOnlyProvablyDeadSeat(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_dead"
	runID := "run_quorum_dead"
	gateWF := "adjudicate"
	gateID := "job_gate"

	// Budget 1: one gating seat may be terminally-absent (provably dead) and the
	// barrier still fires.
	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_a":    "gating",
		"rev_dead": "gating",
	}, 1)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_dead", "reviewer", "lane_dead", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	// The dead seat: a live job row at attempt 1, but its owning lane is provably dead.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_dead1", "rev_dead", "running", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_a": "job_a1", "rev_dead": "job_dead1",
	})
	// Bind the dead seat's job to its session via a lease so the oracle can resolve the
	// owning session's supervisor pointer.
	bindLeaseToJob(t, ctx, runner, repoID, runID, "job_dead1", "sess_dead", "lease_dead")

	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a", "job_a1", "sess_a", "accept")

	// First: the dead seat's lane is NOT yet provably dead (no pointer / attached) → it
	// is a LIVE gating seat and BLOCKS even at budget 1 (silence != consent).
	seedSupervisorPointer(t, ctx, runner, repoID, runID, "sess_dead", "attached", 0)
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("D214b: a LIVE gating seat must block the barrier even within the abstention budget (silence != consent)")
	}

	// Now mark the seat's lane provably dead (a `lost` pointer is conclusive in
	// supervisedAgentConfirmedDead). It becomes structurally_unrecoverable and is
	// skippable within budget 1 → the barrier fires.
	retargetSupervisorPointerState(t, ctx, runner, repoID, "sess_dead", "lost")
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("D214b: a provably-dead gating seat within the abstention budget must be skippable so the barrier fires")
	}
}

// TestQuorumDeadSeatBeyondBudgetStillBlocks proves the budget is a CEILING on
// provably-dead skips: a dead gating seat beyond the abstention budget still blocks
// (budget 0, one dead seat → not skippable).
func TestQuorumDeadSeatBeyondBudgetStillBlocks(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_deadbudget0"
	runID := "run_quorum_deadbudget0"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_a":    "gating",
		"rev_dead": "gating",
	}, 0) // budget 0 — no dead-seat skips allowed
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_dead", "reviewer", "lane_dead", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_dead1", "rev_dead", "running", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_a": "job_a1", "rev_dead": "job_dead1",
	})
	bindLeaseToJob(t, ctx, runner, repoID, runID, "job_dead1", "sess_dead", "lease_dead")
	seedSupervisorPointer(t, ctx, runner, repoID, runID, "sess_dead", "lost", 0)
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_a", "job_a1", "sess_a", "accept")

	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("a provably-dead gating seat BEYOND the abstention budget (0) must still block the barrier")
	}
}

// TestQuorumLiveDissentBlocksAcrossRecovery is the #339 forward-write fence: a live
// dissent_ledger row blocks the quorum at the seat's live attempt, and it BLOCKS even
// where recovery would otherwise move the lineage — because the ledger keys on the
// STABLE workflow_job_id and is sealed at the live attempt. It also proves the
// forward-write happens co-transactionally with a needs_revision verdict.
func TestQuorumLiveDissentBlocksAcrossRecovery(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_dissent"
	runID := "run_quorum_dissent"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{"rev_a": "gating"}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_a", "reviewer", "lane_a", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a1", "rev_a", "completed", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{"rev_a": "job_a1"})

	// Forward-write a dissent at the seat's live attempt 1, co-transactionally with a
	// (needs_revision) verdict, via the same recordDissent path the apply-verdict path
	// uses. We call recordDissent directly here to exercise the forward-write contract.
	tx := beginBarrierTx(t, ctx, pool)
	job, err := rowByID(ctx, tx, repoID, "jobs", "job_id", "job_a1", false)
	if err != nil {
		t.Fatalf("load seat job: %v", err)
	}
	if err := recordDissent(ctx, tx, repoID, job, "sess_a", "v_dissent", "needs_revision"); err != nil {
		t.Fatalf("recordDissent: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit dissent: %v", err)
	}

	// A live dissent at the seat's live attempt 1 BLOCKS the quorum even if an accepting
	// verdict were later recorded at the same attempt (the dissent is the seal-durable
	// blocking witness). Record an accept at attempt 1; the dissent must still block.
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_accept", "job_a1", "sess_a", "accept")
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("#339: a live dissent_ledger row at the seat's live attempt must BLOCK the quorum even with a same-attempt accept")
	}

	// Simulate recovery moving the seat's lineage to a NEW attempt 2 (a fresh job_id).
	// The dissent at attempt 1 is now at a SUPERSEDED seal, so it no longer blocks; the
	// seat needs a fresh accept at the live attempt 2 to satisfy.
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_a2", "rev_a", "completed", 2)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, "sess_a2", "reviewer", "lane_a", nil, "active", 2)
	if quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("after recovery to attempt 2 the seat has no live-seal accept yet — quorum must NOT fire")
	}
	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_accept2", "job_a2", "sess_a2", "accept")
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("once the seat accepts at the live attempt 2 (past the superseded dissent), quorum must fire")
	}
}

// TestQuorumAdvisorySeatExcludedFromDenominator proves advisory seats are excluded
// from the gating denominator: a panel with one gating seat (accepted) and one advisory
// seat (no verdict) fires, because the advisory seat does not count toward the quorum.
func TestQuorumAdvisorySeatExcludedFromDenominator(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoID := "repo_quorum_advisory"
	runID := "run_quorum_advisory"
	gateWF := "adjudicate"
	gateID := "job_gate"

	seedQuorumRun(t, ctx, runner, repoID, runID, gateWF, map[string]string{
		"rev_gate": "gating",
		"rev_adv":  "advisory",
	}, 0)
	intgSeedSession(t, ctx, runner, repoID, runID, "sess_g", "reviewer", "lane_g", nil, "active")
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_g1", "rev_gate", "completed", 1)
	seedReviewSeatJob(t, ctx, runner, repoID, runID, "job_adv1", "rev_adv", "running", 1)
	seedQuorumGateAndDeps(t, ctx, runner, repoID, runID, gateID, gateWF, map[string]string{
		"rev_gate": "job_g1", "rev_adv": "job_adv1",
	})

	seedSeatVerdict(t, ctx, runner, repoID, runID, "v_g", "job_g1", "sess_g", "accept")

	decl, err := resolveQuorumDeclaration(ctx, beginBarrierTx(t, ctx, pool), repoID, gateID)
	if err != nil {
		t.Fatalf("resolve declaration: %v", err)
	}
	if len(decl.Seats) != 1 || decl.Seats[0] != "rev_gate" {
		t.Fatalf("advisory seat must be EXCLUDED from the gating denominator; got %v", decl.Seats)
	}
	if !quorumReady(t, ctx, pool, repoID, gateID) {
		t.Fatal("quorum must fire with the only gating seat accepting; the advisory seat does not count")
	}
}

// TestDissentLedgerIsAppendOnly is the #339 immutability fence (mirroring the freeze
// table's TestSealedBarrierFreezePointIsImmutable): an UPDATE or DELETE of a dissent
// row as the runtime role striatumd_rw is rejected by the BEFORE UPDATE/DELETE
// refuse-trigger AND the SELECT/INSERT-only grant. A recorded dissent is a structurally
// immutable, auditable witness no code path (or operator) can quietly retract.
func TestDissentLedgerIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	privileged, unprivileged := pgtest.Pools(t)

	repoID := "repo_dissent_immutable"
	runID := "run_dissent_immutable"

	intgSeedRepo(t, ctx, privileged.Runner, repoID)
	intgSeedRun(t, ctx, privileged.Runner, repoID, runID, map[string]any{"workflow_id": "wf"})
	seedReviewSeatJob(t, ctx, privileged.Runner, repoID, runID, "job_a1", "rev_a", "completed", 1)

	// Insert the dissent as the unprivileged runtime role (INSERT is granted).
	tx, err := unprivileged.Runner.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin unprivileged tx: %v", err)
	}
	job, err := rowByID(ctx, tx, repoID, "jobs", "job_id", "job_a1", false)
	if err != nil {
		t.Fatalf("load seat job: %v", err)
	}
	if err := recordDissent(ctx, tx, repoID, job, "sess_a", "v_d", "reject"); err != nil {
		t.Fatalf("insert dissent as striatumd_rw should be allowed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit dissent: %v", err)
	}

	// UPDATE as striatumd_rw must be rejected (no UPDATE grant + refuse-trigger).
	if err := unprivileged.Runner.Exec(ctx, `
		UPDATE striatumd.dissent_ledger SET verdict = 'needs_revision'
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err == nil {
		t.Fatal("UPDATE of an immutable dissent row as striatumd_rw must be rejected")
	}
	// DELETE as striatumd_rw must be rejected.
	if err := unprivileged.Runner.Exec(ctx, `
		DELETE FROM striatumd.dissent_ledger
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err == nil {
		t.Fatal("DELETE of an immutable dissent row as striatumd_rw must be rejected")
	}

	// The row is unchanged (read via the privileged pool to confirm).
	row, err := oneRow(ctx, privileged.Runner, `
		SELECT verdict FROM striatumd.dissent_ledger
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read dissent row: %v", err)
	}
	if got := stringField(t, row, "verdict"); got != "reject" {
		t.Fatalf("dissent row was mutated despite immutability: verdict = %q", got)
	}
}

func stringField(t *testing.T, row map[string]any, key string) string {
	t.Helper()
	v, ok := row[key]
	if !ok || v == nil {
		t.Fatalf("row missing %q", key)
	}
	return v.(string)
}

// bindLeaseToJob attaches an active lease (owner_session_id) to a job so the quorum's
// structural-unrecoverability resolver can find the seat's owning session's pointer.
func bindLeaseToJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, sessionID, leaseID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease %s: %v", leaseID, err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_lease_id = $1
		 WHERE repository_id = $2 AND job_id = $3`, leaseID, repoID, jobID); err != nil {
		t.Fatalf("bind lease to job %s: %v", jobID, err)
	}
}
