package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

type supervisionLaunchResult struct {
	PID                 int
	PIDStartTime        string
	HelperPID           int
	HelperPIDStartTime  string
	Metadata            map[string]any
	InitialHelperEvents []map[string]any
	InitialHelperOffset int
}

func launchSupervisedProcess(ctx context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath, eventPath string) (supervisionLaunchResult, error) {
	// RFC 0103 W3 (#141): a supervised helper/lane process must OUTLIVE the daemon
	// so a `systemctl restart striatumd` does not orphan it. exec.CommandContext
	// SIGKILLs the spawned child when its context is Done, and the handler context
	// is canceled on daemon shutdown — so a restart would kill the helper even with
	// the unit's KillMode=process (which only stops systemd's own cgroup kill).
	// Detach the spawn from daemon-lifetime cancellation so the helper (and its
	// tmux-backed agent) survive a restart and the daemon re-binds them on startup;
	// teardown still terminates helpers explicitly by PID (supervise stop / kill).
	ctx = context.WithoutCancel(ctx)
	if config.Transport == supervisionTransportPTYHelper {
		return launchPTYHelper(ctx, config, supervisorID, scratch, pipePath, eventPath)
	}
	return launchPipeProcess(ctx, config, supervisorID, scratch, pipePath)
}

func launchPipeProcess(ctx context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath string) (supervisionLaunchResult, error) {
	fd, err := syscall.Open(pipePath, syscall.O_RDWR, 0)
	if err != nil {
		return supervisionLaunchResult{}, fmt.Errorf("open stdin fifo: %w", err)
	}
	stdin := os.NewFile(uintptr(fd), "stdin.pipe")
	defer func() { _ = stdin.Close() }()
	laneEnv := supervisedLaneEnv(config, supervisorID)
	command := supervisedPushCommand(config, laneEnv)
	envFilePath := ""
	cleanupEnvFile := func() {}
	if strings.TrimSpace(config.RunAsUser) != "" {
		path, cleanup, err := gosupervisor.WriteLaunchEnvFile(ctx, scratch, supervisorID, config.RunAsUser, laneEnv)
		if err != nil {
			return supervisionLaunchResult{}, err
		}
		envFilePath = path
		cleanupEnvFile = cleanup
		command = gosupervisor.EnvFileWrappedCommand(path, command)
		laneEnv = nil
	}
	cmd := supervisedLaneCommandContext(ctx, command, config.RepoRoot, config.RunAsUser, laneEnv)
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		cleanupEnvFile()
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stdout.Close() }()
	stderr, err := openSupervisedStderr()
	if err != nil {
		cleanupEnvFile()
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		cleanupEnvFile()
		return supervisionLaunchResult{}, fmt.Errorf("cmd.Start: %w", err)
	}
	start, _ := processStartToken(cmd.Process.Pid)
	go func() {
		_ = cmd.Wait()
		cleanupEnvFile()
	}()
	metadata := map[string]any{}
	if config.RunAsUser != "" {
		metadata["run_as_user"] = config.RunAsUser
	}
	if envFilePath != "" {
		metadata["launch_env_file_path"] = envFilePath
	}
	return supervisionLaunchResult{PID: cmd.Process.Pid, PIDStartTime: start, Metadata: metadata}, nil
}

func supervisedPushCommand(config supervisionStartConfig, env []string) []string {
	command := append([]string(nil), config.Command...)
	if config.AgentLoopMode != agentLoopModePush || config.adapterName() != "codex" {
		return command
	}
	endpoint, err := agentloop.ResolveMCPEndpoint(config.RepoRoot, env)
	if err != nil || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(config.CapabilityToken) == "" {
		return command
	}
	return agentloop.InjectCodexMCPConfigArgs(command, config.RepoRoot, endpoint)
}

