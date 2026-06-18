package mutations

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// enforceClaimLedgerLattice is the daemon-writer half of the RFC 0134 / D227
// claim-status lattice (the validation-only slice — NO command execution, NO
// verify job type). The artifactcontracts layer already validated the ledger's
// per-row shape and ran the provenance lint (status may not exceed evidence).
// This writer-side check adds the lattice's cross-seal rules, which need the
// PRIOR ledger to evaluate:
//
//   - MONOTONIC, append-only: a new claim_ledger's ledger_seal must STRICTLY
//     EXCEED the most recent prior ledger's seal for the same run. The ledger is
//     append-only — a republish at the same or a lower seal is refused, so the
//     ledger's seal sequence is a total order the reader can trust.
//   - Demotable but NEVER self-promotable: across seals, a claim (matched by id)
//     may be LOWERED on the lattice (VERIFIED → ASSERTED → DESIGNED) but never
//     RAISED by the same author. Promotion to a higher rung is the "sealed mint":
//     it requires bound sealed evidence, which the provenance lint already
//     gates — but a lane cannot hop a prior ASSERTED claim up to VERIFIED across
//     a reseal without that evidence, and even WITH a receipt the executable
//     verifier (a later, off-gate-path slice) is what legitimately mints
//     VERIFIED. In this slice the writer refuses ANY raise across seals.
//   - VERIFIED auto-decay surfacing: the provenance lint already refuses a
//     VERIFIED claim whose evidence_digest does not bind its current inputs, so
//     a claim whose bound inputs changed must be re-authored at its decayed
//     (ASSERTED) status — which the no-self-promotion rule then locks in.
//
// It fails closed (artifact_error / exit code 6) and names the offending claim
// ids so the lane can fix the ledger. It is a pure validation: it reads prior
// ledgers, never executes anything.
func enforceClaimLedgerLattice(ctx context.Context, runner any, repositoryID string, job map[string]any, kind, path string, payload []byte) error {
	if kind != "claim_ledger" {
		return nil
	}
	block, ok := artifactcontracts.FrontMatterBlock(string(payload))
	if !ok {
		return nil
	}
	parsed, err := artifactcontracts.ParseFrontMatterBlock(block)
	if err != nil {
		// The front matter already passed ValidateFrontMatter; a parse failure here
		// is not this gate's concern.
		return nil
	}
	newSeal, ok := artifactcontracts.LedgerSeal(parsed)
	if !ok {
		// ledger_seal is a required, value-checked field; if it is absent the
		// front-matter validation already rejected it.
		return nil
	}
	newClaims := artifactcontracts.ClaimsFromFrontMatter(parsed)

	prior, priorSeal, found, err := latestPriorClaimLedger(ctx, runner, repositoryID, job, path)
	if err != nil {
		return err
	}
	if !found {
		// First ledger for the run; no cross-seal rule to enforce. The per-row
		// shape and provenance lint already ran at the contract layer.
		return nil
	}

	if newSeal <= priorSeal {
		return rpc.NewError("artifact_error", fmt.Sprintf(
			"claim_ledger is append-only: ledger_seal %d does not exceed the prior ledger's seal %d; a republish must advance the monotonic seal — see docs/reference/spec.md#artifact-front-matter-schemas",
			newSeal, priorSeal,
		), nil)
	}

	priorStatusByID := map[string]string{}
	for _, claim := range prior {
		// Compare against the EFFECTIVE prior status (auto-decay applied) so a
		// prior VERIFIED whose evidence had already decayed cannot be used as a
		// promotion floor.
		priorStatusByID[claim.ID] = artifactcontracts.EffectiveClaimStatus(claim)
	}
	promoted := []string{}
	for _, claim := range newClaims {
		priorStatus, existed := priorStatusByID[claim.ID]
		if !existed {
			// A brand-new claim id; its status is gated only by the provenance lint
			// (already run). There is no prior rung to promote past.
			continue
		}
		// EffectiveClaimStatus on the new claim too: a hand-written VERIFIED that
		// fails to bind reads as ASSERTED, and the provenance lint would have
		// already refused publishing it as VERIFIED, so this compares real rungs.
		newStatus := artifactcontracts.EffectiveClaimStatus(claim)
		if artifactcontracts.ClaimStatusRank(newStatus) > artifactcontracts.ClaimStatusRank(priorStatus) {
			promoted = append(promoted, fmt.Sprintf("%s (%s → %s)", claim.ID, priorStatus, newStatus))
		}
	}
	if len(promoted) > 0 {
		sort.Strings(promoted)
		return rpc.NewError("artifact_error", fmt.Sprintf(
			"claim_ledger lattice is demotable but never self-promotable: claim status raised across seals: %s; a claim may only be lowered (VERIFIED → ASSERTED → DESIGNED) by its author — promotion to a higher rung requires sealed evidence minted off the gate path — see docs/reference/spec.md#artifact-front-matter-schemas",
			strings.Join(promoted, ", "),
		), nil)
	}
	return nil
}

// latestPriorClaimLedger loads the most recent claim_ledger for the job's run
// (excluding the artifact being published at `currentPath`) and returns its
// claims, its seal, and whether one was found. Ledgers are scanned newest-first;
// the first readable claim_ledger with a valid seal wins. A ledger whose file
// is gone or unparseable is skipped (the live append-only sequence is what the
// monotonic check needs; an unreadable historical row does not block a fresh
// publish).
func latestPriorClaimLedger(ctx context.Context, runner any, repositoryID string, job map[string]any, currentPath string) ([]artifactcontracts.Claim, int, bool, error) {
	runID := fmt.Sprint(job["run_id"])
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
	if err != nil {
		return nil, 0, false, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	rows, err := queryRows(ctx, runner, `
		SELECT repo_path
		  FROM striatumd.artifacts
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND artifact_kind = 'claim_ledger'
		 ORDER BY created_at DESC, artifact_id DESC`, repositoryID, runID)
	if err != nil {
		return nil, 0, false, err
	}
	for _, row := range rows {
		repoPath := fmt.Sprint(row["repo_path"])
		resolved, err := repoRelativePath(repoRoot, repoPath, false)
		if err != nil {
			continue
		}
		// Skip the artifact currently being published (it may already have a row
		// from an earlier attempt at the same path, or be self-referential).
		if sameResolvedPath(resolved, currentPath) {
			continue
		}
		ledgerBytes, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		block, ok := artifactcontracts.FrontMatterBlock(string(ledgerBytes))
		if !ok {
			continue
		}
		parsed, err := artifactcontracts.ParseFrontMatterBlock(block)
		if err != nil {
			continue
		}
		seal, ok := artifactcontracts.LedgerSeal(parsed)
		if !ok {
			continue
		}
		return artifactcontracts.ClaimsFromFrontMatter(parsed), seal, true, nil
	}
	return nil, 0, false, nil
}

// sameResolvedPath reports whether two filesystem paths point at the same file,
// tolerating one being relative-cleaned. It compares the cleaned suffixes so a
// publish does not count its own (possibly already-recorded) artifact as the
// prior ledger.
func sameResolvedPath(a, b string) bool {
	return artifactcontracts.CleanMarkdownPath(a) == artifactcontracts.CleanMarkdownPath(b)
}
