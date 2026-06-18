package mutations

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/verifier"
)

// RFC 0134 / D227 — the EXECUTABLE-half gate READ. The run-completion gate must
// allow a claim's VERIFIED status by READING the durable receipt the disposable
// sandboxed verifier lane minted — it must NEVER execute a check itself, and a
// missing / wedged / timed-out verify must degrade the claim to ASSERTED rather
// than block completion on engine liveness.
//
// evaluateRunClaimVerification is therefore a PURE READ that NEVER fails the
// gate: it loads the run's latest claim_ledger, and for each VERIFIED-authored
// claim loads the receipt it names, applies the two-signal rule
// (verifier.EffectiveStatusFromReceipt), and returns a per-claim ledger of
// EFFECTIVE statuses (VERIFIED only when the sealed receipt binds the claim's
// inputs AND records an independent re-execution agreement under a strict
// sandbox; ASSERTED otherwise). The caller records this ledger into the
// run.completed event for legibility; it adds NO failing gate and routes NOTHING
// to needs_operator. There is intentionally NO command execution on this path —
// the daemon validates sealed bytes, it does not run checks.
func evaluateRunClaimVerification(ctx context.Context, runner any, repositoryID, runID string) ([]map[string]any, error) {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
	if err != nil {
		return nil, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])

	ledgerRows, err := queryRows(ctx, runner, `
		SELECT repo_path
		  FROM striatumd.artifacts
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND artifact_kind = 'claim_ledger'
		 ORDER BY created_at DESC, artifact_id DESC
		 LIMIT 1`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	if len(ledgerRows) == 0 {
		// No claim_ledger for the run; nothing to evaluate. The gate is unaffected.
		return nil, nil
	}

	claims, ok := loadClaimsFromLedgerRow(repoRoot, ledgerRows[0])
	if !ok || len(claims) == 0 {
		return nil, nil
	}

	entries := make([]map[string]any, 0, len(claims))
	for _, claim := range claims {
		entry := map[string]any{
			"claim_id":         claim.ID,
			"authored_status":  claim.Status,
			"effective_status": artifactcontracts.EffectiveClaimStatus(claim),
		}
		// Only a claim AUTHORED VERIFIED needs a receipt read; DESIGNED / ASSERTED
		// carry no sealed evidence and pass through at their authored rung.
		if claim.Status == artifactcontracts.ClaimStatusVerified {
			effective, basis := effectiveStatusForVerifiedClaim(repoRoot, claim)
			entry["effective_status"] = effective
			entry["verification_basis"] = basis
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return fmt.Sprint(entries[i]["claim_id"]) < fmt.Sprint(entries[j]["claim_id"])
	})
	return entries, nil
}

// effectiveStatusForVerifiedClaim loads the receipt a VERIFIED-authored claim
// names and applies the two-signal rule. It NEVER errors out the gate — every
// failure path (no receipt_ref, receipt body gone, unparseable, seal mismatch,
// non-strict posture, missing agreement) degrades to ASSERTED with a recorded
// basis, so a missing/wedged verify can never block completion.
func effectiveStatusForVerifiedClaim(repoRoot string, claim artifactcontracts.Claim) (status, basis string) {
	if claim.ReceiptRef == "" {
		return artifactcontracts.ClaimStatusAsserted, "no_receipt_ref"
	}
	resolved, err := repoRelativePath(repoRoot, claim.ReceiptRef, false)
	if err != nil {
		return artifactcontracts.ClaimStatusAsserted, "receipt_ref_invalid"
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		// The receipt is missing or wedged. D227: degrade, never block on liveness.
		return artifactcontracts.ClaimStatusAsserted, "receipt_unavailable"
	}
	sig, err := verifier.ReceiptSignalsFromDocument(string(body))
	if err != nil {
		return artifactcontracts.ClaimStatusAsserted, "receipt_unparseable"
	}
	effective := verifier.EffectiveStatusFromReceipt(sig, claim.BoundInputDigest, claim.EvidenceDigest)
	if effective == artifactcontracts.ClaimStatusVerified {
		return effective, "two_signal_sealed_receipt"
	}
	return effective, verificationDegradeBasis(sig, claim)
}

// verificationDegradeBasis names WHY a VERIFIED-authored claim only earns
// ASSERTED on the read, so the recorded ledger is legible to the operator.
func verificationDegradeBasis(sig verifier.ReceiptSignals, claim artifactcontracts.Claim) string {
	switch {
	case claim.BoundInputDigest == "" || claim.EvidenceDigest == "":
		return "claim_unbound"
	case claim.EvidenceDigest != claim.BoundInputDigest:
		return "evidence_digest_decayed"
	case sig.SealDigest != claim.EvidenceDigest:
		return "receipt_seal_mismatch"
	case sig.ExitCode != 0:
		return "check_not_passing"
	case !sig.Strict:
		return "sandbox_not_strict"
	case !sig.Agreement:
		return "no_reexecution_agreement"
	default:
		return "single_signal_only"
	}
}

// loadClaimsFromLedgerRow reads and parses the claim_ledger artifact named by an
// artifacts row, returning its claims. It tolerates a gone/unparseable file (a
// read, not a gate). Mirrors the pattern in latestPriorClaimLedger.
func loadClaimsFromLedgerRow(repoRoot string, row map[string]any) ([]artifactcontracts.Claim, bool) {
	resolved, err := repoRelativePath(repoRoot, fmt.Sprint(row["repo_path"]), false)
	if err != nil {
		return nil, false
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, false
	}
	block, ok := artifactcontracts.FrontMatterBlock(string(body))
	if !ok {
		return nil, false
	}
	parsed, err := artifactcontracts.ParseFrontMatterBlock(block)
	if err != nil {
		return nil, false
	}
	return artifactcontracts.ClaimsFromFrontMatter(parsed), true
}