// startHelperReaper harvests a supervisor-helper child's exit status so it is
// never left as an unreapable zombie in the daemon's PID table (#204). A helper
// is spawned detached (Setsid + context.WithoutCancel per RFC 0103 W3 / #141) and
// is meant to outlive its launch RPC, so nothing on the request path ever called
// cmd.Wait(); when the helper later exited (e.g. its supervisor was marked lost
// during respawn churn) the kernel kept a <defunct> zombie until the daemon
// itself exited. Reaping the exited child does NOT affect restart survival: the
// reaper only collects the wait status AFTER the process has exited on its own —
// it never cancels the (WithoutCancel) context and never signals the process.
//
// The returned channel closes when the helper exits, so the agent_started
// handshake loops can detect an early helper exit deterministically instead of
// polling cmd.ProcessState (which the reaper now owns — concurrent reads of that
// field while cmd.Wait() writes it would be a data race).
func startHelperReaper(cmd *exec.Cmd) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return done
}

func launchPTYHelper(ctx context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath, eventPath string) (supervisionLaunchResult, error) {
	helper, err := resolveSupervisorHelper()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if err := os.WriteFile(eventPath, nil, 0o600); err != nil {
		return supervisionLaunchResult{}, err
	}
	launchSpec := gosupervisor.HelperLaunchSpec{
		SchemaVersion:   gosupervisor.HelperLaunchSchemaVersion,
		SupervisorID:    supervisorID,
		ScratchDir:      filepath.Dir(scratch),
		Command:         config.Command,
		Env:             supervisedPTYHelperSpecEnv(config, supervisorID),
		WorkingDir:      config.RepoRoot,
		RunAsUser:       config.RunAsUser,
		PacketInputPath: pipePath,
		PTYLogPath:      filepath.Join(scratch, "pty.log"),
		RequireTmux:     config.RequireTmux,
	}
	specBody, err := json.Marshal(launchSpec)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	specPath := filepath.Join(scratch, "helper-launch.json")
	if err := os.WriteFile(specPath, append(specBody, '\n'), 0o600); err != nil {
		return supervisionLaunchResult{}, err
	}
	eventFile, err := os.OpenFile(eventPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = eventFile.Close() }()
	cmd := exec.CommandContext(ctx, helper)
	cmd.Dir = config.RepoRoot
	cmd.Stdout = eventFile
	stderr, err := openSupervisedStderr()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return supervisionLaunchResult{}, err
	}
	reaped := startHelperReaper(cmd)
	if _, err := stdin.Write(append(specBody, '\n')); err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	_ = stdin.Close()
	events, offset, err := waitForHelperAgentStart(reaped, eventPath, helperStartTimeout())
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentPID, err := agentPIDFromEvents(events)
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentStart, _ := processStartToken(agentPID)
	helperStart, _ := processStartToken(cmd.Process.Pid)
	metadata := map[string]any{
		"transport":               supervisionTransportPTYHelper,
		"helper_binary":           helper,
		"helper_pid":              cmd.Process.Pid,
		"helper_pid_start_time":   helperStart,
		"helper_launch_spec_path": specPath,
		"helper_events_path":      eventPath,
	}
	if config.RunAsUser != "" {
		metadata["run_as_user"] = config.RunAsUser
	}
	if tmux := tmuxMetadataFromHelperEvents(events); tmux != nil {
		metadata["tmux"] = tmux
		if agentStart == "" {
			if token, _ := tmux["pane_start_token"].(string); token != "" {
				agentStart = token
			}
		}
	}
	return supervisionLaunchResult{
		PID:                 agentPID,
		PIDStartTime:        agentStart,
		HelperPID:           cmd.Process.Pid,
		HelperPIDStartTime:  helperStart,
		InitialHelperEvents: events,
		InitialHelperOffset: offset,
		Metadata:            metadata,
	}, nil
}

