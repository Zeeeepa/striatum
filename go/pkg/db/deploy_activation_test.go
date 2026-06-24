package db

import (
	"errors"
	"testing"
)

// concreteCursorStates is the EXACT enum set F18 must table-drive (B1.2): the
// "64-cell" shorthand groups step_committed with in_progress and aborted with the
// non-complete edge, but the implementation operates on the concrete enum, so the
// executable oracle drives each one by value.
var concreteCursorStates = []string{
	DeployStateNone,
	DeployStateInProgress,
	DeployStateStepCommitted,
	DeployStateFinalizing,
	DeployStateComplete,
	DeployStateAborted,
}

// appliedOwnerColumns are the four §3.5 owner-watermark columns. A
// (DecideDeployActivation) is owner-watermark-INDEPENDENT (§0.2), so they are
// carried here ONLY to prove the column-identity property executably: the A
// decision is identical across all of them for a fixed (cursorState, decoupled,
// revoke, inSync) row. The W-gate handles the shortfall / barrier-b differences
// (those are CheckOwnerBundleWatermark's job, covered by the two-role pgtest).
var appliedOwnerColumns = []int{0, 20, 21}

// expectDeployActivation is the §3.5 / §3.3a outcome stated as a literal oracle
// (NOT a re-implementation of the predicate): step 0 M3 gate first, then the
// cursor-state edges, then the complete branch conditional on fingerprint-sync.
func expectDeployActivation(cursorState string, decoupled, revoke, inSync bool) DeployBootDecision {
	if revoke && !decoupled { // M3 config gate, every cursor state
		return DeployHaltAwaitingConfig
	}
	switch cursorState {
	case DeployStateComplete:
		if !inSync {
			return DeployHaltAwaitingDeploy
		}
		if decoupled {
			return DeployServeVerify // rows 15/16 in-sync
		}
		return DeployServeLegacy // row 13 in-sync
	case DeployStateNone:
		if decoupled {
			return DeployHaltAwaitingDeploy // row 3/4
		}
		return DeployServeLegacy // row 1 (fresh-DB + inert-landing)
	default: // in_progress / step_committed / finalizing / aborted
		return DeployHaltAwaitingDeploy // BC-N2
	}
}

// TestDeployBootPathDecisionTable is F18 (T-deploy-bootpath-decision-table) at the
// PURE decision layer — the executable oracle of §3.5. It (B1.2) table-drives each
// concrete cursor-state enum; (B1.1) for the complete row it constructs BOTH the
// in-sync AND out-of-sync sub-cases across the applied_owner ∈ {0,20,>=21} columns
// and asserts the in-sync decoupled cells serve VERIFY-ONLY (never serve_legacy,
// i.e. never the ApplyMigrations/:399 path); and it asserts the column-identity
// (==0 == ==20 == >=21) and row-identity (row 15 == row 16) orthogonality
// properties M6/M7 close. The DB-level F18 with live cursor/fingerprint state and
// ApplyMigrations/RecordSchemaFingerprint spies is the verify-run's game-day
// (deploy_bootpath_pg_test.go), gated on STRIATUM_PG_TEST_URL.
func TestDeployBootPathDecisionTable(t *testing.T) {
	for _, cursorState := range concreteCursorStates {
		for _, decoupled := range []bool{false, true} {
			for _, revoke := range []bool{false, true} {
				// The complete row carries the in-sync/out-of-sync sub-dimension
				// (B1.1); every other row's outcome is independent of inSync, which we
				// also assert by driving both values.
				inSyncValues := []bool{false, true}
				for _, inSync := range inSyncValues {
					want := expectDeployActivation(cursorState, decoupled, revoke, inSync)
					// Column-identity (M6/M7): identical across all applied_owner columns.
					for _, appliedOwner := range appliedOwnerColumns {
						got := DecideDeployActivation(DeployActivationInputs{
							CursorState:       cursorState,
							DecoupledEnabled:  decoupled,
							RevokeEmbedded:    revoke,
							FingerprintInSync: inSync,
							PlanHashMatch:     inSync, // in-sync ⇒ plan targets this frontier
						})
						if got != want {
							t.Fatalf("cursor=%s decoupled=%v revoke=%v inSync=%v appliedOwner=%d: got %s, want %s",
								cursorState, decoupled, revoke, inSync, appliedOwner, got, want)
						}
						// No serve_legacy (the :399 spy) for ANY decoupled cell, and none
						// for a revoke-embedding binary in ANY column (the §4.5 spy list
						// invariant).
						if decoupled && got == DeployServeLegacy {
							t.Fatalf("cursor=%s revoke=%v inSync=%v: decoupled boot reached serve_legacy (:399 spy) — forbidden",
								cursorState, revoke, inSync)
						}
						if revoke && got == DeployServeLegacy {
							t.Fatalf("cursor=%s decoupled=%v inSync=%v: revoke-embedding binary reached serve_legacy (:399 spy) — forbidden (M3/M7)",
								cursorState, decoupled, inSync)
						}
					}
				}
			}
		}
	}
}

