package recovery

import (
	"context"
	"errors"
	"time"
)

const (
	DefaultSweepInterval = 60 * time.Second
	DefaultMinInterval   = 100 * time.Millisecond

	ReasonContextCanceled  = "context_canceled"
	ReasonMaxSweepsReached = "max_sweeps_reached"
)

type SweepOnceFunc func(context.Context) (map[string]any, error)

type WaitFunc func(context.Context, time.Duration) bool

type SchedulerOptions struct {
	Interval     time.Duration
	MinInterval  time.Duration
	MaxSweeps    *int
	SweepOnce    SweepOnceFunc
	Wait         WaitFunc
	OnSweepError func(error)
}

type SchedulerResult struct {
	Sweeps       int
	FailedSweeps int
	Reason       string
}

func RunScheduler(ctx context.Context, opts SchedulerOptions) (SchedulerResult, error) {
	if opts.SweepOnce == nil {
		return SchedulerResult{}, errors.New("recovery scheduler requires a sweep function")
	}
	interval := EffectiveInterval(opts.Interval, opts.MinInterval)
	wait := opts.Wait
	if wait == nil {
		wait = waitContext
	}

	result := SchedulerResult{}
	for {
		select {
		case <-ctx.Done():
			result.Reason = ReasonContextCanceled
			return result, ctx.Err()
		default:
		}

		result.Sweeps++
		if _, err := opts.SweepOnce(ctx); err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				result.Reason = ReasonContextCanceled
				return result, err
			}
			result.FailedSweeps++
			if opts.OnSweepError != nil {
				opts.OnSweepError(err)
			}
			if opts.MaxSweeps != nil && result.Sweeps >= *opts.MaxSweeps {
				result.Reason = ReasonMaxSweepsReached
				return result, nil
			}
			if !wait(ctx, interval) {
				result.Reason = ReasonContextCanceled
				return result, ctx.Err()
			}
			continue
		}
		if opts.MaxSweeps != nil && result.Sweeps >= *opts.MaxSweeps {
			result.Reason = ReasonMaxSweepsReached
			return result, nil
		}
		if !wait(ctx, interval) {
			result.Reason = ReasonContextCanceled
			return result, ctx.Err()
		}
	}
}

func EffectiveInterval(interval time.Duration, minInterval time.Duration) time.Duration {
	if minInterval <= 0 {
		minInterval = DefaultMinInterval
	}
	if interval <= 0 || interval < minInterval {
		return minInterval
	}
	return interval
}

func waitContext(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
