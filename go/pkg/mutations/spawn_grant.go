package mutations

import (
	"context"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// spawnGrantTTL bounds a spawn-authorization grant's lifetime as a backstop
// beyond the primary run-terminal + revoke bounds (RFC 0122 §6). A run rarely
// runs for days; a generous-but-finite expiry means a forgotten grant cannot
// authorize daemon spawns indefinitely, and gives the Phase 4 poisoned-spawn
// "expired grant" case a real edge to assert against.
const spawnGrantTTL = 7 * 24 * time.Hour

// workflowHasAutoSpawnLane reports whether any lane in the run's workflow snapshot
// opts into supervision.auto_spawn (RFC 0122 §1) — the trigger for capturing a
// spawn-authorization grant at run.start. A workflow-wide default
// (workflow.supervision.auto_spawn) applies to every lane that does not override
// it; default false everywhere reproduces today's behavior exactly.
func workflowHasAutoSpawnLane(workflow map[string]any) bool {
	lanes := asMap(workflow["lanes"])
	for _, raw := range lanes {
		if laneAutoSpawn(workflow, asMap(raw)) {
			return true
		}
	}
	return false
}

// laneAutoSpawn resolves the effective auto_spawn flag for one lane: an explicit
// lane supervision.auto_spawn wins; absent that, the workflow-wide default
// (workflow.supervision.auto_spawn) applies; absent both, false.
func laneAutoSpawn(workflow, lane map[string]any) bool {
	if supervision, ok := lane["supervision"].(map[string]any); ok {
		if value, ok := boolLaneValue(supervision, "auto_spawn"); ok {
			return value
		}
	}
	if supervision, ok := workflow["supervision"].(map[string]any); ok {
		if value, ok := boolLaneValue(supervision, "auto_spawn"); ok {
			return value
		}
	}
	return false
}

// spawnRunAsSpec resolves the run-as identity captured into the grant at
// run.start (RFC 0122 §4). The scheduler later reads this spec; it never chooses
// a run-as identity for a lane no operator asked for. Resolution mirrors the
// supervise.start path (configuredLaneRunAsUser): an empty result is daemon-user
// mode; a configured lane OS user that cannot be resolved on the host is a LOUD
// refusal (spawn_run_as_unresolved) BEFORE any auto-spawn is possible — a
// scheduler must never spawn into a non-existent identity.
func spawnRunAsSpec() (map[string]any, error) {
	runAsUser := configuredLaneRunAsUser()
	if runAsUser == "" {
		return map[string]any{"mode": "daemon_user", "run_as_user": ""}, nil
	}
	if laneOSUserHome(runAsUser) == "" {
		return nil, rpc.NewError("spawn_run_as_unresolved", "auto_spawn run-as identity "+runAsUser+
			" cannot be resolved on this host (no such OS user / no home directory); a scheduler must not spawn into a non-existent identity. Provision the lane user or unset "+
			supervisedLaneOSUserEnv+" to run lanes as the daemon user.", nil)
	}
	return map[string]any{"mode": "lane_user", "run_as_user": runAsUser}, nil
}

// spawnCapabilityEnvelope is the capability set + TTL the scheduler mint anchors
// on (RFC 0122 §3). It is the SAME session-bound grant set the operator-driven
// supervise.start path mints (sessionBoundCapabilities / sessionBoundTokenTTL),
// captured verbatim so a scheduler-minted token is identical to an operator-minted
// one.
func spawnCapabilityEnvelope() map[string]any {
	capabilities := make([]any, 0, len(sessionBoundCapabilities))
	for _, capability := range sessionBoundCapabilities {
		capabilities = append(capabilities, string(capability))
	}
	return map[string]any{
		"capabilities": capabilities,
		"ttl_seconds":  int(sessionBoundTokenTTL / time.Second),
	}
}

// captureSpawnAuthorizationGrant persists the run-owner pre-authorization grant
// (RFC 0122 §2) inside the run.start transaction, so it commits atomically with
// the run becoming `running`. ownerPrincipalID is the attribution identity the
// authority prelude installs as striatum.principal_id (the run owner's client id);
// the scheduler replays it, preserving attestation (C1). It refuses loudly when
// no authenticated owner principal is present (an auto_spawn run cannot be
// replayed against an anonymous owner) or when the run-as identity is unresolved.
func captureSpawnAuthorizationGrant(ctx context.Context, tx db.TxRunner, repositoryID, runID, ownerPrincipalID string) error {
	if ownerPrincipalID == "" {
		return rpc.NewError("spawn_grant_no_owner_principal",
			"auto_spawn run.start has no authenticated owner principal to capture; the daemon scheduler replays the run owner's authority and cannot do so anonymously. Authenticate run.start with a capability token carrying a principal.", nil)
	}
	runAsSpec, err := spawnRunAsSpec()
	if err != nil {
		return err
	}
	runAsArg, err := db.JSONBArg(tx, runAsSpec)
	if err != nil {
		return err
	}
	envelopeArg, err := db.JSONBArg(tx, spawnCapabilityEnvelope())
	if err != nil {
		return err
	}
	grantID, err := newID("sgrant")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(spawnGrantTTL)
	// One active grant per run (partial unique index): a re-capture for a run that
	// already holds a live grant is a no-op, so a re-entrant or retried run.start
	// cannot stack grants or fail spuriously.
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.spawn_authorization_grants (
		  repository_id, grant_id, run_id, owner_principal_id, run_as_spec,
		  capability_envelope, created_at, expires_at
		)
		SELECT $1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8
		 WHERE NOT EXISTS (
		   SELECT 1 FROM striatumd.spawn_authorization_grants
		    WHERE repository_id = $1 AND run_id = $3 AND revoked_at IS NULL
		 )`,
		repositoryID, grantID, runID, ownerPrincipalID, runAsArg, envelopeArg,
		now.Format(time.RFC3339Nano), expiresAt.Format(time.RFC3339Nano),
	); err != nil {
		return err
	}
	return nil
}

// revokeSpawnAuthorizationGrant marks a run's active grant revoked (RFC 0122 §6).
// Called from the terminal-finalization chokepoint so the scheduler refuses any
// further spawn the moment a run reaches terminal state or is stopped. Idempotent
// (a no-op when no active grant exists). The grants table is a runtime migration
// (0027), so it is present for every daemon — no table-absence handling needed.
func revokeSpawnAuthorizationGrant(ctx context.Context, runner any, repositoryID, runID, reason string) error {
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil
	}
	return exec.Exec(ctx, `
		UPDATE striatumd.spawn_authorization_grants
		   SET revoked_at = $1, revoke_reason = $2
		 WHERE repository_id = $3 AND run_id = $4 AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339Nano), reason, repositoryID, runID)
}
