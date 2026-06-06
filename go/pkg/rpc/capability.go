package rpc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// AuthContext is the resolved authorization decision for a single RPC request.
//
// RFC 0096 V2 / GH #135: SessionID, when non-empty, identifies the single
// session a capability token is bound to (minted at session.register for one
// lane). An empty SessionID means the token is session-UNBOUND — the
// repo-scoped operator/coordinator token. Session-scoped handlers
// (interrogation.answer and the work.* family) consult IsSessionBound() to
// reject cross-session impersonation by a bound caller, and to record an
// honest operator-override provenance marker when an unbound caller acts as a
// lane.
type AuthContext struct {
	ClientID     string
	TokenID      string
	RepositoryID string
	SessionID    string
	Capability   Capability
	Decision     string
	DenialReason string
}

// IsSessionBound reports whether the resolved token is bound to a specific
// session. A bound token may only act as its own session; an unbound token is
// the shared operator/coordinator token and may act as any session under the
// honest operator-override path.
func (a AuthContext) IsSessionBound() bool {
	return a.SessionID != ""
}

// authContextKey is the unexported context key under which the resolved
// AuthContext is threaded from the RPC/MCP dispatch into mutation handlers, so
// session-scoped handlers can read the caller's bound SessionID without
// changing every handler signature.
type authContextKey struct{}

// WithAuthContext returns a child context carrying the resolved AuthContext.
// The dispatch layer sets this immediately after Authorize succeeds.
func WithAuthContext(ctx context.Context, auth AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

// AuthFromContext returns the AuthContext threaded onto ctx by the dispatch
// layer and whether one was present. When absent (e.g. a direct handler unit
// test, or a transport that does not thread auth), ok is false and the caller
// must treat the request as session-UNBOUND — never as a bound impersonation
// bypass.
func AuthFromContext(ctx context.Context) (AuthContext, bool) {
	auth, ok := ctx.Value(authContextKey{}).(AuthContext)
	return auth, ok
}

// envelopeKey threads the in-flight RPC envelope into mutation handlers so the
// RFC 0110 authority prelude can label the transaction with the originating
// request id and the mutation-coupled audit append (release N+1) can record the
// method/params without a signature change.
type envelopeKey struct{}

// WithEnvelope returns a child context carrying the in-flight RPC envelope. The
// dispatch layer sets this alongside the AuthContext, after Authorize succeeds.
func WithEnvelope(ctx context.Context, envelope Envelope) context.Context {
	return context.WithValue(ctx, envelopeKey{}, envelope)
}

// EnvelopeFromContext returns the RPC envelope threaded onto ctx by the dispatch
// layer and whether one was present (absent in direct handler unit tests).
func EnvelopeFromContext(ctx context.Context) (Envelope, bool) {
	envelope, ok := ctx.Value(envelopeKey{}).(Envelope)
	return envelope, ok
}

// AuditDispatch carries the per-request audit wiring (RFC 0110 §4.4): the env
// the mutation-coupled audit append needs (daemon version, transport) and the
// outcome a self-auditing handler records back, so the dispatch layer can skip
// the standalone append for a row already written inside the mutation tx.
type AuditDispatch struct {
	DaemonVersion string
	Transport     string
	// AuditID/Appended are written by the in-transaction append (the mutation
	// audited itself); the dispatch layer reads them to avoid a double append.
	AuditID  string
	Appended bool
}

type auditDispatchKey struct{}

// WithAuditDispatch returns a child context carrying the audit dispatch record.
// The dispatch layer sets it before routing a mutating RPC and inspects it
// afterwards.
func WithAuditDispatch(ctx context.Context, dispatch *AuditDispatch) context.Context {
	return context.WithValue(ctx, auditDispatchKey{}, dispatch)
}

// AuditDispatchFromContext returns the audit dispatch record threaded onto ctx,
// and whether one was present (absent in direct handler unit tests).
func AuditDispatchFromContext(ctx context.Context) (*AuditDispatch, bool) {
	dispatch, ok := ctx.Value(auditDispatchKey{}).(*AuditDispatch)
	return dispatch, ok
}

type Authorizer interface {
	Authorize(required *Capability, repositoryID string, token string) AuthContext
}

type AllowAllAuthorizer struct{}

// Authorize for AllowAllAuthorizer always allows and yields a session-UNBOUND
// AuthContext (SessionID == ""). RFC 0096 V2: tests and dev wiring that use
// AllowAllAuthorizer therefore exercise the operator-override path (honest
// provenance), never a bound-session bypass — there is no way for AllowAll to
// fabricate a bound SessionID, so it can never silently satisfy the
// cross-session bound check.
func (AllowAllAuthorizer) Authorize(required *Capability, repositoryID string, token string) AuthContext {
	capability := Capability("")
	if required != nil {
		capability = *required
	}
	return AuthContext{RepositoryID: repositoryID, Capability: capability, Decision: "allowed"}
}

type TokenRecord struct {
	TokenID      string
	SecretHash   string
	ClientID     string
	Capabilities map[Capability]CapabilityGrant
	Revoked      bool
	ExpiresAt    time.Time
}

type CapabilityGrant struct {
	RepositoryID string
	// SessionID binds this grant to a single session (RFC 0096 V2 / #135).
	// Empty means the grant is session-unbound (the operator/coordinator
	// repo-scoped grant, as before).
	SessionID string
	ExpiresAt time.Time
	Revoked   bool
}

type MemoryAuthorizer struct {
	mu     sync.RWMutex
	tokens map[string]TokenRecord
}

func NewMemoryAuthorizer() *MemoryAuthorizer {
	return &MemoryAuthorizer{tokens: map[string]TokenRecord{}}
}

func (a *MemoryAuthorizer) AddToken(token string, clientID string, capabilities map[Capability]CapabilityGrant, expiresAt time.Time) {
	tokenID, secret, ok := splitToken(token)
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[tokenID] = TokenRecord{
		TokenID:      tokenID,
		SecretHash:   HashSecret(secret),
		ClientID:     clientID,
		Capabilities: capabilities,
		ExpiresAt:    expiresAt,
	}
}

func (a *MemoryAuthorizer) Authorize(required *Capability, repositoryID string, token string) AuthContext {
	if required == nil {
		return AuthContext{RepositoryID: repositoryID, Decision: "allowed"}
	}
	if _, ok := Capabilities[*required]; !ok {
		return AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "schema_invalid"}
	}
	if token == "" {
		return AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_missing"}
	}
	tokenID, secret, ok := splitToken(token)
	if !ok {
		return AuthContext{RepositoryID: repositoryID, TokenID: tokenID, Decision: "denied", DenialReason: "token_malformed"}
	}
	a.mu.RLock()
	record, exists := a.tokens[tokenID]
	a.mu.RUnlock()
	if !exists {
		return AuthContext{RepositoryID: repositoryID, TokenID: tokenID, Decision: "denied", DenialReason: "token_invalid"}
	}
	ctx := AuthContext{ClientID: record.ClientID, TokenID: tokenID, RepositoryID: repositoryID}
	if subtle.ConstantTimeCompare([]byte(record.SecretHash), []byte(HashSecret(secret))) != 1 {
		ctx.Decision = "denied"
		ctx.DenialReason = "token_invalid"
		return ctx
	}
	if record.Revoked {
		ctx.Decision = "denied"
		ctx.DenialReason = "token_revoked"
		return ctx
	}
	if expired(record.ExpiresAt) {
		ctx.Decision = "denied"
		ctx.DenialReason = "token_expired"
		return ctx
	}
	grant, exists := record.Capabilities[*required]
	if !exists || grant.Revoked {
		ctx.Decision = "denied"
		ctx.DenialReason = "capability_missing"
		return ctx
	}
	if grant.RepositoryID != "" && grant.RepositoryID != repositoryID {
		ctx.Decision = "denied"
		ctx.DenialReason = "capability_scope_mismatch"
		return ctx
	}
	if expired(grant.ExpiresAt) {
		ctx.Decision = "denied"
		ctx.DenialReason = "capability_expired"
		return ctx
	}
	// RFC 0096 V2 / #135: surface the grant's session binding so session-scoped
	// handlers can enforce that a bound token only acts as its own session.
	ctx.SessionID = grant.SessionID
	ctx.Capability = *required
	ctx.Decision = "allowed"
	return ctx
}

