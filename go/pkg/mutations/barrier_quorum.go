package mutations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
)

// RFC 0135 P4 (D214/D216) — the panel-quorum instance of the sealed expectation
// barrier (#338/#339/#340). The quorum caller consumes the entity/seal-generic
// predicate P0 minted in db.BarrierReadySQL, with:
//
//   - entity   = review_seat, keyed by the STABLE workflow_job_id (NEVER job_id,
//     which recovery churns: a stalled-transfer reassigns job_id/session_id, a
//     requeue/resume bumps attempt). A recovered/transferred seat maps back to the
//     same entity id, so a verdict recorded before recovery and the seat's live
//     attempt after recovery resolve to the same seat (RFC 0132 Risks; the predicate
//     enforces this by construction since entity_id IS the JOIN key).
//   - seal     = attempt (the seat's live attempt). A prior-attempt verdict is
//     non-current by seal mismatch — exactly staged.attempt = live.attempt.
//   - denominator = the FROZEN declared set of review seats the downstream gate job
//     depends on. The denominator is frozen because it is read from the workflow
//     snapshot's job_dependencies, fixed at run.prepare; a seat cannot be added or
//     dropped after fan-out, so an abstention budget over it is meaningful.
//
// The three D214 ratifications shape the classification:
//
//   - panel_role gating|advisory (default gating). Only GATING seats count toward the
//     quorum and its frozen denominator; advisory seats are excluded entirely here
//     (their guards are the deferred P4b #341). max_gating_abstentions (default 0)
//     is the budget: at 0 every gating seat is required (behavior unchanged).
//   - D214(a) — a daemon-authored abstention stub holds a seat / raises the frozen
//     denominator but carries NO verdict value. It is classified `abstain`, never
//     accept/reject; it CANNOT satisfy staged.attempt = live.attempt because it has no
//     seal-bearing accepting contribution — only a held seat counted against the
//     denominator. The never-fabricate-a-vote axiom: the stub has no verdict field.
//   - D214(b) — a gating seat may be treated as terminally-absent (counted against the
//     abstention budget, an is_terminal_gap edge) ONLY when it is
//     structurally_unrecoverable, bound to the forgery-resistant
//     supervisedAgentConfirmedDead oracle (recovery_decision_tree.go). A LIVE gating
//     seat blocks the barrier — quorum may skip a DEAD seat, never a SLOW one
//     (silence from a live lane is not consent).
//
// A live dissent_ledger row (forward-written co-transactionally with a blocking
// verdict, #339) forces the seat's edge BLOCKING regardless of any later accepting
// verdict at a stale seal — the seal-durable dissent witness, the quorum analog of
// "a superseded-seal contribution is invisible" applied to blocking contributions.
//
// k_of_n quorum-shape generalization is OUT (D214: deferred to lint). The primitive
// carries only all_seats (budget 0) | budget:N.

const (
	// quorumBarrierEntityKind is the entity discriminator for the panel-quorum
	// instance: a review seat, keyed by the stable workflow_job_id.
	quorumBarrierEntityKind = "review_seat"
	// quorumBarrierSealColumn is the seal column for the quorum instance: the seat's
	// live attempt. (Identical in shape to the fan-in instance; the revision-coherence
	// caller, P5, uses review_generation.)
	quorumBarrierSealColumn = "attempt"

	// panelRoleGating is the default per-reviewer panel_role: a gating seat counts
	// toward the quorum and its frozen denominator and blocks the barrier when live
	// and unsatisfied.
	panelRoleGating = "gating"
	// panelRoleAdvisory: an advisory seat is excluded from the quorum denominator;
	// its dissent does not block (the advisory guards are the deferred P4b #341).
	panelRoleAdvisory = "advisory"
)

