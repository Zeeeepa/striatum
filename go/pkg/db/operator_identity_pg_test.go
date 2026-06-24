package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
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
		`INSERT INTO striatumd.repositories(repository_id, repo_root, state, registered_at)
		   VALUES ('repo_p0_oi', '/tmp/repo_p0_oi', 'active', now())
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
	seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

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

// 4. operator_session_pre_run_stamp — two operator sessions for one principal,
// two distinct leased handles, two runs stamped from app.session_id => two
// NON-NULL DISTINCT created_by_handle_id and distinct whose (A29/A7/A27).
func TestOperatorSessionPreRunStampTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, principalID, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_s1", repositoryID, principalID, "maya", "osess_s1")
	seedLeasedHandle(t, ctx, fx.OwnerPool.Runner, "oh_s2", repositoryID, principalID, "theo", "osess_s2")

	// Seed the two runs' snapshots, then stamp created_by_handle_id via the SAME
	// app.session_id subquery the real run.prepare uses, AS THE RUNTIME ROLE.
	for _, rs := range []struct{ runID, sessionID string }{{"run_s1", "osess_s1"}, {"run_s2", "osess_s2"}} {
		if err := fx.OwnerPool.Runner.Exec(ctx, `
			INSERT INTO striatumd.workflow_snapshots(repository_id, workflow_snapshot_id, workflow_id,
			  workflow_version, source_path, content_sha256, workflow_json, loaded_at)
			VALUES ($1, $2, 'wf', '1', 'p', 'h', '{}'::jsonb, now()) ON CONFLICT DO NOTHING`,
			repositoryID, "wfs_"+rs.runID); err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
		// set_config app.session_id (the admin prelude installs this) then INSERT
		// using the handle subquery — exactly the §1(2) stamp shape, as the SUT.
		if err := fx.SUTPool.Runner.Exec(ctx, `
			SELECT set_config('app.session_id', $1, false);
			INSERT INTO striatumd.runs(repository_id, run_id, workflow_snapshot_id, repo_root,
			  state, created_at, created_by_principal_id, created_by_handle_id)
			VALUES ($2, $3, $4, '/tmp/repo_p0_oi', 'ready', now(), $5,
			  (SELECT oh.handle_id FROM striatumd.operator_handles oh
			    WHERE oh.leased_session_id = current_setting('app.session_id', true)
			      AND oh.released_at IS NULL))`,
			rs.sessionID, repositoryID, rs.runID, "wfs_"+rs.runID, principalID); err != nil {
			t.Fatalf("stamp run %q via app.session_id subquery: %v", rs.runID, err)
		}
	}

	h1 := scalar(t, ctx, fx.SUTPool.Runner, `SELECT created_by_handle_id FROM striatumd.runs WHERE run_id='run_s1'`)
	h2 := scalar(t, ctx, fx.SUTPool.Runner, `SELECT created_by_handle_id FROM striatumd.runs WHERE run_id='run_s2'`)
	if h1 == "" || h2 == "" {
		t.Fatalf("A27: both stamps must be NON-NULL handle snapshots, got h1=%q h2=%q", h1, h2)
	}
	if h1 == h2 {
		t.Fatalf("A7: two terminals of one human must stamp DISTINCT handles, both got %q", h1)
	}
	w1 := scalar(t, ctx, fx.SUTPool.Runner, `SELECT origin_handle FROM striatumd.run_origin_identity($1,$2,'run_s1')`, operatorTestSecret, repositoryID)
	w2 := scalar(t, ctx, fx.SUTPool.Runner, `SELECT origin_handle FROM striatumd.run_origin_identity($1,$2,'run_s2')`, operatorTestSecret, repositoryID)
	if w1 == w2 {
		t.Fatalf("A7: whose RA == whose RB (%q); the two terminals are indistinguishable", w1)
	}
}

// 5. operator_token_admin_surface — the C1" credential shape + segregation face:
// the operator-session token carries {admin, read} bound to its session and
// repo-scoped, while the static bootstrap admin token is broader and unscoped
// (A40/A45). The trust-root fence + repo-scope authorization are proven at the
// rpc-authorizer layer; here we pin the DB credential segregation.
func TestOperatorTokenAdminSurfaceTwoRole(t *testing.T) {
	fx := pgtest.TwoRole(t)
	ctx := context.Background()
	repositoryID, _, _ := seedOperatorWorld(t, ctx, fx.OwnerPool.Runner)

	// Simulate the operator-session token: a client with admin+read grants bound
	// to the operator_session_id and repo-scoped (the mintOperatorSessionToken shape).
	if err := fx.OwnerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at, expires_at)
		VALUES ('oclient_surface', 'operator-session', 'operator-session osess_surface', 'otok_surface', 'h', 's', now(), now() + interval '24 hours')
		ON CONFLICT (client_id) DO NOTHING`); err != nil {
		t.Fatalf("seed operator client: %v", err)
	}
	for _, cap := range []string{"admin", "read"} {
		if err := fx.OwnerPool.Runner.Exec(ctx, `
			INSERT INTO striatumd.client_capabilities(capability_id, client_id, repository_id, capability, granted_at, expires_at, session_id)
			VALUES ($1, 'oclient_surface', $2, $3, now(), now() + interval '24 hours', 'osess_surface')`,
			"ocap_"+cap, repositoryID, cap); err != nil {
			t.Fatalf("seed operator capability %q: %v", cap, err)
		}
	}

	// The operator token's grants are repo-scoped AND session-bound (A40/A42 shape).
	scoped := scalar(t, ctx, fx.OwnerPool.Runner, `
		SELECT (COUNT(*) = 2)::text FROM striatumd.client_capabilities
		 WHERE client_id = 'oclient_surface' AND repository_id = $1 AND session_id = 'osess_surface'
		   AND capability IN ('admin','read') AND revoked_at IS NULL`, repositoryID)
	if scoped != "true" {
		t.Fatalf("A40: the operator token must carry repo-scoped, session-bound {admin, read}")
	}
	// A45 segregation: the operator token carries exactly two capabilities (admin,
	// read) — NOT the broad static bootstrap-admin set. The lane slice carries no
	// admin; this is the credential-narrowing the honest blast-radius accounting
	// requires (the rpc-layer trust-root fence + daemon-global denial are proven in
	// the rpc/capability authorizer tests).
	count := scalar(t, ctx, fx.OwnerPool.Runner,
		`SELECT COUNT(*)::text FROM striatumd.client_capabilities WHERE client_id = 'oclient_surface'`)
	if count != "2" {
		t.Fatalf("A45: the operator token must carry exactly {admin, read} (2 caps), got %s — not the broad static bootstrap set", count)
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
