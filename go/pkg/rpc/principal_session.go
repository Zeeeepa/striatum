package rpc

// principal_session.go carries the RFC 0107 multi-principal predicate that
// lives on the authorization substrate (not in the mutations handlers, which
// own the enforcement call site). It encodes, in ONE canonical place, the rule
// that the per-session enforcement (mutations.enforceSessionBinding) and the
// per-principal isolation tests both rely on.

// MayActAsSession reports whether this resolved AuthContext is permitted to act
// AS the given session.
//
// RFC 0096 V2 / RFC 0107: a session-UNBOUND token (the operator/coordinator
// grant) may act as any session under the honest operator-override path, so it
// returns true. A session-BOUND token (minted for a single lane, and — under
// RFC 0107 — held by exactly one principal) may act ONLY as its own bound
// session; acting as any other session is refused. This is what stops a
// compromised lane, or principal A's token, from acting for principal B's
// session. The mutations enforcement point (enforceSessionBinding) refuses with
// capability_denied on exactly this condition; this method is the shared,
// independently testable statement of the same rule.
func (a AuthContext) MayActAsSession(actSessionID string) bool {
	if !a.IsSessionBound() {
		return true
	}
	return a.SessionID == actSessionID
}