// panelSeat is one declared gating review seat in a panel's frozen denominator:
// its stable workflow_job_id, its live attempt (the seal), whether it carries a
// live-seal accepting verdict (the satisfying contribution), whether a live-seal
// dissent blocks it, and whether it is structurally_unrecoverable (a terminal gap
// skippable within the abstention budget, D214b).
type panelSeat struct {
	WorkflowJobID string
	// LiveAttempt is the seat's live seal: MAX(attempt) over its job rows. 0 when the
	// seat has no job row yet (an outstanding seat, never satisfiable).
	LiveAttempt int
	// AcceptingAttempt is the attempt at which the seat has a non-superseded accepting
	// verdict, or -1 when it has none. The seat is SATISFIED iff
	// AcceptingAttempt == LiveAttempt (staged.attempt = live.attempt) and no live
	// dissent blocks it.
	AcceptingAttempt int
	// DissentAtLiveSeal is true when a forward-written dissent_ledger row exists for
	// the seat at its live attempt — a LIVE dissent that blocks the barrier wherever
	// recovery moved the seat's lineage (the seal-durable witness).
	DissentAtLiveSeal bool
	// StructurallyUnrecoverable is true when the seat's owning lane is provably dead
	// (supervisedAgentConfirmedDead, D214b) — the ONLY condition under which a gating
	// seat may be treated as a terminal gap (skipped, counted against the budget). A
	// live gating seat (this false) blocks even when silent.
	StructurallyUnrecoverable bool
}

// quorumDeclaration is the frozen quorum spec for one downstream gate job: its
// declared gating review seats (the denominator) and the abstention budget.
type quorumDeclaration struct {
	// Seats is the frozen set of declared GATING review seats (workflow_job_id),
	// sorted for determinism. Advisory seats are excluded.
	Seats []string
	// MaxGatingAbstentions is the budget (default 0): the number of gating seats that
	// may be terminally-absent (structurally_unrecoverable, D214b) and still let the
	// barrier fire. At 0 every gating seat is required — default behavior unchanged.
	MaxGatingAbstentions int
}

// quorumBarrierReadinessSQL wraps the entity/seal-generic predicate P0 minted in
// db.BarrierReadySQL (entity=review_seat, seal=attempt) with the three relations it
// JOINs, supplied as VALUES rows built in Go from the resolved per-seat panel state.
// It is built ONCE here so the quorum caller cannot drift from the canonical shape;
// the static guard TestBarrierPredicateHasNoRefCount scans this file and fails the
// build on any bare ref-COUNT(*) / recency-latest barrier shape.
//
// The Go-side resolution (resolvePanelSeats) does the per-seat classification —
// including the supervisedAgentConfirmedDead oracle probe (D214b), which is a Go
// liveness probe, not a SQL predicate — and the SQL JOIN over the resulting VALUES
// rows is the pure predicate evaluation. This keeps the trap-killer JOIN
// (staged.attempt = live.attempt) as the structural coherence source: a seat whose
// accepting verdict is at a SUPERSEDED attempt has staged.attempt != live.attempt and
// is STRUCTURALLY unsatisfied — never satisfied by a stale-seal verdict.
//
// The barrier id binds as $1, the entity kind as $2 (the predicate's contract).
func quorumBarrierReadinessSQL(seats []panelSeat) (string, error) {
	predicate, err := db.BarrierReadySQL(db.BarrierSpec{
		EntityKind:         quorumBarrierEntityKind,
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         quorumBarrierSealColumn,
	})
	if err != nil {
		return "", err
	}
	if len(seats) == 0 {
		// An empty declared denominator is vacuously satisfied (no gating seats to
		// wait on); the predicate's bool_and over zero rows is NULL, which the caller
		// treats as ready only when there are genuinely no declared gating seats.
		return "SELECT true", nil
	}
	// Build the three relations as VALUES rows, one per declared gating seat. Each
	// row carries the seat's entity_id, its live seal, the seal at which it has a
	// satisfying accepting contribution (or -1), and its is_terminal_gap flag.
	//
	//   - barrier_in_edges: is_terminal_gap is TRUE only for a structurally
	//     unrecoverable seat (D214b) — and only then does the edge fire without a
	//     live-seal contribution, counting against the abstention budget. The budget
	//     is enforced BEFORE this query (resolvePanelSeats marks at most
	//     MaxGatingAbstentions provably-dead seats as terminal gaps; a dead seat beyond
	//     the budget stays NON-terminal and blocks), so the predicate itself stays the
	//     pure all-edges bool_and.
	//   - entity_live_seal: the seat's live attempt (inner-JOINed, always present).
	//   - staged_contrib: the seat's satisfying-contribution seal. A seat with a
	//     live-seal accepting verdict (and no live dissent) carries staged = live; an
	//     unfilled seat (abstention stub, D214a, no verdict value) or a seat whose only
	//     accepting verdict is at a stale seal carries the never-equal sentinel -1, so
	//     staged.attempt = live.attempt is FALSE and the seat is unsatisfied. A live
	//     dissent forces the sentinel even if an accepting verdict exists.
	var edges, live, staged []string
	for i, s := range seats {
		terminalGap := "false"
		if s.terminalGap() {
			terminalGap = "true"
		}
		// The satisfying seal: live attempt iff the seat is satisfied (a live-seal
		// accepting verdict and no live dissent), else the never-equal sentinel -1.
		satisfyingSeal := -1
		if s.satisfied() {
			satisfyingSeal = s.LiveAttempt
		}
		// Quote the entity id as a SQL string literal (workflow_job_id is a daemon-
		// generated identifier, but quote it defensively — the predicate's identifiers
		// are validated; these are data rows bound through VALUES, doubled to escape).
		id := "'" + strings.ReplaceAll(s.WorkflowJobID, "'", "''") + "'"
		kind := "'" + quorumBarrierEntityKind + "'"
		_ = i
		edges = append(edges, fmt.Sprintf("(%s, %s, %s)", kind, id, terminalGap))
		live = append(live, fmt.Sprintf("(%s, %s, %d)", kind, id, s.LiveAttempt))
		staged = append(staged, fmt.Sprintf("(%s, %s, %d)", kind, id, satisfyingSeal))
	}
	return "WITH barrier_in_edges (entity_kind, entity_id, is_terminal_gap) AS (\n  VALUES " +
		strings.Join(edges, ", ") +
		"\n), entity_live_seal (entity_kind, entity_id, attempt) AS (\n  VALUES " +
		strings.Join(live, ", ") +
		"\n), staged_contrib (entity_kind, entity_id, attempt) AS (\n  VALUES " +
		strings.Join(staged, ", ") +
		"\n)\n" + quorumPredicateWithBarrierGuard(predicate), nil
}

