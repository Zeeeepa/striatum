package supervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
)

const (
	tmuxSessionNameMaxLen  = 100
	tmuxSessionNameHashLen = 12
)

// LaunchSpec describes a supervised child process. The fields mirror the
// Python wrapper protocol so the same workflow.json command lines work
// against either supervisor implementation.
type LaunchSpec struct {
	Command       []string          // exec argv
	Env           []string          // additional KEY=VAL entries (merged with os.Environ)
	WorkingDir    string            // wd for the child
	StdinPipePath string            // FIFO path; daemon writes packets here
	StdoutPath    string            // DEVNULL by default; per D028, no transcript capture
	StderrPath    string            // DEVNULL by default
	UsePTY        bool              // true → allocate a PTY (creack/pty); false → plain pipes
	RequireTmux   bool              // true → fail closed instead of falling back to a plain PTY
	Extra         map[string]string // future-use metadata
}

// LaunchResult is returned by Launch. For tmux-backed PTY launches PID is the
// pane process pid; AttachPID is the transient attach client pid used only for
// byte delivery and diagnostics.
type LaunchResult struct {
	PID         int
	StdinWriter io.WriteCloser
	Cmd         *exec.Cmd
	AttachPID   int
	Metadata    map[string]any
}

func getEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

// Launch starts the supervised child. UsePTY=true allocates a pseudo-tty
// via creack/pty and threads the master back as the daemon's stdin handle
// for packet delivery (V1.6 F-pty closure). UsePTY=false uses os.Pipe +
// os/exec so the FIFO/stdin path is exercised by tests that do not require
// terminal semantics.
func Launch(ctx context.Context, scratchDir string, supervisorID string, spec LaunchSpec) (*LaunchResult, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("supervisor: empty command")
	}

	if err := ensureFIFO(scratchDir, supervisorID, spec.StdinPipePath); err != nil {
		return nil, err
	}

	if spec.UsePTY {
		return launchPTY(ctx, supervisorID, spec)
	}

	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = mergeEnv(os.Environ(), spec.Env)

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("supervisor: stdin pipe: %w", err)
	}
	cmd.Stdin = stdinR

	cmd.Stdout, err = openDevNullOr(spec.StdoutPath)
	if err != nil {
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, err
	}
	cmd.Stderr, err = openDevNullOr(spec.StderrPath)
	if err != nil {
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, fmt.Errorf("supervisor: cmd.Start: %w", err)
	}

	return &LaunchResult{
		PID:         cmd.Process.Pid,
		StdinWriter: stdinW,
		Cmd:         cmd,
	}, nil
}

// ensureFIFO creates the supervisor's scratch dir; per D028 we do not capture
// transcripts. The FIFO itself is created by the daemon when delivering
// packets, but the supervisor scratch dir must exist for the wrapper script
// to write progress markers + pidfile.
func ensureFIFO(scratchDir string, supervisorID string, fifoPath string) error {
	dir := filepath.Join(scratchDir, supervisorID)
	return os.MkdirAll(dir, 0o700)
}

