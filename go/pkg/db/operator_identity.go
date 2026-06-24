package db

import (
	"crypto/sha256"
	"encoding/hex"
)

// RFC 0167 P0 (D260): the shared operator-identity renderer. It lives in pkg/db
// so every surface (whose, status --mine, the bootstrap mint result) renders the
// identical handle#suffix string — the RFC's "same chip everywhere, forever"
// stability property — with no cross-package duplication or drift.

// OperatorHandleSuffix is the short, stable, COMPUTED (not stored) disambiguator
// rendered as handle#suffix (e.g. maya#7f3): a fixed-length slice of a hash of the
// principal_id. The same principal always yields the same suffix, so two terminals
// of the same human share the suffix (signalling "same human") while their handles
// differ.
func OperatorHandleSuffix(principalID string) string {
	sum := sha256.Sum256([]byte(principalID))
	return hex.EncodeToString(sum[:])[:3]
}

// RenderOperatorIdentity renders the glanceable identity for a principal and its
// (optional) handle snapshot: handle#suffix when a handle is known, else the bare
// principal id (the friendly name lapses to the immutable id — free, not a special
// case, e.g. when the token is revoked/expired and no live handle resolves).
func RenderOperatorIdentity(principalID, handle string) string {
	if principalID == "" {
		return "unknown"
	}
	if handle == "" {
		return principalID
	}
	return handle + "#" + OperatorHandleSuffix(principalID)
}
