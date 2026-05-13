package rpc

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type AuthContext struct {
	ClientID     string
	TokenID      string
	RepositoryID string
	Capability   Capability
	Decision     string
	DenialReason string
}

type Authorizer interface {
	Authorize(required *Capability, repositoryID string, token string) AuthContext
}

type AllowAllAuthorizer struct{}

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
	ExpiresAt    time.Time
	Revoked      bool
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
	ctx.Capability = *required
	ctx.Decision = "allowed"
	return ctx
}

func RequireAllowed(ctx AuthContext) error {
	if ctx.Decision == "allowed" {
		return nil
	}
	reason := ctx.DenialReason
	if reason == "" {
		reason = "capability_missing"
	}
	return NewError(reason, "daemon RPC authorization failed", nil)
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
