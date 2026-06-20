// Package metrics implements RFC 0137: a lock-disjoint, zero-DB-query Prometheus
// read surface for striatumd.
//
// The exporter never queries PostgreSQL or takes a runner mutex at scrape time.
// A Snapshot is folded once per resident recovery-sweep tick and published into
// a package-level atomic.Pointer with the same lock-free copy-on-publish pattern
// used by go/pkg/db/write_boundary.go and go/pkg/db/authority.go. The scrape
// handler does exactly: Load -> render text -> write. N concurrent scrapers
// therefore cost the same as one and observe the identical snapshot pointer.
//
// Phase A shipped the seed metrics (stranded-supervisor count, run-state counts,
// snapshot age). Phase B (this slice) adds the failure-mode taxonomy: the
// apoptosis/necrosis split, the non-terminal liveness-deadline counter (F-A6),
// lease-transition counts, and the wedge-age / liveness-margin histograms. Every
// Phase B counter is FOLDED FROM THE DURABLE striatumd.events LEDGER at the tick,
// so the numbers are tx-safe (a rolled-back lifecycle transaction never wrote the
// event, so it is never counted) and restart-consistent by construction (the
// counter is re-derived from durable history rather than reset to zero) — the
// preferred mechanism in RFC 0137 §"Design guidance". The Classification/Register
// refusal, the cardinality budget, the metrics_allowlist hash check, the
// doctor_problems collector, and the multi-tenant/consent/alert-rules work are
// Phases C/D and are deliberately out of scope here.
package metrics

import (
	"sort"
	"sync/atomic"
	"time"
)

// canonicalRunStates is the closed enum of striatumd.runs.state values
// (runs_state_check, go/pkg/db/sql/0021_run_needs_operator.sql), rendered in a
// fixed, sorted order so the /metrics body is byte-stable. Any state outside
// this set folds into stateOther, keeping the `state` label a bounded enum even
// if a later migration adds a value — cardinality cannot grow with run history.
var canonicalRunStates = []string{
	"blocked",
	"canceled",
	"completed",
	"compromised",
	"failed",
	"needs_branch_confirmation",
	"needs_operator",
	"ready",
	"running",
}

// stateOther is the reserved catch-all bucket for any non-canonical run state.
const stateOther = "other"

func isCanonicalRunState(state string) bool {
	for _, s := range canonicalRunStates {
		if s == state {
			return true
		}
	}
	return false
}

// RunObservation is one run row as it exists at a sweep tick. It deliberately
// carries the sensitive provenance columns that live next to runs.state in the
// daemon DB (repo filesystem path, git branch, head sha, a lane prompt/argv
// fragment, an author byline) so the fold's contract — copy ONLY the closed-enum
// state onto the wire, never a sensitive value — is executable in the redaction
// test. The production fold (Collector.Refresh) never selects those columns; the
// fields are the second line of the defense-in-depth redaction contract (RFC
// 0137 §2): even if a sensitive value is present on the struct, render drops it.
type RunObservation struct {
	State          string // runs.state — the ONLY field that may reach the wire
	RepoPath       string // sensitive: must never appear in /metrics
	Branch         string // sensitive
	HeadSHA        string // sensitive
	PromptFragment string // sensitive
	AuthorByline   string // sensitive
}

// LeaseTransition is one lease state change as the fold sees it. Only the
// closed-enum from/to states and the bucketed reason reach the wire — never the
// lease/job/session id the underlying event also carries.
type LeaseTransition struct {
	From   string // lease state transitioned FROM (bucketed to the closed enum)
	To     string // lease state transitioned TO (bucketed to the closed enum)
	Reason string // raw release reason — bucketed to the closed enum by the fold
}

// WedgeObservation is one non-terminal run's wedge age (seconds since its last
// job-state advance) at the tick, tagged with the origin enum. The age is an
// aggregate timing, never an id.
type WedgeObservation struct {
	Origin     Origin
	AgeSeconds float64
}

// MarginObservation is one active session's margin (seconds) to its nearest
// liveness deadline at the tick, tagged with the origin enum. Negative when the
// deadline has already elapsed (a reversible pre-death state, F-A6).
type MarginObservation struct {
	Origin        Origin
	MarginSeconds float64
}

// SnapshotInput is the full set of folded observations a Snapshot is built from.
// Runs / RawRunStateCounts feed the Phase A run-state gauge (Runs for tests,
// RawRunStateCounts for the live GROUP BY); the rest feed the Phase B families.
// The *Counts variants carry pre-aggregated GROUP BY rows so the live fold never
// transfers one row per durable event; the unweighted slices are the
// one-per-observation form the unit tests build.
type SnapshotInput struct {
	BuiltAt               time.Time
	Runs                  []RunObservation
	RawRunStateCounts     map[string]int
	StrandedSupervisors   int
	Events                []LifecycleEvent
	EventCounts           []EventCount
	LeaseTransitions      []LeaseTransition
	LeaseTransitionCounts []LeaseTransitionCount
	WedgeAges             []WedgeObservation
	LivenessMargins       []MarginObservation
	// DoctorProblemRecords are the STATIC problem records the doctor checks
	// emitted (each carries a static `check` field), folded into the Phase C
	// doctor_problems{class} gauge. Only the `check` field reaches the wire — see
	// foldDoctorProblemRecords (F-A8). The collector aggregates them across
	// active repositories at the sweep tick.
	DoctorProblemRecords []map[string]any
}

