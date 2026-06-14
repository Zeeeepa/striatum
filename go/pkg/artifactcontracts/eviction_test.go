package artifactcontracts

import "testing"

// TestEvictionTaxonomyKeepsDurableProvenanceOutOfEviction is the S12 guardrail
// for hippo D003 / RFC 0119: durable provenance is never evictable, and every
// eviction-eligible kind is run exhaust, not durable provenance.
func TestEvictionTaxonomyKeepsDurableProvenanceOutOfEviction(t *testing.T) {
	for _, kind := range DurableProvenanceKindList() {
		if IsEvictableKind(kind) {
			t.Fatalf("durable provenance kind %q is marked evictable", kind)
		}
		if !IsDurableProvenanceKind(kind) {
			t.Fatalf("durable provenance kind %q is not reported durable", kind)
		}
		if !IsAllowedKind(kind) {
			t.Fatalf("durable provenance kind %q is not an allowed artifact kind", kind)
		}
	}

	for _, kind := range EvictableKindList() {
		if !IsEvictableKind(kind) {
			t.Fatalf("evictable kind %q is not reported evictable", kind)
		}
		if IsDurableProvenanceKind(kind) {
			t.Fatalf("evictable kind %q is also marked durable provenance", kind)
		}
		if !IsAllowedKind(kind) {
			t.Fatalf("evictable kind %q is not an allowed artifact kind", kind)
		}
	}
}

// TestEvictionSetsAreDisjoint proves a kind cannot be both durable and evictable.
func TestEvictionSetsAreDisjoint(t *testing.T) {
	for _, kind := range DurableProvenanceKindList() {
		if IsEvictableKind(kind) {
			t.Fatalf("kind %q is in both the durable and evictable sets", kind)
		}
	}
	if len(DurableProvenanceKindList()) == 0 || len(EvictableKindList()) == 0 {
		t.Fatal("eviction taxonomy must enumerate both durable and evictable kinds")
	}
}

// TestEvictionClassificationOfNamedKinds pins the kinds the policy explicitly
// names so a future edit cannot silently reclassify durable provenance as
// exhaust (or vice-versa). Decision-log rows and accepted findings are durable;
// progress notes, operator reports, and working ledgers are run exhaust.
func TestEvictionClassificationOfNamedKinds(t *testing.T) {
	durable := []string{"decision", "finding", "synthesis", "operator_brief", "escalation"}
	for _, kind := range durable {
		if IsEvictableKind(kind) {
			t.Fatalf("durable provenance kind %q must never be evictable", kind)
		}
	}

	evictable := []string{
		"progress_note",
		"operator_report",
		"findings_ledger",
		"support_ledger",
		"action_item_ledger",
		"collaboration_ledger",
	}
	for _, kind := range evictable {
		if !IsEvictableKind(kind) {
			t.Fatalf("run-exhaust kind %q must be evictable", kind)
		}
	}
}

// TestEvictionDefaultsToDurableForUnknownKinds proves the fail-safe: a kind not
// explicitly enumerated as exhaust is never silently dropped from git.
func TestEvictionDefaultsToDurableForUnknownKinds(t *testing.T) {
	for _, kind := range []string{"", "  ", "rfc", "marker", "patch_summary", "totally_unknown_kind"} {
		if IsEvictableKind(kind) {
			t.Fatalf("unclassified kind %q must default to non-evictable", kind)
		}
	}
}
