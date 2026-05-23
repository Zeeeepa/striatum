package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunHelperRejectsMalformedLaunchSpec(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "invalid_json", body: "{not json\n"},
		{name: "missing_command", body: `{"schema_version":"striatum.supervisor_helper.launch.v1","supervisor_id":"sup_bad","scratch_dir":"/tmp"}`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var events bytes.Buffer
			err := RunHelper(
				context.Background(),
				strings.NewReader(tc.body),
				&events,
				HelperOptions{},
			)
			if err == nil {
				t.Fatal("expected malformed launch spec to fail")
			}
			decoded, decodeErr := helperEventsFromJSONL(events.Bytes())
			if decodeErr != nil {
				t.Fatalf("decode events: %v\nraw=%s", decodeErr, events.String())
			}
			if len(decoded) != 1 {
				t.Fatalf("events: got %d want 1: %#v", len(decoded), decoded)
			}
			if decoded[0].EventType != HelperEventError {
				t.Fatalf("event type: got %q want %q", decoded[0].EventType, HelperEventError)
			}
			if decoded[0].Payload["phase"] != "decode_launch" {
				t.Fatalf("helper_error phase: %#v", decoded[0].Payload)
			}
		})
	}
}

func TestRunHelperPTYCatPacketRoundTrip(t *testing.T) {
	if _, err := os.Stat("/bin/cat"); err != nil {
		t.Skip("/bin/cat not present; skipping PTY packet round trip")
	}
	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_helper_cat",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/cat"},
	}
	launchBytes, err := json.Marshal(launch)
	if err != nil {
		t.Fatalf("marshal launch: %v", err)
	}
	packet := []byte("hello striatum\n")
	input := bytes.NewReader(append(append(launchBytes, '\n'), packet...))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var events bytes.Buffer
	var ptyOutput bytes.Buffer
	err = RunHelper(ctx, input, &events, HelperOptions{PTYOutput: &ptyOutput})
	if err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s\npty=%q", err, events.String(), ptyOutput.String())
	}

	normalized := bytes.ReplaceAll(ptyOutput.Bytes(), []byte{'\r'}, nil)
	if !bytes.Contains(normalized, bytes.TrimSpace(packet)) {
		t.Fatalf("PTY output missing packet: got %q want substring %q", ptyOutput.String(), packet)
	}
}

func TestRunHelperEmitsLifecyclePacketAndProgressEvents(t *testing.T) {
	if _, err := os.Stat("/bin/cat"); err != nil {
		t.Skip("/bin/cat not present; skipping helper event test")
	}
	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_helper_events",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/cat"},
	}
	launchBytes, err := json.Marshal(launch)
	if err != nil {
		t.Fatalf("marshal launch: %v", err)
	}
	input := bytes.NewReader(append(append(launchBytes, '\n'), []byte("event packet\n")...))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var events bytes.Buffer
	var ptyOutput bytes.Buffer
	if err := RunHelper(ctx, input, &events, HelperOptions{PTYOutput: &ptyOutput}); err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s", err, events.String())
	}

	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	if len(decoded) < 4 {
		t.Fatalf("expected at least 4 events, got %d: %#v", len(decoded), decoded)
	}
	seen := map[string]int{}
	var startedPayload map[string]any
	for _, event := range decoded {
		seen[event.EventType]++
		if event.EventType == HelperEventAgentStarted {
			startedPayload = event.Payload
		}
		if event.SchemaVersion != HelperEventSchemaVersion {
			t.Fatalf("event %q schema_version: got %q want %q", event.EventType, event.SchemaVersion, HelperEventSchemaVersion)
		}
		if event.EventType != HelperEventError && event.SupervisorID != launch.SupervisorID {
			t.Fatalf("event %q supervisor_id: got %q want %q", event.EventType, event.SupervisorID, launch.SupervisorID)
		}
	}
	for _, eventType := range []string{
		HelperEventAgentStarted,
		HelperEventPacketAccepted,
		HelperEventProgress,
		HelperEventAgentExited,
	} {
		if seen[eventType] == 0 {
			t.Fatalf("missing event %q in %#v\nraw=%s", eventType, seen, events.String())
		}
	}
	if seen[HelperEventError] != 0 {
		t.Fatalf("unexpected helper_error event: raw=%s", events.String())
	}
	metadata, ok := startedPayload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("agent_started metadata missing: %#v", startedPayload)
	}
	tmux, ok := metadata["tmux"].(map[string]any)
	if !ok || tmux["unavailable_reason"] != "missing_run_or_lane" {
		t.Fatalf("agent_started tmux metadata = %#v", metadata["tmux"])
	}
}

func TestRunHelperRequireTmuxUnavailableEmitsLaunchError(t *testing.T) {
	truePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("PATH", t.TempDir())

	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_helper_tmux_required",
		ScratchDir:    t.TempDir(),
		Command:       []string{truePath, "-test.run=^$"},
		Env: []string{
			"STRIATUM_RUN_ID=run_helper_tmux_required",
			"STRIATUM_LANE_ID=lane_helper_tmux_required",
		},
		RequireTmux: true,
	}
	launchBytes, err := json.Marshal(launch)
	if err != nil {
		t.Fatalf("marshal launch: %v", err)
	}
	input := bytes.NewReader(append(launchBytes, '\n'))

	var events bytes.Buffer
	err = RunHelper(context.Background(), input, &events, HelperOptions{})
	if err == nil {
		t.Fatal("expected RunHelper to fail when required tmux is unavailable")
	}
	decoded, decodeErr := helperEventsFromJSONL(events.Bytes())
	if decodeErr != nil {
		t.Fatalf("decode events: %v\nraw=%s", decodeErr, events.String())
	}
	if len(decoded) != 1 {
		t.Fatalf("events: got %d want 1: %#v", len(decoded), decoded)
	}
	if decoded[0].EventType != HelperEventError {
		t.Fatalf("event type: got %q want %q", decoded[0].EventType, HelperEventError)
	}
	if decoded[0].Payload["phase"] != "launch" {
		t.Fatalf("helper_error phase: %#v", decoded[0].Payload)
	}
	if errorText, _ := decoded[0].Payload["error"].(string); !strings.Contains(errorText, "tmux required") {
		t.Fatalf("helper_error text = %q, want tmux-required refusal", errorText)
	}
}

func helperEventsFromJSONL(data []byte) ([]HelperControlEvent, error) {
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	events := make([]HelperControlEvent, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event HelperControlEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
