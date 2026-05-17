package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
)

func HandleRunPrepare(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	workflowPath := stringParam(envelope, "workflow")
	if workflowPath == "" {
		return nil, rpc.NewError("schema_invalid", "run.prepare requires workflow", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return runPrepare(ctx, tx, repositoryID, workflowPath)
	})
}

func HandleRunStart(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.start requires run_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		workflow, err := workflowForRun(ctx, tx, repositoryID, run)
		if err != nil {
			return nil, err
		}
		if workflow["provenance_mode"] == "sealed_patch" {
			return nil, rpc.NewError("workflow_error", "provenance_mode sealed_patch is unsupported: no containment mechanism shipped; sealed runs refuse to start rather than silently downgrading to advisory", nil)
		}
		state := fmt.Sprint(run["state"])
		if state == "needs_branch_confirmation" {
			return nil, rpc.NewError("workflow_error", "branch confirmation is required before run start", nil)
		}
		if state != "ready" && state != "running" {
			return nil, rpc.NewError("invalid_transition", "run cannot be started from its current state", nil)
		}
		if state == "ready" {
			now := nowString()
			if err := tx.Exec(ctx, `
				UPDATE striatumd.runs
				   SET state = 'running', started_at = $1
				 WHERE repository_id = $2 AND run_id = $3`, now, repositoryID, runID); err != nil {
				return nil, err
			}
			roots, err := queryRows(ctx, tx, `
				SELECT j.job_id
				  FROM striatumd.jobs j
				 WHERE j.repository_id = $1
				   AND j.run_id = $2
				   AND NOT EXISTS (
				     SELECT 1 FROM striatumd.job_dependencies dep
				      WHERE dep.repository_id = j.repository_id
				        AND dep.job_id = j.job_id
				   )
				 ORDER BY j.created_at`, repositoryID, runID)
			if err != nil {
				return nil, err
			}
			for _, root := range roots {
				if _, err := enqueueJob(ctx, tx, repositoryID, fmt.Sprint(root["job_id"])); err != nil {
					return nil, err
				}
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.started", nil, nil, nil, nil, nil, nil); err != nil {
				return nil, err
			}
		}
		return map[string]any{"run_id": runID, "state": "running"}, nil
	})
}

func HandleRunPause(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.pause requires run_id", nil)
	}
	reason := stringParam(envelope, "reason")
	if reason == "" {
		reason = "operator_paused"
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		state := fmt.Sprint(run["state"])
		if state == "completed" || state == "failed" || state == "canceled" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("run is in terminal state %q and cannot be paused", state), nil)
		}
		if nullable(run["paused_at"]) != nil {
			return map[string]any{"run_id": runID, "state": state, "paused_at": run["paused_at"], "status": "already_paused"}, nil
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.runs
			   SET paused_at = $1, paused_reason = $2
			 WHERE repository_id = $3 AND run_id = $4 AND paused_at IS NULL`, now, reason, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.paused", nil, nil, nil, nil, nil, map[string]any{"reason": reason}); err != nil {
			return nil, err
		}
		return map[string]any{"run_id": runID, "state": state, "paused_at": now, "status": "paused"}, nil
	})
}

func HandleRunResume(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.resume requires run_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		state := fmt.Sprint(run["state"])
		if state == "completed" || state == "failed" || state == "canceled" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("run is in terminal state %q; use retry_job to revive", state), nil)
		}
		if nullable(run["paused_at"]) == nil {
			return map[string]any{"run_id": runID, "state": state, "paused_at": nil, "status": "not_paused"}, nil
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.runs
			   SET paused_at = NULL, paused_reason = NULL
			 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.resumed", nil, nil, nil, nil, nil, nil); err != nil {
			return nil, err
		}
		return map[string]any{"run_id": runID, "state": state, "paused_at": nil, "status": "resumed"}, nil
	})
}

func HandleRunCancel(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.cancel requires run_id", nil)
	}
	reason := stringParam(envelope, "reason")
	if reason == "" {
		reason = "operator_canceled"
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		state := fmt.Sprint(run["state"])
		if state == "canceled" {
			return map[string]any{"run_id": runID, "state": "canceled", "status": "already_canceled"}, nil
		}
		if state == "completed" || state == "failed" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("run is in terminal state %q and cannot be canceled", state), nil)
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'canceled', completed_at = $1
			 WHERE repository_id = $2
			   AND run_id = $3
			   AND state IN ('queued','running','blocked','ready','claimed')`, now, repositoryID, runID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'released', released_at = $1, release_reason = 'run_canceled'
			 WHERE repository_id = $2
			   AND run_id = $3
			   AND state = 'active'`, now, repositoryID, runID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.runs
			   SET state = 'canceled', completed_at = $1, stop_reason = $2
			 WHERE repository_id = $3 AND run_id = $4`, now, reason, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.canceled", nil, nil, nil, nil, nil, map[string]any{"reason": reason}); err != nil {
			return nil, err
		}
		if err := closeRemainingSessions(ctx, tx, repositoryID, runID, "run_canceled", "run_canceled"); err != nil {
			return nil, err
		}
		return map[string]any{"run_id": runID, "state": "canceled", "status": "canceled"}, nil
	})
}

