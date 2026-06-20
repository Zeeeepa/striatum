// Package metrics implements RFC 0137 Phase A: a lock-disjoint, zero-DB-query
// Prometheus read surface for striatumd.
//
// The exporter never queries PostgreSQL or takes a runner mutex at scrape time.
// A Snapshot is folded once per resident recovery-sweep tick and published into
// a package-level atomic.Pointer with the same lock-free copy-on-publish pattern
// used by go/pkg/db/write_boundary.go and go/pkg/db/authority.go. The scrape
// handler does exactly: Load -> render text -> write. N concurrent scrapers
// therefore cost the same as one and observe the identical snapshot pointer.
//
// Phase A intentionally ships only the seed metrics (stranded-supervisor count,
// run-state counts, snapshot age). The failure-mode taxonomy, the
// Classification/Register refusal, the metrics_allowlist hash check, the
// doctor_problems collector, and the multi-tenant/consent/alert-rules work are
// Phases B/C/D and are deliberately out of scope here.
package metrics

import (
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

// Snapshot is the immutable Phase A metrics value object. It is built off the
// hot path (once per sweep tick) and published by pointer; readers never mutate
// it. The zero value is valid and renders an all-zero surface with age 0.
type Snapshot struct {
	builtAt             time.Time
	strandedSupervisors int
	runStates           map[string]int // normalized: every canonical state + stateOther present
}

// BuildSnapshot folds run observations and the stranded-supervisor count into an
// immutable Snapshot, discarding every sensitive column and bucketing unknown
// states into stateOther.
func BuildSnapshot(builtAt time.Time, runs []RunObservation, strandedSupervisors int) *Snapshot {
	raw := make(map[string]int, len(canonicalRunStates))
	for _, r := range runs {
		raw[r.State]++
	}
	return newSnapshot(builtAt, raw, strandedSupervisors)
}

// newSnapshot normalizes a raw state->count map into the closed enum (every
// canonical state plus stateOther, all present) and returns the immutable
// Snapshot. It is the single fold path shared by BuildSnapshot (tests + small
// sets) and Collector.Refresh (the live GROUP BY aggregate).
func newSnapshot(builtAt time.Time, rawCounts map[string]int, strandedSupervisors int) *Snapshot {
	states := make(map[string]int, len(canonicalRunStates)+1)
	for _, s := range canonicalRunStates {
		states[s] = 0
	}
	states[stateOther] = 0
	for state, n := range rawCounts {
		if isCanonicalRunState(state) {
			states[state] += n
		} else {
			states[stateOther] += n
		}
	}
	if strandedSupervisors < 0 {
		strandedSupervisors = 0
	}
	return &Snapshot{
		builtAt:             builtAt,
		strandedSupervisors: strandedSupervisors,
		runStates:           states,
	}
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

// published is the package-level snapshot pointer (RFC 0137 §1). Collector.Refresh
// Stores; the scrape path Loads. atomic.Pointer gives lock-free copy-on-publish
// so the scrape path takes no mutex and N concurrent scrapers read the identical
// pointer.
var published atomic.Pointer[Snapshot]

// Publish installs s as the current snapshot (the Store half of copy-on-publish).
func Publish(s *Snapshot) { published.Store(s) }

// Load returns the current published snapshot, or nil before the first fold.
func Load() *Snapshot { return published.Load() }
