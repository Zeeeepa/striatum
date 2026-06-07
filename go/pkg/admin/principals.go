package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// principalQuerier is the minimal database surface the principal model needs.
// Both db.Runner and db.TxRunner satisfy it structurally, so principal reads
// and links can run either standalone or inside the existing token-create /
// token-rotate transaction (linking the minted client to a principal is part
// of the SAME atomic transaction that issues the token).
type principalQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
}

// boundQueryRower is the optional extended-protocol single-row query surface
// (C-EXTENDED-AUTH-PRELUDE). Both db.PgxRunner and *db.PgxTxRunner implement
// it; canned test fakes do not, and take the direct-SQL path below.
type boundQueryRower interface {
	QueryRowBound(ctx context.Context, sql string, args ...any) pgx.Row
}

// queryIdentityRow reads one row from an RFC 0114 gated identity surface
// (principals / principal_clients) through the dual path that combines the two
// landed precedents (PostgresAuthorizer.authorizeWithProjection's 42883
// fallback, loadTokenForUpdate's secret gate):
//
//   - With a bootstrapped authority secret and a bound-capable querier, call
//     the owner-owned SECURITY DEFINER projection via QueryRowBound — extended
//     protocol, so the secret travels in Bind messages and never appears in
//     pg_stat_activity query text — passing the secret as the first argument.
//   - On SQLSTATE 42883 (function absent: authority bootstrapped but owner
//     bundle 0006 not yet applied) fall back to today's direct SQL, preserving
//     pre-bundle behavior.
//   - Secretless daemons (un-adopted database, direct handler unit tests) run
//     the direct SQL permanently; nothing changes there.
//
// Any other error surfaces: after bundle 0006 a direct read by a secretless
// daemon fails 42501, which is a real misconfiguration and must be visible,
// not masked — the same posture loadTokenForUpdate takes.
func queryIdentityRow(ctx context.Context, q principalQuerier, projectionSQL, directSQL string, scan func(row db.Row) error, args ...any) error {
	secret := db.AuthorityFromContext(ctx).Secret
	if bound, ok := q.(boundQueryRower); ok && secret != "" {
		boundArgs := append([]any{secret}, args...)
		err := scan(bound.QueryRowBound(ctx, projectionSQL, boundArgs...))
		if err == nil || principalPgErrCode(err) != "42883" {
			return err
		}
	}
	return scan(q.QueryRow(ctx, directSQL, args...))
}

