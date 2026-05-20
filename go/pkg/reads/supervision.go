package reads

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const defaultSupervisorStallAfterSeconds = 300

var supervisorTerminalStates = map[string]bool{
	"lost":    true,
	"stopped": true,
}

var supervisorActiveStatesRead = map[string]bool{
	"starting": true,
	"attached": true,
	"detached": true,
}

// HandleSuperviseStatus mirrors the read projection of the Python
// supervise.status handler. It deliberately does not drain helper files,
// reattach rows, or mark missing PIDs lost; those are mutation surfaces.
func HandleSuperviseStatus(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredTextParam(envelope, "session_id", "supervise.status requires session_id")
	if err != nil {
		return nil, err
	}
	exists, err := rowExists(ctx, runner,
		`SELECT session_id FROM striatumd.sessions
		  WHERE repository_id = $1 AND session_id = $2
		  LIMIT 1`,
		repositoryID, sessionID,
	)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, rpc.NewError("not_found", "session not found: "+sessionID, nil)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT *
		   FROM striatumd.process_supervisors
		  WHERE repository_id = $1 AND session_id = $2
		  ORDER BY started_at DESC, supervisor_id DESC
		  LIMIT 1`,
		repositoryID, sessionID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("not_found", fmt.Sprintf("no supervisor recorded for session_id=%q", sessionID), nil)
	}

	supervisor := supervisorView(rows[0])
	state := superviseString(supervisor["state"])
	pid, hasPID := intValueOptional(supervisor["pid"])
	liveness := "gone"
	var progress map[string]any
	if hasPID && supervisorActiveStatesRead[state] {
		if pidAlive(pid) {
			liveness = "alive"
			progress, err = supervisorProgressForSession(ctx, runner, repositoryID, sessionID, defaultSupervisorStallAfterSeconds)
			if err != nil {
				return nil, err
			}
			if progress != nil && boolValue(progress["stalled"]) {
				liveness = "stalled"
			}
		}
	} else if hasPID && state == "stopped" {
		if pidAlive(pid) {
			liveness = "alive"
		}
	}
	supervisor["liveness"] = liveness
	if progress != nil {
		supervisor["active_lease_id"] = progress["lease_id"]
		supervisor["active_lease_expires_at"] = progress["lease_expires_at"]
		supervisor["active_lease_last_heartbeat_at"] = progress["lease_last_heartbeat_at"]
		supervisor["last_progress_at"] = progress["last_progress_at"]
		supervisor["last_progress_age_seconds"] = progress["last_progress_age_seconds"]
		supervisor["stall_after_seconds"] = progress["stall_after_seconds"]
		supervisor["lease_expired"] = progress["lease_expired"]
	}

	reattachRows, err := reattachStatusRows(ctx, runner, repositoryID, "", "", superviseString(supervisor["supervisor_id"]))
	if err != nil {
		return nil, err
	}
	reattachState := ""
	reattachReason := ""
	if len(reattachRows) > 0 {
		reattachView := reattachStatusView(reattachRows[0])
		reattachState = superviseString(reattachView["reattach_state"])
		reattachReason = superviseString(reattachView["reattach_reason"])
	}
	reattachAttested := reattachState == "" || reattachState == "reattachable"
	if state == "attached" && liveness == "alive" && reattachAttested {
		supervisor["lane_attestation"] = "attested"
		supervisor["lane_attestation_reason"] = nil
		return supervisor, nil
	}
	supervisor["lane_attestation"] = "unattested"
	switch {
	case liveness == "stalled":
		supervisor["lane_attestation_reason"] = "supervisor_stalled"
	case reattachState == "needs_repair" || reattachState == "needs_verification":
		supervisor["lane_attestation_reason"] = nullableText(reattachReason)
	default:
		supervisor["lane_attestation_reason"] = "no_live_attached_supervisor"
	}
	return supervisor, nil
}

// HandleSuperviseList returns process supervisor rows for a run, optionally
// filtered by state. It is a PostgreSQL-only read projection.
func HandleSuperviseList(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID, err := requiredTextParam(envelope, "run_id", "supervise.list requires run_id")
	if err != nil {
		return nil, err
	}
	state, err := optionalTextParam(envelope, "state")
	if err != nil {
		return nil, err
	}
	exists, err := rowExists(ctx, runner,
		`SELECT run_id FROM striatumd.runs
		  WHERE repository_id = $1 AND run_id = $2
		  LIMIT 1`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
	}

	args := []any{repositoryID, runID}
	where := "WHERE repository_id = $1 AND run_id = $2"
	if state != "" {
		args = append(args, state)
		where += " AND state = $3"
	}
	rows, err := collectRows(ctx, runner,
		`SELECT *
		   FROM striatumd.process_supervisors
		  `+where+`
		  ORDER BY started_at DESC, supervisor_id DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	supervisors := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		supervisors = append(supervisors, supervisorView(row))
	}
	return map[string]any{
		"run_id":      runID,
		"state":       nullableText(state),
		"supervisors": supervisors,
	}, nil
}

