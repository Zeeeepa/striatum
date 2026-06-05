package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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

func TestRunHelperPassesRunAsUserToLaunchSpec(t *testing.T) {
	origLaunch := helperLaunch
	defer func() { helperLaunch = origLaunch }()
	var got LaunchSpec
	helperLaunch = func(_ context.Context, _ string, _ string, spec LaunchSpec) (*LaunchResult, error) {
		got = spec
		cmd := exec.Command("sh", "-c", "exit 0")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return &LaunchResult{
			PID:         cmd.Process.Pid,
			Cmd:         cmd,
			StdinWriter: eofPTY{},
			Metadata:    map[string]any{"run_as_user": spec.RunAsUser},
		}, nil
	}
	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_run_as",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/true"},
		Env:           []string{"PATH=/usr/bin"},
		RunAsUser:     "striatum-lane",
	}
	body, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	var events bytes.Buffer
	if err := RunHelper(context.Background(), bytes.NewReader(append(body, '\n')), &events, HelperOptions{}); err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s", err, events.String())
	}
	if got.RunAsUser != "striatum-lane" {
		t.Fatalf("RunAsUser = %q, want striatum-lane", got.RunAsUser)
	}
	if len(got.Env) != 1 || got.Env[0] != "PATH=/usr/bin" {
		t.Fatalf("Env = %#v, want helper launch env preserved", got.Env)
	}
	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	if len(decoded) == 0 || decoded[0].Payload["metadata"] == nil {
		t.Fatalf("agent_started metadata missing from events: %#v", decoded)
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

func TestRunHelperContextCancellationEmitsTerminationDiagnostic(t *testing.T) {
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("/bin/sleep not present; skipping helper termination diagnostic test")
	}
	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_helper_cancel",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/sleep", "60"},
	}
	launchBytes, err := json.Marshal(launch)
	if err != nil {
		t.Fatalf("marshal launch: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	events := &lockedBuffer{}
	ptyOutput := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- RunHelper(ctx, bytes.NewReader(append(launchBytes, '\n')), events, HelperOptions{PTYOutput: ptyOutput})
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(events.String(), HelperEventAgentStarted) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(events.String(), HelperEventAgentStarted) {
		cancel()
		t.Fatalf("helper did not report agent_started before timeout; events=%s", events.String())
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunHelper error = %v, want context.Canceled; events=%s", err, events.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunHelper did not exit after context cancellation")
	}

	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	var terminated *HelperControlEvent
	var helperError *HelperControlEvent
	for i := range decoded {
		switch decoded[i].EventType {
		case HelperEventProcessTerminated:
			terminated = &decoded[i]
		case HelperEventError:
			helperError = &decoded[i]
		}
	}
	if terminated == nil {
		t.Fatalf("missing process_terminated event: %#v", decoded)
	}
	if terminated.Payload["phase"] != "context" || terminated.Payload["signal"] != "SIGTERM" || terminated.Payload["method"] != "process_signal" {
		t.Fatalf("process_terminated payload = %#v", terminated.Payload)
	}
	if reason, _ := terminated.Payload["reason"].(string); !strings.Contains(reason, "context canceled") {
		t.Fatalf("termination reason = %q", reason)
	}
	if helperError == nil || helperError.Payload["phase"] != "context" {
		t.Fatalf("missing context helper_error event: %#v", decoded)
	}
	if !strings.Contains(ptyOutput.String(), "## killed by supervisor: phase=context signal=SIGTERM") {
		t.Fatalf("termination diagnostic missing from PTY output: %q", ptyOutput.String())
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

func TestRunHelperAttachClientExitWithLivePaneIsNotAgentExit(t *testing.T) {
	origLaunch := helperLaunch
	defer func() { helperLaunch = origLaunch }()
	attachCmd := exec.Command("sh", "-c", "exit 0")
	if err := attachCmd.Start(); err != nil {
		t.Fatalf("start attach surrogate: %v", err)
	}
	helperLaunch = func(context.Context, string, string, LaunchSpec) (*LaunchResult, error) {
		return &LaunchResult{
			PID:         48211,
			AttachPID:   attachCmd.Process.Pid,
			Cmd:         attachCmd,
			StdinWriter: eofPTY{},
			Metadata: map[string]any{
				"tmux": map[string]any{
					"state":            "backed",
					"session_name":     "striatum-run",
					"pane_id":          "%4",
					"pane_pid":         48211,
					"pane_start_token": "1748452211",
				},
			},
		}, nil
	}

	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_attach_exit",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/true"},
	}
	body, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|0|1748452211\n"},
	}}
	var events bytes.Buffer
	if err := RunHelper(context.Background(), bytes.NewReader(append(body, '\n')), &events, HelperOptions{TmuxRunner: runner}); err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s", err, events.String())
	}
	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	seen := map[string]bool{}
	var attachPayload map[string]any
	for _, event := range decoded {
		seen[event.EventType] = true
		if event.EventType == HelperEventAttachExited {
			attachPayload = event.Payload
		}
	}
	if !seen[HelperEventAttachExited] {
		t.Fatalf("missing attach_client_exited event: %#v", decoded)
	}
	if seen[HelperEventAgentExited] {
		t.Fatalf("attach-client exit should not emit agent_exited: %#v", decoded)
	}
	delivery, ok := attachPayload["delivery_liveness"].(map[string]any)
	if !ok || delivery["class"] != "degraded" || delivery["healthy"] != false || delivery["reason"] != "attach_client_exited" {
		t.Fatalf("attach exit delivery liveness = %#v", attachPayload)
	}
}

