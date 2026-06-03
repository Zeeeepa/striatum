package rpc

import "testing"

// TestMayActAsSession proves the canonical RFC 0107 / RFC 0096 V2 per-session
// predicate: a session-bound token may act only as its own session; an unbound
// (operator/coordinator) token may act as any session. This is the same rule
// mutations.enforceSessionBinding enforces at its call site, stated once here
// so the per-principal isolation tests can rely on it independently.
func TestMayActAsSession(t *testing.T) {
	bound := AuthContext{SessionID: "sess_alice", Decision: "allowed"}
	if !bound.MayActAsSession("sess_alice") {
		t.Fatal("a bound token must be able to act as its own session")
	}
	if bound.MayActAsSession("sess_bob") {
		t.Fatal("a bound token must NOT be able to act as another session")
	}

	unbound := AuthContext{Decision: "allowed"}
	if !unbound.MayActAsSession("sess_alice") || !unbound.MayActAsSession("sess_bob") {
		t.Fatal("an unbound operator token may act as any session under operator-override")
	}
	if unbound.IsSessionBound() {
		t.Fatal("an empty SessionID must report session-unbound")
	}
}
