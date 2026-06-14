package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

// RFC 0118 P1-5 (GH #240): the durable run_completion_record. Snapshot the
// run's last-live state INSIDE the terminal transaction, BEFORE
// closeRemainingSessions transitions supervisor rows and before read-side
// terminal filters would hide them — the record answers "what was alive,
// attested, leased, and override-cleared when this run ended?" without any
// live probe or daemon archaeology. The record is written exactly once
// (operator decision #3) and its sha256 is anchored in the append-only
// terminal event payload.

// buildRunCompletionRecord assembles the record. extra keys (e.g. the P0-3
// provenance ledger and completion_mode on the completed path) are merged
// into the top level. Active sessions are probed while their lanes are still
// live; closingReason records the imminent close reason the terminal
// transition is about to apply to them.
func buildRunCompletionRecord(ctx context.Context, runner any, repositoryID, runID, terminalState, closingReason string, extra map[string]any) (map[string]any, error) {
	record := map[string]any{
		"schema_version": "striatumd.run_completion_record.v1",
		"run_id":         runID,
		"terminal_state": terminalState,
		"recorded_at":    nowString(),
	}
	for key, value := range extra {
		record[key] = value
	}

	jobs, err := queryRows(ctx, runner, `
		SELECT workflow_job_id, job_id, job_type, state, attempt
		  FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY created_at, job_id`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	record["jobs"] = jobs

	verdicts, err := queryRows(ctx, runner, `
		SELECT job_id, verdict_id, verdict, posture,
		       lane_attestation_at_record, review_provenance_override,
		       review_provenance_decision_id, supervisor_id_at_record,
		       created_at
		  FROM striatumd.verdicts
		 WHERE repository_id = $1 AND run_id = $2
		   AND superseded_by_decision_id IS NULL
		 ORDER BY created_at, verdict_id`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	record["verdicts"] = verdicts

	sessionRows, err := queryRows(ctx, runner, `
		SELECT session_id, role_id, lane_id, state, close_reason,
		       registered_at, closed_at, last_pty_activity_at
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY registered_at, session_id`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	sessions := make([]map[string]any, 0, len(sessionRows))
	for _, row := range sessionRows {
		sessionID := fmt.Sprint(row["session_id"])
		entry := map[string]any{
			"session_id":        sessionID,
			"role_id":           row["role_id"],
			"lane_id":           row["lane_id"],
			"state":             row["state"],
			"registered_at":     row["registered_at"],
			"closed_at":         nullable(row["closed_at"]),
			"pty_activity_seen": nullable(row["last_pty_activity_at"]) != nil,
		}
		if fmt.Sprint(row["state"]) == "active" {
			// Probed while the lane is still live — this is the last-live
			// liveness/attestation class the retrospective needs.
			entry["attestation"] = sessionLaneAttestation(ctx, runner, repositoryID, sessionID)
			entry["close_reason"] = closingReason
		} else {
			entry["close_reason"] = nullable(row["close_reason"])
		}
		sessions = append(sessions, entry)
	}
	record["sessions"] = sessions

	supervisors, err := queryRows(ctx, runner, `
		SELECT supervisor_id, session_id, state, pid
		  FROM striatumd.process_supervisors
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY supervisor_id`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	record["supervisors"] = supervisors

	leases, err := queryRows(ctx, runner, `
		SELECT lease_id, owner_session_id, resource_id, state, expires_at,
		       (state = 'active' AND expires_at < now()) AS expired_while_active
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY acquired_at, lease_id`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	record["leases"] = leases

	trajectoryRow, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.trajectory_segments
		 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		record["trajectory_segment_count"] = 0
	case err != nil:
		return nil, err
	default:
		record["trajectory_segment_count"] = intValue(trajectoryRow["n"])
	}

	recoveryEvents, err := queryRows(ctx, runner, `
		SELECT event_type, count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2
		   AND (event_type LIKE 'recovery.%'
		     OR event_type IN ('job.auto_finalized','artifact.auto_finalized','work.claim_overridden'))
		 GROUP BY event_type
		 ORDER BY event_type`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	record["recovery_events"] = recoveryEvents

	return record, nil
}

// persistRunCompletionRecord writes the record (write-once) and returns the
// sha256 over its canonical JSON for anchoring in the terminal event. When a
// record already exists (decision #3 overwrite guard) it returns "", false —
// the terminal event then omits the anchor rather than anchoring a hash that
// does not match the stored record.
func persistRunCompletionRecord(ctx context.Context, runner any, repositoryID, runID string, record map[string]any) (string, bool, error) {
	recordArg, err := db.JSONBArg(runner, record)
	if err != nil {
		return "", false, err
	}
	rows, err := queryRows(ctx, runner, `
		UPDATE striatumd.runs
		   SET completion_record_json = $1::jsonb
		 WHERE repository_id = $2 AND run_id = $3
		   AND completion_record_json IS NULL
		RETURNING run_id`, recordArg, repositoryID, runID)
	if err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	// Hash what a reader will get back from PostgreSQL (jsonb round-trip), not
	// the Go map's own serialization: encoding/json sorts map keys, and the
	// stored jsonb re-marshalled through the same path is what a retrospective
	// re-hashes against the anchored event.
	stored, err := oneRow(ctx, runner, `
		SELECT completion_record_json FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID)
	if err != nil {
		return "", false, err
	}
	canonical, err := json.Marshal(asMap(stored["completion_record_json"]))
	if err != nil {
		return "", false, err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), true, nil
}

// freezeRunCompletionRecord builds, persists, and (when newly written) folds
// the sha256 anchor into the terminal event payload. Call it INSIDE the
// terminal transaction, before the runs.state UPDATE and before
// closeRemainingSessions.
func freezeRunCompletionRecord(ctx context.Context, runner any, repositoryID, runID, terminalState, closingReason string, extra map[string]any, eventPayload map[string]any) (map[string]any, error) {
	// RFC 0122 §6: a terminal run can never be auto-spawned again. Revoke its
	// spawn-authorization grant at the single finalization chokepoint so every
	// terminal path (complete/fail/cancel/compromise) drops the grant — defense in
	// depth beside the scheduler's own terminal-state predicate.
	if err := revokeSpawnAuthorizationGrant(ctx, runner, repositoryID, runID, "run_"+terminalState); err != nil {
		return nil, err
	}
	record, err := buildRunCompletionRecord(ctx, runner, repositoryID, runID, terminalState, closingReason, extra)
	if err != nil {
		return nil, err
	}
	digest, written, err := persistRunCompletionRecord(ctx, runner, repositoryID, runID, record)
	if err != nil {
		return nil, err
	}
	if !written {
		return eventPayload, nil
	}
	if eventPayload == nil {
		eventPayload = map[string]any{}
	}
	eventPayload["completion_record_sha256"] = digest
	return eventPayload, nil
}
