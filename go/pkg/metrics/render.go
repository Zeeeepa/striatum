package metrics

import (
	"io"
	"strconv"
	"time"
)

// scrapeContentType is the Prometheus text exposition content type (version
// 0.0.4) the /metrics handler sets.
const scrapeContentType = "text/plain; version=0.0.4; charset=utf-8"

// Metric names. Underscore-only identifiers; no value or label below is ever
// interpolated from a repo/run/session identifier, a path, a branch, a sha, a
// prompt, or a byline — only aggregate counts/timings, closed enums, and the
// snapshot age reach the wire (RFC 0137 §2). Help text is deliberately free of
// slashes, `--flag=` shapes, `author:` bylines, and 40-hex runs so it can never
// trip the forbidden-content regexes that backstop the exfiltration contract.
const (
	metricSnapshotAge    = "striatum_metrics_snapshot_age_seconds"
	metricStranded       = "striatum_stranded_supervisors"
	metricRuns           = "striatum_runs"
	metricApoptosis      = "striatum_apoptosis_total"
	metricNecrosis       = "striatum_necrosis_total"
	metricLeaseTrans     = "striatum_lease_transitions_total"
	metricLivenessEvents = "striatum_liveness_deadline_events_total"
	metricWedgeAge       = "striatum_run_wedge_age_seconds"
	metricLivenessMargin = "striatum_liveness_deadline_margin_seconds"
)

// WriteText renders the snapshot as Prometheus text exposition. The output is
// deterministic — fixed metric order, closed enums emitted in sorted order, and
// observed label-tuples sorted — so it is byte-stable for the golden redaction
// test.
func (s *Snapshot) WriteText(w io.Writer, now time.Time) error {
	bw := &errWriter{w: w}

	// Phase A seed metrics.
	bw.line("# HELP " + metricSnapshotAge + " Seconds since the metrics snapshot was folded at the recovery sweep tick.")
	bw.line("# TYPE " + metricSnapshotAge + " gauge")
	bw.line(metricSnapshotAge + " " + strconv.FormatFloat(s.ageSeconds(now), 'f', 6, 64))

	bw.line("# HELP " + metricStranded + " Supervisors attached to terminal runs; the phantom-supervisor storm signal.")
	bw.line("# TYPE " + metricStranded + " gauge")
	bw.line(metricStranded + " " + strconv.Itoa(s.strandedSupervisors))

	bw.line("# HELP " + metricRuns + " Run count grouped by lifecycle state; unknown states bucket to other.")
	bw.line("# TYPE " + metricRuns + " gauge")
	for _, state := range canonicalRunStates {
		bw.line(metricRuns + `{state="` + state + `"} ` + strconv.Itoa(s.runStateCount(state)))
	}
	bw.line(metricRuns + `{state="` + stateOther + `"} ` + strconv.Itoa(s.runStateCount(stateOther)))

	// Phase B failure-mode taxonomy.
	s.writeApoptosis(bw)
	s.writeNecrosis(bw)
	s.writeLivenessEvents(bw)
	s.writeLeaseTransitions(bw)
	s.writeWedgeAge(bw)
	s.writeLivenessMargin(bw)

	return bw.err
}

// writeApoptosis renders the healthy programmed self-termination counter. Only
// observed (origin, reason) tuples are emitted (sorted); the closed enums bound
// the maximum series count independent of run/job count.
func (s *Snapshot) writeApoptosis(bw *errWriter) {
	bw.line("# HELP " + metricApoptosis + " Healthy programmed self-terminations by origin and reason.")
	bw.line("# TYPE " + metricApoptosis + " counter")
	for _, k := range sortedOriginReasons(s.apoptosis) {
		bw.line(metricApoptosis + `{origin="` + string(k.origin) + `",reason="` + k.reason + `"} ` + strconv.Itoa(s.apoptosis[k]))
	}
}

