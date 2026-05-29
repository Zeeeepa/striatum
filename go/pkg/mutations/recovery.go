package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
	"github.com/jackc/pgx/v5"
)

var cancelableJobStates = map[string]bool{
	"blocked":       true,
	"queued":        true,
	"claimed":       true,
	"running":       true,
	"stale_lease":   true,
	"waiting_human": true,
}

var processAdapterBlockerKinds = map[string]bool{
	"process_outputs_missing":           true,
	"process_review_verdict_missing":    true,
	"process_exit_nonzero":              true,
	"process_timeout_exceeded":          true,
	"process_lost_with_outputs_missing": true,
}

var processExitBlockerKinds = map[string]bool{
	"process_exit_nonzero":     true,
	"process_timeout_exceeded": true,
}

var recoveryDrainHelperEvents = drainHelperEvents

var terminalJobStates = map[string]bool{
	"completed": true,
	"failed":    true,
	"canceled":  true,
	"skipped":   true,
}

func HandleRecoveryProcessReconcile(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.process_reconcile requires run_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		repoRoot := fmt.Sprint(run["repo_root"])
		rows, err := queryRows(ctx, tx, `
			SELECT pe.*,
			       p.metadata_json AS supervisor_metadata_json,
			       p.pid_start_time AS supervisor_pid_start_time,
			       p.pid AS supervisor_pid,
			       p.supervisor_id,
			       p.daemon_supervisor_id
			  FROM striatumd.process_executions pe
			  LEFT JOIN LATERAL (
			    SELECT ptr.metadata_json, ptr.pid_start_time, ptr.pid, ptr.supervisor_id, ptr.daemon_supervisor_id
			      FROM striatumd.process_supervisor_pointers ptr
			     WHERE ptr.repository_id = pe.repository_id
			       AND ptr.run_id = pe.run_id
			       AND ptr.session_id = pe.session_id
			     ORDER BY ptr.updated_at DESC, ptr.supervisor_id DESC
			     LIMIT 1
			  ) p ON true
			 WHERE pe.repository_id = $1
			   AND pe.run_id = $2
			   AND pe.state = 'running'
			 ORDER BY pe.started_at
			 FOR UPDATE OF pe`, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		stillRunning := []map[string]any{}
		lost := []map[string]any{}
		now := nowString()
		for _, row := range rows {
			pid := intValue(row["pid"])
			metadata := asMap(row["supervisor_metadata_json"])
			probePID := pid
			if supervisorPID := intValue(row["supervisor_pid"]); supervisorPID > 0 {
				probePID = supervisorPID
			}
			expectedStart, _ := row["supervisor_pid_start_time"].(string)
			live := gosupervisor.ProbeLaneLiveness(ctx, supervisionTmuxRunner, metadata, probePID, expectedStart)
			alive := live.Alive
			if live.Backed != "tmux" && pid > 0 {
				alive = pidAlive(pid)
			}
			if live.Class == string(gosupervisor.TmuxLivenessUnavailable) {
				stillRunning = append(stillRunning, map[string]any{
					"process_id": row["process_id"],
					"job_id":     row["job_id"],
					"pid":        pid,
					"started_at": row["started_at"],
					"liveness":   live.Class,
				})
				continue
			}
			if alive {
				stillRunning = append(stillRunning, map[string]any{
					"process_id": row["process_id"],
					"job_id":     row["job_id"],
					"pid":        pid,
					"started_at": row["started_at"],
					"liveness":   live.Class,
				})
				continue
			}
			processID := fmt.Sprint(row["process_id"])
			if err := tx.Exec(ctx, `
				UPDATE striatumd.process_executions
				   SET state = 'lost', ended_at = $1
				 WHERE repository_id = $2 AND process_id = $3`, now, repositoryID, processID); err != nil {
				return nil, err
			}
			var supervisorID string
			if s, ok := row["supervisor_id"].(string); ok {
				supervisorID = s
			}
			var daemonSupervisorID string
			if ds, ok := row["daemon_supervisor_id"].(string); ok {
				daemonSupervisorID = ds
			}
			if supervisorID != "" {
				stopReason := "unexpected child exit (lost)"
				if err := updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "stopped", now, 0, "", "", &now, &stopReason); err != nil {
					return nil, err
				}
				agentloop.CleanupGeminiSettings(repoRoot, supervisorID)
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "process.lost", row["session_id"], row["job_id"], nil, nil, row["lease_id"], map[string]any{
				"process_id": processID,
				"pid":        row["pid"],
				"reason":     live.Class,
			}); err != nil {
				return nil, err
			}
			job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(row["job_id"]), true)
			if err != nil {
				return nil, err
			}
			blockerKind, err := evaluateAndBlockLostProcess(ctx, tx, repositoryID, job, fmt.Sprint(row["session_id"]), processID, row["command_json"])
			if err != nil {
				return nil, err
			}
			lost = append(lost, map[string]any{
				"process_id":   processID,
				"job_id":       row["job_id"],
				"pid":          row["pid"],
				"blocker_kind": blockerKind,
			})
		}
		return map[string]any{
			"run_id":               runID,
			"checked_count":        len(rows),
			"still_running":        stillRunning,
			"transitioned_to_lost": lost,
			"next_actions":         []string{"inspect_process_blockers", "resume_or_requeue_affected_work"},
		}, nil
	})
}

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
		blocker, err := rowByID(ctx, tx, repositoryID, "blockers", "blocker_id", blockerID, true)
		if err != nil {
			return nil, err
		}
		blockerKind := fmt.Sprint(blocker["blocker_kind"])
		if !processAdapterBlockerKinds[blockerKind] {
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

func HandleRecoveryAuto(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.auto_publish_stale_artifacts requires run_id", nil)
	}
	dryRun := boolParam(envelope, "dry_run")
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		if !dryRun {
			if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
				return nil, err
			}
		}
		rows, err := queryRows(ctx, tx, `
			SELECT j.*, l.lease_id, l.owner_session_id,
			       qm.message_id, qm.state AS message_state,
			       l.state AS lease_state
			  FROM striatumd.jobs j
			  LEFT JOIN striatumd.leases l
			    ON l.repository_id = j.repository_id
			   AND (l.lease_id = j.current_lease_id
			        OR (l.resource_id = j.job_id AND l.state = 'expired'))
			  LEFT JOIN striatumd.queue_messages qm
			    ON qm.repository_id = j.repository_id
			   AND qm.message_id = j.current_message_id
			 WHERE j.repository_id = $1
			   AND j.run_id = $2
			   AND j.state IN ('claimed','running','stale_lease')
			   AND (l.state = 'expired'
			        OR (l.state = 'active' AND l.expires_at < $3::timestamptz))
			 ORDER BY j.workflow_job_id`, repositoryID, runID, nowString())
		if err != nil {
			return nil, err
		}
		skipped := []map[string]any{}
		published := []map[string]any{}
		seen := map[string]bool{}
		for _, row := range rows {
			key := fmt.Sprintf("%v/%v", row["job_id"], row["lease_id"])
			if seen[key] {
				continue
			}
			seen[key] = true
			jobID := fmt.Sprint(row["job_id"])
			workflowJobID := fmt.Sprint(row["workflow_job_id"])
			sessionID := fmt.Sprint(nullable(row["owner_session_id"]))
			leaseID := fmt.Sprint(nullable(row["lease_id"]))
			messageID := fmt.Sprint(nullable(row["message_id"]))
			expected := asList(row["expected_artifacts_json"])
			if len(expected) == 0 {
				skipped = append(skipped, map[string]any{
					"workflow_job_id": workflowJobID,
					"reason":          "no expected_artifacts declared",
				})
				continue
			}
			if sessionID == "" || sessionID == "<nil>" || leaseID == "" || leaseID == "<nil>" || messageID == "" || messageID == "<nil>" {
				skipped = append(skipped, map[string]any{
					"workflow_job_id": workflowJobID,
					"reason":          "no recoverable session/lease/message triple",
				})
				continue
			}
			job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
			if err != nil {
				return nil, err
			}
			expectedByline, err := expectedAuthorLine(ctx, tx, repositoryID, job, sessionID)
			if err != nil {
				skipped = append(skipped, map[string]any{
					"workflow_job_id": workflowJobID,
					"reason":          "could not derive expected byline: " + err.Error(),
				})
				continue
			}
			publishable, err := autoPublishableArtifacts(ctx, tx, repositoryID, fmt.Sprint(run["repo_root"]), job, sessionID, expectedByline)
			if err != nil {
				return nil, err
			}
			if len(publishable) == 0 {
				skipped = append(skipped, map[string]any{
					"workflow_job_id": workflowJobID,
					"reason":          "no required expected_artifact found on disk with matching byline",
				})
				continue
			}
			if dryRun {
				items := []map[string]any{}
				for _, declared := range publishable {
					items = append(items, map[string]any{
						"path":         declared["path"],
						"kind":         declared["kind"],
						"logical_name": declared["logical_name"],
					})
				}
				published = append(published, map[string]any{
					"workflow_job_id": workflowJobID,
					"session_id":      sessionID,
					"would_publish":   items,
					"would_expire":    fmt.Sprint(row["lease_state"]) == "active",
				})
				continue
			}
			artifacts := []map[string]any{}
			for _, declared := range publishable {
				artifact, err := publishRecoveredArtifact(ctx, tx, repositoryID, job, sessionID, leaseID, fmt.Sprint(run["repo_root"]), declared)
				if err != nil {
					return nil, err
				}
				artifacts = append(artifacts, artifact)
			}
			complete, err := completeAutoRecoveredJob(ctx, tx, repositoryID, jobID, sessionID, leaseID, messageID)
			if err != nil {
				return nil, err
			}
			paths := []any{}
			for _, declared := range publishable {
				paths = append(paths, declared["path"])
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.auto_published", sessionID, jobID, nil, nil, leaseID, map[string]any{
				"workflow_job_id": workflowJobID,
				"artifacts":       paths,
				"byline":          expectedByline,
			}); err != nil {
				return nil, err
			}
			published = append(published, map[string]any{
				"workflow_job_id": workflowJobID,
				"session_id":      sessionID,
				"artifacts":       artifacts,
				"complete":        complete,
			})
		}
		if !dryRun {
			if err := maybeCompleteRun(ctx, tx, repositoryID, runID); err != nil {
				return nil, err
			}
		}
		helperEvents, err := drainRunHelperEvents(ctx, tx, repositoryID, runID, dryRun)
		if err != nil {
			return nil, err
		}
		liveness, err := refreshRunLiveness(ctx, tx, repositoryID, runID, dryRun)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"run_id":          runID,
			"dry_run":         dryRun,
			"published_count": len(published),
			"published":       published,
			"skipped_count":   len(skipped),
			"skipped":         skipped,
			"helper_events":   helperEvents,
			"liveness":        liveness,
		}, nil
	})
}