// HandleSuperviseReattachStatus returns the daemon reattach health DTO
// without repairing pointer rows or writing events.
func HandleSuperviseReattachStatus(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID, err := optionalTextParam(envelope, "run_id")
	if err != nil {
		return nil, err
	}
	sessionID, err := optionalTextParam(envelope, "session_id")
	if err != nil {
		return nil, err
	}
	supervisorID, err := optionalTextParam(envelope, "supervisor_id")
	if err != nil {
		return nil, err
	}
	if runID != "" {
		exists, err := rowExists(ctx, runner,
			`SELECT run_id FROM striatumd.runs
			  WHERE repository_id = $1 AND run_id = $2
			  LIMIT 1`,
			repositoryID, runID,
		)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
		}
	}
	if sessionID != "" {
		exists, err := rowExists(ctx, runner,
			`SELECT session_id FROM striatumd.sessions
			  WHERE repository_id = $1 AND session_id = $2
			  LIMIT 1`,
			repositoryID, sessionID,
		)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, rpc.NewError("not_found", "session not found: "+sessionID, nil)
		}
	}

	rows, err := reattachStatusRows(ctx, runner, repositoryID, runID, sessionID, supervisorID)
	if err != nil {
		return nil, err
	}
	if supervisorID != "" && len(rows) == 0 {
		return nil, rpc.NewError("not_found", fmt.Sprintf("no supervisor recorded for supervisor_id=%q", supervisorID), nil)
	}
	supervisors := make([]map[string]any, 0, len(rows))
	summary := map[string]int{
		"total":              0,
		"reattachable":       0,
		"lost_candidate":     0,
		"needs_repair":       0,
		"needs_verification": 0,
		"terminal":           0,
	}
	for _, row := range rows {
		view := reattachStatusView(row)
		supervisors = append(supervisors, view)
		summary["total"]++
		state := superviseString(view["reattach_state"])
		summary[state] = summary[state] + 1
	}
	return map[string]any{
		"run_id":        nullableText(runID),
		"session_id":    nullableText(sessionID),
		"supervisor_id": nullableText(supervisorID),
		"summary":       summary,
		"supervisors":   supervisors,
	}, nil
}

