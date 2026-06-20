package metrics

import (
	"io"
	"strconv"
	"time"
)

// scrapeContentType is the Prometheus text exposition content type (version
// 0.0.4) the /metrics handler sets.
const scrapeContentType = "text/plain; version=0.0.4; charset=utf-8"

// Seed metric names (RFC 0137 §1). Underscore-only identifiers; no value or
// label below is ever interpolated from a repo/run/session identifier, a path,
// a branch, a sha, a prompt, or a byline — only aggregate counts, the closed
// run-state enum, and the snapshot age reach the wire.
const (
	metricSnapshotAge = "striatum_metrics_snapshot_age_seconds"
	metricStranded    = "striatum_stranded_supervisors"
	metricRuns        = "striatum_runs"
)

// WriteText renders the snapshot as Prometheus text exposition. The output is
// deterministic — fixed metric order, and the full canonical run-state enum
// emitted in sorted order including zeros — so it is byte-stable for the golden
// redaction test. Help text is deliberately free of slashes, `--flag=` shapes,
// `author:` bylines, and 40-hex runs so it can never trip the forbidden-content
// regexes that backstop the exfiltration contract (RFC 0137 §2).
func (s *Snapshot) WriteText(w io.Writer, now time.Time) error {
	bw := &errWriter{w: w}

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

	return bw.err
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
