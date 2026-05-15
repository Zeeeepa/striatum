package mutations

import (
	"context"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func HandleRecordVerdict(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	leaseID := stringParam(envelope, "lease_id")
	verdict := stringParam(envelope, "verdict")
	findingsArtifactID := nullable(stringParam(envelope, "findings_artifact_id"))
	rationale := nullable(stringParam(envelope, "rationale"))
	if sessionID == "" || jobID == "" || leaseID == "" || verdict == "" {
		return nil, rpc.NewError("schema_invalid", "review.verdict requires session_id, job_id, lease_id, and verdict", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return recordVerdict(ctx, tx, repositoryID, sessionID, jobID, leaseID, verdict, findingsArtifactID, rationale)
	})
}

func HandleSubmitReview(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	leaseID := stringParam(envelope, "lease_id")
	pathText := stringParam(envelope, "path")
	verdict := stringParam(envelope, "verdict")
	logicalName := stringParam(envelope, "logical_name")
	if logicalName == "" {
		logicalName = "review"
	}
	kind := stringParam(envelope, "kind")
	if kind == "" {
		kind = "finding"
	}
	rationale := nullable(stringParam(envelope, "rationale"))
	if sessionID == "" || jobID == "" || leaseID == "" || pathText == "" || verdict == "" {
		return nil, rpc.NewError("schema_invalid", "review.submit requires session_id, job_id, lease_id, path, and verdict", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if err := prevalidateSubmitReview(ctx, tx, repositoryID, job, sessionID, leaseID, logicalName, kind, pathText); err != nil {
			return nil, err
		}
		if fmt.Sprint(job["state"]) == "claimed" && nullable(job["current_message_id"]) != nil {
			if err := ackInline(ctx, tx, repositoryID, sessionID, fmt.Sprint(job["current_message_id"]), leaseID); err != nil {
				return nil, err
			}
		}
		artifact, err := publishArtifact(ctx, tx, repositoryID, sessionID, jobID, leaseID, kind, logicalName, pathText)
		if err != nil {
			return nil, err
		}
		verdictResult, err := recordVerdict(ctx, tx, repositoryID, sessionID, jobID, leaseID, verdict, artifact["artifact_id"], rationale)
		if err != nil {
			return nil, err
		}
		finalJob, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, false)
		if err != nil {
			return nil, err
		}
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", fmt.Sprint(finalJob["run_id"]), false)
		if err != nil {
			return nil, err
		}
		downstream, err := downstreamJobs(ctx, tx, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"artifact":        artifact,
			"verdict":         verdictResult,
			"job_state":       finalJob["state"],
			"run_state":       run["state"],
			"blocker_id":      verdictResult["blocker_id"],
			"downstream_jobs": downstream,
		}, nil
	})
}