// RequireAllowed converts a denied AuthContext into an rpc.Error. The method and
// the capability the route requires are named in the message and details so a
// capability_missing / capability_expired refusal is legible: it tells the
// operator exactly which capability the method needs and how to mint a token
// that carries it (#182, RFC 0111). method and required may be empty for
// callers (e.g. direct handler tests) that lack the route metadata; the message
// then degrades to the generic form.
func RequireAllowed(ctx AuthContext, method string, required *Capability) error {
	if ctx.Decision == "allowed" {
		return nil
	}
	reason := ctx.DenialReason
	if reason == "" {
		reason = "capability_missing"
	}
	requiredCap := ""
	if required != nil {
		requiredCap = string(*required)
	}
	message := "daemon RPC authorization failed"
	var details map[string]any
	if reason == "capability_missing" || reason == "capability_expired" {
		details = map[string]any{}
		if method != "" {
			details["method"] = method
		}
		if requiredCap != "" {
			details["required_capability"] = requiredCap
		}
		lack := "which this token does not carry"
		if reason == "capability_expired" {
			lack = "whose grant on this token has expired"
		}
		switch {
		case method != "" && requiredCap != "":
			message = "daemon RPC authorization failed: method " + method + " requires the " + requiredCap + " capability, " + lack
		case requiredCap != "":
			message = "daemon RPC authorization failed: this method requires the " + requiredCap + " capability, " + lack
		}
		if requiredCap != "" {
			details["remediation"] = "mint a token that grants " + requiredCap + ": `striatum daemon token-create --capability " + requiredCap + "` (admin token required); see docs/how-to/how-to-human.md"
		}
	}
	return NewError(reason, message, details)
}

func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func splitToken(token string) (string, string, bool) {
	tokenID, secret, ok := strings.Cut(token, ".")
	if !ok || tokenID == "" || secret == "" {
		return tokenID, secret, false
	}
	return tokenID, secret, true
}

func expired(value time.Time) bool {
	return !value.IsZero() && !value.After(time.Now().UTC())
}
