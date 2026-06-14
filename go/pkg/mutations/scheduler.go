package mutations

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/runreconcile"
)

// spawnGrant is a run's active spawn-authorization grant, loaded for the daemon
// auto_spawn scheduler (RFC 0122). Expired carries the SQL-evaluated expiry so
// the scheduler never has to parse timestamps to enforce C2 (no authority
// invention: a missing/expired/revoked grant must loud-fail, never silently
// spawn).
type spawnGrant struct {
	GrantID            string
	OwnerPrincipalID   string
	RunAsSpec          map[string]any
	CapabilityEnvelope map[string]any
	Expired            bool
}

// loadActiveSpawnGrant returns the run's single active (un-revoked) grant, or nil
// when none exists. The expiry comparison is evaluated in SQL so the caller works
// in booleans, not parsed timestamps.
func loadActiveSpawnGrant(ctx context.Context, runner any, repositoryID, runID string) (*spawnGrant, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT grant_id, owner_principal_id, run_as_spec, capability_envelope,
		       (expires_at IS NOT NULL AND expires_at <= now()) AS expired
		  FROM striatumd.spawn_authorization_grants
		 WHERE repository_id = $1 AND run_id = $2 AND revoked_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT 1`, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	expired, _ := row["expired"].(bool)
	return &spawnGrant{
		GrantID:            fmt.Sprint(row["grant_id"]),
		OwnerPrincipalID:   fmt.Sprint(row["owner_principal_id"]),
		RunAsSpec:          asMap(row["run_as_spec"]),
		CapabilityEnvelope: asMap(row["capability_envelope"]),
		Expired:            expired,
	}, nil
}

// plannedSpawn is one lane the scheduler will register + supervise for a queued
// job, derived from the shared reconcile predicate.
type plannedSpawn struct {
	Role string
	Lane string
	Slot runreconcile.SlotKey
}

// autoSpawnPlan is the scheduler's decision for one run: whether it is drivable,
// the grant that authorizes spawning, and the lanes to spawn this pass.
type autoSpawnPlan struct {
	Drivable bool
	Reason   string
	Grant    *spawnGrant
	Spawns   []plannedSpawn
}

// planAutoSpawn computes the scheduler's spawn decision for a run WITHOUT side
// effects, so it is fully testable. It runs the SAME reconcile predicate
// (runreconcile.PlanLaunch) that `run drive` runs — the C3 parity guarantee — then
// keeps only the register-fresh decisions for lanes that opted into auto_spawn:
//
//   - terminal / non-running run -> not drivable (no spawn).
//   - paused run -> not drivable (C5 hold: a human paused it deliberately).
//   - LaunchAdoptExisting decisions -> skipped: a live session already serves the
//     slot, which is the scheduler's idempotent double-spawn guard (C4).
//   - non-auto_spawn lanes -> skipped: a job a human means to hold is expressed as
//     the absence of auto_spawn (RFC 0122 §6 human-hold).
//
// C2 (no authority invention) is enforced last: if there is auto_spawn work to do
// but no valid, unexpired grant, planAutoSpawn returns a loud error — never a
// silent spawn.
func planAutoSpawn(ctx context.Context, runner any, repositoryID, runID string) (autoSpawnPlan, error) {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
	if err != nil {
		return autoSpawnPlan{}, err
	}
	state := fmt.Sprint(run["state"])
	if isTerminalRunState(state) {
		return autoSpawnPlan{Drivable: false, Reason: "terminal:" + state}, nil
	}
	if state != "running" {
		return autoSpawnPlan{Drivable: false, Reason: "not_running:" + state}, nil
	}
	grant, err := loadActiveSpawnGrant(ctx, runner, repositoryID, runID)
	if err != nil {
		return autoSpawnPlan{}, err
	}
	if runreconcile.IsPausedRun(run) {
		return autoSpawnPlan{Drivable: false, Reason: "paused", Grant: grant}, nil
	}
	workflow, err := workflowForRun(ctx, runner, repositoryID, run)
	if err != nil {
		return autoSpawnPlan{}, err
	}
	jobs, err := queryRows(ctx, runner, `
		SELECT job_id, workflow_job_id, role_id, state, attempt, lane_selector_json
		  FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID)
	if err != nil {
		return autoSpawnPlan{}, err
	}
	sessions, err := queryRows(ctx, runner, `
		SELECT session_id, role_id, lane_id, state
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND run_id = $2`, repositoryID, runID)
	if err != nil {
		return autoSpawnPlan{}, err
	}
	plan := autoSpawnPlan{Drivable: true, Grant: grant}
	for _, decision := range runreconcile.PlanLaunch(jobs, workflow, sessions, nil) {
		if decision.Kind != runreconcile.LaunchRegisterFresh {
			continue
		}
		if !laneAutoSpawn(workflow, laneConfig(workflow, decision.Lane)) {
			continue
		}
		plan.Spawns = append(plan.Spawns, plannedSpawn{Role: decision.Role, Lane: decision.Lane, Slot: decision.Slot})
	}
	if len(plan.Spawns) > 0 {
		if grant == nil {
			return plan, rpc.NewError("spawn_grant_missing", fmt.Sprintf(
				"run %s has auto_spawn lanes with queued work but no active spawn-authorization grant; the daemon scheduler cannot invent authority (RFC 0122 C2). Re-run run.start to capture a grant.", runID), nil)
		}
		if grant.Expired {
			return plan, rpc.NewError("spawn_grant_expired", fmt.Sprintf(
				"run %s spawn-authorization grant %s has expired; the daemon scheduler refuses to spawn under a stale grant (RFC 0122 C2). Re-authorize by restarting the run.", runID, grant.GrantID), nil)
		}
	}
	return plan, nil
}

