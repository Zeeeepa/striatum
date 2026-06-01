package supervisor

import (
	"os"
	"strconv"
	"time"
)

// RFC 0101 Phase 1 (Layer 1, OQ1): the supervisor helper is the daemon's only
// window onto whether a supervised lane is actually producing output. A lane
// doing long LOCAL work — reading and editing files between MCP calls — emits
// no protocol heartbeat, so the daemon's protocol-only classifier wrongly trips
// agent_lease_heartbeat_stall on a lane that is demonstrably alive and working
// (#80 / #136).
//
// The fix observed here is migration-free: the helper watches PTY OUTPUT VOLUME
// (D028 — volume/timing only, never content) and, when it sees a *meaningful*
// amount of output within a window, tags the progress event so the daemon can
// refresh the active lease's work-heartbeat. progressMeter encapsulates the OQ1
// threshold decision so it is pure and unit-testable.
//
// OQ1 — distinguishing real output from spinner/redraw noise. A TUI repaints
// constantly: a spinner emits a small, steady trickle of control bytes
// (cursor moves + one glyph per frame, on the order of tens of bytes every
// ~100ms). Real output (a file dumped to the pane, a tool result, generated
// prose) arrives as a much larger byte volume. The meter therefore fires only
// when BOTH hold since the last fire:
//
//   - accumulated bytes >= meaningfulByteThreshold  (substance, not a redraw)
//   - elapsed time       >= heartbeatMinInterval     (rate-limit; one heartbeat
//     per interval at most, so a busy pane cannot spam the daemon)
//
// A pure spinner never accumulates meaningfulByteThreshold bytes within
// heartbeatMinInterval, so it never fires; a lane genuinely streaming output
// crosses the byte threshold quickly and heartbeats at the interval cadence.
const (
	// defaultMeaningfulByteThreshold is the minimum number of PTY output bytes
	// that must accumulate since the last heartbeat for the output to count as
	// meaningful progress. Sized well above a few TUI redraw frames but below a
	// single substantive chunk of real output.
	defaultMeaningfulByteThreshold = 512

	// defaultHeartbeatMinInterval rate-limits meaningful-progress heartbeats so
	// an actively streaming lane refreshes its lease about every interval at
	// most, regardless of how much it writes. Comfortably under the
	// LeaseHeartbeatSeconds deadline (300s) so honest local work never stalls.
	defaultHeartbeatMinInterval = 20 * time.Second
)

// progressMeter accumulates observed PTY output volume and decides, per chunk,
// whether a meaningful-output boundary has been crossed since the last
// heartbeat. It is intentionally allocation-free and holds no output content.
type progressMeter struct {
	byteThreshold int
	minInterval   time.Duration

	bytesSinceFire int
	lastFire       time.Time
	started        bool
}

func newProgressMeter() *progressMeter {
	return &progressMeter{
		byteThreshold: meaningfulByteThresholdFromEnv(),
		minInterval:   heartbeatMinIntervalFromEnv(),
	}
}

// observe records that n output bytes were written to the PTY at time now and
// reports whether this observation crosses the meaningful-progress boundary.
// When it returns true the caller should tag the emitted progress event so the
// daemon refreshes the active lease work-heartbeat.
//
// The first observation seeds the interval clock without firing: a lane that
// has only just started producing output is not yet a heartbeat candidate, and
// the protocol-discovery rungs already cover the startup window.
func (m *progressMeter) observe(n int, now time.Time) bool {
	if n <= 0 {
		return false
	}
	if !m.started {
		m.started = true
		m.lastFire = now
		m.bytesSinceFire = n
		return false
	}
	m.bytesSinceFire += n
	if m.bytesSinceFire < m.byteThreshold {
		return false
	}
	if now.Sub(m.lastFire) < m.minInterval {
		return false
	}
	m.lastFire = now
	m.bytesSinceFire = 0
	return true
}

func meaningfulByteThresholdFromEnv() int {
	if raw := os.Getenv("STRIATUM_HELPER_MEANINGFUL_BYTES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultMeaningfulByteThreshold
}

func heartbeatMinIntervalFromEnv() time.Duration {
	if raw := os.Getenv("STRIATUM_HELPER_HEARTBEAT_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultHeartbeatMinInterval
}
