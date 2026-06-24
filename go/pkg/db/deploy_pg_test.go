package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestDeployerDryRunPlanLive is the F5 live arm: `daemon deploy --dry-run` reports
// the plan over a real database and mutates NOTHING — no deploy_cursor row is
// written. (The destructive full apply / resume / two-role CREATE-denial are the
// verify-run's game-days GD-1..GD-12.)
func TestDeployerDryRunPlanLive(t *testing.T) {
	pool := pgtest.Pool(t) // skips without STRIATUM_PG_TEST_URL
	ctx := context.Background()

	baseOwner, err := db.OwnerBundleVersion(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("OwnerBundleVersion: %v", err)
	}
	baseRuntime, err := db.ReadSchemaVersion(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("ReadSchemaVersion: %v", err)
	}
	want, err := db.BuildPlan(baseOwner, baseRuntime)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	deployer := &db.Deployer{DaemonVersion: "pgtest", DryRun: true}
	result, err := deployer.Apply(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("dry-run Apply: %v", err)
	}
	if result.PlanHash != want.PlanHash {
		t.Fatalf("dry-run plan_hash=%s, want %s", result.PlanHash, want.PlanHash)
	}
	if result.StepsTotal != len(want.Steps) {
		t.Fatalf("dry-run steps=%d, want %d", result.StepsTotal, len(want.Steps))
	}
	cursor, err := db.LoadDeployCursor(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("LoadDeployCursor: %v", err)
	}
	if cursor.State != db.DeployStateNone {
		t.Fatalf("dry-run materialized a cursor (state=%s); it must mutate nothing", cursor.State)
	}
}

// TestDeployActivationCompleteInSyncServesVerifyLive is the live arm of F18's
// B1.1 obligation: a decoupled `complete` cursor whose schema_state.fingerprint ==
// ExpectedFingerprint() AND whose plan targets this frontier SERVES verify-only;
// flipping the recorded fingerprint out of sync makes the SAME cursor halt
// awaiting_deploy. The constructible in-sync row-15/16 cell (the M7 derivation),
// exercised against a real database — and asserted revokeEmbedded-INDEPENDENT.
func TestDeployActivationCompleteInSyncServesVerifyLive(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	plan, err := db.BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	// Seed an immutable deploy_plan row whose target frontier matches this binary
	// (the steps content is irrelevant to the A-gate; '[]' keeps the seed minimal).
	if err := pool.Runner.Exec(ctx,
		`INSERT INTO striatumd.deploy_plan(plan_hash, steps, revoke_step_index,
		   base_owner_version, base_runtime_version, target_owner_version, target_runtime_version)
		 VALUES ($1, '[]'::jsonb, $2, $3, $4, $5, $6)`,
		plan.PlanHash, plan.RevokeStepIndex, plan.BaseOwnerVersion, plan.BaseRuntimeVersion,
		plan.TargetOwnerVersion, plan.TargetRuntimeVersion); err != nil {
		t.Fatalf("seed deploy_plan: %v", err)
	}
	// Independently set the recorded fingerprint in sync (schema_state is orthogonal
	// to owner_bundle_meta — the M6/M7 orthogonality the cell proves).
	if err := db.RecordSchemaFingerprint(ctx, pool.Runner, "pgtest"); err != nil {
		t.Fatalf("RecordSchemaFingerprint: %v", err)
	}
	if err := pool.Runner.Exec(ctx,
		`INSERT INTO striatumd.deploy_cursor(id, plan_hash, state, step_index, step_id, updated_at)
		 VALUES ('singleton', $1, 'complete', 0, '', now())`, plan.PlanHash); err != nil {
		t.Fatalf("seed deploy_cursor complete: %v", err)
	}

	// In-sync: serve verify-only (rows 15/16) — identical for revoke=false/true (M7).
	for _, revoke := range []bool{false, true} {
		decision, _, err := db.CheckDeployActivation(ctx, pool.Runner, revoke, true)
		if err != nil {
			t.Fatalf("CheckDeployActivation (in-sync, revoke=%v): %v", revoke, err)
		}
		if decision != db.DeployServeVerify {
			t.Fatalf("in-sync decoupled complete (revoke=%v) decision=%s, want serve_verify", revoke, decision)
		}
	}

	// Out-of-sync: the SAME cursor halts awaiting_deploy.
	if err := pool.Runner.Exec(ctx,
		"UPDATE striatumd.schema_state SET fingerprint = $1 WHERE id = 'singleton'",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err != nil {
		t.Fatalf("corrupt fingerprint: %v", err)
	}
	decision, _, err := db.CheckDeployActivation(ctx, pool.Runner, true, true)
	if err != nil {
		t.Fatalf("CheckDeployActivation (out-of-sync): %v", err)
	}
	if decision != db.DeployHaltAwaitingDeploy {
		t.Fatalf("out-of-sync decoupled complete decision=%s, want awaiting_deploy", decision)
	}
}

// TestDeployActivationNoneCursorLive is the F18a / row-1 A-gate live arm: with NO
// deploy in flight (no deploy_cursor row), a no-revoke flag-OFF binary's A-gate
// returns serve_legacy (the legacy ApplyMigrations path), while the decoupled
// flag-ON case halts awaiting_deploy (the decoupled boot never auto-applies).
func TestDeployActivationNoneCursorLive(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	cursor, err := db.LoadDeployCursor(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("LoadDeployCursor: %v", err)
	}
	if cursor.State != db.DeployStateNone {
		t.Fatalf("fresh pool has cursor state %s, want none", cursor.State)
	}

	legacy, _, err := db.CheckDeployActivation(ctx, pool.Runner, false, false)
	if err != nil {
		t.Fatalf("CheckDeployActivation (none/off/no-revoke): %v", err)
	}
	if legacy != db.DeployServeLegacy {
		t.Fatalf("none/off/no-revoke decision=%s, want serve_legacy (the fresh/inert-landing serve cell)", legacy)
	}

	decoupled, _, err := db.CheckDeployActivation(ctx, pool.Runner, false, true)
	if err != nil {
		t.Fatalf("CheckDeployActivation (none/on): %v", err)
	}
	if decoupled != db.DeployHaltAwaitingDeploy {
		t.Fatalf("none/on decision=%s, want awaiting_deploy (decoupled never auto-applies)", decoupled)
	}
}