// spawnExecutor performs the two side-effecting steps of a scheduler spawn:
// registering a session and starting its supervisor. It is an interface so the
// scheduler's decision/attribution wiring is testable without launching a real
// lane subprocess; the production executor (realSpawnExecutor) replays the exact
// operator path.
type spawnExecutor interface {
	Spawn(ctx context.Context, runner db.Runner, repositoryID, runID, role, lane string) (string, error)
}

// realSpawnExecutor registers a fresh session and starts its supervisor through
// the SAME production handlers the operator-driven path uses (HandleRegisterSession
// + HandleSuperviseStart). The minting, attestation, run-as, and audit are
// therefore identical to a manual supervise.start (C1) — the only difference is
// the authority prelude's principal, which the scheduler sets to the captured run
// owner via the ctx it is handed.
type realSpawnExecutor struct{}

func (realSpawnExecutor) Spawn(ctx context.Context, runner db.Runner, repositoryID, runID, role, lane string) (string, error) {
	reg, err := HandleRegisterSession(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
		"run_id": runID,
		"role":   role,
		"lane":   lane,
		"fresh":  true,
	}))
	if err != nil {
		return "", err
	}
	sessionID := fmt.Sprint(reg["session_id"])
	if sessionID == "" || sessionID == "<nil>" {
		return "", fmt.Errorf("scheduler register returned no session id for %s/%s", role, lane)
	}
	if _, err := HandleSuperviseStart(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
		"session_id":         sessionID,
		"provider_auth_gate": "auto",
	})); err != nil {
		// Mirror run drive: a failed start closes the dangling session so the slot
		// is eligible for relaunch on the next sweep rather than wedging.
		_, _ = HandleCloseSession(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
			"session_id": sessionID,
			"reason":     "scheduler supervise start failed",
		}))
		return sessionID, err
	}
	return sessionID, nil
}

// SchedulerSpawnOnce reconciles one run's auto_spawn lanes: it plans the spawn
// decision (planAutoSpawn) and, for a drivable run with a valid grant, registers +
// supervises each queued auto_spawn lane under the captured owner principal. It is
// the daemon-side analog of one `run drive` reconcile pass, with the spawn
// authority sourced from the grant rather than a live operator credential.
func SchedulerSpawnOnce(ctx context.Context, runner db.Runner, repositoryID, runID, author string) (map[string]any, error) {
	return schedulerSpawnOnce(ctx, runner, repositoryID, runID, author, realSpawnExecutor{})
}

func schedulerSpawnOnce(ctx context.Context, runner db.Runner, repositoryID, runID, author string, executor spawnExecutor) (map[string]any, error) {
	plan, err := planAutoSpawn(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	if !plan.Drivable {
		return map[string]any{"status": plan.Reason, "spawned": 0}, nil
	}
	if len(plan.Spawns) == 0 {
		return map[string]any{"status": "idle", "spawned": 0}, nil
	}
	ownerCtx := spawnOwnerContext(ctx, repositoryID, plan.Grant)
	spawns := make([]map[string]any, 0, len(plan.Spawns))
	spawnedCount := 0
	for _, planned := range plan.Spawns {
		entry := map[string]any{
			"role":            planned.Role,
			"lane":            planned.Lane,
			"workflow_job_id": planned.Slot.WorkflowJobID,
			"attempt":         planned.Slot.Attempt,
		}
		sessionID, spawnErr := executor.Spawn(ownerCtx, runner, repositoryID, runID, planned.Role, planned.Lane)
		if spawnErr != nil {
			entry["result"] = "error"
			entry["error"] = spawnErr.Error()
		} else {
			entry["result"] = "spawned"
			entry["session_id"] = sessionID
			spawnedCount++
		}
		spawns = append(spawns, entry)
	}
	return map[string]any{
		"status":  "active",
		"spawned": spawnedCount,
		"spawns":  spawns,
	}, nil
}

// spawnOwnerContext installs the captured run owner as the calling principal for
// the spawn mutations. The authority prelude (BeginAuthorizedMutation) reads
// AuthorityFromContext(ctx).PrincipalID — i.e. the AuthContext ClientID — into
// striatum.principal_id, so a scheduler spawn is attributed to the run owner
// exactly as if they had called supervise.start themselves (C1 attestation
// parity). The scheduler replays captured authority; it never mints a synthetic
// identity.
func spawnOwnerContext(ctx context.Context, repositoryID string, grant *spawnGrant) context.Context {
	owner := ""
	if grant != nil {
		owner = grant.OwnerPrincipalID
	}
	return rpc.WithAuthContext(ctx, rpc.AuthContext{
		ClientID:     owner,
		RepositoryID: repositoryID,
	})
}

// schedulerEnvelope builds the RPC envelope the scheduler hands to the spawn
// handlers. repository_id is required by every handler; the auth identity travels
// on the context (spawnOwnerContext), not the envelope.
func schedulerEnvelope(repositoryID string, params map[string]any) rpc.Envelope {
	p := map[string]any{"repository_id": repositoryID}
	for key, value := range params {
		p[key] = value
	}
	return rpc.Envelope{SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "scheduler", Params: p}
}
