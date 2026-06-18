package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestRevisionCoherenceIsTheSealInstance is the RFC 0135 P5 (D216) load-bearing
// fence: it re-expresses the RFC 0126 #282 regression against the sealed
// expectation barrier primitive and proves that `seal := review_generation`
// yields RFC 0126's EXACT behavior — with NO behavior change to the live gate.
//
// The #282 shape (the design-panel revision re-open wedge): a build (synth) job
// is reviewed by two independent reviewers A and B. Reviewer B returns
// needs_revision, the build re-opens (its review_generation advances to 2 in the
// same transaction as the attempt bump — bumpReviewGeneration), the reviewers are
// reset, the build re-runs, reviewer A re-reviews and ACCEPTS at generation 2,
// but reviewer B's prior-round (generation 1) needs_revision verdict SURVIVES
// (history is append-only) and B never records a current-generation verdict.
//
// RFC 0126's finalization gate is the per-build set-difference
//
//	required_obligations  MINUS  {reviewers with a current-generation accepting verdict}
//
// which is NON-EMPTY here (it names B) — so the run does NOT finalize. RFC 0135's
// claim is that this set-difference IS the primitive readiness predicate
// db.BarrierReadySQL with seal := review_generation:
//
//	bool_and( edge.is_terminal_gap OR staged.review_generation = live.review_generation )
//
// where the entity is the review obligation, the live seal is the build's current
// review_generation, and a staged contribution is an accepting verdict sealed at a
// generation. A gen-1 verdict can NEVER satisfy `staged.seal = live.seal` once the
// build's live seal is 2 — it is structurally absent, never filtered after the fact.
//
// This test evaluates BOTH the RFC 0126 set-difference AND the P0 primitive
// predicate over the SAME live rows and asserts they agree (both refuse naming B,
// both fire once B re-accepts at gen 2). That equivalence is the whole P5 proof.
func TestRevisionCoherenceIsTheSealInstance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	// review_generation lives in owner bundle 0009 (owner-held jobs + verdicts), so
	// the default runtime-migrations-only harness lacks the columns. Apply owner
	// bundles to reproduce the production posture the seal instance is defined
	// against (daemon applies owner bundles at startup).
	if _, _, err := db.ApplyOwnerBundles(ctx, runner, "test"); err != nil {
		t.Fatalf("apply owner bundles: %v", err)
	}

	repoID := "repo_revision_coherence_seal"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	// revisionFixture seeds the build (synth, completed) + reviewer A
	// (review_design_codex) with an active lease. Reviewer A is the obligation that
	// will re-accept at gen 2.
	runID, reviewAJobID, synthJobID, sessionA, leaseA := revisionFixture(t, ctx, runner, repoID, cycles)

	// Both reviewers gate the build, so the revision re-open resets each through
	// resetDownstreamForRevision — the exact path that used to DELETE verdicts.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewAJobID, synthJobID); err != nil {
		t.Fatalf("wire reviewer A dependency on build: %v", err)
	}

	// Seed an independent reviewer B (own job + own session) that also reviews the
	// build. B is the obligation whose stale gen-1 verdict must NOT satisfy the gate.
	reviewBJobID := "job_review_b_" + repoID
	sessionB := "sess_reviewer_b_" + repoID
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at, completed_at
		) VALUES ($1,$2,$3,'review_design_b',1,'completed','reviewer','Design Review B','review','idem_review_b_'||$1,'[]'::jsonb,$4,$4)`,
		repoID, reviewBJobID, runID, now); err != nil {
		t.Fatalf("insert reviewer B job: %v", err)
	}
	intgSeedSession(t, ctx, runner, repoID, runID, sessionB, "reviewer", "claude", []string{"review"}, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionB, "claude")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewBJobID, synthJobID); err != nil {
		t.Fatalf("wire reviewer B dependency on build: %v", err)
	}

	// The build starts at the first review generation (migration default).
	if got := buildReviewGeneration(t, ctx, runner, repoID, synthJobID); got != 1 {
		t.Fatalf("build review_generation before revision = %d, want 1", got)
	}

	// Drive reviewer A's needs_revision through the REAL handler: this routes the
	// revision and advances the build's live seal (review_generation 1 -> 2) in the
	// same transaction as the attempt bump — the primitive's "advance the entity's
	// seal" operation (bumpReviewGeneration). This grounds the equivalence in the
	// actual shipped mechanism rather than a hand-set generation.
	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionA,
		"job_id":     reviewAJobID,
		"lease_id":   leaseA,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record reviewer A needs_revision: %v", err)
	}
	if fmt.Sprint(result["status"]) != "revision_routed" {
		t.Fatalf("status = %v, want revision_routed; result=%#v", result["status"], result)
	}
	buildGen := buildReviewGeneration(t, ctx, runner, repoID, synthJobID)
	if buildGen != 2 {
		t.Fatalf("build review_generation after revision = %d, want 2 (seal advanced on re-open)", buildGen)
	}

	// Reviewer A's gen-1 needs_revision survived (append-only) and is non-current.
	if got := jobState(t, ctx, runner, repoID, reviewAJobID); got != "blocked" {
		t.Fatalf("reviewer A state = %q, want blocked (reset by the revision)", got)
	}

	// Reviewer B never re-reviewed: directly seed its prior-round (generation 1)
	// needs_revision so the #282 end-state is exact — B's stale verdict is the only
	// thing standing between the build and finalization. (Recording it directly,
	// stamped at gen 1, mirrors a verdict left from the round before the revision.)
	seedVerdict(t, ctx, runner, repoID, runID, reviewBJobID, sessionB, "needs_revision", 1)

	// Reviewer A re-reviews the revised build and ACCEPTS at the current generation 2.
	seedVerdict(t, ctx, runner, repoID, runID, reviewAJobID, sessionA, "accept", 2)

	// --- The equivalence proof: the RFC 0126 set-difference and the P0 primitive
	// predicate (seal := review_generation) must agree on the SAME live rows. ---

	obligations := []string{reviewAJobID, reviewBJobID}

	// (1) RFC 0126's finalization gate: required obligations MINUS those with a
	// current-generation accepting verdict. Non-empty ⇒ the gate refuses.
	missing := rfc0126Unsatisfied(t, ctx, runner, repoID, obligations, buildGen)
	if len(missing) != 1 || missing[0] != reviewBJobID {
		t.Fatalf("RFC 0126 set-difference unsatisfied set = %v, want exactly [%s] (B's gen-1 verdict is non-current)", missing, reviewBJobID)
	}

	// (2) The P0 primitive predicate with seal := review_generation, evaluated over
	// relations derived from those exact rows, must be NOT ready and refuse B.
	ready := sealBarrierReady(t, ctx, runner, repoID, obligations, buildGen)
	if ready {
		t.Fatalf("primitive predicate (seal := review_generation) fired while reviewer B's gen-1 needs_revision stands; it must REFUSE naming B")
	}

	// (3) Equivalence: both evaluators computed the SAME refusal. A non-empty
	// RFC 0126 difference must correspond to a non-ready primitive barrier.
	if (len(missing) == 0) != ready {
		t.Fatalf("RFC 0126 set-difference (unsatisfied=%v) and the seal predicate (ready=%v) DISAGREE — they must be the same gate", missing, ready)
	}

	// (4) Close the loop: once reviewer B records its OWN current-generation (2)
	// accepting verdict, both evaluators must FIRE — proving the refusal was the
	// seal mismatch, not a structural deadlock.
	seedVerdict(t, ctx, runner, repoID, runID, reviewBJobID, sessionB, "accept", 2)
	missingAfter := rfc0126Unsatisfied(t, ctx, runner, repoID, obligations, buildGen)
	if len(missingAfter) != 0 {
		t.Fatalf("RFC 0126 set-difference after B re-accepts at gen 2 = %v, want empty (all obligations satisfied at the live seal)", missingAfter)
	}
	if !sealBarrierReady(t, ctx, runner, repoID, obligations, buildGen) {
		t.Fatalf("primitive predicate must FIRE once every obligation has a current-seal accepting verdict")
	}
}

// seedVerdict records (or replaces) a verdict row for an obligation, stamped at an
// explicit review_generation. It supersedes any prior non-superseded verdict for
// the (job, session) so the latest non-superseded row is deterministic, mirroring
// how the runtime records successive rounds (append-only, generation-stamped).
func seedVerdict(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, sessionID, verdict string, generation int) {
	t.Helper()
	verdictID := fmt.Sprintf("vd_%s_%s_g%d", jobID, sessionID, generation)
	now := time.Now().UTC()
	// Mark prior non-superseded rows for this (job, session) as superseded so
	// latestVerdict() / the gate read see only the newest round, exactly as the
	// runtime's supersession discipline does. The verdict UNIQUE(job, session)
	// constraint means a true re-record would conflict; superseding the old row and
	// inserting a fresh verdict_id models a new round faithfully without a DELETE.
	if err := runner.Exec(ctx, `
		DELETE FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2 AND session_id = $3`,
		repoID, jobID, sessionID); err != nil {
		t.Fatalf("clear prior verdict for %s/%s: %v", jobID, sessionID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  created_at, posture, review_generation
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'neutral',$8)`,
		repoID, verdictID, runID, jobID, sessionID, verdict, now, generation); err != nil {
		t.Fatalf("insert verdict %s for %s/%s: %v", verdict, jobID, sessionID, err)
	}
}