func reattachStatusRows(ctx context.Context, runner db.Runner, repositoryID, runID, sessionID, supervisorID string) ([]map[string]any, error) {
	args := []any{repositoryID}
	where := "ps.repository_id = $1"
	if runID != "" {
		args = append(args, runID)
		where += " AND ps.run_id = $" + strconv.Itoa(len(args))
	}
	if sessionID != "" {
		args = append(args, sessionID)
		where += " AND ps.session_id = $" + strconv.Itoa(len(args))
	}
	if supervisorID != "" {
		args = append(args, supervisorID)
		where += " AND ps.supervisor_id = $" + strconv.Itoa(len(args))
	}
	return collectRows(ctx, runner,
		`SELECT
		      ps.supervisor_id,
		      ps.run_id,
		      ps.session_id,
		      ps.adapter,
		      ps.cwd,
		      ps.scratch_path,
		      ps.stdin_pipe_path,
		      ps.pid,
		      ps.pid_start_time,
		      ps.state,
		      ps.started_at,
		      ps.heartbeat_at,
		      ps.ended_at,
		      ps.stop_reason,
		      p.daemon_supervisor_id AS pointer_daemon_supervisor_id,
		      p.pid AS pointer_pid,
		      p.pid_start_time AS pointer_pid_start_time,
		      p.state AS pointer_state,
		      p.updated_at AS pointer_updated_at,
		      p.metadata_json AS pointer_metadata_json,
		      ds.daemon_supervisor_id AS daemon_supervisor_id,
		      ds.daemon_instance_id,
		      ds.pid AS daemon_pid,
		      ds.pid_start_time AS daemon_pid_start_time,
		      ds.state AS daemon_state,
		      ds.heartbeat_at AS daemon_heartbeat_at,
		      ds.ended_at AS daemon_ended_at,
		      ds.stop_reason AS daemon_stop_reason
		   FROM striatumd.process_supervisors ps
		   LEFT JOIN striatumd.process_supervisor_pointers p
		     ON p.repository_id = ps.repository_id
		    AND p.supervisor_id = ps.supervisor_id
		   LEFT JOIN striatumd.daemon_supervisors ds
		     ON ds.repository_id = ps.repository_id
		    AND ds.daemon_supervisor_id = p.daemon_supervisor_id
		  WHERE `+where+`
		  ORDER BY ps.started_at DESC, ps.supervisor_id DESC`,
		args...,
	)
}

func reattachStatusView(row map[string]any) map[string]any {
	pid, hasPID := intValueOptional(row["pid"])
	pidAlive := hasPID && pidAlive(pid)
	currentStart := ""
	if pidAlive {
		currentStart, _ = processStartToken(pid)
	}
	state, reason, action := reattachState(row, pidAlive, currentStart)
	return map[string]any{
		"supervisor_id":          row["supervisor_id"],
		"run_id":                 row["run_id"],
		"session_id":             row["session_id"],
		"state":                  row["state"],
		"pid":                    optionalIntValue(pid, hasPID),
		"pid_liveness":           pidLiveness(pidAlive),
		"pid_start_time":         row["pid_start_time"],
		"current_pid_start_time": nullableText(currentStart),
		"pid_identity":           pidIdentity(row, pidAlive, currentStart),
		"reattach_state":         state,
		"reattach_reason":        nullableText(reason),
		"recommended_action":     action,
		"started_at":             timestampValue(row["started_at"]),
		"heartbeat_at":           timestampValue(row["heartbeat_at"]),
		"ended_at":               timestampValue(row["ended_at"]),
		"stop_reason":            row["stop_reason"],
		"stdin_pipe_path":        row["stdin_pipe_path"],
		"pointer": map[string]any{
			"daemon_supervisor_id": row["pointer_daemon_supervisor_id"],
			"state":                row["pointer_state"],
			"pid":                  optionalIntFromAny(row["pointer_pid"]),
			"pid_start_time":       row["pointer_pid_start_time"],
			"updated_at":           timestampValue(row["pointer_updated_at"]),
			"metadata":             superviseObject(row["pointer_metadata_json"]),
		},
		"daemon_supervisor": map[string]any{
			"daemon_supervisor_id": row["daemon_supervisor_id"],
			"daemon_instance_id":   row["daemon_instance_id"],
			"state":                row["daemon_state"],
			"pid":                  optionalIntFromAny(row["daemon_pid"]),
			"pid_start_time":       row["daemon_pid_start_time"],
			"heartbeat_at":         timestampValue(row["daemon_heartbeat_at"]),
			"ended_at":             timestampValue(row["daemon_ended_at"]),
			"stop_reason":          row["daemon_stop_reason"],
		},
	}
}

