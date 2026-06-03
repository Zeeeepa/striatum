package admin

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestCreateTokenAttributesClientToPrincipal proves a token minted with a
// principal_id is auto-linked to that principal (creating it on first use when
// principal_kind/principal_display_name are supplied), so grant across
// principals attributes the new client. Uses the real pool (the handler opens
// its own transaction); the ephemeral database is dropped at cleanup.
func TestCreateTokenAttributesClientToPrincipal(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	svc := Service{Runner: runner}

	result, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"display_name":           "alice cli",
		"capabilities":           []any{"read"},
		"principal_id":           "prin_alice",
		"principal_kind":         "human",
		"principal_display_name": "Alice",
		"expires_in":             float64(120),
	}})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	clientID, _ := result["client_id"].(string)
	if clientID == "" {
		t.Fatalf("missing client_id in result: %#v", result)
	}
	if result["principal_id"] != "prin_alice" {
		t.Fatalf("result principal_id = %v, want prin_alice", result["principal_id"])
	}

	ref, ok, err := ResolvePrincipalForClient(ctx, runner, clientID)
	if err != nil || !ok || ref.PrincipalID != "prin_alice" {
		t.Fatalf("minted client resolved to %#v ok=%v err=%v, want prin_alice", ref, ok, err)
	}
	if ref.Kind != "human" || ref.DisplayName != "Alice" {
		t.Fatalf("principal metadata = kind:%q name:%q, want human/Alice", ref.Kind, ref.DisplayName)
	}

	// A second token for the same principal links the new client to the same
	// principal — proving rotation/regrant keeps a stable principal identity.
	result2, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"display_name": "alice second",
		"capabilities": []any{"read"},
		"principal_id": "prin_alice",
		"expires_in":   float64(120),
	}})
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}
	client2, _ := result2["client_id"].(string)
	ref2, ok2, err := ResolvePrincipalForClient(ctx, runner, client2)
	if err != nil || !ok2 || ref2.PrincipalID != "prin_alice" {
		t.Fatalf("second client resolved to %#v ok=%v err=%v, want prin_alice", ref2, ok2, err)
	}

	scopes, err := ListPrincipals(ctx, runner)
	if err != nil {
		t.Fatalf("list principals: %v", err)
	}
	var alice *PrincipalScope
	for i := range scopes {
		if scopes[i].PrincipalID == "prin_alice" {
			alice = &scopes[i]
			break
		}
	}
	if alice == nil {
		t.Fatalf("alice missing from principals: %#v", scopes)
	}
	if alice.ClientCount != 2 {
		t.Fatalf("alice client count = %d, want 2 (two minted tokens)", alice.ClientCount)
	}
}

// TestRotateTokenCarriesPrincipalAcrossRotation proves token rotation keeps a
// stable principal identity: the replacement client is attributed to the same
// principal as the rotated-from client, and the old client's active link is
// dropped (rotation invalidates the old token).
func TestRotateTokenCarriesPrincipalAcrossRotation(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	svc := Service{Runner: runner}

	created, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"display_name":           "alice cli",
		"capabilities":           []any{"read"},
		"principal_id":           "prin_alice",
		"principal_kind":         "human",
		"principal_display_name": "Alice",
		"expires_in":             float64(3600),
	}})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	oldClient, _ := created["client_id"].(string)
	fullToken, _ := created["token"].(string)

	rotated, err := svc.RotateToken(ctx, rpc.Envelope{Params: map[string]any{
		"token": fullToken,
	}})
	if err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	if rotated["principal_id"] != "prin_alice" {
		t.Fatalf("rotated principal_id = %v, want prin_alice", rotated["principal_id"])
	}
	newClient, _ := rotated["client_id"].(string)
	if newClient == "" || newClient == oldClient {
		t.Fatalf("rotation did not mint a distinct client: old=%s new=%s", oldClient, newClient)
	}

	// The replacement client resolves to the same principal.
	ref, ok, err := ResolvePrincipalForClient(ctx, runner, newClient)
	if err != nil || !ok || ref.PrincipalID != "prin_alice" {
		t.Fatalf("rotated client resolved to %#v ok=%v err=%v, want prin_alice", ref, ok, err)
	}
	// The old client's active link is gone (rotation invalidated the old token).
	_, oldStillLinked, err := ResolvePrincipalForClient(ctx, runner, oldClient)
	if err != nil {
		t.Fatalf("resolve old client: %v", err)
	}
	if oldStillLinked {
		t.Fatal("rotated-from client must no longer be actively attributed")
	}

	// The principal now owns exactly one active client (the replacement).
	scopes, err := ListPrincipals(ctx, runner)
	if err != nil {
		t.Fatalf("list principals: %v", err)
	}
	for _, s := range scopes {
		if s.PrincipalID == "prin_alice" && s.ClientCount != 1 {
			t.Fatalf("alice active client count = %d, want 1 after rotation", s.ClientCount)
		}
	}
}

// TestCreateTokenRejectsUnknownPrincipalWithoutKind proves that referencing a
// principal_id that does not exist, without supplying principal_kind to create
// it, is refused rather than silently creating an unkinded principal.
func TestCreateTokenRejectsUnknownPrincipalWithoutKind(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	svc := Service{Runner: pool.Runner}
	_, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"capabilities": []any{"read"},
		"principal_id": "prin_ghost",
		"expires_in":   float64(120),
	}})
	if code(err) != "not_found" {
		t.Fatalf("unknown principal without kind error = %v, want not_found", err)
	}
}

// TestCreateTokenRefusesPrincipalRedefinition proves a principal's identity
// cannot be silently rewritten: minting a token for an existing principal_id
// with a conflicting kind/display_name is refused.
func TestCreateTokenRefusesPrincipalRedefinition(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	svc := Service{Runner: pool.Runner}
	if _, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"capabilities":           []any{"read"},
		"principal_id":           "prin_alice",
		"principal_kind":         "human",
		"principal_display_name": "Alice",
		"expires_in":             float64(120),
	}}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.CreateToken(ctx, rpc.Envelope{Params: map[string]any{
		"capabilities":           []any{"read"},
		"principal_id":           "prin_alice",
		"principal_kind":         "ai_operator",
		"principal_display_name": "Not Alice",
		"expires_in":             float64(120),
	}})
	if code(err) != "conflict" {
		t.Fatalf("redefinition error = %v, want conflict", err)
	}
}