func launchRebridgeHelper(ctx context.Context, supervisor supervisorControlRow, identity gosupervisor.TmuxIdentity, eventPath string) (supervisionLaunchResult, error) {
	// RFC 0103 W3 (#141): a rebridged helper must also outlive the daemon (see
	// launchSupervisedProcess) so it is not re-orphaned by the next restart.
	ctx = context.WithoutCancel(ctx)
	helper, err := resolveSupervisorHelper()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if eventPath == "" {
		return supervisionLaunchResult{}, fmt.Errorf("helper event path is missing")
	}
	if err := os.MkdirAll(filepath.Dir(eventPath), 0o700); err != nil {
		return supervisionLaunchResult{}, err
	}
	eventFile, err := os.OpenFile(eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = eventFile.Close() }()
	startOffset, err := eventFile.Seek(0, io.SeekEnd)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	spec := gosupervisor.HelperLaunchSpec{
		SchemaVersion:   gosupervisor.HelperLaunchSchemaVersion,
		SupervisorID:    supervisor.SupervisorID,
		ScratchDir:      filepath.Dir(supervisor.ScratchPath),
		Env:             rebridgeHelperEnv(supervisor),
		WorkingDir:      supervisorWorkingDir(supervisor),
		RunAsUser:       supervisorRunAsUser(supervisor.Metadata),
		PacketInputPath: supervisor.StdinPipePath,
		PTYLogPath:      filepath.Join(filepath.Dir(eventPath), "pty.log"),
		RebridgeTmux:    &identity,
	}
	specBody, err := json.Marshal(spec)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	cmd := exec.CommandContext(ctx, helper)
	cmd.Dir = supervisorWorkingDir(supervisor)
	cmd.Stdout = eventFile
	stderr, err := openSupervisedStderr()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return supervisionLaunchResult{}, err
	}
	reaped := startHelperReaper(cmd)
	if _, err := stdin.Write(append(specBody, '\n')); err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	_ = stdin.Close()
	events, offset, err := waitForHelperAgentStartFromOffset(reaped, eventPath, helperStartTimeout(), int(startOffset))
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentPID, err := agentPIDFromEvents(events)
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	helperStart, _ := processStartToken(cmd.Process.Pid)
	metadata := map[string]any{
		"transport":             supervisionTransportPTYHelper,
		"helper_binary":         helper,
		"helper_pid":            cmd.Process.Pid,
		"helper_pid_start_time": helperStart,
		"helper_events_path":    eventPath,
	}
	if runAsUser := supervisorRunAsUser(supervisor.Metadata); runAsUser != "" {
		metadata["run_as_user"] = runAsUser
	}
	agentStart := identity.PaneStartToken
	if tmux := tmuxMetadataFromHelperEvents(events); tmux != nil {
		metadata["tmux"] = tmux
		if agentStart == "" {
			if token, _ := tmux["pane_start_token"].(string); token != "" {
				agentStart = token
			}
		}
		if attachPID, ok := intValueOptional(tmux["attach_client_pid"]); ok {
			metadata["attach_client_pid"] = attachPID
		}
	}
	return supervisionLaunchResult{
		PID:                 agentPID,
		PIDStartTime:        agentStart,
		HelperPID:           cmd.Process.Pid,
		HelperPIDStartTime:  helperStart,
		InitialHelperEvents: events,
		InitialHelperOffset: offset,
		Metadata:            metadata,
	}, nil
}

