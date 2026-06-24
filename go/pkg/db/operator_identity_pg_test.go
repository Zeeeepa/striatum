package db_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

// RFC 0167 P0 (D260) — the ten named two-role pgtests for owner bundle 0022
// (operator identity & run attribution). They run as the escape-proof,
// privilege-constrained SUT login role (pgtest.TwoRole) against a database
// bootstrapped to production's owner/runtime ownership split, so a SQLSTATE 42501
// on an identity column is the genuine composed-route read closure (C2"), not a
// fixture artifact. They skip when STRIATUM_PG_TEST_URL is unset (the no-network
// verifier sandbox); the verifier stage proves them against a live cluster.
//
// Gate-critical targets: A35/A36/A37 (composed Route 1/2/3 closed), A38
// (positive reads survive), A39 (drift reassert re-closes), A14 (write-once),
// A12 (lease flap), A6/A11 (live-unique collision), A1/A28 (projection stamp).

const operatorTestSecret = "p0_operator_identity_secret"
const operatorTestSalt = "p0_operator_identity_salt"

// seedOperatorWorld seeds, AS THE OWNER, a repository + a human principal P with a
// linked client, so the runtime-role probes have a real identity graph to read
// through (or be denied). It returns (repositoryID, principalID, clientID).
func seedOperatorWorld(t *testing.T, ctx context.Context, owner db.Runner) (string, string, string) {
	t.Helper()
	repositoryID := "repo_p0_oi"
	principalID := "prin_p0_oi"
	clientID := "client_p0_oi"
	stmts := []string{
		`INSERT INTO striatumd.repositories(repository_id, repo_identity, repo_root, state_db_path, display_name, last_schema_version, state, registered_at)
		   VALUES ('repo_p0_oi', 'ident_p0_oi', '/tmp/repo_p0_oi', '/tmp/repo_p0_oi/state.db', 'p0 oi repo', 17, 'active', now())
		 ON CONFLICT (repository_id) DO NOTHING`,
		`INSERT INTO striatumd.principals(principal_id, principal_kind, display_name, created_at)
		   VALUES ('prin_p0_oi', 'human', 'p0 operator', now())
		 ON CONFLICT (principal_id) DO NOTHING`,
		`INSERT INTO striatumd.principal_clients(principal_id, client_id, linked_at, unlinked_at)
		   VALUES ('prin_p0_oi', 'client_p0_oi', now(), NULL)
		 ON CONFLICT (principal_id, client_id) DO UPDATE SET unlinked_at = NULL`,
	}
	for _, s := range stmts {
		if err := owner.Exec(ctx, s); err != nil {
			t.Fatalf("seed operator world (%q): %v", s, err)
		}
	}
	registerAuthoritySecret(t, ctx, owner, operatorTestSecret, operatorTestSalt)
	return repositoryID, principalID, clientID
}

// seedLeasedHandle leases (AS THE OWNER) one handle for a principal+session.
func seedLeasedHandle(t *testing.T, ctx context.Context, owner db.Runner, handleID, repositoryID, principalID, handle, sessionID string) {
	t.Helper()
	if err := owner.Exec(ctx, `
		INSERT INTO striatumd.operator_handles(
		  handle_id, repository_id, principal_id, handle, leased_session_id,
		  leased_until, released_at, last_heartbeat_at)
		VALUES ($1, $2, $3, $4, $5, now() + interval '1 hour', NULL, now())`,
		handleID, repositoryID, principalID, handle, sessionID,
	); err != nil {
		t.Fatalf("seed leased handle %q: %v", handle, err)
	}
}

