package mutations

import (
	"context"
	"fmt"
	"sort"

	"github.com/halbritt/striatum/go/pkg/db"
)

// RFC 0132 Layer C / P4b (#341, D214) — advisory-review guards.
//
// The RFC 0135 P4 quorum core (barrier_quorum.go) already EXCLUDES advisory seats
// from the gating denominator: an advisory verdict cannot satisfy or block the
// frozen-denominator quorum. P4b adds the rule the RFC's Layer C names —
// "advisory: loud reject, silent accept, mandatory minority report" — so an
// advisory seat is non-blocking-by-default but NEVER SILENT. It can stop the line
// (open an operator-resolvable blocker on the downstream gate, the `checkpoint
// resolve` shape) but never auto-flips a verdict and never silently wedges.
//
// The three guards (evaluated co-transactionally with the verdict insert in
// applyVerdict, under the per-run advisory lock the review path already holds):
//
//  1. advisory_corroborated_abstention — a current-attempt GATING abstention
//     (a declared gating seat with no live-seal accepting verdict and no live
//     dissent) co-occurring with ANY current-attempt advisory needs_revision /
//     reject. The advisory dissent corroborates the gating gap: the abstention is
//     reclassified to the must_escalate outcome (it does not silently fall into
//     the abstention budget). A live advisory voice raising a concern alongside a
//     missing gating voice is not a clean skip.
//  2. unanimous_advisory_reject — >= 1 submitted advisory seat, ALL of which
//     reject. This blocks finalize even under full gating accept: a unanimous
//     advisory rejection is a loud signal the gate must not paper over.
//  3. advisory_only_panel_ungrounded — a downstream gate whose panel has ZERO
//     declared gating seats but >= 1 advisory seat. An ungrounded advisory-only
//     signal must not auto-finalize; it blocks unconditionally until an operator
//     resolves it.
//
// All three are operator-resolvable BLOCKED-severity blockers + an escalation_inbox
// row (the artifact-plus-escalation_inbox form, D229 / RFC 0132 Migration §): never
// an auto-needs_revision of the implementer, never a silent wedge. The blocker sits
// on the DOWNSTREAM GATE job, so the gate stays non-terminal and maybeCompleteRun's
// `remaining` check keeps the run from finalizing until the operator clears it
// (checkpoint resolve).

const (
	// advisoryBlockerKindCorroborated is the blocker_kind for guard 1.
	advisoryBlockerKindCorroborated = "advisory_corroborated_abstention"
	// advisoryBlockerKindUnanimousReject is the blocker_kind for guard 2.
	advisoryBlockerKindUnanimousReject = "unanimous_advisory_reject"
	// advisoryBlockerKindUngrounded is the blocker_kind for guard 3.
	advisoryBlockerKindUngrounded = "advisory_only_panel_ungrounded"
	// advisoryOutcomeMustEscalate is the must_escalate outcome the corroborated
	// abstention reclassifies into (and the resolved advisory_minority_report's
	// advisory_outcome when any guard fired).
	advisoryOutcomeMustEscalate = "must_escalate"
)

// advisoryGuardKinds is the closed, ordered set of advisory-guard blocker kinds —
// the single source of truth the hold-the-gate read and any future guard sweep
// share so they cannot drift from the three Layer C guards.
var advisoryGuardKinds = []string{
	advisoryBlockerKindCorroborated,
	advisoryBlockerKindUnanimousReject,
	advisoryBlockerKindUngrounded,
}

