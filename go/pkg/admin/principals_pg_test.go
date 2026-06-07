package admin

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// identityClosedDB returns owner + runtime pools on a database with every
// owner bundle applied (so the RFC 0114 identity read close is live) and a
// registered daemon-authority secret. Tests opt in to the projection path by
// installing the secret via db.SetAuthorityRuntime; the helper resets the
// process authority runtime on cleanup.
func identityClosedDB(t *testing.T) (*db.Pool, *db.Pool, string) {
	t.Helper()
	ownerPool, runtimePool := pgtest.Pools(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, ownerPool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}
	const secret = "identity-close-secret"
	const salt = "identity-close-salt"
	if err := ownerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ('inst-identity-close', 'striatumd_rw',
		        encode(striatumd.digest(convert_to($1 || $2, 'UTF8'), 'sha256'), 'hex'), $2)`,
		secret, salt); err != nil {
		t.Fatalf("register authority secret: %v", err)
	}
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })
	return ownerPool, runtimePool, secret
}

// TestPrincipalReadProjectionParity is the RFC 0114 parity gate: on a bundled
// database, ListPrincipals and ResolvePrincipalForClient return identical DTOs
// through the direct path (secretless, reading as the table-owning role) and
// the projection path (authority secret set, reading as the runtime role whose
// direct SELECT the bundle revoked). The projection bodies are verbatim
// transplants of the direct SQL, so any divergence is a bug.
func TestPrincipalReadProjectionParity(t *testing.T) {
	ownerPool, runtimePool, secret := identityClosedDB(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)
	seedRepos(t, ownerPool.Runner, now)

	if err := ownerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.principals(principal_id, principal_kind, display_name, created_at)
		VALUES ('prin_parity', 'human', 'parity person', $1),
		       ('prin_idle', 'service', 'idle service', $1)`, now); err != nil {
		t.Fatalf("seed principals: %v", err)
	}
	if err := ownerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.clients (
		  client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at
		) VALUES ('client_parity','test','parity client','tok_parity','hash','salt',$1)`, now); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := ownerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.principal_clients(principal_id, client_id, linked_at)
		VALUES ('prin_parity', 'client_parity', $1)`, now); err != nil {
		t.Fatalf("seed principal link: %v", err)
	}
	if err := ownerPool.Runner.Exec(ctx, `
		INSERT INTO striatumd.client_capabilities (
		  capability_id, client_id, repository_id, capability, granted_at, session_id
		) VALUES ('cap_parity_read','client_parity','repo_a',$1,$2,'sess_parity'),
		         ('cap_parity_write','client_parity',NULL,$3,$2,NULL)`,
		string(rpc.CapabilityRead), now, string(rpc.CapabilityWrite)); err != nil {
		t.Fatalf("seed capabilities: %v", err)
	}

	// Direct path: secretless, owner runner (the runtime role can no longer
	// read these tables directly — that is the point of the bundle).
	db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false)
	directScopes, err := ListPrincipals(ctx, ownerPool.Runner)
	if err != nil {
		t.Fatalf("direct ListPrincipals: %v", err)
	}
	directRef, directOK, err := ResolvePrincipalForClient(ctx, ownerPool.Runner, "client_parity")
	if err != nil {
		t.Fatalf("direct ResolvePrincipalForClient: %v", err)
	}

	// Projection path: authority secret installed, runtime runner.
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV2, "", false)
	projectedScopes, err := ListPrincipals(ctx, runtimePool.Runner)
	if err != nil {
		t.Fatalf("projection ListPrincipals: %v", err)
	}
	projectedRef, projectedOK, err := ResolvePrincipalForClient(ctx, runtimePool.Runner, "client_parity")
	if err != nil {
		t.Fatalf("projection ResolvePrincipalForClient: %v", err)
	}

	if len(directScopes) != 2 {
		t.Fatalf("direct ListPrincipals returned %d scopes, want 2: %#v", len(directScopes), directScopes)
	}
	if !directOK || directRef.PrincipalID != "prin_parity" {
		t.Fatalf("direct resolve = (%#v, %v), want prin_parity", directRef, directOK)
	}
	if !reflect.DeepEqual(directScopes, projectedScopes) {
		t.Fatalf("ListPrincipals parity broke:\ndirect:    %#v\nprojected: %#v", directScopes, projectedScopes)
	}
	if directOK != projectedOK || !reflect.DeepEqual(directRef, projectedRef) {
		t.Fatalf("ResolvePrincipalForClient parity broke:\ndirect:    (%#v, %v)\nprojected: (%#v, %v)",
			directRef, directOK, projectedRef, projectedOK)
	}
}