// seedStampedRun inserts (AS THE OWNER) a run carrying the origin stamp.
func seedStampedRun(t *testing.T, ctx context.Context, owner db.Runner, repositoryID, runID, principalID, handleID string) {
	t.Helper()
	// A run needs a workflow snapshot (FK). Seed a minimal one.
	snapshotID := "wfs_" + runID
	if err := owner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots(
		  repository_id, workflow_snapshot_id, workflow_id, workflow_version,
		  source_path, content_sha256, workflow_json, loaded_at)
		VALUES ($1, $2, 'wf', '1', 'p', 'h', '{}'::jsonb, now())
		ON CONFLICT DO NOTHING`,
		repositoryID, snapshotID,
	); err != nil {
		t.Fatalf("seed workflow snapshot: %v", err)
	}
	var handleArg any
	if handleID != "" {
		handleArg = handleID
	}
	var principalArg any
	if principalID != "" {
		principalArg = principalID
	}
	if err := owner.Exec(ctx, `
		INSERT INTO striatumd.runs(
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  created_by_principal_id, created_by_handle_id)
		VALUES ($1, $2, $3, '/tmp/repo_p0_oi', 'ready', now(), $4, $5)`,
		repositoryID, runID, snapshotID, principalArg, handleArg,
	); err != nil {
		t.Fatalf("seed stamped run %q: %v", runID, err)
	}
}

func isSQLState(err error, code string) bool {
	var pgErr *pgconn.PgError
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), code) || (asPgErr(err, &pgErr) && pgErr.Code == code)
}

func asPgErr(err error, target **pgconn.PgError) bool {
	for e := err; e != nil; {
		if pe, ok := e.(*pgconn.PgError); ok {
			*target = pe
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// 1. owner_bundle_applies_clean — the bundle applied (fixture Phase A), the new
// objects exist, the runtime role can do exactly what P0 needs and nothing more.
func TestOperatorOwnerBundleAppliesCleanTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	// Objects exist (owner view).
	for label, q := range map[string]string{
		"operator_handles table":  "SELECT (to_regclass('striatumd.operator_handles') IS NOT NULL)::text",
		"operator_sessions table": "SELECT (to_regclass('striatumd.operator_sessions') IS NOT NULL)::text",
		"runs.created_by_principal_id": `SELECT EXISTS(SELECT 1 FROM information_schema.columns
		   WHERE table_schema='striatumd' AND table_name='runs' AND column_name='created_by_principal_id')::text`,
		"runs.created_by_handle_id": `SELECT EXISTS(SELECT 1 FROM information_schema.columns
		   WHERE table_schema='striatumd' AND table_name='runs' AND column_name='created_by_handle_id')::text`,
		"write-once trigger": `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='runs_origin_write_once')::text`,
	} {
		if got := scalar(t, ctx, fx.OwnerPool.Runner, q); got != "true" {
			t.Fatalf("%s: got %q, want true", label, got)
		}
	}

	// Runtime role CAN do the P0 operations.
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_clean", repositoryID, principalID, "maya", "osess_clean")
	if err := fx.SUTPool.Runner.Exec(ctx,
		`UPDATE striatumd.operator_handles SET last_heartbeat_at = now() WHERE handle_id = 'oh_clean'`); err != nil {
		t.Fatalf("runtime UPDATE operator_handles must succeed: %v", err)
	}
	if got := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT handle FROM striatumd.operator_handles WHERE handle_id = 'oh_clean'`); got != "maya" {
		t.Fatalf("runtime SELECT of granted operator_handles columns must succeed: got %q", got)
	}

	// Runtime role CANNOT: ALTER runs (owner-held), nor SELECT the identity columns.
	assertSQLState42501(t, fx.SUTPool.Runner.Exec(ctx,
		"ALTER TABLE striatumd.runs ADD COLUMN p0_probe integer"),
		"must be owner of table runs", "runtime ALTER runs")
	for _, q := range []string{
		"SELECT created_by_principal_id FROM striatumd.runs LIMIT 1",
		"SELECT principal_id FROM striatumd.operator_handles LIMIT 1",
		"SELECT principal_id FROM striatumd.operator_sessions LIMIT 1",
		"SELECT client_id FROM striatumd.operator_sessions LIMIT 1",
	} {
		_, err := fx.SUTPool.Runner.QueryScalar(ctx, q)
		if !isSQLState(err, "42501") {
			t.Fatalf("expected 42501 on %q (identity column denied), got %v", q, err)
		}
	}
}