// gateHeldByOpenAdvisoryGuard reports whether the gate job carries an open
// advisory-guard blocker (any of the three Layer C guard kinds). The
// dependenciesSatisfied gate consults it so a fired advisory guard HOLDS the gate
// from enqueuing until an operator resolves the blocker (checkpoint resolve). A gate
// with no advisory seats never carries such a blocker, so default behavior is
// unchanged.
func gateHeldByOpenAdvisoryGuard(ctx context.Context, runner any, repositoryID, gateJobID string) (bool, error) {
	return existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.blockers
		 WHERE repository_id = $1 AND job_id = $2 AND state = 'open'
		   AND blocker_kind = ANY($3)
		 LIMIT 1`, repositoryID, gateJobID, advisoryGuardKinds)
}

// advisorySeatVerdict is one declared advisory seat's current-attempt verdict for
// the tally: its stable workflow_job_id, its live-seal verdict (or "" when it has
// no current-seal verdict / is unsubmitted), and whether it carries a live dissent.
type advisorySeatVerdict struct {
	WorkflowJobID string
	Verdict       string
	Submitted     bool
}

// advisoryTally is the per-gate advisory + gating-abstention summary the guards
// read. It is keyed on the downstream gate job and resolved over the frozen
// declared seats (gating from the quorum declaration; advisory enumerated here).
type advisoryTally struct {
	// GatingSeatCount is the number of declared gating seats in the frozen
	// denominator (from resolveQuorumDeclaration).
	GatingSeatCount int
	// HasGatingAbstention is true when at least one declared gating seat is
	// unsatisfied at its live seal and not blocked by a live dissent — a silent
	// gating gap (an abstention) at the current attempt.
	HasGatingAbstention bool
	// AdvisorySeats is the per-advisory-seat current-seal verdict tally.
	AdvisorySeats []advisorySeatVerdict
}

// submittedAdvisory returns the advisory seats that carry a current-seal verdict
// (the submitted advisory voices), sorted by seat for determinism.
func (t advisoryTally) submittedAdvisory() []advisorySeatVerdict {
	out := make([]advisorySeatVerdict, 0, len(t.AdvisorySeats))
	for _, s := range t.AdvisorySeats {
		if s.Submitted {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkflowJobID < out[j].WorkflowJobID })
	return out
}

// hasAdvisoryDissent reports whether any submitted advisory seat voted a blocking
// verdict (needs_revision / reject) at its current seal.
func (t advisoryTally) hasAdvisoryDissent() bool {
	for _, s := range t.submittedAdvisory() {
		if s.Verdict == "needs_revision" || s.Verdict == "reject" {
			return true
		}
	}
	return false
}

// unanimousAdvisoryReject reports whether there is >= 1 submitted advisory seat and
// EVERY submitted advisory seat rejected (guard 2). An empty submitted set is not
// unanimous-reject.
func (t advisoryTally) unanimousAdvisoryReject() bool {
	submitted := t.submittedAdvisory()
	if len(submitted) == 0 {
		return false
	}
	for _, s := range submitted {
		if s.Verdict != "reject" {
			return false
		}
	}
	return true
}

// advisoryGuardDecision is the resolved guard outcome for one downstream gate:
// which guard fired (or "" when none), and the legible per-seat advisory tally for
// the minority report / blocker payload.
type advisoryGuardDecision struct {
	GuardFired string
	Tally      advisoryTally
}

// evaluateAdvisoryGuards classifies the advisory tally into at most one fired
// guard. Precedence: advisory_only_panel_ungrounded (no gating ground at all) >
// unanimous_advisory_reject (a loud unanimous block) > advisory_corroborated_
// abstention (a gating gap corroborated by an advisory dissent). Returns GuardFired
// == "" when no advisory guard fires (the silent-accept case — a report is still
// authored downstream, but no blocker opens).
func evaluateAdvisoryGuards(tally advisoryTally) advisoryGuardDecision {
	decision := advisoryGuardDecision{Tally: tally}
	switch {
	case tally.GatingSeatCount == 0 && len(tally.submittedAdvisory()) > 0:
		decision.GuardFired = advisoryBlockerKindUngrounded
	case tally.unanimousAdvisoryReject():
		decision.GuardFired = advisoryBlockerKindUnanimousReject
	case tally.HasGatingAbstention && tally.hasAdvisoryDissent():
		decision.GuardFired = advisoryBlockerKindCorroborated
	}
	return decision
}

// resolveAdvisoryTally resolves the advisory + gating-abstention tally for one
// downstream gate job. It reuses resolveQuorumDeclaration for the frozen gating
// denominator (so the gating-abstention signal is read-consistent with the quorum
// the gate would evaluate) and enumerates the gate's advisory review-seat
// dependencies separately.
func resolveAdvisoryTally(ctx context.Context, runner db.TxRunner, repositoryID, downstreamJobID string) (advisoryTally, error) {
	tally := advisoryTally{}
	decl, err := resolveQuorumDeclaration(ctx, runner, repositoryID, downstreamJobID)
	if err != nil {
		return tally, err
	}
	tally.GatingSeatCount = len(decl.Seats)

	// Gating abstention: at least one declared gating seat is unsatisfied at its
	// live seal and not blocked by a live dissent (a silent gating gap). This reuses
	// the same per-seat resolution the quorum predicate uses, so the abstention
	// signal is consistent with the denominator the gate would evaluate.
	if len(decl.Seats) > 0 {
		seats, err := resolvePanelSeats(ctx, runner, repositoryID, downstreamJobID, decl)
		if err != nil {
			return tally, err
		}
		for _, s := range seats {
			if s.satisfied() {
				continue
			}
			if s.DissentAtLiveSeal {
				// A live dissent is a blocking gating voice, not a silent abstention;
				// the quorum already blocks on it.
				continue
			}
			tally.HasGatingAbstention = true
			break
		}
	}

	// Advisory seats: the gate's review-seat dependencies declared panel_role
	// advisory, with each seat's current-seal verdict.
	run, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", downstreamJobID, false)
	if err != nil {
		return tally, err
	}
	runID := fmt.Sprint(run["run_id"])
	advisorySeatIDs, err := advisoryDeclaredSeats(ctx, runner, repositoryID, downstreamJobID)
	if err != nil {
		return tally, err
	}
	for _, seatID := range advisorySeatIDs {
		verdict, submitted, err := seatCurrentSealVerdict(ctx, runner, repositoryID, runID, seatID)
		if err != nil {
			return tally, err
		}
		tally.AdvisorySeats = append(tally.AdvisorySeats, advisorySeatVerdict{
			WorkflowJobID: seatID,
			Verdict:       verdict,
			Submitted:     submitted,
		})
	}
	sort.Slice(tally.AdvisorySeats, func(i, j int) bool {
		return tally.AdvisorySeats[i].WorkflowJobID < tally.AdvisorySeats[j].WorkflowJobID
	})
	return tally, nil
}

// advisoryDeclaredSeats returns the downstream gate's review-seat dependencies that
// declare panel_role advisory (the complement of resolveQuorumDeclaration's gating
// seats). Sorted, de-duplicated by stable workflow_job_id.
func advisoryDeclaredSeats(ctx context.Context, runner db.TxRunner, repositoryID, downstreamJobID string) ([]string, error) {
	deps, err := queryRows(ctx, runner, `
		SELECT depends_on_job_id FROM striatumd.job_dependencies
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, downstreamJobID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	seats := []string{}
	for _, dep := range deps {
		upstream, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(dep["depends_on_job_id"]), false)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(upstream["job_type"]) != "review" {
			continue
		}
		seatID := fmt.Sprint(upstream["workflow_job_id"])
		if seen[seatID] {
			continue
		}
		def, err := workflowJobDefinitionForRow(ctx, runner, repositoryID, upstream)
		if err != nil {
			return nil, err
		}
		if panelRoleOf(def) != panelRoleAdvisory {
			continue
		}
		seen[seatID] = true
		seats = append(seats, seatID)
	}
	sort.Strings(seats)
	return seats, nil
}

