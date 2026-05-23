package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
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

// LaunchResult is returned by Launch. The PID is the child pid; StdinWriter
// (if non-nil) is the daemon's handle into the FIFO.
type LaunchResult struct {
	PID         int
	StdinWriter io.WriteCloser
	Cmd         *exec.Cmd
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
	cmd.Env = append(os.Environ(), spec.Env...)

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
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// 1. Create the detached tmux session
	newSessionArgs := []string{"new-session", "-d", "-s", sessionName, "-c", spec.WorkingDir}
	newSessionArgs = append(newSessionArgs, spec.Command...)
	createCmd := exec.Command("tmux", newSessionArgs...)
	createCmd.Env = append(os.Environ(), spec.Env...)
	if err := createCmd.Run(); err != nil {
		return nil, fmt.Errorf("supervisor: failed to create tmux session: %w", err)
	}

	// 2. Disable status bar to avoid polluting stdout
	_ = exec.Command("tmux", "set-option", "-t", sessionName, "status", "off").Run()

	// 3. Attach to the session in the PTY
	attachCmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", sessionName)
	attachCmd.Dir = spec.WorkingDir
	attachCmd.Env = append(os.Environ(), spec.Env...)

	ptmx, err := pty.Start(attachCmd)
	if err != nil {
		return nil, fmt.Errorf("supervisor: pty.Start (tmux attach): %w", err)
	}

	return &LaunchResult{
		PID:         attachCmd.Process.Pid,
		StdinWriter: ptmx,
		Cmd:         attachCmd,
		Metadata: map[string]any{
			"tmux": tmuxAttachMetadata(sessionName),
		},
	}, nil
}

func launchPlainPTY(ctx context.Context, spec LaunchSpec, metadata map[string]any) (*LaunchResult, error) {
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = append(os.Environ(), spec.Env...)
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
	name := "striatum-" + sanitizeTmuxName(runID) + "-" + sanitizeTmuxName(laneID)
	if supervisorID != "" {
		name += "-" + sanitizeTmuxName(supervisorID)
	}
	if len(name) > 100 {
		return name[:100]
	}
	return name
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

func tmuxAttachMetadata(sessionName string) map[string]any {
	metadata := map[string]any{
		"session_name":   sessionName,
		"attach_command": "tmux attach-session -t " + sessionName,
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", sessionName, "#{window_id} #{pane_id}").Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 1 {
			metadata["window_id"] = parts[0]
		}
		if len(parts) >= 2 {
			metadata["pane_id"] = parts[1]
		}
	}
	return metadata
}

func tmuxUnavailableMetadata(reason string) map[string]any {
	return map[string]any{
		"tmux": map[string]any{
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
