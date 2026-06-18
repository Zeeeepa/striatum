package mutations

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// barrierRunEntityEnabled reports whether the RFC 0135 P6 run-entity barrier gates
// run.integrate (the default). STRIATUM_BARRIER_RUN_ENTITY=0 forces the legacy
// terminal-state-only gate — the recoverable kill switch the cutover keeps so a
// composition regression is reversible without redeploying older code. Any value
// other than the explicit "0" leaves the barrier ON.
func barrierRunEntityEnabled() bool {
	return os.Getenv("STRIATUM_BARRIER_RUN_ENTITY") != "0"
}

// RFC 0135 P6 (D216) — run.integrate folds in as the RUN-ENTITY sealed barrier
// (entity = run). This is the highest-risk fold (RFC 0135 Risks): run.integrate
// keys on run_id at a HIGHER layer than the per-job staging barrier, so recasting
// it as a run-entity barrier whose in-edges are the run's job-level barriers must
// preserve RFC 0108's per-repo serialization (lockRepo, integrate.go) and the
// integration idempotency (runIntegratedInto) EXACTLY, or it regresses the gate.
//
// NON-BREAKING DISCIPLINE. P6 is a STRUCTURAL RECASTING, not a behavior change. It
// ships behind the equivalence-fixture discipline RFC 0135 mandates for the bet
// folds: nothing here flips a default. Concretely:
//
//   - The run-entity ASSEMBLY (merge-tree → conflict-detection → commit-tree) is
//     factored into assembleRunEntityIntegration, shared verbatim by the live
//     HandleRunIntegrate path and by this barrier — exactly as RFC 0133's
//     barrier_assembly is the JOB-entity's assembly. The assembly is a pure
//     computation: it writes NO refs and mutates NO working tree, so the live
//     path's observable behavior (same integrated tree, same conflict surfacing) is
//     byte-for-byte preserved; only the side-effecting event-append + CAS update-ref
//     remain inline in HandleRunIntegrate.
//   - runEntityBarrierReady composes the run's terminal-acceptable state (the
//     `completed` check at integrate.go) AND every declared job-level sealed barrier
//     having fired, expressed through P0's db.BarrierReadySQL shape (entity_kind
//     = 'run', in-edges = the run's job barriers). It is a READ — it gates nothing
//     live yet.
//   - shadowRunEntityIntegrate is the asserted-equivalent SHADOW path: it computes
//     the SAME integrated tree and the SAME idempotency outcome HandleRunIntegrate
//     produces, so TestRunIntegrateIsTheRunEntityBarrier can prove equivalence
//     BEFORE any caller flips. It does NOT replace the live path.
//
// The honest record (RFC 0135 Risks, verified at main): run.integrate's run_id key
// lives above the staging layer, so the four-caller unification's run fold is a
// design BET. P6 ships the run-entity barrier as a shadow proof rather than flipping
// the live integrate path, because — per the deliverable gate — a half-working
// integrate path that diverges from RFC 0108 is worse than a shadow proof.

const (
	// runEntityBarrierEntityKind is the entity discriminator for the run-entity
	// instance of the sealed expectation barrier (RFC 0135 P6). The run is the
	// entity; its in-edges are the run's job-level barriers.
	runEntityBarrierEntityKind = "run"
	// runEntityBarrierSealColumn is the seal column name for the run-entity
	// instance: the run's monotonic integration epoch (the number of times the run
	// has been integrated into a given target). The predicate's shape is identical
	// to the fan-in (seal=attempt) and revision (seal=review_generation) instances;
	// only the seal column differs. It is validated as a bare SQL identifier by
	// db.BarrierReadySQL, so it must be a legal column name.
	runEntityBarrierSealColumn = "integration_epoch"
)

// runEntityIntegration is the pure, side-effect-free result of the run-entity
// assembly: the resolved endpoints and the computed integration commit. It writes
// no refs, so both the live HandleRunIntegrate path and the run-entity barrier can
// share it without either mutating repository state until its own CAS step.
type runEntityIntegration struct {
	// IntoSHA is the resolved tip of the integration target branch at assembly time
	// (the base the merge was computed against; the CAS update-ref guards on it).
	IntoSHA string
	// BranchSHA is the resolved tip of the run branch being integrated.
	BranchSHA string
	// TreeOID is the merged tree OID merge-tree produced.
	TreeOID string
	// MergeCommit is the commit-tree result with parents (IntoSHA, BranchSHA).
	MergeCommit string
}

