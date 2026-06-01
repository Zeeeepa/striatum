package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
	"github.com/jackc/pgx/v5"
)

var superviseReportEventTypes = map[string]bool{
	gosupervisor.HelperEventPacketAccepted: true,
	gosupervisor.HelperEventAgentStarted:   true,
	gosupervisor.HelperEventAttachExited:   true,
	"artifact_observed":                    true,
	gosupervisor.HelperEventProgress:       true,
	gosupervisor.HelperEventAgentExited:    true,
	gosupervisor.HelperEventError:          true,
}

var supervisorActiveStates = map[string]bool{
	"starting": true,
	"attached": true,
	"detached": true,
}

// supervisorTerminalStates are the supervisor states that mean the lane process
// is gone. Reaching any of them must trigger per-lane MCP-config teardown
// (issue #62): the ephemeral .gemini/settings.json bearer-token file the agy
// lane wrote must be removed/restored regardless of which exit path fired.
var supervisorTerminalStates = map[string]bool{
	"stopped": true,
	"lost":    true,
	"failed":  true,
}

type superviseReportEvent struct {
	EventType    string
	SessionID    string
	SupervisorID string
	PacketID     string
	Payload      map[string]any
	ReportedAt   string
}

type supervisorReportRow struct {
	SupervisorID       string
	RunID              string
	SessionID          string
	State              string
	DaemonSupervisorID string
	PID                int
	HasPID             bool
	PIDStartTime       string
	Metadata           map[string]any
}

func HandleSuperviseReport(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	events, batch, err := parseSuperviseReportEvents(envelope.Params)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return map[string]any{"events_recorded": 0, "results": []map[string]any{}, "state": nil}, nil
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		results := make([]map[string]any, 0, len(events))
		for _, event := range events {
			result, err := recordSuperviseReportEvent(ctx, tx, repositoryID, event)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
		if batch {
			state := any(nil)
			if len(results) > 0 {
				state = results[len(results)-1]["state"]
			}
			return map[string]any{
				"events_recorded": len(results),
				"results":         results,
				"state":           state,
			}, nil
		}
		return results[0], nil
	})
}

func parseSuperviseReportEvents(params map[string]any) ([]superviseReportEvent, bool, error) {
	if _, direct := params["event_type"]; direct {
		event, err := normalizeSuperviseReportEvent(params, "", "", 0)
		if err != nil {
			return nil, false, err
		}
		return []superviseReportEvent{event}, false, nil
	}

	source, ok, err := helperEventSource(params)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, rpc.NewError("schema_invalid", "event_type is required", nil)
	}
	defaultSessionID, err := optionalStringParam(params, "session_id")
	if err != nil {
		return nil, true, err
	}
	defaultSupervisorID, err := optionalStringParam(params, "supervisor_id")
	if err != nil {
		return nil, true, err
	}
	if defaultSessionID == "" && defaultSupervisorID == "" {
		return nil, true, rpc.NewError("schema_invalid", "session_id or supervisor_id is required", nil)
	}

	rawEvents, err := parseHelperEvents(source)
	if err != nil {
		return nil, true, err
	}
	events := make([]superviseReportEvent, 0, len(rawEvents))
	for i, raw := range rawEvents {
		event, err := normalizeSuperviseReportEvent(raw, defaultSessionID, defaultSupervisorID, i+1)
		if err != nil {
			return nil, true, err
		}
		events = append(events, event)
	}
	return events, true, nil
}

func helperEventSource(params map[string]any) (any, bool, error) {
	if value, ok := params["events_path"]; ok {
		path, ok := value.(string)
		if !ok || path == "" {
			return nil, true, rpc.NewError("schema_invalid", "events_path must be a string path", nil)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, true, err
		}
		return string(body), true, nil
	}
	if value, ok := params["events"]; ok {
		return value, true, nil
	}
	if value, ok := params["events_jsonl"]; ok {
		return value, true, nil
	}
	return nil, false, nil
}