// EventCount is a GROUP BY (event coordinates -> count) row from the live fold.
type EventCount struct {
	Event LifecycleEvent
	Count int
}

// LeaseTransitionCount is a GROUP BY (lease transition -> count) row from the
// live fold.
type LeaseTransitionCount struct {
	Transition LeaseTransition
	Count      int
}

// originReason keys the apoptosis/necrosis counter maps.
type originReason struct {
	origin Origin
	reason string
}

// leaseTriple keys the lease-transition counter map.
type leaseTriple struct {
	from   string
	to     string
	reason string
}

// runWedgeAgeBuckets / livenessMarginBuckets are the fixed histogram bucket
// upper bounds (Prometheus `le`), ascending. They are CONSTANTS — independent of
// run/job/session count — so the histogram series count is bounded.
var (
	runWedgeAgeBuckets    = []float64{60, 300, 900, 1800, 3600, 7200, 21600}
	livenessMarginBuckets = []float64{-1800, -300, -60, 0, 60, 300, 900, 1800, 3600}
)

// histogram is a minimal fixed-bucket Prometheus histogram (cumulative buckets +
// sum + count). counts[i] is the number of observations <= buckets[i]; the
// implicit +Inf bucket equals count.
type histogram struct {
	bounds []float64
	counts []uint64 // len == len(bounds); cumulative is computed at render
	sum    float64
	total  uint64
}

func newHistogram(bounds []float64) *histogram {
	return &histogram{bounds: bounds, counts: make([]uint64, len(bounds))}
}

func (h *histogram) observe(v float64) {
	h.sum += v
	h.total++
	for i, b := range h.bounds {
		if v <= b {
			h.counts[i]++
		}
	}
}

// Snapshot is the immutable metrics value object. It is built off the hot path
// (once per sweep tick) and published by pointer; readers never mutate it. The
// zero value is valid and renders an all-zero surface with age 0.
type Snapshot struct {
	builtAt             time.Time
	strandedSupervisors int
	runStates           map[string]int // normalized: every canonical state + stateOther present

	// Phase B failure-mode families, all folded from durable events / point-in-
	// time observations at the tick. Maps are keyed by their closed-enum labels.
	apoptosis        map[originReason]int
	necrosis         map[originReason]int
	livenessEvents   map[string]int // LivenessDeadlineReason -> count
	leaseTransitions map[leaseTriple]int
	runWedgeAge      map[Origin]*histogram
	livenessMargin   map[Origin]*histogram

	// Phase C contract families. doctorProblems counts open doctor integrity
	// problems by their STATIC check class (F-A8); cardinalityClipped reports, per
	// family, how many distinct label-tuples the series budget collapsed onto the
	// reserved `other` bucket this tick.
	doctorProblems     map[string]int
	cardinalityClipped map[string]int
}

// BuildSnapshot is the Phase A constructor: it folds run observations and the
// stranded-supervisor count into an immutable Snapshot. Retained as a stable API
// (the redaction/scrape tests call it); it delegates to Build.
func BuildSnapshot(builtAt time.Time, runs []RunObservation, strandedSupervisors int) *Snapshot {
	return Build(SnapshotInput{BuiltAt: builtAt, Runs: runs, StrandedSupervisors: strandedSupervisors})
}

