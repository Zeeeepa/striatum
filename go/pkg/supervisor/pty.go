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
	Extra         map[string]string // future-use metadata
}

// LaunchResult is returned by Launch. The PID is the child pid; StdinWriter
// (if non-nil) is the daemon's handle into the FIFO.
type LaunchResult struct {
	PID         int
	StdinWriter io.WriteCloser
	Cmd         *exec.Cmd
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
		return launchPTY(ctx, spec)
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
func launchPTY(ctx context.Context, spec LaunchSpec) (*LaunchResult, error) {
	runID := getEnvValue(spec.Env, "STRIATUM_RUN_ID")
	laneID := getEnvValue(spec.Env, "STRIATUM_LANE_ID")
	if runID != "" && laneID != "" {
		if _, err := exec.LookPath("tmux"); err == nil {
			sessionName := fmt.Sprintf("striatum-%s-%s", runID, laneID)
			
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
			}, nil
		}
	}

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
	}, nil
}

// openDevNullOr returns the file at path if non-empty, else os.DevNull.
func openDevNullOr(path string) (*os.File, error) {
	target := path
	if target == "" {
		target = os.DevNull
	}
	return os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}