// launchPTY allocates a pseudo-tty via creack/pty (RFC 0039 V1.6 F-pty).
// pty.Start sets up the child's stdin/stdout/stderr on the slave side and
// returns the master file we hand back to the daemon as StdinWriter — the
// daemon writes packets to the master, the child reads them off the slave
// as ordinary stdin.
func launchPTY(ctx context.Context, supervisorID string, spec LaunchSpec) (*LaunchResult, error) {
	runID := getEnvValue(spec.Env, "STRIATUM_RUN_ID")
	laneID := getEnvValue(spec.Env, "STRIATUM_LANE_ID")
	if runID == "" || laneID == "" {
		if spec.RequireTmux {
			return nil, tmuxRequiredError("missing_run_or_lane")
		}
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("missing_run_or_lane"))
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		if spec.RequireTmux {
			return nil, tmuxRequiredError("tmux_not_found")
		}
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_not_found"))
	}

	sessionName := tmuxSessionName(runID, laneID, supervisorID)

	// Kill existing session with the same name if any (to avoid collisions / stale sessions)
	_ = runTmuxSetupCommand(ctx, "kill-session", "-t", sessionName)

	// 1. Create a detached tmux session with a placeholder process. The real
	// lane command is respawned only after remain-on-exit is set, so even a
	// command that exits immediately leaves a dead pane for diagnostics instead
	// of destroying the session before liveness can classify it.
	// RFC 0088 P3 follow-up: pass STRIATUM_*/PATH env vars via tmux's `-e
	// KEY=VAL` so the new session's pane child sees them. A long-running global
	// tmux server inherits its environment from FIRST launch, not from our
	// `new-session`/`respawn-pane` call's createCmd.Env — so without `-e`, the
	// pane child gets the SERVER's stale env (no STRIATUM_RUN_ID/SESSION_ID/REPO
	// etc.) and the agent-loop wrapper exits code 1 on the env check before any
	// output.
	newSessionArgs := []string{"new-session", "-d", "-s", sessionName, "-c", spec.WorkingDir}
	newSessionArgs = append(newSessionArgs, tmuxEnvArgs(spec.Env)...)
	newSessionArgs = append(newSessionArgs, "--")
	newSessionArgs = append(newSessionArgs, "sh", "-c", "while :; do sleep 3600; done")
	createCmd := exec.Command("tmux", newSessionArgs...)
	createCmd.Env = mergeEnv(os.Environ(), spec.Env)
	if err := runPreparedTmuxSetupCommand(ctx, createCmd, newSessionArgs...); err != nil {
		return nil, fmt.Errorf("supervisor: failed to create tmux session: %w", err)
	}
	cleanupTmux := true
	defer func() {
		if cleanupTmux {
			_ = runTmuxSetupCommand(context.Background(), "kill-session", "-t", sessionName)
		}
	}()

	// 2. Set tmux options before the real command is allowed to run.
	if err := runTmuxSetupCommand(ctx, "set-option", "-t", sessionName, "status", "off"); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to configure tmux status: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_setup_failed"))
	}
	if err := runTmuxSetupCommand(ctx, "set-window-option", "-t", sessionName, "remain-on-exit", "on"); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to configure tmux remain-on-exit: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_setup_failed"))
	}

	respawnArgs := []string{"respawn-pane", "-k", "-t", sessionName + ":0.0", "-c", spec.WorkingDir}
	respawnArgs = append(respawnArgs, tmuxEnvArgs(spec.Env)...)
	respawnArgs = append(respawnArgs, "--")
	respawnArgs = append(respawnArgs, spec.Command...)
	respawnCmd := exec.CommandContext(ctx, "tmux", respawnArgs...)
	respawnCmd.Env = mergeEnv(os.Environ(), spec.Env)
	if err := runPreparedTmuxSetupCommand(ctx, respawnCmd, respawnArgs...); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to respawn tmux lane command: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_respawn_failed"))
	}

	identity, err := CaptureTmuxIdentity(ctx, DefaultTmuxRunner(), sessionName)
	if err != nil || identity.WindowID == "" || identity.PaneID == "" || identity.PanePID <= 0 {
		if spec.RequireTmux {
			if err != nil {
				return nil, fmt.Errorf("supervisor: tmux identity capture failed: %w", err)
			}
			return nil, fmt.Errorf("supervisor: tmux identity capture failed")
		}
		_ = runTmuxSetupCommand(context.Background(), "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_identity_capture_failed"))
	}

	// 3. Attach to the session in the PTY
	result, err := attachTmuxPTY(ctx, identity, spec)
	if err != nil {
		return nil, err
	}
	cleanupTmux = false

	return result, nil
}

func attachTmuxPTY(ctx context.Context, identity TmuxIdentity, spec LaunchSpec) (*LaunchResult, error) {
	if strings.TrimSpace(identity.SessionName) == "" {
		return nil, fmt.Errorf("supervisor: tmux session name is required")
	}
	attachCmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", identity.SessionName)
	attachCmd.Dir = spec.WorkingDir
	attachCmd.Env = mergeEnv(os.Environ(), spec.Env)

	ptmx, err := pty.Start(attachCmd)
	if err != nil {
		return nil, fmt.Errorf("supervisor: pty.Start (tmux attach): %w", err)
	}
	return &LaunchResult{
		PID:         identity.PanePID,
		StdinWriter: ptmx,
		Cmd:         attachCmd,
		AttachPID:   attachCmd.Process.Pid,
		Metadata: map[string]any{
			"tmux": tmuxBackedMetadata(identity, attachCmd.Process.Pid),
		},
	}, nil
}