func TestRunHelperAttachClientExitWithDeadPaneIsAgentExit(t *testing.T) {
	origLaunch := helperLaunch
	defer func() { helperLaunch = origLaunch }()
	attachCmd := exec.Command("sh", "-c", "exit 0")
	if err := attachCmd.Start(); err != nil {
		t.Fatalf("start attach surrogate: %v", err)
	}
	helperLaunch = func(context.Context, string, string, LaunchSpec) (*LaunchResult, error) {
		return &LaunchResult{
			PID:         48211,
			AttachPID:   attachCmd.Process.Pid,
			Cmd:         attachCmd,
			StdinWriter: eofPTY{},
			Metadata: map[string]any{
				"tmux": map[string]any{
					"state":            "backed",
					"session_name":     "striatum-run",
					"pane_id":          "%4",
					"pane_pid":         48211,
					"pane_start_token": "1748452211",
				},
			},
		}, nil
	}

	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_attach_exit_dead_pane",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/true"},
	}
	body, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|1|1748452211\n"},
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|1|1748452211\n"},
	}}
	var events bytes.Buffer
	if err := RunHelper(context.Background(), bytes.NewReader(append(body, '\n')), &events, HelperOptions{TmuxRunner: runner}); err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s", err, events.String())
	}
	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	var exitEvent *HelperControlEvent
	for i := range decoded {
		if decoded[i].EventType == HelperEventAgentExited {
			exitEvent = &decoded[i]
		}
		if decoded[i].EventType == HelperEventAttachExited {
			t.Fatalf("dead pane should not emit attach_client_exited: %#v", decoded)
		}
	}
	if exitEvent == nil {
		t.Fatalf("missing agent_exited event: %#v", decoded)
	}
	if exitEvent.Payload["cause"] != string(TmuxLivenessPaneDead) {
		t.Fatalf("agent_exited cause = %#v", exitEvent.Payload)
	}
}