// 2. composed_identity_map_unreadable — the C2" decisive gate: both composed
// routes fail 42501, and the spawn-grant third route holds a client id, not a
// principal (A35/A36/A37/A44).
func TestOperatorComposedIdentityMapUnreadableTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	// Route 1: cc ⋈ oh on oh.principal_id.
	_, err := fx.SUTPool.Runner.QueryScalar(ctx, `
		SELECT DISTINCT cc.client_id, oh.principal_id
		  FROM striatumd.client_capabilities cc
		  JOIN striatumd.operator_handles oh ON oh.leased_session_id = cc.session_id
		 LIMIT 1`)
	if !isSQLState(err, "42501") {
		t.Fatalf("A35 Route 1 must be 42501 (oh.principal_id ungranted), got %v", err)
	}

	// Route 2: cc ⋈ oh ⋈ runs on r.created_by_principal_id.
	_, err = fx.SUTPool.Runner.QueryScalar(ctx, `
		SELECT DISTINCT cc.client_id, r.created_by_principal_id
		  FROM striatumd.client_capabilities cc
		  JOIN striatumd.operator_handles oh ON oh.leased_session_id = cc.session_id
		  JOIN striatumd.runs r ON r.created_by_handle_id = oh.handle_id
		 LIMIT 1`)
	if !isSQLState(err, "42501") {
		t.Fatalf("A36 Route 2 must be 42501 (r.created_by_principal_id ungranted), got %v", err)
	}

	// Route 3 (A44): spawn_authorization_grants.owner_principal_id is a bare text
	// column holding a CLIENT id — NO FK to principals. The ACL exception is sound
	// only while that holds; assert there is no FK from owner_principal_id to
	// principals.
	hasFK := scalar(t, ctx, fx.OwnerPool.Runner, `
		SELECT EXISTS(
		  SELECT 1
		    FROM information_schema.table_constraints tc
		    JOIN information_schema.key_column_usage kcu
		      ON kcu.constraint_name = tc.constraint_name
		    JOIN information_schema.constraint_column_usage ccu
		      ON ccu.constraint_name = tc.constraint_name
		   WHERE tc.constraint_type = 'FOREIGN KEY'
		     AND tc.table_schema = 'striatumd'
		     AND tc.table_name = 'spawn_authorization_grants'
		     AND kcu.column_name = 'owner_principal_id'
		     AND ccu.table_name = 'principals')::text`)
	if hasFK != "false" {
		t.Fatalf("A44 refuted: spawn_authorization_grants.owner_principal_id has an FK to principals; it must stay a bare client-id column or the table needs the full identity-bearing treatment")
	}

	// Route 3, the COMPOSED form (§F F-1 / A44): seed the full
	// cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants chain over an auto_spawn-captured
	// grant whose owner_principal_id holds the run owner's CLIENT id, then prove that
	// AS THE RUNTIME ROLE the composed route reconstructs client_id -> client_id (the
	// grant's owner_principal_id IS a client id), NOT client_id -> principal_id.
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_sg", repositoryID, principalID, "maya", "osess_sg")
	seedStampedRun(t, ctx, fx.OwnerPool.Runner, repositoryID, "run_sg", principalID, "oh_sg")
	if err := fx.OwnerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at)
		VALUES ('client_sg', 'session', 'sg', 'tok_sg', 'h', 's', now())
		ON CONFLICT (client_id) DO NOTHING`); err != nil {
		t.Fatalf("seed sg client: %v", err)
	}
	if err := fx.OwnerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.client_capabilities(capability_id, client_id, repository_id, capability, granted_at, session_id)
		VALUES ('cap_sg', 'client_sg', $1, 'admin', now(), 'osess_sg')`, repositoryID); err != nil {
		t.Fatalf("seed sg capability: %v", err)
	}
	// The captured RFC 0122 grant: owner_principal_id holds the run owner's CLIENT id.
	if err := fx.OwnerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.spawn_authorization_grants(
		  repository_id, grant_id, run_id, owner_principal_id, run_as_spec, capability_envelope, created_at)
		VALUES ($1, 'grant_sg', 'run_sg', 'client_sg', '{}'::jsonb, '{}'::jsonb, now())`, repositoryID); err != nil {
		t.Fatalf("seed spawn_authorization_grant: %v", err)
	}
	got := scalar(t, ctx, fx.SUTPool.Runner, `
		SELECT sag.owner_principal_id
		  FROM striatumd.client_capabilities cc
		  JOIN striatumd.operator_handles oh ON oh.leased_session_id = cc.session_id
		  JOIN striatumd.runs r ON r.created_by_handle_id = oh.handle_id
		  JOIN striatumd.spawn_authorization_grants sag ON sag.run_id = r.run_id
		 WHERE oh.released_at IS NULL
		 LIMIT 1`)
	if got != "client_sg" {
		t.Fatalf("A44 Route 3: the composed route must reconstruct the run owner's CLIENT id (client_sg), got %q", got)
	}

	// The information_schema.role_column_grants exception (§F F-1): owner_principal_id
	// is the ONE *principal_id*-named column granted to striatumd_rw — a client id, not
	// a real principal — while every real-principal identity column stays ungranted.
	exception := scalar(t, ctx, fx.OwnerPool.Runner, `
		SELECT (
		  has_column_privilege('striatumd_rw','striatumd.spawn_authorization_grants','owner_principal_id','SELECT')
		  AND NOT has_column_privilege('striatumd_rw','striatumd.runs','created_by_principal_id','SELECT')
		  AND NOT has_column_privilege('striatumd_rw','striatumd.operator_handles','principal_id','SELECT')
		  AND NOT has_column_privilege('striatumd_rw','striatumd.operator_sessions','principal_id','SELECT')
		)::text`)
	if exception != "true" {
		t.Fatalf("A44: spawn_authorization_grants.owner_principal_id must be the granted client-id exception while the real-principal identity columns stay ungranted")
	}
}

// 3. whose_status_mine_via_projection — the C2" positive: the identity reads
// resolve correctly THROUGH the DEFINER projections (with the daemon secret),
// while the direct principal_clients read stays denied (A1/A28/A38).
func TestOperatorWhoseStatusMineViaProjectionTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, clientID := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_w", repositoryID, principalID, "maya", "osess_w")
	seedStampedRun(t, ctx, fx.OwnerPool.Runner, repositoryID, "run_w", principalID, "oh_w")

	// Direct principal map stays denied.
	_, derr := fx.SUTPool.Runner.QueryScalar(ctx,
		"SELECT principal_id FROM striatumd.principal_clients LIMIT 1")
	if !isSQLState(derr, "42501") {
		t.Fatalf("direct principal_clients.principal_id must be 42501, got %v", derr)
	}

	// whose via run_origin_identity (DEFINER, secret-gated) returns P + handle.
	gotPrincipal := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT created_by_principal_id FROM striatumd.run_origin_identity($1, $2, $3)`,
		operatorTestSecret, repositoryID, "run_w")
	if gotPrincipal != principalID {
		t.Fatalf("whose projection returned principal %q, want %q", gotPrincipal, principalID)
	}
	gotHandle := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT origin_handle FROM striatumd.run_origin_identity($1, $2, $3)`,
		operatorTestSecret, repositoryID, "run_w")
	if gotHandle != "maya" {
		t.Fatalf("whose projection returned handle %q, want maya", gotHandle)
	}

	// status --mine via runs_for_origin_client returns the run for the linked client.
	gotRun := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT run_id FROM striatumd.runs_for_origin_client($1, $2, $3)`,
		operatorTestSecret, repositoryID, clientID)
	if gotRun != "run_w" {
		t.Fatalf("status --mine projection returned run %q, want run_w", gotRun)
	}

	// The converted star-readers' explicit column list still reads (A38): a
	// non-identity SELECT over runs succeeds under the column grant.
	if got := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT state FROM striatumd.runs WHERE run_id = 'run_w'`); got != "ready" {
		t.Fatalf("non-identity runs SELECT must succeed under the column grant, got %q", got)
	}
}

// 4. operator_session_pre_run_stamp — two operator sessions for one human, each
// minted via the REAL operator.bootstrap RPC, each authorizing run.prepare
// through the REAL PostgresAuthorizer, each creating a run via the REAL
// run.prepare RPC as the runtime role => two NON-NULL DISTINCT
// created_by_handle_id and distinct whose (A29/A7/A27), plus the lane-no-admin
// (A30/A43) and closed-session (A31) denials through the same authorizer.
func TestOperatorSessionPreRunStampTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, _, callerClientID := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	// Point the repository at an on-disk workflow so the REAL run.prepare RPC can
	// load it (branch mode "confirm" => no git ref creation needed).
	repoRoot := t.TempDir()
	operatorWriteWorkflow(t, filepath.Join(repoRoot, "workflow.json"))
	if err := fx.OwnerPool.Runner.Exec(ctx,
		`UPDATE striatumd.repositories SET repo_root = $1 WHERE repository_id = $2`, repoRoot, repositoryID); err != nil {
		t.Fatalf("point repo_root at the workflow dir: %v", err)
	}

	// The run-origin stamp rides the daemon-secret-gated resolve_principal_for_client
	// DEFINER projection; install the process secret matching the registered one so
	// the prelude satisfies assert_daemon_authority (restored on cleanup).
	db.SetAuthorityRuntime(operatorTestSecret, db.AuditHashFormatV2, "", false)
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })

	// run.prepare emits the run.created event. In production the operator declares
	// the full write boundary (RFC 0110 §7 P2), so the append routes through the
	// owner-owned SECURITY DEFINER append_event_row; below PhaseFull it falls to a
	// direct INSERT the runtime role cannot do (events INSERT is revoked from
	// striatumd_rw, owner/0004_phase2_events.sql:169). Install the full boundary so
	// the REAL run.prepare RPC writes events through the authorized SD path exactly
	// as the production daemon does (restored on cleanup).
	db.SetActiveWriteBoundary(db.PhaseFull)
	t.Cleanup(func() { db.SetActiveWriteBoundary(db.PhaseNone) })

	authz := &rpc.PostgresAuthorizer{Runner: fx.SUTPool.Runner, AuthoritySecret: operatorTestSecret}
	runPrepareCap := rpc.MethodRegistry["run.prepare"].RequiredCapability

	type stamped struct{ runID, handleID, whose string }
	results := make([]stamped, 0, 2)
	for _, label := range []string{"terminal_1", "terminal_2"} {
		// Same human (callerClientID is linked to principal P): mint the operator
		// session + lease a handle through the REAL operator.bootstrap RPC.
		boot, err := mutations.HandleOperatorBootstrap(
			operatorAuthCtx(ctx, callerClientID, "", repositoryID),
			fx.SUTPool.Runner,
			operatorEnvelope("operator.bootstrap", repositoryID, nil))
		if err != nil {
			t.Fatalf("%s operator.bootstrap: %v", label, err)
		}
		operatorClientID, _ := boot["client_id"].(string)
		operatorSessionID, _ := boot["operator_session_id"].(string)
		token, _ := boot["token"].(string)

		// A29: the minted operator token AUTHORIZES run.prepare through the real authorizer.
		if d := authz.Authorize(runPrepareCap, repositoryID, token); d.Decision != "allowed" {
			t.Fatalf("%s A29: operator token must authorize run.prepare, got %s/%s", label, d.Decision, d.DenialReason)
		}

		// The REAL run.prepare RPC, run AS THE RUNTIME ROLE under the operator's auth:
		// the stamp resolves created_by_principal_id via the projection and snapshots
		// created_by_handle_id from app.session_id = the operator session.
		res, err := mutations.HandleRunPrepare(
			operatorAuthCtx(ctx, operatorClientID, operatorSessionID, repositoryID),
			fx.SUTPool.Runner,
			operatorEnvelope("run.prepare", repositoryID, map[string]any{"workflow": "workflow.json"}))
		if err != nil {
			t.Fatalf("%s run.prepare: %v", label, err)
		}
		runID, _ := res["run_id"].(string)
		handleID := scalar(t, ctx, fx.SUTPool.Runner, `SELECT created_by_handle_id FROM striatumd.runs WHERE run_id = $1`, runID)
		whose := scalar(t, ctx, fx.SUTPool.Runner, `SELECT origin_handle FROM striatumd.run_origin_identity($1,$2,$3)`, operatorTestSecret, repositoryID, runID)
		results = append(results, stamped{runID, handleID, whose})
	}

	if results[0].handleID == "" || results[1].handleID == "" {
		t.Fatalf("A27: real run.prepare must stamp NON-NULL created_by_handle_id, got %q / %q", results[0].handleID, results[1].handleID)
	}
	if results[0].handleID == results[1].handleID {
		t.Fatalf("A7: two operator sessions of one human must stamp DISTINCT handles, both got %q", results[0].handleID)
	}
	if results[0].whose == "" || results[0].whose == results[1].whose {
		t.Fatalf("A7: whose must differ across the two terminals, got %q / %q", results[0].whose, results[1].whose)
	}

	// A30/A43: a lane token (no admin) is DENIED run.prepare through the same authorizer.
	laneToken := seedOperatorLaneToken(t, ctx, fx.OwnerPool.Runner, "prestamp", repositoryID, "sess_lane_prestamp")
	if d := authz.Authorize(runPrepareCap, repositoryID, laneToken); d.Decision == "allowed" {
		t.Fatalf("A30/A43: a lane token must NOT authorize run.prepare, got allowed")
	}

	// A31: closing an operator session revokes its token; it can no longer authorize a stamp.
	boot, err := mutations.HandleOperatorBootstrap(
		operatorAuthCtx(ctx, callerClientID, "", repositoryID),
		fx.SUTPool.Runner,
		operatorEnvelope("operator.bootstrap", repositoryID, nil))
	if err != nil {
		t.Fatalf("bootstrap for close: %v", err)
	}
	closedSession, _ := boot["operator_session_id"].(string)
	closedToken, _ := boot["token"].(string)
	if _, err := mutations.HandleOperatorClose(ctx, fx.SUTPool.Runner,
		operatorEnvelope("operator.close", repositoryID, map[string]any{"operator_session_id": closedSession})); err != nil {
		t.Fatalf("operator.close: %v", err)
	}
	if d := authz.Authorize(runPrepareCap, repositoryID, closedToken); d.Decision == "allowed" {
		t.Fatalf("A31: a closed operator session's token must NOT authorize run.prepare, got allowed")
	}
}

// 5. operator_token_admin_surface — the C1" justified-acceptance gate, exercised
// through the REAL authorizer + handlers: a token minted by the REAL
// operator.bootstrap RPC authorizes the accepted operator repo-admin surface
// (A40), is refused typed at the verifier.attest trust-root fence (A41), is
// unreachable for daemon-global admin (A42), a lane token is denied (A43), a
// closed session is denied (A31), and the routine credential carries exactly
// {admin, read} — the honest credential-segregation face of §F F-2 (A45).
func TestOperatorTokenAdminSurfaceTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, _, callerClientID := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	db.SetAuthorityRuntime(operatorTestSecret, db.AuditHashFormatV2, "", false)
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })

	// A45 (F-2): mint + present via the REAL operator.bootstrap RPC. The returned
	// token IS the routine repo-admin credential — the static bootstrap-admin token
	// is never minted or presented here.
	boot, err := mutations.HandleOperatorBootstrap(
		operatorAuthCtx(ctx, callerClientID, "", repositoryID),
		fx.SUTPool.Runner,
		operatorEnvelope("operator.bootstrap", repositoryID, nil))
	if err != nil {
		t.Fatalf("operator.bootstrap: %v", err)
	}
	operatorToken, _ := boot["token"].(string)
	operatorSessionID, _ := boot["operator_session_id"].(string)
	operatorClientID, _ := boot["client_id"].(string)

	authz := &rpc.PostgresAuthorizer{Runner: fx.SUTPool.Runner, AuthoritySecret: operatorTestSecret}

	// A40: the accepted operator repo-admin surface is AUTHORIZED through the real authorizer.
	for _, method := range []string{"run.prepare", "checkpoint.resolve", "review.override", "branch.confirm"} {
		if d := authz.Authorize(rpc.MethodRegistry[method].RequiredCapability, repositoryID, operatorToken); d.Decision != "allowed" {
			t.Fatalf("A40: operator token must authorize %s, got %s/%s", method, d.Decision, d.DenialReason)
		}
	}

	// A41: the verifier.attest trust-root fence refuses the session-bound operator
	// token typed capability_denied, through the real handler.
	_, aerr := mutations.HandleVerifierAttest(
		operatorAuthCtx(ctx, operatorClientID, operatorSessionID, repositoryID),
		fx.SUTPool.Runner,
		operatorEnvelope("verifier.attest", repositoryID, map[string]any{
			"check_id":      "chk_p0_surface",
			"binary_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
		}))
	if !isRPCCode(aerr, "capability_denied") {
		t.Fatalf("A41: verifier.attest must refuse the session-bound operator token capability_denied, got %v", aerr)
	}

	// A42: a daemon-global admin route is unreachable by the repo-scoped operator token.
	if d := authz.Authorize(rpc.MethodRegistry["daemon.token.create"].RequiredCapability, "", operatorToken); d.Decision == "allowed" {
		t.Fatalf("A42: a daemon-global admin route must be unreachable by the repo-scoped operator token, got allowed")
	}

	// A43: a lane token (no admin) is denied run.prepare through the same authorizer.
	laneToken := seedOperatorLaneToken(t, ctx, fx.OwnerPool.Runner, "surface", repositoryID, "sess_lane_surface")
	runPrepareCap := rpc.MethodRegistry["run.prepare"].RequiredCapability
	if d := authz.Authorize(runPrepareCap, repositoryID, laneToken); d.Decision == "allowed" {
		t.Fatalf("A43: a lane token must NOT authorize run.prepare, got allowed")
	}

	// A31: closing the operator session revokes the token; it can no longer authorize.
	if _, err := mutations.HandleOperatorClose(ctx, fx.SUTPool.Runner,
		operatorEnvelope("operator.close", repositoryID, map[string]any{"operator_session_id": operatorSessionID})); err != nil {
		t.Fatalf("operator.close: %v", err)
	}
	if d := authz.Authorize(runPrepareCap, repositoryID, operatorToken); d.Decision == "allowed" {
		t.Fatalf("A31: a closed operator session's token must NOT authorize run.prepare, got allowed")
	}

	// A45 segregation: the routine credential carries exactly {admin, read} — NOT the
	// broad static bootstrap-admin set ({admin,read,write,claim,review,apply,recovery,
	// surgical_recovery}). The narrowing is the honest blast-radius accounting.
	caps := scalar(t, ctx, fx.OwnerPool.Runner,
		`SELECT COALESCE(string_agg(capability, ',' ORDER BY capability), '') FROM striatumd.client_capabilities
		  WHERE client_id = $1`, operatorClientID)
	if caps != "admin,read" {
		t.Fatalf("A45: the operator token must carry exactly {admin, read}, got %q (not the broad static bootstrap set)", caps)
	}
}

// 6. forged_update_created_by_rejected — the write-once trigger raises on any
// UPDATE that changes a stamped origin column (A14).
func TestOperatorForgedUpdateCreatedByRejectedTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedStampedRun(t, ctx, fx.OwnerPool.Runner, repositoryID, "run_wo", principalID, "")

	// Insert a second principal to forge toward.
	if err := fx.OwnerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.principals(principal_id, principal_kind, display_name, created_at)
		VALUES ('prin_other', 'human', 'other', now()) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed other principal: %v", err)
	}
	err := fx.OwnerPool.Runner.Exec(ctx,
		`UPDATE striatumd.runs SET created_by_principal_id = 'prin_other' WHERE run_id = 'run_wo'`)
	if err == nil || !strings.Contains(err.Error(), "write-once") {
		t.Fatalf("A14: UPDATE of a stamped origin column must raise the write-once trigger, got %v", err)
	}
}

// 7. drift_reassert_recloses_routes — a stray GRANT SELECT ON runs is re-closed by
// ReassertReadRevokes from the 0022 capability stamp (A39).
func TestOperatorDriftReassertReclosesRoutesTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	// Simulate a drift/repair grant that re-opens Route 2.
	if err := fx.OwnerPool.Runner.Exec(ctx,
		"GRANT SELECT ON striatumd.runs TO striatumd_rw"); err != nil {
		t.Fatalf("simulate drift grant: %v", err)
	}
	// Drift is open: the runtime role can now read the identity column.
	if _, err := fx.SUTPool.Runner.QueryScalar(ctx,
		"SELECT created_by_principal_id FROM striatumd.runs LIMIT 1"); isSQLState(err, "42501") {
		t.Fatalf("precondition: the drift GRANT should have re-opened the runs SELECT, but it is still 42501")
	}
	// Reassert from the stamp.
	if err := db.ReassertReadRevokes(ctx, fx.OwnerPool.Runner); err != nil {
		t.Fatalf("ReassertReadRevokes: %v", err)
	}
	// Route 2 is re-closed.
	_, err := fx.SUTPool.Runner.QueryScalar(ctx,
		"SELECT created_by_principal_id FROM striatumd.runs LIMIT 1")
	if !isSQLState(err, "42501") {
		t.Fatalf("A39: ReassertReadRevokes must re-close the runs identity column, got %v", err)
	}
}

// 8. two_live_maya — two operator sessions of one principal cannot both hold
// "maya": the live-unique partial index forces the second to a different word
// (A6/A11). The ON CONFLICT DO NOTHING lease idiom yields no row on collision.
func TestOperatorTwoLiveMayaTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_m1", repositoryID, principalID, "maya", "osess_m1")

	// A second live "maya" lease (different session) must be refused by the
	// live-unique index: the runtime-role ON CONFLICT lease returns no row.
	got, err := fx.SUTPool.Runner.QueryScalar(ctx, `
		INSERT INTO striatumd.operator_handles(handle_id, repository_id, principal_id, handle,
		  leased_session_id, leased_until, released_at, last_heartbeat_at)
		VALUES ('oh_m2', $1, $2, 'maya', 'osess_m2', now() + interval '1 hour', NULL, now())
		ON CONFLICT (repository_id, lower(handle)) WHERE released_at IS NULL DO NOTHING
		RETURNING handle_id`, repositoryID, principalID)
	if err == nil && got != "" {
		t.Fatalf("A6: a second live 'maya' must NOT be leasable, but it succeeded (handle_id=%q)", got)
	}
	// The escalation word (e.g. "theo") IS leasable.
	if err := fx.SUTPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.operator_handles(handle_id, repository_id, principal_id, handle,
		  leased_session_id, leased_until, released_at, last_heartbeat_at)
		VALUES ('oh_m3', $1, $2, 'theo', 'osess_m2', now() + interval '1 hour', NULL, now())`,
		repositoryID, principalID); err != nil {
		t.Fatalf("A11: the escalation word must be leasable, got %v", err)
	}
}

