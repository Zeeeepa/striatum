package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if action != "continue" && action != "cancel" {
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("unknown checkpoint resolve action %q", action), nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
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
		}
		now := nowString()
		downstream := []map[string]any{}
		nextActions := []string{}
		eventType := "checkpoint.resolved"
		if action == "continue" {
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
				if messageID != nil {
					if err := tx.Exec(ctx, `
						UPDATE striatumd.queue_messages
						   SET state = 'pending', current_lease_id = NULL, updated_at = $1
						 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
						return nil, err
					}
					if err := tx.Exec(ctx, `
						UPDATE striatumd.jobs
						   SET state = 'queued', current_lease_id = NULL, ready_at = $1
						 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, blockerJobID); err != nil {
						return nil, err
					}
				} else {
					if err := tx.Exec(ctx, `
						UPDATE striatumd.jobs
						   SET state = 'blocked', current_lease_id = NULL
						 WHERE repository_id = $1 AND job_id = $2`, repositoryID, blockerJobID); err != nil {
						return nil, err
					}
					if _, err := enqueueJob(ctx, tx, repositoryID, fmt.Sprint(blockerJobID)); err != nil {
						return nil, err
					}
				}
				downstream, err = downstreamJobs(ctx, tx, repositoryID, fmt.Sprint(blockerJobID))
				if err != nil {
					return nil, err
				}
			}
			nextActions = []string{"claim_available_work", "monitor_run_progress"}
		} else {
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