func reattachState(row map[string]any, pidAlive bool, currentStart string) (string, string, string) {
	state := superviseString(row["state"])
	if supervisorTerminalStates[state] {
		return "terminal", state, "no_action"
	}
	_, hasPID := intValueOptional(row["pid"])
	if !hasPID {
		return "lost_candidate", "pid_missing", "mark_lost_or_reconcile"
	}
	if !pidAlive {
		return "lost_candidate", "pid_gone", "mark_lost_or_reconcile"
	}
	expectedStart := superviseString(row["pid_start_time"])
	if expectedStart == "" || currentStart == "" {
		return "needs_verification", "pid_identity_unavailable", "verify_before_reattach"
	}
	if currentStart != expectedStart {
		return "lost_candidate", "pid_identity_mismatch", "mark_lost_or_reconcile"
	}
	pointerState := superviseString(row["pointer_state"])
	if row["pointer_daemon_supervisor_id"] == nil {
		return "needs_repair", "pointer_missing", "repair_supervisor_pointer"
	}
	if pointerState != state {
		return "needs_repair", "pointer_state_mismatch", "repair_supervisor_pointer"
	}
	if row["daemon_supervisor_id"] == nil {
		return "needs_repair", "daemon_supervisor_missing", "repair_daemon_supervisor"
	}
	daemonState := superviseString(row["daemon_state"])
	if daemonState != state {
		return "needs_repair", "daemon_state_mismatch", "repair_daemon_supervisor"
	}
	return "reattachable", "", "reattach"
}

func pidIdentity(row map[string]any, pidAlive bool, currentStart string) string {
	expectedStart := superviseString(row["pid_start_time"])
	if !pidAlive {
		return "pid_gone"
	}
	if expectedStart == "" || currentStart == "" {
		return "unverified"
	}
	if currentStart != expectedStart {
		return "mismatch"
	}
	return "matched"
}

