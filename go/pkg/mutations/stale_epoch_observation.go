package mutations

import (
	"context"
	"strings"
)

// RFC 0143 Slice A (#512): the daemon-observed stale-epoch rotation observation.
//
// These two append-only daemon event types form the forge-resistant primary
// producer (T1) of the session_unrecoverable_across_rotation recovery floor and its
// over-fire guard (Correction 1):
//
//   - staleEpochRotationEventType is recorded when validateBootEpoch (pkg/mcp) rejects
//     a request as stale_daemon_identity, attributed to the presenting session by the
//     daemon's OWN read-only token store (no inbound authenticated frame; widens
//     nothing; the request stays rejected). It is the daemon observing its own
//     rejection — durable, daemon-side, idempotent.
//   - staleEpochRecoveredEventType SUPERSEDES the observation when that same session
//     later passes the epoch check with the CURRENT live epoch (it reconnected). After
//     a recovery the observation is no longer LIVE, so a later ORDINARY death of the
//     session classifies agent_exited_unsealed, NOT the typed floor (Correction 1, the
//     no-over-fire invariant the v2 falsifiers required).
const (
	staleEpochRotationEventType  = "daemon.stale_epoch_rotation"
	staleEpochRecoveredEventType = "daemon.stale_epoch_recovered"
)

// RecordStaleEpochRejection appends a durable daemon.stale_epoch_rotation observation
// for the presenting session (RFC 0143 Slice A §3.3). It is idempotent in EFFECT (one
// standing LIVE observation per session is sufficient; appending another is harmless
// and the LIVE-observation predicate reads the latest) and best-effort at the call
// site: the HTTP handler ignores its error so a record failure never alters the 403.
// It grants nothing — it records an observation row, never a capability.
func RecordStaleEpochRejection(ctx context.Context, runner any, repositoryID, runID, sessionID string) error {
	repositoryID = strings.TrimSpace(repositoryID)
	sessionID = strings.TrimSpace(sessionID)
	if repositoryID == "" || sessionID == "" {
		return nil
	}
	payload := map[string]any{
		"session_id": sessionID,
		"source":     "validate_boot_epoch",
		"reason":     "stale_daemon_identity",
	}
	_, err := appendEvent(ctx, runner, repositoryID, nullable(emptyToNil(runID)), staleEpochRotationEventType, sessionID, nil, nil, nil, nil, payload)
	return err
}

// RecordStaleEpochRecovered appends a daemon.stale_epoch_recovered event that
// SUPERSEDES any LIVE stale-epoch observation for the session (Correction 1). It is
// recorded when the session next presents the CURRENT live boot epoch (it
// reconnected across the rotation), so the typed floor will not fire on a later
// ordinary death. Best-effort + idempotent; only writes when a LIVE observation
// actually exists, so a healthy session that never recorded one stays event-free.
func RecordStaleEpochRecovered(ctx context.Context, runner any, repositoryID, runID, sessionID string) error {
	repositoryID = strings.TrimSpace(repositoryID)
	sessionID = strings.TrimSpace(sessionID)
	if repositoryID == "" || sessionID == "" {
		return nil
	}
	live, err := hasLiveStaleEpochObservation(ctx, runner, repositoryID, sessionID)
	if err != nil {
		return err
	}
	if !live {
		return nil // nothing to supersede; do not litter the event chain.
	}
	payload := map[string]any{
		"session_id": sessionID,
		"source":     "validate_boot_epoch",
		"reason":     "epoch_revalidated",
	}
	_, err = appendEvent(ctx, runner, repositoryID, nullable(emptyToNil(runID)), staleEpochRecoveredEventType, sessionID, nil, nil, nil, nil, payload)
	return err
}

// hasLiveStaleEpochObservation reports whether the session currently has a LIVE
// (un-superseded) daemon.stale_epoch_rotation observation: a rotation event whose
// event_id is greater than the latest stale-epoch-recovered event for the same
// session (or any rotation event when no recovered event exists). This is the exact
// Correction-1 predicate: a session that recorded an observation, then RECOVERED, has
// no LIVE observation, so the typed floor does not fire for it.
//
// It is read-only and uses ONLY durable daemon state (striatumd.events). No inbound
// frame, no capability.
func hasLiveStaleEpochObservation(ctx context.Context, runner any, repositoryID, sessionID string) (bool, error) {
	repositoryID = strings.TrimSpace(repositoryID)
	sessionID = strings.TrimSpace(sessionID)
	if repositoryID == "" || sessionID == "" {
		return false, nil
	}
	return existsRow(ctx, runner, `
		SELECT 1
		  FROM striatumd.events rot
		 WHERE rot.repository_id = $1
		   AND rot.actor_session_id = $2
		   AND rot.event_type = $3
		   AND rot.event_id > COALESCE((
		         SELECT MAX(rec.event_id)
		           FROM striatumd.events rec
		          WHERE rec.repository_id = $1
		            AND rec.actor_session_id = $2
		            AND rec.event_type = $4
		       ), 0)
		 LIMIT 1`,
		repositoryID, sessionID, staleEpochRotationEventType, staleEpochRecoveredEventType)
}

// emptyToNil maps an empty run id to nil so appendEvent stores NULL run_id (the
// observation is keyed primarily on the session; run_id is best-effort context).
func emptyToNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