func drainRunHelperEvents(ctx context.Context, tx db.TxRunner, repositoryID string, runID string, dryRun bool) (map[string]any, error) {
	rows, err := queryRows(ctx, tx,
		`SELECT supervisor_id
		   FROM striatumd.process_supervisor_pointers
		  WHERE repository_id = $1
		    AND run_id = $2
		    AND state IN ('starting','attached')
		  ORDER BY supervisor_id`,
		repositoryID,
		runID,
	)
	if err != nil {
		return nil, err
	}
	drained := []string{}
	for _, row := range rows {
		supervisorID := fmt.Sprint(nullable(row["supervisor_id"]))
		if supervisorID == "" || supervisorID == "<nil>" {
			continue
		}
		if dryRun {
			continue
		}
		if err := recoveryDrainHelperEvents(ctx, tx, repositoryID, supervisorID, 0); err != nil {
			return nil, err
		}
		drained = append(drained, supervisorID)
	}
	return map[string]any{
		"checked_count": len(rows),
		"drained_count": len(drained),
		"drained":       drained,
		"dry_run":       dryRun,
	}, nil
}

func refreshRunLiveness(ctx context.Context, tx db.TxRunner, repositoryID string, runID string, dryRun bool) (map[string]any, error) {
	rows, err := queryRows(ctx, tx,
		`SELECT s.session_id, s.run_id, s.role_id, s.lane_id, s.state, s.registered_at,
		        s.last_mcp_request_at,
		        s.last_tools_list_at,
		        s.last_await_packet_at,
		        s.last_packet_delivered_at,
		        s.last_ack_at,
		        s.last_work_block_at,
		        s.last_work_release_at,
		        s.last_work_complete_at,
		        s.last_work_heartbeat_at,
		        s.last_session_ready_at,
		        s.last_session_heartbeat_at,
		        s.last_session_question_at,
		        s.last_session_escalate_at,
		        s.liveness_stall_class,
		        s.liveness_stall_since,
		        active_lease.lease_id AS active_lease_id,
		        active_lease.acquired_at AS active_lease_acquired_at,
		        active_lease.expires_at AS active_lease_expires_at,
		        active_lease.last_heartbeat_at AS active_lease_last_heartbeat_at
		   FROM striatumd.sessions s
		   LEFT JOIN LATERAL (
		     SELECT l.lease_id, l.acquired_at, l.expires_at, l.last_heartbeat_at
		       FROM striatumd.leases l
		      WHERE l.repository_id = s.repository_id
		        AND l.owner_session_id = s.session_id
		        AND l.state = 'active'
		      ORDER BY l.acquired_at DESC, l.lease_id DESC
		      LIMIT 1
		   ) active_lease ON true
		  WHERE s.repository_id = $1
		    AND s.run_id = $2
		    AND s.state = 'active'
		  ORDER BY s.registered_at, s.session_id
		  FOR UPDATE OF s`,
		repositoryID,
		runID,
	)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	missed := []map[string]any{}
	recovered := []map[string]any{}
	for _, row := range rows {
		activity := sessionliveness.ActivityFromRow(row)
		result := sessionliveness.Classify(activity, sessionliveness.DefaultPolicy(), now)
		previous := fmt.Sprint(nullable(row[sessionliveness.LivenessStallClass]))
		if previous == "<nil>" {
			previous = ""
		}
		if previous == result.StallClass {
			continue
		}
		sessionID := fmt.Sprint(row["session_id"])
		var stallSince any
		if result.StallSince != nil {
			stallSince = *result.StallSince
		}
		if !dryRun {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.sessions
				   SET liveness_stall_class = $1,
				       liveness_stall_since = $2
				 WHERE repository_id = $3 AND session_id = $4`,
				nullable(result.StallClass),
				stallSince,
				repositoryID,
				sessionID,
			); err != nil {
				return nil, err
			}
			if result.StallClass != "" {
				if _, err := appendEvent(ctx, tx, repositoryID, runID, "session.liveness_deadline_missed", sessionID, nil, nil, nil, nil, map[string]any{
					"repository_id":           repositoryID,
					"run_id":                  runID,
					"session_id":              sessionID,
					"lane_id":                 row["lane_id"],
					"role_id":                 row["role_id"],
					"stall_class":             result.StallClass,
					"deadline_name":           result.DeadlineName,
					"deadline_seconds":        result.DeadlineSeconds,
					"observed_at":             now.Format(time.RFC3339),
					"last_activity_timestamp": relevantActivityTimestamp(activity, result.DeadlineName),
				}); err != nil {
					return nil, err
				}
			} else if previous != "" {
				if _, err := appendEvent(ctx, tx, repositoryID, runID, "session.liveness_recovered", sessionID, nil, nil, nil, nil, map[string]any{
					"repository_id":        repositoryID,
					"run_id":               runID,
					"session_id":           sessionID,
					"lane_id":              row["lane_id"],
					"role_id":              row["role_id"],
					"previous_stall_class": previous,
					"observed_at":          now.Format(time.RFC3339),
					"signal":               "liveness_sweep",
				}); err != nil {
					return nil, err
				}
			}
		}
		item := map[string]any{
			"session_id": sessionID,
			"previous":   nullable(previous),
			"current":    nullable(result.StallClass),
		}
		if result.StallClass != "" {
			missed = append(missed, item)
		} else {
			recovered = append(recovered, item)
		}
	}
	return map[string]any{
		"checked_count":   len(rows),
		"missed_count":    len(missed),
		"missed":          missed,
		"recovered_count": len(recovered),
		"recovered":       recovered,
		"dry_run":         dryRun,
	}, nil
}

func relevantActivityTimestamp(activity sessionliveness.Activity, deadline string) any {
	var value *time.Time
	switch deadline {
	case sessionliveness.DeadlineDiscovery:
		value = activity.RegisteredAt
	case sessionliveness.DeadlineAwaitPacket:
		value = activity.LastToolsListAt
	case sessionliveness.DeadlineAck:
		value = activity.LastPacketDeliveredAt
	case sessionliveness.DeadlineLeaseHeartbeat:
		value = latestMutationTime(activity.LastWorkHeartbeatAt, activity.ActiveLeaseHeartbeatAt, activity.ActiveLeaseAcquiredAt)
	case sessionliveness.DeadlineQuestionPending:
		value = activity.LastSessionQuestionAt
	case sessionliveness.DeadlineEscalation:
		value = activity.LastSessionEscalateAt
	default:
		value = latestMutationTime(
			activity.LastMCPRequestAt,
			activity.LastToolsListAt,
			activity.LastAwaitPacketAt,
			activity.LastPacketDeliveredAt,
			activity.LastAckAt,
			activity.LastWorkBlockAt,
			activity.LastWorkReleaseAt,
			activity.LastWorkCompleteAt,
			activity.LastWorkHeartbeatAt,
			activity.LastSessionReadyAt,
			activity.LastSessionHeartbeatAt,
			activity.LastSessionQuestionAt,
			activity.LastSessionEscalateAt,
			activity.RegisteredAt,
		)
	}
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func latestMutationTime(values ...*time.Time) *time.Time {
	var latest *time.Time
	for _, value := range values {
		if value == nil {
			continue
		}
		if latest == nil || value.After(*latest) {
			latest = value
		}
	}
	return latest
}

func SweepRun(ctx context.Context, runner db.Runner, repositoryID string, runID string, author string) (map[string]any, error) {
	if author == "" {
		author = "striatumd-go"
	}
	result, err := HandleRecoveryAuto(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "daemon_sweep_" + runID,
		Method:        "recovery.sweep",
		Params: map[string]any{
			"repository_id": repositoryID,
			"run_id":        runID,
		},
	})
	if err != nil {
		return nil, err
	}
	_, err = withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		_, err := appendEvent(ctx, tx, repositoryID, runID, "daemon.recovery_sweep", nil, nil, nil, nil, nil, map[string]any{
			"author":        author,
			"repository_id": repositoryID,
			"result":        result,
		})
		return map[string]any{}, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func autoPublishableArtifacts(ctx context.Context, runner any, repositoryID string, repoRoot string, job map[string]any, sessionID string, expectedByline string) ([]map[string]any, error) {
	publishable := []map[string]any{}
	for _, item := range asList(job["expected_artifacts_json"]) {
		declared := asMap(item)
		if declared["required"] == false {
			continue
		}
		pathText, _ := declared["path"].(string)
		kind, _ := declared["kind"].(string)
		logicalName, _ := declared["logical_name"].(string)
		if pathText == "" || kind == "" || logicalName == "" {
			continue
		}
		path, err := repoRelativePath(repoRoot, pathText, false)
		if err != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !utf8.Valid(payload) {
			continue
		}
		matched := false
		for _, line := range markdownTitleBlockAuthorLines(string(payload)) {
			if canonicalBylineForm(line) == expectedByline {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		publishable = append(publishable, map[string]any{
			"path":         pathText,
			"kind":         kind,
			"logical_name": logicalName,
		})
	}
	return publishable, nil
}

func publishRecoveredArtifact(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string, leaseID string, repoRoot string, declared map[string]any) (map[string]any, error) {
	kind := fmt.Sprint(declared["kind"])
	logicalName := fmt.Sprint(declared["logical_name"])
	pathText := fmt.Sprint(declared["path"])
	if kind == "transcript" {
		return nil, rpc.NewError("artifact_error", "transcript artifacts are not allowed by default", nil)
	}
	if !allowedArtifactKinds[kind] {
		return nil, rpc.NewError("artifact_error", fmt.Sprintf("artifact kind %q is not in the allowed kinds list", kind), nil)
	}
	if !pathAllowed(repoRoot, pathText, asMap(job["write_scope_json"])) {
		return nil, rpc.NewError("artifact_error", "artifact path is outside the job write scope", nil)
	}
	path, err := repoRelativePath(repoRoot, pathText, false)
	if err != nil {
		return nil, rpc.NewError("artifact_error", err.Error(), nil)
	}
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, rpc.NewError("artifact_error", "artifact file does not exist", nil)
	}
	payload, err = ensureRequiredFrontMatter(kind, path, payload)
	if err != nil {
		return nil, err
	}
	if err := validateMarkdownAuthorLine(ctx, runner, repositoryID, job, sessionID, path, payload); err != nil {
		return nil, err
	}
	if err := validateArtifactFrontMatter(kind, path, payload); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	existing, err := oneRow(ctx, runner, `
		SELECT * FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3 AND logical_name = $4
		 LIMIT 1`, repositoryID, job["run_id"], job["job_id"], logicalName)
	if err == nil {
		if fmt.Sprint(existing["content_sha256"]) == digest && fmt.Sprint(existing["repo_path"]) == pathText {
			return map[string]any{"status": "already_published", "artifact_id": existing["artifact_id"]}, nil
		}
		return nil, rpc.NewError("artifact_error", "artifact logical name already exists with different content", nil)
	}
	if !errorsIsNoRows(err) {
		return nil, err
	}
	artifactID, err := newID("art")
	if err != nil {
		return nil, err
	}
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
		  created_at, author_line
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'create',$11,$12)`,
		repositoryID, artifactID, job["run_id"], job["job_id"], sessionID, logicalName,
		kind, pathText, digest, len(payload), now, nullable(firstAuthorLine(payload))); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "artifact.published", sessionID, job["job_id"], nil, artifactID, leaseID, map[string]any{
		"logical_name": logicalName,
		"path":         pathText,
		"sha256":       digest,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"status": "published", "artifact_id": artifactID, "sha256": digest}, nil
}

func completeAutoRecoveredJob(ctx context.Context, runner any, repositoryID, jobID, sessionID, leaseID, messageID string) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(job["state"]) != "stale_lease" && fmt.Sprint(job["state"]) != "running" && fmt.Sprint(job["state"]) != "claimed" {
		return nil, rpc.NewError("invalid_transition", "stale job is no longer auto-recoverable", nil)
	}
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	now := nowString()
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
	if messageID != "" && messageID != "<nil>" {
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
		   SET released_at = COALESCE(released_at, $1),
		       release_reason = COALESCE(release_reason, 'auto_published')
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.completed", sessionID, jobID, nullable(messageID), nil, leaseID, map[string]any{
		"summary": "auto-published on stale lease",
	}); err != nil {
		return nil, err
	}
	if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "job_id": jobID}, nil
}

