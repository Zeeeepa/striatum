package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func HandleRegisterSession(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	role := stringParam(envelope, "role")
	lane := stringParam(envelope, "lane")
	if runID == "" || role == "" || lane == "" {
		return nil, rpc.NewError("schema_invalid", "session.register requires run_id, role, and lane", nil)
	}
	capabilities := stringSliceParam(envelope, "capabilities", "capability")
	fresh := boolParam(envelope, "fresh")
	parentSessionID := nullable(stringParam(envelope, "parent_session_id"))
	forceNonFresh := boolParam(envelope, "force_non_fresh")
	nonFreshReason := stringParam(envelope, "non_fresh_reason")
	if nonFreshReason == "" {
		nonFreshReason = stringParam(envelope, "reason")
	}
	operatorLabel := stringParam(envelope, "operator_label")

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		snapshot, err := rowByID(ctx, tx, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
		if err != nil {
			return nil, err
		}
		workflow := asMap(snapshot["workflow_json"])
		roles := asMap(workflow["roles"])
		if _, ok := roles[role]; !ok {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("unknown role %q for run", role), nil)
		}
		lanes := asMap(workflow["lanes"])
		if _, ok := lanes[lane]; !ok {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("unknown lane %q for run", lane), nil)
		}
		var recordedLabel any
		if operatorLabel != "" {
			cleaned, err := validateOperatorLabel(operatorLabel, workflow)
			if err != nil {
				return nil, err
			}
			recordedLabel = cleaned
		}
		var recordedReason any
		if role == "reviewer" && workflowDeclaresFreshReviewer(workflow) {
			_, err := oneRow(ctx, tx, `
				SELECT 1 FROM striatumd.sessions
				 WHERE repository_id = $1 AND run_id = $2
				   AND role_id = 'author' AND state = 'active'
				 LIMIT 1`, repositoryID, runID)
			if err == nil {
				if !forceNonFresh {
					return nil, rpc.NewError("invalid_transition", "workflow declares reviewer_context_policy: fresh and an active author session already exists on this run; pass --force-non-fresh --reason \"...\" to register a non-fresh reviewer explicitly", nil)
				}
				if strings.TrimSpace(nonFreshReason) == "" {
					return nil, rpc.NewError("invalid_transition", "--force-non-fresh requires a non-empty --reason", nil)
				}
				recordedReason = strings.TrimSpace(nonFreshReason)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
		rows, err := queryRows(ctx, tx, `
			SELECT ordinal
			  FROM striatumd.sessions
			 WHERE repository_id = $1 AND run_id = $2
			   AND role_id = $3 AND lane_id = $4
			 FOR UPDATE`, repositoryID, runID, role, lane)
		if err != nil {
			return nil, err
		}
		ordinal := 1
		for _, row := range rows {
			switch value := row["ordinal"].(type) {
			case int:
				if value >= ordinal {
					ordinal = value + 1
				}
			case int32:
				if int(value) >= ordinal {
					ordinal = int(value) + 1
				}
			case int64:
				if int(value) >= ordinal {
					ordinal = int(value) + 1
				}
			}
		}
		sessionID, err := newID("sess")
		if err != nil {
			return nil, err
		}
		slug := fmt.Sprintf("%s-%s-%d", role, lane, ordinal)
		now := nowString()
		capabilitiesArg, err := db.JSONBArg(tx, capabilities)
		if err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.sessions (
			  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
			  capabilities_json, parent_session_id, fresh_context, state,
			  registered_at, last_heartbeat_at, non_fresh_reason, operator_label
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,'active',$11,$12,$13,$14)`,
			repositoryID,
			sessionID,
			runID,
			role,
			lane,
			slug,
			ordinal,
			capabilitiesArg,
			parentSessionID,
			fresh,
			now,
			now,
			recordedReason,
			recordedLabel,
		); err != nil {
			return nil, err
		}
		payload := map[string]any{"role": role, "lane": lane, "slug": slug}
		if recordedReason != nil {
			payload["non_fresh_reason"] = recordedReason
		}
		if recordedLabel != nil {
			payload["operator_label"] = recordedLabel
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "session.registered", sessionID, nil, nil, nil, nil, payload); err != nil {
			return nil, err
		}
		attestation := sessionLaneAttestation(ctx, tx, repositoryID, sessionID)
		return map[string]any{
			"session_id":              sessionID,
			"slug":                    slug,
			"lane_attestation":        attestation["state"],
			"lane_attestation_reason": attestation["reason"],
			"operator_label":          recordedLabel,
			"supervise_hint":          "striatum supervise start --session-id " + sessionID,
		}, nil
	})
}

func HandleCloseSession(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	reason := strings.TrimSpace(stringParam(envelope, "reason"))
	if sessionID == "" {
		return nil, rpc.NewError("schema_invalid", "session.close requires session_id", nil)
	}
	if reason == "" {
		return nil, rpc.NewError("invalid_transition", "session close reason must not be empty", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		session, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, true)
		if err != nil {
			return nil, err
		}
		state := fmt.Sprint(session["state"])
		if state == "closed" || state == "expired" || state == "stopped" || state == "lost" {
			return map[string]any{
				"session_id":   sessionID,
				"run_id":       session["run_id"],
				"role_id":      session["role_id"],
				"lane_id":      session["lane_id"],
				"state":        state,
				"closed_at":    session["closed_at"],
				"close_reason": session["close_reason"],
				"note":         "session was already " + state,
			}, nil
		}
		active, err := oneRow(ctx, tx, `
			SELECT lease_id FROM striatumd.leases
			 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
			 LIMIT 1`, repositoryID, sessionID)
		if err == nil {
			return nil, rpc.NewError("lease_error", fmt.Sprintf("session has an active lease (%s); release the lease (striatum release) before closing the session", active["lease_id"]), nil)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.sessions
			   SET state = 'closed', closed_at = $1, close_reason = $2
			 WHERE repository_id = $3 AND session_id = $4`, now, reason, repositoryID, sessionID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, session["run_id"], "session.closed", sessionID, nil, nil, nil, nil, map[string]any{
			"session_id": sessionID,
			"role_id":    session["role_id"],
			"lane_id":    session["lane_id"],
			"reason":     reason,
			"source":     "explicit",
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id":   sessionID,
			"run_id":       session["run_id"],
			"role_id":      session["role_id"],
			"lane_id":      session["lane_id"],
			"state":        "closed",
			"closed_at":    now,
			"close_reason": reason,
		}, nil
	})
}

