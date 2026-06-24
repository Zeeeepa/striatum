package mutations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
)

// RFC 0133 fan-in barrier cutover — the LIVE FAN-OUT seam (#527, the last leg of
// #354). At run materialization it declares the durable fan-in barriers that the
// completion path stages and the downstream gate assembles. STRIATUM_BARRIER_FANIN=0
// is the recoverable kill switch back to the shipped D206 per-completion merge.
//
// WHAT A FAN-IN IS, structurally. A downstream job that depends on TWO OR MORE
// upstream sibling jobs is a fan-in join: the siblings run in parallel and the
// downstream job consumes their combined work (the canonical case is a phase's
// synthesis job depending on every author seat in the phase). The freeze record
// declares, once at fan-out, the immutable sibling set the barrier later joins. A
// downstream job with a single upstream is a plain chain edge, not a fan-in, so no
// freeze point is recorded for it.

// faninBarrierID derives the deterministic, stable barrier id for a fan-in keyed on
// its downstream (join) seat within a run. The downstream workflow_job_id is stable
// across recovery (job_id churns), so the barrier id is stable too — the staging hook
// (faninBarrierForSeat) and the dispatcher both resolve the barrier from the run +
// declared sibling set, not from this id, but a deterministic id keeps the freeze
// record idempotent (the recorder's ON CONFLICT DO NOTHING) across a re-prepare.
func faninBarrierID(runID, downstreamWorkflowJobID string) string {
	return fmt.Sprintf("barrier_fanin_%s_%s", runID, downstreamWorkflowJobID)
}

// recordRunFaninFreezePoints is the live fan-out caller (#527). At run materialization
// it inspects the workflow's dependency edges, finds every downstream join seat with
// two or more upstream siblings, and records one immutable fan-in freeze point per
// such seat declaring its sibling set, frozen at the run-branch tip.
//
// It is a STRICT NO-OP when STRIATUM_BARRIER_FANIN=0. It is ALSO a no-op when the
// run branch is not yet confirmed (no frozen tip is knowable, so recording one would
// be a ghost base): a fan-in run whose branch is confirmed later simply does not
// declare a barrier this cycle, which keeps it on the per-completion path rather
// than recording a freeze point against an unconfirmed tip. frozenTip is the
// confirmed run-branch tip SHA ("" when the branch is not yet confirmed).
//
// declaredSiblingJobIDs is keyed on the stable workflow_job_id (the seat identity the
// staging predicate JOINs on), NOT the churning job_id. The set is sorted for a
// deterministic, reproducible freeze record.
func recordRunFaninFreezePoints(ctx context.Context, runner db.TxRunner, repositoryID, runID, frozenTip string, edges []dependencyEdge) error {
	if !barrierFaninAssemblyEnabled() {
		// Kill switch: the fan-out records nothing and the per-completion merge (D206)
		// stays the sole fan-in path.
		return nil
	}
	frozenTip = strings.TrimSpace(frozenTip)
	if !isFullGitSHA(frozenTip) {
		// The run branch is not confirmed yet (or the tip is unresolvable): there is no
		// frozen base to record against, so do not declare a barrier this cycle. The run
		// stays on the unchanged per-completion path.
		return nil
	}
	// Group upstream siblings by their downstream join seat. A downstream with >= 2
	// distinct upstreams is a fan-in; a downstream with exactly one upstream is a plain
	// chain edge and is skipped.
	siblings := map[string]map[string]struct{}{}
	for _, edge := range edges {
		from := strings.TrimSpace(edge.fromID)
		to := strings.TrimSpace(edge.toID)
		if from == "" || to == "" || from == to {
			continue
		}
		set := siblings[to]
		if set == nil {
			set = map[string]struct{}{}
			siblings[to] = set
		}
		set[from] = struct{}{}
	}
	// Record one freeze point per fan-in join seat, in deterministic seat order so a
	// re-prepare writes the identical record set (the recorder is idempotent anyway).
	downstreams := make([]string, 0, len(siblings))
	for to := range siblings {
		downstreams = append(downstreams, to)
	}
	sort.Strings(downstreams)
	for _, to := range downstreams {
		set := siblings[to]
		if len(set) < 2 {
			continue
		}
		declared := make([]string, 0, len(set))
		for from := range set {
			declared = append(declared, from)
		}
		sort.Strings(declared)
		if err := recordFaninFreezePoint(ctx, runner, repositoryID, faninFreezePoint{
			BarrierID:               faninBarrierID(runID, to),
			RunID:                   runID,
			DownstreamWorkflowJobID: to,
			FrozenTipSHA:            frozenTip,
			DeclaredSiblingJobIDs:   declared,
		}); err != nil {
			return fmt.Errorf("record fan-in freeze point for downstream %q: %w", to, err)
		}
	}
	return nil
}
