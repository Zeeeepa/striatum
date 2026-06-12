package supervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	HelperLaunchSchemaVersion = "striatum.supervisor_helper.launch.v1"
	HelperEventSchemaVersion  = "striatum.supervisor_helper.event.v1"

	HelperEventAgentStarted      = "agent_started"
	HelperEventPacketAccepted    = "packet_accepted"
	HelperEventProgress          = "progress"
	HelperEventAgentExited       = "agent_exited"
	HelperEventAttachExited      = "attach_client_exited"
	HelperEventProcessTerminated = "process_terminated"
	HelperEventError             = "helper_error"
)

// HelperLaunchSpec is the single JSON object accepted by the standalone
// supervisor helper. It is deliberately process-only: the daemon core owns
// supervise.* RPC, Postgres rows, packet construction, and domain validation.
type HelperLaunchSpec struct {
	SchemaVersion   string        `json:"schema_version,omitempty"`
	SupervisorID    string        `json:"supervisor_id"`
	ScratchDir      string        `json:"scratch_dir"`
	Command         []string      `json:"command"`
	Env             []string      `json:"env,omitempty"`
	WorkingDir      string        `json:"working_dir,omitempty"`
	RunAsUser       string        `json:"run_as_user,omitempty"`
	PacketInputPath string        `json:"packet_input_path,omitempty"`
	PTYLogPath      string        `json:"pty_log_path,omitempty"`
	RequireTmux     bool          `json:"require_tmux,omitempty"`
	RebridgeTmux    *TmuxIdentity `json:"rebridge_tmux,omitempty"`
}

// HelperControlEvent is emitted as one JSON object per line on the helper's
// control-event stream. Payloads intentionally carry lifecycle metadata and
// byte counts only; agent output bytes stay out of the control channel.
type HelperControlEvent struct {
	SchemaVersion string         `json:"schema_version"`
	EventType     string         `json:"event_type"`
	SupervisorID  string         `json:"supervisor_id,omitempty"`
	PacketID      string         `json:"packet_id,omitempty"`
	Timestamp     string         `json:"timestamp"`
	Payload       map[string]any `json:"payload,omitempty"`
}

func decodeHelperLaunchSpec(decoder *json.Decoder) (HelperLaunchSpec, error) {
	decoder.DisallowUnknownFields()
	var spec HelperLaunchSpec
	if err := decoder.Decode(&spec); err != nil {
		return HelperLaunchSpec{}, fmt.Errorf("decode launch spec: %w", err)
	}
	if spec.SchemaVersion != "" && spec.SchemaVersion != HelperLaunchSchemaVersion {
		return HelperLaunchSpec{}, fmt.Errorf("unsupported schema_version %q", spec.SchemaVersion)
	}
	if strings.TrimSpace(spec.SupervisorID) == "" {
		return HelperLaunchSpec{}, fmt.Errorf("supervisor_id is required")
	}
	if strings.TrimSpace(spec.ScratchDir) == "" {
		return HelperLaunchSpec{}, fmt.Errorf("scratch_dir is required")
	}
	if spec.RebridgeTmux == nil && (len(spec.Command) == 0 || strings.TrimSpace(spec.Command[0]) == "") {
		return HelperLaunchSpec{}, fmt.Errorf("command is required")
	}
	for i, part := range spec.Command {
		if strings.TrimSpace(part) == "" {
			return HelperLaunchSpec{}, fmt.Errorf("command[%d] is empty", i)
		}
	}
	return spec, nil
}

func newHelperEvent(eventType string, supervisorID string, payload map[string]any) HelperControlEvent {
	return HelperControlEvent{
		SchemaVersion: HelperEventSchemaVersion,
		EventType:     eventType,
		SupervisorID:  supervisorID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Payload:       payload,
	}
}

func encodeHelperEvent(w io.Writer, event HelperControlEvent) error {
	if event.SchemaVersion == "" {
		event.SchemaVersion = HelperEventSchemaVersion
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return json.NewEncoder(w).Encode(event)
}
