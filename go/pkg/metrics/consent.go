package metrics

// RFC 0137 Phase D — tick-status SLI and the lifecycle-balance blind-spot gauge.
//
// This file holds the small Phase D closed enums and the conservation helper that
// feed the multi-tenant/consent surface in snapshot.go and render.go:
//
//   - TickStatus (deliverable #3): the closed ok|partial|error enum stamped on a
//     snapshot at the sweep tick, so a partial/errored fold is DIRECTLY visible
//     rather than silently serving last-good numbers (OQ1).
//   - isUnaccountedTerminal (OQ2): the lifecycle-balance "second doctor" — a
//     terminal transition that declared a death the fold could account for in
//     NEITHER the apoptosis nor the necrosis counter is a provable runner blind
//     spot, and increments striatum_lifecycle_balance.

// TickStatus is the closed enum describing whether the sweep-tick fold that
// produced a snapshot completed cleanly. ok = every fold succeeded; partial = a
// best-effort (Phase B/C/D) fold degraded to empty but the load-bearing Phase A
// folds succeeded; error = a load-bearing Phase A fold failed and the snapshot is
// the carried-forward last-good data, republished only to surface the failure.
type TickStatus string

const (
	TickOK      TickStatus = "ok"
	TickPartial TickStatus = "partial"
	TickError   TickStatus = "error"
)

// tickStatusDomain is the closed, sorted tick-status enum. The full enum is always
// rendered (1 for the active status, 0 for the others) so the gauge is present on
// every scrape and a status flip is alertable without waiting for a series to
// appear.
var tickStatusDomain = []TickStatus{TickError, TickOK, TickPartial}

// TickStatuses returns a copy of the closed tick-status enum.
func TickStatuses() []TickStatus {
	out := make([]TickStatus, len(tickStatusDomain))
	copy(out, tickStatusDomain)
	return out
}

func isTickStatus(s TickStatus) bool {
	switch s {
	case TickOK, TickPartial, TickError:
		return true
	}
	return false
}

// normalizeTickStatus maps the empty/zero value (and any out-of-domain value) to
// ok, so a hand-built snapshot or the pre-first-fold zero value renders a
// well-formed status.
func normalizeTickStatus(s TickStatus) TickStatus {
	if !isTickStatus(s) {
		return TickOK
	}
	return s
}

// isUnaccountedTerminal reports whether a durable event is a terminal transition
// that DECLARED a death the fold cannot account for in either lifecycle counter —
// the OQ2 lifecycle-balance blind spot. Concretely: a session.closed tagged
// necrosis whose stall_class is not in the closed necrosis domain. Such an event
// asserts a confirmed death (so it is not a clean apoptosis) yet carries no
// classifiable necrosis reason (so it folds into no necrosis series); without the
// balance gauge it would vanish from both counters. Intentional skips
// (recovery_transfer closes, non-handoff lease releases, non-exhausted
// escalations) are NOT blind spots and return false.
func isUnaccountedTerminal(ev LifecycleEvent) bool {
	return ev.EventType == "session.closed" &&
		ev.LifecycleTag == LifecycleTagNecrosis &&
		!isNecrosisReason(NecrosisReason(ev.StallClass))
}

// erroredTickSnapshot returns the snapshot to publish when a load-bearing fold
// failed at a sweep tick (Phase D deliverable #3 / OQ1: publish-on-errored-tick).
// It preserves the previous snapshot's folded families AND its builtAt — so the
// last-good DATA keeps serving and striatum_metrics_snapshot_age_seconds keeps
// CLIMBING (a failed tick must not reset the staleness SLI) — while stamping
// tick_status=error so a wedged/erroring reconcile loop is DIRECTLY visible
// instead of silently inferred from a rising age. With no prior snapshot it
// returns an empty surface stamped error. The previous snapshot's maps are
// immutable after Build, so the shallow copy can safely share them.
func erroredTickSnapshot(prev *Snapshot) *Snapshot {
	if prev == nil {
		return Build(SnapshotInput{TickStatus: TickError})
	}
	clone := *prev
	clone.tickStatus = TickError
	return &clone
}