func parseHelperEvents(source any) ([]map[string]any, error) {
	switch typed := source.(type) {
	case string:
		return parseHelperJSONL(typed)
	case []any:
		events := make([]map[string]any, 0, len(typed))
		for i, value := range typed {
			event, ok := value.(map[string]any)
			if !ok {
				return nil, rpc.NewError("schema_invalid", fmt.Sprintf("helper event %d must be an object", i+1), nil)
			}
			events = append(events, event)
		}
		return events, nil
	case []map[string]any:
		return append([]map[string]any(nil), typed...), nil
	default:
		return nil, rpc.NewError("schema_invalid", "helper events must be provided as JSONL text, a path, or a list of objects", nil)
	}
}

func parseHelperJSONL(text string) ([]map[string]any, error) {
	events := []map[string]any{}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, rpc.NewError("schema_invalid", fmt.Sprintf("invalid helper event JSON on line %d: %s", i+1, jsonErrorMessage(err)), nil)
		}
		events = append(events, event)
	}
	return events, nil
}

func normalizeSuperviseReportEvent(raw map[string]any, defaultSessionID string, defaultSupervisorID string, index int) (superviseReportEvent, error) {
	prefix := ""
	if index > 0 {
		prefix = fmt.Sprintf("helper event %d ", index)
	}
	if schema, ok := raw["schema_version"]; ok && schema != nil && schema != "" {
		schemaText, ok := schema.(string)
		if !ok || schemaText != gosupervisor.HelperEventSchemaVersion {
			return superviseReportEvent{}, rpc.NewError("schema_invalid", fmt.Sprintf("%shas unsupported schema_version %q", prefix, schema), nil)
		}
	}
	eventType, ok := raw["event_type"].(string)
	if !ok || eventType == "" {
		return superviseReportEvent{}, rpc.NewError("schema_invalid", prefix+"requires event_type", nil)
	}
	if !superviseReportEventTypes[eventType] {
		return superviseReportEvent{}, rpc.NewError("schema_invalid", prefix+"event_type must be one of: agent_exited, agent_started, artifact_observed, attach_client_exited, helper_error, packet_accepted, progress", nil)
	}
	supervisorID, err := optionalStringParam(raw, "supervisor_id")
	if err != nil {
		return superviseReportEvent{}, err
	}
	sessionID, err := optionalStringParam(raw, "session_id")
	if err != nil {
		return superviseReportEvent{}, err
	}
	if supervisorID == "" {
		supervisorID = defaultSupervisorID
	}
	if sessionID == "" {
		sessionID = defaultSessionID
	}
	if supervisorID == "" && sessionID == "" {
		return superviseReportEvent{}, rpc.NewError("schema_invalid", prefix+"requires supervisor_id or session_id", nil)
	}
	packetID, err := optionalStringParam(raw, "packet_id")
	if err != nil {
		return superviseReportEvent{}, err
	}
	reportedAt, err := optionalStringParam(raw, "reported_at")
	if err != nil {
		return superviseReportEvent{}, err
	}
	if reportedAt == "" {
		reportedAt, err = optionalStringParam(raw, "timestamp")
		if err != nil {
			return superviseReportEvent{}, err
		}
	}
	payload, err := objectParam(raw, "payload")
	if err != nil {
		if index > 0 {
			return superviseReportEvent{}, rpc.NewError("schema_invalid", fmt.Sprintf("helper event %d payload must be an object", index), nil)
		}
		return superviseReportEvent{}, err
	}
	return superviseReportEvent{
		EventType:    eventType,
		SessionID:    sessionID,
		SupervisorID: supervisorID,
		PacketID:     packetID,
		Payload:      payload,
		ReportedAt:   reportedAt,
	}, nil
}