func supervisorProgressForSession(ctx context.Context, runner db.Runner, repositoryID, sessionID string, stallAfterSeconds int) (map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.pid,
		        ps.state AS supervisor_state,
		        COALESCE(ptr.updated_at, ps.heartbeat_at) AS supervisor_heartbeat_at,
		        s.last_heartbeat_at AS session_last_heartbeat_at,
		        l.lease_id, l.resource_id AS job_id, l.acquired_at,
		        l.expires_at, l.last_heartbeat_at AS lease_last_heartbeat_at,
		        j.workflow_job_id, j.state AS job_state,
		        j.current_message_id AS message_id,
		        qm.state AS message_state
		   FROM striatumd.process_supervisors ps
		   JOIN striatumd.sessions s
		     ON s.repository_id = ps.repository_id
		    AND s.session_id = ps.session_id
		   JOIN striatumd.leases l
		     ON l.repository_id = ps.repository_id
		    AND l.run_id = ps.run_id
		    AND l.owner_session_id = ps.session_id
		   JOIN striatumd.jobs j
		     ON j.repository_id = l.repository_id
		    AND j.job_id = l.resource_id
		    AND j.current_lease_id = l.lease_id
		   LEFT JOIN striatumd.queue_messages qm
		     ON qm.repository_id = j.repository_id
		    AND qm.message_id = j.current_message_id
		   LEFT JOIN striatumd.process_supervisor_pointers ptr
		     ON ptr.repository_id = ps.repository_id
		    AND ptr.supervisor_id = ps.supervisor_id
		  WHERE ps.repository_id = $1
		    AND ps.session_id = $2
		    AND ps.state = 'attached'
		    AND l.state = 'active'
		    AND l.resource_type = 'job'
		    AND j.state IN ('claimed', 'running')
		  ORDER BY ps.started_at DESC, ps.supervisor_id DESC
		  LIMIT 1`,
		repositoryID, sessionID,
	)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return enrichSupervisorProgress(rows[0], time.Now().UTC().Truncate(time.Second), stallAfterSeconds), nil
}

func enrichSupervisorProgress(row map[string]any, now time.Time, stallAfterSeconds int) map[string]any {
	candidates := []any{
		row["lease_last_heartbeat_at"],
		row["session_last_heartbeat_at"],
		row["supervisor_heartbeat_at"],
		row["acquired_at"],
	}
	var lastProgress time.Time
	for _, candidate := range candidates {
		if parsed, ok := parseTimeValue(candidate); ok && (lastProgress.IsZero() || parsed.After(lastProgress)) {
			lastProgress = parsed
		}
	}
	var ageSeconds any
	stalledByAge := false
	if !lastProgress.IsZero() {
		age := int(now.Sub(lastProgress).Seconds())
		if age < 0 {
			age = 0
		}
		ageSeconds = age
		stalledByAge = age >= stallAfterSeconds
	}
	expiresAt, hasExpiresAt := parseTimeValue(row["expires_at"])
	acquiredAt, hasAcquiredAt := parseTimeValue(row["acquired_at"])
	leaseExpired := hasExpiresAt && expiresAt.Before(now)
	var leaseSeconds any
	preExpiryTracked := true
	if hasExpiresAt && hasAcquiredAt {
		seconds := int(expiresAt.Sub(acquiredAt).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		leaseSeconds = seconds
		preExpiryTracked = seconds <= 24*60*60
	}
	item := copyMap(row)
	item["last_progress_at"] = timestampValue(lastProgress)
	item["last_progress_age_seconds"] = ageSeconds
	item["stall_after_seconds"] = stallAfterSeconds
	item["lease_duration_seconds"] = leaseSeconds
	item["lease_expired"] = leaseExpired
	item["stalled"] = leaseExpired || (preExpiryTracked && stalledByAge)
	item["lease_expires_at"] = timestampValue(row["expires_at"])
	item["lease_last_heartbeat_at"] = timestampValue(row["lease_last_heartbeat_at"])
	item["session_last_heartbeat_at"] = timestampValue(row["session_last_heartbeat_at"])
	item["supervisor_heartbeat_at"] = timestampValue(row["supervisor_heartbeat_at"])
	return item
}

func supervisorView(row map[string]any) map[string]any {
	view := copyMap(row)
	for _, key := range []string{"started_at", "heartbeat_at", "ended_at"} {
		view[key] = timestampValue(view[key])
	}
	return view
}

func rowExists(ctx context.Context, runner db.Runner, sql string, args ...any) (bool, error) {
	rows, err := collectRows(ctx, runner, sql, args...)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func requiredTextParam(envelope rpc.Envelope, key string, message string) (string, error) {
	value, ok := envelope.Params[key]
	if !ok || value == nil {
		return "", rpc.NewError("schema_invalid", message, nil)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", rpc.NewError("schema_invalid", message, nil)
	}
	return text, nil
}

func optionalTextParam(envelope rpc.Envelope, key string) (string, error) {
	value, ok := envelope.Params[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", rpc.NewError("schema_invalid", key+" must be a string", nil)
	}
	return text, nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func pidLiveness(alive bool) string {
	if alive {
		return "alive"
	}
	return "gone"
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalIntValue(value int, ok bool) any {
	if !ok {
		return nil
	}
	return value
}

func optionalIntFromAny(value any) any {
	if parsed, ok := intValueOptional(value); ok {
		return parsed
	}
	return nil
}

func intValueOptional(value any) (int, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case int:
		return typed, true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case string:
		if typed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	}
	return 0, false
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func superviseString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func superviseObject(value any) map[string]any {
	result := objectFromJSONish(value)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func timestampValue(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		if typed.IsZero() {
			return nil
		}
		return typed.UTC().Truncate(time.Second).Format(time.RFC3339)
	case *time.Time:
		if typed == nil || typed.IsZero() {
			return nil
		}
		return typed.UTC().Truncate(time.Second).Format(time.RFC3339)
	default:
		return value
	}
}

func parseTimeValue(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), true
	case string:
		if typed == "" {
			return time.Time{}, false
		}
		parsed, err := time.Parse(time.RFC3339, typed)
		if err != nil {
			return time.Time{}, false
		}
		return parsed.UTC(), true
	}
	return time.Time{}, false
}