func waitForHelperAgentStartFromOffset(reaped <-chan struct{}, eventPath string, timeout time.Duration, startOffset int) ([]map[string]any, int, error) {
	deadline := time.Now().Add(timeout)
	var lastEvents []map[string]any
	lastOffset := startOffset
	for time.Now().Before(deadline) {
		events, offset, err := readHelperEventsFromFile(eventPath, startOffset)
		if err != nil {
			return nil, 0, err
		}
		if len(events) > 0 {
			lastEvents = events
			lastOffset = offset
			for _, event := range events {
				switch event["event_type"] {
				case gosupervisor.HelperEventAgentStarted:
					return events, offset, nil
				case gosupervisor.HelperEventError:
					return nil, 0, fmt.Errorf("PTY helper failed before rebridge: %v", event["payload"])
				case gosupervisor.HelperEventAgentExited:
					return nil, 0, fmt.Errorf("PTY helper agent exited before rebridge")
				}
			}
		}
		if helperExited(reaped) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("PTY helper did not report rebridge agent_started before timeout (events=%d, offset=%d)", len(lastEvents), lastOffset)
}

// helperExited reports whether the reaper goroutine has already collected the
// helper child's exit status (the channel is closed by startHelperReaper after
// cmd.Wait() returns). A non-nil channel that has not closed means the helper is
// still running; a nil channel (test callers that do not spawn a real process)
// is treated as "not exited" so the handshake loop falls through to its timeout.
func helperExited(reaped <-chan struct{}) bool {
	if reaped == nil {
		return false
	}
	select {
	case <-reaped:
		return true
	default:
		return false
	}
}

func supervisorWorkingDir(supervisor supervisorControlRow) string {
	if dir := metadataString(supervisor.Metadata["repo_root"]); dir != "" {
		return dir
	}
	if supervisor.ScratchPath != "" {
		return filepath.Dir(filepath.Dir(filepath.Dir(supervisor.ScratchPath)))
	}
	return "."
}

func tmuxMetadataFromHelperEvents(events []map[string]any) map[string]any {
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAgentStarted {
			continue
		}
		payload := asMap(event["payload"])
		metadata := asMap(payload["metadata"])
		tmux := asMap(metadata["tmux"])
		if len(tmux) > 0 {
			if lastExit := attachClientExitMetadataFromHelperEvents(events); len(lastExit) > 0 {
				tmux["attach_client_last_exit"] = lastExit
				if delivery := asMap(lastExit["delivery_liveness"]); len(delivery) > 0 {
					tmux["delivery_liveness"] = delivery
				}
			}
			return tmux
		}
	}
	return nil
}

func attachClientExitMetadataFromHelperEvents(events []map[string]any) map[string]any {
	var last map[string]any
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAttachExited {
			continue
		}
		payload := asMap(event["payload"])
		if len(payload) == 0 {
			continue
		}
		out := map[string]any{}
		if observedAt := metadataString(event["timestamp"]); observedAt != "" {
			out["observed_at"] = observedAt
		}
		if observedAt := metadataString(payload["observed_at"]); observedAt != "" {
			out["observed_at"] = observedAt
		}
		if tmuxLiveness := metadataString(payload["tmux_liveness"]); tmuxLiveness != "" {
			out["tmux_liveness"] = tmuxLiveness
		}
		if pid, ok := intValueOptional(payload["attach_client_pid"]); ok {
			out["attach_pid"] = pid
		} else if pid, ok := intValueOptional(payload["attach_pid"]); ok {
			out["attach_pid"] = pid
		}
		if exitCode, ok := intValueOptional(payload["attach_exit_code"]); ok {
			out["attach_exit_code"] = exitCode
		} else if exitCode, ok := intValueOptional(payload["exit_code"]); ok {
			out["attach_exit_code"] = exitCode
		}
		if panePID, ok := intValueOptional(payload["pid"]); ok {
			out["pane_pid"] = panePID
		}
		delivery := asMap(payload["delivery_liveness"])
		if len(delivery) == 0 && payload["delivery_degraded"] == true {
			observedAt := metadataString(out["observed_at"])
			delivery = map[string]any{
				"class":       "degraded",
				"healthy":     false,
				"reason":      "attach_client_exited",
				"observed_at": observedAt,
			}
		}
		if len(delivery) > 0 {
			out["delivery_liveness"] = delivery
		}
		last = out
	}
	if last == nil {
		return nil
	}
	return last
}

func tmuxPaneStartTokenFromMetadata(metadata map[string]any) string {
	tmux := asMap(metadata["tmux"])
	token, _ := tmux["pane_start_token"].(string)
	return token
}