func HandleRunRetryJob(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	jobID := stringParam(envelope, "job_id")
	if runID == "" || jobID == "" {
		return nil, rpc.NewError("schema_invalid", "run.retry_job requires run_id and job_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(run["state"]) == "completed" {
			return nil, rpc.NewError("invalid_transition", "run is completed; retry would revive a finished run", nil)
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(job["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "job does not belong to the requested run", nil)
		}
		previousState := fmt.Sprint(job["state"])
		if previousState != "failed" && previousState != "canceled" && previousState != "blocked" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("job state %q is not retriable (must be failed, canceled, or blocked)", previousState), nil)
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'queued',
			       started_at = NULL,
			       completed_at = NULL,
			       current_lease_id = NULL,
			       current_message_id = NULL,
			       attempt = attempt + 1
			 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.blockers
			   SET state = 'canceled', resolved_at = $1
			 WHERE repository_id = $2 AND job_id = $3 AND resolved_at IS NULL`, now, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'canceled', updated_at = $1
			 WHERE repository_id = $2
			   AND job_id = $3
			   AND state IN ('pending','claimed','acked')`, now, repositoryID, jobID); err != nil {
			return nil, err
		}
		if _, err := enqueueJob(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		runRevived := false
		if fmt.Sprint(run["state"]) == "failed" || fmt.Sprint(run["state"]) == "canceled" {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.runs
				   SET state = 'running', completed_at = NULL, stop_reason = NULL
				 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.revived", nil, nil, nil, nil, nil, map[string]any{
				"trigger_job_id":     jobID,
				"previous_run_state": run["state"],
			}); err != nil {
				return nil, err
			}
			runRevived = true
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "job.retried", nil, jobID, nil, nil, nil, map[string]any{
			"previous_state": previousState,
			"attempt":        intValue(job["attempt"]) + 1,
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"run_id":         runID,
			"job_id":         jobID,
			"previous_state": previousState,
			"new_state":      "queued",
			"run_revived":    runRevived,
		}, nil
	})
}

