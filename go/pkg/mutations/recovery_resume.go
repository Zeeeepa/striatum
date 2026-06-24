package mutations

import (
	"context"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"strings"
)

func HandleRecoveryResume(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	blockerID := stringParam(envelope, "blocker_id")
	if blockerID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.resume requires blocker_id", nil)
	}
	complete := boolParam(envelope, "complete")
	sessionID := stringParam(envelope, "session_id")
	summary := stringParam(envelope, "summary")
	force := boolParam(envelope, "force")
	extendSeconds := intParam(envelope, "extend_seconds", 900)
	if extendSeconds <= 0 {
		return nil, rpc.NewError("invalid_transition", "--extend-seconds must be positive", nil)
	}
	if complete && sessionID == "" {
		return nil, rpc.NewError("invalid_transition", "--complete requires --session-id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first.
		if err := lockRunForBlocker(ctx, tx, repositoryID, blockerID); err != nil {
			return nil, err
		}
		blocker, err := rowByID(ctx, tx, repositoryID, "blockers", "blocker_id", blockerID, true)
		if err != nil {
			return nil, err
		}
		blockerKind := fmt.Sprint(blocker["blocker_kind"])
		if !processAdapterBlockerKinds[blockerKind] {
			if writeScopeResumeBlockerKinds[blockerKind] {
				return resumeWriteScopeBlocker(ctx, tx, writeScopeResumeRequest{
					RepositoryID:      repositoryID,
					Blocker:           blocker,
					BlockerKind:       blockerKind,
					SessionID:         sessionID,
					Summary:           summary,
					CompleteRequested: complete,
				})
			}
			return nil, rpc.NewError("invalid_transition", "recovery resume supports only process-adapter blockers", nil)
		}
		if nullable(blocker["job_id"]) == nil {
			return nil, rpc.NewError("invalid_transition", "process-adapter blocker is not job-bound", nil)
		}
		jobID := fmt.Sprint(blocker["job_id"])
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		runID := fmt.Sprint(job["run_id"])
		if fmt.Sprint(blocker["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "blocker does not belong to the job run", nil)
		}
		if fmt.Sprint(blocker["state"]) != "open" {
			return map[string]any{
				"status":           "already_resolved",
				"run_id":           runID,
				"job_id":           jobID,
				"workflow_job_id":  job["workflow_job_id"],
				"blocker_id":       blockerID,
				"blocker_kind":     blockerKind,
				"completed_inline": false,
				"next_actions":     []string{"inspect_job_state"},
			}, nil
		}
		jobState := fmt.Sprint(job["state"])
		if terminalJobStates[jobState] && force {
			now := nowString()
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND blocker_id = $3`, now, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.blocker_dismissed_terminal", nullable(sessionID), jobID, nil, nil, nil, map[string]any{
				"blocker_id":   blockerID,
				"blocker_kind": blockerKind,
				"job_state":    jobState,
				"reason":       "process-adapter blocker dismissed against terminal job (GH #7 legacy state)",
			}); err != nil {
				return nil, err
			}
			return map[string]any{
				"status":           "resolved_terminal_no_op",
				"run_id":           runID,
				"job_id":           jobID,
				"workflow_job_id":  job["workflow_job_id"],
				"blocker_id":       blockerID,
				"blocker_kind":     blockerKind,
				"completed_inline": false,
				"job_state":        jobState,
				"next_actions":     []string{"inspect_job_state"},
			}, nil
		}
		if jobState != "blocked" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("job must be blocked before recovery resume (state=%q); pass --force for GH #7 legacy process-adapter blockers on terminal jobs", jobState), nil)
		}
		if processExitBlockerKinds[blockerKind] && !force {
			return nil, rpc.NewError("invalid_transition", blockerKind+" requires --force after operator inspection", nil)
		}
		missingPaths, reviewVerdictMissing, err := validateProcessOutputs(ctx, tx, repositoryID, job)
		if err != nil {
			return nil, err
		}
		if len(missingPaths) > 0 {
			return nil, rpc.NewError("invalid_transition", "process-adapter blocker still has missing required artifacts: "+strings.Join(missingPaths, ", "), nil)
		}
		if complete && reviewVerdictMissing {
			return nil, rpc.NewError("invalid_transition", "process-adapter blocker cannot complete while review verdict is missing", nil)
		}
		if reviewVerdictMissing && fmt.Sprint(job["job_type"]) != "review" {
			return nil, rpc.NewError("invalid_transition", "process-adapter blocker still has a missing review verdict", nil)
		}
		leaseID := nullable(job["current_lease_id"])
		if leaseID == nil {
			return nil, rpc.NewError("invalid_transition", "process-adapter blocker job has no current lease to resume", nil)
		}
		lease, err := rowByID(ctx, tx, repositoryID, "leases", "lease_id", fmt.Sprint(leaseID), true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(lease["state"]) != "active" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("process-adapter blocker lease is not active (state=%q)", lease["state"]), nil)
		}
		leaseOwner := fmt.Sprint(lease["owner_session_id"])
		if sessionID != "" && sessionID != leaseOwner {
			return nil, rpc.NewError("invalid_transition", "session does not own the process-adapter lease", nil)
		}
		actorSessionID := sessionID
		if actorSessionID == "" {
			actorSessionID = fmt.Sprint(nullable(blocker["session_id"]))
		}
		if actorSessionID == "" || actorSessionID == "<nil>" {
			actorSessionID = leaseOwner
		}
		now := nowString()
		expiresAt := expiresAfter(extendSeconds)
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET last_heartbeat_at = $1, expires_at = $2
			 WHERE repository_id = $3 AND lease_id = $4`, now, expiresAt, repositoryID, leaseID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.blockers
			   SET state = 'resolved', resolved_at = $1
			 WHERE repository_id = $2 AND blocker_id = $3`, now, repositoryID, blockerID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'running'
			 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.process_blocker_resolved", actorSessionID, jobID, nil, nil, leaseID, map[string]any{
			"blocker_id":             blockerID,
			"blocker_kind":           blockerKind,
			"verb":                   "recovery resume",
			"force":                  force,
			"completed_inline":       complete,
			"missing_artifact_paths": missingPaths,
			"review_verdict_missing": reviewVerdictMissing,
			"lease_extended_until":   expiresAt,
			"original_envelope":      asMap(blocker["payload_json"]),
		}); err != nil {
			return nil, err
		}
		result := map[string]any{
			"status":                 "resumed",
			"run_id":                 runID,
			"job_id":                 jobID,
			"workflow_job_id":        job["workflow_job_id"],
			"blocker_id":             blockerID,
			"blocker_kind":           blockerKind,
			"lease_id":               leaseID,
			"lease_extended_until":   expiresAt,
			"force":                  force,
			"completed_inline":       false,
			"review_verdict_missing": reviewVerdictMissing,
			"next_actions":           []string{"complete_job", "monitor_run_progress"},
		}
		if reviewVerdictMissing {
			result["next_actions"] = []string{"record_review_verdict"}
		}
		if !complete {
			return result, nil
		}
		completion, err := completeRecoveredJob(ctx, tx, repositoryID, jobID, actorSessionID, fmt.Sprint(leaseID), summary)
		if err != nil {
			return nil, err
		}
		result["status"] = "resumed_completed"
		result["completed_inline"] = true
		result["completion"] = completion
		result["next_actions"] = []string{"monitor_run_progress", "export_run_evidence"}
		return result, nil
	})
}

type writeScopeResumeRequest struct {
	RepositoryID      string
	Blocker           map[string]any
	BlockerKind       string
	SessionID         string
	Summary           string
	CompleteRequested bool
}

type writeScopeResumeTarget struct {
	JobID string
	RunID string
	Job   map[string]any
}

func resumeWriteScopeBlocker(ctx context.Context, tx db.TxRunner, request writeScopeResumeRequest) (map[string]any, error) {
	if nullable(request.Blocker["job_id"]) == nil {
		return nil, rpc.NewError("invalid_transition", "write-scope blocker is not job-bound", nil)
	}
	if fmt.Sprint(request.Blocker["state"]) != "open" {
		return alreadyResolvedWriteScopeResumeResult(request), nil
	}
	target, err := validateWriteScopeResumeTarget(ctx, tx, request)
	if err != nil {
		return nil, err
	}
	if err := enforceWriteScopeClean(ctx, tx, request.RepositoryID, target.Job); err != nil {
		return nil, err
	}
	if err := resolveWriteScopeBlocker(ctx, tx, request, target); err != nil {
		return nil, err
	}
	requeue, err := requeueJobSameAttempt(ctx, tx, request.RepositoryID, target.Job, requeueSameAttemptOptions{
		operatorOverride: true,
		justification:    request.Summary,
		author:           request.SessionID,
	})
	if err != nil {
		return nil, err
	}
	return writeScopeResumeResult(request, target, requeue), nil
}

func alreadyResolvedWriteScopeResumeResult(request writeScopeResumeRequest) map[string]any {
	return map[string]any{
		"status":           "already_resolved",
		"run_id":           request.Blocker["run_id"],
		"job_id":           request.Blocker["job_id"],
		"blocker_id":       request.Blocker["blocker_id"],
		"blocker_kind":     request.BlockerKind,
		"completed_inline": false,
		"next_actions":     []string{"inspect_job_state"},
	}
}

func validateWriteScopeResumeTarget(ctx context.Context, tx db.TxRunner, request writeScopeResumeRequest) (writeScopeResumeTarget, error) {
	jobID := fmt.Sprint(request.Blocker["job_id"])
	job, err := rowByID(ctx, tx, request.RepositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return writeScopeResumeTarget{}, err
	}
	runID := fmt.Sprint(job["run_id"])
	if fmt.Sprint(request.Blocker["run_id"]) != runID {
		return writeScopeResumeTarget{}, rpc.NewError("invalid_transition", "blocker does not belong to the job run", nil)
	}
	if fmt.Sprint(job["state"]) != "blocked" {
		return writeScopeResumeTarget{}, rpc.NewError("invalid_transition", fmt.Sprintf("job must be blocked before write-scope recovery resume (state=%q)", job["state"]), nil)
	}
	return writeScopeResumeTarget{JobID: jobID, RunID: runID, Job: job}, nil
}

func resolveWriteScopeBlocker(ctx context.Context, tx db.TxRunner, request writeScopeResumeRequest, target writeScopeResumeTarget) error {
	now := nowString()
	if err := tx.Exec(ctx, `
		UPDATE striatumd.blockers
		   SET state = 'resolved', resolved_at = $1
		 WHERE repository_id = $2 AND blocker_id = $3`, now, request.RepositoryID, request.Blocker["blocker_id"]); err != nil {
		return err
	}
	_, err := appendEvent(ctx, tx, request.RepositoryID, target.RunID, "recovery.write_scope_blocker_resolved", nullable(request.SessionID), target.JobID, nil, nil, nil, map[string]any{
		"blocker_id":         request.Blocker["blocker_id"],
		"blocker_kind":       request.BlockerKind,
		"verb":               "recovery resume",
		"complete_requested": request.CompleteRequested,
		"summary":            request.Summary,
		"original_envelope":  asMap(request.Blocker["payload_json"]),
	})
	return err
}

func writeScopeResumeResult(request writeScopeResumeRequest, target writeScopeResumeTarget, requeue requeueSameAttemptResult) map[string]any {
	nextActions := []string{"claim_available_work", "complete_job"}
	if request.CompleteRequested {
		nextActions = []string{"claim_available_work", "complete_job_after_claim"}
	}
	return map[string]any{
		"status":              "resumed_requeued",
		"run_id":              target.RunID,
		"job_id":              target.JobID,
		"workflow_job_id":     target.Job["workflow_job_id"],
		"blocker_id":          request.Blocker["blocker_id"],
		"blocker_kind":        request.BlockerKind,
		"message_id":          requeue.messageID,
		"already_reclaimable": requeue.alreadyReclaimable,
		"completed_inline":    false,
		"complete_requested":  request.CompleteRequested,
		"note":                "write-scope blockers release their lease when blocked; recovery.resume resolves the clean blocker and requeues the same attempt for a fresh claim before completion",
		"next_actions":        nextActions,
	}
}

func completeRecoveredJob(ctx context.Context, runner any, repositoryID, jobID, sessionID, leaseID, summary string) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return nil, err
	}
	if fmt.Sprint(job["state"]) != "running" {
		return nil, rpc.NewError("invalid_transition", "job must be running before completion", nil)
	}
	applyFrozenAttemptWriteScope(ctx, runner, repositoryID, job, leaseID)
	if err := enforceWriteScopeClean(ctx, runner, repositoryID, job); err != nil {
		return nil, err
	}
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	if err := ensurePerJobPublishedArtifactsDurable(ctx, runner, repositoryID, job, "recovery.resume --complete"); err != nil {
		return nil, err
	}
	now := nowString()
	messageID := nullable(job["current_message_id"])
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
		return nil, err
	}
	if messageID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $2,
			       current_lease_id = NULL
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return nil, err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'completed'
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.completed", sessionID, jobID, messageID, nil, leaseID, map[string]any{"summary": summary}); err != nil {
		return nil, err
	}
	// #304: resolve the completing job's open autonomous blockers (e.g. a
	// blocked-severity write_scope conflict raised on a prior attempt) so a
	// recovery.resume --complete does not leave them dangling open.
	if err := resolveAutonomousBlockersOnCompletion(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), jobID, sessionID, now); err != nil {
		return nil, err
	}
	if err := markJobTerminal(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), jobID); err != nil {
		return nil, err
	}
	if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "job_id": jobID}, nil
}