// quorumPredicateWithBarrierGuard injects a barrier_id column the VALUES-built
// in-edges do not carry. The minted predicate filters `edge.barrier_id = $1`; the
// quorum's in-edges are scoped by the downstream gate (resolved in Go), so we add a
// constant barrier_id column to barrier_in_edges via a wrapping SELECT so the
// predicate's $1 / $2 binds still apply unchanged. We do this by aliasing the
// in-edges CTE through a SELECT that projects $1 as barrier_id.
func quorumPredicateWithBarrierGuard(predicate string) string {
	// The minted predicate reads `FROM barrier_in_edges edge` and filters
	// `edge.barrier_id = $1 AND edge.entity_kind = $2`. Our barrier_in_edges CTE has
	// no barrier_id column, so redefine the alias to a SELECT that adds it. The
	// predicate text is fixed; we prepend a CTE-level barrier_in_edges that already
	// includes barrier_id by replacing the CTE name the predicate consumes.
	return strings.Replace(
		predicate,
		"FROM barrier_in_edges      edge",
		"FROM (SELECT entity_kind, entity_id, is_terminal_gap, $1::text AS barrier_id FROM barrier_in_edges) edge",
		1,
	)
}

// terminalGap reports whether the seat is a terminal-acceptable gap: only a
// structurally_unrecoverable seat the budget admits (D214b). A live gating seat is
// never a terminal gap, so it blocks the barrier when unsatisfied.
func (s panelSeat) terminalGap() bool {
	return s.StructurallyUnrecoverable
}

// satisfied reports whether the seat carries a SATISFYING contribution: a live-seal
// accepting verdict (AcceptingAttempt == LiveAttempt) with NO live dissent. An
// abstention stub (no verdict value, D214a) is never satisfied; a stale-seal accepting
// verdict is never satisfied (staged.attempt != live.attempt).
func (s panelSeat) satisfied() bool {
	if s.DissentAtLiveSeal {
		return false
	}
	if s.LiveAttempt <= 0 {
		return false
	}
	return s.AcceptingAttempt == s.LiveAttempt
}