func HandleBranchConfirm(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	branch := stringParam(envelope, "branch")
	if runID == "" || branch == "" {
		return nil, rpc.NewError("schema_invalid", "branch.confirm requires run_id and branch", nil)
	}
	create := boolParam(envelope, "create")
	useCurrent := boolParam(envelope, "use_current")
	strict := boolParam(envelope, "strict")
	if create && useCurrent {
		return nil, rpc.NewError("workflow_error", "--create and --use-current are mutually exclusive", nil)
	}
	if strict && (create || useCurrent) {
		return nil, rpc.NewError("workflow_error", "--strict is incompatible with --create and --use-current", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		repoRoot := fmt.Sprint(run["repo_root"])
		requestedBranch := branch
		targetBranch := branch
		created := false
		mode := "records_only"
		if useCurrent {
			mode = "use_current"
			current := currentGitBranch(repoRoot)
			if current == "" {
				return nil, rpc.NewError("workflow_error", "--use-current requires a detectable current git branch in the target repo", nil)
			}
			if branch != current {
				return nil, rpc.NewError("workflow_error", fmt.Sprintf("--use-current was given but --branch=%q does not match current git branch %q", branch, current), nil)
			}
			targetBranch = current
		} else if create {
			mode = "create"
			var err error
			targetBranch, created, err = gitCreateOrCheckoutBranch(repoRoot, branch)
			if err != nil {
				return nil, err
			}
		} else if strict {
			mode = "strict"
			current := currentGitBranch(repoRoot)
			if current != branch {
				return nil, rpc.NewError("workflow_error", fmt.Sprintf("--strict requires current git branch to match --branch=%q; current branch is %q", branch, current), nil)
			}
		}
		state := fmt.Sprint(run["state"])
		if state != "needs_branch_confirmation" && state != "ready" {
			return nil, rpc.NewError("invalid_transition", "run is not waiting for branch confirmation", nil)
		}
		currentBranch := currentGitBranch(repoRoot)
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.runs
			   SET branch_name = $1, branch_confirmed_at = $2,
			       branch_confirmed_by = 'human', state = 'ready'
			 WHERE repository_id = $3 AND run_id = $4`, targetBranch, now, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "run.branch_confirmed", nil, nil, nil, nil, nil, map[string]any{
			"branch":  targetBranch,
			"mode":    mode,
			"created": created,
		}); err != nil {
			return nil, err
		}
		var warning any
		if currentBranch != "" && currentBranch != targetBranch {
			warning = "current git branch differs from recorded branch confirmation"
		}
		return map[string]any{
			"run_id":             runID,
			"state":              "ready",
			"branch":             targetBranch,
			"requested_branch":   requestedBranch,
			"current_git_branch": nullable(currentBranch),
			"records_only":       true,
			"warning":            warning,
			"created":            created,
			"mode":               mode,
		}, nil
	})
}

func currentGitBranch(repoRoot string) string {
	result := exec.Command("git", "branch", "--show-current")
	result.Dir = repoRoot
	out, err := result.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitCreateOrCheckoutBranch(repoRoot string, branch string) (string, bool, error) {
	create := exec.Command("git", "checkout", "-b", branch)
	create.Dir = repoRoot
	if out, err := create.CombinedOutput(); err == nil {
		_ = out
		return branch, true, nil
	}
	checkout := exec.Command("git", "checkout", branch)
	checkout.Dir = repoRoot
	out, err := checkout.CombinedOutput()
	if err == nil {
		return branch, false, nil
	}
	stderr := strings.TrimSpace(string(out))
	if len(stderr) > 200 {
		stderr = stderr[:200] + "..."
	}
	if stderr != "" {
		return "", false, rpc.NewError("workflow_error", fmt.Sprintf("git checkout failed for branch %q: %s", branch, stderr), nil)
	}
	return "", false, rpc.NewError("workflow_error", fmt.Sprintf("git checkout failed for branch %q", branch), nil)
}

func runPrepare(ctx context.Context, runner any, repositoryID string, workflowPath string) (map[string]any, error) {
	repo, err := rowByID(ctx, runner, repositoryID, "repositories", "repository_id", repositoryID, false)
	if err != nil {
		return nil, err
	}
	repoRoot := fmt.Sprint(repo["repo_root"])
	workflow, sourcePath, err := workflowauthoring.LoadFile(repoRoot, workflowPath)
	if err != nil {
		return nil, rpc.NewError("workflow_error", err.Error(), nil)
	}
	workflowID, _ := workflow["workflow_id"].(string)
	if workflowID == "" {
		return nil, rpc.NewError("workflow_error", "workflow config must declare workflow_id", nil)
	}
	phaseIndex, err := validateWorkflowForPrepare(workflow)
	if err != nil {
		return nil, err
	}
	now := nowString()
	normalized, err := json.Marshal(workflow)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(normalized)
	snapshotID, err := newID("wfs")
	if err != nil {
		return nil, err
	}
	runID, err := newID("run")
	if err != nil {
		return nil, err
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, workflow_version,
		  source_path, content_sha256, workflow_json, loaded_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		repositoryID,
		snapshotID,
		workflowID,
		nullable(workflow["workflow_version"]),
		sourcePath,
		hex.EncodeToString(sum[:]),
		workflow,
		now,
	); err != nil {
		return nil, err
	}
	branch := asMap(workflow["branch"])
	suggestedBranch, _ := branch["suggested_name"].(string)
	state := "needs_branch_confirmation"
	var confirmedAt any
	var confirmedBy any
	if branch["mode"] == "auto" && suggestedBranch != "" {
		state = "ready"
		confirmedAt = now
		confirmedBy = "daemon"
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
		  created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		repositoryID,
		runID,
		snapshotID,
		repoRoot,
		state,
		nullable(suggestedBranch),
		nil,
		confirmedAt,
		confirmedBy,
		now,
	); err != nil {
		return nil, err
	}
	jobIDs := map[string]string{}
	for _, item := range asList(workflow["jobs"]) {
		job := asMap(item)
		workflowJobID, _ := job["id"].(string)
		if workflowJobID == "" {
			return nil, rpc.NewError("workflow_error", "workflow job is missing id", nil)
		}
		jobID := fmt.Sprintf("job_%s_%s", runID, workflowJobID)
		jobIDs[workflowJobID] = jobID
		jobType, _ := job["type"].(string)
		if jobType == "" {
			jobType = "generic"
		}
		if jobType == "phase_synthesis" {
			jobType = "review"
		}
		capabilityReqs := map[string]any{
			"objective":   job["objective"],
			"task_prompt": asMap(job["task_prompt"]),
			"inputs":      asList(job["inputs"]),
		}
		laneID, _ := job["lane_id"].(string)
		laneSelector := map[string]any{}
		if laneID != "" {
			laneSelector["lane_id"] = laneID
		}
		if err := exec.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, title, job_type,
			  role_id, lane_selector_json, capability_requirements_json, state,
			  max_attempts, fresh_session_required, write_scope_json,
			  expected_artifacts_json, idempotency_key, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'blocked',$10,$11,$12,$13,$14,$15)`,
			repositoryID,
			jobID,
			runID,
			workflowJobID,
			valueOr(job["title"], workflowJobID),
			jobType,
			job["role_id"],
			laneSelector,
			capabilityReqs,
			valueOr(job["max_attempts"], 1),
			effectiveFreshSessionRequired(job),
			valueOr(job["write_scope"], map[string]any{}),
			valueOr(job["expected_artifacts"], []any{}),
			fmt.Sprintf("%s:%s:1", runID, workflowJobID),
			now,
		); err != nil {
			return nil, err
		}
	}
	for _, edge := range edgeDependencyPairs(workflow, phaseIndex, true) {
		fromID := edge.fromID
		toID := edge.toID
		fromJobID := jobIDs[fromID]
		toJobID := jobIDs[toID]
		if fromJobID == "" || toJobID == "" {
			return nil, rpc.NewError("workflow_error", "workflow edge references an unknown job", nil)
		}
		gate := map[string]any{"on": "completed", "from": fromID, "to": toID}
		if workflowJobType(workflow, fromID) == "review" || workflowJobType(workflow, fromID) == "phase_synthesis" {
			gate["requires_verdict"] = []string{"accept", "accept_with_findings"}
		}
		if err := exec.Exec(ctx, `
			INSERT INTO striatumd.job_dependencies (
			  repository_id, job_id, depends_on_job_id, gate_json
			)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (repository_id, job_id, depends_on_job_id) DO NOTHING`,
			repositoryID, toJobID, fromJobID, gate); err != nil {
			return nil, err
		}
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, "run.created", nil, nil, nil, nil, nil, map[string]any{
		"workflow_id":          workflowID,
		"workflow_snapshot_id": snapshotID,
	}); err != nil {
		return nil, err
	}
	if state == "ready" {
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "run.branch_confirmed", nil, nil, nil, nil, nil, map[string]any{
			"branch":  suggestedBranch,
			"mode":    "auto",
			"created": false,
		}); err != nil {
			return nil, err
		}
	}
	branchMode, _ := branch["mode"].(string)
	if branchMode == "" {
		branchMode = "auto"
	}
	return map[string]any{
		"run_id":                runID,
		"state":                 state,
		"branch_mode":           branchMode,
		"suggested_branch_name": nullable(suggestedBranch),
	}, nil
}

