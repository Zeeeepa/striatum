package mutations

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteToPipeReportsBufferedWhenNoReader is the #358 item-5 regression: when
// the FIFO has no reader, writeToPipe falls back to the in-memory pipeBuffers and
// returns buffered=true. That buffered=true is the signal the delivery layer uses
// to record supervisor.packet_buffered (degraded) instead of
// supervisor.packet_delivered — a daemon restart drops the in-memory queue, so a
// buffered write is NOT durably delivered.
func TestWriteToPipeReportsBufferedWhenNoReader(t *testing.T) {
	dir := t.TempDir()
	pipePath := filepath.Join(dir, "stdin.pipe")
	if err := syscall.Mkfifo(pipePath, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	// Reset the process-global buffer for this path so the test is isolated.
	pipeBuffersMu.Lock()
	delete(pipeBuffers, pipePath)
	pipeBuffersMu.Unlock()

	// No reader is attached: O_WRONLY|O_NONBLOCK open returns ENXIO, so the
	// payload is buffered rather than written to the FIFO.
	n, buffered, err := writeToPipe(context.Background(), pipePath, []byte("packet\n"))
	if err != nil {
		t.Fatalf("writeToPipe err = %v", err)
	}
	if !buffered {
		t.Fatal("writeToPipe reported buffered=false with no reader attached; the no-reader write was treated as durably delivered")
	}
	if n != len("packet\n") {
		t.Fatalf("writeToPipe n = %d, want %d", n, len("packet\n"))
	}
}
