package artifactcontracts

import (
	"strings"
	"testing"
)

// claimLedgerBody builds a claim_ledger front-matter document with the given
// claims[] YAML block (already indented under "claims:").
func claimLedgerBody(seal string, claimsBlock string) []byte {
	return []byte(`---
schema_version: "striatum.claim_ledger.v1"
artifact_kind: "claim_ledger"
run_id: "run_1"
ledger_seal: ` + seal + `
claims:
` + claimsBlock + `---

# Claims
`)
}

// TestClaimLedgerAcceptsDesignedAndAsserted verifies the two lower lattice rungs
// publish without any evidence (a producer may DESIGN or ASSERT on its word).
func TestClaimLedgerAcceptsDesignedAndAsserted(t *testing.T) {
	body := claimLedgerBody("0", `  - id: C1
    status: DESIGNED
    text: "We intend to gate run completion on a mechanical verdict."
  - id: C2
    status: ASSERTED
    text: "The publisher refuses invalid front matter with exit code 6."
`)
	if err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body); err != nil {
		t.Fatalf("valid DESIGNED/ASSERTED claim_ledger refused: %v", err)
	}
}

// TestClaimLedgerAcceptsVerifiedWithBoundReceipt verifies VERIFIED publishes
// when it carries a bound receipt and an evidence_digest matching its
// bound_input_digest (the sealed mint).
func TestClaimLedgerAcceptsVerifiedWithBoundReceipt(t *testing.T) {
	body := claimLedgerBody("1", `  - id: C1
    status: VERIFIED
    text: "go test ./pkg/artifactcontracts/ passes."
    bound_input_digest: "treesha_abc"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "treesha_abc"
`)
	if err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body); err != nil {
		t.Fatalf("VERIFIED claim_ledger with a bound receipt refused: %v", err)
	}
}

// TestProvenanceLintRefusesVerifiedWithoutReceipt is the core provenance lint:
// a claim authored VERIFIED with no receipt exceeds its evidence and is rejected
// with exit code 6, naming the offending claim id.
func TestProvenanceLintRefusesVerifiedWithoutReceipt(t *testing.T) {
	body := claimLedgerBody("0", `  - id: C1
    status: VERIFIED
    text: "Asserted-as-verified: no witness names this."
`)
	err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body)
	if err == nil {
		t.Fatal("a VERIFIED claim with no receipt must be rejected")
	}
	for _, want := range []string{"provenance lint fails closed", "exceeds evidence", "C1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("rejection must name the provenance lint and the claim; want %q, got: %v", want, err)
		}
	}
}

// TestProvenanceLintRefusesVerifiedWithDigestMismatch verifies a VERIFIED claim
// whose receipt evidence_digest does not match its bound inputs (decayed) is
// refused — the receipt does not bind these inputs.
func TestProvenanceLintRefusesVerifiedWithDigestMismatch(t *testing.T) {
	body := claimLedgerBody("0", `  - id: C1
    status: VERIFIED
    text: "Stale witness."
    bound_input_digest: "treesha_new"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "treesha_OLD"
`)
	err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body)
	if err == nil {
		t.Fatal("a VERIFIED claim whose evidence does not bind its inputs must be rejected")
	}
	if !strings.Contains(err.Error(), "decayed") || !strings.Contains(err.Error(), "C1") {
		t.Fatalf("rejection must report the decay for C1: %v", err)
	}
}

// TestClaimLedgerRejectsUnknownStatus verifies the status enum is closed.
func TestClaimLedgerRejectsUnknownStatus(t *testing.T) {
	body := claimLedgerBody("0", `  - id: C1
    status: PROVEN
    text: "Not a lattice rung."
`)
	err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body)
	if err == nil {
		t.Fatal("an unknown claim status must be rejected")
	}
	for _, want := range []string{"DESIGNED", "ASSERTED", "VERIFIED"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("status rejection must name the lattice; want %q, got: %v", want, err)
		}
	}
}

// TestClaimLedgerRejectsDuplicateIDsAndUnknownKeys guards the per-row shape.
func TestClaimLedgerRejectsDuplicateIDsAndUnknownKeys(t *testing.T) {
	dup := claimLedgerBody("0", `  - id: C1
    status: ASSERTED
    text: "first"
  - id: C1
    status: ASSERTED
    text: "second"
`)
	if err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", dup); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate claim id must be rejected: %v", err)
	}
	unknown := claimLedgerBody("0", `  - id: C1
    status: ASSERTED
    text: "x"
    bogus_key: "y"
`)
	if err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", unknown); err == nil || !strings.Contains(err.Error(), "unknown fields") {
		t.Fatalf("unknown claim row key must be rejected: %v", err)
	}
}