func recordSuperviseReportEvent(ctx context.Context, runner db.TxRunner, repositoryID string, event superviseReportEvent) (map[string]any, error) {
	supervisor, err := findReportSupervisor(ctx, runner, repositoryID, event.SupervisorID, event.SessionID)
	if err != nil {
		return nil, err
	}
	if event.SessionID != "" && supervisor.SessionID != event.SessionID {
		return nil, rpc.NewError("invalid_transition", "session_id does not match supervisor_id", nil)
	}
	if !supervisorActiveStates[supervisor.State] {
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("supervise report requires an active supervisor (state=%s)", supervisor.State), nil)
	}

	now := nowString()
	state := supervisor.State
	if event.EventType == gosupervisor.HelperEventAgentExited {
		stopReason := "agent exited"
		if exitCode, ok := event.Payload["exit_code"]; ok && exitCode != nil {
			stopReason = fmt.Sprintf("agent exited with code %v", exitCode)
		}
		if err := updateReportSupervisorStopped(ctx, runner, repositoryID, supervisor, now, stopReason); err != nil {
			return nil, err
		}
		state = "stopped"
	} else if event.EventType == gosupervisor.HelperEventAttachExited {
		event = attachExitWithDaemonObservedLiveness(ctx, supervisor, event)
		if attachExitPaneStillLive(supervisor, event) {
			if err := refreshReportSupervisorHeartbeat(ctx, runner, repositoryID, supervisor, now); err != nil {
				return nil, err
			}
			if err := updateReportSupervisorAttachExitMetadata(ctx, runner, repositoryID, supervisor, event, now); err != nil {
				return nil, err
			}
		} else {
			if err := updateReportSupervisorDetached(ctx, runner, repositoryID, supervisor, now); err != nil {
				return nil, err
			}
			state = "detached"
		}
	} else if err := refreshReportSupervisorHeartbeat(ctx, runner, repositoryID, supervisor, now); err != nil {
		return nil, err
	}

	// RFC 0101 Phase 1 (Layer 1): a progress event the helper flagged as
	// meaningful is positive evidence the lane is producing real output (D028 —
	// observed by volume/timing, never content). Refresh the active lease's
	// work-heartbeat so honest local work between MCP calls keeps the lease
	// alive instead of tripping agent_lease_heartbeat_stall (#80 / #136). This
	// is the daemon-mediated form of the "auto-heartbeat the lease" mechanism:
	// the helper observes the PTY and the daemon — the sole authority over lease
	// state (D094) — performs the lease transition.
	if event.EventType == gosupervisor.HelperEventProgress && progressIsMeaningful(event.Payload) && supervisor.SessionID != "" {
		// Slice 2 (#80 working_local): stamp last_pty_activity_at on meaningful
		// PTY output regardless of lease state — it is the raw "the child is
		// producing output" signal the classifier reads to report working_local
		// and to keep an honestly-working local lane out of protocol_idle.
		if err := sessionliveness.Record(ctx, runner, repositoryID, supervisor.SessionID, sessionliveness.LastPTYActivityAt); err != nil {
			return nil, err
		}
		// Lease-gated: refresh the active lease work-heartbeat (no-op without one).
		if err := refreshActiveLeaseWorkHeartbeat(ctx, runner, repositoryID, supervisor.RunID, supervisor.SessionID, now); err != nil {
			return nil, err
		}
	}

	payload := map[string]any{
		"supervisor_id":        supervisor.SupervisorID,
		"daemon_supervisor_id": nullableString(supervisor.DaemonSupervisorID),
		"session_id":           supervisor.SessionID,
		"control_event_type":   event.EventType,
	}
	if event.PacketID != "" {
		payload["packet_id"] = event.PacketID
	}
	if event.ReportedAt != "" {
		payload["reported_at"] = event.ReportedAt
	}
	if nested := curatedSuperviseReportPayload(event, now); len(nested) > 0 {
		payload["payload"] = nested
	}
	if _, err := appendEvent(ctx, runner, repositoryID, supervisor.RunID, "supervisor."+event.EventType, supervisor.SessionID, nil, nil, nil, nil, payload); err != nil {
		return nil, err
	}

	return map[string]any{
		"supervisor_id": supervisor.SupervisorID,
		"session_id":    supervisor.SessionID,
		"event_type":    event.EventType,
		"recorded_at":   now,
		"state":         state,
	}, nil
}