func objectOrNil(value any) map[string]any {
	object := asMap(value)
	if len(object) == 0 {
		return nil
	}
	return object
}

func waitForHelperAgentStart(reaped <-chan struct{}, eventPath string, timeout time.Duration) ([]map[string]any, int, error) {
	deadline := time.Now().Add(timeout)
	var lastEvents []map[string]any
	lastOffset := 0
	for time.Now().Before(deadline) {
		events, offset, err := readHelperEventsFromFile(eventPath, 0)
		if err != nil {
			return nil, 0, err
		}
		if len(events) > 0 {
			lastEvents = events
			lastOffset = offset
			for _, event := range events {
				switch event["event_type"] {
				case gosupervisor.HelperEventAgentStarted:
					return events, offset, nil
				case gosupervisor.HelperEventError:
					return nil, 0, fmt.Errorf("PTY helper failed before attach: %v", event["payload"])
				case gosupervisor.HelperEventAgentExited:
					return nil, 0, fmt.Errorf("PTY helper agent exited before attach")
				}
			}
		}
		if helperExited(reaped) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("PTY helper did not report agent_started before timeout (events=%d, offset=%d)", len(lastEvents), lastOffset)
}

func agentPIDFromEvents(events []map[string]any) (int, error) {
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAgentStarted {
			continue
		}
		pid, ok := intValueOptional(asMap(event["payload"])["pid"])
		if !ok {
			return 0, fmt.Errorf("PTY helper did not report agent pid")
		}
		return pid, nil
	}
	return 0, fmt.Errorf("PTY helper did not report agent_started")
}

func drainHelperEvents(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, wait time.Duration) error {
	metadata, err := pointerMetadata(ctx, runner, repositoryID, supervisorID)
	if err != nil {
		return err
	}
	if metadata["transport"] != supervisionTransportPTYHelper {
		return nil
	}
	path, _ := metadata["helper_events_path"].(string)
	if path == "" {
		return nil
	}
	offset, _ := intValueOptional(metadata["helper_events_offset"])
	deadline := time.Now().Add(wait)
	var events []map[string]any
	newOffset := offset
	for {
		events, newOffset, err = readHelperEventsFromFile(path, offset)
		if err != nil {
			return err
		}
		if len(events) > 0 || time.Now().After(deadline) || wait <= 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(events) > 0 {
		for _, event := range events {
			normalized, normErr := normalizeSuperviseReportEvent(event, "", supervisorID, 0)
			if normErr != nil {
				return normErr
			}
			if _, recErr := recordSuperviseReportEvent(ctx, runner, repositoryID, normalized); recErr != nil {
				return recErr
			}
		}
	}
	if newOffset != offset {
		return mergePointerMetadata(ctx, runner, repositoryID, supervisorID, map[string]any{"helper_events_offset": newOffset})
	}
	return nil
}

func readHelperEventsFromFile(path string, offset int) ([]map[string]any, int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, offset, nil
	}
	if err != nil {
		return nil, offset, err
	}
	if offset >= len(data) {
		return nil, len(data), nil
	}
	chunk := data[offset:]
	if len(chunk) == 0 {
		return nil, offset, nil
	}
	complete := chunk
	newOffset := len(data)
	if chunk[len(chunk)-1] != '\n' {
		last := strings.LastIndexByte(string(chunk), '\n')
		if last < 0 {
			return nil, offset, nil
		}
		complete = chunk[:last+1]
		newOffset = offset + last + 1
	}
	if strings.TrimSpace(string(complete)) == "" {
		return nil, newOffset, nil
	}
	events, err := parseHelperJSONL(string(complete))
	return events, newOffset, err
}

func helperStartTimeout() time.Duration {
	raw := os.Getenv("STRIATUM_SUPERVISOR_HELPER_START_TIMEOUT")
	if raw == "" {
		return 5 * time.Second
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < 0.1 {
		return 5 * time.Second
	}
	return time.Duration(seconds * float64(time.Second))
}