func HandleRecoveryStaleLeases(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.stale_leases requires run_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if _, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true); err != nil {
			return nil, err
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		rows, err := queryRows(ctx, tx, `
			SELECT j.job_id, j.workflow_job_id, j.state AS job_state,
			       j.write_scope_json,
			       l.lease_id, l.owner_session_id, l.acquired_at,
			       l.expires_at, l.released_at, l.release_reason,
			       l.state AS lease_state,
			       qm.message_id, qm.state AS message_state
			  FROM striatumd.jobs j
			  LEFT JOIN striatumd.leases l
			    ON l.repository_id = j.repository_id
			   AND (l.lease_id = j.current_lease_id
			        OR (l.resource_id = j.job_id AND l.state = 'expired'))
			  LEFT JOIN striatumd.queue_messages qm
			    ON qm.repository_id = j.repository_id
			   AND qm.message_id = j.current_message_id
			 WHERE j.repository_id = $1
			   AND j.run_id = $2
			   AND (j.state = 'stale_lease' OR l.state = 'expired')
			 ORDER BY j.workflow_job_id, l.expires_at`, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		entries := []map[string]any{}
		seen := map[string]bool{}
		for _, row := range rows {
			key := fmt.Sprintf("%s/%v", row["job_id"], row["lease_id"])
			if seen[key] {
				continue
			}
			seen[key] = true
			repoWrite := isRepoWrite(row)
			policy := "safe_to_reclaim_when_pending"
			actions := []string{"register_or_select_session", "claim_available_work"}
			if repoWrite {
				policy = "manual_inspection_required"
				actions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
			}
			entries = append(entries, map[string]any{
				"job_id":           row["job_id"],
				"workflow_job_id":  row["workflow_job_id"],
				"job_state":        row["job_state"],
				"lease_id":         row["lease_id"],
				"owner_session_id": row["owner_session_id"],
				"expires_at":       row["expires_at"],
				"released_at":      row["released_at"],
				"release_reason":   row["release_reason"],
				"message_id":       row["message_id"],
				"message_state":    row["message_state"],
				"repo_write":       repoWrite,
				"recovery_policy":  policy,
				"next_actions":     actions,
			})
		}
		nextActions := []string{}
		if len(entries) > 0 {
			nextActions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
		}
		return map[string]any{
			"run_id":       runID,
			"stale_count":  len(entries),
			"stale_leases": entries,
			"next_actions": nextActions,
		}, nil
	})
}