// assembleRunEntityIntegration runs the RFC 0108 merge-tree → conflict-detection →
// commit-tree plumbing and returns the computed integration commit WITHOUT touching
// any ref. It is the run-entity barrier's "assembly," shared verbatim with the live
// HandleRunIntegrate path (the same way RFC 0133's barrier_assembly shares its
// merge-tree/CAS plumbing for the job entity). The conflict and plumbing-failure
// error surfaces are byte-identical to RFC 0108's so that factoring this out is a
// pure structural recasting, not a behavior change.
//
// It is pure: it resolves the two endpoints with rev-parse, simulates the 3-way
// merge with `merge-tree --write-tree` (read-only), and computes the merge commit
// with `commit-tree`. commit-tree writes a commit OBJECT (content-addressed,
// unreferenced) but advances no ref and mutates no working tree, so a caller that
// computes-then-discards leaves the repository observably unchanged.
func assembleRunEntityIntegration(ctx context.Context, repoRoot, runID, runBranch, into string) (runEntityIntegration, error) {
	intoSHA, err := integrateRevParse(ctx, repoRoot, into)
	if err != nil {
		return runEntityIntegration{}, err
	}
	branchSHA, err := integrateRevParse(ctx, repoRoot, runBranch)
	if err != nil {
		return runEntityIntegration{}, err
	}

	mergeTree, mergeTreeErr, exit, err := mergeTreeWriteTree(ctx, repoRoot, into, runBranch)
	if err != nil {
		return runEntityIntegration{}, err
	}
	if exit != 0 {
		conflicts := parseMergeTreeConflicts(mergeTree)
		if len(conflicts) > 0 {
			return runEntityIntegration{}, rpc.NewError("merge_conflict", fmt.Sprintf(
				"integrating run branch %q into %q conflicts in %d path(s): %s — resolve the overlap on a branch a maintainer merges (RFC 0108 never auto-resolves); mainline %q is untouched.",
				runBranch, into, len(conflicts), strings.Join(conflicts, ", "), into),
				map[string]any{"conflicting_paths": conflicts, "into": into, "run_branch": runBranch})
		}
		// Non-zero exit but no parseable conflict path: a merge-tree/plumbing
		// failure, NOT a content overlap. Surface it honestly with the raw git
		// output rather than reporting "conflicts in 0 path(s)" (the #327
		// mislabel); mainline is untouched.
		detail := strings.TrimSpace(mergeTree + "\n" + mergeTreeErr)
		return runEntityIntegration{}, rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
			"integrating run branch %q into %q: merge-tree exited %d with no parseable conflict path; mainline %q is untouched: %s",
			runBranch, into, exit, into, detail),
			map[string]any{"into": into, "run_branch": runBranch})
	}
	treeOID := firstLine(mergeTree)
	if treeOID == "" {
		return runEntityIntegration{}, rpc.NewError("git_commit_apply_failed", "merge-tree produced no merged tree", nil)
	}

	message := fmt.Sprintf("striatum: integrate run %s (%s) into %s", runID, runBranch, into)
	commitOut, exit, err := integrateGit(ctx, repoRoot,
		"-c", "user.name=striatum-integrator", "-c", "user.email=integrator@striatum.local",
		"commit-tree", treeOID, "-p", intoSHA, "-p", branchSHA, "-m", message)
	if err != nil {
		return runEntityIntegration{}, err
	}
	if exit != 0 {
		return runEntityIntegration{}, rpc.NewError("git_commit_apply_failed", "commit-tree failed: "+strings.TrimSpace(commitOut), nil)
	}
	mergeCommit := firstLine(commitOut)
	if !isFullGitSHA(mergeCommit) {
		return runEntityIntegration{}, rpc.NewError("git_commit_apply_failed", "commit-tree produced no commit id", nil)
	}
	return runEntityIntegration{
		IntoSHA:     intoSHA,
		BranchSHA:   branchSHA,
		TreeOID:     treeOID,
		MergeCommit: mergeCommit,
	}, nil
}

