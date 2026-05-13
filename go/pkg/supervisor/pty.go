package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// Launch starts the supervised child. UsePTY is a placeholder for the
// creack/pty integration; this minimal path uses os.Pipe + os/exec so the
// supervisor surface compiles and the FIFO/stdin path is exercised by tests
// that do not require terminal semantics.
//
// The PTY branch is intentionally returning a not-yet-wired sentinel so
// callers can detect the gap and Track A's CLI integration can probe
// capability via build tags during the V2.0 PTY landing.
func Launch(ctx context.Context, scratchDir string, supervisorID string, spec LaunchSpec) (*LaunchResult, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("supervisor: empty command")
	}
	if spec.UsePTY {
		return nil, fmt.Errorf("supervisor: PTY launch not yet wired in Go core; set USE_PTY=false or fall back to Python supervisor (RFC 0039 V1.6 follow-up)")
	}

	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = append(os.Environ(), spec.Env...)

	if err := ensureFIFO(scratchDir, supervisorID, spec.StdinPipePath); err != nil {
		return nil, err
	}

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
	return os.MkdirAll(dir, 0o755)
}

// openDevNullOr returns the file at path if non-empty, else os.DevNull.
func openDevNullOr(path string) (*os.File, error) {
	target := path
	if target == "" {
		target = os.DevNull
	}
	return os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
}
