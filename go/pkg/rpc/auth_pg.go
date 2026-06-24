package rpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// AuthQuerier is the minimal database surface required by PostgresAuthorizer.
// It is satisfied structurally by db.Runner; we declare a local interface so
// the rpc package does not have to import db (which would form an import
// cycle through audit.go).
type AuthQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
}

type boundAuthQueryRower interface {
	QueryRowBound(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresAuthorizer authorizes Go transition/runtime RPC requests by
// validating Python-issued capability tokens against the production daemon's
// PostgreSQL substrate. Denial reasons match
// src/striatum/daemon_rpc/capability.py for compatibility with existing
// contract and harness tests.
type PostgresAuthorizer struct {
	Runner          AuthQuerier
	Clock           func() time.Time
	AuthoritySecret string
}

func (a *PostgresAuthorizer) now() time.Time {
	if a.Clock != nil {
		return a.Clock().UTC()
	}
	return time.Now().UTC()
}

func (a *PostgresAuthorizer) Authorize(required *Capability, repositoryID string, token string) AuthContext {
	ctx := context.Background()
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

	if projected, ok := a.authorizeWithProjection(ctx, *required, repositoryID, tokenID, secret); ok {
		return projected
	}

	var clientID, tokenHash, tokenSalt string
	var revokedAt, expiresAt pgtype.Timestamptz
	scanErr := a.Runner.QueryRow(
		ctx,
		`SELECT client_id, token_hash, token_salt, revoked_at, expires_at
		   FROM striatumd.clients WHERE token_id = $1`,
		tokenID,
	).Scan(&clientID, &tokenHash, &tokenSalt, &revokedAt, &expiresAt)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return AuthContext{RepositoryID: repositoryID, TokenID: tokenID, Decision: "denied", DenialReason: "token_invalid"}
		}
		return AuthContext{RepositoryID: repositoryID, TokenID: tokenID, Decision: "denied", DenialReason: "internal_error"}
	}
	expected := hmacHexSecret(tokenSalt, secret)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(tokenHash)) != 1 {
		return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_invalid"}
	}
	if revokedAt.Valid {
		return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_revoked"}
	}
	if expiresAt.Valid && !expiresAt.Time.UTC().After(a.now()) {
		return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_expired"}
	}

	var capabilityID string
	var capRepoID pgtype.Text
	var capExpiresAt pgtype.Timestamptz
	var capRevokedAt pgtype.Timestamptz
	var capSessionID pgtype.Text
	// RFC 0096 V2 / #135: also read session_id so the resolved AuthContext can
	// carry the grant's session binding (NULL = session-unbound, the
	// operator/coordinator grant). Prefer the narrowest grant: a
	// session-scoped, repo-scoped grant over a broader one, so a lane minted a
	// bound token enforces against its own session.
	capErr := a.Runner.QueryRow(
		ctx,
		`SELECT capability_id, repository_id, expires_at, revoked_at, session_id
		   FROM striatumd.client_capabilities
		  WHERE client_id = $1
		    AND capability = $2
		    AND (repository_id IS NULL OR repository_id = $3)
		    AND revoked_at IS NULL
		  ORDER BY session_id IS NULL, repository_id IS NULL
		  LIMIT 1`,
		clientID,
		string(*required),
		repositoryID,
	).Scan(&capabilityID, &capRepoID, &capExpiresAt, &capRevokedAt, &capSessionID)
	if capErr != nil {
		if !errors.Is(capErr, pgx.ErrNoRows) {
			return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "internal_error"}
		}
		if repositoryID != "" {
			scope, scopeErr := a.Runner.QueryScalar(
				ctx,
				`SELECT '1' FROM striatumd.client_capabilities
				  WHERE client_id = $1
				    AND capability = $2
				    AND repository_id IS NOT NULL
				    AND repository_id <> $3
				    AND revoked_at IS NULL
				  LIMIT 1`,
				clientID,
				string(*required),
				repositoryID,
			)
			if scopeErr == nil && scope != "" {
				return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "capability_scope_mismatch"}
			}
		}
		return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "capability_missing"}
	}
	if capExpiresAt.Valid && !capExpiresAt.Time.UTC().After(a.now()) {
		return AuthContext{ClientID: clientID, TokenID: tokenID, RepositoryID: repositoryID, Decision: "denied", DenialReason: "capability_expired"}
	}
	boundSession := ""
	if capSessionID.Valid {
		boundSession = capSessionID.String
	}
	return AuthContext{
		ClientID:     clientID,
		TokenID:      tokenID,
		RepositoryID: repositoryID,
		SessionID:    boundSession,
		Capability:   *required,
		Decision:     "allowed",
	}
}

