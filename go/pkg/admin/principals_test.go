package admin

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// seedRepos inserts the two repositories used by the cross-repo isolation
// tests. Mirrors the rpc integration fixtures so capability scoping behaves
// against real rows.
func seedRepos(t *testing.T, runner db.Runner, now time.Time) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES
		  ('repo_a','ident_a','/tmp/repo-a','/tmp/repo-a/.striatum','repo-a',$1,23,'active'),
		  ('repo_b','ident_b','/tmp/repo-b','/tmp/repo-b/.striatum','repo-b',$1,23,'active')`,
		now,
	); err != nil {
		t.Fatalf("seed repositories: %v", err)
	}
}

func seedClient(t *testing.T, runner db.Runner, clientID, tokenID, secret, salt string, now time.Time) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.clients (
		  client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at
		) VALUES ($1,'test',$1,$2,$3,$4,$5)`,
		clientID, tokenID, hmacHexToken(salt, secret), salt, now,
	); err != nil {
		t.Fatalf("seed client %s: %v", clientID, err)
	}
}

func seedCapability(t *testing.T, runner db.Runner, capID, clientID, repoID string, capability rpc.Capability, sessionID string, now time.Time) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.client_capabilities (
		  capability_id, client_id, repository_id, capability, granted_at, session_id
		) VALUES ($1,$2,$3,$4,$5,$6)`,
		capID, clientID, nullableTextArg(repoID), string(capability), now, nullableTextArg(sessionID),
	); err != nil {
		t.Fatalf("seed capability %s: %v", capID, err)
	}
}

func nullableTextArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// TestPrincipalLinkResolvesAuditAttribution proves per-principal audit
// attribution: a client linked to a principal resolves to THAT principal, two
// principals do not bleed into each other, and an unlinked client resolves to
// "no principal" (back-compat). This is the linkage the daemon-global audit
// chain (audit_log.client_id) uses to attribute every mutation to a principal.
func TestPrincipalLinkResolvesAuditAttribution(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)

	if err := CreatePrincipal(ctx, runner, "prin_alice", "human", "Alice"); err != nil {
		t.Fatalf("create principal alice: %v", err)
	}
	if err := CreatePrincipal(ctx, runner, "prin_bob", "human", "Bob"); err != nil {
		t.Fatalf("create principal bob: %v", err)
	}
	seedClient(t, runner, "client_alice", "tok_alice", "secret", "salt", now)
	seedClient(t, runner, "client_bob", "tok_bob", "secret", "salt", now)
	seedClient(t, runner, "client_orphan", "tok_orphan", "secret", "salt", now)

	if err := LinkClientToPrincipal(ctx, runner, "prin_alice", "client_alice"); err != nil {
		t.Fatalf("link alice client: %v", err)
	}
	if err := LinkClientToPrincipal(ctx, runner, "prin_bob", "client_bob"); err != nil {
		t.Fatalf("link bob client: %v", err)
	}

	for clientID, wantPrincipal := range map[string]string{
		"client_alice": "prin_alice",
		"client_bob":   "prin_bob",
	} {
		ref, ok, err := ResolvePrincipalForClient(ctx, runner, clientID)
		if err != nil {
			t.Fatalf("resolve %s: %v", clientID, err)
		}
		if !ok || ref.PrincipalID != wantPrincipal {
			t.Fatalf("client %s resolved to %#v (ok=%v), want principal %s", clientID, ref, ok, wantPrincipal)
		}
	}

	// Cross-principal: Alice's client must NEVER resolve to Bob.
	aliceRef, _, err := ResolvePrincipalForClient(ctx, runner, "client_alice")
	if err != nil {
		t.Fatalf("resolve alice: %v", err)
	}
	if aliceRef.PrincipalID == "prin_bob" {
		t.Fatal("cross-principal attribution bleed: alice client resolved to bob")
	}

	// An unlinked client resolves to "no principal" (ok=false), not a wrong one.
	_, ok, err := ResolvePrincipalForClient(ctx, runner, "client_orphan")
	if err != nil {
		t.Fatalf("resolve orphan: %v", err)
	}
	if ok {
		t.Fatal("unlinked client unexpectedly resolved to a principal")
	}
}

// TestPrincipalLinkRejectsSecondActivePrincipal proves a client cannot be
// claimed by two principals at once (the active-link unique index), keeping
// audit attribution unambiguous.
func TestPrincipalLinkRejectsSecondActivePrincipal(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)

	if err := CreatePrincipal(ctx, runner, "prin_one", "ai_operator", "One"); err != nil {
		t.Fatalf("create principal one: %v", err)
	}
	if err := CreatePrincipal(ctx, runner, "prin_two", "ai_operator", "Two"); err != nil {
		t.Fatalf("create principal two: %v", err)
	}
	seedClient(t, runner, "client_shared", "tok_shared", "secret", "salt", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_one", "client_shared"); err != nil {
		t.Fatalf("first link: %v", err)
	}
	if err := LinkClientToPrincipal(ctx, runner, "prin_two", "client_shared"); err == nil {
		t.Fatal("expected the second active link of one client to a different principal to be refused")
	}
}

// TestListPrincipalsSurfacesPerPrincipalCapabilityRepoScope proves doctor's
// data source lists EACH principal with only its OWN capability/repo scope —
// the cross-principal + cross-repo isolation the operator must be able to see.
func TestListPrincipalsSurfacesPerPrincipalCapabilityRepoScope(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	seedRepos(t, runner, now)

	if err := CreatePrincipal(ctx, runner, "prin_alice", "human", "Alice"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := CreatePrincipal(ctx, runner, "prin_bob", "human", "Bob"); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	// Alice: write on repo_a only. Bob: read on repo_b only.
	seedClient(t, runner, "client_alice", "tok_alice", "secret", "salt", now)
	seedCapability(t, runner, "cap_alice", "client_alice", "repo_a", rpc.CapabilityWrite, "", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_alice", "client_alice"); err != nil {
		t.Fatalf("link alice: %v", err)
	}
	seedClient(t, runner, "client_bob", "tok_bob", "secret", "salt", now)
	seedCapability(t, runner, "cap_bob", "client_bob", "repo_b", rpc.CapabilityRead, "", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_bob", "client_bob"); err != nil {
		t.Fatalf("link bob: %v", err)
	}

	principals, err := ListPrincipals(ctx, runner)
	if err != nil {
		t.Fatalf("list principals: %v", err)
	}
	byID := map[string]PrincipalScope{}
	for _, p := range principals {
		byID[p.PrincipalID] = p
	}
	alice, ok := byID["prin_alice"]
	if !ok {
		t.Fatalf("alice missing from principals: %#v", principals)
	}
	bob, ok := byID["prin_bob"]
	if !ok {
		t.Fatalf("bob missing from principals: %#v", principals)
	}
	// Alice sees ONLY write@repo_a; never bob's read@repo_b.
	if got := scopeKeys(alice); len(got) != 1 || got[0] != "write@repo_a" {
		t.Fatalf("alice scope = %v, want [write@repo_a]", got)
	}
	if got := scopeKeys(bob); len(got) != 1 || got[0] != "read@repo_b" {
		t.Fatalf("bob scope = %v, want [read@repo_b]", got)
	}
	// Cross-repo isolation surfaced: neither principal lists the other's repo.
	for _, key := range scopeKeys(alice) {
		if key == "read@repo_b" {
			t.Fatal("cross-repo bleed: alice scope contains bob's repo_b grant")
		}
	}
	if alice.ClientCount != 1 || bob.ClientCount != 1 {
		t.Fatalf("client counts = alice:%d bob:%d, want 1/1", alice.ClientCount, bob.ClientCount)
	}
}

func scopeKeys(p PrincipalScope) []string {
	keys := make([]string, 0, len(p.Capabilities))
	for _, c := range p.Capabilities {
		repo := "<global>"
		if c.RepositoryID != nil {
			repo = *c.RepositoryID
		}
		keys = append(keys, string(c.Capability)+"@"+repo)
	}
	return keys
}

// TestCrossRepoIsolationIsEnforcedAndAttributed proves the cross-repo
// isolation invariant at the principal granularity: principal Alice (client
// granted write on repo_a only) is ALLOWED on repo_a and DENIED on repo_b with
// capability_scope_mismatch, and the allowed action attributes back to Alice.
// No principal reads another repo's live state without a grant.
func TestCrossRepoIsolationIsEnforcedAndAttributed(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	seedRepos(t, runner, now)

	if err := CreatePrincipal(ctx, runner, "prin_alice", "human", "Alice"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	seedClient(t, runner, "client_alice", "tok_alice", "secret", "salt", now)
	seedCapability(t, runner, "cap_alice", "client_alice", "repo_a", rpc.CapabilityWrite, "", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_alice", "client_alice"); err != nil {
		t.Fatalf("link alice: %v", err)
	}

	required := rpc.CapabilityWrite
	auth := rpc.PostgresAuthorizer{Runner: runner, Clock: func() time.Time { return now }}

	allowed := auth.Authorize(&required, "repo_a", "tok_alice.secret")
	if allowed.Decision != "allowed" {
		t.Fatalf("alice on repo_a = %#v, want allowed", allowed)
	}
	// The allowed action attributes to Alice via the client->principal link.
	ref, ok, err := ResolvePrincipalForClient(ctx, runner, allowed.ClientID)
	if err != nil || !ok || ref.PrincipalID != "prin_alice" {
		t.Fatalf("attribution of allowed action = %#v ok=%v err=%v, want prin_alice", ref, ok, err)
	}

	denied := auth.Authorize(&required, "repo_b", "tok_alice.secret")
	if denied.Decision != "denied" || denied.DenialReason != "capability_scope_mismatch" {
		t.Fatalf("alice on repo_b = %#v, want denied/capability_scope_mismatch", denied)
	}
}

// TestCrossPrincipalSessionIsolation proves principal A's session-bound token
// cannot act for principal B's session. Each principal holds a client whose
// grant is bound to that principal's own session; the resolved AuthContext
// surfaces the correct bound session, and the canonical MayActAsSession
// predicate (shared with the mutations enforcement point) refuses A acting as
// B's session while permitting A acting as its own.
func TestCrossPrincipalSessionIsolation(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	seedRepos(t, runner, now)

	if err := CreatePrincipal(ctx, runner, "prin_alice", "human", "Alice"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := CreatePrincipal(ctx, runner, "prin_bob", "human", "Bob"); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	seedClient(t, runner, "client_alice", "tok_alice", "secret", "salt", now)
	seedCapability(t, runner, "cap_alice", "client_alice", "repo_a", rpc.CapabilityWrite, "sess_alice", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_alice", "client_alice"); err != nil {
		t.Fatalf("link alice: %v", err)
	}
	seedClient(t, runner, "client_bob", "tok_bob", "secret", "salt", now)
	seedCapability(t, runner, "cap_bob", "client_bob", "repo_a", rpc.CapabilityWrite, "sess_bob", now)
	if err := LinkClientToPrincipal(ctx, runner, "prin_bob", "client_bob"); err != nil {
		t.Fatalf("link bob: %v", err)
	}

	required := rpc.CapabilityWrite
	auth := rpc.PostgresAuthorizer{Runner: runner, Clock: func() time.Time { return now }}

	aliceAuth := auth.Authorize(&required, "repo_a", "tok_alice.secret")
	if aliceAuth.Decision != "allowed" || !aliceAuth.IsSessionBound() || aliceAuth.SessionID != "sess_alice" {
		t.Fatalf("alice auth = %#v, want allowed bound to sess_alice", aliceAuth)
	}
	bobAuth := auth.Authorize(&required, "repo_a", "tok_bob.secret")
	if bobAuth.Decision != "allowed" || bobAuth.SessionID != "sess_bob" {
		t.Fatalf("bob auth = %#v, want allowed bound to sess_bob", bobAuth)
	}

	// Cross-principal session impersonation is refused; same-session is allowed.
	if aliceAuth.MayActAsSession("sess_bob") {
		t.Fatal("cross-principal session isolation breached: alice may act as bob's session")
	}
	if !aliceAuth.MayActAsSession("sess_alice") {
		t.Fatal("alice should be able to act as her own session")
	}

	// Attribution of each principal's allowed action stays distinct.
	aliceRef, _, err := ResolvePrincipalForClient(ctx, runner, aliceAuth.ClientID)
	if err != nil {
		t.Fatalf("resolve alice: %v", err)
	}
	bobRef, _, err := ResolvePrincipalForClient(ctx, runner, bobAuth.ClientID)
	if err != nil {
		t.Fatalf("resolve bob: %v", err)
	}
	if aliceRef.PrincipalID != "prin_alice" || bobRef.PrincipalID != "prin_bob" {
		t.Fatalf("attribution alice=%s bob=%s, want prin_alice/prin_bob", aliceRef.PrincipalID, bobRef.PrincipalID)
	}
}
