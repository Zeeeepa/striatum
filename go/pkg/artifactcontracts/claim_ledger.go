package artifactcontracts

import (
	"fmt"
	"sort"
	"strings"
)

// Claim status lattice (RFC 0134 / D227). The ordering is the point: a stage
// may only ever LOWER a claim's effective status, never raise it — the lane's
// "sealed mint". VERIFIED is the top rung and is reachable in this slice ONLY
// through a validated, bound receipt artifact (the executable verifier lane is
// a later, separate slice); default authoring without a receipt can assert at
// most ASSERTED.
//
//	VERIFIED  > ASSERTED > DESIGNED
//
//   - DESIGNED: the claim is a planned/decided intention; nothing built or
//     measured backs it. The decision-log "accepted" rung — "we decided to".
//   - ASSERTED: a producer states the claim from a structured read of source;
//     no sealed mechanical evidence. The default ceiling for unwitnessed claims.
//   - VERIFIED: a sealed receipt (a tamper-evident transcript) is bound to the
//     claim AND the claim's bound input digest matches the receipt's. The
//     highest rung; mechanically earned, never authored.
const (
	ClaimStatusDesigned = "DESIGNED"
	ClaimStatusAsserted = "ASSERTED"
	ClaimStatusVerified = "VERIFIED"
)

// claimStatusRank is the lattice height of each status. A higher rank dominates
// a lower one; only same-or-lower transitions are self-authorable (demotion),
// and VERIFIED is unreachable without bound sealed evidence (the provenance
// lint). Unknown statuses rank -1 so they never compare as "at most".
func claimStatusRank(status string) int {
	switch status {
	case ClaimStatusDesigned:
		return 0
	case ClaimStatusAsserted:
		return 1
	case ClaimStatusVerified:
		return 2
	default:
		return -1
	}
}

// claimStatusOrder lists the lattice from low to high for error messages.
var claimStatusOrder = []string{ClaimStatusDesigned, ClaimStatusAsserted, ClaimStatusVerified}

// Claim is one row of a claim_ledger value object (RFC 0134). It is parsed from
// the ledger's claims[] front-matter list and carries the bound evidence the
// provenance lint cross-checks against the asserted status.
type Claim struct {
	// ID is the stable, ledger-unique claim identifier. A demotion or
	// auto-decay carries the same ID forward across ledger seals.
	ID string
	// Status is one of the lattice rungs (DESIGNED|ASSERTED|VERIFIED).
	Status string
	// Text is the human-readable claim statement.
	Text string
	// BoundInputDigest is the content digest of the inputs this claim is bound
	// to (e.g. a worktree tree-sha or a hash over the witnessed files). A
	// VERIFIED claim records it so the read/validation surface can detect drift:
	// when the bound inputs change, VERIFIED auto-decays to ASSERTED. Empty for
	// a claim that binds no inputs (which then cannot be VERIFIED).
	BoundInputDigest string
	// ReceiptRef is the repo-relative path (or artifact ref) of the sealed
	// receipt that earns VERIFIED. Required for a VERIFIED claim; ignored below.
	ReceiptRef string
	// EvidenceDigest is the sealed receipt's own digest (the receipt seal). For
	// a VERIFIED claim it must equal the bound input digest the receipt sealed,
	// i.e. the receipt is bound to THESE inputs, not a stale set.
	EvidenceDigest string
}

// claimRowKeys is the closed set of keys a claims[] row may carry.
var claimRowKeys = map[string]bool{
	"id":                 true,
	"status":             true,
	"text":               true,
	"bound_input_digest": true,
	"receipt_ref":        true,
	"evidence_digest":    true,
}

// ParseClaims validates a claims[] block and returns the typed rows. It
// enforces the per-row shape (closed key set, required id/status/text, valid
// status enum, unique ids) but NOT the lattice/provenance rules — those are
// LintClaimLedger, so a caller can parse-then-lint with one shared schema. A
// nil or empty value yields an error (a claim_ledger must carry at least one
// claim).
func ParseClaims(value any) ([]Claim, error) {
	rows, ok := mapList(value)
	if !ok {
		return nil, fmt.Errorf("claim_ledger artifact front matter field 'claims' must be a non-empty list of objects")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("claim_ledger artifact front matter field 'claims' must declare at least one claim")
	}
	result := make([]Claim, 0, len(rows))
	seen := map[string]bool{}
	for idx, row := range rows {
		extra := []string{}
		for key := range row {
			if !claimRowKeys[key] {
				extra = append(extra, key)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d] has unknown fields: %s", idx, strings.Join(extra, ", "))
		}
		id, ok := nonEmptyString(row["id"])
		if !ok {
			return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].id must be a non-empty string", idx)
		}
		if seen[id] {
			return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].id %q is duplicated", idx, id)
		}
		seen[id] = true
		status, ok := nonEmptyString(row["status"])
		if !ok || claimStatusRank(status) < 0 {
			return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].status must be one of %s", idx, strings.Join(claimStatusOrder, ", "))
		}
		if _, ok := nonEmptyString(row["text"]); !ok {
			return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].text must be a non-empty string", idx)
		}
		claim := Claim{
			ID:     id,
			Status: status,
			Text:   fmt.Sprint(row["text"]),
		}
		if v, exists := row["bound_input_digest"]; exists {
			text, ok := nonEmptyString(v)
			if !ok {
				return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].bound_input_digest must be a non-empty string", idx)
			}
			claim.BoundInputDigest = text
		}
		if v, exists := row["receipt_ref"]; exists {
			text, ok := nonEmptyString(v)
			if !ok {
				return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].receipt_ref must be a non-empty string", idx)
			}
			claim.ReceiptRef = text
		}
		if v, exists := row["evidence_digest"]; exists {
			text, ok := nonEmptyString(v)
			if !ok {
				return nil, fmt.Errorf("claim_ledger artifact front matter claims[%d].evidence_digest must be a non-empty string", idx)
			}
			claim.EvidenceDigest = text
		}
		result = append(result, claim)
	}
	return result, nil
}