func curatedSuperviseReportPayload(event superviseReportEvent, now string) map[string]any {
	switch event.EventType {
	case gosupervisor.HelperEventPacketAccepted:
		return copyAllowedPayloadFields(event.Payload, "bytes", "sequence")
	case gosupervisor.HelperEventProgress:
		return copyAllowedPayloadFields(event.Payload, "bytes", "total_bytes", "meaningful")
	case gosupervisor.HelperEventAgentStarted:
		payload := copyAllowedPayloadFields(event.Payload, "pid", "attach_pid", "attach_client_pid")
		if metadata := curatedAgentStartedMetadata(asMap(event.Payload["metadata"])); len(metadata) > 0 {
			payload["metadata"] = metadata
		}
		return payload
	case gosupervisor.HelperEventAgentExited:
		return copyAllowedPayloadFields(event.Payload, "exit_code", "error", "cause")
	case gosupervisor.HelperEventAttachExited:
		payload := copyAllowedPayloadFields(event.Payload,
			"pid",
			"attach_pid",
			"attach_client_pid",
			"attach_exit_code",
			"exit_code",
			"attach_error",
			"observed_at",
			"tmux_liveness",
		)
		payload["delivery_degraded"] = true
		payload["delivery_liveness"] = attachExitDeliveryLiveness(event, attachExitObservedAt(event, now))
		return payload
	case gosupervisor.HelperEventError:
		return copyAllowedPayloadFields(event.Payload, "phase", "error")
	case "artifact_observed":
		return copyAllowedPayloadFields(event.Payload, "artifact_id", "kind", "logical_name", "path", "sha256")
	default:
		return map[string]any{}
	}
}

func copyAllowedPayloadFields(source map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			out[key] = value
		}
	}
	return out
}

func curatedAgentStartedMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := copyAllowedPayloadFields(metadata, "stdin_delivery", "transport")
	if tmux := curatedTmuxMetadata(asMap(metadata["tmux"])); len(tmux) > 0 {
		out["tmux"] = tmux
	}
	return out
}

func curatedTmuxMetadata(tmux map[string]any) map[string]any {
	return copyAllowedPayloadFields(tmux,
		"state",
		"session_name",
		"window_id",
		"pane_id",
		"pane_pid",
		"pane_start_time",
		"pane_start_token",
		"attach_command",
	)
}

func findReportSupervisor(ctx context.Context, runner db.TxRunner, repositoryID string, supervisorID string, sessionID string) (supervisorReportRow, error) {
	if supervisorID != "" {
		return scanReportSupervisor(runner.QueryRow(ctx, `
			SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.state,
			       ps.pid, ps.pid_start_time,
			       p.daemon_supervisor_id, COALESCE(p.metadata_json, '{}'::jsonb)
			  FROM striatumd.process_supervisors ps
			  LEFT JOIN striatumd.process_supervisor_pointers p
			    ON p.repository_id = ps.repository_id AND p.supervisor_id = ps.supervisor_id
			 WHERE ps.repository_id = $1 AND ps.supervisor_id = $2
			 LIMIT 1
			 FOR UPDATE OF ps`, repositoryID, supervisorID),
			rpc.NewError("not_found", fmt.Sprintf("no supervisor recorded for supervisor_id=%q", supervisorID), nil),
		)
	}
	return scanReportSupervisor(runner.QueryRow(ctx, `
		SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.state,
		       ps.pid, ps.pid_start_time,
		       p.daemon_supervisor_id, COALESCE(p.metadata_json, '{}'::jsonb)
		  FROM striatumd.process_supervisors ps
		  LEFT JOIN striatumd.process_supervisor_pointers p
		    ON p.repository_id = ps.repository_id AND p.supervisor_id = ps.supervisor_id
		 WHERE ps.repository_id = $1 AND ps.session_id = $2
		   AND ps.state IN ('starting','attached','detached')
		 ORDER BY ps.started_at DESC, ps.supervisor_id DESC
		 LIMIT 1
		 FOR UPDATE OF ps`, repositoryID, sessionID),
		rpc.NewError("invalid_transition", fmt.Sprintf("no active supervisor for session_id=%q", sessionID), nil),
	)
}

func scanReportSupervisor(row db.Row, notFound error) (supervisorReportRow, error) {
	var supervisor supervisorReportRow
	var daemonSupervisorID *string
	var pid *int
	var pidStartTime *string
	var rawMetadata any
	err := row.Scan(
		&supervisor.SupervisorID,
		&supervisor.RunID,
		&supervisor.SessionID,
		&supervisor.State,
		&pid,
		&pidStartTime,
		&daemonSupervisorID,
		&rawMetadata,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return supervisorReportRow{}, notFound
		}
		return supervisorReportRow{}, err
	}
	if daemonSupervisorID != nil {
		supervisor.DaemonSupervisorID = *daemonSupervisorID
	}
	if pid != nil {
		supervisor.PID = *pid
		supervisor.HasPID = true
	}
	if pidStartTime != nil {
		supervisor.PIDStartTime = *pidStartTime
	}
	supervisor.Metadata = asMap(rawMetadata)
	return supervisor, nil
}