func TestTerminateProcessTmuxBackedUsesKillSessionInsteadOfPaneSignal(t *testing.T) {
	processPIDs, panePIDs := captureHelperSignals(t)
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"kill-session"}},
	}}
	result := helperTerminateResult(222, 111, map[string]any{
		"tmux": map[string]any{
			"state":            "backed",
			"session_name":     "striatum-run",
			"pane_start_token": "123",
		},
	})

	terminateProcess(context.Background(), result, runner)

	assertIntSlice(t, "process signals", *processPIDs, []int{111})
	assertIntSlice(t, "pane signals", *panePIDs, nil)
	if len(runner.calls) != 1 || !argsPrefix(runner.calls[0], []string{"kill-session", "-t", "striatum-run"}) {
		t.Fatalf("tmux calls = %#v, want kill-session for striatum-run", runner.calls)
	}
}

func TestTerminateProcessTmuxBackedSkipsPanePIDWithoutVerifiedToken(t *testing.T) {
	processPIDs, panePIDs := captureHelperSignals(t)
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"kill-session"}, err: errors.New("can't find session")},
	}}
	result := helperTerminateResult(222, 111, map[string]any{
		"tmux": map[string]any{
			"state":            "backed",
			"session_name":     "striatum-run",
			"pane_start_token": "#{pane_start_time}",
		},
	})

	terminateProcess(context.Background(), result, runner)

	assertIntSlice(t, "process signals", *processPIDs, []int{111})
	assertIntSlice(t, "pane signals", *panePIDs, nil)
	if len(runner.calls) != 1 || !argsPrefix(runner.calls[0], []string{"kill-session", "-t", "striatum-run"}) {
		t.Fatalf("tmux calls = %#v, want attempted kill-session before skipping unverified pane pid", runner.calls)
	}
}

func TestTerminateProcessTmuxBackedSkipsPanePIDOnStartTokenMismatch(t *testing.T) {
	currentStart, ok := ProcessStartToken(os.Getpid())
	if !ok || currentStart == "" {
		t.Skip("current process start token unavailable")
	}
	processPIDs, panePIDs := captureHelperSignals(t)
	result := helperTerminateResult(os.Getpid(), 111, map[string]any{
		"tmux": map[string]any{
			"state":            "backed",
			"pane_start_token": currentStart + "1",
		},
	})

	terminateProcess(context.Background(), result, nil)

	assertIntSlice(t, "process signals", *processPIDs, []int{111})
	assertIntSlice(t, "pane signals", *panePIDs, nil)
}

func TestTerminateProcessPlainAttachSignalsChildPID(t *testing.T) {
	processPIDs, panePIDs := captureHelperSignals(t)
	result := helperTerminateResult(222, 111, nil)

	terminateProcess(context.Background(), result, nil)

	assertIntSlice(t, "process signals", *processPIDs, []int{111})
	assertIntSlice(t, "pane signals", *panePIDs, []int{222})
}

func helperTerminateResult(pid, attachPID int, metadata map[string]any) *LaunchResult {
	return &LaunchResult{
		PID:       pid,
		AttachPID: attachPID,
		Cmd:       &exec.Cmd{Process: &os.Process{Pid: attachPID}},
		Metadata:  metadata,
	}
}

func captureHelperSignals(t *testing.T) (*[]int, *[]int) {
	t.Helper()
	origSignalProcess := helperSignalProcess
	origSignalPID := helperSignalPID
	var processPIDs []int
	var panePIDs []int
	helperSignalProcess = func(process *os.Process) error {
		processPIDs = append(processPIDs, process.Pid)
		return nil
	}
	helperSignalPID = func(pid int) error {
		panePIDs = append(panePIDs, pid)
		return nil
	}
	t.Cleanup(func() {
		helperSignalProcess = origSignalProcess
		helperSignalPID = origSignalPID
	})
	return &processPIDs, &panePIDs
}

func assertIntSlice(t *testing.T, name string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %#v want %#v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %#v want %#v", name, got, want)
		}
	}
}

type eofPTY struct{}

func (eofPTY) Read([]byte) (int, error)    { return 0, io.EOF }
func (eofPTY) Write(p []byte) (int, error) { return len(p), nil }
func (eofPTY) Close() error                { return nil }

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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
