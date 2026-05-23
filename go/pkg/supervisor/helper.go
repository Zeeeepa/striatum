package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

const defaultProgressReadSize = 4096

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
		defer packetCloser.Close()
	}

	launchSpec := LaunchSpec{
		Command:     spec.Command,
		Env:         spec.Env,
		WorkingDir:  spec.WorkingDir,
		UsePTY:      true,
		RequireTmux: spec.RequireTmux,
	}
	result, err := Launch(ctx, spec.ScratchDir, spec.SupervisorID, launchSpec)
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
	defer ptmx.Close()

	startedPayload := map[string]any{"pid": result.PID}
	if len(result.Metadata) > 0 {
		startedPayload["metadata"] = result.Metadata
	}
	if err := emitter.emit(newHelperEvent(
		HelperEventAgentStarted,
		spec.SupervisorID,
		startedPayload,
	)); err != nil {
		terminateProcess(result)
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
				terminateProcess(result)
				return emitter.helperError(spec.SupervisorID, "packet_forward", err)
			}
		case err := <-childDone:
			childDone = nil
			if packetCloser != nil {
				_ = packetCloser.Close()
			}
			drainProgress(progressDone, opts.ProgressDrainDelay)
			exitPayload := agentExitPayload(err)
			if emitErr := emitter.emit(newHelperEvent(HelperEventAgentExited, spec.SupervisorID, exitPayload)); emitErr != nil {
				return emitErr
			}
			return nil
		case <-ctx.Done():
			terminateProcess(result)
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
			if emitErr := emitter.emit(newHelperEvent(
				HelperEventProgress,
				supervisorID,
				map[string]any{
					"bytes":       n,
					"total_bytes": total,
				},
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

func processExitCode(err error) (int, bool) {
	var exitError interface {
		ExitCode() int
	}
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), true
	}
	return 0, false
}

func terminateProcess(result *LaunchResult) {
	if result == nil || result.Cmd == nil || result.Cmd.Process == nil {
		return
	}
	_ = result.Cmd.Process.Signal(syscall.SIGTERM)
}
