package mutations

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// revisionCycle is the runtime view of a workflow `cycles[]` entry that the
// engine consults when a review job records a `needs_revision` verdict.
// See RFC 0083 §2 (bounded revision cycle) and RFC 0045 for the schema shape.
type revisionCycle struct {
	from          string
	to            string
	onVerdict     string
	maxIterations int
	allowSameLane bool
}

// workflowCyclesForJob loads the run's workflow snapshot and returns the
// declared cycles whose `from` matches the given review job's workflow_job_id.
func workflowCyclesForJob(ctx context.Context, runner any, repositoryID string, job map[string]any) ([]revisionCycle, error) {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return nil, err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return nil, err
	}
	fromID := fmt.Sprint(job["workflow_job_id"])
	cycles := []revisionCycle{}
	for _, item := range asList(asMap(snapshot["workflow_json"])["cycles"]) {
		def := asMap(item)
		if fmt.Sprint(def["from"]) != fromID {
			continue
		}
		cycles = append(cycles, revisionCycle{
			from:          fromID,
			to:            fmt.Sprint(def["to"]),
			onVerdict:     fmt.Sprint(def["on_verdict"]),
			maxIterations: intValue(def["max_iterations"]),
			allowSameLane: boolValue(def["allow_same_lane"]),
		})
	}
	return cycles, nil
}

// matchRevisionCycle returns the cycle whose `from` is the review job and whose
// `on_verdict` matches the recorded verdict. An absent/empty on_verdict defaults
// to "needs_revision" per RFC 0083 (the only verdict that drives a revision
// cycle today).
func matchRevisionCycle(cycles []revisionCycle, verdict string) (revisionCycle, bool) {
	for _, cycle := range cycles {
		on := cycle.onVerdict
		if on == "" {
			on = "needs_revision"
		}
		if on == verdict {
			return cycle, true
		}
	}
	return revisionCycle{}, false
}

// countRevisionRoutings returns how many times this cycle (review job ->
// target) has already been fired for the run, so max_iterations can be
// enforced. It counts emitted revision.cycle_routed events for the review job's
// workflow_job_id, which is monotonic and crash-safe (events are append-only).
func countRevisionRoutings(ctx context.Context, runner any, repositoryID string, job map[string]any, cycle revisionCycle) (int, error) {
	row, err := oneRow(ctx, runner, `
		SELECT count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND event_type = 'revision.cycle_routed'
		   AND payload_json->>'from_workflow_job_id' = $3
		   AND payload_json->>'to_workflow_job_id' = $4`,
		repositoryID, fmt.Sprint(job["run_id"]), cycle.from, cycle.to)
	if err != nil {
		return 0, err
	}
	return intValue(row["n"]), nil
}

