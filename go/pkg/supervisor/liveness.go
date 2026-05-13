package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

// LivenessConfig controls heartbeat cadence and lost-detection thresholds.
// Defaults match the Python supervisor for cross-implementation parity.
type LivenessConfig struct {
	HeartbeatInterval time.Duration // default 5s
	LostAfter         time.Duration // default 30s
	GraceOnTerm       time.Duration // default 5s — wait for clean exit after SIGTERM
}

func (c LivenessConfig) withDefaults() LivenessConfig {
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 5 * time.Second
	}
	if c.LostAfter <= 0 {
		c.LostAfter = 30 * time.Second
	}
	if c.GraceOnTerm <= 0 {
		c.GraceOnTerm = 5 * time.Second
	}
	return c
}

// Liveness drives the heartbeat goroutine and the SIGTERM cleanup path for a
// single supervisor process. It is safe for concurrent use; Stop is
// idempotent.
type Liveness struct {
	cfg          LivenessConfig
	store        PointerStore
	supervisorID string
	pid          int

	mu       sync.Mutex
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopped  bool
	lastBeat time.Time
}

// NewLiveness constructs a Liveness controller. Callers must invoke Start to
// begin the heartbeat goroutine and Stop to drain it.
func NewLiveness(cfg LivenessConfig, store PointerStore, supervisorID string, pid int) *Liveness {
	return &Liveness{
		cfg:          cfg.withDefaults(),
		store:        store,
		supervisorID: supervisorID,
		pid:          pid,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		lastBeat:     time.Now().UTC(),
	}
}

// Start launches the heartbeat goroutine. Each tick: probe the process via
// signal-0, update PointerRow.LastHeartbeatAt + State, and on lost detection
// mark the row as lost so the daemon's stale-lease scanner can recover the
// session.
func (l *Liveness) Start(ctx context.Context) {
	go l.run(ctx)
}

// Stop signals the heartbeat goroutine, optionally SIGTERMs the supervised
// process, and waits up to GraceOnTerm for clean exit before returning.
func (l *Liveness) Stop(ctx context.Context, signalProcess bool) error {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return nil
	}
	l.stopped = true
	close(l.stopCh)
	l.mu.Unlock()

	if signalProcess && l.pid > 0 {
		if err := signalSIGTERM(l.pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("supervisor: SIGTERM pid %d: %w", l.pid, err)
		}
	}

	select {
	case <-l.doneCh:
		return nil
	case <-time.After(l.cfg.GraceOnTerm):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Liveness) run(ctx context.Context) {
	defer close(l.doneCh)
	tick := time.NewTicker(l.cfg.HeartbeatInterval)
	defer tick.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ctx.Done():
			return
		case now := <-tick.C:
			alive := processAlive(l.pid)
			row, err := l.store.GetSupervisorPointer(ctx, l.supervisorID)
			if err != nil {
				continue
			}
			row.LastHeartbeatAt = now.UTC()
			if alive {
				row.State = "running"
				_ = l.store.UpsertSupervisorPointer(ctx, row)
				l.mu.Lock()
				l.lastBeat = now
				l.mu.Unlock()
			} else {
				_ = l.store.MarkSupervisorLost(ctx, l.supervisorID, "process_exited")
				return
			}
		}
	}
}

// LastHeartbeat exposes the timestamp of the last successful tick. Tests use
// it to assert lost-detection latency.
func (l *Liveness) LastHeartbeat() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastBeat
}

// processAlive returns true if the pid is signalable (signal-0). False if the
// process has exited.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func signalSIGTERM(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