// TestClaimLedgerRequiresAtLeastOneClaim verifies an empty claims[] is rejected.
func TestClaimLedgerRequiresAtLeastOneClaim(t *testing.T) {
	body := []byte(`---
schema_version: "striatum.claim_ledger.v1"
artifact_kind: "claim_ledger"
run_id: "run_1"
ledger_seal: 0
claims: []
---

# Claims
`)
	if err := ValidateFrontMatter("claim_ledger", "CLAIMS.md", body); err == nil {
		t.Fatal("an empty claims[] must be rejected")
	}
}

// TestEffectiveClaimStatusAutoDecays verifies the read-surface VERIFIED→ASSERTED
// auto-decay applied when bound inputs change (evidence no longer binds).
func TestEffectiveClaimStatusAutoDecays(t *testing.T) {
	bound := Claim{ID: "C1", Status: ClaimStatusVerified, BoundInputDigest: "a", ReceiptRef: "r", EvidenceDigest: "a"}
	if got := EffectiveClaimStatus(bound); got != ClaimStatusVerified {
		t.Fatalf("a properly bound VERIFIED claim must read VERIFIED, got %q", got)
	}
	drifted := Claim{ID: "C1", Status: ClaimStatusVerified, BoundInputDigest: "b", ReceiptRef: "r", EvidenceDigest: "a"}
	if got := EffectiveClaimStatus(drifted); got != ClaimStatusAsserted {
		t.Fatalf("a VERIFIED claim whose inputs drifted must auto-decay to ASSERTED, got %q", got)
	}
	noReceipt := Claim{ID: "C1", Status: ClaimStatusVerified, BoundInputDigest: "a", EvidenceDigest: "a"}
	if got := EffectiveClaimStatus(noReceipt); got != ClaimStatusAsserted {
		t.Fatalf("a VERIFIED claim with no receipt must auto-decay to ASSERTED, got %q", got)
	}
	asserted := Claim{ID: "C1", Status: ClaimStatusAsserted}
	if got := EffectiveClaimStatus(asserted); got != ClaimStatusAsserted {
		t.Fatalf("ASSERTED must pass through unchanged, got %q", got)
	}
}

// TestClaimStatusLatticeOrdering verifies the lattice height comparisons the
// daemon-writer self-promotion guard relies on.
func TestClaimStatusLatticeOrdering(t *testing.T) {
	if !ClaimStatusDominates(ClaimStatusVerified, ClaimStatusAsserted) {
		t.Fatal("VERIFIED must dominate ASSERTED")
	}
	if !ClaimStatusDominates(ClaimStatusAsserted, ClaimStatusDesigned) {
		t.Fatal("ASSERTED must dominate DESIGNED")
	}
	if ClaimStatusDominates(ClaimStatusDesigned, ClaimStatusVerified) {
		t.Fatal("DESIGNED must NOT dominate VERIFIED")
	}
	if ClaimStatusRank(ClaimStatusVerified) <= ClaimStatusRank(ClaimStatusAsserted) {
		t.Fatal("VERIFIED rank must exceed ASSERTED rank")
	}
	if ClaimStatusDominates("PROVEN", ClaimStatusDesigned) {
		t.Fatal("an unknown status must never dominate")
	}
}

// TestReceiptContractValidates verifies the receipt evidence artifact's required
// transcript fields are enforced.
func TestReceiptContractValidates(t *testing.T) {
	valid := []byte(`---
schema_version: "striatum.receipt.v1"
artifact_kind: "receipt"
check_id: "go-test-artifactcontracts"
argv: ["go", "test", "./pkg/artifactcontracts/..."]
binary_sha256: "deadbeef"
exit_code: 0
stdout_sha256: "feedface"
cwd_tree_sha: "treesha_abc"
seal_digest: "treesha_abc"
---

# Receipt
`)
	if err := ValidateFrontMatter("receipt", "RECEIPT.md", valid); err != nil {
		t.Fatalf("valid receipt refused: %v", err)
	}
	missing := []byte(`---
schema_version: "striatum.receipt.v1"
artifact_kind: "receipt"
check_id: "go-test"
---

# Receipt
`)
	if err := ValidateFrontMatter("receipt", "RECEIPT.md", missing); err == nil {
		t.Fatal("a receipt missing required transcript fields must be rejected")
	}
}