// runEntityBarrierReadinessSQL wraps P0's entity/seal-generic predicate
// (db.BarrierReadySQL, entity_kind='run', seal=integration_epoch) over the run's
// terminal state and the run's declared job-level barriers, derived in-query. It is
// built ONCE here so the run-entity caller cannot drift from the canonical shape;
// the static guard TestBarrierPredicateHasNoRefCount scans this file and fails the
// build on any bare ref-COUNT(*) / recency-latest barrier shape.
//
// The run-entity barrier composes two facts (RFC 0135 P6):
//
//	(a) the run is in a terminal-acceptable state (state='completed', matching
//	    HandleRunIntegrate's check), AND
//	(b) every declared job-level sealed barrier inside the run has fired.
//
// In the predicate's three-relation shape (all keyed on (entity_kind, entity_id),
// entity_id = run_id):
//
//   - barrier_in_edges  edge   — one in-edge per declared job-level barrier in the
//     run (its fanin_freeze_points row). is_terminal_gap is FALSE: a run-level
//     barrier admits no terminal gap of its own (a quarantined job is a gap WITHIN
//     its job-level barrier, resolved by that barrier's own predicate, not skipped
//     at the run level). A run with no declared job-level barriers (the common
//     single-lane case) has no in-edges; bool_and over an empty set is NULL, treated
//     as NOT-ready by composition with the terminal-state gate below in Go.
//   - entity_live_seal  live   — the run's LIVE integration epoch, COALESCEd from
//     the count of prior run.integrated events for the run into the target. The seal
//     advances monotonically on each integration; a not-yet-integrated run has epoch
//     0.
//   - staged_contrib    staged — per declared job-level barrier, whether that
//     barrier has fired (a committed barrier_state row). A FIRED job barrier carries
//     the run's live seal; an unfired one carries the never-equal sentinel (-1), so
//     the disjunct staged.seal = live.seal is FALSE (fail-closed) for an outstanding
//     job barrier.
//
// The barrier id binds as $1 and the entity kind as $2 (the predicate's contract);
// $3 binds the repository id, $4 the run id, scoping every CTE. Readiness here is
// the JOB-BARRIER composition half only; the terminal-state half is applied in Go
// (runEntityBarrierReady) so this query stays a faithful instance of the P0 shape.
func runEntityBarrierReadinessSQL() (string, error) {
	predicate, err := db.BarrierReadySQL(db.BarrierSpec{
		EntityKind:         runEntityBarrierEntityKind,
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         runEntityBarrierSealColumn,
	})
	if err != nil {
		return "", err
	}
	// The CTEs supply the predicate's three relations for the run entity. The run's
	// declared job-level barriers (its fanin_freeze_points rows) expand into one
	// in-edge per job barrier; the run's live seal is its integration epoch (count of
	// prior run.integrated events); the staged contribution per job barrier is its
	// committed barrier_state, projected to the run's live seal when fired.
	return `WITH run_epoch AS (
    SELECT COUNT(*)::int AS integration_epoch
      FROM striatumd.events
     WHERE repository_id = $3 AND run_id = $4 AND event_type = 'run.integrated'
), declared AS (
    SELECT '` + runEntityBarrierEntityKind + `'::text AS entity_kind,
           $4::text AS entity_id,
           fp.barrier_id AS job_barrier_id
      FROM striatumd.fanin_freeze_points fp
     WHERE fp.repository_id = $3 AND fp.run_id = $4
), barrier_in_edges AS (
    -- One in-edge per declared job-level barrier in the run. A run-level barrier
    -- admits no terminal gap of its own (gaps live inside each job barrier).
    SELECT d.entity_kind,
           d.entity_id,
           $1::text AS barrier_id,
           d.job_barrier_id,
           false AS is_terminal_gap
      FROM declared d
), entity_live_seal AS (
    -- The run's LIVE integration epoch, one row per declared in-edge so the
    -- predicate's INNER JOIN never DROPS a job barrier.
    SELECT d.entity_kind,
           d.entity_id,
           d.job_barrier_id,
           (SELECT integration_epoch FROM run_epoch) AS integration_epoch
      FROM declared d
), staged_contrib AS (
    -- Per declared job barrier: the run's live seal when the job barrier has FIRED
    -- (a committed barrier_state row), else the never-equal sentinel (-1) so an
    -- outstanding job barrier fails the disjunct CLOSED (FALSE, not NULL).
    SELECT d.entity_kind,
           d.entity_id,
           d.job_barrier_id,
           CASE WHEN bs.state = 'committed'
                THEN (SELECT integration_epoch FROM run_epoch)
                ELSE -1
           END AS integration_epoch
      FROM declared d
      LEFT JOIN striatumd.barrier_state bs
        ON bs.repository_id = $3 AND bs.barrier_id = d.job_barrier_id
)
` + predicate, nil
}

