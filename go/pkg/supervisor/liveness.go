package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
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

var livenessTmuxRunner = DefaultTmuxRunner()

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
			if tmux := objectValue(row.Metadata["tmux"]); strings.TrimSpace(stringValue(tmux["state"])) == "backed" {
				if _, ok := TmuxIdentityFromMetadata(row.Metadata); !ok {
					_ = l.store.MarkSupervisorLost(ctx, l.supervisorID, "tmux_metadata_corrupt")
					return
				}
			}
			live := ProbeLaneLiveness(ctx, livenessTmuxRunner, row.Metadata, l.pid, row.PIDStartTime)
			row.LastHeartbeatAt = now.UTC()
			if live.Alive {
				recordTmuxProbeOK(row.Metadata, now)
				row.State = "running"
				_ = l.store.UpsertSupervisorPointer(ctx, row)
				l.mu.Lock()
				l.lastBeat = now
				l.mu.Unlock()
				continue
			}
			if live.Class == string(TmuxLivenessUnavailable) {
				count := tmuxProbeUnavailableCount(row.Metadata) + 1
				if count < TmuxUnavailableLostThreshold() {
					if row.Metadata == nil {
						row.Metadata = map[string]any{}
					}
					row.Metadata["tmux_probe_skipped_at"] = now.UTC().Format(time.RFC3339Nano)
					recordTmuxProbeUnavailable(row.Metadata, now, count, live)
					_ = l.store.UpsertSupervisorPointer(ctx, row)
					continue
				}
				_ = l.store.MarkSupervisorLost(ctx, l.supervisorID, "tmux_unavailable_persistent")
				return
			}
			reason := live.Class
			if reason == "" {
				reason = "process_exited"
			}
			_ = l.store.MarkSupervisorLost(ctx, l.supervisorID, reason)
			return
		}
	}
}

func recordTmuxProbeOK(metadata map[string]any, now time.Time) {
	tmux := objectValue(metadata["tmux"])
	if strings.TrimSpace(stringValue(tmux["state"])) != "backed" {
		return
	}
	tmux["liveness_state"] = "healthy"
	tmux["liveness"] = TmuxLivenessPayload(TmuxLiveness{Class: TmuxLivenessOK, State: "healthy", Healthy: true})
	tmux["last_ok_at"] = now.UTC().Format(time.RFC3339Nano)
	delete(tmux, "probe_skipped_at")
	delete(tmux, "probe_unavailable_count")
	delete(tmux, "last_unavailable_detail")
	metadata["tmux"] = tmux
}

func recordTmuxProbeUnavailable(metadata map[string]any, now time.Time, count int, live LaneLiveness) {
	tmux := objectValue(metadata["tmux"])
	if len(tmux) == 0 {
		return
	}
	tmux["liveness_state"] = "degraded"
	tmux["probe_skipped_at"] = now.UTC().Format(time.RFC3339Nano)
	tmux["probe_unavailable_count"] = count
	if live.Tmux != nil {
		tmux["liveness"] = TmuxLivenessPayload(*live.Tmux)
	}
	if strings.TrimSpace(live.Detail) != "" {
		tmux["last_unavailable_detail"] = strings.TrimSpace(live.Detail)
	}
	metadata["tmux"] = tmux
}

func tmuxProbeUnavailableCount(metadata map[string]any) int {
	tmux := objectValue(metadata["tmux"])
	count, ok := intValue(tmux["probe_unavailable_count"])
	if !ok || count < 0 {
		return 0
	}
	return count
}

func TmuxUnavailableLostThreshold() int {
	raw := strings.TrimSpace(os.Getenv("STRIATUM_TMUX_UNAVAILABLE_LOST_THRESHOLD"))
	if raw == "" {
		return 3
	}
	threshold, err := strconv.Atoi(raw)
	if err != nil || threshold < 1 {
		return 3
	}
	return threshold
}

// LastHeartbeat exposes the timestamp of the last successful tick. Tests use
// it to assert lost-detection latency.
func (l *Liveness) LastHeartbeat() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastBeat
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
// pid. The actual reader is per-OS: Linux reads /proc/<pid>/stat field 22 +
// /proc/stat btime; darwin uses `ps -o lstart=` (V1.7); unsupported OSes
// return ok=false so the caller falls back to signal-0 only.
func readProcessStartTime(pid int) (time.Time, bool) {
	return readProcessStartTimeOS(pid)
}

func signalSIGTERM(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
