package metrics

import (
	"io"
	"strconv"
	"time"
)

// scrapeContentType is the Prometheus text exposition content type (version
// 0.0.4) the /metrics handler sets.
const scrapeContentType = "text/plain; version=0.0.4; charset=utf-8"

// ScrapeContentType exposes the Prometheus exposition content type so an external
// handler (the capability-scoped wrapper in the daemon) sets the identical header
// as the in-package Handler.
func ScrapeContentType() string { return scrapeContentType }

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
	// Phase C contract families.
	metricDoctorProblems     = "striatum_doctor_problems"
	metricCardinalityClipped = "striatum_metrics_cardinality_clipped_total"
	// Phase D families.
	metricTickStatus       = "striatum_metrics_tick_status"
	metricLifecycleBalance = "striatum_lifecycle_balance"
	metricRepoConsent      = "striatum_metrics_repo_consent"
	metricRepoRuns         = "striatum_repo_runs"
)

// WriteText renders the FULL snapshot as Prometheus text exposition — the
// loopback / default-open surface (Phase A). The output is deterministic — fixed
// metric order, closed enums emitted in sorted order, and observed label-tuples
// sorted — so it is byte-stable for the golden redaction test.
func (s *Snapshot) WriteText(w io.Writer, now time.Time) error {
	return s.WriteTextScoped(w, now, nil)
}

// WriteTextScoped renders the snapshot, filtering the per-repo families
// (striatum_metrics_repo_consent and striatum_repo_runs, the only families carrying
// the salted `bucket` surrogate) to the repositories a capability-scoped scraper is
// authorized for (Phase D deliverable #1). `allowedRepos` is keyed by REAL
// repository_id: a nil set is the loopback / default-open case (every repo
// rendered); a non-nil set includes a per-repo series only when its repository_id is
// authorized. The filter is applied on the repository_id BEFORE the per-repo data is
// aggregated under its salted bucket, so two repos that collide into one surrogate
// bucket stay isolated — a tailnet scraper holding only repo-A's token sees repo-A's
// series alone, never a colliding repo-B's (RFC 0137 §4). Every repo-aggregate
// Operational family is always rendered in full. The output stays deterministic and
// byte-stable for the chosen scope.
func (s *Snapshot) WriteTextScoped(w io.Writer, now time.Time, allowedRepos map[string]bool) error {
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

	// Phase C contract families.
	s.writeDoctorProblems(bw)
	s.writeCardinalityClipped(bw)

	// Phase D families. tick_status and lifecycle_balance are repo-aggregate
	// Operational signals and are always rendered; the per-repo families are
	// filtered to the authorized repositories (by real repository_id) before their
	// salted bucket is rendered.
	s.writeTickStatus(bw)
	s.writeLifecycleBalance(bw)
	s.writeRepoConsent(bw, allowedRepos)
	s.writeRepoRuns(bw, allowedRepos)

	return bw.err
}

// writeTickStatus renders the sweep-tick status SLI (Phase D deliverable #3). The
// full closed enum is emitted every scrape — 1 for the active status, 0 for the
// others — so a flip to partial/error is immediately alertable and never waits for
// a new series to appear. Combined with publish-on-errored-tick (the collector
// republishes a carried-forward snapshot stamped error rather than silently
// serving last-good), a wedged reconcile loop is directly visible.
func (s *Snapshot) writeTickStatus(bw *errWriter) {
	active := normalizeTickStatus(s.tickStatus)
	bw.line("# HELP " + metricTickStatus + " Status of the sweep tick that folded the current snapshot; one of ok, partial, error.")
	bw.line("# TYPE " + metricTickStatus + " gauge")
	for _, status := range tickStatusDomain {
		value := 0
		if status == active {
			value = 1
		}
		bw.line(metricTickStatus + `{status="` + string(status) + `"} ` + strconv.Itoa(value))
	}
}

// writeLifecycleBalance renders the OQ2 lifecycle-balance "second doctor" gauge:
// the count of terminal transitions that declared a death the fold could account
// for in NEITHER lifecycle counter. A persistent nonzero value is a provable
// runner blind spot (a confirmed-dead transition vanishing from apoptosis AND
// necrosis) and is directly alertable; it stays zero in healthy operation.
func (s *Snapshot) writeLifecycleBalance(bw *errWriter) {
	bw.line("# HELP " + metricLifecycleBalance + " Terminal transitions unaccounted for in apoptosis or necrosis; a nonzero value is a runner blind spot.")
	bw.line("# TYPE " + metricLifecycleBalance + " gauge")
	bw.line(metricLifecycleBalance + " " + strconv.Itoa(s.unaccountedTerminal))
}

