package mutations

import (
	"context"
	"errors"
	"hash/fnv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0167 P0 (D260) — operator identity & run attribution: the lease +
// operator-session Go layer and the operator-bootstrap mint+lease RPC. The
// owner-bundle 0022 substrate (operator_handles / operator_sessions / the runs
// origin columns + DEFINER projections + composed-route read closure) is in
// go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql.

// operatorSessionCapabilities is the operator-session token's capability set:
// admin (a real operator drives run.prepare + repo-admin verbs — checkpoint
// resolve / review override / branch confirm) and read. It is DISTINCT from the
// lane slice sessionBoundCapabilities (session_token.go), which stays UNCHANGED so
// a lane token can never gain admin (C1' / A30 / A32 / A43). The justified
// acceptance of this admin surface and its repo-scope/trust-root bounds are §C1";
// the trust-root route (verifier.attest) is already fenced for any session-bound
// token (verifier_attest.go).
var operatorSessionCapabilities = []rpc.Capability{rpc.CapabilityAdmin, rpc.CapabilityRead}

// operatorHandlePool is the curated, privacy-safe, lowercase first-name pool an
// operator handle is drawn from (OQ1). Memorable + neutral is the only hard
// constraint; the exact membership is not load-bearing, the deterministic,
// reconnect-stable default + escalation walk over it is. Reserved/role words
// (operatorReservedHandles) are excluded from the pool by construction.
var operatorHandlePool = []string{
	"maya", "theo", "iris", "milo", "nova", "cleo", "ezra", "luna", "kai", "remy",
	"juno", "otis", "vera", "arlo", "fern", "ivy", "leon", "mira", "nico", "opal",
	"rhea", "silas", "tess", "uma", "wren", "yael", "zane", "ada", "bram", "cora",
	"dane", "elsa", "finn", "gwen", "hugo", "ines", "jude", "kira", "lars", "moss",
	"nell", "oren", "pax", "quinn", "rosa", "sven", "thea", "ulla", "vlad", "wade",
	"xena", "yara", "zev", "asa", "bea", "cole", "dora", "eli", "faye", "gus",
	"hana", "ian", "june", "knox", "lia", "max", "noor", "ona", "pia", "ravi",
	"sage", "tom", "ula", "vin", "wyn", "xan", "yuki", "zola", "ari", "bo",
	"cy", "del", "eve", "flo", "gil", "hal", "ida", "jax", "kit", "lou",
	"mae", "ned", "ola", "pip", "ren", "sol", "tia", "uri", "val", "win",
}

// operatorReservedHandles are words a handle must never take: they would read as
// authority or as the bare-id fallback rather than a person. Service principals
// draw a disjoint sub-pool (operatorServiceHandlePool) so a human and a service
// never collide on a word that implies the wrong kind.
var operatorReservedHandles = map[string]struct{}{
	"daemon": {}, "scheduler": {}, "system": {}, "admin": {}, "root": {},
	"unknown": {}, "anon": {}, "none": {}, "human": {}, "service": {},
	"ai_operator": {}, "operator": {},
}

// operatorServiceHandlePool is the disjoint sub-pool service principals draw from,
// so an auto-spawned scheduler run renders e.g. "scout#a19" — clearly a service —
// next to a human's "maya#7f3" without ever colliding on the same word.
var operatorServiceHandlePool = []string{
	"scout", "relay", "warden", "pilot", "beacon", "sentry", "drone", "harbor",
	"forge", "anchor", "ember", "comet", "vector", "atlas", "probe", "signal",
}

// operatorHandleTTL bounds a handle lease and the operator session/token; the
// heartbeat renews it (§3, never release-then-reacquire). Matches the lane
// session-bound token TTL so an operator token never outlives the session by much.
const operatorHandleTTL = sessionBoundTokenTTL

// handlePoolFor returns the pool a principal of the given kind draws from.
func handlePoolFor(principalKind string) []string {
	if principalKind == "human" || principalKind == "" {
		return operatorHandlePool
	}
	return operatorServiceHandlePool
}

// operatorHandleCandidates returns the deterministic, principal-seeded candidate
// walk: candidates[0] is the default (a hash of principal_id mod len, so the same
// human reattaches to the same word per repo across reconnects — NOT tty/pane),
// and candidates[1..] are the escalation order on a live collision. Reserved
// words are already absent from the pool, so every candidate is usable.
func operatorHandleCandidates(principalID, principalKind string) []string {
	pool := handlePoolFor(principalKind)
	if len(pool) == 0 {
		return nil
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(principalID))
	start := int(h.Sum64() % uint64(len(pool)))
	out := make([]string, 0, len(pool))
	for i := 0; i < len(pool); i++ {
		word := pool[(start+i)%len(pool)]
		if _, reserved := operatorReservedHandles[strings.ToLower(word)]; reserved {
			continue
		}
		out = append(out, word)
	}
	return out
}

// mintOperatorSessionToken inserts a client + per-capability grants bound to
// operatorSessionID and the repository, returning the bearer token. It is the
// sibling of mintSessionBoundToken (session_token.go): same atomic-in-the-mint-tx
// shape and HMAC scheme so the production authorizer validates it with no new
// path, but it grants operatorSessionCapabilities ({admin, read}) rather than the
// lane slice. The shared sessionBoundCapabilities slice is NOT touched.
func mintOperatorSessionToken(ctx context.Context, tx db.TxRunner, repositoryID, operatorSessionID string) (map[string]any, error) {
	tokenID := "otok_" + randomURLSafe(12)
	secret := randomURLSafe(32)
	salt := randomHexToken(16)
	clientID := "oclient_" + randomHexToken(16)
	now := time.Now().UTC()
	expiresAt := now.Add(operatorHandleTTL)
	tokenHash := hmacHexToken(salt, secret)

	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name,
		  token_id, token_hash, token_salt, created_at, expires_at)
		VALUES ($1, 'operator-session', $2, $3, $4, $5, $6, $7)`,
		clientID, "operator-session "+operatorSessionID, tokenID, tokenHash, salt, now, expiresAt,
	); err != nil {
		return nil, err
	}
	for _, capability := range operatorSessionCapabilities {
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.client_capabilities(
			  capability_id, client_id, repository_id, capability, granted_at,
			  expires_at, session_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			"ocap_"+randomHexToken(16), clientID, repositoryID, string(capability),
			now, expiresAt, operatorSessionID,
		); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"token":      tokenID + "." + secret,
		"token_id":   tokenID,
		"client_id":  clientID,
		"session_id": operatorSessionID,
		"expires_at": expiresAt.Format(time.RFC3339Nano),
	}, nil
}

// acquireOperatorHandle walks the principal-seeded candidates and leases the first
// word not currently live on the repo, returning (handle_id, handle). It uses
// INSERT ... ON CONFLICT DO NOTHING against the live-unique partial index so a
// collision is a no-row return (try the next candidate) rather than a 23505 that
// poisons the transaction — and only the owning live session can ever hold a word
// (the lease never deletes; lazy expiry only sets released_at).
func acquireOperatorHandle(ctx context.Context, tx db.TxRunner, repositoryID, principalID, principalKind, operatorSessionID string, now, leasedUntil time.Time) (string, string, error) {
	candidates := operatorHandleCandidates(principalID, principalKind)
	for _, handle := range candidates {
		handleID, err := newID("ohandle")
		if err != nil {
			return "", "", err
		}
		var got string
		err = tx.QueryRow(ctx, `
			INSERT INTO striatumd.operator_handles(
			  handle_id, repository_id, principal_id, handle, leased_session_id,
			  leased_until, released_at, last_heartbeat_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, NULL, $7)
			ON CONFLICT (repository_id, lower(handle)) WHERE released_at IS NULL DO NOTHING
			RETURNING handle_id`,
			handleID, repositoryID, principalID, handle, operatorSessionID, leasedUntil, now,
		).Scan(&got)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // the word is live; escalate to the next candidate
		}
		if err != nil {
			return "", "", err
		}
		return got, handle, nil
	}
	return "", "", rpc.NewError("workflow_error", "operator handle pool exhausted: every candidate word is live on this repository", nil)
}

// HandleOperatorBootstrap mints a session-bound operator token and leases a
// handle for the calling human, all in one transaction (§1(1)): resolve/create
// the principal (kind=human), mint the operator token bound to a fresh
// operator_session_id, link the minted client to the principal (so the run-origin
// stamp + status --mine resolve), INSERT the operator_sessions row, and acquire
// the handle lease. The presented token is the session-bound operator token —
// per §F F-2 the static bootstrap token is NOT injected for routine repo-admin.
func HandleOperatorBootstrap(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	displayName := stringParam(envelope, "display_name")
	if displayName == "" {
		displayName = "operator"
	}
	callerClientID := db.AuthorityFromContext(ctx).PrincipalID

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// (a) resolve/create the caller's principal (kind=human).
		principalID := ""
		principalKind := "human"
		if callerClientID != "" {
			ref, found, rerr := admin.ResolvePrincipalForClient(ctx, tx, callerClientID)
			if rerr != nil {
				return nil, rerr
			}
			if found {
				principalID = ref.PrincipalID
				if ref.Kind != "" {
					principalKind = ref.Kind
				}
			}
		}
		if principalID == "" {
			principalID, err = newID("prin")
			if err != nil {
				return nil, err
			}
			if cerr := admin.CreatePrincipal(ctx, tx, principalID, "human", displayName); cerr != nil {
				return nil, cerr
			}
			if callerClientID != "" {
				if lerr := admin.LinkClientToPrincipal(ctx, tx, principalID, callerClientID); lerr != nil {
					return nil, lerr
				}
			}
		}

		// (b) mint the session-bound operator token (fresh operator_session_id).
		operatorSessionID, err := newID("osess")
		if err != nil {
			return nil, err
		}
		minted, err := mintOperatorSessionToken(ctx, tx, repositoryID, operatorSessionID)
		if err != nil {
			return nil, err
		}
		operatorClientID, _ := minted["client_id"].(string)

		// (c) link the minted operator client to the principal so its runs resolve
		// through resolve_principal_for_client / runs_for_origin_client.
		if operatorClientID != "" {
			if lerr := admin.LinkClientToPrincipal(ctx, tx, principalID, operatorClientID); lerr != nil {
				return nil, lerr
			}
		}

		// (d) INSERT the operator_sessions row + (e) acquire the handle lease.
		now := time.Now().UTC()
		expiresAt := now.Add(operatorHandleTTL)
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.operator_sessions(
			  repository_id, operator_session_id, principal_id, client_id, state,
			  registered_at, last_heartbeat_at, expires_at
			)
			VALUES ($1, $2, $3, $4, 'active', $5, $5, $6)`,
			repositoryID, operatorSessionID, principalID, operatorClientID, now, expiresAt,
		); err != nil {
			return nil, err
		}
		handleID, handle, err := acquireOperatorHandle(ctx, tx, repositoryID, principalID, principalKind, operatorSessionID, now, expiresAt)
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"operator_session_id": operatorSessionID,
			"token":               minted["token"],
			"token_id":            minted["token_id"],
			"client_id":           operatorClientID,
			"handle":              handle,
			"handle_id":           handleID,
			"identity":            db.RenderOperatorIdentity(principalID, handle),
			"expires_at":          expiresAt.Format(time.RFC3339Nano),
		}, nil
	})
}