func attachExitWithDaemonObservedLiveness(ctx context.Context, supervisor supervisorReportRow, event superviseReportEvent) superviseReportEvent {
	live := gosupervisor.ProbeLaneLiveness(ctx, supervisionTmuxRunner, supervisor.Metadata, supervisor.PID, supervisor.PIDStartTime)
	if live.Backed != "tmux" {
		return event
	}
	payload := map[string]any{}
	for key, value := range event.Payload {
		payload[key] = value
	}
	if helperClass, _ := event.Payload["tmux_liveness"].(string); strings.TrimSpace(helperClass) != "" {
		payload["helper_tmux_liveness"] = helperClass
	}
	payload["tmux_liveness"] = live.Class
	if live.Detail != "" {
		payload["tmux_liveness_detail"] = live.Detail
	}
	event.Payload = payload
	return event
}

func attachExitPaneStillLive(supervisor supervisorReportRow, event superviseReportEvent) bool {
	tmux := asMap(supervisor.Metadata["tmux"])
	if strings.TrimSpace(fmt.Sprint(tmux["state"])) != "backed" {
		return false
	}
	class, _ := event.Payload["tmux_liveness"].(string)
	return class == string(gosupervisor.TmuxLivenessOK) || class == string(gosupervisor.TmuxLivenessUnavailable)
}

func updateReportSupervisorAttachExitMetadata(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorReportRow, event superviseReportEvent, now string) error {
	metadata := map[string]any{}
	for key, value := range supervisor.Metadata {
		metadata[key] = value
	}
	tmux := asMap(metadata["tmux"])
	if len(tmux) == 0 {
		tmux = map[string]any{"state": "backed"}
	}
	lastExit := map[string]any{
		"observed_at":   now,
		"tmux_liveness": event.Payload["tmux_liveness"],
	}
	if event.ReportedAt != "" {
		lastExit["observed_at"] = event.ReportedAt
		lastExit["reported_at"] = event.ReportedAt
	}
	if observedAt, ok := event.Payload["observed_at"].(string); ok && observedAt != "" {
		lastExit["observed_at"] = observedAt
	}
	if pid, ok := event.Payload["attach_client_pid"]; ok {
		lastExit["attach_pid"] = pid
	} else if pid, ok := event.Payload["attach_pid"]; ok {
		lastExit["attach_pid"] = pid
	}
	if exitCode, ok := event.Payload["attach_exit_code"]; ok {
		lastExit["attach_exit_code"] = exitCode
	} else if exitCode, ok := event.Payload["exit_code"]; ok {
		lastExit["attach_exit_code"] = exitCode
	}
	if panePID, ok := event.Payload["pid"]; ok {
		lastExit["pane_pid"] = panePID
	}
	observedAt, _ := lastExit["observed_at"].(string)
	deliveryLiveness := attachExitDeliveryLiveness(event, observedAt)
	lastExit["delivery_liveness"] = deliveryLiveness
	tmux["attach_client_last_exit"] = lastExit
	tmux["delivery_liveness"] = deliveryLiveness
	metadata["tmux"] = tmux
	metadataArg, err := db.JSONBArg(runner, metadata)
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET metadata_json = $1::jsonb,
		       updated_at = $2
		 WHERE repository_id = $3 AND supervisor_id = $4`,
		metadataArg, now, repositoryID, supervisor.SupervisorID,
	)
}

func attachExitDeliveryLiveness(event superviseReportEvent, observedAt string) map[string]any {
	payload := map[string]any{
		"class":       "degraded",
		"healthy":     false,
		"reason":      "attach_client_exited",
		"observed_at": observedAt,
	}
	if observedAt == "" {
		delete(payload, "observed_at")
	}
	return payload
}

func attachExitObservedAt(event superviseReportEvent, fallback string) string {
	if observedAt, ok := event.Payload["observed_at"].(string); ok && strings.TrimSpace(observedAt) != "" {
		return observedAt
	}
	if strings.TrimSpace(event.ReportedAt) != "" {
		return event.ReportedAt
	}
	return fallback
}

func updateReportSupervisorDetached(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorReportRow, now string) error {
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisors
		   SET state = 'detached',
		       heartbeat_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		now, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET state = 'detached',
		       updated_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		now, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if supervisor.DaemonSupervisorID == "" {
		return nil
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.daemon_supervisors
		   SET state = 'detached',
		       heartbeat_at = $1
		 WHERE repository_id = $2 AND daemon_supervisor_id = $3`,
		now, repositoryID, supervisor.DaemonSupervisorID,
	)
}