// TestCreatePrincipalSemanticsAfterReadClose runs CreatePrincipal as
// striatumd_rw on a closed database: a fresh insert succeeds (plain INSERT
// needs no SELECT), an idempotent same-definition replay returns success, and
// a conflicting redefinition is refused. The idempotency pre-check reads
// through get_principal, so this is where a full SELECT denial on principals
// could otherwise hide a projection/direct-SQL split (RFC 0114 review F1).
func TestCreatePrincipalSemanticsAfterReadClose(t *testing.T) {
	ownerPool, runtimePool, secret := identityClosedDB(t)
	ctx := context.Background()
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV2, "", false)

	if err := CreatePrincipal(ctx, runtimePool.Runner, "prin_close", "human", "close person"); err != nil {
		t.Fatalf("fresh CreatePrincipal as the runtime role: %v", err)
	}
	if got, err := ownerPool.Runner.QueryScalar(ctx,
		"SELECT display_name FROM striatumd.principals WHERE principal_id = 'prin_close'"); err != nil || got != "close person" {
		t.Fatalf("created principal row = (%q, %v), want close person", got, err)
	}
	if err := CreatePrincipal(ctx, runtimePool.Runner, "prin_close", "human", "close person"); err != nil {
		t.Fatalf("idempotent CreatePrincipal replay: %v", err)
	}
	if err := CreatePrincipal(ctx, runtimePool.Runner, "prin_close", "human", "different person"); err == nil {
		t.Fatal("conflicting CreatePrincipal redefinition succeeded; want refusal")
	}
	if got, _ := ownerPool.Runner.QueryScalar(ctx,
		"SELECT display_name FROM striatumd.principals WHERE principal_id = 'prin_close'"); got != "close person" {
		t.Fatalf("conflicting redefinition rewrote display_name to %q", got)
	}
}