func HandleRecoveryRequeueStale(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	jobID := stringParam(envelope, "job_id")
	if runID == "" || jobID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.requeue_stale requires run_id and job_id", nil)
	}
	force := boolParam(envelope, "force")
	justification := strings.TrimSpace(stringParam(envelope, "justification"))
	if force && justification == "" {
		return nil, rpc.NewError("invalid_transition", "--force requeue requires --justification", nil)
	}
	recoveryAuthor := stringParam(envelope, "recovery_author")

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if _, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true); err != nil {
			return nil, err
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		rows, err := queryRows(ctx, tx, `
			SELECT j.job_id, j.run_id, j.workflow_job_id, j.state,
			       j.role_id, j.lane_selector_json, j.max_attempts,
			       j.write_scope_json, j.current_message_id,
			       l.lease_id, l.owner_session_id, l.expires_at,
			       qm.message_id, qm.state AS message_state
			  FROM striatumd.jobs j
			  JOIN striatumd.leases l
			    ON l.repository_id = j.repository_id
			   AND l.resource_id = j.job_id
			   AND l.state = 'expired'
			  LEFT JOIN striatumd.queue_messages qm
			    ON qm.repository_id = j.repository_id
			   AND qm.message_id = j.current_message_id
			 WHERE j.repository_id = $1
			   AND j.run_id = $2
			   AND j.job_id = $3
			   AND j.state IN ('queued', 'blocked', 'stale_lease')
			 ORDER BY l.expires_at DESC
			 LIMIT 1
			 FOR UPDATE OF j, l`, repositoryID, runID, jobID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, rpc.NewError("invalid_transition", "job has no stale expired lease to requeue", nil)
		}
		row := rows[0]
		repoWrite := isRepoWrite(row)
		if repoWrite && !force {
			return nil, rpc.NewError("invalid_transition", "repo-write stale jobs require manual inspection; rerun with `--force --justification \"<reason>\"` to override after inspection", nil)
		}
		messageID := nullable(row["message_id"])
		alreadyReclaimable := fmt.Sprint(row["state"]) == "queued" && fmt.Sprint(row["message_state"]) == "pending"
		now := nowString()
		if messageID == nil {
			created, err := insertPendingMessageForJob(ctx, tx, repositoryID, row, now)
			if err != nil {
				return nil, err
			}
			messageID = created
		} else {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.jobs
				   SET state = 'queued', current_lease_id = NULL
				 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.queue_messages
				   SET state = 'pending', current_lease_id = NULL, updated_at = $1
				 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
				return nil, err
			}
		}
		payload := map[string]any{
			"already_reclaimable": alreadyReclaimable,
			"repo_write":          repoWrite,
		}
		if recoveryAuthor != "" {
			payload["author"] = recoveryAuthor
		}
		if force && repoWrite {
			payload["operator_override"] = true
			payload["justification"] = justification
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.stale_requeued", nil, jobID, messageID, nil, row["lease_id"], payload); err != nil {
			return nil, err
		}
		status := "requeued"
		if alreadyReclaimable {
			status = "already_reclaimable"
		}
		return map[string]any{
			"status":            status,
			"run_id":            runID,
			"job_id":            jobID,
			"workflow_job_id":   row["workflow_job_id"],
			"lease_id":          row["lease_id"],
			"message_id":        messageID,
			"repo_write":        repoWrite,
			"operator_override": force && repoWrite,
			"next_actions":      []string{"register_or_select_session", "claim_available_work"},
		}, nil
	})
}

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

