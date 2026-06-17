package mutations

import (
	"context"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// HandleRecoveryAcceptQuarantined is GH #311 P0: the operator's narrow action on
// a job the autonomous-recovery decision tree quarantined when its run
// finalized-the-majority. Quarantine sets a single recovery-exhausted,
// downstream-clear job aside in the 'quarantined' state (never completed, never
// sealed) so the run can complete on its sibling deliverables; this verb is how
// the operator disposes of that one set-aside job.
//
// It resolves the job's open recovery_exhausted blocker + escalation_inbox row
// and marks the job 'canceled' (terminal, canceled-by-operator). It is
// idempotent: a job already canceled with its blocker resolved returns
// already_accepted rather than erroring, so a retried operator action is safe.
//
// It applies ONLY to a 'quarantined' job (the verb is the inverse of quarantine,
// not a general cancel). It never completes the job or seals an artifact — the
// quarantined work was genuinely unrecoverable; accepting it records that
// honestly as canceled-by-operator, not as a false success.
func HandleRecoveryAcceptQuarantined(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	jobID := stringParam(envelope, "job_id")
	if runID == "" || jobID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.accept_quarantined requires run_id and job_id", nil)
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first, scoped via the job's immutable
		// run_id (mirrors HandleRecoveryReseal / HandleRecoveryCompleteStalled).
		if err := lockRunForJob(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.NewError("not_found", fmt.Sprintf("could not find jobs row for %q", jobID), nil)
		}
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(job["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "job does not belong to the given run_id", nil)
		}

		jobState := fmt.Sprint(job["state"])
		// Idempotency: an already-accepted job is canceled with no open
		// recovery_exhausted blocker. Re-running returns the terminal result.
		if jobState == "canceled" {
			open, err := openRecoveryExhaustedBlockerForJob(ctx, tx, repositoryID, jobID)
			if err != nil {
				return nil, err
			}
			if open == nil {
				return map[string]any{
					"status":          "already_accepted",
					"run_id":          runID,
					"job_id":          jobID,
					"workflow_job_id": job["workflow_job_id"],
					"job_state":       "canceled",
					"next_actions":    []string{"inspect_run_state"},
				}, nil
			}
		}
		if jobState != "quarantined" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf(
				"recovery.accept_quarantined applies only to a quarantined job; state=%q", jobState), nil)
		}

		now := nowString()
		// Resolve the quarantine blocker + escalation_inbox row (so it leaves the
		// operator's pending inbox) BEFORE the cancel, then mark the job canceled.
		blocker, err := openRecoveryExhaustedBlockerForJob(ctx, tx, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		blockerID := ""
		if blocker != nil {
			blockerID = fmt.Sprint(blocker["blocker_id"])
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND blocker_id = $3 AND state = 'open'`,
				now, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.escalation_inbox
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND escalation_id = $3 AND state <> 'resolved'`,
				now, repositoryID, blockerID); err != nil {
				return nil, err
			}
		}

		// Mark the quarantined job canceled-by-operator (terminal). Reuse
		// cancelSingleJob so lease/message teardown + the job.canceled event match
		// every other cancel path; the quarantine already released the lease, so
		// this is mostly the state transition + event.
		canceled, err := cancelSingleJob(ctx, tx, repositoryID, job, "operator_accept_quarantined", now)
		if err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.quarantine_accepted", nil, jobID, nil, nil, nil, map[string]any{
			"workflow_job_id": job["workflow_job_id"],
			"blocker_id":      nullableString(blockerID),
			"disposition":     "canceled_by_operator",
		}); err != nil {
			return nil, err
		}
		// The run is already terminal (it finalized-the-majority when this job was
		// quarantined); maybeCompleteRun is still safe (it no-ops on a non-running
		// run) and keeps the path identical to the other cancel verbs.
		if err := maybeCompleteRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		runAfter, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":          "accepted",
			"run_id":          runID,
			"job_id":          jobID,
			"workflow_job_id": canceled["workflow_job_id"],
			"previous_state":  "quarantined",
			"job_state":       "canceled",
			"blocker_id":      nullableString(blockerID),
			"run_state":       runAfter["state"],
			"next_actions":    []string{"inspect_run_state", "export_run_evidence"},
		}, nil
	})
}