// panelQuorumSatisfied evaluates the panel-quorum barrier for one downstream gate
// job under the per-run advisory lock the caller already holds (RFC 0104). It reads
// the frozen declared-seat denominator + abstention budget from the gate's workflow
// definition, resolves each gating seat's live attempt / satisfying verdict / live
// dissent / structural-unrecoverability (the supervisedAgentConfirmedDead oracle,
// D214b), and evaluates the P0 predicate over the resulting seats.
//
// It is the quorum branch of dependenciesSatisfied/latestVerdict: where
// dependenciesSatisfied gates edge-by-edge, panelQuorumSatisfied gates over the
// aggregate frozen denominator with the abstention budget. With the DEFAULT budget 0
// and all-gating seats it is equivalent to "every gating seat has a live-seal
// accepting verdict" — i.e. exactly the edge-by-edge behavior, so the default is
// unchanged.
// The runner is accepted as `any` (not db.TxRunner) so the live edge-by-edge gate
// `dependenciesSatisfied` — itself `runner any`, reached with both a db.Runner and a
// db.TxRunner across the enqueue call sites — can route paneled review gates through
// the quorum barrier (RFC 0135 P4 cutover, #354). Every query goes through the
// `oneRow`/`queryRows`/`existsRow` helpers (which assert the shared `queryer` surface
// both runner kinds implement), so no caller has to hand the quorum a transaction it
// does not have.
func panelQuorumSatisfied(ctx context.Context, runner any, repositoryID, downstreamJobID string) (bool, error) {
	decl, err := resolveQuorumDeclaration(ctx, runner, repositoryID, downstreamJobID)
	if err != nil {
		return false, err
	}
	if len(decl.Seats) == 0 {
		// No declared gating seats: the quorum is vacuously satisfied (the gate has no
		// gating panel; ordinary dependenciesSatisfied governs any non-panel edges).
		return true, nil
	}
	seats, err := resolvePanelSeats(ctx, runner, repositoryID, downstreamJobID, decl)
	if err != nil {
		return false, err
	}
	query, err := quorumBarrierReadinessSQL(seats)
	if err != nil {
		return false, err
	}
	// Evaluate the single-row bool_and predicate through the `oneRow` helper (the
	// shared `queryer` surface) rather than runner.QueryRow, so this works for a
	// db.Runner and a db.TxRunner identically. A NULL bool_and (no resolvable edges)
	// reads as not-ready, matching the fan-in instance.
	row, err := oneRow(ctx, runner, query, downstreamJobID, quorumBarrierEntityKind)
	if err != nil {
		if errorsIsNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("panel quorum readiness query: %w", err)
	}
	ready, ok := row["bool_and"]
	if !ok {
		// The single projected column may be unnamed depending on the driver's column
		// alias; fall back to the first value.
		for _, v := range row {
			ready = v
			break
		}
	}
	if ready == nil {
		return false, nil
	}
	return boolValue(ready), nil
}

