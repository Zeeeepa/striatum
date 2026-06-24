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

// TestDeployActivationCompleteInSyncServesVerifyLive is the TIGHTENED (D1') live arm
// of F18's B1.1 obligation. It constructs the decoupled `complete` in-sync cell
// across ALL THREE owner-watermark buckets B1.1 names — owner_bundle_meta ABSENT
// (==0), ==20, and >=21 — proving the A-gate's serve_verify is owner-watermark- AND
// revokeEmbedded-INDEPENDENT (M6/M7), and asserts the mutating serve-boot path is NOT
// entered via a REAL behavioral spy: schema_state (the only RecordSchemaFingerprint
// writer) and the schema_migrations row count (what ApplyMigrations grows) are
// snapshotted before/after CheckDeployActivation and must be byte-identical — not the
// decision-value tautology the prior round relied on. The out-of-sync sub-case halts
// awaiting_deploy, equally without mutation.
func TestDeployActivationCompleteInSyncServesVerifyLive(t *testing.T) {
	buckets := []struct {
		name    string
		version int // owner_bundle_meta MAX(version); 0 means leave the table ABSENT
	}{
		{"owner_bundle_meta_absent", 0},
		{"owner_bundle_meta_20", 20},
		{"owner_bundle_meta_21", 21},
	}
	for _, bucket := range buckets {
		bucket := bucket
		t.Run(bucket.name, func(t *testing.T) {
			pool := pgtest.Pool(t) // fresh DB per bucket; skips without STRIATUM_PG_TEST_URL
			ctx := context.Background()

			plan, err := db.BuildPlan(0, 0)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}
			// Seed an immutable deploy_plan row whose target frontier matches this binary
			// (steps content is irrelevant to the A-gate; '[]' keeps the seed minimal).
			if err := pool.Runner.Exec(ctx,
				`INSERT INTO striatumd.deploy_plan(plan_hash, steps, revoke_step_index,
				   base_owner_version, base_runtime_version, target_owner_version, target_runtime_version)
				 VALUES ($1, '[]'::jsonb, $2, $3, $4, $5, $6)`,
				plan.PlanHash, plan.RevokeStepIndex, plan.BaseOwnerVersion, plan.BaseRuntimeVersion,
				plan.TargetOwnerVersion, plan.TargetRuntimeVersion); err != nil {
				t.Fatalf("seed deploy_plan: %v", err)
			}
			// Independently set the owner-watermark bucket. CheckDeployActivation NEVER
			// reads owner_bundle_meta (§0.2), so seeding it to 0/20/21 must not move the
			// decision — that orthogonality is exactly what B1.1 demands the live arm prove.
			seedOwnerBundleBucket(ctx, t, pool.Runner, bucket.version)
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

			// In-sync: serve verify-only (rows 15/16) — identical for revoke=false/true (M7) —
			// and the mutating path (ApplyMigrations / RecordSchemaFingerprint) is NOT entered.
			for _, revoke := range []bool{false, true} {
				before := mutationSpyState(ctx, t, pool.Runner)
				decision, _, err := db.CheckDeployActivation(ctx, pool.Runner, revoke, true)
				if err != nil {
					t.Fatalf("CheckDeployActivation (in-sync, revoke=%v): %v", revoke, err)
				}
				if decision != db.DeployServeVerify {
					t.Fatalf("in-sync decoupled complete (revoke=%v, bucket=%s) decision=%s, want serve_verify",
						revoke, bucket.name, decision)
				}
				if after := mutationSpyState(ctx, t, pool.Runner); after != before {
					t.Fatalf("in-sync serve_verify (revoke=%v) MUTATED serve-boot state — ApplyMigrations/RecordSchemaFingerprint was entered:\n before=%s\n after =%s",
						revoke, before, after)
				}
			}

			// Out-of-sync: the SAME cursor halts awaiting_deploy, also without mutation.
			if err := pool.Runner.Exec(ctx,
				"UPDATE striatumd.schema_state SET fingerprint = $1 WHERE id = 'singleton'",
				"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err != nil {
				t.Fatalf("corrupt fingerprint: %v", err)
			}
			// Baseline the spy AFTER the deliberate fingerprint edit (that UPDATE is the
			// test's mutation, not the A-gate's).
			before := mutationSpyState(ctx, t, pool.Runner)
			decision, _, err := db.CheckDeployActivation(ctx, pool.Runner, true, true)
			if err != nil {
				t.Fatalf("CheckDeployActivation (out-of-sync): %v", err)
			}
			if decision != db.DeployHaltAwaitingDeploy {
				t.Fatalf("out-of-sync decoupled complete (bucket=%s) decision=%s, want awaiting_deploy", bucket.name, decision)
			}
			if after := mutationSpyState(ctx, t, pool.Runner); after != before {
				t.Fatalf("out-of-sync awaiting_deploy MUTATED serve-boot state:\n before=%s\n after =%s", before, after)
			}
		})
	}
}

// mutationSpyState captures the serve-boot mutation surface CheckDeployActivation
// must NEVER touch: the schema_state singleton (the sole RecordSchemaFingerprint
// writer) serialized whole, plus the schema_migrations row count (what ApplyMigrations
// grows). A byte-identical before/after pair is a behavioral proof the mutating path
// was not entered — strictly stronger than asserting the returned decision value.
func mutationSpyState(ctx context.Context, t *testing.T, runner db.Runner) string {
	t.Helper()
	state, err := runner.QueryScalar(ctx,
		"SELECT COALESCE((SELECT to_jsonb(s.*)::text FROM striatumd.schema_state s WHERE id = 'singleton'), '<absent>')")
	if err != nil {
		t.Fatalf("spy schema_state: %v", err)
	}
	migrations, err := runner.QueryScalar(ctx, "SELECT count(*)::text FROM striatumd.schema_migrations")
	if err != nil {
		t.Fatalf("spy schema_migrations count: %v", err)
	}
	return "schema_state=" + state + " schema_migrations=" + migrations
}

// seedOwnerBundleBucket sets the owner-bundle watermark bucket for the B1.1 column
// (version 0 leaves owner_bundle_meta ABSENT, the fresh-DB ==0 column). A non-zero
// version create-if-absent + inserts the row so OwnerBundleVersion would read it —
// the point being that CheckDeployActivation must STILL not read it (orthogonality).
func seedOwnerBundleBucket(ctx context.Context, t *testing.T, runner db.Runner, version int) {
	t.Helper()
	if version == 0 {
		return
	}
	if err := runner.Exec(ctx, `CREATE TABLE IF NOT EXISTS striatumd.owner_bundle_meta (
		version integer PRIMARY KEY, label text, sha256 text, daemon_version text,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatalf("seed owner_bundle_meta table: %v", err)
	}
	if err := runner.Exec(ctx,
		`INSERT INTO striatumd.owner_bundle_meta(version, label, sha256, daemon_version)
		 VALUES ($1, 'b1.1-orthogonality-seed', 'seed', 'pgtest') ON CONFLICT (version) DO NOTHING`,
		version); err != nil {
		t.Fatalf("seed owner_bundle_meta version %d: %v", version, err)
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