func expireLeases(ctx context.Context, runner any, repositoryID, runID string) ([]map[string]any, error) {
	now := nowString()
	rows, err := queryRows(ctx, runner, `
		SELECT * FROM striatumd.leases
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND state = 'active'
		   AND expires_at < $3::timestamptz
		 FOR UPDATE`, repositoryID, runID, now)
	if err != nil {
		return nil, err
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	summaries := []map[string]any{}
	for _, lease := range rows {
		job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(lease["resource_id"]), true)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		messageID := nullable(job["current_message_id"])
		repoWrite := isRepoWrite(job)
		jobState := "queued"
		messageState := "pending"
		if repoWrite {
			jobState = "stale_lease"
			messageState = "blocked"
		}
		if err := exec.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'expired', released_at = $1, release_reason = 'expired'
			 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, lease["lease_id"]); err != nil {
			return nil, err
		}
		if err := exec.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = $1, current_lease_id = NULL
			 WHERE repository_id = $2 AND job_id = $3`, jobState, repositoryID, job["job_id"]); err != nil {
			return nil, err
		}
		if messageID != nil {
			if err := exec.Exec(ctx, `
				UPDATE striatumd.queue_messages
				   SET state = $1, current_lease_id = NULL, updated_at = $2
				 WHERE repository_id = $3 AND message_id = $4`, messageState, now, repositoryID, messageID); err != nil {
				return nil, err
			}
		}
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "lease.expired", nil, job["job_id"], messageID, nil, lease["lease_id"], map[string]any{
			"job_state":     jobState,
			"message_state": messageState,
		}); err != nil {
			return nil, err
		}
		worktrees, err := queryRows(ctx, runner, `
			SELECT worktree_id, lease_id, base_branch
			  FROM striatumd.job_worktrees
			 WHERE repository_id = $1
			   AND job_id = $2
			   AND state = 'active'
			 FOR UPDATE`, repositoryID, job["job_id"])
		if err != nil {
			return nil, err
		}
		for _, worktree := range worktrees {
			if fmt.Sprint(worktree["lease_id"]) != fmt.Sprint(lease["lease_id"]) {
				continue
			}
			if err := exec.Exec(ctx, `
				UPDATE striatumd.job_worktrees
				   SET state = 'abandoned'
				 WHERE repository_id = $1 AND worktree_id = $2`, repositoryID, worktree["worktree_id"]); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, runner, repositoryID, runID, "worktree.abandoned", nil, job["job_id"], nil, nil, lease["lease_id"], map[string]any{
				"worktree_id": fmt.Sprint(worktree["worktree_id"]),
				"base_branch": worktree["base_branch"],
			}); err != nil {
				return nil, err
			}
		}
		summaries = append(summaries, map[string]any{
			"lease_id":      fmt.Sprint(lease["lease_id"]),
			"job_id":        fmt.Sprint(job["job_id"]),
			"message_id":    nullable(messageID),
			"job_state":     jobState,
			"message_state": messageState,
			"repo_write":    repoWrite,
		})
	}
	return summaries, nil
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func evaluateAndBlockLostProcess(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string, processID string, command any) (any, error) {
	missingPaths, verdictMissing, err := validateProcessOutputs(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if len(missingPaths) == 0 && !verdictMissing {
		return nil, nil
	}
	existing, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.blockers
		 WHERE repository_id = $1
		   AND job_id = $2
		   AND state = 'open'
		 LIMIT 1`, repositoryID, job["job_id"])
	if err != nil {
		return nil, err
	}
	if existing {
		return nil, nil
	}
	blockerID, err := newID("blk")
	if err != nil {
		return nil, err
	}
	now := nowString()
	blockerKind := "process_lost_with_outputs_missing"
	description := fmt.Sprintf("process %s was lost (external kill or runner exit); required outputs missing: %d artifact(s), verdict missing=%v", processID, len(missingPaths), verdictMissing)
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	blockerPayload := map[string]any{
		"process_id":             processID,
		"command":                command,
		"missing_artifact_paths": missingPaths,
		"review_verdict_missing": verdictMissing,
	}
	blockerPayloadArg, err := db.JSONBArg(runner, blockerPayload)
	if err != nil {
		return nil, err
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id,
		  severity, blocker_kind, description, state, created_at, payload_json
		)
		VALUES ($1,$2,$3,$4,$5,'blocked',$6,$7,'open',$8,$9::jsonb)`,
		repositoryID,
		blockerID,
		job["run_id"],
		job["job_id"],
		sessionID,
		blockerKind,
		description,
		now,
		blockerPayloadArg,
	); err != nil {
		return nil, err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'blocked'
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, job["job_id"]); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.blocked", sessionID, job["job_id"], nil, nil, nil, map[string]any{
		"blocker_id":   blockerID,
		"blocker_kind": blockerKind,
	}); err != nil {
		return nil, err
	}
	return blockerKind, nil
}

func validateProcessOutputs(ctx context.Context, runner any, repositoryID string, job map[string]any) ([]string, bool, error) {
	requiredPaths := []string{}
	for _, item := range asList(job["expected_artifacts_json"]) {
		expected := asMap(item)
		if expected["required"] == false {
			continue
		}
		path, _ := expected["path"].(string)
		if path != "" {
			requiredPaths = append(requiredPaths, path)
		}
	}
	published := map[string]bool{}
	if len(requiredPaths) > 0 {
		rows, err := queryRows(ctx, runner, `
			SELECT repo_path FROM striatumd.artifacts
			 WHERE repository_id = $1 AND job_id = $2`, repositoryID, job["job_id"])
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			published[fmt.Sprint(row["repo_path"])] = true
		}
	}
	missing := []string{}
	for _, path := range requiredPaths {
		if !published[path] {
			missing = append(missing, path)
		}
	}
	verdictMissing := false
	if fmt.Sprint(job["job_type"]) == "review" {
		found, err := existsRow(ctx, runner, `
			SELECT 1 FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2
			 LIMIT 1`, repositoryID, job["job_id"])
		if err != nil {
			return nil, false, err
		}
		verdictMissing = !found
	}
	return missing, verdictMissing, nil
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
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
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
	if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "job_id": jobID}, nil
}

func insertPendingMessageForJob(ctx context.Context, runner any, repositoryID string, job map[string]any, now string) (string, error) {
	messageID, err := newID("msg")
	if err != nil {
		return "", err
	}
	selector := asMap(job["lane_selector_json"])
	targetLane, _ := selector["lane_id"].(string)
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return "", fmt.Errorf("runner does not support exec")
	}
	payloadArg, err := db.JSONBArg(runner, map[string]any{})
	if err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, payload_json, claim_count,
		  max_claims, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7::jsonb,0,$8,$9,$10)`,
		repositoryID,
		messageID,
		job["run_id"],
		job["job_id"],
		job["role_id"],
		nullable(targetLane),
		payloadArg,
		job["max_attempts"],
		now,
		now,
	); err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'queued', current_lease_id = NULL, current_message_id = $1
		 WHERE repository_id = $2 AND job_id = $3`, messageID, repositoryID, job["job_id"]); err != nil {
		return "", err
	}
	return messageID, nil
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
	return map[string]any{
		"job_id":          jobID,
		"workflow_job_id": job["workflow_job_id"],
		"previous_state":  job["state"],
	}, nil
}
