package metrics

// RFC 0162 — the lane-auth silent-failure observability families.
//
// These eight Operational families make the ABSENCE of expected lane
// authentication loud. They follow the established exporter contract: folded off
// the hot path (Collector.Refresh / Build), closed label NAMES only, every
// lane-bearing family bounded by the per-family series budget (overflow → the
// reserved lane="other" sentinel, visible via the cardinality-clip counter).
//
// Coverage classes (F1 fold):
//   - the roster `expected` vector is the census expected set (one series per
//     declared lane mechanism, incl. non-codex api_key);
//   - `sample_present` is the observed set — 1 when the sampler resolved + parsed
//     the lane's runtime credential this sweep (api_key emits it WITHOUT an
//     expiry series);
//   - the absence rule `expected unless on(lane) (sample_present == 1)` names the
//     missing lane (PromQL in rules/alerting_rules.yml).
//
// Fail-closed (F2 fold): an unprovable runtime credential source emits
// `resolver_mismatch` (which pages) and NO green expiry/sample series.

import (
	"regexp"
	"sort"
)

// laneAuthSeriesBudget bounds the distinct lanes any lane-keyed family emits
// (OQ3). With ≤16 rostered lanes × ≤2 kinds the families sit well inside it; the
// budget is the OOM backstop, collapsing overflow lanes onto lane="other" and
// surfacing the drop via striatum_metrics_cardinality_clipped_total.
const laneAuthSeriesBudget = 32

// laneLabelOther is the reserved overflow / unknown bucket for the `lane` label.
const laneLabelOther = "other"

// Clip-counter family ids (the `family` label on the cardinality-clip counter).
const (
	clipFamilyLaneAuthExpected         = "lane_auth_expected"
	clipFamilyLaneCredSamplePresent    = "lane_cred_sample_present"
	clipFamilyLaneCredResolverMismatch = "lane_cred_resolver_mismatch"
	clipFamilyLaneCredSecondsToExpiry  = "lane_cred_seconds_to_expiry"
	clipFamilyLaneCredAge              = "lane_cred_age_seconds"
	clipFamilyLaneAuthLastSuccess      = "lane_auth_last_success"
	clipFamilyLaneAuthThresholds       = "lane_auth_thresholds"
)

// laneLabelShape is the safe shape a `lane`/`provider`/`kind` label value must
// match (lowercase, bounded); anything else folds to "other" so an unexpected
// value can never carry a path/sha/id onto the wire (defense-in-depth alongside
// the roster validation and the series budget).
var laneLabelShape = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func safeLaneLabel(v string) string {
	if len(v) > 64 || !laneLabelShape.MatchString(v) {
		return laneLabelOther
	}
	return v
}

// LaneRosterObservation is one declared roster mechanism folded into the census
// expected vector + the two OQ4 threshold gauges. Provider/Kind/Lane are the
// closed-enum/slug labels; the thresholds are the operator-declared (effective)
// values.
type LaneRosterObservation struct {
	Lane                      string
	Provider                  string
	Kind                      string
	StalenessThresholdSeconds float64
	ExpiryLeadSeconds         float64
}

// LaneCredSampleObservation is one resolver-proven credential sample. Present is
// implicit (a returned sample is present); HasExpiry distinguishes an OAuth
// sample (with SecondsToExpiry/AgeSeconds) from an api_key sample (present, no
// expiry series).
type LaneCredSampleObservation struct {
	Lane            string
	Kind            string
	HasExpiry       bool
	SecondsToExpiry float64
	AgeSeconds      float64
}

// LaneResolverMismatchObservation is one lane whose runtime credential source
// could not be proven (F2 fail-closed). It pages and emits no green series.
type LaneResolverMismatchObservation struct {
	Lane string
	Kind string
}

// LaneAuthSuccessObservation is one lane's most-recent real auth success (the
// codex-scoped Layer 3 heartbeat), folded from the MAX(created_at) of
// lane.auth_success events.
type LaneAuthSuccessObservation struct {
	Lane             string
	TimestampSeconds float64
}

// laneProviderKind keys the expected vector.
type laneProviderKind struct {
	lane     string
	provider string
	kind     string
}

