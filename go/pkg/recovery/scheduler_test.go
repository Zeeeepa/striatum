package recovery

import (
	"context"
	"testing"
	"time"
)

func TestRunSchedulerRunsImmediateSweepThenWaitsForInterval(t *testing.T) {
	maxSweeps := 2
	calls := 0
	waits := []time.Duration{}

	result, err := RunScheduler(context.Background(), SchedulerOptions{
		Interval:    25 * time.Millisecond,
		MinInterval: time.Millisecond,
		MaxSweeps:   &maxSweeps,
		SweepOnce: func(context.Context) (map[string]any, error) {
			calls++
			return map[string]any{"call": calls}, nil
		},
		Wait: func(ctx context.Context, interval time.Duration) bool {
			waits = append(waits, interval)
			return true
		},
	})
	if err != nil {
		t.Fatalf("RunScheduler returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 sweeps, got %d", calls)
	}
	if len(waits) != 1 || waits[0] != 25*time.Millisecond {
		t.Fatalf("expected one interval wait of 25ms, got %#v", waits)
	}
	if result.Sweeps != 2 || result.Reason != ReasonMaxSweepsReached {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunSchedulerMaxSweepsZeroStillRunsOneStartupSweep(t *testing.T) {
	maxSweeps := 0
	calls := 0
	waitCalled := false

	result, err := RunScheduler(context.Background(), SchedulerOptions{
		Interval:  25 * time.Millisecond,
		MaxSweeps: &maxSweeps,
		SweepOnce: func(context.Context) (map[string]any, error) {
			calls++
			return nil, nil
		},
		Wait: func(ctx context.Context, interval time.Duration) bool {
			waitCalled = true
			return true
		},
	})
	if err != nil {
		t.Fatalf("RunScheduler returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one startup sweep for max_sweeps=0, got %d", calls)
	}
	if waitCalled {
		t.Fatalf("scheduler waited after max_sweeps=0 startup sweep")
	}
	if result.Sweeps != 1 || result.Reason != ReasonMaxSweepsReached {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEffectiveIntervalFloorsToPythonMinimum(t *testing.T) {
	minimum := 100 * time.Millisecond
	for _, interval := range []time.Duration{-time.Second, 0, time.Millisecond} {
		if got := EffectiveInterval(interval, minimum); got != minimum {
			t.Fatalf("EffectiveInterval(%s) = %s, want %s", interval, got, minimum)
		}
	}
	if got := EffectiveInterval(2*time.Second, minimum); got != 2*time.Second {
		t.Fatalf("EffectiveInterval above minimum = %s", got)
	}
}
