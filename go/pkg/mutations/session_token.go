package mutations

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// sessionBoundTokenTTL bounds a per-session capability token's lifetime. A lane
// session is short-lived relative to the daemon, so a generous-but-bounded TTL
// keeps a leaked token from outliving the run by much. The token is also
// implicitly useless once its session closes (the bound enforcement still
// requires the act-as session to exist and be live for the underlying verb).
const sessionBoundTokenTTL = 24 * time.Hour

// sessionBoundCapabilities are the capabilities a supervised lane needs to
// drive its own workflow loop:
//   - claim for the work.* lifecycle (await_packet / ack / heartbeat / release / complete);
//   - write for artifact.publish + interrogation answers;
//   - read because the agent-loop daemon receiver polls supervise.status (a
//     CapabilityRead method) as its await_packet readiness gate (loop.go
//     daemonReceiverReady). Omitting read left the receiver looping forever on
//     "daemon RPC authorization failed", so a supervised lane could never claim
//     its first packet — a silent rescue-forcing wedge that surfaced only once
//     the #135 session-bound token was actually wired into the lane env (before
//     that, lanes used the shared repo-scoped token, which carries read).
//
// All grants are bound to the registering session_id (RFC 0096 V2 / #135), so
// the resolved AuthContext for this token reports IsSessionBound and the
// session-scoped handlers refuse any act-as session other than this one.
var sessionBoundCapabilities = []rpc.Capability{rpc.CapabilityClaim, rpc.CapabilityWrite, rpc.CapabilityRead}

// mintSessionBoundToken inserts a client + per-capability grants all bound to
// sessionID and the registering repository, returning the bearer token string
// (token_id.secret). Runs inside the registration transaction so the token is
// committed atomically with the session row. The hashing scheme matches
// admin.CreateToken / PostgresAuthorizer (HMAC-SHA256 over salt+secret) so the
// production authorizer validates it without any new code path.
//
// RFC 0096 V2 / #135: the supervised lane's env now carries this token (wired in
// HandleSuperviseStart), so the session-binding enforcement bites for live lanes
// — which is exactly why the grant set above must cover every capability the lane
// loop exercises (read for the receiver's supervise.status readiness gate, claim
// for work.*, write for publish).
func mintSessionBoundToken(ctx context.Context, tx db.TxRunner, repositoryID, sessionID string) (map[string]any, error) {
	tokenID := "stok_" + randomURLSafe(12)
	secret := randomURLSafe(32)
	salt := randomHexToken(16)
	clientID := "sclient_" + randomHexToken(16)
	now := time.Now().UTC()
	expiresAt := now.Add(sessionBoundTokenTTL)
	tokenHash := hmacHexToken(salt, secret)

	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name,
		  token_id, token_hash, token_salt, created_at, expires_at)
		VALUES ($1, 'session', $2, $3, $4, $5, $6, $7)`,
		clientID, "session "+sessionID, tokenID, tokenHash, salt, now, expiresAt,
	); err != nil {
		return nil, err
	}
	for _, capability := range sessionBoundCapabilities {
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.client_capabilities(
			  capability_id, client_id, repository_id, capability, granted_at,
			  expires_at, session_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			"scap_"+randomHexToken(16), clientID, repositoryID, string(capability),
			now, expiresAt, sessionID,
		); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"token":      tokenID + "." + secret,
		"token_id":   tokenID,
		"client_id":  clientID,
		"session_id": sessionID,
		"expires_at": expiresAt.Format(time.RFC3339Nano),
	}, nil
}

func hmacHexToken(salt, secret string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomURLSafe(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func randomHexToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