// laneKind keys the per-(lane,kind) families.
type laneKind struct {
	lane string
	kind string
}

// laneAuth holds the folded lane-auth families. A nil value renders only the
// HELP/TYPE headers (the empty-but-valid surface).
type laneAuth struct {
	expected            map[laneProviderKind]int
	samplePresent       map[laneKind]int
	resolverMismatch    map[laneKind]int
	secondsToExpiry     map[laneKind]float64
	ageSeconds          map[laneKind]float64
	stalenessThreshold  map[string]float64
	expiryLead          map[string]float64
	lastSuccessUnixSecs map[string]float64
}

func newLaneAuth() *laneAuth {
	return &laneAuth{
		expected:            map[laneProviderKind]int{},
		samplePresent:       map[laneKind]int{},
		resolverMismatch:    map[laneKind]int{},
		secondsToExpiry:     map[laneKind]float64{},
		ageSeconds:          map[laneKind]float64{},
		stalenessThreshold:  map[string]float64{},
		expiryLead:          map[string]float64{},
		lastSuccessUnixSecs: map[string]float64{},
	}
}

// foldLaneAuth builds the lane-auth families from the snapshot input and applies
// the per-family lane series budget, recording any clips into the shared
// cardinality-clip counter. It is pure (no DB / FS), so the unit tests drive it
// through Build exactly like every other family.
func foldLaneAuth(in SnapshotInput, clipped map[string]int) *laneAuth {
	la := newLaneAuth()
	for _, r := range in.LaneAuthRoster {
		lane := safeLaneLabel(r.Lane)
		key := laneProviderKind{lane: lane, provider: safeLaneLabel(r.Provider), kind: safeLaneLabel(r.Kind)}
		la.expected[key] = 1
		if r.StalenessThresholdSeconds > 0 {
			la.stalenessThreshold[lane] = r.StalenessThresholdSeconds
		}
		if r.ExpiryLeadSeconds > 0 {
			la.expiryLead[lane] = r.ExpiryLeadSeconds
		}
	}
	for _, s := range in.LaneCredSamples {
		key := laneKind{lane: safeLaneLabel(s.Lane), kind: safeLaneLabel(s.Kind)}
		la.samplePresent[key] = 1
		if s.HasExpiry {
			la.secondsToExpiry[key] = s.SecondsToExpiry
			la.ageSeconds[key] = s.AgeSeconds
		}
	}
	for _, m := range in.LaneResolverMismatches {
		la.resolverMismatch[laneKind{lane: safeLaneLabel(m.Lane), kind: safeLaneLabel(m.Kind)}] = 1
	}
	for _, h := range in.LaneAuthSuccesses {
		lane := safeLaneLabel(h.Lane)
		if existing, ok := la.lastSuccessUnixSecs[lane]; !ok || h.TimestampSeconds > existing {
			la.lastSuccessUnixSecs[lane] = h.TimestampSeconds
		}
	}

	la.applyBudget(clipped)
	return la
}

// applyBudget caps each lane-keyed family at laneAuthSeriesBudget distinct lanes,
// collapsing overflow lanes onto a single lane="other" sentinel and recording the
// clip count. The two threshold gauges share one budget pass (they are keyed by
// the same lane set) so a clip is reported once for both.
func (la *laneAuth) applyBudget(clipped map[string]int) {
	if n := clipExpected(la.expected, laneAuthSeriesBudget); n > 0 {
		clipped[clipFamilyLaneAuthExpected] = n
	}
	if n := clipLaneKind(la.samplePresent, laneAuthSeriesBudget, 1); n > 0 {
		clipped[clipFamilyLaneCredSamplePresent] = n
	}
	if n := clipLaneKind(la.resolverMismatch, laneAuthSeriesBudget, 1); n > 0 {
		clipped[clipFamilyLaneCredResolverMismatch] = n
	}
	if n := clipLaneKindFloat(la.secondsToExpiry, laneAuthSeriesBudget); n > 0 {
		clipped[clipFamilyLaneCredSecondsToExpiry] = n
	}
	if n := clipLaneKindFloat(la.ageSeconds, laneAuthSeriesBudget); n > 0 {
		clipped[clipFamilyLaneCredAge] = n
	}
	if n := clipLaneFloat(la.lastSuccessUnixSecs, laneAuthSeriesBudget); n > 0 {
		clipped[clipFamilyLaneAuthLastSuccess] = n
	}
	thresholdClips := clipLaneFloat(la.stalenessThreshold, laneAuthSeriesBudget)
	if n := clipLaneFloat(la.expiryLead, laneAuthSeriesBudget); n > thresholdClips {
		thresholdClips = n
	}
	if thresholdClips > 0 {
		clipped[clipFamilyLaneAuthThresholds] = thresholdClips
	}
}