// LintClaimLedger is the provenance lint (RFC 0134 §4 / D227, pure validation):
// it refuses a claim whose asserted status EXCEEDS the evidence it carries. A
// claim may always be DESIGNED or ASSERTED on a producer's word, but VERIFIED
// is the sealed mint — it requires a bound receipt AND a matching input digest,
// so a lane cannot author its way to the top rung. This is the lane analogue of
// the dogfood product's own dependency-guard: completion language above the
// status the evidence earns is rejected (exit code 6 at the publisher).
//
// The rules, per claim:
//   - DESIGNED / ASSERTED: no evidence required.
//   - VERIFIED: requires a non-empty receipt_ref, a non-empty bound_input_digest,
//     a non-empty evidence_digest, AND evidence_digest == bound_input_digest
//     (the receipt is bound to THESE inputs). A VERIFIED claim whose receipt
//     digest does not match its bound inputs is "decayed" — the lint reports it
//     as exceeding its evidence rather than silently demoting (the writer-side
//     auto-decay in pkg/mutations is what rewrites it; the lint refuses an
//     author who hand-writes an unbacked VERIFIED).
func LintClaimLedger(claims []Claim) error {
	violations := []string{}
	for _, claim := range claims {
		if claim.Status != ClaimStatusVerified {
			continue
		}
		switch {
		case claim.ReceiptRef == "":
			violations = append(violations, fmt.Sprintf("%s (VERIFIED without a receipt_ref)", claim.ID))
		case claim.BoundInputDigest == "":
			violations = append(violations, fmt.Sprintf("%s (VERIFIED without a bound_input_digest)", claim.ID))
		case claim.EvidenceDigest == "":
			violations = append(violations, fmt.Sprintf("%s (VERIFIED without an evidence_digest)", claim.ID))
		case claim.EvidenceDigest != claim.BoundInputDigest:
			violations = append(violations, fmt.Sprintf("%s (VERIFIED but evidence_digest does not match bound_input_digest — decayed)", claim.ID))
		}
	}
	if len(violations) == 0 {
		return nil
	}
	sort.Strings(violations)
	return fmt.Errorf(
		"claim_ledger provenance lint fails closed: claim status exceeds evidence: %s; a claim is VERIFIED only with a bound receipt_ref and an evidence_digest matching its bound_input_digest — otherwise assert ASSERTED or DESIGNED — see docs/reference/spec.md#artifact-front-matter-schemas",
		strings.Join(violations, ", "),
	)
}

// EffectiveClaimStatus returns the status a claim actually earns given its
// evidence, applying VERIFIED→ASSERTED auto-decay: a claim authored VERIFIED
// whose receipt no longer binds its current inputs (evidence_digest mismatch or
// absent) reads back as ASSERTED. DESIGNED and ASSERTED are returned unchanged.
// This is the read/validation surface D227 calls for: VERIFIED auto-decays when
// the bound input hashes change. It does not mutate; callers decide whether to
// persist the decayed status.
func EffectiveClaimStatus(claim Claim) string {
	if claim.Status != ClaimStatusVerified {
		return claim.Status
	}
	if claim.ReceiptRef == "" || claim.BoundInputDigest == "" || claim.EvidenceDigest == "" {
		return ClaimStatusAsserted
	}
	if claim.EvidenceDigest != claim.BoundInputDigest {
		return ClaimStatusAsserted
	}
	return ClaimStatusVerified
}

// ClaimStatusDominates reports whether status `a` is at least as high on the
// lattice as status `b` (a >= b). Unknown statuses never dominate. Exported so
// the daemon-writer monotonic check (pkg/mutations) shares one ordering.
func ClaimStatusDominates(a, b string) bool {
	ra, rb := claimStatusRank(a), claimStatusRank(b)
	return ra >= 0 && rb >= 0 && ra >= rb
}

// ClaimStatusRank exposes the lattice height for the daemon-writer
// self-promotion guard, so pkg/mutations does not re-encode the ordering.
func ClaimStatusRank(status string) int { return claimStatusRank(status) }

// validateClaimLedger is the kind-specific validator wired into
// validateKindSpecific: it parses the claims[] block and runs the provenance
// lint, so an invalid claim_ledger is refused at the publisher with exit code 6.
func validateClaimLedger(parsed map[string]any) error {
	claims, err := ParseClaims(parsed["claims"])
	if err != nil {
		return err
	}
	return LintClaimLedger(claims)
}

// ClaimsFromFrontMatter parses (without re-linting) the claims[] block of an
// already-validated claim_ledger front-matter map, for the daemon-writer
// monotonic/decay comparison in pkg/mutations. It returns nil on any parse
// problem (the front matter was validated at publish time; this is a read).
func ClaimsFromFrontMatter(parsed map[string]any) []Claim {
	claims, err := ParseClaims(parsed["claims"])
	if err != nil {
		return nil
	}
	return claims
}

// LedgerSeal returns the append-only monotonic seal of a claim_ledger front
// matter map, and whether it was present and a non-negative integer.
func LedgerSeal(parsed map[string]any) (int, bool) {
	v, ok := intValue(parsed["ledger_seal"])
	if !ok || v < 0 {
		return 0, false
	}
	return v, true
}