// seatCurrentSealVerdict returns a seat's current-seal verdict (the non-superseded
// verdict at the seat's live attempt) and whether the seat has submitted a verdict
// at its live seal. A verdict at a SUPERSEDED attempt is non-current (the seat
// advanced past it) — exactly the seal trap-killer applied to the advisory tally,
// so an advisory seat's prior-round reject does not haunt a revised target.
func seatCurrentSealVerdict(ctx context.Context, runner db.TxRunner, repositoryID, runID, seatID string) (string, bool, error) {
	liveRow, err := oneRow(ctx, runner, `
		SELECT MAX(attempt) AS live FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3`,
		repositoryID, runID, seatID)
	if err != nil {
		if errorsIsNoRows(err) {
			return "", false, nil
		}
		return "", false, err
	}
	liveAttempt := intValue(liveRow["live"])
	if liveAttempt <= 0 {
		return "", false, nil
	}
	row, err := oneRow(ctx, runner, `
		SELECT v.verdict
		  FROM striatumd.verdicts v
		  JOIN striatumd.jobs j
		    ON j.repository_id = v.repository_id AND j.job_id = v.job_id
		 WHERE v.repository_id = $1 AND v.run_id = $2 AND j.workflow_job_id = $3
		   AND j.attempt = $4 AND v.superseded_by_decision_id IS NULL
		 ORDER BY v.created_at DESC, v.verdict_id DESC
		 LIMIT 1`,
		repositoryID, runID, seatID, liveAttempt)
	if err != nil {
		if errorsIsNoRows(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return fmt.Sprint(row["verdict"]), true, nil
}

// applyAdvisoryGuards is the Layer C hook called from applyVerdict after a verdict
// is recorded on a panel seat. For each downstream gate that depends on the
// just-recorded seat, it resolves the advisory tally, evaluates the guards, and —
// when a guard fires — opens an operator-resolvable BLOCKED-severity blocker on the
// gate job plus an escalation_inbox row (idempotent: it does not re-open a guard
// blocker already open for the same gate+kind). It never auto-flips a verdict and
// never silently wedges. It is a no-op when no advisory guard fires.
func applyAdvisoryGuards(ctx context.Context, runner db.TxRunner, repositoryID, recordedJobID string) error {
	recorded, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", recordedJobID, false)
	if err != nil {
		return err
	}
	runID := fmt.Sprint(recorded["run_id"])
	// The downstream gates that depend on this seat (the gates whose advisory tally
	// the just-recorded verdict could change). Also include the advisory seat's own
	// downstream gates even when the recorded seat is gating — a gating accept that
	// completes the denominator can be the moment a unanimous advisory reject or an
	// ungrounded advisory panel becomes evaluable.
	gates, err := queryRows(ctx, runner, `
		SELECT DISTINCT jd.job_id
		  FROM striatumd.job_dependencies jd
		  JOIN striatumd.jobs g
		    ON g.repository_id = jd.repository_id AND g.job_id = jd.job_id
		 WHERE jd.repository_id = $1 AND jd.depends_on_job_id = $2`,
		repositoryID, recordedJobID)
	if err != nil {
		return err
	}
	for _, gateRow := range gates {
		gateJobID := fmt.Sprint(gateRow["job_id"])
		tally, err := resolveAdvisoryTally(ctx, runner, repositoryID, gateJobID)
		if err != nil {
			return err
		}
		decision := evaluateAdvisoryGuards(tally)
		if decision.GuardFired == "" {
			continue
		}
		if err := openAdvisoryGuardBlocker(ctx, runner, repositoryID, runID, gateJobID, decision); err != nil {
			return err
		}
	}
	return nil
}

// openAdvisoryGuardBlocker opens a BLOCKED-severity, operator-resolvable blocker on
// the gate job plus an escalation_inbox row for a fired advisory guard, idempotently
// (a guard blocker already open for the same gate+kind is left in place rather than
// duplicated). The blocker sits on the gate so the gate stays non-terminal and the
// run cannot finalize until an operator resolves it (checkpoint resolve). It records
// the legible advisory tally (per-seat verdicts) and the must_escalate outcome in the
// blocker / escalation payload so the hold is self-explaining.
func openAdvisoryGuardBlocker(ctx context.Context, runner db.TxRunner, repositoryID, runID, gateJobID string, decision advisoryGuardDecision) error {
	existing, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.blockers
		 WHERE repository_id = $1 AND job_id = $2 AND blocker_kind = $3 AND state = 'open'
		 LIMIT 1`, repositoryID, gateJobID, decision.GuardFired)
	if err != nil {
		return err
	}
	if existing {
		return nil
	}
	blockerID, err := newID("blk")
	if err != nil {
		return err
	}
	now := nowString()
	payload := advisoryGuardPayload(decision)
	payloadArg, err := db.JSONBArg(runner, payload)
	if err != nil {
		return err
	}
	description := advisoryGuardDescription(decision.GuardFired)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		)
		VALUES ($1,$2,$3,$4,NULL,'blocked',$5,$6,'open',$7,$8::jsonb)`,
		repositoryID, blockerID, runID, gateJobID,
		decision.GuardFired, description, now, payloadArg,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.escalation_inbox (
		  repository_id, escalation_id, run_id, job_id, session_id,
		  blocker_id, blocker_kind, severity, state, created_at, payload_json
		)
		VALUES ($1,$2,$3,$4,NULL,$5,$6,'blocked','pending',$7,$8::jsonb)`,
		repositoryID, blockerID, runID, gateJobID,
		blockerID, decision.GuardFired, now, payloadArg,
	); err != nil {
		return err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, "advisory_guard.fired", nil, gateJobID, nil, nil, nil, map[string]any{
		"guard_fired":       decision.GuardFired,
		"advisory_outcome":  advisoryOutcomeMustEscalate,
		"blocker_id":        blockerID,
		"advisory_tally":    payload["advisory_tally"],
		"gating_seat_count": decision.Tally.GatingSeatCount,
	}); err != nil {
		return err
	}
	return nil
}

// advisoryGuardPayload builds the legible blocker / escalation payload for a fired
// guard: the must_escalate outcome and the per-seat advisory tally.
func advisoryGuardPayload(decision advisoryGuardDecision) map[string]any {
	seats := make([]map[string]any, 0, len(decision.Tally.AdvisorySeats))
	for _, s := range decision.Tally.AdvisorySeats {
		verdict := s.Verdict
		if !s.Submitted {
			verdict = "abstain"
		}
		seats = append(seats, map[string]any{
			"workflow_job_id": s.WorkflowJobID,
			"verdict":         verdict,
		})
	}
	return map[string]any{
		"guard_fired":           decision.GuardFired,
		"advisory_outcome":      advisoryOutcomeMustEscalate,
		"gating_seat_count":     decision.Tally.GatingSeatCount,
		"has_gating_abstention": decision.Tally.HasGatingAbstention,
		"advisory_tally":        seats,
	}
}

// advisoryGuardDescription renders the operator-facing description for a fired
// advisory guard's blocker.
func advisoryGuardDescription(guard string) string {
	switch guard {
	case advisoryBlockerKindCorroborated:
		return "advisory_corroborated_abstention: a gating seat abstained while a live advisory reviewer raised needs_revision/reject; the abstention is reclassified must_escalate (it cannot fall silently into the abstention budget). Resolve with checkpoint resolve once the gating gap is addressed, or override the gate."
	case advisoryBlockerKindUnanimousReject:
		return "unanimous_advisory_reject: every submitted advisory reviewer rejected; finalize is blocked even under full gating accept (an advisory reviewer can stop the line). Resolve with checkpoint resolve to proceed, or route the implementer back through revision."
	case advisoryBlockerKindUngrounded:
		return "advisory_only_panel_ungrounded: this gate's panel has no gating seats but at least one advisory seat; an ungrounded advisory-only signal must not auto-finalize. Resolve with checkpoint resolve once a gating ground is established, or override the gate."
	default:
		return "advisory guard blocked finalize"
	}
}