// keptLanes returns the lexicographically-smallest `limit` lanes from the set,
// plus the count of clipped lanes. A deterministic sorted cap (not a stateful
// LRU) mirrors applySeriesBudget so the surface stays byte-stable.
func keptLanes(lanes map[string]bool, limit int) (map[string]bool, int) {
	if limit < 0 {
		limit = 0
	}
	if len(lanes) <= limit {
		return lanes, 0
	}
	all := make([]string, 0, len(lanes))
	for lane := range lanes {
		all = append(all, lane)
	}
	sort.Strings(all)
	keep := map[string]bool{}
	for _, lane := range all[:limit] {
		keep[lane] = true
	}
	return keep, len(all) - limit
}

func clipExpected(m map[laneProviderKind]int, limit int) int {
	lanes := map[string]bool{}
	for k := range m {
		lanes[k.lane] = true
	}
	keep, clipped := keptLanes(lanes, limit)
	if clipped == 0 {
		return 0
	}
	for k := range m {
		if !keep[k.lane] {
			delete(m, k)
		}
	}
	m[laneProviderKind{lane: laneLabelOther, provider: laneLabelOther, kind: laneLabelOther}] = 1
	return clipped
}

func clipLaneKind(m map[laneKind]int, limit, sentinel int) int {
	lanes := map[string]bool{}
	for k := range m {
		lanes[k.lane] = true
	}
	keep, clipped := keptLanes(lanes, limit)
	if clipped == 0 {
		return 0
	}
	for k := range m {
		if !keep[k.lane] {
			delete(m, k)
		}
	}
	m[laneKind{lane: laneLabelOther, kind: laneLabelOther}] = sentinel
	return clipped
}

func clipLaneKindFloat(m map[laneKind]float64, limit int) int {
	lanes := map[string]bool{}
	for k := range m {
		lanes[k.lane] = true
	}
	keep, clipped := keptLanes(lanes, limit)
	if clipped == 0 {
		return 0
	}
	for k := range m {
		if !keep[k.lane] {
			delete(m, k)
		}
	}
	m[laneKind{lane: laneLabelOther, kind: laneLabelOther}] = 0
	return clipped
}

func clipLaneFloat(m map[string]float64, limit int) int {
	lanes := map[string]bool{}
	for lane := range m {
		lanes[lane] = true
	}
	keep, clipped := keptLanes(lanes, limit)
	if clipped == 0 {
		return 0
	}
	for lane := range m {
		if !keep[lane] {
			delete(m, lane)
		}
	}
	m[laneLabelOther] = 0
	return clipped
}

// sortedLaneProviderKinds / sortedLaneKinds / sortedLaneFloatKeys return a
// family's keys in deterministic order so the rendered body is byte-stable.
func sortedLaneProviderKinds(m map[laneProviderKind]int) []laneProviderKind {
	keys := make([]laneProviderKind, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].lane != keys[j].lane {
			return keys[i].lane < keys[j].lane
		}
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		return keys[i].kind < keys[j].kind
	})
	return keys
}

func sortedLaneKindsInt(m map[laneKind]int) []laneKind {
	keys := make([]laneKind, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortLaneKinds(keys)
	return keys
}

func sortedLaneKindsFloat(m map[laneKind]float64) []laneKind {
	keys := make([]laneKind, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortLaneKinds(keys)
	return keys
}

func sortLaneKinds(keys []laneKind) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].lane != keys[j].lane {
			return keys[i].lane < keys[j].lane
		}
		return keys[i].kind < keys[j].kind
	})
}

func sortedLaneFloatKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