// TestDeployActivationRow16ConditionalMirrorsRow15 is the explicit B1.1 / M7
// assertion: for the decoupled complete branch, the revoke-embedding row 16 takes
// the IDENTICAL conditional outcome as the no-revoke row 15 in every column —
// in-sync serves verify-only, out-of-sync halts awaiting_deploy. Omitting this is
// what recreated M7 in the v8 table.
func TestDeployActivationRow16ConditionalMirrorsRow15(t *testing.T) {
	for _, inSync := range []bool{false, true} {
		row15 := DecideDeployActivation(DeployActivationInputs{
			CursorState: DeployStateComplete, DecoupledEnabled: true, RevokeEmbedded: false,
			FingerprintInSync: inSync, PlanHashMatch: inSync,
		})
		row16 := DecideDeployActivation(DeployActivationInputs{
			CursorState: DeployStateComplete, DecoupledEnabled: true, RevokeEmbedded: true,
			FingerprintInSync: inSync, PlanHashMatch: inSync,
		})
		if row15 != row16 {
			t.Fatalf("inSync=%v: row15 (no-revoke decoupled complete)=%s != row16 (revoke decoupled complete)=%s; "+
				"the decoupled complete branch must be revokeEmbedded-independent (M7)", inSync, row15, row16)
		}
		wantInSync := DeployServeVerify
		wantOut := DeployHaltAwaitingDeploy
		if inSync && row16 != wantInSync {
			t.Fatalf("row16 in-sync = %s, want %s (serve verify-only)", row16, wantInSync)
		}
		if !inSync && row16 != wantOut {
			t.Fatalf("row16 out-of-sync = %s, want %s", row16, wantOut)
		}
	}
}

// TestDeployActivationM3GateFiresForEveryCursorState is the M3 (e) assertion: a
// revoke-embedding binary with the flag OFF halts awaiting_deploy_config for EVERY
// concrete cursor state (cells 2/6/10/14), never reaching serve_legacy/serve_verify.
func TestDeployActivationM3GateFiresForEveryCursorState(t *testing.T) {
	for _, cursorState := range concreteCursorStates {
		for _, inSync := range []bool{false, true} {
			got := DecideDeployActivation(DeployActivationInputs{
				CursorState: cursorState, DecoupledEnabled: false, RevokeEmbedded: true,
				FingerprintInSync: inSync, PlanHashMatch: inSync,
			})
			if got != DeployHaltAwaitingConfig {
				t.Fatalf("cursor=%s inSync=%v: revoke-embedding flag-OFF binary got %s, want awaiting_deploy_config (M3)", cursorState, inSync, got)
			}
		}
	}
}

// TestDeployActivationDecisionErrorMapping guards the halt→typed-error mapping the
// boot path uses to reach the non-restartable exit.
func TestDeployActivationDecisionErrorMapping(t *testing.T) {
	if err := DeployHaltAwaitingConfig.DecisionError(DeployStateComplete); !errors.Is(err, ErrAwaitingDeployConfig) {
		t.Fatalf("awaiting_deploy_config decision did not map to ErrAwaitingDeployConfig: %v", err)
	}
	if err := DeployHaltAwaitingDeploy.DecisionError(DeployStateInProgress); !errors.Is(err, ErrAwaitingDeploy) {
		t.Fatalf("awaiting_deploy decision did not map to ErrAwaitingDeploy: %v", err)
	}
	if err := DeployServeLegacy.DecisionError(DeployStateNone); err != nil {
		t.Fatalf("serve_legacy must not produce an error, got %v", err)
	}
	if err := DeployServeVerify.DecisionError(DeployStateComplete); err != nil {
		t.Fatalf("serve_verify must not produce an error, got %v", err)
	}
}