// HandleOperatorHeartbeat renews an operator session: the handle lease (the
// guarded UPDATE of §3 — never release-then-reacquire, so a racing session can
// never steal the word during a flap), the operator_sessions liveness/expiry, and
// the operator token's capability expiry, in one transaction.
func HandleOperatorHeartbeat(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	operatorSessionID := stringParam(envelope, "operator_session_id")
	if operatorSessionID == "" {
		return nil, rpc.NewError("schema_invalid", "operator.heartbeat requires operator_session_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		now := time.Now().UTC()
		expiresAt := now.Add(operatorHandleTTL)
		// Guarded lease renewal (§3): only the owning, still-live session renews,
		// and the row never transits a released state mid-renewal. The §3 shape is
		// `leased_until = now() + TTL, last_heartbeat_at = now()`: leased_until is the
		// future expiry, but last_heartbeat_at MUST be `now` (not expiresAt) or
		// operator-handle liveness diagnostics would read a future heartbeat.
		if err := tx.Exec(ctx, `
			UPDATE striatumd.operator_handles
			   SET leased_until = $1, last_heartbeat_at = $2
			 WHERE repository_id = $3 AND leased_session_id = $4 AND released_at IS NULL`,
			expiresAt, now, repositoryID, operatorSessionID,
		); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.operator_sessions
			   SET last_heartbeat_at = $1, expires_at = $2
			 WHERE repository_id = $3 AND operator_session_id = $4 AND state = 'active'`,
			now, expiresAt, repositoryID, operatorSessionID,
		); err != nil {
			return nil, err
		}
		// Renew the operator token's capability expiry in lockstep.
		if err := tx.Exec(ctx, `
			UPDATE striatumd.client_capabilities
			   SET expires_at = $1
			 WHERE session_id = $2 AND revoked_at IS NULL`,
			expiresAt, operatorSessionID,
		); err != nil {
			return nil, err
		}
		return map[string]any{
			"operator_session_id": operatorSessionID,
			"expires_at":          expiresAt.Format(time.RFC3339Nano),
		}, nil
	})
}

// HandleOperatorClose gracefully closes an operator session in one transaction
// (the dedicated close path, never the run-scoped closeRemainingSessions, C-3):
// mark the session closed, release the handle (lazy — only sets released_at, so a
// run's frozen created_by_handle_id snapshot never dangles), and revoke the
// operator token. Lazy expiry (no reaper) bounds anything left unclosed.
func HandleOperatorClose(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	operatorSessionID := stringParam(envelope, "operator_session_id")
	if operatorSessionID == "" {
		return nil, rpc.NewError("schema_invalid", "operator.close requires operator_session_id", nil)
	}
	reason := stringParam(envelope, "reason")
	if reason == "" {
		reason = "operator session closed"
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		now := time.Now().UTC()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.operator_sessions
			   SET state = 'closed', closed_at = $1, close_reason = $2
			 WHERE repository_id = $3 AND operator_session_id = $4 AND state = 'active'`,
			now, reason, repositoryID, operatorSessionID,
		); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.operator_handles
			   SET released_at = $1
			 WHERE repository_id = $2 AND leased_session_id = $3 AND released_at IS NULL`,
			now, repositoryID, operatorSessionID,
		); err != nil {
			return nil, err
		}
		// Revoke the operator token: a revoked grant fails Authorize (auth filters
		// revoked_at IS NULL), so the closed session's token is immediately inert.
		if err := tx.Exec(ctx, `
			UPDATE striatumd.client_capabilities
			   SET revoked_at = $1, revoked_reason = $2
			 WHERE session_id = $3 AND revoked_at IS NULL`,
			now, reason, operatorSessionID,
		); err != nil {
			return nil, err
		}
		return map[string]any{
			"operator_session_id": operatorSessionID,
			"state":               "closed",
		}, nil
	})
}