// Build folds a full SnapshotInput into the immutable Snapshot, discarding every
// sensitive column, bucketing unknown enum values, and classifying each durable
// event into its failure-mode family via classifyLifecycleEvent.
func Build(in SnapshotInput) *Snapshot {
	rawStates := make(map[string]int, len(canonicalRunStates))
	for _, r := range in.Runs {
		rawStates[r.State]++
	}
	for state, n := range in.RawRunStateCounts {
		rawStates[state] += n
	}
	states := make(map[string]int, len(canonicalRunStates)+1)
	for _, s := range canonicalRunStates {
		states[s] = 0
	}
	states[stateOther] = 0
	for state, n := range rawStates {
		if isCanonicalRunState(state) {
			states[state] += n
		} else {
			states[stateOther] += n
		}
	}

	stranded := in.StrandedSupervisors
	if stranded < 0 {
		stranded = 0
	}

	snap := &Snapshot{
		builtAt:             in.BuiltAt,
		strandedSupervisors: stranded,
		runStates:           states,
		apoptosis:           map[originReason]int{},
		necrosis:            map[originReason]int{},
		livenessEvents:      map[string]int{},
		leaseTransitions:    map[leaseTriple]int{},
		runWedgeAge:         map[Origin]*histogram{},
		livenessMargin:      map[Origin]*histogram{},
		doctorProblems:      map[string]int{},
		cardinalityClipped:  map[string]int{},
	}

	for _, ev := range in.Events {
		snap.addEvent(ev, 1)
	}
	for _, ec := range in.EventCounts {
		snap.addEvent(ec.Event, ec.Count)
	}

	for _, lt := range in.LeaseTransitions {
		snap.addLeaseTransition(lt, 1)
	}
	for _, lc := range in.LeaseTransitionCounts {
		snap.addLeaseTransition(lc.Transition, lc.Count)
	}

	for _, w := range in.WedgeAges {
		origin := w.Origin
		if !isCanonicalOrigin(origin) {
			origin = OriginDaemonCore
		}
		h := snap.runWedgeAge[origin]
		if h == nil {
			h = newHistogram(runWedgeAgeBuckets)
			snap.runWedgeAge[origin] = h
		}
		h.observe(w.AgeSeconds)
	}

	for _, m := range in.LivenessMargins {
		origin := m.Origin
		if !isCanonicalOrigin(origin) {
			origin = OriginLane
		}
		h := snap.livenessMargin[origin]
		if h == nil {
			h = newHistogram(livenessMarginBuckets)
			snap.livenessMargin[origin] = h
		}
		h.observe(m.MarginSeconds)
	}

	// Phase C: fold doctor_problems from the STATIC problem-record check codes and
	// enforce the per-family series budget. A clip is recorded under the family's
	// stable id so striatum_metrics_cardinality_clipped_total{family} surfaces the
	// silent dimension loss.
	snap.doctorProblems = foldDoctorProblemRecords(in.DoctorProblemRecords)
	if clipped := applySeriesBudget(snap.doctorProblems, doctorProblemsSeriesBudget, seriesClassOther); clipped > 0 {
		snap.cardinalityClipped[clipFamilyDoctorProblems] = clipped
	}

	return snap
}

// addEvent classifies one durable event and folds `weight` into its failure-mode
// family. weight <= 0 is ignored so an empty GROUP BY row never under/over-counts.
func (s *Snapshot) addEvent(ev LifecycleEvent, weight int) {
	if weight <= 0 {
		return
	}
	class, origin, reason, ok := classifyLifecycleEvent(ev)
	if !ok {
		return
	}
	switch class {
	case ClassApoptosis:
		s.apoptosis[originReason{origin, reason}] += weight
	case ClassNecrosis:
		s.necrosis[originReason{origin, reason}] += weight
	case ClassLiveness:
		s.livenessEvents[reason] += weight
	}
}

// addLeaseTransition buckets a lease transition to the closed enums and folds in
// `weight`.
func (s *Snapshot) addLeaseTransition(lt LeaseTransition, weight int) {
	if weight <= 0 {
		return
	}
	s.leaseTransitions[leaseTriple{
		from:   bucketLeaseState(lt.From),
		to:     bucketLeaseState(lt.To),
		reason: bucketLeaseReason(lt.Reason),
	}] += weight
}

// BuiltAt reports when the snapshot was folded.
func (s *Snapshot) BuiltAt() time.Time { return s.builtAt }

// ageSeconds is now - builtAt, clamped at zero. A zero builtAt (the pre-first-
// fold zero value) reports 0 rather than an epoch-distance.
func (s *Snapshot) ageSeconds(now time.Time) float64 {
	if s.builtAt.IsZero() {
		return 0
	}
	age := now.Sub(s.builtAt).Seconds()
	if age < 0 {
		return 0
	}
	return age
}

func (s *Snapshot) runStateCount(state string) int {
	if s.runStates == nil {
		return 0
	}
	return s.runStates[state]
}

// sortedOriginReasons returns the keys of an apoptosis/necrosis counter map in a
// deterministic (origin, reason) order so the rendered body is byte-stable.
func sortedOriginReasons(m map[originReason]int) []originReason {
	keys := make([]originReason, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].origin != keys[j].origin {
			return keys[i].origin < keys[j].origin
		}
		return keys[i].reason < keys[j].reason
	})
	return keys
}

// sortedLeaseTriples returns the lease-transition keys in deterministic order.
func sortedLeaseTriples(m map[leaseTriple]int) []leaseTriple {
	keys := make([]leaseTriple, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		if keys[i].to != keys[j].to {
			return keys[i].to < keys[j].to
		}
		return keys[i].reason < keys[j].reason
	})
	return keys
}

// sortedStringKeys returns the keys of a string-keyed counter map in sorted
// order so the rendered body is byte-stable (used by doctor_problems and the
// cardinality-clip counter).
func sortedStringKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedOrigins returns the origins present in a per-origin histogram map in
// deterministic order.
func sortedOrigins(m map[Origin]*histogram) []Origin {
	keys := make([]Origin, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// published is the package-level snapshot pointer (RFC 0137 §1). Collector.Refresh
// Stores; the scrape path Loads. atomic.Pointer gives lock-free copy-on-publish
// so the scrape path takes no mutex and N concurrent scrapers read the identical
// pointer.
var published atomic.Pointer[Snapshot]

// Publish installs s as the current snapshot (the Store half of copy-on-publish).
func Publish(s *Snapshot) { published.Store(s) }

// Load returns the current published snapshot, or nil before the first fold.
func Load() *Snapshot { return published.Load() }
