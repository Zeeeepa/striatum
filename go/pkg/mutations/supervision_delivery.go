package mutations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

type supervisorPipeNoReaderDeliveryError struct {
	supervisorID string
	metadata     map[string]any
	reason       string
}

func (e *supervisorPipeNoReaderDeliveryError) Error() string {
	return "supervisor delivery is degraded: " + e.reason
}

func (e *supervisorPipeNoReaderDeliveryError) Unwrap() error {
	return errSupervisorPipeNoReader
}

func ensureSupervisorFIFO(path string) error {
	if info, err := os.Stat(path); err == nil {
		if info.Mode()&os.ModeNamedPipe == 0 {
			return fmt.Errorf("stdin path exists but is not a FIFO: %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return supervisionMkfifo(path)
}

type supervisorDeliveryResult struct {
	BytesWritten          int
	StdinDelivery         string
	StdinClosedAfterWrite bool
}

func writeSupervisorPayload(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, pipePath string, payload []byte) (supervisorDeliveryResult, error) {
	metadata, err := pointerMetadata(ctx, runner, repositoryID, supervisorID)
	if err != nil {
		return supervisorDeliveryResult{}, err
	}
	stdinDelivery := metadataStdinDelivery(metadata)
	if stdinDelivery == stdinDeliveryOneShotEOF && metadata["stdin_delivery_consumed"] == true {
		return supervisorDeliveryResult{}, rpc.NewError("invalid_transition", "one-shot supervisor stdin has already been consumed", nil)
	}
	bytesWritten, err := writeToPipe(ctx, pipePath, payload)
	if err != nil {
		if errors.Is(err, errSupervisorPipeNoReader) {
			return supervisorDeliveryResult{}, &supervisorPipeNoReaderDeliveryError{
				supervisorID: supervisorID,
				metadata:     metadata,
				reason:       "stdin_reader_missing",
			}
		}
		return supervisorDeliveryResult{}, err
	}
	closed := stdinDelivery == stdinDeliveryOneShotEOF
	if closed {
		releaseOneShotFIFOHold(pipePath)
		_ = os.Remove(pipePath)
		if err := mergePointerMetadata(ctx, runner, repositoryID, supervisorID, map[string]any{"stdin_delivery_consumed": true}); err != nil {
			return supervisorDeliveryResult{}, err
		}
	}
	return supervisorDeliveryResult{BytesWritten: bytesWritten, StdinDelivery: stdinDelivery, StdinClosedAfterWrite: closed}, nil
}

type NamedPipeBuffer struct {
	mu       sync.Mutex
	queue    [][]byte
	degraded bool
}

func (b *NamedPipeBuffer) Push(payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.degraded {
		return fmt.Errorf("buffer is degraded")
	}
	if len(b.queue) >= 10 {
		b.degraded = true
		return fmt.Errorf("buffer overflow, degraded")
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.queue = append(b.queue, cp)
	return nil
}

func (b *NamedPipeBuffer) PopAll() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	q := b.queue
	b.queue = nil
	return q
}

func (b *NamedPipeBuffer) IsDegraded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.degraded
}

var (
	pipeBuffersMu sync.Mutex
	pipeBuffers   = make(map[string]*NamedPipeBuffer)
)

var (
	oneShotFIFOHoldsMu sync.Mutex
	oneShotFIFOHolds   = make(map[string]*os.File)
)

func getPipeBuffer(pipePath string) *NamedPipeBuffer {
	pipeBuffersMu.Lock()
	defer pipeBuffersMu.Unlock()
	buf, ok := pipeBuffers[pipePath]
	if !ok {
		buf = &NamedPipeBuffer{}
		pipeBuffers[pipePath] = buf
	}
	return buf
}

func registerOneShotFIFOHold(pipePath string, file *os.File) {
	oneShotFIFOHoldsMu.Lock()
	defer oneShotFIFOHoldsMu.Unlock()
	if existing := oneShotFIFOHolds[pipePath]; existing != nil && existing != file {
		_ = existing.Close()
	}
	oneShotFIFOHolds[pipePath] = file
}

func releaseOneShotFIFOHold(pipePath string) {
	oneShotFIFOHoldsMu.Lock()
	file := oneShotFIFOHolds[pipePath]
	delete(oneShotFIFOHolds, pipePath)
	oneShotFIFOHoldsMu.Unlock()
	if file != nil {
		_ = file.Close()
	}
}

func writeToPipe(ctx context.Context, pipePath string, payload []byte) (int, error) {
	buf := getPipeBuffer(pipePath)
	if buf.IsDegraded() {
		return 0, errSupervisorPipeNoReader
	}

	fd, err := syscall.Open(pipePath, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ENXIO) {
			if pushErr := buf.Push(payload); pushErr != nil {
				return 0, errSupervisorPipeNoReader
			}
			return len(payload), nil
		}
		return 0, err
	}
	file := os.NewFile(uintptr(fd), pipePath)
	defer func() { _ = file.Close() }()

	buffered := buf.PopAll()
	for _, pkt := range buffered {
		if _, err := writeAll(ctx, file, pkt); err != nil {
			return 0, err
		}
	}

	return writeAll(ctx, file, payload)
}

func writeAll(ctx context.Context, file *os.File, payload []byte) (int, error) {
	total := 0
	for total < len(payload) {
		n, err := file.Write(payload[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return total, rpc.NewError("invalid_transition", "supervisor pipe is broken; child has closed stdin", nil)
			}
			if errors.Is(err, syscall.EAGAIN) {
				select {
				case <-ctx.Done():
					return total, ctx.Err()
				case <-time.After(20 * time.Millisecond):
					continue
				}
			}
			return total, err
		}
		if n == 0 {
			return total, rpc.NewError("invalid_transition", "supervisor pipe write returned zero bytes", nil)
		}
	}
	return total, nil
}

func markPointerDeliveryDegraded(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, metadata map[string]any, reason string) error {
	updated := map[string]any{}
	for key, value := range metadata {
		updated[key] = value
	}
	delivery := map[string]any{
		"class":       "degraded",
		"healthy":     false,
		"reason":      reason,
		"observed_at": nowString(),
	}
	if tmux := asMap(updated["tmux"]); len(tmux) > 0 {
		tmux["delivery_liveness"] = delivery
		updated["tmux"] = tmux
	} else {
		updated["delivery_liveness"] = delivery
	}
	return mergePointerMetadata(ctx, runner, repositoryID, supervisorID, updated)
}

func metadataStdinDelivery(metadata map[string]any) string {
	value, _ := metadata["stdin_delivery"].(string)
	if value == stdinDeliveryOneShotEOF || value == stdinDeliveryPersistentFIFO {
		return value
	}
	return stdinDeliveryPersistentFIFO
}
