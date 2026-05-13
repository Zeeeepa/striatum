package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
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
			row, err := l.store.GetSupervisorPointer(ctx, l.supervisorID)
			if err != nil {
				continue
			}
			alive := processAliveAtStartTime(l.pid, row.StartedAt)
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

// processAlive returns true if the pid is signalable (signal-0) AND the
// kernel-reported process start time is consistent with the supervised
// row's StartedAt. Pairing the signal-0 probe with start-time validation
// closes the gemini F1 PID-recycling finding from dogfood-049 (a freshly
// reused PID can pass signal-0 while being an entirely unrelated process).
//
// If startedAt is zero (legacy callers without recorded start time) the
// check falls back to signal-0 only, preserving prior behavior.
func processAlive(pid int) bool {
	return processAliveAtStartTime(pid, time.Time{})
}

// processAliveAtStartTime is the V1.6 form: pass in the recorded
// PointerRow.StartedAt so the probe can compare the kernel-reported
// start time and refuse if they diverge by more than the tolerance.
func processAliveAtStartTime(pid int, expectedStart time.Time) bool {
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
	if expectedStart.IsZero() {
		return true
	}
	actualStart, ok := readProcessStartTime(pid)
	if !ok {
		// platforms without a reliable start-time reader fall back to
		// signal-0 only; the caller's heartbeat record is still useful
		// as a soft signal.
		return true
	}
	const tolerance = 2 * time.Second
	delta := actualStart.Sub(expectedStart)
	if delta < 0 {
		delta = -delta
	}
	return delta <= tolerance
}

// readProcessStartTime returns the kernel-reported process start time for
// pid. On Linux this reads field 22 of /proc/<pid>/stat (clock ticks since
// boot) and converts to absolute time using btime from /proc/stat and the
// CLK_TCK from sysconf (assumed 100Hz on standard kernels). Returns ok=false
// on platforms that don't have a stable reader yet; the V1.6 acceptance gate
// covers Linux explicitly.
func readProcessStartTime(pid int) (time.Time, bool) {
	if runtime.GOOS != "linux" {
		return time.Time{}, false
	}
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, false
	}
	// Format: pid (comm) state ppid ... starttime ... — comm may contain
	// spaces and parens, so anchor on the last ')'.
	endComm := strings.LastIndex(string(data), ")")
	if endComm < 0 || endComm+2 >= len(data) {
		return time.Time{}, false
	}
	fields := strings.Fields(string(data[endComm+1:]))
	// After the ')' we have: state(1) ppid(2) ... starttime is field 22
	// in the full layout, which here is index 22 - 2 = 20 (state is field
	// 3 of full, here index 0).
	const starttimeIdx = 22 - 3
	if len(fields) <= starttimeIdx {
		return time.Time{}, false
	}
	clockTicks, err := strconv.ParseUint(fields[starttimeIdx], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	// Read btime from /proc/stat (seconds since epoch when the kernel booted).
	statData, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, false
	}
	var btime int64
	for _, line := range strings.Split(string(statData), "\n") {
		if strings.HasPrefix(line, "btime ") {
			btime, err = strconv.ParseInt(strings.TrimSpace(line[len("btime "):]), 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			break
		}
	}
	if btime == 0 {
		return time.Time{}, false
	}
	const clkTck = 100 // standard sysconf(_SC_CLK_TCK) on Linux x86_64 / arm64
	seconds := int64(clockTicks) / clkTck
	return time.Unix(btime+seconds, 0).UTC(), true
}

func signalSIGTERM(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