// writeRepoConsent renders the per-repo consent gauge (Phase D deliverable #2).
// One series per active repo's salted bucket carries the consent state (0|1) so
// the ABSENCE of provenance series is itself a scrapeable fact rather than
// ambiguous. The gauge is aggregated from the retained per-repo entries over only
// the authorized repositories (by real repository_id) BEFORE the bucket label is
// applied, so the per-repo surrogate cannot leak a colliding repo across a
// capability boundary. HELP/TYPE are always emitted so an empty scope still
// produces a well-formed family.
func (s *Snapshot) writeRepoConsent(bw *errWriter, allowedRepos map[string]bool) {
	bw.line("# HELP " + metricRepoConsent + " Per-repo provenance-metrics consent state by salted bucket; 1 when consented, 0 otherwise.")
	bw.line("# TYPE " + metricRepoConsent + " gauge")
	consent := s.aggregateRepoConsent(allowedRepos)
	for _, bucket := range sortedStringKeys(consent) {
		bw.line(metricRepoConsent + `{bucket="` + bucket + `"} ` + strconv.Itoa(consent[bucket]))
	}
}

// writeRepoRuns renders the per-repo (Provenance) run-state gauge. It exists only
// for repos that set the per-repo consent flag (the consent gate), and is further
// filtered to the authorized repositories here. Both gates compose: a series appears
// only when the repo consented AND the scraper is authorized for that repository.
// The aggregation by bucket happens AFTER the repository_id filter, so a colliding
// repo outside the scope never merges its counts into an authorized repo's bucket.
// The `state` label is the same closed run-state enum as the global striatum_runs
// gauge, so cardinality is bounded by buckets * states.
func (s *Snapshot) writeRepoRuns(bw *errWriter, allowedRepos map[string]bool) {
	bw.line("# HELP " + metricRepoRuns + " Per-repo run count by salted bucket and lifecycle state; consent-gated provenance family.")
	bw.line("# TYPE " + metricRepoRuns + " gauge")
	runs := s.aggregateRepoRuns(allowedRepos)
	for _, k := range sortedBucketStates(runs) {
		bw.line(metricRepoRuns + `{bucket="` + k.bucket + `",state="` + k.state + `"} ` + strconv.Itoa(runs[k]))
	}
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

// writeDoctorProblems renders the open-doctor-integrity-problems gauge. The
// `class` label is ONLY ever a static problem-record check code (F-A8); the fold
// never reads the dynamic-id `problems` strings, so no run/gate/supervisor id can
// appear here. HELP/TYPE are always emitted so absence is distinguishable from a
// genuine zero; only observed classes (sorted) carry a value, and the series
// budget bounds them at doctorProblemsSeriesBudget+1.
func (s *Snapshot) writeDoctorProblems(bw *errWriter) {
	bw.line("# HELP " + metricDoctorProblems + " Open doctor integrity problems grouped by static check class.")
	bw.line("# TYPE " + metricDoctorProblems + " gauge")
	for _, class := range sortedStringKeys(s.doctorProblems) {
		bw.line(metricDoctorProblems + `{class="` + class + `"} ` + strconv.Itoa(s.doctorProblems[class]))
	}
}

// writeCardinalityClipped renders the per-family series-budget clip counter — the
// number of distinct label-tuples collapsed onto the reserved `other` bucket. A
// nonzero value is itself alertable (silent dimension loss is made visible).
func (s *Snapshot) writeCardinalityClipped(bw *errWriter) {
	bw.line("# HELP " + metricCardinalityClipped + " Distinct label tuples collapsed to the other bucket by the per family series budget.")
	bw.line("# TYPE " + metricCardinalityClipped + " counter")
	for _, family := range sortedStringKeys(s.cardinalityClipped) {
		bw.line(metricCardinalityClipped + `{family="` + family + `"} ` + strconv.Itoa(s.cardinalityClipped[family]))
	}
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