func HandleAckWork(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	messageID := stringParam(envelope, "message_id")
	leaseID := stringParam(envelope, "lease_id")
	if sessionID == "" || messageID == "" || leaseID == "" {
		return nil, rpc.NewError("schema_invalid", "work.ack requires session_id, message_id, and lease_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		message, err := rowByID(ctx, tx, repositoryID, "queue_messages", "message_id", messageID, true)
		if err != nil {
			return nil, err
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(message["job_id"]), true)
		if err != nil {
			return nil, err
		}
		if _, err := activeLeaseFor(ctx, tx, repositoryID, leaseID, sessionID, fmt.Sprint(job["job_id"])); err != nil {
			return nil, err
		}
		if fmt.Sprint(message["state"]) == "acked" {
			return map[string]any{"status": "acked", "job_id": job["job_id"]}, nil
		}
		if fmt.Sprint(message["state"]) != "claimed" || fmt.Sprint(job["state"]) != "claimed" {
			return nil, rpc.NewError("invalid_transition", "work must be claimed before ack", nil)
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'acked', acked_at = $1, updated_at = $2
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'running', started_at = $1
			 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, job["job_id"]); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, job["run_id"], "queue.acked", sessionID, job["job_id"], messageID, nil, leaseID, nil); err != nil {
			return nil, err
		}
		return map[string]any{"status": "acked", "job_id": job["job_id"]}, nil
	})
}