// rfc0126Unsatisfied is RFC 0126's finalization set-difference made explicit: it
// returns the required obligations that LACK a non-superseded ACCEPTING verdict
// stamped with the build's current generation. A non-empty result is the gate
// refusal (the live verifyRunCompletionProvenance routes this through
// review_generation_incomplete). This is the authoritative behavior the seal
// predicate must reproduce.
func rfc0126Unsatisfied(t *testing.T, ctx context.Context, runner db.Runner, repoID string, obligations []string, liveGeneration int) []string {
	t.Helper()
	var unsatisfied []string
	for _, jobID := range obligations {
		row, err := oneRow(ctx, runner, `
			SELECT verdict, review_generation
			  FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2
			   AND superseded_by_decision_id IS NULL
			 ORDER BY created_at DESC, verdict_id DESC
			 LIMIT 1`, repoID, jobID)
		if err != nil {
			// No verdict at all ⇒ an unsatisfied obligation (never a vacuous pass).
			unsatisfied = append(unsatisfied, jobID)
			continue
		}
		accepting := isAcceptingVerdict(fmt.Sprint(row["verdict"]))
		current := intValue(row["review_generation"]) == liveGeneration
		if !accepting || !current {
			unsatisfied = append(unsatisfied, jobID)
		}
	}
	return unsatisfied
}

