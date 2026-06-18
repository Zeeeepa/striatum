package mutations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/verifier"
)

// writeReceiptFile mints a receipt.v1 document with the given posture/agreement
// and writes it under the run repo at receiptRel. It returns the receipt's seal
// digest so the test can bind a claim's evidence_digest to it.
func writeReceiptFile(t *testing.T, repoRoot, receiptRel, checkID, treeSHA string, exitCode int, strict, agreement bool) string {
	t.Helper()
	r := verifier.Receipt{
		CheckID:         checkID,
		Argv:            []string{"/bin/true"},
		BinarySHA256:    "abc",
		ExitCode:        exitCode,
		StdoutSHA256:    "def",
		CwdTreeSHA:      treeSHA,
		CreatedAt:       "2026-06-18T00:00:00Z",
		Posture:         verifier.Posture{Mechanism: verifier.MechanismBubblewrap, Strict: strict},
		AgreementSignal: agreement,
	}
	// The seal the receipt carries is keyed to the worktree tree-sha so the gate's
	// bound-input check (evidence_digest == seal_digest == bound_input_digest)
	// holds: the claim binds the receipt that sealed THESE inputs.
	r.SealDigest = treeSHA
	full := filepath.Join(repoRoot, receiptRel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := r.MarshalFrontMatter()
	if err := os.WriteFile(full, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return treeSHA
}

func claimVerificationEntry(t *testing.T, entries []map[string]any, claimID string) map[string]any {
	t.Helper()
	for _, e := range entries {
		if fmt.Sprint(e["claim_id"]) == claimID {
			return e
		}
	}
	t.Fatalf("no claim_verification entry for %q in %#v", claimID, entries)
	return nil
}

// TestRunClaimVerificationVerifiedWithTwoSignalReceipt verifies the gate READ:
// a VERIFIED-authored claim bound to a strict, agreeing, exit-0 receipt reads
// back as VERIFIED. This is the executable-half completion read — the daemon
// reads the sealed receipt; it executes nothing.
func TestRunClaimVerificationVerifiedWithTwoSignalReceipt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	const treeSHA = "treesha_verified"
	writeReceiptFile(t, repoRoot, "docs/run/receipts/C1.md", "go-test", treeSHA, 0, true, true)

	body := claimLedgerDoc("0", `  - id: C1
    status: VERIFIED
    text: "go test passes under a sealed two-signal receipt."
    bound_input_digest: "`+treeSHA+`"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "`+treeSHA+`"
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", body); err != nil {
		t.Fatalf("publish ledger: %v", err)
	}

	entries, err := evaluateRunClaimVerification(ctx, runner, "repo_claim", "run_claim")
	if err != nil {
		t.Fatalf("evaluateRunClaimVerification: %v", err)
	}
	entry := claimVerificationEntry(t, entries, "C1")
	if got := fmt.Sprint(entry["effective_status"]); got != "VERIFIED" {
		t.Fatalf("a strict two-signal receipt must read VERIFIED, got %q (basis %v)", got, entry["verification_basis"])
	}
	if got := fmt.Sprint(entry["verification_basis"]); got != "two_signal_sealed_receipt" {
		t.Fatalf("basis = %q, want two_signal_sealed_receipt", got)
	}
}

// TestRunClaimVerificationDegradesOnMissingReceipt verifies the load-bearing
// D227 rule: a missing/wedged verify degrades the claim to ASSERTED and NEVER
// blocks completion — the gate reads, finds no receipt, and records ASSERTED.
func TestRunClaimVerificationDegradesOnMissingReceipt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)
	_ = repoRoot

	const treeSHA = "treesha_missing"
	// Author VERIFIED with a receipt_ref that is never written (the verifier lane
	// wedged / timed out). The provenance lint passes (the ledger is internally
	// consistent), but the gate read finds no receipt bytes.
	body := claimLedgerDoc("0", `  - id: C1
    status: VERIFIED
    text: "claims verified, but the verifier lane never produced the receipt."
    bound_input_digest: "`+treeSHA+`"
    receipt_ref: "docs/run/receipts/MISSING.md"
    evidence_digest: "`+treeSHA+`"
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", body); err != nil {
		t.Fatalf("publish ledger: %v", err)
	}

	entries, err := evaluateRunClaimVerification(ctx, runner, "repo_claim", "run_claim")
	if err != nil {
		t.Fatalf("evaluateRunClaimVerification: %v", err)
	}
	entry := claimVerificationEntry(t, entries, "C1")
	if got := fmt.Sprint(entry["effective_status"]); got != "ASSERTED" {
		t.Fatalf("a missing receipt must degrade to ASSERTED, got %q", got)
	}
	if got := fmt.Sprint(entry["verification_basis"]); got != "receipt_unavailable" {
		t.Fatalf("basis = %q, want receipt_unavailable", got)
	}
}

// TestRunClaimVerificationDegradesOnNonStrictReceipt verifies a lone-signal
// receipt (passing but non-strict sandbox, no agreement) reads back as ASSERTED,
// never VERIFIED — a lone exit-0 earns only ASSERTED.
func TestRunClaimVerificationDegradesOnNonStrictReceipt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	const treeSHA = "treesha_nonstrict"
	// exit-0 but non-strict sandbox and no agreement → only one signal.
	writeReceiptFile(t, repoRoot, "docs/run/receipts/C1.md", "go-test", treeSHA, 0, false, false)

	body := claimLedgerDoc("0", `  - id: C1
    status: VERIFIED
    text: "passing but under a non-strict envelope with no agreement."
    bound_input_digest: "`+treeSHA+`"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "`+treeSHA+`"
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", body); err != nil {
		t.Fatalf("publish ledger: %v", err)
	}

	entries, err := evaluateRunClaimVerification(ctx, runner, "repo_claim", "run_claim")
	if err != nil {
		t.Fatalf("evaluateRunClaimVerification: %v", err)
	}
	entry := claimVerificationEntry(t, entries, "C1")
	if got := fmt.Sprint(entry["effective_status"]); got != "ASSERTED" {
		t.Fatalf("a non-strict lone-signal receipt must read ASSERTED, got %q (basis %v)", got, entry["verification_basis"])
	}
}

// TestRunClaimVerificationPassesThroughLowerRungs verifies DESIGNED / ASSERTED
// claims need no receipt and pass through at their authored rung (no read).
func TestRunClaimVerificationPassesThroughLowerRungs(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	body := claimLedgerDoc("0", `  - id: C1
    status: DESIGNED
    text: "we intend to."
  - id: C2
    status: ASSERTED
    text: "a structured read says so."
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", body); err != nil {
		t.Fatalf("publish ledger: %v", err)
	}

	entries, err := evaluateRunClaimVerification(ctx, runner, "repo_claim", "run_claim")
	if err != nil {
		t.Fatalf("evaluateRunClaimVerification: %v", err)
	}
	if got := fmt.Sprint(claimVerificationEntry(t, entries, "C1")["effective_status"]); got != "DESIGNED" {
		t.Fatalf("C1 must pass through DESIGNED, got %q", got)
	}
	if got := fmt.Sprint(claimVerificationEntry(t, entries, "C2")["effective_status"]); got != "ASSERTED" {
		t.Fatalf("C2 must pass through ASSERTED, got %q", got)
	}
}

// TestRunClaimVerificationNoLedgerIsNoOp verifies a run with no claim_ledger
// yields no entries (the gate is unaffected).
func TestRunClaimVerificationNoLedgerIsNoOp(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	_ = setupClaimLedgerRun(t, ctx, runner)

	entries, err := evaluateRunClaimVerification(ctx, runner, "repo_claim", "run_claim")
	if err != nil {
		t.Fatalf("evaluateRunClaimVerification: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("a run with no claim_ledger must yield no entries, got %#v", entries)
	}
}
