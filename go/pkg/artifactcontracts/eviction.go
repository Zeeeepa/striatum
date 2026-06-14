package artifactcontracts

import (
	"sort"
	"strings"
)

// Eviction taxonomy under the three-axis model (hippo D003 / RFC 0119 / D179).
//
// The eviction axis governs only which artifacts may stop living in the git
// working tree. It is independent of the placement axis (placement.go): where an
// artifact body is stored is a separate question from whether the artifact is
// durable provenance. A finding routed to blob_exhaust storage is still durable
// provenance and is never evictable.
//
// Durable provenance is canonical in git and is NEVER evictable: ratified
// decisions, accepted findings and syntheses, governance briefs and work plans,
// escalations, and build handoffs that cite committed work. (RFCs and the
// decision log live in git as documents, not as artifact rows, and are durable
// by construction.) Only run exhaust and unsynthesized intermediates are
// eviction-eligible: progress notes, operator reports, working ledgers, gate
// evidence, and unsynthesized improvement proposals.
//
// Classification is fail-safe: a kind that is not explicitly enumerated as
// evictable is treated as durable (IsEvictableKind returns false), so an
// unrecognized or future artifact kind is never silently dropped from git.
//
// S12 finalizes the eviction policy that RFC 0119 / S11 explicitly deferred (the
// S11 spec recorded only an initial allow-list of progress_note with enforcement
// "deferred to S12"). This taxonomy does not move any durable-provenance kind out
// of git; it only enumerates the run-exhaust kinds that were always eviction
// scope under D003.

var durableProvenanceKinds = map[string]bool{
	"decision":       true,
	"finding":        true,
	"synthesis":      true,
	"operator_brief": true,
	"work_plan":      true,
	"escalation":     true,
	"handoff":        true,
}

var evictableExhaustKinds = map[string]bool{
	"progress_note":                true,
	"operator_report":              true,
	"test_report":                  true,
	"auto_finalize_gate_evidence":  true,
	"findings_ledger":              true,
	"support_ledger":               true,
	"action_item_ledger":           true,
	"collaboration_ledger":         true,
	"harness_improvement_proposal": true,
}

// IsDurableProvenanceKind reports whether an artifact kind is durable provenance
// that must stay committed in git and is never eviction-eligible.
func IsDurableProvenanceKind(kind string) bool {
	return durableProvenanceKinds[strings.TrimSpace(kind)]
}

// IsEvictableKind reports whether an artifact kind is run exhaust or an
// unsynthesized intermediate that may stop living in the git working tree.
// Unrecognized kinds are treated as durable (not evictable) — fail-safe.
func IsEvictableKind(kind string) bool {
	return evictableExhaustKinds[strings.TrimSpace(kind)]
}

// DurableProvenanceKindList returns the durable-provenance kinds, sorted.
func DurableProvenanceKindList() []string {
	return sortedKeys(durableProvenanceKinds)
}

// EvictableKindList returns the eviction-eligible exhaust kinds, sorted.
func EvictableKindList() []string {
	return sortedKeys(evictableExhaustKinds)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