// resolvePanelSeats classifies each declared gating seat for the predicate: its live
// attempt, its satisfying accepting-verdict seal, its live-dissent state, and — for
// at most MaxGatingAbstentions PROVABLY-DEAD seats — its terminal-gap status (D214b).
//
// The abstention budget is enforced HERE, not in SQL: of the gating seats that are
// neither satisfied nor blocked-by-live-dissent AND are structurally_unrecoverable
// (provably dead), at most MaxGatingAbstentions are marked terminal gaps; any beyond
// the budget stay NON-terminal and block. A seat that is merely SLOW (not provably
// dead) is NEVER a terminal gap — it blocks regardless of budget (silence != consent,
// D214b). This keeps the predicate the pure all-edges bool_and while the budget
// policy lives in Go where the oracle probe lives.
func resolvePanelSeats(ctx context.Context, runner any, repositoryID, downstreamJobID string, decl quorumDeclaration) ([]panelSeat, error) {
	run, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", downstreamJobID, false)
	if err != nil {
		return nil, err
	}
	runID := fmt.Sprint(run["run_id"])
	seats := make([]panelSeat, 0, len(decl.Seats))
	for _, seatID := range decl.Seats {
		seat, err := resolvePanelSeat(ctx, runner, repositoryID, runID, seatID)
		if err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	// Apply the abstention budget: admit at most MaxGatingAbstentions provably-dead,
	// unsatisfied, undissented seats as terminal gaps; the rest stay blocking. Sort
	// the candidates deterministically (by workflow_job_id) so the choice of which
	// dead seats are admitted is stable.
	admitted := 0
	sort.SliceStable(seats, func(i, j int) bool { return seats[i].WorkflowJobID < seats[j].WorkflowJobID })
	for i := range seats {
		s := &seats[i]
		if s.satisfied() {
			// A satisfied seat is never a terminal gap (it fires via the live-seal JOIN).
			s.StructurallyUnrecoverable = false
			continue
		}
		if s.DissentAtLiveSeal {
			// A live dissent always blocks; never a terminal gap.
			s.StructurallyUnrecoverable = false
			continue
		}
		if !s.StructurallyUnrecoverable {
			// A live (not provably dead) unsatisfied gating seat blocks (silence !=
			// consent). Leave it non-terminal.
			continue
		}
		if admitted < decl.MaxGatingAbstentions {
			admitted++
			// Stays StructurallyUnrecoverable=true → terminal gap.
		} else {
			// Provably dead but beyond the budget: it must still block.
			s.StructurallyUnrecoverable = false
		}
	}
	return seats, nil
}

// resolvePanelSeat resolves one gating seat's live attempt, satisfying accepting
// verdict seal, live-dissent state, and structural-unrecoverability.
func resolvePanelSeat(ctx context.Context, runner any, repositoryID, runID, seatID string) (panelSeat, error) {
	seat := panelSeat{WorkflowJobID: seatID, AcceptingAttempt: -1}
	// Live attempt: MAX(attempt) over the seat's job rows (the live seal). No
	// COUNT, no recency ORDER BY — a MAX aggregate, exactly as the fan-in live seal.
	liveRow, err := oneRow(ctx, runner, `
		SELECT MAX(attempt) AS live FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3`,
		repositoryID, runID, seatID)
	if err == nil {
		seat.LiveAttempt = intValue(liveRow["live"])
	} else if !errorsIsNoRows(err) {
		return seat, err
	}
	// The satisfying accepting verdict's seal: the MAX attempt at which the seat has a
	// non-superseded accepting verdict, joined to the seat's job rows. A MAX aggregate,
	// not a recency read; a prior-attempt accepting verdict yields a seal below the
	// live attempt and so fails staged.attempt = live.attempt.
	acceptRow, err := oneRow(ctx, runner, `
		SELECT MAX(j.attempt) AS accept_attempt
		  FROM striatumd.verdicts v
		  JOIN striatumd.jobs j
		    ON j.repository_id = v.repository_id AND j.job_id = v.job_id
		 WHERE v.repository_id = $1 AND v.run_id = $2 AND j.workflow_job_id = $3
		   AND v.verdict IN ('accept', 'accept_with_findings')`,
		repositoryID, runID, seatID)
	if err == nil {
		if val := liveValueOrAbsent(acceptRow["accept_attempt"]); val != nil {
			seat.AcceptingAttempt = *val
		}
	} else if !errorsIsNoRows(err) {
		return seat, err
	}
	// A LIVE dissent: a forward-written dissent_ledger row at the seat's live attempt.
	// The dissent table may be absent in a runtime-migrations-only test DB; tolerate
	// that exactly as the verdict path tolerates an absent owner column.
	if seat.LiveAttempt > 0 {
		dissent, derr := seatHasLiveDissent(ctx, runner, repositoryID, runID, seatID, seat.LiveAttempt)
		if derr != nil {
			return seat, derr
		}
		seat.DissentAtLiveSeal = dissent
	}
	// Structural unrecoverability (D214b): the seat's live job row's owning lane is
	// provably dead (supervisedAgentConfirmedDead). Resolved against the seat's live
	// job row's supervisor pointer. A seat with no live-attempt job, or whose pointer
	// is not probeable, is NOT provably dead (left false → blocks if unsatisfied).
	if seat.LiveAttempt > 0 {
		dead, derr := seatStructurallyUnrecoverable(ctx, runner, repositoryID, runID, seatID, seat.LiveAttempt)
		if derr != nil {
			return seat, derr
		}
		seat.StructurallyUnrecoverable = dead
	}
	return seat, nil
}

// liveValueOrAbsent returns the int value of a possibly-NULL aggregate (MAX over an
// empty set is NULL), or nil when absent.
func liveValueOrAbsent(value any) *int {
	if value == nil {
		return nil
	}
	v := intValue(value)
	return &v
}

// seatHasLiveDissent reports whether a forward-written dissent_ledger row exists for
// the seat at its live attempt. The dissent ledger keys on the stable workflow_job_id,
// so a recovered/transferred seat maps back to the same dissent row. Tolerates an
// absent dissent_ledger table (runtime-only test DB before the migration is applied)
// by returning false.
func seatHasLiveDissent(ctx context.Context, runner any, repositoryID, runID, seatID string, liveAttempt int) (bool, error) {
	if !dissentLedgerEnabled(ctx, runner) {
		return false, nil
	}
	return existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.dissent_ledger
		 WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3 AND attempt = $4
		 LIMIT 1`, repositoryID, runID, seatID, liveAttempt)
}

// seatStructurallyUnrecoverable reports whether the seat's live-attempt job row's
// owning lane is provably dead (supervisedAgentConfirmedDead, D214b). It loads the
// seat's live job row joined to its supervisor pointer and probes the same
// forgery-resistant oracle the recovery sweep uses. A seat with no resolvable pointer
// is NOT provably dead.
func seatStructurallyUnrecoverable(ctx context.Context, runner any, repositoryID, runID, seatID string, liveAttempt int) (bool, error) {
	// The seat's live-attempt job row's owning session is its current lease owner; the
	// supervisor pointer is keyed by that session. The aliases mirror the recovery
	// sweep's row shape (recovery_decision_tree.go) so supervisedAgentConfirmedDead
	// reads exactly the keys it expects.
	row, err := oneRow(ctx, runner, `
		SELECT sp.pid AS supervisor_pointer_pid,
		       sp.pid_start_time AS supervisor_pointer_pid_start_time,
		       sp.state AS supervisor_pointer_state,
		       sp.metadata_json AS supervisor_pointer_metadata_json
		  FROM striatumd.jobs j
		  LEFT JOIN striatumd.leases l
		    ON l.repository_id = j.repository_id AND l.lease_id = j.current_lease_id
		  LEFT JOIN striatumd.process_supervisor_pointers sp
		    ON sp.repository_id = j.repository_id AND sp.session_id = l.owner_session_id
		 WHERE j.repository_id = $1 AND j.run_id = $2 AND j.workflow_job_id = $3
		   AND j.attempt = $4`,
		repositoryID, runID, seatID, liveAttempt)
	if err != nil {
		if errorsIsNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return supervisedAgentConfirmedDead(ctx, row), nil
}

// resolveQuorumDeclaration reads the frozen quorum spec for a downstream gate job:
// its declared GATING review-seat dependencies (the denominator) and the abstention
// budget (max_gating_abstentions). panel_role and max_gating_abstentions live in the
// workflow snapshot (no DDL — they are workflow config, D215); the denominator is the
// set of review-seat dependencies frozen at run.prepare.
//
//   - The downstream gate job's workflow definition carries max_gating_abstentions
//     (default 0).
//   - Each upstream review-seat dependency's workflow definition carries panel_role
//     (default gating); only gating seats join the denominator.
func resolveQuorumDeclaration(ctx context.Context, runner any, repositoryID, downstreamJobID string) (quorumDeclaration, error) {
	decl := quorumDeclaration{}
	gate, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", downstreamJobID, false)
	if err != nil {
		return decl, err
	}
	gateDef, err := workflowJobDefinitionForRow(ctx, runner, repositoryID, gate)
	if err != nil {
		return decl, err
	}
	decl.MaxGatingAbstentions = maxGatingAbstentions(gateDef)
	// The declared review-seat dependencies: each job_dependencies row's
	// depends_on_job_id resolves to an upstream job whose workflow_job_id is the seat
	// id. We only count REVIEW seats with panel_role gating.
	deps, err := queryRows(ctx, runner, `
		SELECT depends_on_job_id FROM striatumd.job_dependencies
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, downstreamJobID)
	if err != nil {
		return decl, err
	}
	seen := map[string]bool{}
	for _, dep := range deps {
		upstream, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(dep["depends_on_job_id"]), false)
		if err != nil {
			return decl, err
		}
		if fmt.Sprint(upstream["job_type"]) != "review" {
			continue
		}
		seatID := fmt.Sprint(upstream["workflow_job_id"])
		if seen[seatID] {
			continue
		}
		upstreamDef, err := workflowJobDefinitionForRow(ctx, runner, repositoryID, upstream)
		if err != nil {
			return decl, err
		}
		if panelRoleOf(upstreamDef) != panelRoleGating {
			continue
		}
		seen[seatID] = true
		decl.Seats = append(decl.Seats, seatID)
	}
	sort.Strings(decl.Seats)
	return decl, nil
}

