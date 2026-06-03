package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func HandleDecisionRecord(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	pathText := stringParam(envelope, "path")
	outcome := stringParam(envelope, "outcome")
	title := strings.TrimSpace(stringParam(envelope, "title"))
	decisionID := strings.TrimSpace(stringParam(envelope, "decision_id"))
	rationale := stringParam(envelope, "rationale")
	followUp := stringParam(envelope, "follow_up")
	if runID == "" || pathText == "" || outcome == "" || title == "" {
		return nil, rpc.NewError("schema_invalid", "decision.record requires run_id, path, outcome, and title", nil)
	}
	if outcome == "accepted_with_follow_up" && strings.TrimSpace(followUp) == "" {
		return nil, rpc.NewError("artifact_error", "accepted_with_follow_up decisions require --follow-up", nil)
	}
	if decisionID == "" {
		decisionID, err = newID("dec")
		if err != nil {
			return nil, err
		}
	}
	for _, char := range decisionID {
		if unicode.IsSpace(char) {
			return nil, rpc.NewError("artifact_error", "decision id cannot be empty or contain whitespace", nil)
		}
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		_, err = oneRow(ctx, tx, `
			SELECT artifact_id FROM striatumd.artifacts
			 WHERE repository_id = $1
			   AND run_id = $2
			   AND artifact_kind = 'decision'
			   AND (logical_name = $3 OR repo_path = $4)
			 LIMIT 1`, repositoryID, runID, decisionID, pathText)
		if err == nil {
			return nil, rpc.NewError("artifact_error", "decision artifact already exists for this run id/path", nil)
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}
		target, err := repoRelativePath(fmt.Sprint(run["repo_root"]), pathText, false)
		if err != nil {
			return nil, rpc.NewError("artifact_error", err.Error(), nil)
		}
		if _, err := os.Stat(target); err == nil {
			return nil, rpc.NewError("artifact_error", "decision artifact path already exists", nil)
		}
		createdAt := nowString()
		body := renderDecisionMarkdown(decisionID, runID, outcome, title, createdAt, rationale, followUp)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(body))
		digest := hex.EncodeToString(sum[:])
		artifactID, err := newID("art")
		if err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.artifacts (
			  repository_id, artifact_id, run_id, job_id, session_id,
			  logical_name, artifact_kind, repo_path, content_sha256,
			  size_bytes, publish_mode, created_at
			)
			VALUES ($1,$2,$3,NULL,NULL,$4,'decision',$5,$6,$7,'create',$8)`,
			repositoryID, artifactID, runID, decisionID, pathText, digest, len(body), createdAt); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "decision.recorded", nil, nil, nil, artifactID, nil, map[string]any{
			"decision_id": decisionID,
			"outcome":     outcome,
			"path":        pathText,
			"sha256":      digest,
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":                   "recorded",
			"run_id":                   runID,
			"decision_id":              decisionID,
			"artifact_id":              artifactID,
			"path":                     pathText,
			"outcome":                  outcome,
			"sha256":                   digest,
			"run_state_transition":     nil,
			"superseded_verdict_count": 0,
		}, nil
	})
}

func HandleCheckpointResolve(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	blockerID := stringParam(envelope, "blocker_id")
	action := stringParam(envelope, "action")
	decisionID := nullable(stringParam(envelope, "decision_id"))
	if blockerID == "" || action == "" {
		return nil, rpc.NewError("schema_invalid", "checkpoint.resolve requires blocker_id and action", nil)
	}
	if action != "continue" && action != "cancel" && action != "override" {
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("unknown checkpoint resolve action %q", action), nil)
	}
	if action == "override" && decisionID == nil {
		return nil, rpc.NewError("schema_invalid", "checkpoint.resolve override requires decision_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first — checkpoint.resolve can re-open a
		// job (reopenJobForAttempt) and complete the run (maybeCompleteRun), which
		// inverts against the claim path.
		if err := lockRunForBlocker(ctx, tx, repositoryID, blockerID); err != nil {
			return nil, err
		}
		blocker, err := rowByID(ctx, tx, repositoryID, "blockers", "blocker_id", blockerID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(blocker["state"]) != "open" {
			return nil, rpc.NewError("invalid_transition", "blocker is not open", nil)
		}
		if fmt.Sprint(blocker["severity"]) != "human_checkpoint" {
			return nil, rpc.NewError("invalid_transition", "checkpoint resolve only applies to human_checkpoint blockers", nil)
		}
		runID := fmt.Sprint(blocker["run_id"])
		blockerJobID := nullable(blocker["job_id"])
		var artifactID any
		var decisionOutcome string
		if decisionID != nil {
			artifact, err := oneRow(ctx, tx, `
				SELECT artifact_id, run_id, job_id, session_id, logical_name
				  FROM striatumd.artifacts
				 WHERE repository_id = $1
				   AND artifact_kind = 'decision'
				   AND run_id = $2
				   AND logical_name = $3
				 LIMIT 1`, repositoryID, runID, decisionID)
			if err != nil {
				return nil, err
			}
			if nullable(artifact["job_id"]) != nil || nullable(artifact["session_id"]) != nil {
				return nil, rpc.NewError("invalid_transition", "decision artifact must be run-level (no job or session binding)", nil)
			}
			artifactID = artifact["artifact_id"]
			// The decision's outcome lives in the decision.recorded event payload
			// (the artifacts table does not store it). Override requires an
			// accepting outcome; continue/cancel do not consult it.
			outcomeRow, err := oneRow(ctx, tx, `
				SELECT payload_json->>'outcome' AS outcome
				  FROM striatumd.events
				 WHERE repository_id = $1
				   AND run_id = $2
				   AND event_type = 'decision.recorded'
				   AND payload_json->>'decision_id' = $3
				 ORDER BY event_id DESC
				 LIMIT 1`, repositoryID, runID, decisionID)
			if err == nil {
				decisionOutcome = fmt.Sprint(outcomeRow["outcome"])
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
		now := nowString()
		downstream := []map[string]any{}
		var nextActions []string
		var payloadExtra map[string]any
		eventType := "checkpoint.resolved"
		switch action {
		case "continue":
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND blocker_id = $3`, now, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.escalation_inbox
				   SET state = 'resolved', resolved_at = $1, decision_artifact_id = $2
				 WHERE repository_id = $3 AND escalation_id = $4`, now, artifactID, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if blockerJobID != nil {
				job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(blockerJobID), true)
				if err != nil {
					return nil, err
				}
				if fmt.Sprint(job["state"]) != "waiting_human" {
					return nil, rpc.NewError("invalid_transition", fmt.Sprintf("checkpoint job is not in waiting_human (state=%q)", job["state"]), nil)
				}
				// RFC 0095 Phase 2 (#65 P3): re-open atomically through the shared
				// helper. The prior inline path moved the existing message back to
				// `pending` (or re-enqueued) but never released a still-active lease
				// and never bumped the attempt, so a re-claim could fail the
				// uq_active_resource_lease index and a stale prior-attempt artifact
				// could later auto-finalize the job (the #65 P2 wedge). The helper
				// releases the lease, retires the parked message, re-blocks
				// transitive downstream terminal jobs, clears stale verdicts, bumps
				// the attempt, and re-enqueues a fresh message.
				if _, err := reopenJobForAttempt(ctx, tx, repositoryID, job, "checkpoint_continue"); err != nil {
					return nil, err
				}
				downstream, err = downstreamJobs(ctx, tx, repositoryID, fmt.Sprint(blockerJobID))
				if err != nil {
					return nil, err
				}
			}
			nextActions = []string{"claim_available_work", "monitor_run_progress"}
		case "override":
			// F2 (issue #63): accept a needs_revision revision_routing checkpoint
			// as superseded by a recorded run-level decision and make the
			// downstream gate reachable WITHOUT re-queueing the same review.
			// No new override authority is created here; the audit/rationale live
			// entirely in the referenced decision artifact (same model as
			// D099/D100, RFC 0064 workflow.accept_risk).
			if fmt.Sprint(blocker["blocker_kind"]) != "revision_routing" {
				return nil, rpc.NewError("invalid_transition", "checkpoint override only applies to revision_routing checkpoints", nil)
			}
			if decisionOutcome != "accepted" && decisionOutcome != "accepted_with_follow_up" {
				return nil, rpc.NewError("invalid_transition", fmt.Sprintf("checkpoint override requires an accepting decision outcome (got %q)", decisionOutcome), nil)
			}
			if blockerJobID == nil {
				return nil, rpc.NewError("invalid_transition", "revision_routing checkpoint has no review job to clear", nil)
			}
			job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(blockerJobID), true)
			if err != nil {
				return nil, err
			}
			if fmt.Sprint(job["state"]) != "waiting_human" {
				return nil, rpc.NewError("invalid_transition", fmt.Sprintf("checkpoint job is not in waiting_human (state=%q)", job["state"]), nil)
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND blocker_id = $3`, now, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.escalation_inbox
				   SET state = 'resolved', resolved_at = $1, decision_artifact_id = $2
				 WHERE repository_id = $3 AND escalation_id = $4`, now, artifactID, repositoryID, blockerID); err != nil {
				return nil, err
			}
			// Complete the review job: it is settled by the operator decision,
			// not re-run. Match completeReviewJob's queue_messages state value.
			messageID := nullable(job["current_message_id"])
			if err := tx.Exec(ctx, `
				UPDATE striatumd.jobs
				   SET state = 'completed', completed_at = $1, current_lease_id = NULL,
				       current_message_id = NULL
				 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, blockerJobID); err != nil {
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
			// Record a superseding clearing verdict so latestVerdict() returns an
			// accepting value and the downstream gate (requires_verdict) is met.
			// The verdicts table requires a non-null session_id (FK + UNIQUE per
			// job/session); the original reviewer already holds the needs_revision
			// row, so we mint a fresh operator-labeled reviewer session for the
			// clearing row (same mechanism as review.override's
			// resolveOverrideSession). The stale needs_revision row is preserved
			// for audit; posture='override' marks the row as operator-cleared.
			// Seed the resolver with the original reviewer session (carried on the
			// blocker). That session already holds the needs_revision verdict, so
			// resolveOverrideSession mints a fresh operator-labeled reviewer session
			// in the same lane for the clearing row.
			reviewerSessionID := nullable(blocker["session_id"])
			if reviewerSessionID == nil {
				reviewerSessionID = nullable(job["current_session_id"])
			}
			overrideSessionID, err := resolveOverrideSession(ctx, tx, repositoryID, fmt.Sprint(reviewerSessionID), fmt.Sprint(blockerJobID))
			if err != nil {
				return nil, err
			}
			verdictID, err := newID("verdict")
			if err != nil {
				return nil, err
			}
			// nowString() truncates to whole seconds, so the clearing verdict can
			// share a created_at with the stale needs_revision row recorded the same
			// second; latestVerdict() then breaks the tie on verdict_id, which is
			// random. Stamp the clearing row strictly after the newest existing
			// verdict for the job so latestVerdict() deterministically returns the
			// clear. Also flag the superseded needs_revision rows for audit.
			if err := tx.Exec(ctx, `
				UPDATE striatumd.verdicts
				   SET superseded_by_decision_id = $1, superseded_at = $2
				 WHERE repository_id = $3 AND job_id = $4
				   AND verdict = 'needs_revision'
				   AND superseded_by_decision_id IS NULL`, decisionID, now, repositoryID, blockerJobID); err != nil {
				return nil, err
			}
			overrideCreatedAt := now
			maxRow, err := oneRow(ctx, tx, `
				SELECT max(created_at) AS latest FROM striatumd.verdicts
				 WHERE repository_id = $1 AND job_id = $2`, repositoryID, blockerJobID)
			if err == nil && maxRow["latest"] != nil {
				if latest, ok := asTime(maxRow["latest"]); ok {
					// Strictly after every existing verdict for the job.
					overrideCreatedAt = latest.Add(time.Second).UTC().Format(time.RFC3339)
				}
			} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			overrideRationale := fmt.Sprintf("operator override of revision_routing checkpoint %s, superseded by decision %v", blockerID, decisionID)
			if err := tx.Exec(ctx, `
				INSERT INTO striatumd.verdicts (
				  repository_id, verdict_id, run_id, job_id, session_id, verdict,
				  rationale, findings_artifact_id, created_at, posture
				)
				VALUES ($1,$2,$3,$4,$5,'accept_with_findings',$6,$7,$8,'override')`,
				repositoryID, verdictID, job["run_id"], blockerJobID, overrideSessionID,
				overrideRationale, artifactID, overrideCreatedAt); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "verdict.recorded", overrideSessionID, blockerJobID, nil, artifactID, nil, map[string]any{
				"verdict":     "accept_with_findings",
				"posture":     "override",
				"source":      "checkpoint.override",
				"decision_id": decisionID,
			}); err != nil {
				return nil, err
			}
			if err := maybeEnqueueDownstream(ctx, tx, repositoryID, fmt.Sprint(blockerJobID)); err != nil {
				return nil, err
			}
			downstream, err = downstreamJobs(ctx, tx, repositoryID, fmt.Sprint(blockerJobID))
			if err != nil {
				return nil, err
			}
			if err := maybeCompleteRun(ctx, tx, repositoryID, runID); err != nil {
				return nil, err
			}
			payloadExtra = map[string]any{"superseded_verdict": "needs_revision"}
			nextActions = []string{"claim_available_work", "monitor_run_progress"}
		default: // cancel
			eventType = "checkpoint.canceled"
			if err := tx.Exec(ctx, `
				UPDATE striatumd.blockers
				   SET state = 'resolved', resolved_at = $1
				 WHERE repository_id = $2 AND blocker_id = $3`, now, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.escalation_inbox
				   SET state = 'resolved', resolved_at = $1, decision_artifact_id = $2
				 WHERE repository_id = $3 AND escalation_id = $4`, now, artifactID, repositoryID, blockerID); err != nil {
				return nil, err
			}
			if blockerJobID != nil {
				job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(blockerJobID), true)
				if err != nil {
					return nil, err
				}
				if fmt.Sprint(job["state"]) != "waiting_human" {
					return nil, rpc.NewError("invalid_transition", fmt.Sprintf("checkpoint job is not in waiting_human (state=%q)", job["state"]), nil)
				}
				messageID := nullable(job["current_message_id"])
				if err := tx.Exec(ctx, `
					UPDATE striatumd.jobs
					   SET state = 'canceled', current_lease_id = NULL,
					       current_message_id = NULL, completed_at = $1
					 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, blockerJobID); err != nil {
					return nil, err
				}
				if messageID != nil {
					if err := tx.Exec(ctx, `
						UPDATE striatumd.queue_messages
						   SET state = 'canceled', current_lease_id = NULL, updated_at = $1
						 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
						return nil, err
					}
				}
				downstream, err = downstreamJobs(ctx, tx, repositoryID, fmt.Sprint(blockerJobID))
				if err != nil {
					return nil, err
				}
			}
			if err := maybeCompleteRun(ctx, tx, repositoryID, runID); err != nil {
				return nil, err
			}
			nextActions = []string{"inspect_run_state", "export_run_evidence"}
		}
		payload := map[string]any{"blocker_id": blockerID, "action": action}
		if decisionID != nil {
			payload["decision_id"] = decisionID
		}
		if artifactID != nil {
			payload["decision_artifact_id"] = artifactID
		}
		for k, v := range payloadExtra {
			payload[k] = v
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, eventType, nil, blockerJobID, nil, artifactID, nil, payload); err != nil {
			return nil, err
		}
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":               "resolved",
			"blocker_id":           blockerID,
			"job_id":               blockerJobID,
			"action":               action,
			"decision_id":          decisionID,
			"decision_artifact_id": artifactID,
			"run_state":            run["state"],
			"downstream_jobs":      downstream,
			"next_actions":         nextActions,
		}, nil
	})
}

func renderDecisionMarkdown(decisionID, runID, outcome, title, createdAt, rationale, followUp string) string {
	followUpRequired := outcome == "accepted_with_follow_up"
	decisionJSON, _ := json.Marshal(decisionID)
	runJSON, _ := json.Marshal(runID)
	titleJSON, _ := json.Marshal(title)
	createdJSON, _ := json.Marshal(createdAt)
	lines := []string{
		"---",
		"schema_version: striatum.decision.v1",
		"decision_id: " + string(decisionJSON),
		"run_id: " + string(runJSON),
		"artifact_kind: decision",
		"owner: human",
		"outcome: " + outcome,
		fmt.Sprintf("follow_up_required: %v", followUpRequired),
		"title: " + string(titleJSON),
		"created_at: " + string(createdJSON),
		"---",
		"",
		"# " + title,
		"",
		"Decision ID: `" + decisionID + "`",
		"Run ID: `" + runID + "`",
		"Outcome: `" + outcome + "`",
		"",
	}
	if strings.TrimSpace(rationale) != "" {
		lines = append(lines, "## Rationale", "", strings.TrimSpace(rationale), "")
	}
	if strings.TrimSpace(followUp) != "" {
		lines = append(lines, "## Follow-Up", "", strings.TrimSpace(followUp), "")
	}
	return strings.Join(lines, "\n")
}