// IdentifyBoundSession verifies the presented bearer is a genuine daemon-minted,
// non-revoked, non-expired token and returns the (repositoryID, sessionID) bound
// into its session-scoped capability grant. It authorizes NOTHING — no capability
// is checked or granted; it is a pure IDENTITY read of the daemon's own token store
// (the same striatumd.clients + client_capabilities rows Authorize reads). It is the
// RFC 0143 Slice A pre-auth attribution (#512): the boot-epoch rejection branch
// consults it to learn WHICH session a rejected lane belongs to so the daemon can
// record a durable, attributed stale-epoch observation. The request stays rejected;
// this widens no token and reads no admin runtime client-token.
//
// ok=false (and nothing is recorded — no over-fire) when the bearer is absent,
// malformed, fails the HMAC, is revoked/expired, or carries NO session binding (a
// session-unbound operator/coordinator grant). Forge-resistance: the session binding
// is an HMAC over a daemon-held salt at mint time, so an UNPRIVILEGED party cannot
// fabricate a token bound to a session it does not own. The shared-uid residual (a
// same-uid sibling presenting a victim lane's stolen token) is RFC-0168-bounded
// (#585) and observability-only — it can mislabel a recovery REASON, never grant a
// capability or a seal.
func (a *PostgresAuthorizer) IdentifyBoundSession(token string) (repositoryID, sessionID string, ok bool) {
	if a == nil || a.Runner == nil || strings.TrimSpace(token) == "" {
		return "", "", false
	}
	ctx := context.Background()
	tokenID, secret, split := splitToken(token)
	if !split {
		return "", "", false
	}

	var clientID, tokenHash, tokenSalt string
	var revokedAt, expiresAt pgtype.Timestamptz
	scanErr := a.Runner.QueryRow(
		ctx,
		`SELECT client_id, token_hash, token_salt, revoked_at, expires_at
		   FROM striatumd.clients WHERE token_id = $1`,
		tokenID,
	).Scan(&clientID, &tokenHash, &tokenSalt, &revokedAt, &expiresAt)
	if scanErr != nil {
		return "", "", false
	}
	expected := hmacHexSecret(tokenSalt, secret)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(tokenHash)) != 1 {
		return "", "", false
	}
	if revokedAt.Valid {
		return "", "", false
	}
	if expiresAt.Valid && !expiresAt.Time.UTC().After(a.now()) {
		return "", "", false
	}

	// Read the NARROWEST session-scoped grant for this client: a row that carries a
	// non-NULL session_id is the bound-lane grant. A grant with no session binding
	// (operator/coordinator) is NOT attributable to a lane, so ok=false.
	var capRepoID, capSessionID pgtype.Text
	var capRevokedAt, capExpiresAt pgtype.Timestamptz
	capErr := a.Runner.QueryRow(
		ctx,
		`SELECT repository_id, session_id, revoked_at, expires_at
		   FROM striatumd.client_capabilities
		  WHERE client_id = $1
		    AND session_id IS NOT NULL
		    AND revoked_at IS NULL
		  ORDER BY repository_id IS NULL, granted_at DESC
		  LIMIT 1`,
		clientID,
	).Scan(&capRepoID, &capSessionID, &capRevokedAt, &capExpiresAt)
	if capErr != nil {
		return "", "", false
	}
	if capExpiresAt.Valid && !capExpiresAt.Time.UTC().After(a.now()) {
		return "", "", false
	}
	if !capSessionID.Valid || strings.TrimSpace(capSessionID.String) == "" {
		return "", "", false
	}
	return nullableText(capRepoID), capSessionID.String, true
}

func (a *PostgresAuthorizer) authorizeWithProjection(ctx context.Context, required Capability, repositoryID, tokenID, secret string) (AuthContext, bool) {
	if a.AuthoritySecret == "" {
		return AuthContext{}, false
	}
	bound, ok := a.Runner.(boundAuthQueryRower)
	if !ok {
		return AuthContext{}, false
	}
	var clientID, sessionID, denialReason pgtype.Text
	var resolvedTokenID, resolvedRepositoryID, resolvedCapability, decision string
	err := bound.QueryRowBound(ctx, `
		SELECT client_id, token_id, repository_id, session_id,
		       capability, decision, denial_reason
		  FROM striatumd.authorize_capability($1, $2, $3, $4, $5, $6)`,
		a.AuthoritySecret,
		string(required),
		repositoryID,
		tokenID,
		secret,
		a.now(),
	).Scan(
		&clientID,
		&resolvedTokenID,
		&resolvedRepositoryID,
		&sessionID,
		&resolvedCapability,
		&decision,
		&denialReason,
	)
	if err != nil {
		if pgErrCode(err) == "42883" {
			return AuthContext{}, false
		}
		return AuthContext{RepositoryID: repositoryID, TokenID: tokenID, Decision: "denied", DenialReason: "internal_error"}, true
	}
	result := AuthContext{
		ClientID:     nullableText(clientID),
		TokenID:      resolvedTokenID,
		RepositoryID: resolvedRepositoryID,
		SessionID:    nullableText(sessionID),
		Decision:     decision,
		DenialReason: nullableText(denialReason),
	}
	if decision == "allowed" {
		result.Capability = Capability(resolvedCapability)
	}
	return result, true
}

func nullableText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// hmacHexSecret reproduces src/striatum/daemon.py::_hash_token so that
// Python-issued tokens validate against Go without re-issuing.
func hmacHexSecret(salt, secret string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}