// TestLinkUpsertAndUnlinkSurviveReadClose pins the RFC 0114 write-entanglement
// analysis as striatumd_rw on a closed database: link, idempotent re-link,
// conflicting-principal refusal, unlink, and unlink-then-revive all succeed.
// The ON CONFLICT upsert is the pinned PostgreSQL privilege assumption (its DO
// UPDATE reads no existing columns, so INSERT + UPDATE suffice without
// SELECT); if PostgreSQL ever demands more, this test goes red (RFC 0114 Open
// Question 4 holds the contingency).
func TestLinkUpsertAndUnlinkSurviveReadClose(t *testing.T) {
	_, runtimePool, secret := identityClosedDB(t)
	ctx := context.Background()
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV2, "", false)

	for _, p := range [][2]string{{"prin_link1", "link person one"}, {"prin_link2", "link person two"}} {
		if err := CreatePrincipal(ctx, runtimePool.Runner, p[0], "human", p[1]); err != nil {
			t.Fatalf("seed principal %s: %v", p[0], err)
		}
	}

	if err := LinkClientToPrincipal(ctx, runtimePool.Runner, "prin_link1", "cl_link"); err != nil {
		t.Fatalf("fresh link: %v", err)
	}
	if err := LinkClientToPrincipal(ctx, runtimePool.Runner, "prin_link1", "cl_link"); err != nil {
		t.Fatalf("idempotent re-link: %v", err)
	}
	if err := LinkClientToPrincipal(ctx, runtimePool.Runner, "prin_link2", "cl_link"); err == nil {
		t.Fatal("linking an actively-linked client to a different principal succeeded; want refusal")
	}

	// unlinkClientFromPrincipals is the live UPDATE ... WHERE that reads
	// client_id/unlinked_at — the columns the bundle's column gate preserves.
	tx, err := runtimePool.Runner.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin unlink tx: %v", err)
	}
	if err := unlinkClientFromPrincipals(ctx, tx, "cl_link"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("unlink as the runtime role: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit unlink: %v", err)
	}
	if _, ok, err := ResolvePrincipalForClient(ctx, runtimePool.Runner, "cl_link"); err != nil || ok {
		t.Fatalf("resolve after unlink = (ok=%v, err=%v), want unattributed", ok, err)
	}

	// Revive the prior unlinked row for the same pair (ON CONFLICT DO UPDATE).
	if err := LinkClientToPrincipal(ctx, runtimePool.Runner, "prin_link1", "cl_link"); err != nil {
		t.Fatalf("unlink-then-revive link: %v", err)
	}
	ref, ok, err := ResolvePrincipalForClient(ctx, runtimePool.Runner, "cl_link")
	if err != nil || !ok || ref.PrincipalID != "prin_link1" {
		t.Fatalf("resolve after revive = (%#v, ok=%v, err=%v), want prin_link1", ref, ok, err)
	}
}

// TestTokenRotationCarriesPrincipalAfterReadClose is the end-to-end RFC 0114
// rotation drill as striatumd_rw on a closed database: rotation re-attributes
// the replacement client to the rotated-from client's principal and unlinks
// the old one, with every identity read going through the projections.
func TestTokenRotationCarriesPrincipalAfterReadClose(t *testing.T) {
	ownerPool, runtimePool, secret := identityClosedDB(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC)
	seedRepos(t, ownerPool.Runner, now)
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV2, "", false)

	svc := Service{Runner: runtimePool.Runner}
	created, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"capabilities":           []any{string(rpc.CapabilityRead)},
		"repository_id":          "repo_a",
		"principal_id":           "prin_rotate",
		"principal_kind":         "human",
		"principal_display_name": "rotating person",
	}})
	if err != nil {
		t.Fatalf("create token with principal attribution: %v", err)
	}
	if created["principal_id"] != "prin_rotate" {
		t.Fatalf("create principal_id = %v, want prin_rotate", created["principal_id"])
	}
	oldClientID, _ := created["client_id"].(string)
	token, _ := created["token"].(string)
	if oldClientID == "" || token == "" {
		t.Fatalf("create result missing client_id/token: %#v", created)
	}

	rotated, err := svc.RotateToken(ctx, rpc.Envelope{Params: map[string]any{"token": token}})
	if err != nil {
		t.Fatalf("rotate token on the closed database: %v", err)
	}
	if rotated["principal_id"] != "prin_rotate" {
		t.Fatalf("rotation principal_id = %v, want carried prin_rotate", rotated["principal_id"])
	}
	newClientID, _ := rotated["client_id"].(string)
	if newClientID == "" || newClientID == oldClientID {
		t.Fatalf("rotation client_id = %q (old %q), want a fresh client", newClientID, oldClientID)
	}

	ref, ok, err := ResolvePrincipalForClient(ctx, runtimePool.Runner, newClientID)
	if err != nil || !ok || ref.PrincipalID != "prin_rotate" {
		t.Fatalf("resolve new client = (%#v, ok=%v, err=%v), want prin_rotate", ref, ok, err)
	}
	if _, ok, err := ResolvePrincipalForClient(ctx, runtimePool.Runner, oldClientID); err != nil || ok {
		t.Fatalf("resolve old client = (ok=%v, err=%v), want unlinked", ok, err)
	}
}
