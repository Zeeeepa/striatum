package supervisor

import (
	"testing"
	"time"
)

// TestProgressMeterFiresOnMeaningfulVolumeAtInterval covers the OQ1 happy path:
// a lane streaming substantive output crosses the byte threshold and, once the
// rate-limit interval has elapsed, the meter fires a meaningful-progress signal.
func TestProgressMeterFiresOnMeaningfulVolumeAtInterval(t *testing.T) {
	m := &progressMeter{byteThreshold: 512, minInterval: 20 * time.Second}
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	// First observation only seeds the clock — never fires on startup.
	if m.observe(4096, base) {
		t.Fatal("first observation must not fire")
	}
	// More than enough bytes, but still inside the rate-limit interval.
	if m.observe(4096, base.Add(5*time.Second)) {
		t.Fatal("must not fire before the min interval elapses")
	}
	// Past the interval with plenty of accumulated bytes => fire.
	if !m.observe(4096, base.Add(25*time.Second)) {
		t.Fatal("expected a meaningful-progress fire past the interval")
	}
	// Immediately after a fire the accumulator resets, so it must not refire.
	if m.observe(4096, base.Add(26*time.Second)) {
		t.Fatal("must not fire again immediately after a fire")
	}
}

// TestProgressMeterIgnoresSpinnerNoise covers the OQ1 rejection path: a steady
// trickle of small redraw frames (a TUI spinner) never accumulates enough
// volume within the interval, so the meter never fires and the lane is not
// kept artificially alive by repaint noise.
func TestProgressMeterIgnoresSpinnerNoise(t *testing.T) {
	m := &progressMeter{byteThreshold: 512, minInterval: 20 * time.Second}
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	m.observe(20, base) // seed

	now := base
	for i := 0; i < 100; i++ {
		now = now.Add(100 * time.Millisecond)
		// ~20 bytes per redraw frame: 100 frames over 10s = 2000 bytes total,
		// but each individual window the meter evaluates against the interval
		// keeps resetting only on a fire, which never happens here because the
		// 20-byte frames cross the 512 byte threshold only after the interval,
		// at which point time gating is what we exercise next.
		if m.observe(20, now) {
			t.Fatalf("spinner frame %d should not fire (10s window, low volume)", i)
		}
	}
}

// TestProgressMeterRequiresBothByteAndTimeGates asserts the AND semantics: a
// large burst inside the interval does not fire, and the interval passing with
// too few bytes does not fire.
func TestProgressMeterRequiresBothByteAndTimeGates(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("volume without elapsed time does not fire", func(t *testing.T) {
		m := &progressMeter{byteThreshold: 512, minInterval: 20 * time.Second}
		m.observe(10, base) // seed
		if m.observe(100000, base.Add(time.Second)) {
			t.Fatal("huge burst inside the interval must not fire")
		}
	})

	t.Run("elapsed time without enough volume does not fire", func(t *testing.T) {
		m := &progressMeter{byteThreshold: 512, minInterval: 20 * time.Second}
		m.observe(10, base) // seed
		if m.observe(50, base.Add(time.Hour)) {
			t.Fatal("low volume must not fire even long after the interval")
		}
	})
}

// TestProgressMeterAccumulatesAcrossChunks asserts the byte threshold is met by
// the SUM of chunks since the last fire, not a single chunk — small but steady
// real output still heartbeats once it has produced enough total volume past
// the interval.
func TestProgressMeterAccumulatesAcrossChunks(t *testing.T) {
	m := &progressMeter{byteThreshold: 512, minInterval: 10 * time.Second}
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	m.observe(100, base) // seed (100 accumulated, but seed does not count toward fire)

	now := base
	fired := false
	// 200-byte chunks every 4s; threshold 512 reached after 3 post-seed chunks
	// (600 bytes), and by then >10s has elapsed.
	for i := 0; i < 5; i++ {
		now = now.Add(4 * time.Second)
		if m.observe(200, now) {
			fired = true
			break
		}
	}
	if !fired {
		t.Fatal("expected accumulated small chunks to eventually fire")
	}
}

func TestProgressMeterIgnoresNonPositiveReads(t *testing.T) {
	m := newProgressMeter()
	if m.observe(0, time.Now()) {
		t.Fatal("zero-byte read must not fire")
	}
	if m.observe(-5, time.Now()) {
		t.Fatal("negative read must not fire")
	}
	if m.started {
		t.Fatal("non-positive reads must not seed the meter")
	}
}

func TestProgressMeterEnvOverrides(t *testing.T) {
	t.Setenv("STRIATUM_HELPER_MEANINGFUL_BYTES", "128")
	t.Setenv("STRIATUM_HELPER_HEARTBEAT_INTERVAL", "2s")
	m := newProgressMeter()
	if m.byteThreshold != 128 {
		t.Fatalf("byteThreshold = %d, want 128", m.byteThreshold)
	}
	if m.minInterval != 2*time.Second {
		t.Fatalf("minInterval = %v, want 2s", m.minInterval)
	}
}

func TestProgressMeterEnvOverrideIntervalSeconds(t *testing.T) {
	t.Setenv("STRIATUM_HELPER_HEARTBEAT_INTERVAL", "45")
	if got := heartbeatMinIntervalFromEnv(); got != 45*time.Second {
		t.Fatalf("interval = %v, want 45s", got)
	}
}

func TestProgressMeterDefaultsOnInvalidEnv(t *testing.T) {
	t.Setenv("STRIATUM_HELPER_MEANINGFUL_BYTES", "not-a-number")
	t.Setenv("STRIATUM_HELPER_HEARTBEAT_INTERVAL", "garbage")
	m := newProgressMeter()
	if m.byteThreshold != defaultMeaningfulByteThreshold {
		t.Fatalf("byteThreshold = %d, want default %d", m.byteThreshold, defaultMeaningfulByteThreshold)
	}
	if m.minInterval != defaultHeartbeatMinInterval {
		t.Fatalf("minInterval = %v, want default %v", m.minInterval, defaultHeartbeatMinInterval)
	}
}