func HandleHeartbeat(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	leaseID := stringParam(envelope, "lease_id")
	extendSeconds := intParam(envelope, "extend_seconds", 1800)
	if sessionID == "" || leaseID == "" {
		return nil, rpc.NewError("schema_invalid", "work.heartbeat requires session_id and lease_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		lease, err := activeLeaseFor(ctx, tx, repositoryID, leaseID, sessionID, "")
		if err != nil {
			return nil, err
		}
		now := nowString()
		expiresAt := expiresAfter(extendSeconds)
		if err := tx.Exec(ctx, `
			UPDATE striatumd.sessions
			   SET last_heartbeat_at = $1
			 WHERE repository_id = $2 AND session_id = $3`, now, repositoryID, sessionID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET last_heartbeat_at = $1, expires_at = $2
			 WHERE repository_id = $3 AND lease_id = $4`, now, expiresAt, repositoryID, leaseID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, lease["run_id"], "lease.heartbeat", sessionID, lease["resource_id"], nil, nil, leaseID, map[string]any{"expires_at": expiresAt}); err != nil {
			return nil, err
		}
		return map[string]any{"status": "heartbeat", "expires_at": expiresAt}, nil
	})
}

func HandleReleaseWork(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	messageID := stringParam(envelope, "message_id")
	leaseID := stringParam(envelope, "lease_id")
	reason := stringParam(envelope, "reason")
	if reason == "" {
		reason = "released"
	}
	requeue := boolParam(envelope, "requeue")
	if sessionID == "" || leaseID == "" {
		return nil, rpc.NewError("schema_invalid", "work.release requires session_id and lease_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		var message map[string]any
		var err error
		if messageID != "" {
			message, err = rowByID(ctx, tx, repositoryID, "queue_messages", "message_id", messageID, true)
			if err != nil {
				return nil, err
			}
		} else {
			message, err = oneRow(ctx, tx, `
				SELECT *
				  FROM striatumd.queue_messages
				 WHERE repository_id = $1 AND current_lease_id = $2
				 LIMIT 1
				 FOR UPDATE`, repositoryID, leaseID)
			if err != nil {
				return nil, err
			}
			messageID = fmt.Sprint(message["message_id"])
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(message["job_id"]), true)
		if err != nil {
			return nil, err
		}
		if _, err := activeLeaseFor(ctx, tx, repositoryID, leaseID, sessionID, fmt.Sprint(job["job_id"])); err != nil {
			return nil, err
		}
		if requeue && isRepoWrite(job) {
			return nil, rpc.NewError("invalid_transition", "release --requeue is not supported for repo_write jobs; use striatum recovery requeue-stale after operator inspection", nil)
		}
		now := nowString()
		jobState := "blocked"
		messageState := "blocked"
		if requeue && !isRepoWrite(job) {
			jobState = "queued"
			messageState = "pending"
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'released', released_at = $1, release_reason = $2
			 WHERE repository_id = $3 AND lease_id = $4`, now, reason, repositoryID, leaseID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = $1, current_lease_id = NULL
			 WHERE repository_id = $2 AND job_id = $3`, jobState, repositoryID, job["job_id"]); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = $1, current_lease_id = NULL, updated_at = $2
			 WHERE repository_id = $3 AND message_id = $4`, messageState, now, repositoryID, messageID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, job["run_id"], "lease.released", sessionID, job["job_id"], messageID, nil, leaseID, map[string]any{"reason": reason, "job_state": jobState}); err != nil {
			return nil, err
		}
		return map[string]any{"status": "released", "job_state": jobState}, nil
	})
}