func workflowForRun(ctx context.Context, runner any, repositoryID string, run map[string]any) (map[string]any, error) {
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return nil, err
	}
	return asMap(snapshot["workflow_json"]), nil
}

func effectiveFreshSessionRequired(job map[string]any) bool {
	if job["fresh_session_required"] == true {
		return true
	}
	_, hasExplicit := job["fresh_session_required"]
	return job["type"] == "review" && job["reviewer_context_policy"] == "fresh" && !hasExplicit
}

func workflowJobType(workflow map[string]any, workflowJobID string) string {
	for _, item := range asList(workflow["jobs"]) {
		job := asMap(item)
		if job["id"] == workflowJobID {
			typ, _ := job["type"].(string)
			return typ
		}
	}
	return ""
}

type phaseIndex struct {
	declared         bool
	phaseOrder       []string
	phasePosition    map[string]int
	jobPhase         map[string]string
	synthesisByPhase map[string]string
}

type dependencyEdge struct {
	fromID string
	toID   string
}

func validateWorkflowForPrepare(workflow map[string]any) (phaseIndex, error) {
	for _, field := range []string{"schema_version", "workflow_id", "branch", "lanes", "roles", "jobs"} {
		if _, ok := workflow[field]; !ok {
			return phaseIndex{}, rpc.NewError("workflow_error", "workflow is missing required field "+field, nil)
		}
	}
	schemaVersion, _ := workflow["schema_version"].(string)
	if schemaVersion != "striatum.workflow.v1" && schemaVersion != "striatum.workflow.v1.1" {
		return phaseIndex{}, rpc.NewError("workflow_error", "workflow schema_version must be one of: striatum.workflow.v1, striatum.workflow.v1.1", nil)
	}
	roles := asMap(workflow["roles"])
	lanes := asMap(workflow["lanes"])
	jobs := workflowJobDefinitions(workflow)
	seen := map[string]bool{}
	for index, job := range jobs {
		jobID, _ := job["id"].(string)
		if jobID == "" {
			return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("jobs[%d].id must be a non-empty string", index), nil)
		}
		if seen[jobID] {
			return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("duplicate job id %q", jobID), nil)
		}
		seen[jobID] = true
		roleID, _ := job["role_id"].(string)
		if roleID == "" || roles[roleID] == nil {
			return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("job %q references unknown role %q", jobID, roleID), nil)
		}
		if laneID, ok := job["lane_id"].(string); ok && laneID != "" && lanes[laneID] == nil {
			return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("job %q references unknown lane %q", jobID, laneID), nil)
		}
		for _, artifactValue := range asList(job["expected_artifacts"]) {
			artifact := asMap(artifactValue)
			path, _ := artifact["path"].(string)
			if path == "" || filepath.IsAbs(path) || strings.Contains(path, "..") {
				return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("job %q has invalid artifact path", jobID), nil)
			}
			if kind, ok := artifact["kind"].(string); ok && kind != "" && !allowedArtifactKinds[kind] {
				return phaseIndex{}, rpc.NewError("workflow_error", fmt.Sprintf("job %s declares unknown artifact kind %s", jobID, kind), nil)
			}
		}
	}
	index, err := workflowPhaseIndex(workflow, jobs, schemaVersion)
	if err != nil {
		return phaseIndex{}, err
	}
	for _, edge := range edgeDependencyPairs(workflow, index, false) {
		if !seen[edge.fromID] || !seen[edge.toID] {
			return phaseIndex{}, rpc.NewError("workflow_error", "workflow edge references an unknown job", nil)
		}
	}
	if err := validatePhaseEdges(workflow, index); err != nil {
		return phaseIndex{}, err
	}
	return index, nil
}

