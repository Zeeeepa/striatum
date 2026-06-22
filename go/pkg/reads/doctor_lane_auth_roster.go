package reads

// RFC 0162 — the Backbone-roster reconciliation check. It corroborates the
// Slack-paging census/resolver alerts (it is NOT the page itself) by surfacing
// roster-vs-observation drift through the existing striatum_doctor_problems{class}
// family: only the STATIC check codes below ever become the `class` label — the
// lane slug travels in a detail field the doctor_problems fold never reads (F-A8),
// exactly like every other doctor check.

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
)

// Static reconciliation check codes (the only values that may become the
// doctor_problems `class` label). Each is lowercase snake_case so it matches the
// fold's static-code shape and can never carry a dynamic id.
const (
	checkLaneAuthUnrosteredLiveLane = "lane_auth_unrostered_live_lane"
	checkLaneAuthMissingSample      = "lane_auth_missing_sample"
	checkLaneAuthResolverMismatch   = "lane_auth_resolver_mismatch"
)

// laneAuthReconcileWindow is the default SLA window: a roster lane that emitted no
// auth-success within this window is flagged as missing a sample. It mirrors the
// metrics sweep cadence headroom; the operator can widen it per deployment.
const laneAuthReconcileWindow = 30 * time.Minute

// LaneAuthObservedState is the observed side of the reconciliation: the lanes seen
// authenticating recently and the lanes currently failing closed.
type LaneAuthObservedState struct {
	// LiveLanes are lanes that emitted a real auth-success (or sample) within the
	// window — keyed by lane slug (OS user).
	LiveLanes map[string]bool
	// MismatchLanes are lanes whose runtime credential source could not be proven
	// this sweep (resolver fail-closed).
	MismatchLanes map[string]bool
}

// ReconcileLaneAuthRoster compares the declared roster against observed state and
// returns the STATIC problem records the doctor_problems gauge folds:
//
//	(a) a live lane with NO roster entry  → lane_auth_unrostered_live_lane
//	(b) a roster lane with NO recent sample → lane_auth_missing_sample
//	(c) a lane standing in resolver_mismatch → lane_auth_resolver_mismatch
//
// It is pure (no DB/FS) so the reconciliation contract is testable at one point.
// Records are returned in a deterministic order.
func ReconcileLaneAuthRoster(roster laneproviderauth.Roster, observed LaneAuthObservedState) []map[string]any {
	rostered := map[string]bool{}
	for _, e := range roster.Entries {
		rostered[e.Lane] = true
	}

	var records []map[string]any

	// (b) rostered lanes with no recent sample.
	for _, e := range roster.Entries {
		if !observed.LiveLanes[e.Lane] {
			records = append(records, laneAuthRecord(checkLaneAuthMissingSample, e.Lane))
		}
	}

	// (a) live lanes that are not in the roster (a census denominator gap).
	for _, lane := range sortedKeys(observed.LiveLanes) {
		if !rostered[lane] {
			records = append(records, laneAuthRecord(checkLaneAuthUnrosteredLiveLane, lane))
		}
	}

	// (c) lanes currently failing closed.
	for _, lane := range sortedKeys(observed.MismatchLanes) {
		records = append(records, laneAuthRecord(checkLaneAuthResolverMismatch, lane))
	}

	return records
}

func laneAuthRecord(check, lane string) map[string]any {
	// `check` is the only field the doctor_problems fold reads; `lane` is a bounded
	// roster slug carried for human doctor output, never the wire class label.
	return map[string]any{"check": check, "lane": lane}
}

// LaneAuthLiveLanesFromEvents reads the lanes that emitted a lane.auth_success
// within laneAuthReconcileWindow of now from the durable events ledger — the
// observed LiveLanes for the reconciliation. It selects only the closed-enum
// lane_user payload tag, never a run/session/repo id, path, or byline.
// Best-effort: the caller treats an error as "no observation this sweep".
func LaneAuthLiveLanesFromEvents(ctx context.Context, runner db.Runner, now time.Time) (map[string]bool, error) {
	since := now.Add(-laneAuthReconcileWindow)
	rows, err := collectRows(ctx, runner, `
		SELECT DISTINCT payload_json->>'lane_user' AS lane_user
		  FROM striatumd.events
		 WHERE event_type = 'lane.auth_success'
		   AND created_at >= $1
		   AND COALESCE(payload_json->>'lane_user', '') <> ''`, since.UTC())
	if err != nil {
		return nil, err
	}
	live := map[string]bool{}
	for _, row := range rows {
		if lane, ok := row["lane_user"].(string); ok {
			if lane = strings.TrimSpace(lane); lane != "" {
				live[lane] = true
			}
		}
	}
	return live, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