// 9. token_revoked_bare_id — after the creating client is unlinked, status --mine
// (runs_for_origin_client) yields nothing (bare-id fallback), while whose
// (run_origin_identity) still renders the FROZEN historical handle from the
// snapshot (the join is on handle_id, not released_at).
func TestOperatorTokenRevokedBareIDTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, clientID := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_r", repositoryID, principalID, "maya", "osess_r")
	seedStampedRun(t, ctx, fx.OwnerPool.Runner, repositoryID, "run_r", principalID, "oh_r")

	// Release the handle + unlink the client (the revoke/close shape).
	if err := fx.OwnerPool.Runner.Exec(ctx,
		`UPDATE striatumd.operator_handles SET released_at = now() WHERE handle_id = 'oh_r'`); err != nil {
		t.Fatalf("release handle: %v", err)
	}
	if err := fx.OwnerPool.Runner.Exec(ctx,
		`UPDATE striatumd.principal_clients SET unlinked_at = now() WHERE client_id = $1`, clientID); err != nil {
		t.Fatalf("unlink client: %v", err)
	}

	// status --mine now resolves nothing for the unlinked client (bare-id fallback).
	mine, err := fx.SUTPool.Runner.QueryScalar(ctx,
		`SELECT run_id FROM striatumd.runs_for_origin_client($1, $2, $3)`,
		operatorTestSecret, repositoryID, clientID)
	if err != nil {
		t.Fatalf("runs_for_origin_client after unlink: %v", err)
	}
	if mine != "" {
		t.Fatalf("token_revoked_bare_id: status --mine must fall back (no rows) for the unlinked client, got %q", mine)
	}
	// whose <past-run> still renders the FROZEN historical handle.
	frozen := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT origin_handle FROM striatumd.run_origin_identity($1, $2, 'run_r')`,
		operatorTestSecret, repositoryID)
	if frozen != "maya" {
		t.Fatalf("token_revoked_bare_id: whose must still render the frozen handle 'maya', got %q", frozen)
	}
}

// 10. lease_flap_steal — a guarded renewal of S1's lease never frees the word, so
// a racing S2 attempt is always refused and S1's row is never released (A12).
func TestOperatorLeaseFlapStealTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_f1", repositoryID, principalID, "maya", "osess_f1")

	// S1's guarded renewal (the §3 idiom): never deletes, never sets released_at.
	if err := fx.SUTPool.Runner.Exec(ctx, `
		UPDATE striatumd.operator_handles
		   SET leased_until = now() + interval '1 hour', last_heartbeat_at = now()
		 WHERE handle_id = 'oh_f1' AND leased_session_id = 'osess_f1' AND released_at IS NULL`); err != nil {
		t.Fatalf("S1 guarded renewal: %v", err)
	}
	// Interleaved S2 steal attempt during the flap: refused (no row).
	got, _ := fx.SUTPool.Runner.QueryScalar(ctx, `
		INSERT INTO striatumd.operator_handles(handle_id, repository_id, principal_id, handle,
		  leased_session_id, leased_until, released_at, last_heartbeat_at)
		VALUES ('oh_f2', $1, $2, 'maya', 'osess_f2', now() + interval '1 hour', NULL, now())
		ON CONFLICT (repository_id, lower(handle)) WHERE released_at IS NULL DO NOTHING
		RETURNING handle_id`, repositoryID, principalID)
	if got != "" {
		t.Fatalf("A12: S2 must never steal 'maya' during S1's flap, but the lease succeeded (%q)", got)
	}
	// S1's row was never released.
	released := scalar(t, ctx, fx.SUTPool.Runner,
		`SELECT (released_at IS NULL)::text FROM striatumd.operator_handles WHERE handle_id = 'oh_f1'`)
	if released != "true" {
		t.Fatalf("A12: S1's lease row must never transit a released state during a flap")
	}
}

// --- real-path helpers (operator.bootstrap / run.prepare / verifier.attest /
// PostgresAuthorizer) for the C1'/C1" gate tests ---

// operatorAuthCtx threads a resolved AuthContext onto ctx exactly as the RPC
// dispatch does after Authorize succeeds, so the RFC 0110 authority prelude
// installs striatum.principal_id = clientID and app.session_id = sessionID for
// the handler's mutation transaction.
func operatorAuthCtx(ctx context.Context, clientID, sessionID, repositoryID string) context.Context {
	return rpc.WithAuthContext(ctx, rpc.AuthContext{
		ClientID:     clientID,
		SessionID:    sessionID,
		RepositoryID: repositoryID,
		Capability:   rpc.CapabilityAdmin,
		Decision:     "allowed",
	})
}

// operatorEnvelope builds a minimal valid RPC envelope for a handler call.
func operatorEnvelope(method, repositoryID string, extra map[string]any) rpc.Envelope {
	params := map[string]any{"repository_id": repositoryID}
	for key, value := range extra {
		params[key] = value
	}
	return rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_" + strings.NewReplacer(".", "_").Replace(method),
		Method:        method,
		Params:        params,
	}
}

// operatorWriteWorkflow writes the canonical minimal-valid run.prepare workflow
// (the project's run.prepare test fixture shape) at path: one lane, one role,
// one handoff job, branch mode "confirm" (so the real run.prepare needs no git
// ref creation).
func operatorWriteWorkflow(t *testing.T, path string) {
	t.Helper()
	workflow := map[string]any{
		"schema_version":   "striatum.workflow.v1",
		"workflow_id":      "rfc0167-operator-stamp",
		"workflow_version": "1",
		"name":             "RFC 0167 Operator Stamp",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "test/operator-stamp"},
		"coordinator":      map[string]any{"role_id": "worker", "lane_id": "lane_a"},
		"lanes":            map[string]any{"lane_a": map[string]any{"command": []any{"true"}}},
		"roles":            map[string]any{"worker": map[string]any{"description": "worker"}},
		"context_docs":     []any{},
		"parallelism":      map[string]any{"max_active_jobs": float64(1)},
		"jobs": []any{
			map[string]any{
				"id":                 "job_a",
				"type":               "generic",
				"role_id":            "worker",
				"lane_id":            "lane_a",
				"write_scope":        map[string]any{"allowed_paths": []any{"docs/"}},
				"expected_artifacts": []any{},
			},
		},
		"edges":  []any{},
		"cycles": []any{},
	}
	payload, err := json.Marshal(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedOperatorLaneToken seeds (AS THE OWNER) a lane client with the lane
// capability slice {claim, write, read, review} — NO admin — and returns its
// bearer token (tokenID.secret), so the real authorizer can prove a lane token
// is denied run.prepare (A30/A43).
func seedOperatorLaneToken(t *testing.T, ctx context.Context, owner db.Runner, suffix, repositoryID, sessionID string) string {
	t.Helper()
	clientID := "lclient_" + suffix
	tokenID := "ltok_" + suffix
	secret := "lane_secret_" + suffix
	salt := "lane_salt_" + suffix
	if err := owner.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at, expires_at)
		VALUES ($1, 'session', $1, $2, $3, $4, now(), now() + interval '24 hours')
		ON CONFLICT (client_id) DO NOTHING`,
		clientID, tokenID, operatorHMACHex(salt, secret), salt); err != nil {
		t.Fatalf("seed lane client: %v", err)
	}
	for _, capName := range []string{"claim", "write", "read", "review"} {
		if err := owner.Exec(ctx, `
			INSERT INTO striatumd.client_capabilities(capability_id, client_id, repository_id, capability, granted_at, expires_at, session_id)
			VALUES ($1, $2, $3, $4, now(), now() + interval '24 hours', $5)`,
			"lcap_"+suffix+"_"+capName, clientID, repositoryID, capName, sessionID); err != nil {
			t.Fatalf("seed lane capability %q: %v", capName, err)
		}
	}
	return tokenID + "." + secret
}

// operatorHMACHex reproduces the token-hash scheme the PostgresAuthorizer
// validates against (HMAC-SHA256 keyed on the salt over the secret).
func operatorHMACHex(salt, secret string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

// isRPCCode reports whether err is an rpc.Error carrying the given code.
func isRPCCode(err error, code string) bool {
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == code
	}
	return false
}