func workflowJobDefinitions(workflow map[string]any) []map[string]any {
	result := []map[string]any{}
	for _, item := range asList(workflow["jobs"]) {
		result = append(result, asMap(item))
	}
	return result
}

func workflowPhaseIndex(workflow map[string]any, jobs []map[string]any, schemaVersion string) (phaseIndex, error) {
	empty := phaseIndex{phasePosition: map[string]int{}, jobPhase: map[string]string{}, synthesisByPhase: map[string]string{}}
	phases := asList(workflow["phases"])
	if schemaVersion == "striatum.workflow.v1" {
		if len(phases) > 0 {
			return empty, rpc.NewError("workflow_error", "striatum.workflow.v1 workflows must not declare phases", nil)
		}
		for _, job := range jobs {
			if _, ok := job["phase_id"]; ok {
				return empty, rpc.NewError("workflow_error", fmt.Sprintf("striatum.workflow.v1 job %q must not declare phase_id", job["id"]), nil)
			}
			if job["type"] == "phase_synthesis" {
				return empty, rpc.NewError("workflow_error", fmt.Sprintf("striatum.workflow.v1 job %q must not use type phase_synthesis", job["id"]), nil)
			}
		}
		return empty, nil
	}
	if len(phases) == 0 {
		for _, job := range jobs {
			if _, ok := job["phase_id"]; ok {
				return empty, rpc.NewError("workflow_error", fmt.Sprintf("job %q may declare phase_id only when workflow phases are declared", job["id"]), nil)
			}
			if job["type"] == "phase_synthesis" {
				return empty, rpc.NewError("workflow_error", fmt.Sprintf("job %q may use type phase_synthesis only when workflow phases are declared", job["id"]), nil)
			}
		}
		return empty, nil
	}
	index := phaseIndex{declared: true, phasePosition: map[string]int{}, jobPhase: map[string]string{}, synthesisByPhase: map[string]string{}}
	phaseSeen := map[string]bool{}
	for pos, phaseValue := range phases {
		phase := asMap(phaseValue)
		phaseID, _ := phase["id"].(string)
		if phaseID == "" || phaseSeen[phaseID] {
			return empty, rpc.NewError("workflow_error", "phase id must be unique and non-empty", nil)
		}
		phaseSeen[phaseID] = true
		index.phaseOrder = append(index.phaseOrder, phaseID)
		index.phasePosition[phaseID] = pos
	}
	jobCountByPhase := map[string]int{}
	for _, job := range jobs {
		jobID, _ := job["id"].(string)
		phaseID, _ := job["phase_id"].(string)
		if phaseID == "" || !phaseSeen[phaseID] {
			return empty, rpc.NewError("workflow_error", fmt.Sprintf("job %q references unknown phase_id %q", jobID, phaseID), nil)
		}
		index.jobPhase[jobID] = phaseID
		jobCountByPhase[phaseID]++
		if job["type"] != "phase_synthesis" {
			continue
		}
		if existing := index.synthesisByPhase[phaseID]; existing != "" {
			return empty, rpc.NewError("workflow_error", fmt.Sprintf("phase %q has multiple phase_synthesis jobs: %s, %s", phaseID, existing, jobID), nil)
		}
		index.synthesisByPhase[phaseID] = jobID
	}
	for _, phaseID := range index.phaseOrder {
		if index.synthesisByPhase[phaseID] == "" {
			return empty, rpc.NewError("workflow_error", fmt.Sprintf("phase %q must declare exactly one phase_synthesis job", phaseID), nil)
		}
		if jobCountByPhase[phaseID] < 2 {
			return empty, rpc.NewError("workflow_error", fmt.Sprintf("phase %q phase_synthesis job must have at least one peer job", phaseID), nil)
		}
	}
	return index, nil
}

