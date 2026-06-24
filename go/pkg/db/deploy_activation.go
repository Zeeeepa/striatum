package db

import (
	"context"
	"os"
	"strings"
)

// DeployDecoupledEnabled reports whether the operator opted serve-boot into the
// decoupled ConnectAndVerify path via STRIATUM_DEPLOY_DECOUPLED. Default
// (unset/empty/false) is the legacy ConnectAndMigrate apply-on-boot path.
func DeployDecoupledEnabled() bool {
	return envTruthy(os.Getenv(EnvDeployDecoupled))
}

// EnvDeployDecoupled is the shadow-first opt-in that promotes serve-boot from the
// legacy ConnectAndMigrate (apply-on-boot) path to the decoupled ConnectAndVerify
// path (RFC 0142 P4). Default OFF: the existing serve-boot behaviour is unchanged
// for a no-revoke binary when the flag is absent or false. When ON, serve-boot
// verifies (W then A) and serves WITHOUT mutating schema; a pending change halts
// awaiting_deploy and `striatum daemon deploy` is the sole writer.
const EnvDeployDecoupled = "STRIATUM_DEPLOY_DECOUPLED"

// DeployBootDecision is the A-gate (CheckDeployActivation) outcome — what the boot
// path does next, mutating nothing to reach it.
type DeployBootDecision string

const (
	// DeployServeLegacy: the legacy serve-boot path may run ApplyMigrations + the
	// :399 self-record (a no-revoke flag-OFF binary over no transcript, or a
	// complete/in-sync no-revoke cursor whose apply is an idempotent no-op).
	DeployServeLegacy DeployBootDecision = "serve_legacy"
	// DeployServeVerify: the decoupled binary serves verify-only — NO ApplyMigrations,
	// NO :399 self-record (a complete/in-sync decoupled cursor, rows 15 and 16).
	DeployServeVerify DeployBootDecision = "serve_verify"
	// DeployHaltAwaitingDeploy: refuse-to-serve awaiting_deploy (BC-N2 / A3 out-of-sync).
	DeployHaltAwaitingDeploy DeployBootDecision = "awaiting_deploy"
	// DeployHaltAwaitingConfig: refuse-to-serve awaiting_deploy_config (M3 gate).
	DeployHaltAwaitingConfig DeployBootDecision = "awaiting_deploy_config"
)

// DeployActivationInputs are the PURE facts the A-gate decides on. It deliberately
// carries NO owner watermark (applied_owner): A is owner-watermark-independent
// (INVARIANT W→A-INDEPENDENCE, §0.2), so the `==0` and `==20` columns are
// identical in every cursor row (M6). On the decoupled complete branch it reads
// neither applied_owner NOR revokeEmbedded (§0.2 sub-invariant), so rows 15 and 16
// take the identical conditional outcome (M7).
type DeployActivationInputs struct {
	CursorState       string
	DecoupledEnabled  bool
	RevokeEmbedded    bool
	FingerprintInSync bool // LiveFingerprint(recorded) == ExpectedFingerprint() AND a fingerprint is recorded
	PlanHashMatch     bool // the cursor's plan targets THIS binary's frontier (not a foreign plan)
}

// DecideDeployActivation is the pure §3.3a/§3.5 A-gate predicate (fail-closed, in
// order). It mutates nothing and reads nothing — every fact is in the inputs — so
// it is the exhaustively table-drivable oracle the F18 decision-table test
// exercises over each concrete cursor-state enum and both in-sync/out-of-sync
// sub-cases (B1.1 + B1.2).
func DecideDeployActivation(in DeployActivationInputs) DeployBootDecision {
	// Step 0 (M3 — the hoisted universal decoupling-config gate, fires FIRST for
	// EVERY cursor state). The ONLY A predicate that reads revokeEmbedded; it does
	// NOT fire when decoupledEnabled == true (§0.2 sub-invariant).
	if in.RevokeEmbedded && !in.DecoupledEnabled {
		return DeployHaltAwaitingConfig
	}
	switch in.CursorState {
	case DeployStateInProgress, DeployStateStepCommitted, DeployStateFinalizing:
		// Step 1 — UNIVERSAL incomplete-deploy edge (BC-N2). DB untouched.
		return DeployHaltAwaitingDeploy
	case DeployStateAborted:
		// Step 2.
		return DeployHaltAwaitingDeploy
	case DeployStateComplete:
		// Step 3.
		inSync := in.FingerprintInSync && in.PlanHashMatch
		if in.DecoupledEnabled {
			// Decoupled complete branch: reads NEITHER applied_owner NOR
			// revokeEmbedded (M7) — rows 15 and 16, identical conditional.
			if inSync {
				return DeployServeVerify
			}
			return DeployHaltAwaitingDeploy
		}
		// !decoupledEnabled ⇒ !revokeEmbedded (step 0 caught revoke+flag-OFF): the
		// no-revoke complete comparison (M3/M6), applied_owner-independent. In-sync
		// → legacy serve (the subsequent ApplyMigrations is a no-op + the :399
		// self-record is an idempotent rewrite); out-of-sync → awaiting_deploy.
		if inSync {
			return DeployServeLegacy
		}
		return DeployHaltAwaitingDeploy
	default:
		// Step 4 — cursorState none (absent table/row, or idle): NO transcript.
		if in.DecoupledEnabled {
			// The decoupled boot never auto-applies; fresh-DB bring-up runs `deploy`.
			return DeployHaltAwaitingDeploy
		}
		// !decoupledEnabled ⇒ !revokeEmbedded: the legacy serve branch reached by
		// BOTH the fresh applied_owner==0 bootstrap (M5) and the inert-landing
		// applied_owner==20 re-boot — the ONLY none branch that legitimately reaches
		// the mutating legacy :399 writer.
		return DeployServeLegacy
	}
}

