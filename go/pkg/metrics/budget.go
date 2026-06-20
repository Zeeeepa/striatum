package metrics

// RFC 0137 Phase C — the per-family series budget (deliverable #2).
//
// The closed-enum families (apoptosis/necrosis/lease/liveness/wedge/margin) are
// structurally cardinality-bounded: their label values come from CREATE-defined
// closed Go enums, so no run/job/session identifier can ever widen them. The one
// Phase C family whose label domain is NOT a compile-time-closed enum is
// doctor_problems{class}, whose `class` is folded from the doctor checks. Its
// codes are static by construction, but the series budget is the defense-in-depth
// backstop: if a future check ever emitted an unexpectedly large set of distinct
// codes (or a misused, id-bearing `check` field slipped past the fold sanitizer),
// the budget caps the emitted series so neither the daemon registry nor a
// downstream Prometheus can be ID-bombed into OOM, and the drop is itself
// observable via striatum_metrics_cardinality_clipped_total.

import "sort"

// seriesClassOther is the reserved overflow bucket the series budget collapses
// excess label-tuples onto. The doctor family's label is `class`, so its reserved
// overflow series renders as {class="other"} (the RFC's generic {bucket="other"}
// realized for this family's label).
const seriesClassOther = "other"

// applySeriesBudget enforces a per-family distinct-key budget on a string-keyed
// counter map. It KEEPS the `limit` lexicographically-smallest keys and collapses
// every remaining distinct key onto the reserved `other` bucket, returning the
// number of distinct keys that were clipped. Emitted series are therefore bounded
// at limit+1 regardless of how many distinct keys arrive.
//
// The fold is rebuilt from durable state every tick, so a deterministic sorted
// cap — rather than a stateful cross-tick LRU — is the faithful realization of
// the RFC's "series budget": it delivers the same hard OOM cap, is byte-stable
// for the golden, and needs no persistent collector state to survive a restart.
func applySeriesBudget(counts map[string]int, limit int, other string) int {
	if limit < 0 {
		limit = 0
	}
	if len(counts) <= limit {
		return 0
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	clipped := 0
	overflow := 0
	for _, k := range keys[limit:] {
		if k == other {
			// A genuine `other` key is always kept; it absorbs the overflow rather
			// than being clipped onto itself.
			continue
		}
		overflow += counts[k]
		delete(counts, k)
		clipped++
	}
	if clipped > 0 {
		counts[other] += overflow
	}
	return clipped
}
