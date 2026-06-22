package metrics

// RFC 0162 — the Collector-side folds for the lane-auth families, run best-effort
// on the recovery-sweep cadence (NEVER the scrape path):
//
//   - laneAuthRosterObservations loads the operator-declared Backbone roster
//     (the census expected set + the OQ4 thresholds);
//   - sampleLaneCredentials is the Layer 1 sampler: a read-only file sample (no
//     provider round-trip) that resolves each lane's credential via the F2
//     resolver contract, reads it AS THE LANE USER bounded by a per-lane timeout,
//     and extracts expiry — failing closed into a resolver_mismatch rather than a
//     green gauge;
//   - laneAuthSuccessObservations folds the codex-scoped Layer 3 heartbeat from
//     the durable lane.auth_success events (MAX timestamp per lane).
//
// All three are best-effort: an error degrades the affected family to empty and
// flips the tick to partial, exactly like the existing Phase B/D folds.

import (
	"context"
	"errors"
	"os/user"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
)

// laneAuthSampleTimeout bounds the per-lane credential read so a slow/hung
// sudo/file read can never stall the recovery sweep that drives the fold (the
// spirit of doctorFoldTimeout).
const laneAuthSampleTimeout = 10 * time.Second

// laneAuthSuccessEventType is the durable event the Layer 3 heartbeat writes on a
// real provider-auth success.
const laneAuthSuccessEventType = "lane.auth_success"

// laneCredentialReader is the seam the sampler reads lane credentials through. It
// defaults to the production sudo-as-lane reader; tests override it to read
// in-memory without sudo. The arguments mirror laneproviderauth.LaneFileReader.
var laneCredentialReader = func(ctx context.Context, runAsUser string, launchEnv []string) laneproviderauth.CredentialReader {
	return laneproviderauth.LaneFileReader(ctx, laneproviderauth.ExecRunner{}, runAsUser, launchEnv)
}

// laneRosterLoader is the seam the fold loads the roster through (overridable in
// tests). It defaults to reading the daemon-config roster path.
var laneRosterLoader = func() (laneproviderauth.Roster, error) {
	return laneproviderauth.LoadRoster(laneproviderauth.DefaultRosterPath())
}

// laneOSUserHome resolves a lane OS user's home dir for the sampler launch env. A
// var so tests can avoid a real OS lookup.
var laneOSUserHome = func(name string) string {
	u, err := user.Lookup(name)
	if err != nil || strings.TrimSpace(u.HomeDir) == "" {
		return ""
	}
	return u.HomeDir
}

// laneAuthRosterObservations loads the roster and projects it into the census
// expected vector + the OQ4 threshold gauges. A missing/empty roster yields no
// observations (no census denominator) and no error.
func laneAuthRosterObservations() (laneproviderauth.Roster, []LaneRosterObservation) {
	roster, err := laneRosterLoader()
	if err != nil {
		return laneproviderauth.Roster{}, nil
	}
	out := make([]LaneRosterObservation, 0, len(roster.Entries))
	for _, e := range roster.Entries {
		out = append(out, LaneRosterObservation{
			Lane:                      e.Lane,
			Provider:                  e.Provider,
			Kind:                      e.Kind,
			StalenessThresholdSeconds: e.EffectiveStalenessThresholdSeconds(),
			ExpiryLeadSeconds:         e.EffectiveExpiryLeadSeconds(),
		})
	}
	return roster, out
}

// sampleLaneCredentials runs the Layer 1 sampler over the roster, returning the
// resolver-proven samples and the fail-closed resolver mismatches. An absent
// credential yields neither (the per-lane census names it via expected-unless-
// sample_present). It performs NO DB query and never touches c.runner, so it is
// safe on any collector.
func sampleLaneCredentials(ctx context.Context, roster laneproviderauth.Roster, now time.Time) ([]LaneCredSampleObservation, []LaneResolverMismatchObservation) {
	var samples []LaneCredSampleObservation
	var mismatches []LaneResolverMismatchObservation
	for _, entry := range roster.Entries {
		launchEnv := laneLaunchEnv(entry)
		sampleCtx, cancel := context.WithTimeout(ctx, laneAuthSampleTimeout)
		read := laneCredentialReader(sampleCtx, entry.Lane, launchEnv)
		sample, err := laneproviderauth.SampleLaneCredential(entry, launchEnv, read)
		cancel()
		switch {
		case err == nil:
			obs := LaneCredSampleObservation{Lane: entry.Lane, Kind: entry.Kind, HasExpiry: sample.HasExpiry}
			if sample.HasExpiry {
				obs.SecondsToExpiry = sample.ExpiresAt.Sub(now).Seconds()
				if !sample.IssuedAt.IsZero() {
					obs.AgeSeconds = now.Sub(sample.IssuedAt).Seconds()
				}
			}
			samples = append(samples, obs)
		case isResolverMismatch(err):
			mismatches = append(mismatches, LaneResolverMismatchObservation{Lane: entry.Lane, Kind: entry.Kind})
			// ErrCredentialAbsent and any other error: emit no sample (the absence
			// census names the lane). Only an unprovable source pages via mismatch.
		}
	}
	return samples, mismatches
}

func isResolverMismatch(err error) bool {
	// Match the exported fail-closed sentinel, not its message text: a future
	// reword of ErrResolverMismatch's string must not silently reclassify a
	// fail-closed mismatch as "absent" and drop a page (RFC 0162 review F-1).
	return errors.Is(err, laneproviderauth.ErrResolverMismatch)
}

// laneLaunchEnv reconstructs the launch env the lane CLI resolves its credential
// from (the roster-declared launch_env keys plus the lane OS user's HOME). At fold
// time there is no live lane process, so this is a reconstruction — not a read of
// the live process env — but it is what lets the resolver prefer a provider's
// config-dir key over a daemon-side HOME decoy (RFC 0162 review F-4).
func laneLaunchEnv(entry laneproviderauth.RosterEntry) []string {
	env := []string{}
	if home := laneOSUserHome(entry.Lane); home != "" {
		env = append(env, laneproviderauth.EnvHome+"="+home)
	}
	for k, v := range entry.LaunchEnv {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		env = append(env, k+"="+v)
	}
	return env
}

// laneAuthSuccessObservations folds the codex-scoped Layer 3 heartbeat from the
// durable lane.auth_success events: the MAX success timestamp per lane_user,
// mapped to its roster slug (an unrostered lane_user folds to lane="other"). It is
// best-effort; an error is returned so the caller can degrade the tick to partial.
func (c *Collector) laneAuthSuccessObservations(ctx context.Context, roster laneproviderauth.Roster) ([]LaneAuthSuccessObservation, error) {
	rosterLanes := map[string]bool{}
	for _, e := range roster.Entries {
		rosterLanes[e.Lane] = true
	}
	rows, err := c.runner.Query(ctx, `
		SELECT payload_json->>'lane_user' AS lane_user,
		       MAX(EXTRACT(EPOCH FROM created_at))::float8 AS last_success
		  FROM striatumd.events
		 WHERE event_type = $1
		   AND COALESCE(payload_json->>'lane_user', '') <> ''
		 GROUP BY 1`, laneAuthSuccessEventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LaneAuthSuccessObservation{}
	for rows.Next() {
		var laneUser string
		var ts float64
		if err := rows.Scan(&laneUser, &ts); err != nil {
			return nil, err
		}
		lane := strings.TrimSpace(laneUser)
		if lane == "" {
			continue
		}
		if !rosterLanes[lane] {
			lane = laneLabelOther
		}
		out = append(out, LaneAuthSuccessObservation{Lane: lane, TimestampSeconds: ts})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
