package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultProgressReadSize = 4096

var (
	helperLaunch        = Launch
	helperSignalProcess = func(process *os.Process) error {
		return process.Signal(syscall.SIGTERM)
	}
	helperSignalPID = func(pid int) error {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return proc.Signal(syscall.SIGTERM)
	}
)

type readWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

// HelperOptions contains test hooks and conservative lifecycle knobs for
// RunHelper. Production callers should normally pass the zero value.
type HelperOptions struct {
	PTYOutput          io.Writer
	ProgressReadSize   int
	StdinEOFClosesPTY  bool
	ProgressDrainDelay time.Duration
	TmuxRunner         TmuxRunner
}

type helperEmitter struct {
	mu sync.Mutex
	w  io.Writer
}

func newHelperEmitter(w io.Writer) *helperEmitter {
	return &helperEmitter{w: w}
}

func (e *helperEmitter) emit(event HelperControlEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return encodeHelperEvent(e.w, event)
}

func (e *helperEmitter) helperError(supervisorID string, phase string, err error) error {
	payload := map[string]any{
		"phase": phase,
		"error": err.Error(),
	}
	_ = e.emit(newHelperEvent(HelperEventError, supervisorID, payload))
	return err
}

// RunHelper reads exactly one launch JSON object from launchReader, starts
// the command under a PTY, forwards packet frames from either the launch
// stream remainder or packet_input_path, and emits JSONL lifecycle events.
//
// It deliberately does not open Postgres, call daemon RPC, inspect workflow
// state, publish artifacts, complete jobs, or acknowledge work. The Python
// daemon remains the authority for every state transition; this helper only
// moves process bytes and reports control events.
func RunHelper(ctx context.Context, launchReader io.Reader, eventWriter io.Writer, opts HelperOptions) error {
	emitter := newHelperEmitter(eventWriter)
	decoder := json.NewDecoder(launchReader)
	spec, err := decodeHelperLaunchSpec(decoder)
	if err != nil {
		return emitter.helperError("", "decode_launch", err)
	}

	packetReader, packetCloser, stdinPackets, err := packetInput(decoder, launchReader, spec.PacketInputPath)
	if err != nil {
		return emitter.helperError(spec.SupervisorID, "packet_input", err)
	}
	if packetCloser != nil {
		defer func() { _ = packetCloser.Close() }()
	}

	launchSpec := LaunchSpec{
		Command:     spec.Command,
		Env:         spec.Env,
		WorkingDir:  spec.WorkingDir,
		RunAsUser:   spec.RunAsUser,
		UsePTY:      true,
		RequireTmux: spec.RequireTmux,
	}
	var result *LaunchResult
	if spec.RebridgeTmux != nil {
		result, err = attachTmuxPTY(ctx, *spec.RebridgeTmux, launchSpec)
	} else {
		result, err = helperLaunch(ctx, spec.ScratchDir, spec.SupervisorID, launchSpec)
	}
	if err != nil {
		return emitter.helperError(spec.SupervisorID, "launch", err)
	}
	if result == nil || result.Cmd == nil || result.StdinWriter == nil {
		return emitter.helperError(spec.SupervisorID, "launch", fmt.Errorf("launch returned incomplete result"))
	}
	ptmx, ok := result.StdinWriter.(readWriteCloser)
	if !ok {
		_ = result.StdinWriter.Close()
		return emitter.helperError(spec.SupervisorID, "launch", fmt.Errorf("PTY handle is not read/write/close capable"))
	}
	defer func() { _ = ptmx.Close() }()

	startedPayload := map[string]any{"pid": result.PID}
	if spec.RebridgeTmux != nil {
		startedPayload["rebridge"] = true
	}
	if result.AttachPID > 0 {
		startedPayload["attach_client_pid"] = result.AttachPID
	}
	if len(result.Metadata) > 0 {
		startedPayload["metadata"] = result.Metadata
	}
	if err := emitter.emit(newHelperEvent(
		HelperEventAgentStarted,
		spec.SupervisorID,
		startedPayload,
	)); err != nil {
		terminateProcess(ctx, result, opts.TmuxRunner)
		return err
	}

	childDone := make(chan error, 1)
	go func() {
		childDone <- result.Cmd.Wait()
	}()

	progressDone := make(chan error, 1)
	go func() {
		progressDone <- pumpPTYProgress(ctx, ptmx, opts.PTYOutput, emitter, spec.SupervisorID, opts.ProgressReadSize)
	}()

	packetDone := make(chan error, 1)
	go func() {
		packetDone <- forwardPacketStream(ctx, packetReader, ptmx, emitter, spec.SupervisorID, stdinPackets || opts.StdinEOFClosesPTY)
	}()

	for packetDone != nil || childDone != nil {
		select {
		case err := <-packetDone:
			packetDone = nil
			if err != nil {
				reportProcessTermination(ctx, result, opts, emitter, spec.SupervisorID, "packet_forward", err)
				return emitter.helperError(spec.SupervisorID, "packet_forward", err)
			}
		case err := <-childDone:
			childDone = nil
			if packetCloser != nil {
				_ = packetCloser.Close()
			}
			drainProgress(progressDone, opts.ProgressDrainDelay)
			if attachPayload, ok := attachClientExitPayload(ctx, result, err, opts.TmuxRunner); ok {
				if emitErr := emitter.emit(newHelperEvent(HelperEventAttachExited, spec.SupervisorID, attachPayload)); emitErr != nil {
					return emitErr
				}
			} else {
				exitPayload := agentExitPayload(err)
				if attachCause := tmuxExitCause(ctx, result, opts.TmuxRunner); attachCause != "" {
					exitPayload["cause"] = attachCause
				}
				if emitErr := emitter.emit(newHelperEvent(HelperEventAgentExited, spec.SupervisorID, exitPayload)); emitErr != nil {
					return emitErr
				}
			}
			return nil
		case <-ctx.Done():
			reportProcessTermination(ctx, result, opts, emitter, spec.SupervisorID, "context", ctx.Err())
			return emitter.helperError(spec.SupervisorID, "context", ctx.Err())
		}
	}
	return nil
}

