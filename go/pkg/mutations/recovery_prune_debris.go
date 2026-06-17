package mutations

import (
	"context"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// HandleRecoveryPruneDebris is GH #303: the operator verb that prunes terminal-run
// artifact debris so `doctor` can report `ok` again.
//
// Background. The doctor artifact-anchor pass reclassifies an integrity finding
// on an artifact of a terminal (canceled/failed) run into the
// artifact_debris_terminal_run WARNING when the artifact's files are GONE from
// every durable ref and the default branch (unrecoverable archived leftover).
// Those warnings keep the report `degraded` forever: there is no live job to
// recover, the files cannot be restored, and the artifact row cannot be deleted —
// striatumd.artifacts is owner-owned and append-only (delete/update triggers,
// runtime role has only SELECT). Direct PostgreSQL deletion is against the
// product boundary.
//
// prune-debris closes that gap WITHOUT a hard delete. It records an append-only
// recovery.debris_pruned event (Model A tombstone) keyed on each pruned
// artifact_id; the doctor pass then suppresses tombstoned artifacts, so a pruned
// run's debris stops degrading the report. The artifact row, its provenance, and
// the tombstone event are all preserved in the durable chain.
//
// Eligibility is byte-identical to what doctor reports: it reuses
// reads.ArtifactDebrisVerdicts, which runs the exact same per-row classifiers the
// doctor pass uses. An artifact is pruned only when (a) the run is in a terminal
// debris state, (b) the underlying integrity finding is a debris code, and (c) no
// preservation guard (file present on the default branch, anchored on a durable
// ref, in default-branch history, or in the acknowledged-loss baseline) matched —
// all three folded into the classifier's artifact_debris_terminal_run verdict.
//
// Guards mirror the recovery-capability handlers: requireRepositoryID + run_id,
// withTx + lockRun, and a hard REFUSE with invalid_transition when the run is not
// terminal (it never prunes an active or completed run). --dry-run previews the
// partition and writes nothing. It is idempotent: an already-tombstoned artifact
// is skipped, so pruned_count reflects only NEW prunes.
func HandleRecoveryPruneDebris(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.prune_debris requires run_id", nil)
	}
	dryRun := boolParam(envelope, "dry_run")
	sweepPins := boolParam(envelope, "sweep_pins")

	// Eligibility classification is read-only (SELECT + bounded git probes) and
	// reuses the doctor's own per-row classifiers via reads.ArtifactDebrisVerdicts,
	// so the prune partition can never drift from what doctor reports. It runs
	// against the handler runner (not the tx), exactly like the doctor pass; the
	// idempotency re-check inside the tx keys on committed tombstone events, so a
	// concurrent prune cannot double-tombstone.
	verdicts, err := reads.ArtifactDebrisVerdicts(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first (mirrors HandleRecoveryReseal).
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.NewError("not_found", fmt.Sprintf("could not find runs row for %q", runID), nil)
		}
		if err != nil {
			return nil, err
		}
		runState := fmt.Sprint(run["state"])
		// Run-state gate: only terminal/abandoned (canceled/failed) runs carry
		// prunable debris. Refuse anything else (active/completed) so the verb can
		// never tombstone a live or successful run's artifacts.
		if !reads.ArtifactDebrisRunStateTerminal(runState) {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf(
				"run is %s; prune-debris applies only to a terminal-debris run (canceled/failed)", runState), nil)
		}

		// Idempotency: skip artifacts already tombstoned by an earlier prune.
		alreadyPruned, err := debrisPrunedArtifactIDsForRun(ctx, tx, repositoryID, runID)
		if err != nil {
			return nil, err
		}

		pruned := []map[string]any{}
		retained := []map[string]any{}
		alreadyPrunedEntries := []map[string]any{}
		for _, v := range verdicts {
			entry := map[string]any{
				"artifact_id":    v.ArtifactID,
				"run_id":         v.RunID,
				"job_id":         v.JobID,
				"repo_path":      v.RepoPath,
				"content_sha256": v.ContentSHA256,
				"doctor_code":    v.Code,
			}
			if !v.Eligible {
				entry["reason"] = v.Reason
				retained = append(retained, entry)
				continue
			}
			if alreadyPruned[v.ArtifactID] {
				entry["reason"] = "already_pruned"
				alreadyPrunedEntries = append(alreadyPrunedEntries, entry)
				continue
			}
			pruned = append(pruned, entry)
		}

		base := map[string]any{
			"run_id":               runID,
			"run_state":            runState,
			"retained":             retained,
			"retained_count":       len(retained),
			"already_pruned":       alreadyPrunedEntries,
			"already_pruned_count": len(alreadyPrunedEntries),
		}

		if dryRun {
			base["status"] = "would_prune"
			base["would_prune"] = pruned
			base["pruned_count"] = len(pruned)
			base["next_actions"] = []string{"recovery prune-debris (omit --dry-run to tombstone)"}
			return base, nil
		}

		for _, entry := range pruned {
			artifactID := fmt.Sprint(entry["artifact_id"])
			jobID := fmt.Sprint(entry["job_id"])
			// Append-only tombstone (NEVER a delete): the artifact_id is set on the
			// event column (FK to artifacts) so the doctor pass suppresses it with a
			// single set query.
			if _, err := appendEvent(ctx, tx, repositoryID, runID, reads.DebrisPrunedEventType, nil, jobID, nil, artifactID, nil, map[string]any{
				"run_id":         runID,
				"job_id":         jobID,
				"artifact_id":    artifactID,
				"repo_path":      entry["repo_path"],
				"content_sha256": entry["content_sha256"],
				"doctor_code":    entry["doctor_code"],
				"source":         "recovery.prune_debris",
			}); err != nil {
				return nil, err
			}
		}

		base["status"] = "pruned"
		base["pruned"] = pruned
		base["pruned_count"] = len(pruned)
		base["next_actions"] = []string{"run doctor to confirm the debris is cleared"}

		if sweepPins {
			repoRoot := fmt.Sprint(run["repo_root"])
			deleted, retainedPins, err := sweepRunPins(ctx, repoRoot, runID,
				fmt.Sprint(run["branch_name"]), fmt.Sprint(run["branch_base"]))
			if err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "worktree.pins_swept", nil, nil, nil, nil, nil, map[string]any{
				"deleted":        deleted,
				"retained":       retainedPins,
				"deleted_count":  len(deleted),
				"retained_count": len(retainedPins),
				"source":         "recovery.prune_debris",
			}); err != nil {
				return nil, err
			}
			base["pins_swept"] = true
			base["pins_deleted"] = deleted
			base["pins_retained"] = retainedPins
			base["pins_deleted_count"] = len(deleted)
			base["pins_retained_count"] = len(retainedPins)
		}
		return base, nil
	})
}

// debrisPrunedArtifactIDsForRun loads the set of artifact_ids already tombstoned
// by a recovery.debris_pruned event for the run, FOR the idempotency check. It
// scopes by run_id (the prune is per-run) so a run with many siblings still
// issues one bounded query.
func debrisPrunedArtifactIDsForRun(ctx context.Context, runner any, repositoryID, runID string) (map[string]bool, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT DISTINCT artifact_id
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND event_type = $3
		   AND artifact_id IS NOT NULL`,
		repositoryID, runID, reads.DebrisPrunedEventType,
	)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(rows))
	for _, row := range rows {
		if id := fmt.Sprint(row["artifact_id"]); id != "" && id != "<nil>" {
			set[id] = true
		}
	}
	return set, nil
}