// principalExists reports whether a principal row exists, through the
// get_principal projection when daemon authority is available.
func principalExists(ctx context.Context, q principalQuerier, principalID string) (bool, error) {
	var one string
	err := queryIdentityRow(ctx, q,
		`SELECT '1' FROM striatumd.get_principal($1, $2)`,
		`SELECT '1' FROM striatumd.principals WHERE principal_id = $1`,
		func(row db.Row) error { return row.Scan(&one) },
		principalID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func principalPgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// principalKinds enumerates the RFC 0107 principal kinds. `human` generalizes
// RFC 0053's single escalation-only human principal to several humans;
// `ai_operator` is an autonomous operator identity; `service` covers
// non-interactive daemon/automation clients.
var principalKinds = map[string]struct{}{
	"human":       {},
	"ai_operator": {},
	"service":     {},
}

// PrincipalRef is the resolved identity a client (and therefore an audit row's
// client_id) attributes to.
type PrincipalRef struct {
	PrincipalID string `json:"principal_id"`
	Kind        string `json:"kind"`
	DisplayName string `json:"display_name"`
	Disabled    bool   `json:"disabled"`
}

// PrincipalCapability is one capability grant in a principal's effective scope,
// aggregated across every client the principal owns.
type PrincipalCapability struct {
	Capability   rpc.Capability `json:"capability"`
	RepositoryID *string        `json:"repository_id"`
	SessionBound bool           `json:"session_bound"`
}

// PrincipalScope is a principal plus its effective capability/repository scope,
// used by `daemon doctor` so the operator can see exactly which principals are
// configured and what each may do, on which repositories.
type PrincipalScope struct {
	PrincipalID  string                `json:"principal_id"`
	Kind         string                `json:"kind"`
	DisplayName  string                `json:"display_name"`
	Disabled     bool                  `json:"disabled"`
	ClientCount  int                   `json:"client_count"`
	Repositories []string              `json:"repositories"`
	Capabilities []PrincipalCapability `json:"capabilities"`
}

// validatePrincipalKind returns the kind unchanged when valid, else a
// schema_invalid error.
func validatePrincipalKind(kind string) (string, error) {
	if _, ok := principalKinds[kind]; !ok {
		return "", rpc.NewError("schema_invalid", fmt.Sprintf("unsupported principal kind %q (want human|ai_operator|service)", kind), nil)
	}
	return kind, nil
}

// CreatePrincipal inserts a principal. It is idempotent on (principal_id) only
// when the existing row's kind/display_name match; a conflicting redefinition
// is refused so a principal's identity cannot be silently rewritten.
func CreatePrincipal(ctx context.Context, q principalQuerier, principalID, kind, displayName string) error {
	if principalID == "" {
		return rpc.NewError("schema_invalid", "principal_id must be non-empty", nil)
	}
	if displayName == "" {
		return rpc.NewError("schema_invalid", "principal display_name must be non-empty", nil)
	}
	if _, err := validatePrincipalKind(kind); err != nil {
		return err
	}
	now := time.Now().UTC()
	var existingKind, existingName string
	err := queryIdentityRow(ctx, q,
		`SELECT principal_kind, display_name FROM striatumd.get_principal($1, $2)`,
		`SELECT principal_kind, display_name FROM striatumd.principals WHERE principal_id = $1`,
		func(row db.Row) error { return row.Scan(&existingKind, &existingName) },
		principalID,
	)
	switch {
	case err == nil:
		if existingKind != kind || existingName != displayName {
			return rpc.NewError("conflict", fmt.Sprintf("principal %q already exists with a different kind/display_name", principalID), nil)
		}
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		// fall through to insert
	default:
		return err
	}
	return q.Exec(ctx,
		`INSERT INTO striatumd.principals(principal_id, principal_kind, display_name, created_at)
		 VALUES ($1, $2, $3, $4)`,
		principalID, kind, displayName, now,
	)
}

// LinkClientToPrincipal active-links a client to a principal. A client may be
// actively linked to at most one principal (enforced by the partial unique
// index); attempting to link an already actively-linked client to a DIFFERENT
// principal is refused so audit attribution stays unambiguous. Re-linking the
// same (principal, client) pair is idempotent.
func LinkClientToPrincipal(ctx context.Context, q principalQuerier, principalID, clientID string) error {
	if principalID == "" || clientID == "" {
		return rpc.NewError("schema_invalid", "principal_id and client_id must be non-empty", nil)
	}
	exists, err := principalExists(ctx, q, principalID)
	if err != nil {
		return err
	}
	if !exists {
		return rpc.NewError("not_found", fmt.Sprintf("unknown principal %q", principalID), nil)
	}
	// Already actively linked? Same principal => idempotent; different => refuse.
	// principal_id is the bundle-0006 gated column, so the active-link check
	// reads through the resolve_principal_for_client projection; the direct
	// fallback is the pre-bundle query. The projection's join to principals
	// cannot change the result: principal_clients.principal_id carries a real
	// FK to principals, so an active link always joins.
	var current string
	err = queryIdentityRow(ctx, q,
		`SELECT principal_id FROM striatumd.resolve_principal_for_client($1, $2)`,
		`SELECT principal_id FROM striatumd.principal_clients
		  WHERE client_id = $1 AND unlinked_at IS NULL`,
		func(row db.Row) error { return row.Scan(&current) },
		clientID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		current = ""
	} else if err != nil {
		return err
	}
	if current != "" {
		if current == principalID {
			return nil
		}
		return rpc.NewError("conflict", fmt.Sprintf("client %q is already attributed to principal %q", clientID, current), nil)
	}
	// Insert (revive a prior unlinked row for the same pair if present).
	if err := q.Exec(ctx,
		`INSERT INTO striatumd.principal_clients(principal_id, client_id, linked_at, unlinked_at)
		 VALUES ($1, $2, now(), NULL)
		 ON CONFLICT (principal_id, client_id)
		 DO UPDATE SET unlinked_at = NULL, linked_at = now()`,
		principalID, clientID,
	); err != nil {
		return err
	}
	return nil
}

// ResolvePrincipalForClient maps a client_id to the principal it is actively
// attributed to. ok is false when the client has no active principal link
// (back-compat: pre-RFC-0107 clients and the bootstrap admin remain
// unattributed rather than mis-attributed). This is the join the daemon-global
// audit chain (audit_log.client_id) uses to attribute a mutation to a principal.
func ResolvePrincipalForClient(ctx context.Context, q principalQuerier, clientID string) (PrincipalRef, bool, error) {
	if clientID == "" {
		return PrincipalRef{}, false, nil
	}
	var ref PrincipalRef
	var disabledAt pgtype.Timestamptz
	err := queryIdentityRow(ctx, q,
		`SELECT principal_id, principal_kind, display_name, disabled_at
		   FROM striatumd.resolve_principal_for_client($1, $2)`,
		`SELECT p.principal_id, p.principal_kind, p.display_name, p.disabled_at
		   FROM striatumd.principal_clients pc
		   JOIN striatumd.principals p ON p.principal_id = pc.principal_id
		  WHERE pc.client_id = $1 AND pc.unlinked_at IS NULL`,
		func(row db.Row) error {
			return row.Scan(&ref.PrincipalID, &ref.Kind, &ref.DisplayName, &disabledAt)
		},
		clientID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PrincipalRef{}, false, nil
		}
		return PrincipalRef{}, false, err
	}
	ref.Disabled = disabledAt.Valid
	return ref, true, nil
}

// ListPrincipals returns every configured principal with its effective
// capability/repository scope, aggregated across the active (non-revoked,
// unexpired) capability grants of every client the principal actively owns.
// The query is repo-scoped per principal — a principal's scope NEVER includes a
// grant on a repository it was not granted, which is the cross-principal +
// cross-repo isolation the operator must be able to inspect.
func ListPrincipals(ctx context.Context, q principalQuerier) ([]PrincipalScope, error) {
	// The projection body is a verbatim transplant of the direct aggregate, so
	// the JSON payload — and therefore the PrincipalScope decode — is
	// bit-identical on both paths.
	var payload string
	err := queryIdentityRow(ctx, q,
		`SELECT striatumd.list_principal_scopes($1)`, `
		WITH grants AS (
		  SELECT pc.principal_id,
		         cc.capability,
		         cc.repository_id,
		         (cc.session_id IS NOT NULL) AS session_bound
		    FROM striatumd.principal_clients pc
		    JOIN striatumd.client_capabilities cc ON cc.client_id = pc.client_id
		   WHERE pc.unlinked_at IS NULL
		     AND cc.revoked_at IS NULL
		     AND (cc.expires_at IS NULL OR cc.expires_at > now())
		),
		client_counts AS (
		  SELECT principal_id, COUNT(*) AS client_count
		    FROM striatumd.principal_clients
		   WHERE unlinked_at IS NULL
		   GROUP BY principal_id
		)
		SELECT COALESCE(
		  jsonb_agg(
		    jsonb_build_object(
		      'principal_id', p.principal_id,
		      'kind', p.principal_kind,
		      'display_name', p.display_name,
		      'disabled', (p.disabled_at IS NOT NULL),
		      'client_count', COALESCE(cc.client_count, 0),
		      'repositories', COALESCE((
		        SELECT jsonb_agg(DISTINCT g.repository_id ORDER BY g.repository_id)
		          FROM grants g
		         WHERE g.principal_id = p.principal_id AND g.repository_id IS NOT NULL
		      ), '[]'::jsonb),
		      'capabilities', COALESCE((
		        SELECT jsonb_agg(
		                 jsonb_build_object(
		                   'capability', g.capability,
		                   'repository_id', g.repository_id,
		                   'session_bound', g.session_bound
		                 )
		                 ORDER BY g.capability, g.repository_id NULLS FIRST
		               )
		          FROM (
		            SELECT capability, repository_id, bool_or(session_bound) AS session_bound
		              FROM grants
		             WHERE principal_id = p.principal_id
		             GROUP BY capability, repository_id
		          ) g
		      ), '[]'::jsonb)
		    )
		    ORDER BY p.principal_id
		  ),
		  '[]'::jsonb
		)::text
		  FROM striatumd.principals p
		  LEFT JOIN client_counts cc ON cc.principal_id = p.principal_id`,
		func(row db.Row) error { return row.Scan(&payload) },
	)
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return []PrincipalScope{}, nil
	}
	var scopes []PrincipalScope
	if err := json.Unmarshal([]byte(payload), &scopes); err != nil {
		return nil, fmt.Errorf("decode principal scopes: %w", err)
	}
	return scopes, nil
}