func packetInput(decoder *json.Decoder, launchReader io.Reader, path string) (io.Reader, io.Closer, bool, error) {
	if path != "" && path != "-" {
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return nil, nil, false, fmt.Errorf("open packet input %q: %w", path, err)
		}
		return file, file, false, nil
	}
	buffered := decoder.Buffered()
	if buffered == nil {
		return launchReader, nil, true, nil
	}
	return io.MultiReader(buffered, launchReader), nil, true, nil
}

func forwardPacketStream(
	ctx context.Context,
	r io.Reader,
	w io.Writer,
	emitter *helperEmitter,
	supervisorID string,
	closeOnEOF bool,
) error {
	reader := bufio.NewReader(r)
	sequence := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		frame, err := reader.ReadBytes('\n')
		if len(frame) > 0 {
			sequence++
			if _, writeErr := writeFull(w, frame); writeErr != nil {
				return writeErr
			}
			if emitErr := emitter.emit(newHelperEvent(
				HelperEventPacketAccepted,
				supervisorID,
				map[string]any{
					"bytes":    len(frame),
					"sequence": sequence,
				},
			)); emitErr != nil {
				return emitErr
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if closeOnEOF {
				_, _ = w.Write([]byte{4})
			}
			return nil
		}
		return fmt.Errorf("read packet stream: %w", err)
	}
}