func edgeDependencyPairs(workflow map[string]any, index phaseIndex, includePhaseMaterialized bool) []dependencyEdge {
	pairs := []dependencyEdge{}
	seen := map[string]bool{}
	for _, edgeValue := range asList(workflow["edges"]) {
		edge := asMap(edgeValue)
		fromID, _ := edge["from"].(string)
		toID, _ := edge["to"].(string)
		if fromID == "" || toID == "" || edge["on"] != "completed" {
			continue
		}
		key := fromID + "\x00" + toID
		if seen[key] {
			continue
		}
		seen[key] = true
		pairs = append(pairs, dependencyEdge{fromID: fromID, toID: toID})
	}
	if includePhaseMaterialized && index.declared {
		for _, job := range workflowJobDefinitions(workflow) {
			jobID, _ := job["id"].(string)
			phaseID := index.jobPhase[jobID]
			synthesisID := index.synthesisByPhase[phaseID]
			if jobID == "" || jobID == synthesisID {
				continue
			}
			key := jobID + "\x00" + synthesisID
			if seen[key] {
				continue
			}
			seen[key] = true
			pairs = append(pairs, dependencyEdge{fromID: jobID, toID: synthesisID})
		}
	}
	return pairs
}

func validatePhaseEdges(workflow map[string]any, index phaseIndex) error {
	if !index.declared {
		return nil
	}
	jobTypes := map[string]string{}
	for _, job := range workflowJobDefinitions(workflow) {
		jobID, _ := job["id"].(string)
		jobType, _ := job["type"].(string)
		jobTypes[jobID] = jobType
	}
	for _, edge := range edgeDependencyPairs(workflow, index, false) {
		fromPhase := index.jobPhase[edge.fromID]
		toPhase := index.jobPhase[edge.toID]
		if fromPhase == "" || toPhase == "" || fromPhase == toPhase {
			continue
		}
		fromPos := index.phasePosition[fromPhase]
		toPos := index.phasePosition[toPhase]
		if toPos < fromPos {
			return rpc.NewError("workflow_error", fmt.Sprintf("workflow edge %q -> %q points from later phase %q to earlier phase %q", edge.fromID, edge.toID, fromPhase, toPhase), nil)
		}
		if toPos != fromPos+1 {
			return rpc.NewError("workflow_error", fmt.Sprintf("workflow edge %q -> %q skips phases; cross-phase edges may target only the immediate next phase", edge.fromID, edge.toID), nil)
		}
		if index.synthesisByPhase[fromPhase] != edge.fromID {
			return rpc.NewError("workflow_error", fmt.Sprintf("workflow edge %q -> %q crosses phases without using source phase %q synthesis job", edge.fromID, edge.toID, fromPhase), nil)
		}
		if jobTypes[edge.toID] == "phase_synthesis" {
			return rpc.NewError("workflow_error", fmt.Sprintf("workflow edge %q -> %q cannot target a later phase_synthesis job", edge.fromID, edge.toID), nil)
		}
	}
	return nil
}

func valueOr(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	if text, ok := value.(string); ok && text == "" {
		return fallback
	}
	return value
}