// DecisionError maps a halt decision to its typed error (DB untouched), or nil for
// a serve decision. cursorState is carried for the awaiting_deploy message.
func (d DeployBootDecision) DecisionError(cursorState string) error {
	switch d {
	case DeployHaltAwaitingConfig:
		return &AwaitingDeployConfigError{}
	case DeployHaltAwaitingDeploy:
		return &AwaitingDeployError{CursorState: cursorState}
	default:
		return nil
	}
}

// CheckDeployActivation is the DB-reading wrapper around DecideDeployActivation
// called at the boot site AFTER CheckOwnerBundleWatermark (W) and BEFORE
// ApplyMigrations / RecordSchemaFingerprint in both boot paths. It mutates
// nothing. It reads deploy_cursor defensively (absent → none), the stored plan's
// target frontier, and LiveFingerprint/ExpectedFingerprint — and DELIBERATELY does
// NOT read owner_bundle_meta / applied_owner (§0.2). It returns the decision plus
// the live cursor so the caller can render an awaiting_deploy message.
func CheckDeployActivation(ctx context.Context, runner Runner, revokeEmbedded, decoupledEnabled bool) (DeployBootDecision, DeployCursor, error) {
	cursor, err := LoadDeployCursor(ctx, runner)
	if err != nil {
		return "", cursor, err
	}
	in := DeployActivationInputs{
		CursorState:      cursor.State,
		DecoupledEnabled: decoupledEnabled,
		RevokeEmbedded:   revokeEmbedded,
	}
	// The fingerprint + plan-target reads only matter on the complete branch; skip
	// the DB work otherwise (and so a none-cursor fresh DB without schema_state does
	// not pay for an unused read).
	if cursor.State == DeployStateComplete {
		inSync, err := fingerprintInSync(ctx, runner)
		if err != nil {
			return "", cursor, err
		}
		in.FingerprintInSync = inSync
		match, err := cursorPlanTargetsThisFrontier(ctx, runner, cursor.PlanHash)
		if err != nil {
			return "", cursor, err
		}
		in.PlanHashMatch = match
	}
	return DecideDeployActivation(in), cursor, nil
}

// fingerprintInSync reports whether the recorded schema fingerprint is present and
// equals this binary's ExpectedFingerprint(). An unrecorded fingerprint ("") is
// NOT in sync (the complete branch needs a recorded match to serve verify-only).
func fingerprintInSync(ctx context.Context, runner Runner) (bool, error) {
	expected, err := ExpectedFingerprint()
	if err != nil {
		return false, err
	}
	live, err := LiveFingerprint(ctx, runner)
	if err != nil {
		return false, err
	}
	return live != "" && live == expected, nil
}

// cursorPlanTargetsThisFrontier reports whether the cursor's stored plan targets
// THIS binary's frontier (target_runtime == LatestDaemonDBVersion AND target_owner
// == the highest embedded owner bundle). A foreign or absent plan is not a match
// (it is genuine drift / a different binary's deploy), so the complete branch
// refuses. It is base-INDEPENDENT: an incremental deploy from any base that
// targeted this frontier matches, exactly as the verify path requires.
func cursorPlanTargetsThisFrontier(ctx context.Context, runner Runner, planHash string) (bool, error) {
	if strings.TrimSpace(planHash) == "" {
		return false, nil
	}
	stored, err := LoadStoredPlan(ctx, runner, planHash)
	if err != nil {
		return false, err
	}
	if stored == nil {
		return false, nil
	}
	bundles, err := OwnerBundles()
	if err != nil {
		return false, err
	}
	return stored.TargetRuntimeVersion == LatestDaemonDBVersion &&
		stored.TargetOwnerVersion == maxEmbeddedOwnerVersion(bundles), nil
}