func HandleBlockWork(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	leaseID := stringParam(envelope, "lease_id")
	kind := stringParam(envelope, "kind")
	severity := stringParam(envelope, "severity")
	description := stringParam(envelope, "description")
	if sessionID == "" || jobID == "" || leaseID == "" || kind == "" || severity == "" || description == "" {
		return nil, rpc.NewError("schema_invalid", "work.block requires session_id, job_id, lease_id, kind, severity, and description", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if _, err := activeLeaseFor(ctx, tx, repositoryID, leaseID, sessionID, jobID); err != nil {
			return nil, err
		}
		blockerID, err := newID("blk")
		if err != nil {
			return nil, err
		}
		now := nowString()
		state := "blocked"
		if severity == "human_checkpoint" {
			state = "waiting_human"
		}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.blockers (
			  repository_id, blocker_id, run_id, job_id, session_id, severity,
			  blocker_kind, description, state, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'open',$9)`,
			repositoryID, blockerID, job["run_id"], jobID, sessionID, severity, kind, description, now,
		); err != nil {
			return nil, err
		}
		if isEscalation(severity, kind) {
			if err := tx.Exec(ctx, `
				INSERT INTO striatumd.escalation_inbox (
				  repository_id, escalation_id, run_id, job_id, session_id,
				  blocker_id, blocker_kind, severity, state, created_at
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',$9)`,
				repositoryID, blockerID, job["run_id"], jobID, sessionID, blockerID, kind, severity, now,
			); err != nil {
				return nil, err
			}
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = $1, current_lease_id = NULL
			 WHERE repository_id = $2 AND job_id = $3`, state, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'released', released_at = $1, release_reason = 'blocked'
			 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
			return nil, err
		}
		messageID := nullable(job["current_message_id"])
		if messageID != nil {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.queue_messages
				   SET state = 'blocked', current_lease_id = NULL
				 WHERE repository_id = $1 AND message_id = $2`, repositoryID, messageID); err != nil {
				return nil, err
			}
		}
		if _, err := appendEvent(ctx, tx, repositoryID, job["run_id"], "job.blocked", sessionID, jobID, nil, nil, leaseID, map[string]any{
			"blocker_id": blockerID,
			"severity":   severity,
		}); err != nil {
			return nil, err
		}
		return map[string]any{"status": "blocked", "blocker_id": blockerID}, nil
	})
}

func HandleCompleteWork(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	leaseID := stringParam(envelope, "lease_id")
	summary := stringParam(envelope, "summary")
	if sessionID == "" || jobID == "" || leaseID == "" {
		return nil, rpc.NewError("schema_invalid", "work.complete requires session_id, job_id, and lease_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if _, err := activeLeaseFor(ctx, tx, repositoryID, leaseID, sessionID, jobID); err != nil {
			return nil, err
		}
		if fmt.Sprint(job["state"]) != "running" {
			return nil, rpc.NewError("invalid_transition", "job must be running before completion", nil)
		}
		if err := verifyRequiredArtifacts(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		now := nowString()
		messageID := nullable(job["current_message_id"])
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'completed', completed_at = $1, current_lease_id = NULL
			 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
			return nil, err
		}
		if messageID != nil {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.queue_messages
				   SET state = 'completed', completed_at = $1, updated_at = $2,
				       current_lease_id = NULL
				 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
				return nil, err
			}
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'released', released_at = $1, release_reason = 'completed'
			 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, job["run_id"], "job.completed", sessionID, jobID, messageID, nil, leaseID, map[string]any{"summary": summary}); err != nil {
			return nil, err
		}
		if err := maybeEnqueueDownstream(ctx, tx, repositoryID, jobID); err != nil {
			return nil, err
		}
		if err := maybeCompleteRun(ctx, tx, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
			return nil, err
		}
		return map[string]any{"status": "completed", "job_id": jobID}, nil
	})
}

func isEscalation(severity string, kind string) bool {
	if severity == "human_checkpoint" {
		return true
	}
	switch kind {
	case "ambiguous_goal", "missing_authority", "contradicting_decisions",
		"no_available_reviewer_lane", "committee_stalemate", "override_required",
		"ai_self_declared":
		return true
	}
	return false
}