// routeRevisionCycle handles a `needs_revision` verdict that matches a declared
// workflow cycle: it completes the review job, then re-opens the cycle target
// job for revision. Returns false (and no error) when the cycle's iteration
// budget is exhausted or the target job is absent, so the caller can fall back
// to a human checkpoint.
func routeRevisionCycle(
	ctx context.Context,
	runner any,
	repositoryID string,
	job map[string]any,
	sessionID, leaseID string,
	cycle revisionCycle,
	verdict string,
) (map[string]any, bool, error) {
	priorRoutings, err := countRevisionRoutings(ctx, runner, repositoryID, job, cycle)
	if err != nil {
		return nil, false, err
	}
	// max_iterations bounds how many revision re-works the cycle may drive.
	// Enforce it only when a positive budget is declared; once exhausted the
	// caller opens a human checkpoint instead of looping forever.
	if cycle.maxIterations > 0 && priorRoutings >= cycle.maxIterations {
		return nil, false, nil
	}

	target, err := latestJobForWorkflowID(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), cycle.to)
	if err != nil {
		return nil, false, err
	}
	if target == nil {
		// The declared cycle target does not exist in this run's job graph;
		// fall back to a human checkpoint rather than silently dropping it.
		return nil, false, nil
	}

	now := nowString()

	// Complete the review job (the verdict is recorded; the review is done).
	if err := completeReviewJob(ctx, runner, repositoryID, job, sessionID, leaseID, "needs_revision routed to revision cycle"); err != nil {
		return nil, false, err
	}

	iteration := priorRoutings + 1
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "revision.cycle_routed", sessionID, job["job_id"], nil, nil, leaseID, map[string]any{
		"from_workflow_job_id": cycle.from,
		"to_workflow_job_id":   cycle.to,
		"target_job_id":        target["job_id"],
		"verdict":              verdict,
		"iteration":            iteration,
		"max_iterations":       cycle.maxIterations,
		"allow_same_lane":      cycle.allowSameLane,
	}); err != nil {
		return nil, false, err
	}

	// Re-open the cycle target for revision. The target may be `completed`
	// (the usual case: an upstream synthesis/implement job that already
	// finished); re-opening it as part of an accepted revision cycle is a
	// legitimate transition (F3). Reset it to a clean state and re-enqueue.
	//
	// If the target is already non-terminal (queued/claimed/running) — e.g. an
	// operator manually retried it concurrently — the verdict and cycle-routed
	// event are still recorded, but the re-open is skipped rather than erroring
	// the whole verdict transaction (Bug B).
	targetState := fmt.Sprint(target["state"])
	if !isTerminalJobState(targetState) {
		if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
			return nil, false, err
		}
		return map[string]any{
			"status":               "revision_routed",
			"job_id":               job["job_id"],
			"verdict":              verdict,
			"cycle_target_job_id":  target["job_id"],
			"cycle_to":             cycle.to,
			"cycle_iteration":      iteration,
			"cycle_max_iterations": cycle.maxIterations,
			"target_message_id":    nil,
			"target_already_open":  true,
		}, true, nil
	}

	// Reset the cycle target's transitive downstream jobs (the reviews that
	// reviewed it, and anything further downstream) back to `blocked` so the
	// normal dependency gating re-enqueues them as the target — and then each
	// subsequent job — re-completes. Without this, the reviews stay `completed`
	// and `maybeEnqueueDownstream` (which only enqueues `blocked` jobs) skips
	// them, leaving the revised work UNREVIEWED (RFC 0083 review-after-revision
	// contract). This must run before re-opening the target itself, so the
	// recursive walk still sees the target's current downstream edges.
	if err := resetDownstreamForRevision(ctx, runner, repositoryID, fmt.Sprint(target["job_id"]), now); err != nil {
		return nil, false, err
	}

	if err := reopenJobForRevision(ctx, runner, repositoryID, target, now); err != nil {
		return nil, false, err
	}
	messageID, err := enqueueJob(ctx, runner, repositoryID, fmt.Sprint(target["job_id"]))
	if err != nil {
		return nil, false, err
	}

	if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
		return nil, false, err
	}

	return map[string]any{
		"status":               "revision_routed",
		"job_id":               job["job_id"],
		"verdict":              verdict,
		"cycle_target_job_id":  target["job_id"],
		"cycle_to":             cycle.to,
		"cycle_iteration":      iteration,
		"cycle_max_iterations": cycle.maxIterations,
		"target_message_id":    messageID,
	}, true, nil
}

// isDeclaredCycleTarget reports whether the given job's workflow_job_id is the
// `to` target of any declared revision cycle in the run's workflow. This lets
// the operator retry path (run.retry_job) re-open a completed cycle target for
// a manual revision (F3) without opening retries on arbitrary completed jobs.
func isDeclaredCycleTarget(ctx context.Context, runner any, repositoryID string, job map[string]any) (bool, error) {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return false, err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return false, err
	}
	workflowJobID := fmt.Sprint(job["workflow_job_id"])
	for _, item := range asList(asMap(snapshot["workflow_json"])["cycles"]) {
		def := asMap(item)
		if fmt.Sprint(def["to"]) != workflowJobID {
			continue
		}
		// Only `needs_revision` cycles declare a genuine revision target. A
		// cycle keyed on any other verdict does not unlock the F3 retry path,
		// so the completed job remains non-retriable (Bug C). An absent or
		// empty on_verdict defaults to needs_revision per RFC 0083, matching
		// matchRevisionCycle.
		on := fmt.Sprint(def["on_verdict"])
		if def["on_verdict"] == nil || on == "" {
			on = "needs_revision"
		}
		if on == "needs_revision" {
			return true, nil
		}
	}
	return false, nil
}

