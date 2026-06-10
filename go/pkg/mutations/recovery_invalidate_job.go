package mutations

import (
	"context"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// HandleRecoveryInvalidateJob is the RFC 0118 P1-6 (GH #240) per-job
// invalidate: given a completed verdict-capable job whose review is
// compromised, supersede its verdict under a scoped operator decision and
// reopen just that job — the run stays running and the fresh attempt must
// itself pass the attested admission gate to re-complete.
//
// The durable invalidation receipt is the superseded verdict ROW (open
// question (a), resolved here): the row keeps its frozen P0-1 provenance
// stamp and gains superseded_by_decision_id/superseded_at, so the
// invalidation is reconstructable from a durable row — not only from the
// events log — and the run_completion_record cites it. The reopen therefore
// uses the verdict-preserving reset variant; the historical reset's DELETE
// would destroy the receipt one step after it was written.
//
// Decision validation reuses requireClaimOverrideDecision (#222): an
// accepting decision scoped to the exact (session_id, job_id) of the
// compromised verdict. A broad decision is refused.
func HandleRecoveryInvalidateJob(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	jobID := stringParam(envelope, "job_id")
	decisionID := stringParam(envelope, "decision_id")
	if jobID == "" || decisionID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.invalidate_job requires job_id and decision_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := lockRunForJob(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if !isVerdictCapableJobType(fmt.Sprint(job["job_type"])) {
			return nil, rpc.NewError("invalid_transition", "recovery.invalidate_job is valid only for verdict-capable jobs", nil)
		}
		if state := fmt.Sprint(job["state"]); state != "completed" {
			return nil, rpc.NewError("invalid_transition", "recovery.invalidate_job requires a completed job; job is "+state, nil)
		}
		runID := fmt.Sprint(job["run_id"])
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		if state := fmt.Sprint(run["state"]); state != "running" {
			return nil, rpc.NewError("invalid_transition", "recovery.invalidate_job requires a running run (a terminal run is immutable; use decision record --mark-run-compromised); run is "+state, nil)
		}
		verdictRow, err := oneRow(ctx, tx, `
			SELECT verdict_id, session_id, verdict
			  FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2
			   AND superseded_by_decision_id IS NULL
			 ORDER BY created_at DESC, verdict_id DESC
			 LIMIT 1`, repositoryID, jobID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.NewError("invalid_transition", "completed job has no active verdict to invalidate", nil)
		}
		if err != nil {
			return nil, err
		}
		verdictSessionID := fmt.Sprint(verdictRow["session_id"])
		if err := requireClaimOverrideDecision(ctx, tx, repositoryID, runID, verdictSessionID, jobID, decisionID); err != nil {
			return nil, err
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.verdicts
			   SET superseded_by_decision_id = $1, superseded_at = $2
			 WHERE repository_id = $3 AND job_id = $4
			   AND superseded_by_decision_id IS NULL`,
			decisionID, now, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := closeInterrogationTargetForReopen(ctx, tx, repositoryID, runID, jobID); err != nil {
			return nil, err
		}
		if err := resetDownstreamForRevision(ctx, tx, repositoryID, jobID, now); err != nil {
			return nil, err
		}
		if err := resetJobToBlockedPreservingVerdicts(ctx, tx, repositoryID, jobID, now, "job_invalidated"); err != nil {
			return nil, err
		}
		messageID, err := enqueueJob(ctx, tx, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.job_invalidated", nil, jobID, nil, nil, nil, map[string]any{
			"decision_id":            decisionID,
			"invalidated_verdict_id": verdictRow["verdict_id"],
			"invalidated_verdict":    verdictRow["verdict"],
			"verdict_session_id":     verdictSessionID,
			"reopened_attempt":       intValue(job["attempt"]) + 1,
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":                 "invalidated",
			"job_id":                 jobID,
			"run_id":                 runID,
			"decision_id":            decisionID,
			"invalidated_verdict_id": verdictRow["verdict_id"],
			"reopened_attempt":       intValue(job["attempt"]) + 1,
			"message_id":             messageID,
		}, nil
	})
}
