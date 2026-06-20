package metrics

// RFC 0137 Phase D — per-family time-series rate semantics (deliverable #4).
//
// A committed Prometheus rule may wrap a series in a counter-only function
// (rate/irate/increase/resets) ONLY when that series is monotonic. The Prometheus
// exposition `# TYPE` line is NOT a sufficient ground for this: this exporter folds
// an immutable snapshot once per sweep tick, so several families are SNAPSHOT
// (gauge) semantics even though their type or name reads counter-ish —
//
//   - striatum_run_wedge_age_seconds / striatum_liveness_deadline_margin_seconds are
//     exposed as `# TYPE histogram`, but each tick REBUILDS the histogram from the
//     currently non-terminal runs / active sessions. The _bucket/_sum/_count series
//     therefore rise AND fall (a run completes -> its observation disappears next
//     tick), so they are gauge-histograms: histogram_quantile reads them directly,
//     never through rate().
//   - striatum_metrics_cardinality_clipped_total carries the `_total` suffix and is
//     exposed as `# TYPE counter` (the Phase C contract), but its VALUE is the
//     number of label-tuples clipped in the CURRENT snapshot — recomputed each tick,
//     not accumulated from an append-only ledger — so it is non-monotonic. Rules read
//     it with a direct comparison / max_over_time, never increase().
//
// The TRUE counters (apoptosis/necrosis/lease/liveness-events) are folded from the
// append-only striatumd.events ledger, so they only ever grow and are
// restart-consistent — rate()/increase() is valid. This file is the single source
// of truth the rules guardrail test grounds against; TestMetricRateSemantics-
// CoversRegistry pins it to exactly the registry's exported families, so a new
// family must declare its rate semantics deliberately.

// RateSemantics classifies an exported family by whether its time series is
// monotonic. It is independent of the Prometheus exposition type (a histogram or a
// `_total` counter can still be snapshot semantics in this snapshot-fold model).
type RateSemantics string

const (
	// SeriesMonotonic is a true counter that only increases (folded from the
	// append-only events ledger); rate()/irate()/increase()/resets() are valid.
	SeriesMonotonic RateSemantics = "monotonic"
	// SeriesSnapshot is a gauge or per-tick gauge-histogram that can rise and fall;
	// a counter-only function over it is semantically invalid. Rules must use a
	// direct comparison, max_over_time/min_over_time, or histogram_quantile.
	SeriesSnapshot RateSemantics = "snapshot"
)

// metricRateSemantics maps every exported family to its rate semantics. The keys
// are the BASE family names; the histogram-derived _bucket/_sum/_count series
// inherit their base family's semantics (RateSemanticsForSeries resolves them).
var metricRateSemantics = map[string]RateSemantics{
	// True counters — folded from the append-only durable events ledger.
	metricApoptosis:      SeriesMonotonic,
	metricNecrosis:       SeriesMonotonic,
	metricLeaseTrans:     SeriesMonotonic,
	metricLivenessEvents: SeriesMonotonic,

	// Point-in-time gauges.
	metricSnapshotAge:      SeriesSnapshot,
	metricStranded:         SeriesSnapshot,
	metricRuns:             SeriesSnapshot,
	metricDoctorProblems:   SeriesSnapshot,
	metricTickStatus:       SeriesSnapshot,
	metricLifecycleBalance: SeriesSnapshot,
	metricRepoConsent:      SeriesSnapshot,
	metricRepoRuns:         SeriesSnapshot,

	// Per-tick gauge-histograms — rebuilt each fold from the current population, so
	// their _bucket/_sum/_count rise and fall. histogram_quantile reads them
	// directly; rate() is invalid.
	metricWedgeAge:       SeriesSnapshot,
	metricLivenessMargin: SeriesSnapshot,

	// Snapshot despite the `_total` suffix and the `# TYPE counter` exposition: the
	// value is the current snapshot's clip count, recomputed each tick.
	metricCardinalityClipped: SeriesSnapshot,
}

// histogramSeriesSuffixes are the derived series a histogram family emits. A
// referenced series ending in one of these resolves to its base family's semantics.
var histogramSeriesSuffixes = []string{"_bucket", "_sum", "_count"}

// MetricRateSemantics returns a copy of the base-family rate-semantics map. The
// guardrail test grounds rule validation against it.
func MetricRateSemantics() map[string]RateSemantics {
	out := make(map[string]RateSemantics, len(metricRateSemantics))
	for name, kind := range metricRateSemantics {
		out[name] = kind
	}
	return out
}

// RateSemanticsForSeries resolves the rate semantics of a referenced series name,
// stripping a histogram suffix to find its base family. ok is false for a name that
// is not an exported family (or a derived series of one), so the caller can flag an
// unknown reference distinctly from a known-but-snapshot one.
func RateSemanticsForSeries(series string) (RateSemantics, bool) {
	if kind, ok := metricRateSemantics[series]; ok {
		return kind, true
	}
	for _, suffix := range histogramSeriesSuffixes {
		if len(series) > len(suffix) && series[len(series)-len(suffix):] == suffix {
			if kind, ok := metricRateSemantics[series[:len(series)-len(suffix)]]; ok {
				return kind, true
			}
		}
	}
	return "", false
}