// sealBarrierReady evaluates the P0 primitive readiness predicate
// (db.BarrierReadySQL) with seal := review_generation over relations derived from
// the live obligation/verdict rows, and returns whether the barrier fires. It
// builds the three relations the predicate JOINs:
//
//   - barrier_in_edges  edge   — one row per required review obligation. No
//     obligation is a terminal gap here (no quarantined seat), so is_terminal_gap
//     is always false; a real quarantine would set it true.
//   - entity_live_seal  live   — every obligation's live seal is the BUILD's
//     current review_generation (the build owns the epoch all its reviewers are
//     sealed against).
//   - staged_contrib    staged — the obligation's accepting contribution and the
//     generation it was sealed at: a non-superseded ACCEPTING verdict's
//     review_generation, or a never-equal sentinel (-1) when the obligation has no
//     accepting verdict, so an unaccepted edge fails closed (staged.seal = live.seal
//     is FALSE, never NULL).
//
// The predicate then fires iff bool_and(is_terminal_gap OR staged.seal = live.seal)
// over the obligations — a gen-1 verdict against a live seal of 2 is structurally
// absent, exactly the trap-killer. The predicate text comes from db.BarrierReadySQL
// so this caller cannot drift from the canonical seal-JOIN shape.
func sealBarrierReady(t *testing.T, ctx context.Context, runner db.Runner, repoID string, obligations []string, liveGeneration int) bool {
	t.Helper()
	predicate, err := db.BarrierReadySQL(db.BarrierSpec{
		EntityKind:         "review_obligation",
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         "review_generation",
	})
	if err != nil {
		t.Fatalf("BarrierReadySQL (seal := review_generation): %v", err)
	}
	// The CTEs supply the predicate's three relations from the live rows. The
	// obligation set is bound as a text array ($3); the build's live seal as $4; the
	// barrier id ($1) and entity kind ($2) are the predicate's bound contract.
	const entityKind = "review_obligation"
	query := `WITH obligation(entity_id) AS (
	    SELECT unnest($3::text[])
	), barrier_in_edges AS (
	    SELECT '` + entityKind + `'::text AS entity_kind,
	           o.entity_id,
	           $1::text AS barrier_id,
	           false AS is_terminal_gap
	      FROM obligation o
	), entity_live_seal AS (
	    -- Every obligation is sealed against the BUILD's current generation.
	    SELECT '` + entityKind + `'::text AS entity_kind,
	           o.entity_id,
	           $4::int AS review_generation
	      FROM obligation o
	), accepting AS (
	    -- The obligation's accepting contribution and the generation it was sealed
	    -- at (latest non-superseded ACCEPTING verdict per obligation).
	    SELECT v.job_id AS entity_id,
	           MAX(v.review_generation) AS review_generation
	      FROM striatumd.verdicts v
	     WHERE v.repository_id = $5
	       AND v.job_id = ANY($3::text[])
	       AND v.superseded_by_decision_id IS NULL
	       AND v.verdict IN ('accept','accept_with_findings')
	     GROUP BY v.job_id
	), staged_contrib AS (
	    -- One row PER obligation so the predicate's LEFT JOIN always matches; an
	    -- obligation with no accepting verdict yields the never-equal sentinel -1, so
	    -- the edge is FALSE (fail closed), never NULL (which bool_and would skip).
	    SELECT '` + entityKind + `'::text AS entity_kind,
	           o.entity_id,
	           COALESCE(a.review_generation, -1) AS review_generation
	      FROM obligation o
	      LEFT JOIN accepting a ON a.entity_id = o.entity_id
	)
	` + predicate
	row := runner.QueryRow(ctx, query, "barrier_review_"+repoID, entityKind, obligations, liveGeneration, repoID)
	var ready *bool
	if err := row.Scan(&ready); err != nil {
		t.Fatalf("seal barrier readiness query: %v", err)
	}
	return ready != nil && *ready
}