// panelRoleOf returns a review job's declared panel_role, defaulting to gating. A
// value other than gating|advisory is treated as gating (fail-safe: an unknown role
// keeps the seat blocking rather than silently dropping it from the denominator);
// the lint layer rejects unknown values at authoring time.
func panelRoleOf(def map[string]any) string {
	role, ok := def["panel_role"].(string)
	if !ok || role == "" {
		return panelRoleGating
	}
	if role == panelRoleAdvisory {
		return panelRoleAdvisory
	}
	return panelRoleGating
}

// dissentLedgerEnabled reports whether the striatumd.dissent_ledger table exists
// (RFC 0135 P4 migration 0031). Mirrors reviewGenerationEnabled: in a runtime-only
// test DB applied before the migration the table is absent, and the forward-write +
// the live-dissent read both no-op rather than erroring — exactly as the verdict path
// tolerates an absent owner column. In production (migration applied) it is present.
func dissentLedgerEnabled(ctx context.Context, runner any) bool {
	row, err := oneRow(ctx, runner, `
		SELECT EXISTS (
		  SELECT 1 FROM information_schema.tables
		   WHERE table_schema = 'striatumd' AND table_name = 'dissent_ledger'
		) AS enabled`)
	return err == nil && boolValue(row["enabled"])
}