// principalParamsFromEnvelope extracts the optional principal attribution
// parameters from a daemon.token.create / daemon.token.rotate envelope. An
// empty principalID means "do not attribute" (back-compat). When principalID is
// set but principalKind/principalDisplayName are empty, the principal must
// already exist (we refuse to fabricate an unkinded principal).
func principalParamsFromEnvelope(params map[string]any) (principalID, principalKind, principalDisplayName string) {
	principalID = stringParam(params, "principal_id")
	principalKind = stringParam(params, "principal_kind")
	principalDisplayName = stringParam(params, "principal_display_name")
	return principalID, principalKind, principalDisplayName
}

// attributeClientToPrincipal, inside the token transaction, ensures the
// principal exists (creating it when kind+display_name are supplied) and
// active-links the freshly minted client to it.
func attributeClientToPrincipal(ctx context.Context, q principalQuerier, params map[string]any, clientID string) error {
	principalID, principalKind, principalDisplayName := principalParamsFromEnvelope(params)
	if principalID == "" {
		return nil
	}
	exists, err := principalExists(ctx, q, principalID)
	if err != nil {
		return err
	}
	switch {
	case !exists:
		if principalKind == "" || principalDisplayName == "" {
			return rpc.NewError("not_found", fmt.Sprintf("unknown principal %q; supply principal_kind and principal_display_name to create it", principalID), nil)
		}
		if err := CreatePrincipal(ctx, q, principalID, principalKind, principalDisplayName); err != nil {
			return err
		}
	case principalKind != "" && principalDisplayName != "":
		// Both supplied for an existing principal: route through CreatePrincipal,
		// which is idempotent when they match and refuses a conflicting
		// redefinition (no silent rewrite of a principal's identity).
		if err := CreatePrincipal(ctx, q, principalID, principalKind, principalDisplayName); err != nil {
			return err
		}
	case principalKind != "":
		// Only the kind was supplied: validate it but do not rewrite the row.
		if _, err := validatePrincipalKind(principalKind); err != nil {
			return err
		}
	}
	return LinkClientToPrincipal(ctx, q, principalID, clientID)
}