func HandleOverrideVerdict(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	verdict := stringParam(envelope, "verdict")
	rationale := stringParam(envelope, "rationale")
	findingsArtifactID := nullable(stringParam(envelope, "findings_artifact_id"))
	autoFreshSession := boolParam(envelope, "auto_fresh_session")
	if sessionID == "" || jobID == "" || verdict == "" || rationale == "" {
		return nil, rpc.NewError("schema_invalid", "review.override requires session_id, job_id, verdict, and rationale", nil)
	}
	if verdict != "accept" && verdict != "accept_with_findings" {
		return nil, rpc.NewError("invalid_transition", "override verdict must be 'accept' or 'accept_with_findings'", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(job["job_type"]) != "review" {
			return nil, rpc.NewError("invalid_transition", "verdict override is valid only for review jobs", nil)
		}
		jobState := fmt.Sprint(job["state"])
		if jobState != "completed" && jobState != "waiting_human" {
			return nil, rpc.NewError("invalid_transition", "verdict override requires a completed or waiting_human review job", nil)
		}
		if autoFreshSession {
			resolved, err := resolveOverrideSession(ctx, tx, repositoryID, sessionID, jobID)
			if err != nil {
				return nil, err
			}
			sessionID = resolved
		}
		session, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, false)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(session["run_id"]) != fmt.Sprint(job["run_id"]) {
			return nil, rpc.NewError("invalid_transition", "override session does not belong to the job run", nil)
		}
		if fmt.Sprint(session["state"]) != "active" {
			return nil, rpc.NewError("invalid_transition", "override session must be active", nil)
		}
		hasOwnVerdict, err := existsRow(ctx, tx, `
			SELECT 1 FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2 AND session_id = $3
			 LIMIT 1`, repositoryID, jobID, sessionID)
		if err != nil {
			return nil, err
		}
		if hasOwnVerdict {
			return nil, rpc.NewError("invalid_transition", "override session already has a verdict for this job; register a fresh session", nil)
		}
		previous, err := oneRow(ctx, tx, `
			SELECT * FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2
			 ORDER BY created_at DESC, verdict_id DESC
			 LIMIT 1`, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		previousVerdict := fmt.Sprint(previous["verdict"])
		if previousVerdict == "accept" || previousVerdict == "accept_with_findings" {
			return map[string]any{"status": "already_accepting", "job_id": jobID, "previous_verdict": previousVerdict}, nil
		}
		effectiveArtifactID := findingsArtifactID
		if effectiveArtifactID == nil {
			effectiveArtifactID = nullable(previous["findings_artifact_id"])
		}
		if effectiveArtifactID != nil {
			artifact, err := rowByID(ctx, tx, repositoryID, "artifacts", "artifact_id", fmt.Sprint(effectiveArtifactID), false)
			if err != nil {
				return nil, err
			}
			if fmt.Sprint(artifact["run_id"]) != fmt.Sprint(job["run_id"]) {
				return nil, rpc.NewError("invalid_transition", "findings artifact belongs to a different run", nil)
			}
			if fmt.Sprint(artifact["job_id"]) != jobID {
				return nil, rpc.NewError("invalid_transition", "findings artifact belongs to a different job", nil)
			}
		}
		verdictID, err := newID("verdict")
		if err != nil {
			return nil, err
		}
		posture, err := resolveReviewPosture(ctx, tx, repositoryID, job)
		if err != nil {
			return nil, err
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.verdicts (
			  repository_id, verdict_id, run_id, job_id, session_id,
			  verdict, rationale, findings_artifact_id, created_at, posture
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			repositoryID,
			verdictID,
			job["run_id"],
			jobID,
			sessionID,
			verdict,
			strings.TrimSpace(rationale),
			effectiveArtifactID,
			now,
			posture,
		); err != nil {
			return nil, err
		}
		resolvedBlockers := 0
		if jobState == "waiting_human" {
			messageID := nullable(job["current_message_id"])
			if err := tx.Exec(ctx, `
				UPDATE striatumd.jobs
				   SET state = 'completed', completed_at = $1
				 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
				return nil, err
			}
			if messageID != nil {
				if err := tx.Exec(ctx, `
					UPDATE striatumd.queue_messages
					   SET state = 'completed', completed_at = $1, updated_at = $2
					 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
					return nil, err
				}
			}
			rows, err := queryRows(ctx, tx, `
				SELECT blocker_id FROM striatumd.blockers
				 WHERE repository_id = $1
				   AND job_id = $2
				   AND state = 'open'
				   AND severity = 'human_checkpoint'
				   AND blocker_kind = 'revision_routing'
				 FOR UPDATE`, repositoryID, jobID)
			if err != nil {
				return nil, err
			}
			resolvedBlockers = len(rows)
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2
				   AND job_id = $3
				   AND state = 'open'
				   AND severity = 'human_checkpoint'
				   AND blocker_kind = 'revision_routing'`, now, repositoryID, jobID); err != nil {
				return nil, err
			}
		}
		if _, err := appendEvent(ctx, tx, repositoryID, job["run_id"], "verdict.overridden", sessionID, jobID, nil, effectiveArtifactID, nil, map[string]any{
			"previous_verdict":    previousVerdict,
			"verdict":             verdict,
			"previous_verdict_id": previous["verdict_id"],
			"verdict_id":          verdictID,
			"resolved_blockers":   resolvedBlockers,
		}); err != nil {
			return nil, err
		}
		if err := maybeEnqueueDownstream(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := maybeCompleteRun(ctx, tx, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":               "overridden",
			"job_id":               jobID,
			"previous_verdict":     previousVerdict,
			"verdict":              verdict,
			"verdict_id":           verdictID,
			"findings_artifact_id": effectiveArtifactID,
			"resolved_blockers":    resolvedBlockers,
		}, nil
	})
}

func resolveOverrideSession(ctx context.Context, runner any, repositoryID, requestedSessionID, jobID string) (string, error) {
	hasOwnVerdict, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2 AND session_id = $3
		 LIMIT 1`, repositoryID, jobID, requestedSessionID)
	if err != nil {
		return "", err
	}
	if !hasOwnVerdict {
		return requestedSessionID, nil
	}
	requested, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", requestedSessionID, false)
	if err != nil {
		return requestedSessionID, nil
	}
	runID := fmt.Sprint(requested["run_id"])
	laneID := fmt.Sprint(requested["lane_id"])
	rows, err := queryRows(ctx, runner, `
		SELECT ordinal
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND run_id = $2
		   AND role_id = 'reviewer' AND lane_id = $3
		 FOR UPDATE`, repositoryID, runID, laneID)
	if err != nil {
		return "", err
	}
	ordinal := 1
	for _, row := range rows {
		if value := intValue(row["ordinal"]); value >= ordinal {
			ordinal = value + 1
		}
	}
	sessionID, err := newID("sess")
	if err != nil {
		return "", err
	}
	labelSuffix := jobID
	if len(labelSuffix) > 12 {
		labelSuffix = labelSuffix[len(labelSuffix)-12:]
	}
	operatorLabel := "operator-override-" + strings.ToLower(labelSuffix)
	slug := fmt.Sprintf("reviewer-%s-%d", laneID, ordinal)
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return "", fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, parent_session_id, fresh_context, state,
		  registered_at, last_heartbeat_at, non_fresh_reason, operator_label
		)
		VALUES ($1,$2,$3,'reviewer',$4,$5,$6,$7,NULL,true,'active',$8,$9,NULL,$10)`,
		repositoryID, sessionID, runID, laneID, slug, ordinal, []string{}, now, now, operatorLabel); err != nil {
		return "", err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, "session.registered", sessionID, nil, nil, nil, nil, map[string]any{
		"role":           "reviewer",
		"lane":           laneID,
		"slug":           slug,
		"operator_label": operatorLabel,
		"source":         "override_auto_fresh_session",
	}); err != nil {
		return "", err
	}
	return sessionID, nil
}

func recordVerdict(
	ctx context.Context,
	runner any,
	repositoryID string,
	sessionID string,
	jobID string,
	leaseID string,
	verdict string,
	findingsArtifactID any,
	rationale any,
) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(job["job_type"]) != "review" && fmt.Sprint(job["job_type"]) != "phase_synthesis" {
		return nil, rpc.NewError("invalid_transition", "verdict is valid only for verdict-capable jobs", nil)
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return nil, err
	}
	if fmt.Sprint(job["state"]) != "running" {
		label := "review"
		if fmt.Sprint(job["job_type"]) == "phase_synthesis" {
			label = "phase_synthesis"
		}
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("%s job must be running before verdict", label), nil)
	}
	if err := enforceRequiredAttestationForVerdict(ctx, runner, repositoryID, job, sessionID); err != nil {
		return nil, err
	}
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	if findingsArtifactID != nil {
		artifact, err := rowByID(ctx, runner, repositoryID, "artifacts", "artifact_id", fmt.Sprint(findingsArtifactID), false)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(artifact["run_id"]) != fmt.Sprint(job["run_id"]) || fmt.Sprint(artifact["job_id"]) != jobID {
			return nil, rpc.NewError("invalid_transition", "findings artifact belongs to a different job", nil)
		}
	}
	verdictID, err := newID("verdict")
	if err != nil {
		return nil, err
	}
	now := nowString()
	posture, err := resolveReviewPosture(ctx, runner, repositoryID, job)
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
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  rationale, findings_artifact_id, created_at, posture
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		repositoryID,
		verdictID,
		job["run_id"],
		jobID,
		sessionID,
		verdict,
		rationale,
		findingsArtifactID,
		now,
		posture,
	); err != nil {
		return nil, err
	}
	attestation := sessionLaneAttestation(ctx, runner, repositoryID, sessionID)
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "verdict.recorded", sessionID, jobID, nil, nil, leaseID, map[string]any{
		"verdict":          verdict,
		"posture":          posture,
		"lane_attestation": attestation["state"],
	}); err != nil {
		return nil, err
	}
	switch verdict {
	case "accept", "accept_with_findings":
		if err := completeReviewJob(ctx, runner, repositoryID, job, sessionID, leaseID, verdict); err != nil {
			return nil, err
		}
		if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
			return nil, err
		}
		return map[string]any{"status": "completed", "job_id": jobID, "verdict": verdict, "verdict_id": verdictID}, nil
	case "needs_revision":
		blockerID, err := openHumanCheckpoint(ctx, runner, repositoryID, job, sessionID, leaseID, "needs_revision verdict has no matching workflow cycle")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "waiting_human", "job_id": jobID, "verdict": verdict, "blocker_id": blockerID, "verdict_id": verdictID}, nil
	case "reject":
		if err := failReviewJob(ctx, runner, repositoryID, job, sessionID, leaseID); err != nil {
			return nil, err
		}
		if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
			return nil, err
		}
		return map[string]any{"status": "failed", "job_id": jobID, "verdict": verdict, "verdict_id": verdictID}, nil
	default:
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("unknown verdict %q", verdict), nil)
	}
}

func prevalidateSubmitReview(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID, leaseID, logicalName, kind, pathText string) error {
	if fmt.Sprint(job["job_type"]) != "review" && fmt.Sprint(job["job_type"]) != "phase_synthesis" {
		return rpc.NewError("invalid_transition", "submit-review is valid only for verdict-capable jobs", nil)
	}
	state := fmt.Sprint(job["state"])
	if state != "claimed" && state != "running" {
		return rpc.NewError("invalid_transition", "verdict-capable job must be claimed or running before submit-review", nil)
	}
	if state == "claimed" && nullable(job["current_message_id"]) == nil {
		return rpc.NewError("invalid_transition", "claimed verdict-capable job is missing its current message", nil)
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, fmt.Sprint(job["job_id"])); err != nil {
		return err
	}
	if err := enforceRequiredAttestationForVerdict(ctx, runner, repositoryID, job, sessionID); err != nil {
		return err
	}
	for _, item := range asList(job["expected_artifacts_json"]) {
		expected := asMap(item)
		if expected["required"] != true {
			continue
		}
		if expected["logical_name"] == logicalName && expected["kind"] == kind && expected["path"] == pathText {
			continue
		}
		found, err := existsRow(ctx, runner, `
			SELECT 1 FROM striatumd.artifacts
			 WHERE repository_id = $1 AND job_id = $2 AND logical_name = $3
			   AND artifact_kind = $4 AND repo_path = $5
			 LIMIT 1`,
			repositoryID,
			job["job_id"],
			expected["logical_name"],
			expected["kind"],
			expected["path"],
		)
		if err != nil {
			return err
		}
		if !found {
			return rpc.NewError("invalid_transition", fmt.Sprintf(
				"required artifact would still be missing after submit-review: logical_name=%q, kind=%q, path=%q",
				expected["logical_name"],
				expected["kind"],
				expected["path"],
			), nil)
		}
	}
	return nil
}

func ackInline(ctx context.Context, runner any, repositoryID, sessionID, messageID, leaseID string) error {
	message, err := rowByID(ctx, runner, repositoryID, "queue_messages", "message_id", messageID, true)
	if err != nil {
		return err
	}
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(message["job_id"]), true)
	if err != nil {
		return err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, fmt.Sprint(job["job_id"])); err != nil {
		return err
	}
	if fmt.Sprint(message["state"]) == "acked" {
		return nil
	}
	if fmt.Sprint(message["state"]) != "claimed" || fmt.Sprint(job["state"]) != "claimed" {
		return rpc.NewError("invalid_transition", "work must be claimed before ack", nil)
	}
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'acked', acked_at = $1, updated_at = $2
		 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
		return err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'running', started_at = $1
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, job["job_id"]); err != nil {
		return err
	}
	_, err = appendEvent(ctx, runner, repositoryID, job["run_id"], "queue.acked", sessionID, job["job_id"], messageID, nil, leaseID, nil)
	return err
}

func completeReviewJob(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID, leaseID, summary string) error {
	now := nowString()
	messageID := nullable(job["current_message_id"])
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, job["job_id"]); err != nil {
		return err
	}
	if messageID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $2,
			       current_lease_id = NULL
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'verdict'
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return err
	}
	_, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.completed", sessionID, job["job_id"], messageID, nil, leaseID, map[string]any{"summary": summary})
	return err
}

func failReviewJob(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID, leaseID string) error {
	now := nowString()
	messageID := nullable(job["current_message_id"])
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'failed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, job["job_id"]); err != nil {
		return err
	}
	if messageID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $2,
			       current_lease_id = NULL
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'reject'
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return err
	}
	_, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.failed", sessionID, job["job_id"], messageID, nil, leaseID, map[string]any{"reason": "reject"})
	return err
}

func openHumanCheckpoint(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID, leaseID, description string) (string, error) {
	blockerID, err := newID("blk")
	if err != nil {
		return "", err
	}
	now := nowString()
	messageID := nullable(job["current_message_id"])
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return "", fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at
		)
		VALUES ($1,$2,$3,$4,$5,'human_checkpoint','revision_routing',$6,'open',$7)`,
		repositoryID, blockerID, job["run_id"], job["job_id"], sessionID, description, now); err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'waiting_human', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, job["job_id"]); err != nil {
		return "", err
	}
	if messageID != nil {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'blocked', current_lease_id = NULL, updated_at = $1
			 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
			return "", err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'needs_revision'
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return "", err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "human_checkpoint.opened", sessionID, job["job_id"], nil, nil, leaseID, map[string]any{
		"blocker_id":  blockerID,
		"description": description,
	}); err != nil {
		return "", err
	}
	return blockerID, nil
}