// latestJobForWorkflowID returns the highest-attempt job row for a given
// workflow_job_id in a run, or nil when no such job exists.
func latestJobForWorkflowID(ctx context.Context, runner any, repositoryID, runID, workflowJobID string) (map[string]any, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT * FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3
		 ORDER BY attempt DESC, created_at DESC
		 LIMIT 1
		 FOR UPDATE`, repositoryID, runID, workflowJobID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// isTerminalJobState reports whether a job state is one a revision cycle may
// re-open: a finished state (completed/failed/canceled/skipped/waiting_human)
// or `blocked` (not yet enqueued). Live states (queued/claimed/running) are
// non-terminal and must not be clobbered by an in-flight revision routing.
func isTerminalJobState(state string) bool {
	switch state {
	case "completed", "failed", "canceled", "skipped", "waiting_human", "blocked":
		return true
	default:
		return false
	}
}

// reopenJobForRevision resets a (typically completed) cycle-target job back to
// a blocked state so enqueueJob can re-issue its work. It releases any active
// lease, cancels in-flight work messages and open blockers, clears terminal
// timestamps, and bumps the attempt counter.
func reopenJobForRevision(ctx context.Context, runner any, repositoryID string, job map[string]any, now string) error {
	state := fmt.Sprint(job["state"])
	if !isTerminalJobState(state) {
		return rpc.NewError("invalid_transition", fmt.Sprintf("cycle target job state %q cannot be re-opened for revision", state), nil)
	}
	return resetJobToBlocked(ctx, runner, repositoryID, fmt.Sprint(job["job_id"]), now)
}

// resetJobToBlocked is the shared reset core used both to re-open the cycle
// target and to re-block its transitive downstream jobs. It releases any active
// lease, cancels in-flight work messages and open blockers, clears the
// per-attempt verdicts of verdict-capable jobs (so a re-run review's fresh
// verdict supersedes the prior round and the (job, session) verdict uniqueness
// constraint is freed), clears terminal timestamps, and bumps the attempt.
func resetJobToBlocked(ctx context.Context, runner any, repositoryID, jobID, now string) error {
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	// Release any active lease still pinned to the job.
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'revision_cycle'
		 WHERE repository_id = $2 AND resource_id = $3 AND state = 'active'`,
		now, repositoryID, jobID); err != nil {
		return err
	}
	// Cancel any in-flight work messages so enqueueJob can publish a fresh one
	// (the uq_active_work_message_per_job partial unique index would otherwise
	// reject a new pending message).
	if err := exec.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'canceled', updated_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3
		   AND state IN ('pending','claimed','acked')`,
		now, repositoryID, jobID); err != nil {
		return err
	}
	// Cancel any open blockers on the job.
	if err := exec.Exec(ctx, `
		UPDATE striatumd.blockers
		   SET state = 'canceled', resolved_at = $1
		 WHERE repository_id = $2 AND job_id = $3 AND resolved_at IS NULL`,
		now, repositoryID, jobID); err != nil {
		return err
	}
	// Clear the job's verdicts so the downstream gate re-evaluates against the
	// fresh review round. `latestVerdict` orders by created_at (truncated to the
	// second), so a prior-round verdict recorded in the same wall-clock second
	// as the new one could otherwise win the tiebreak (verdict_id is random).
	// Deleting the stale rows removes that ambiguity and frees the
	// (repository_id, job_id, session_id) uniqueness constraint so a re-claiming
	// session can record again. Verdict rows are not referenced by any FK
	// (events carry verdict_id in payload only), so deletion is safe.
	if err := exec.Exec(ctx, `
		DELETE FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`,
		repositoryID, jobID); err != nil {
		return err
	}
	// Reset the job to a clean blocked state and bump the attempt. enqueueJob
	// (or the normal downstream gating) transitions blocked->queued and attaches
	// a fresh message.
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'blocked',
		       started_at = NULL,
		       completed_at = NULL,
		       current_lease_id = NULL,
		       current_message_id = NULL,
		       attempt = attempt + 1
		 WHERE repository_id = $1 AND job_id = $2`,
		repositoryID, jobID); err != nil {
		return err
	}
	return nil
}

// resetDownstreamForRevision walks the job_dependencies graph from the cycle
// target and re-blocks every transitive downstream job that is currently in a
// terminal state (completed/failed/canceled/skipped/waiting_human). Each such
// job — the reviews that reviewed the target, plus anything further downstream
// such as implement/build-review — is returned to `blocked` so the normal
// dependency gating re-enqueues it as the target (and then each predecessor)
// re-completes. Jobs already non-terminal (queued/claimed/running) are left
// alone so an in-flight worker is not clobbered.
func resetDownstreamForRevision(ctx context.Context, runner any, repositoryID, targetJobID, now string) error {
	rows, err := queryRows(ctx, runner, `
		WITH RECURSIVE downstream(job_id) AS (
		    SELECT dep.job_id
		      FROM striatumd.job_dependencies dep
		     WHERE dep.repository_id = $1 AND dep.depends_on_job_id = $2
		  UNION
		    SELECT dep.job_id
		      FROM striatumd.job_dependencies dep
		      JOIN downstream d
		        ON dep.depends_on_job_id = d.job_id
		     WHERE dep.repository_id = $1
		)
		SELECT j.job_id, j.state
		  FROM downstream d
		  JOIN striatumd.jobs j
		    ON j.repository_id = $1 AND j.job_id = d.job_id
		 WHERE j.state IN ('completed','failed','canceled','skipped','waiting_human')
		 ORDER BY j.created_at, j.job_id
		 FOR UPDATE OF j`, repositoryID, targetJobID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := resetJobToBlocked(ctx, runner, repositoryID, fmt.Sprint(row["job_id"]), now); err != nil {
			return err
		}
	}
	return nil
}
