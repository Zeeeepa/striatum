package mutations

import (
	"context"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"strings"
)

func HandleRecoveryCancelJob(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	jobID := stringParam(envelope, "job_id")
	reason := strings.TrimSpace(stringParam(envelope, "reason"))
	cascade := boolParam(envelope, "cascade")
	if runID == "" || jobID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.cancel_job requires run_id and job_id", nil)
	}
	if reason == "" {
		return nil, rpc.NewError("invalid_transition", "cancel reason must not be empty", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first.
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true); err != nil {
			return nil, err
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(job["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "job does not belong to the requested run", nil)
		}
		if !cancelableJobStates[fmt.Sprint(job["state"])] {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("job state %q is terminal and cannot be canceled", job["state"]), nil)
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		job, err = rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if !cancelableJobStates[fmt.Sprint(job["state"])] {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("job state %q is terminal and cannot be canceled", job["state"]), nil)
		}
		dependents, err := dependentsBlockedOnlyThrough(ctx, tx, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		if len(dependents) > 0 && !cascade {
			names := []string{}
			for _, row := range dependents {
				names = append(names, fmt.Sprint(row["workflow_job_id"]))
			}
			return nil, rpc.NewError("invalid_transition", "job has blocked dependents whose only path is through this job; rerun with --cascade or cancel them explicitly: "+strings.Join(names, ", "), nil)
		}
		now := nowString()
		canceled, err := cancelSingleJob(ctx, tx, repositoryID, job, reason, now)
		if err != nil {
			return nil, err
		}
		downstream := []map[string]any{}
		if cascade {
			queue := append([]map[string]any(nil), dependents...)
			visited := map[string]bool{jobID: true}
			for len(queue) > 0 {
				next := []map[string]any{}
				for _, item := range queue {
					depID := fmt.Sprint(item["job_id"])
					if visited[depID] {
						continue
					}
					visited[depID] = true
					fresh, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", depID, true)
					if err != nil {
						return nil, err
					}
					if !cancelableJobStates[fmt.Sprint(fresh["state"])] {
						continue
					}
					summary, err := cancelSingleJob(ctx, tx, repositoryID, fresh, "cascade:"+reason, now)
					if err != nil {
						return nil, err
					}
					downstream = append(downstream, summary)
					more, err := dependentsBlockedOnlyThrough(ctx, tx, repositoryID, depID)
					if err != nil {
						return nil, err
					}
					next = append(next, more...)
				}
				queue = next
			}
		}
		if err := maybeCompleteRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		runAfter, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":              "canceled",
			"run_id":              runID,
			"job_id":              jobID,
			"workflow_job_id":     canceled["workflow_job_id"],
			"previous_state":      canceled["previous_state"],
			"reason":              reason,
			"cascade":             cascade,
			"downstream_canceled": downstream,
			"run_state":           runAfter["state"],
			"next_actions":        []string{"inspect_run_state", "export_run_evidence"},
		}, nil
	})
}

func dependentsBlockedOnlyThrough(ctx context.Context, runner any, repositoryID, jobID string) ([]map[string]any, error) {
	candidates, err := queryRows(ctx, runner, `
		SELECT j.job_id, j.workflow_job_id, j.state
		  FROM striatumd.job_dependencies dep
		  JOIN striatumd.jobs j
		    ON j.repository_id = dep.repository_id
		   AND j.job_id = dep.job_id
		 WHERE dep.repository_id = $1
		   AND dep.depends_on_job_id = $2
		   AND j.state = 'blocked'
		 ORDER BY j.workflow_job_id`, repositoryID, jobID)
	if err != nil {
		return nil, err
	}
	qualifying := []map[string]any{}
	for _, candidate := range candidates {
		otherDeps, err := queryRows(ctx, runner, `
			SELECT up.state
			  FROM striatumd.job_dependencies dep
			  JOIN striatumd.jobs up
			    ON up.repository_id = dep.repository_id
			   AND up.job_id = dep.depends_on_job_id
			 WHERE dep.repository_id = $1
			   AND dep.job_id = $2
			   AND dep.depends_on_job_id != $3`, repositoryID, candidate["job_id"], jobID)
		if err != nil {
			return nil, err
		}
		onlyThrough := true
		for _, row := range otherDeps {
			state := fmt.Sprint(row["state"])
			if state != "completed" && state != "canceled" {
				onlyThrough = false
				break
			}
		}
		if onlyThrough {
			qualifying = append(qualifying, candidate)
		}
	}
	return qualifying, nil
}

func cancelSingleJob(ctx context.Context, runner any, repositoryID string, job map[string]any, reason, now string) (map[string]any, error) {
	jobID := fmt.Sprint(job["job_id"])
	leaseID := nullable(job["current_lease_id"])
	messageID := nullable(job["current_message_id"])
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if leaseID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'released', released_at = $1, release_reason = 'canceled'
			 WHERE repository_id = $2
			   AND lease_id = $3
			   AND state = 'active'`, now, repositoryID, leaseID); err != nil {
			return nil, err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET release_reason = COALESCE(release_reason, 'canceled')
		 WHERE repository_id = $1
		   AND resource_id = $2
		   AND state = 'expired'`, repositoryID, jobID); err != nil {
		return nil, err
	}
	if messageID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'canceled', current_lease_id = NULL, updated_at = $1
			 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
			return nil, err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'canceled', current_lease_id = NULL,
		       current_message_id = NULL, completed_at = $1
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.canceled", nil, jobID, messageID, nil, leaseID, map[string]any{
		"reason":          reason,
		"workflow_job_id": job["workflow_job_id"],
	}); err != nil {
		return nil, err
	}
	if err := markJobTerminal(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), jobID); err != nil {
		return nil, err
	}
	return map[string]any{
		"job_id":          jobID,
		"workflow_job_id": job["workflow_job_id"],
		"previous_state":  job["state"],
	}, nil
}