func writeFull(w io.Writer, payload []byte) (int, error) {
	total := 0
	for total < len(payload) {
		n, err := w.Write(payload[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func pumpPTYProgress(
	ctx context.Context,
	r io.Reader,
	output io.Writer,
	emitter *helperEmitter,
	supervisorID string,
	readSize int,
) error {
	if output == nil {
		output = io.Discard
	}
	if readSize <= 0 {
		readSize = defaultProgressReadSize
	}
	buf := make([]byte, readSize)
	total := 0
	// RFC 0101 Phase 1: the meter watches output VOLUME (not content, per D028)
	// and flags a progress event as meaningful when the lane has produced enough
	// output within a window to count as real work rather than TUI redraw noise.
	// The daemon refreshes the active lease work-heartbeat on a meaningful
	// progress event, so honest local work between MCP calls no longer trips
	// agent_lease_heartbeat_stall (#80 / #136).
	meter := newProgressMeter()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, err := r.Read(buf)
		if n > 0 {
			total += n
			if _, writeErr := output.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			payload := map[string]any{
				"bytes":       n,
				"total_bytes": total,
			}
			if meter.observe(n, time.Now()) {
				payload["meaningful"] = true
			}
			if emitErr := emitter.emit(newHelperEvent(
				HelperEventProgress,
				supervisorID,
				payload,
			)); emitErr != nil {
				return emitErr
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EIO) {
			return nil
		}
		return fmt.Errorf("read PTY progress: %w", err)
	}
}

func drainProgress(done <-chan error, delay time.Duration) {
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}
	select {
	case <-done:
	case <-time.After(delay):
	}
}

func agentExitPayload(err error) map[string]any {
	payload := map[string]any{}
	if err == nil {
		payload["exit_code"] = 0
		return payload
	}
	if exitCode, ok := processExitCode(err); ok {
		payload["exit_code"] = exitCode
	} else {
		payload["error"] = err.Error()
	}
	return payload
}

func attachClientExitPayload(ctx context.Context, result *LaunchResult, err error, runner TmuxRunner) (map[string]any, bool) {
	if result == nil || result.AttachPID <= 0 {
		return nil, false
	}
	live := ProbeLaneLiveness(ctx, runnerOrDefault(runner), result.Metadata, result.PID, "")
	if live.Backed != "tmux" {
		return nil, false
	}
	if live.Class != string(TmuxLivenessOK) && live.Class != string(TmuxLivenessUnavailable) {
		return nil, false
	}
	payload := map[string]any{
		"pid":               result.PID,
		"attach_pid":        result.AttachPID,
		"attach_client_pid": result.AttachPID,
		"delivery_degraded": true,
		"delivery_liveness": map[string]any{
			"class":   "degraded",
			"healthy": false,
			"reason":  "attach_client_exited",
		},
		"observed_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"tmux_liveness": live.Class,
	}
	if err == nil {
		payload["attach_exit_code"] = 0
		payload["exit_code"] = 0
	} else {
		if exitCode, ok := processExitCode(err); ok {
			payload["attach_exit_code"] = exitCode
			payload["exit_code"] = exitCode
		} else {
			payload["attach_error"] = err.Error()
		}
	}
	return payload, true
}

func tmuxExitCause(ctx context.Context, result *LaunchResult, runner TmuxRunner) string {
	if result == nil || result.AttachPID <= 0 {
		return ""
	}
	live := ProbeLaneLiveness(ctx, runnerOrDefault(runner), result.Metadata, result.PID, "")
	if live.Backed != "tmux" {
		return ""
	}
	return live.Class
}

func runnerOrDefault(runner TmuxRunner) TmuxRunner {
	if runner != nil {
		return runner
	}
	return DefaultTmuxRunner()
}

func processExitCode(err error) (int, bool) {
	var exitError interface {
		ExitCode() int
	}
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), true
	}
	return 0, false
}

func reportProcessTermination(
	ctx context.Context,
	result *LaunchResult,
	opts HelperOptions,
	emitter *helperEmitter,
	supervisorID string,
	phase string,
	cause error,
) {
	payload := processTerminationPayload(result, phase, cause)
	_ = emitter.emit(newHelperEvent(HelperEventProcessTerminated, supervisorID, payload))
	writeTerminationDiagnostic(opts.PTYOutput, payload)
	terminateProcess(ctx, result, opts.TmuxRunner)
}

func processTerminationPayload(result *LaunchResult, phase string, cause error) map[string]any {
	payload := map[string]any{
		"phase":  phase,
		"signal": "SIGTERM",
		"method": "process_signal",
	}
	if cause != nil {
		payload["reason"] = cause.Error()
	}
	if result == nil {
		return payload
	}
	if result.PID > 0 {
		payload["pid"] = result.PID
	}
	if result.AttachPID > 0 {
		payload["attach_pid"] = result.AttachPID
	}
	if tmux := objectValue(result.Metadata["tmux"]); strings.TrimSpace(stringValue(tmux["state"])) == "backed" {
		if sessionName := strings.TrimSpace(stringValue(tmux["session_name"])); sessionName != "" {
			payload["method"] = "tmux_kill_session"
			payload["tmux_session_name"] = sessionName
		}
	}
	return payload
}

func writeTerminationDiagnostic(output io.Writer, payload map[string]any) {
	if output == nil {
		return
	}
	phase := strings.TrimSpace(stringValue(payload["phase"]))
	if phase == "" {
		phase = "unknown"
	}
	signal := strings.TrimSpace(stringValue(payload["signal"]))
	if signal == "" {
		signal = "unknown"
	}
	reason := strings.TrimSpace(stringValue(payload["reason"]))
	if reason == "" {
		reason = "unspecified"
	}
	_, _ = fmt.Fprintf(output, "\r\n## killed by supervisor: phase=%s signal=%s reason=%s\r\n", phase, signal, reason)
}

func terminateProcess(ctx context.Context, result *LaunchResult, runner TmuxRunner) {
	if result == nil || result.Cmd == nil || result.Cmd.Process == nil {
		return
	}
	_ = helperSignalProcess(result.Cmd.Process)
	if result.AttachPID > 0 && result.AttachPID != result.PID {
		if terminateTmuxBackedPane(ctx, result, runner) {
			return
		}
		_ = helperSignalPID(result.PID)
	}
}

func terminateTmuxBackedPane(ctx context.Context, result *LaunchResult, runner TmuxRunner) bool {
	tmux := objectValue(result.Metadata["tmux"])
	if strings.TrimSpace(stringValue(tmux["state"])) != "backed" {
		return false
	}
	sessionName := strings.TrimSpace(stringValue(tmux["session_name"]))
	if sessionName != "" {
		killCtx, cancel := context.WithTimeout(context.Background(), tmuxProbeTimeout())
		if ctx != nil && ctx.Err() == nil {
			killCtx, cancel = context.WithTimeout(ctx, tmuxProbeTimeout())
		}
		_, err := runnerOrDefault(runner).Run(killCtx, "kill-session", "-t", sessionName)
		cancel()
		if err == nil {
			return true
		}
	}
	token := verifiedStartToken(stringValue(tmux["pane_start_token"]))
	if token == "" {
		return true
	}
	alive, _, _ := PIDLiveWithStartToken(result.PID, token)
	return !alive
}