// writeNecrosis renders the confirmed-dead counter. F-A6: a recoverable liveness
// miss never appears here — it moves the liveness-events counter below instead.
func (s *Snapshot) writeNecrosis(bw *errWriter) {
	bw.line("# HELP " + metricNecrosis + " Confirmed-dead uncontrolled terminations by origin and reason; any nonzero rate is alertable.")
	bw.line("# TYPE " + metricNecrosis + " counter")
	for _, k := range sortedOriginReasons(s.necrosis) {
		bw.line(metricNecrosis + `{origin="` + string(k.origin) + `",reason="` + k.reason + `"} ` + strconv.Itoa(s.necrosis[k]))
	}
}

// writeLivenessEvents renders the non-terminal liveness-deadline counter (F-A6).
// The full closed reason enum is emitted (including zeros) so the reversible
// pair is always visible; this is the home for the deadline_missed/recovered
// signal that is deliberately OUTSIDE the apoptosis/necrosis conservation law.
func (s *Snapshot) writeLivenessEvents(bw *errWriter) {
	bw.line("# HELP " + metricLivenessEvents + " Reversible liveness-deadline observations; excluded from necrosis by design.")
	bw.line("# TYPE " + metricLivenessEvents + " counter")
	for _, reason := range LivenessDeadlineReasons() {
		bw.line(metricLivenessEvents + `{reason="` + string(reason) + `"} ` + strconv.Itoa(s.livenessEvents[string(reason)]))
	}
}

// writeLeaseTransitions renders the lease state-change counter. from/to are the
// closed lease-state enum; reason is the bucketed category enum.
func (s *Snapshot) writeLeaseTransitions(bw *errWriter) {
	bw.line("# HELP " + metricLeaseTrans + " Lease state transitions by from, to and bucketed reason.")
	bw.line("# TYPE " + metricLeaseTrans + " counter")
	for _, k := range sortedLeaseTriples(s.leaseTransitions) {
		bw.line(metricLeaseTrans + `{from="` + k.from + `",to="` + k.to + `",reason="` + k.reason + `"} ` + strconv.Itoa(s.leaseTransitions[k]))
	}
}

// writeWedgeAge renders the wedged-run age histogram per origin.
func (s *Snapshot) writeWedgeAge(bw *errWriter) {
	bw.line("# HELP " + metricWedgeAge + " Age in seconds since a non-terminal run last advanced a job state.")
	bw.line("# TYPE " + metricWedgeAge + " histogram")
	writeHistogramFamily(bw, metricWedgeAge, s.runWedgeAge)
}

// writeLivenessMargin renders the liveness-deadline margin histogram per origin.
func (s *Snapshot) writeLivenessMargin(bw *errWriter) {
	bw.line("# HELP " + metricLivenessMargin + " Seconds of margin to the nearest liveness deadline; negative once elapsed.")
	bw.line("# TYPE " + metricLivenessMargin + " histogram")
	writeHistogramFamily(bw, metricLivenessMargin, s.livenessMargin)
}

// writeHistogramFamily renders a per-origin histogram family in deterministic
// order: for each origin (sorted), the cumulative _bucket series in ascending le
// order, then the +Inf bucket, _sum and _count.
func writeHistogramFamily(bw *errWriter, name string, byOrigin map[Origin]*histogram) {
	for _, origin := range sortedOrigins(byOrigin) {
		h := byOrigin[origin]
		label := `{origin="` + string(origin) + `"`
		for i, bound := range h.bounds {
			bw.line(name + `_bucket` + label + `,le="` + formatFloat(bound) + `"} ` + strconv.FormatUint(h.counts[i], 10))
		}
		bw.line(name + `_bucket` + label + `,le="+Inf"} ` + strconv.FormatUint(h.total, 10))
		bw.line(name + `_sum` + label + `} ` + formatFloat(h.sum))
		bw.line(name + `_count` + label + `} ` + strconv.FormatUint(h.total, 10))
	}
}

// formatFloat renders a float as a compact, deterministic decimal for le labels
// and histogram sums.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// errWriter accumulates the first write error so the render reads linearly.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) line(s string) {
	if e.err != nil {
		return
	}
	if _, err := io.WriteString(e.w, s); err != nil {
		e.err = err
		return
	}
	_, e.err = io.WriteString(e.w, "\n")
}