func tmuxEnvArgs(env []string) []string {
	args := make([]string, 0, len(env)*2)
	for _, entry := range env {
		args = append(args, "-e", entry)
	}
	return args
}

func runTmuxSetupCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	return runPreparedTmuxSetupCommand(ctx, cmd, args...)
}

func runPreparedTmuxSetupCommand(ctx context.Context, cmd *exec.Cmd, args ...string) error {
	timeout := tmuxSetupTimeout()
	if timeout <= 0 {
		timeout = tmuxProbeTimeout()
	}
	dir := cmd.Dir
	env := cmd.Env
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd = exec.CommandContext(runCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() != nil {
		return fmt.Errorf("tmux setup command %s timed out: %w", strings.Join(args, " "), runCtx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("tmux setup command %s failed: %s", strings.Join(args, " "), detail)
	}
	return nil
}

func tmuxSetupTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("STRIATUM_TMUX_SETUP_TIMEOUT"))
	if raw == "" {
		return tmuxProbeTimeout()
	}
	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	return tmuxProbeTimeout()
}

func launchPlainPTY(ctx context.Context, spec LaunchSpec, metadata map[string]any) (*LaunchResult, error) {
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = mergeEnv(os.Environ(), spec.Env)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("supervisor: pty.Start: %w", err)
	}
	return &LaunchResult{
		PID:         cmd.Process.Pid,
		StdinWriter: ptmx,
		Cmd:         cmd,
		Metadata:    metadata,
	}, nil
}

func tmuxSessionName(runID, laneID, supervisorID string) string {
	prefix := "striatum-" + sanitizeTmuxName(runID) + "-" + sanitizeTmuxName(laneID)
	if supervisorID != "" {
		prefix += "-" + sanitizeTmuxName(supervisorID)
	}
	hashInput := runID + "\x00" + laneID + "\x00" + supervisorID
	sum := sha256.Sum256([]byte(hashInput))
	suffix := "-" + hex.EncodeToString(sum[:])[:tmuxSessionNameHashLen]
	maxPrefix := tmuxSessionNameMaxLen - len(suffix)
	if len(prefix) > maxPrefix {
		prefix = prefix[:maxPrefix]
	}
	return prefix + suffix
}

func sanitizeTmuxName(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}

func mergeEnv(base []string, updates []string) []string {
	keys := map[string]bool{}
	for _, entry := range updates {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			keys[key] = true
		}
	}
	out := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || keys[key] {
			continue
		}
		out = append(out, entry)
	}
	return append(out, updates...)
}

func tmuxBackedMetadata(identity TmuxIdentity, attachPID int) map[string]any {
	metadata := map[string]any{
		"state":             "backed",
		"session_name":      identity.SessionName,
		"window_id":         identity.WindowID,
		"pane_id":           identity.PaneID,
		"pane_pid":          identity.PanePID,
		"attach_command":    "tmux attach-session -t " + identity.SessionName,
		"attach_client_pid": attachPID,
		"captured_at":       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if identity.PaneStartToken != "" {
		metadata["pane_start_token"] = identity.PaneStartToken
	}
	return metadata
}

func tmuxUnavailableMetadata(reason string) map[string]any {
	return map[string]any{
		"tmux": map[string]any{
			"state":              "unavailable",
			"unavailable_reason": reason,
		},
	}
}

func tmuxRequiredError(reason string) error {
	switch reason {
	case "missing_run_or_lane":
		return fmt.Errorf("supervisor: tmux required but STRIATUM_RUN_ID or STRIATUM_LANE_ID is missing")
	case "tmux_not_found":
		return fmt.Errorf("supervisor: tmux required but tmux was not found in PATH; install tmux or unset supervision.require_tmux for non-interactive lanes")
	default:
		return fmt.Errorf("supervisor: tmux required but unavailable: %s", reason)
	}
}

// openDevNullOr returns the file at path if non-empty, else os.DevNull.
func openDevNullOr(path string) (*os.File, error) {
	target := path
	if target == "" {
		target = os.DevNull
	}
	return os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}
