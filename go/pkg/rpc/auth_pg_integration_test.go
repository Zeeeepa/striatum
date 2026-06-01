package rpc_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestPostgresAuthorizerCapabilityDenialMatrix(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES
		  ('repo_a','ident_a','/tmp/repo-a','/tmp/repo-a/.striatum','repo-a',$1,14,'active'),
		  ('repo_b','ident_b','/tmp/repo-b','/tmp/repo-b/.striatum','repo-b',$1,14,'active')`,
		now,
	); err != nil {
		t.Fatalf("insert repositories: %v", err)
	}
	insertClient(t, runner, "client_ok", "tok_ok", "secret", "salt", now, "", "")
	insertCapability(t, runner, "cap_ok_read_repo_a", "client_ok", "repo_a", rpc.CapabilityRead, now, "", "")
	insertClient(t, runner, "client_revoked", "tok_revoked", "secret", "salt", now, "", now.Format(time.RFC3339))
	insertClient(t, runner, "client_expired", "tok_expired", "secret", "salt", now, now.Add(-time.Minute).Format(time.RFC3339), "")
	insertClient(t, runner, "client_no_cap", "tok_no_cap", "secret", "salt", now, "", "")
	insertClient(t, runner, "client_cap_expired", "tok_cap_expired", "secret", "salt", now, "", "")
	insertCapability(t, runner, "cap_expired", "client_cap_expired", "", rpc.CapabilityRead, now, now.Add(-time.Minute).Format(time.RFC3339), "")

	required := rpc.CapabilityRead
	auth := rpc.PostgresAuthorizer{Runner: runner, Clock: func() time.Time { return now }}
	for _, tc := range []struct {
		name   string
		repoID string
		token  string
		want   string
	}{
		{"missing", "repo_a", "", "token_missing"},
		{"malformed", "repo_a", "tok_only", "token_malformed"},
		{"invalid", "repo_a", "tok_missing.secret", "token_invalid"},
		{"wrong_secret", "repo_a", "tok_ok.wrong", "token_invalid"},
		{"revoked_token", "repo_a", "tok_revoked.secret", "token_revoked"},
		{"expired_token", "repo_a", "tok_expired.secret", "token_expired"},
		{"missing_capability", "repo_a", "tok_no_cap.secret", "capability_missing"},
		{"scope_mismatch", "repo_b", "tok_ok.secret", "capability_scope_mismatch"},
		{"expired_capability", "repo_a", "tok_cap_expired.secret", "capability_expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := auth.Authorize(&required, tc.repoID, tc.token)
			if got.Decision != "denied" || got.DenialReason != tc.want {
				t.Fatalf("auth = decision:%q reason:%q, want denied/%s", got.Decision, got.DenialReason, tc.want)
			}
		})
	}
	allowed := auth.Authorize(&required, "repo_a", "tok_ok.secret")
	if allowed.Decision != "allowed" || allowed.Capability != rpc.CapabilityRead {
		t.Fatalf("allowed auth = %#v", allowed)
	}
}

type execer interface {
	Exec(context.Context, string, ...any) error
}

func insertClient(t *testing.T, runner execer, clientID, tokenID, secret, salt string, now time.Time, expiresAt, revokedAt string) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.clients (
		  client_id, client_kind, display_name, token_id, token_hash, token_salt,
		  created_at, expires_at, revoked_at
		) VALUES ($1,'test',$1,$2,$3,$4,$5,$6::timestamptz,$7::timestamptz)`,
		clientID, tokenID, hmacHex(salt, secret), salt, now, nullableText(expiresAt), nullableText(revokedAt),
	); err != nil {
		t.Fatalf("insert client %s: %v", clientID, err)
	}
}

func insertCapability(t *testing.T, runner execer, capabilityID, clientID, repoID string, capability rpc.Capability, now time.Time, expiresAt, revokedAt string) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.client_capabilities (
		  capability_id, client_id, repository_id, capability, granted_at, expires_at, revoked_at
		) VALUES ($1,$2,$3,$4,$5,$6::timestamptz,$7::timestamptz)`,
		capabilityID, clientID, nullableText(repoID), string(capability), now, nullableText(expiresAt), nullableText(revokedAt),
	); err != nil {
		t.Fatalf("insert capability %s: %v", capabilityID, err)
	}
}

// TestPostgresAuthorizerSurfacesSessionBinding verifies RFC 0096 V2 / #135: a
// session-bound capability grant surfaces SessionID on the resolved
// AuthContext (so session-scoped handlers can enforce per-session binding),
// while an unbound grant yields an empty SessionID (operator/coordinator).
func TestPostgresAuthorizerSurfacesSessionBinding(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_s','ident_s','/tmp/repo-s','/tmp/repo-s/.striatum','repo-s',$1,22,'active')`,
		now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	// bound token: a grant carrying session_id.
	insertClient(t, runner, "client_bound", "tok_bound", "secret", "salt", now, "", "")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.client_capabilities (
		  capability_id, client_id, repository_id, capability, granted_at, session_id
		) VALUES ('cap_bound','client_bound','repo_s',$1,$2,'sess_target')`,
		string(rpc.CapabilityWrite), now,
	); err != nil {
		t.Fatalf("insert bound capability: %v", err)
	}
	// unbound (operator) token: a grant with NULL session_id.
	insertClient(t, runner, "client_unbound", "tok_unbound", "secret", "salt", now, "", "")
	insertCapability(t, runner, "cap_unbound", "client_unbound", "repo_s", rpc.CapabilityWrite, now, "", "")

	required := rpc.CapabilityWrite
	auth := rpc.PostgresAuthorizer{Runner: runner, Clock: func() time.Time { return now }}

	bound := auth.Authorize(&required, "repo_s", "tok_bound.secret")
	if bound.Decision != "allowed" || !bound.IsSessionBound() || bound.SessionID != "sess_target" {
		t.Fatalf("bound auth = %#v, want allowed bound to sess_target", bound)
	}
	unbound := auth.Authorize(&required, "repo_s", "tok_unbound.secret")
	if unbound.Decision != "allowed" || unbound.IsSessionBound() {
		t.Fatalf("unbound auth = %#v, want allowed and session-unbound", unbound)
	}
}

func hmacHex(salt, secret string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