func resolveReviewPosture(ctx context.Context, runner any, repositoryID string, job map[string]any) (string, error) {
	if fmt.Sprint(job["job_type"]) == "phase_synthesis" {
		return "neutral", nil
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return "", err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return "", err
	}
	for _, item := range asList(asMap(snapshot["workflow_json"])["jobs"]) {
		def := asMap(item)
		if def["id"] == job["workflow_job_id"] && def["type"] == "review" {
			if posture, ok := def["review_posture"].(string); ok && posture != "" {
				return posture, nil
			}
			return "neutral", nil
		}
	}
	return "neutral", nil
}

func enforceRequiredAttestationForVerdict(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string) error {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return err
	}
	for _, item := range asList(asMap(snapshot["workflow_json"])["jobs"]) {
		def := asMap(item)
		if def["id"] != job["workflow_job_id"] {
			continue
		}
		if def["require_attested_lane"] != true {
			return nil
		}
		attestation := sessionLaneAttestation(ctx, runner, repositoryID, sessionID)
		if attestation["attested"] == true {
			return nil
		}
		reason := ""
		if attestation["reason"] != nil {
			reason = fmt.Sprintf(" (%v)", attestation["reason"])
		}
		return rpc.NewError("invalid_transition", "review job requires an attached lane supervisor before recording a verdict"+reason+"; recovery: striatum supervise start --session-id "+sessionID, nil)
	}
	return nil
}

func downstreamJobs(ctx context.Context, runner any, repositoryID, jobID string) ([]map[string]any, error) {
	return queryRows(ctx, runner, `
		SELECT j.job_id, j.workflow_job_id, j.state
		  FROM striatumd.job_dependencies dep
		  JOIN striatumd.jobs j
		    ON j.repository_id = dep.repository_id
		   AND j.job_id = dep.job_id
		 WHERE dep.repository_id = $1 AND dep.depends_on_job_id = $2
		 ORDER BY j.created_at, j.job_id`, repositoryID, jobID)
}
