package reads

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0135 P3 (D216) — `striatum join verify` (#347). A read-only RPC + CLI verb
// that verifies the integrity of ONE sealed expectation barrier: that its manifest
// (the per-seat sealed contributions) matches the staged refs at the live seal,
// that the assembly-journal intent is consistent with the staged contributions, and
// that no in-edge is a live intervention-needed contribution (the BARRIER_BLOCKED
// condition; dependency-blocked seats are pending, not hard-blocked).
//
// It mirrors the read/verify verb shape of apply.receipt.verify (a read-only
// (ctx, envelope) -> (map[string]any, error) handler registered through
// reads.Register): it never mutates state. It is the operator-facing surface of the
// same barrier integrity invariant the doctor block (doctorBarrierIntegrity) runs
// across all barriers — `join verify` audits one barrier on demand and FAILS (a
// non-nil rpc.Error) when that barrier is corrupted or blocked, so it is usable as a
// CI/operator gate.
//
// The verification reads the migration-0031 striatumd.barrier_status view (the
// single read projection) plus the durable staging rows; it never re-derives the
// authoritative readiness predicate (db.BarrierReadySQL stays authoritative). A
// barrier is `valid` iff:
//
//   - it has a freeze record (a known barrier);
//   - it has NO live blocking in-edge (no BARRIER_BLOCKED condition);
//   - every non-terminal-gap declared seat is live-staged at its live seal (the
//     manifest matches the staged refs at the live seal); and
//   - if the assembly journal is present, its state is consistent (an 'assembling'
//     barrier's journaled target is reachable; a 'committed' barrier's staged seats
//     still cover its non-gap declared seats; a 'failed' barrier is reported as
//     invalid with its fail_reason).

// barrierConditionBlocked is the named BARRIER_BLOCKED condition the
// striatumd.barrier_status view emits when a barrier has a live blocking in-edge.
// It is a derived runtime/doctor condition, NOT a barrier_state CHECK value: the
// barrier_state assembly-journal lifecycle stays sealed/assembling/committed/failed
// (owner-bundle-free, runtime-owned), and "blocked" is an UPSTREAM, pre-seal
// condition over the in-edges that this view derives — keeping it out of the
// barrier_state CHECK avoids touching an owner-held constraint (RFC 0135 P3 / D215).
const barrierConditionBlocked = "BARRIER_BLOCKED"

// HandleJoinVerify verifies one barrier's integrity. It requires repository_id and
// barrier_id; it returns {barrier_id, valid, condition, manifest, blocked_manifest,
// problems}. On a corrupted or blocked barrier it returns a non-nil rpc.Error so a
// CLI/CI caller exits non-zero — the verb is a gate, not just a report.
func HandleJoinVerify(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	barrierID := strings.TrimSpace(stringParam(envelope, "barrier_id"))
	if barrierID == "" {
		return nil, rpc.NewError("schema_invalid", "barrier_id must be a non-empty string", nil)
	}

	rows, err := collectRows(ctx, runner, `
		SELECT barrier_id, run_id, downstream_workflow_job_id, frozen_tip_sha,
		       declared_seats, staged_seats, terminal_gap_seats, blocking_seats,
		       outstanding_seats,
		       COALESCE(assembly_state, '')              AS assembly_state,
		       COALESCE(assembly_target_commit_sha, '')  AS assembly_target_commit_sha,
		       COALESCE(assembly_tree_sha, '')           AS assembly_tree_sha,
		       COALESCE(assembly_fail_reason, '')        AS assembly_fail_reason,
		       condition
		  FROM striatumd.barrier_status
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repositoryID, barrierID,
	)
	if err != nil {
		return nil, rpc.NewError("internal_error", "could not read barrier_status: "+err.Error(), nil)
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("not_found", fmt.Sprintf("no barrier %q (no freeze record) in repository %q", barrierID, repositoryID),
			map[string]any{"barrier_id": barrierID})
	}

	raw := rows[0]
	row := barrierStatusRow{
		BarrierID:               stringFrom(raw, "barrier_id"),
		RunID:                   stringFrom(raw, "run_id"),
		DownstreamWorkflowJobID: stringFrom(raw, "downstream_workflow_job_id"),
		FrozenTipSHA:            stringFrom(raw, "frozen_tip_sha"),
		DeclaredSeats:           intFromAny(raw["declared_seats"]),
		StagedSeats:             intFromAny(raw["staged_seats"]),
		TerminalGapSeats:        intFromAny(raw["terminal_gap_seats"]),
		BlockingSeats:           intFromAny(raw["blocking_seats"]),
		OutstandingSeats:        intFromAny(raw["outstanding_seats"]),
		AssemblyState:           stringFrom(raw, "assembly_state"),
		AssemblyTargetCommitSHA: stringFrom(raw, "assembly_target_commit_sha"),
		AssemblyTreeSHA:         stringFrom(raw, "assembly_tree_sha"),
		AssemblyFailReason:      stringFrom(raw, "assembly_fail_reason"),
		Condition:               stringFrom(raw, "condition"),
	}

	problems := []string{}
	blockedManifest := []blockedInEdge{}

	// BARRIER_BLOCKED: a live blocking in-edge cannot fire without intervention.
	if row.BlockingSeats > 0 || row.Condition == barrierConditionBlocked {
		blockedManifest = loadBlockedManifest(ctx, runner, repositoryID, row.RunID, row.BarrierID)
		row = normalizeBarrierBlockedView(row, len(blockedManifest))
		if len(blockedManifest) > 0 {
			problems = append(problems, fmt.Sprintf(
				"BARRIER_BLOCKED: %d live blocking in-edge(s) cannot fire without intervention", row.BlockingSeats))
		}
	}

	// Manifest matches staged refs at the live seal: every non-terminal-gap declared
	// seat must be live-staged. An outstanding (not staged, not gap) seat means the
	// manifest does NOT yet match the staged refs at the live seal.
	expectedStaged := row.DeclaredSeats - row.TerminalGapSeats
	if expectedStaged < 0 {
		expectedStaged = 0
	}
	if row.StagedSeats < expectedStaged && row.BlockingSeats == 0 {
		problems = append(problems, fmt.Sprintf(
			"manifest incomplete: %d of %d non-gap declared seat(s) live-staged at the live seal (%d outstanding)",
			row.StagedSeats, expectedStaged, row.OutstandingSeats))
	}

	// Assembly-journal consistency.
	switch row.AssemblyState {
	case "assembling":
		repoRoot := doctorRepoRoot(ctx, runner, repositoryID)
		if row.AssemblyTargetCommitSHA != "" && repoRoot != "" &&
			!gitCommitExists(ctx, repoRoot, row.AssemblyTargetCommitSHA) {
			problems = append(problems, fmt.Sprintf(
				"journaled assembly target %s is unreachable in the run repo (corruption / base-loss)",
				row.AssemblyTargetCommitSHA))
		}
	case "committed":
		if row.StagedSeats < expectedStaged {
			problems = append(problems, fmt.Sprintf(
				"committed barrier manifest mismatch: %d of %d non-gap seat(s) still staged at the live seal",
				row.StagedSeats, expectedStaged))
		}
	case "failed":
		reason := row.AssemblyFailReason
		if reason == "" {
			reason = "(no reason recorded)"
		}
		problems = append(problems, "assembly is in terminal state 'failed': "+reason)
	}

	valid := len(problems) == 0
	result := map[string]any{
		"barrier_id":       row.BarrierID,
		"run_id":           row.RunID,
		"valid":            valid,
		"condition":        row.Condition,
		"declared_seats":   row.DeclaredSeats,
		"staged_seats":     row.StagedSeats,
		"terminal_gaps":    row.TerminalGapSeats,
		"outstanding":      row.OutstandingSeats,
		"assembly_state":   row.AssemblyState,
		"manifest":         barrierVerifyManifest(ctx, runner, repositoryID, row.RunID, row.BarrierID),
		"blocked_manifest": manifestToAny(blockedManifest),
		"problems":         problems,
	}

	if !valid {
		detail := map[string]any{
			"barrier_id":       row.BarrierID,
			"condition":        row.Condition,
			"problems":         problems,
			"blocked_manifest": manifestToAny(blockedManifest),
			"result":           result,
		}
		msg := fmt.Sprintf("barrier %q failed verification: %s", row.BarrierID, strings.Join(problems, "; "))
		// A live blocking in-edge is the named BARRIER_BLOCKED failure; any other
		// integrity failure (manifest/journal inconsistency) is barrier_integrity_failed.
		// Literal codes (not a variable) so the error-catalog source-reconciliation
		// guard sees them.
		if row.BlockingSeats > 0 || row.Condition == barrierConditionBlocked {
			return nil, rpc.NewError("barrier_blocked", msg, detail)
		}
		return nil, rpc.NewError("barrier_integrity_failed", msg, detail)
	}
	return result, nil
}

// barrierVerifyManifest snapshots the per-seat sealed contribution for one barrier:
// each declared seat, its live seal (live completed attempt), and the staged ref it
// joined at (or its gap classification). This is the audit projection of the
// join_manifest.v1 in_edges — read directly from the durable rows so the verb's
// answer is grounded in the same data the assembly folds.
func barrierVerifyManifest(ctx context.Context, runner db.Runner, repositoryID, runID, barrierID string) []map[string]any {
	rows, err := collectRows(ctx, runner, `
		WITH declared AS (
		    SELECT sib.value::text AS workflow_job_id
		      FROM striatumd.fanin_freeze_points fp,
		           jsonb_array_elements_text(fp.declared_sibling_job_ids) AS sib(value)
		     WHERE fp.repository_id = $1 AND fp.barrier_id = $2
		)
		SELECT d.workflow_job_id,
		       COALESCE((
		         SELECT MAX(j.attempt) FROM striatumd.jobs j
		          WHERE j.repository_id = $1 AND j.run_id = $3
		            AND j.workflow_job_id = d.workflow_job_id AND j.state = 'completed'
		       ), 0) AS live_seal,
		       COALESCE((
		         SELECT bool_or(j.state = 'canceled'
		                        AND COALESCE(j.write_scope_json->>'quarantined','') = 'true')
		           FROM striatumd.jobs j
		          WHERE j.repository_id = $1 AND j.run_id = $3
		            AND j.workflow_job_id = d.workflow_job_id
		       ), false) AS is_terminal_gap,
		       (SELECT s.staging_ref FROM striatumd.barrier_staged_contributions s
		         WHERE s.repository_id = $1 AND s.barrier_id = $2
		           AND s.workflow_job_id = d.workflow_job_id AND s.status = 'staged'
		           AND s.staging_ref NOT LIKE 'refs/striatum/recovery/%'
		           AND s.attempt = COALESCE((
		                 SELECT MAX(j.attempt) FROM striatumd.jobs j
		                  WHERE j.repository_id = $1 AND j.run_id = $3
		                    AND j.workflow_job_id = d.workflow_job_id AND j.state = 'completed'
		               ), -1)
		         LIMIT 1) AS staged_ref
		  FROM declared d
		 ORDER BY d.workflow_job_id`,
		repositoryID, barrierID, runID,
	)
	if err != nil {
		return []map[string]any{}
	}
	manifest := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		seat := stringFrom(r, "workflow_job_id")
		stagedRef := stringFrom(r, "staged_ref")
		terminalGap := false
		switch b := r["is_terminal_gap"].(type) {
		case bool:
			terminalGap = b
		}
		status := "outstanding"
		switch {
		case terminalGap:
			status = "terminal_gap"
		case stagedRef != "":
			status = "staged_live"
		}
		manifest = append(manifest, map[string]any{
			"workflow_job_id": seat,
			"live_seal":       intFromAny(r["live_seal"]),
			"status":          status,
			"staging_ref":     stagedRef,
		})
	}
	sort.Slice(manifest, func(i, j int) bool {
		return stringFrom(manifest[i], "workflow_job_id") < stringFrom(manifest[j], "workflow_job_id")
	})
	return manifest
}