// recordDissent forward-writes a dissent_ledger row co-transactionally with a
// blocking verdict insert (#339), under the per-run advisory lock the verdict path
// already holds. The row is keyed on the STABLE workflow_job_id and sealed at the
// seat's live attempt, so it BLOCKS the panel quorum wherever recovery later moved the
// seat's lineage — the seal-durable dissent witness. It is APPEND-ONLY (the migration's
// refuse-trigger + SELECT/INSERT-only grant), so a later accepting verdict cannot
// retract it; the seat clears only by advancing its live seal past the dissent's seal.
//
// It is a no-op when the dissent_ledger table is absent (runtime-only test DB before
// the migration) and when the verdict is not a blocking verdict (only needs_revision /
// reject are dissents; accept / accept_with_findings are not, and the verdict-LESS
// abstention stub of D214a is never recorded here — it has no verdict value to record).
func recordDissent(ctx context.Context, runner db.TxRunner, repositoryID string, job map[string]any, sessionID, verdictID, verdict string) error {
	if verdict != "needs_revision" && verdict != "reject" {
		return nil
	}
	if !dissentLedgerEnabled(ctx, runner) {
		return nil
	}
	dissentID, err := newID("dissent")
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.dissent_ledger
		  (repository_id, dissent_id, run_id, workflow_job_id, attempt, job_id, session_id, verdict_id, verdict)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		repositoryID, dissentID, job["run_id"], job["workflow_job_id"],
		intValue(job["attempt"]), job["job_id"], nullable(sessionID), nullable(verdictID), verdict)
}

// maxGatingAbstentions returns a gate job's declared abstention budget, defaulting to
// 0 (every gating seat required — behavior unchanged). A negative or non-numeric value
// is clamped to 0.
func maxGatingAbstentions(def map[string]any) int {
	value, exists := def["max_gating_abstentions"]
	if !exists {
		return 0
	}
	n := intValue(value)
	if n < 0 {
		return 0
	}
	return n
}