// runEntityBarrierReady evaluates the run-entity sealed barrier (RFC 0135 P6): the
// run is in a terminal-acceptable state AND every declared job-level barrier in the
// run has fired. It is a READ — it gates nothing live (the live integrate path is
// HandleRunIntegrate); it is the composition half of the P6 recasting, used by the
// equivalence proof and available to the doctor/status surface.
//
// The terminal-state gate (state='completed') is applied in Go so the SQL stays a
// faithful instance of the P0 db.BarrierReadySQL shape (the predicate's contract is
// "every in-edge has a live-seal staged contribution or is a terminal gap"; the
// run's own terminal state is an entity-level precondition, not a per-in-edge fact).
// A run with no declared job-level barriers (the common single-lane case) has no
// in-edges; bool_and over an empty set is NULL, which runEntityBarrierReady treats
// as "the job-barrier composition is vacuously satisfied" so a single-lane run's
// barrier reduces to its terminal-state gate — matching HandleRunIntegrate, which
// integrates any completed run regardless of whether it declared a fan-in.
func runEntityBarrierReady(ctx context.Context, runner db.TxRunner, repositoryID, runID string) (bool, error) {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, true)
	if err != nil {
		return false, err
	}
	if state := fmt.Sprint(run["state"]); state != "completed" {
		return false, nil
	}
	query, err := runEntityBarrierReadinessSQL()
	if err != nil {
		return false, err
	}
	barrierID := runEntityBarrierID(runID)
	row := runner.QueryRow(ctx, query, barrierID, runEntityBarrierEntityKind, repositoryID, runID)
	var jobBarriersSatisfied *bool
	if err := row.Scan(&jobBarriersSatisfied); err != nil {
		return false, fmt.Errorf("run-entity barrier readiness query: %w", err)
	}
	// A NULL bool_and means no declared job-level barriers (empty in-edge set): the
	// job-barrier composition is vacuously satisfied, so the run-entity barrier
	// reduces to its terminal-state gate (already passed above).
	if jobBarriersSatisfied == nil {
		return true, nil
	}
	return *jobBarriersSatisfied, nil
}

// runEntityBarrierID is the run-entity barrier's id: the run-scoped identifier the
// predicate binds as $1. The run IS the entity, so the barrier id is derived from
// the run id (one run-entity barrier per run).
func runEntityBarrierID(runID string) string {
	return "run-entity:" + runID
}

// shadowRunEntityIntegrate is the asserted-equivalent SHADOW of HandleRunIntegrate
// (RFC 0135 P6). It computes the run-entity barrier's integration outcome — the
// SAME merged tree OID, the SAME merge commit, and the SAME idempotency outcome — as
// the live HandleRunIntegrate path, WITHOUT advancing the mainline ref or appending
// an event. It is the equivalence-fixture engine TestRunIntegrateIsTheRunEntityBarrier
// drives: the run-entity barrier is proven to produce the identical result before any
// caller flips onto it.
//
// It reuses the EXACT shared assembly (assembleRunEntityIntegration) HandleRunIntegrate
// now uses, and the EXACT idempotency oracle (runIntegratedInto), so the shadow cannot
// drift from the live path. It is read-only with respect to repository ref state.
type shadowIntegrateOutcome struct {
	Status      string // already_integrated | would_integrate
	IntoSHA     string
	TreeOID     string
	MergeCommit string
}

func shadowRunEntityIntegrate(ctx context.Context, runner db.TxRunner, repoRoot, repositoryID, runID, runBranch, into string) (shadowIntegrateOutcome, error) {
	// Same idempotency oracle the live path consults: a run already integrated into
	// this target is a no-op, returning the prior merge commit.
	if prior, err := runIntegratedInto(ctx, runner, repositoryID, runID, into); err != nil {
		return shadowIntegrateOutcome{}, err
	} else if prior != "" {
		return shadowIntegrateOutcome{Status: "already_integrated", MergeCommit: prior}, nil
	}
	asm, err := assembleRunEntityIntegration(ctx, repoRoot, runID, runBranch, into)
	if err != nil {
		return shadowIntegrateOutcome{}, err
	}
	return shadowIntegrateOutcome{
		Status:      "would_integrate",
		IntoSHA:     asm.IntoSHA,
		TreeOID:     asm.TreeOID,
		MergeCommit: asm.MergeCommit,
	}, nil
}