func updateReportSupervisorStopped(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorReportRow, now string, stopReason string) error {
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisors
		   SET state = 'stopped',
		       ended_at = $1,
		       stop_reason = $2
		 WHERE repository_id = $3 AND supervisor_id = $4`,
		now, stopReason, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET state = 'stopped',
		       updated_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		now, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if supervisor.DaemonSupervisorID == "" {
		return nil
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.daemon_supervisors
		   SET state = 'stopped',
		       ended_at = $1,
		       stop_reason = $2
		 WHERE repository_id = $3 AND daemon_supervisor_id = $4`,
		now, stopReason, repositoryID, supervisor.DaemonSupervisorID,
	)
}

func refreshReportSupervisorHeartbeat(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorReportRow, now string) error {
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisors
		   SET heartbeat_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		now, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET updated_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		now, repositoryID, supervisor.SupervisorID,
	); err != nil {
		return err
	}
	if supervisor.DaemonSupervisorID == "" {
		return nil
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.daemon_supervisors
		   SET heartbeat_at = $1
		 WHERE repository_id = $2 AND daemon_supervisor_id = $3`,
		now, repositoryID, supervisor.DaemonSupervisorID,
	)
}

// progressIsMeaningful reports whether the helper tagged this progress event as
// meaningful output (RFC 0101 Phase 1). The helper sets payload["meaningful"]
// only when output volume crossed the OQ1 threshold within its window; a bare
// redraw/spinner frame leaves it unset.
func progressIsMeaningful(payload map[string]any) bool {
	value, ok := payload["meaningful"]
	if !ok || value == nil {
		return false
	}
	if flag, ok := value.(bool); ok {
		return flag
	}
	return false
}

// refreshActiveLeaseWorkHeartbeat finds the session's single active lease (if
// any) and refreshes it the same way work.heartbeat does: it stamps the
// session's last_work_heartbeat_at activity column (which sessionliveness.Classify
// treats as a fresh lease/work signal) and extends the lease so the lane is not
// reaped while it is demonstrably producing output. It is a no-op when the
// session holds no active lease — a lease-less lane has no lease-heartbeat
// deadline to miss, so there is nothing to refresh.
func refreshActiveLeaseWorkHeartbeat(ctx context.Context, runner db.TxRunner, repositoryID string, runID string, sessionID string, now string) error {
	var leaseID string
	var resourceID *string
	err := runner.QueryRow(ctx, `
		SELECT lease_id, resource_id
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
		 ORDER BY acquired_at DESC, lease_id DESC
		 LIMIT 1`, repositoryID, sessionID).Scan(&leaseID, &resourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(leaseID) == "" {
		return nil
	}
	expiresAt := expiresAfter(1800)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET last_heartbeat_at = $1, expires_at = $2
		 WHERE repository_id = $3 AND lease_id = $4`, now, expiresAt, repositoryID, leaseID); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET last_heartbeat_at = $1
		 WHERE repository_id = $2 AND session_id = $3`, now, repositoryID, sessionID); err != nil {
		return err
	}
	if err := sessionliveness.Record(ctx, runner, repositoryID, sessionID, sessionliveness.LastWorkHeartbeatAt); err != nil {
		return err
	}
	resource := ""
	if resourceID != nil {
		resource = *resourceID
	}
	_, err = appendEvent(ctx, runner, repositoryID, runID, "lease.heartbeat", sessionID, resource, nil, nil, leaseID, map[string]any{
		"expires_at": expiresAt,
		"source":     "supervisor_pty_progress",
	})
	return err
}

func optionalStringParam(params map[string]any, key string) (string, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", rpc.NewError("schema_invalid", key+" must be a string", nil)
	}
	return text, nil
}

func objectParam(params map[string]any, key string) (map[string]any, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return map[string]any{}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, rpc.NewError("schema_invalid", key+" must be an object when provided", nil)
	}
	return object, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func jsonErrorMessage(err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return err.Error()
	}
	return err.Error()
}
