package admin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type tokenExecCall struct {
	sql  string
	args []any
}

type tokenFakeRunner struct {
	repositories map[string]bool
	record       tokenRecord
	recordErr    error
	revokeCount  int64
	activeJSON   string
	ambiguous    string
	beginCount   int
	committed    bool
	rolledBack   bool
	execs        []tokenExecCall
}

func (f *tokenFakeRunner) Exec(_ context.Context, sql string, args ...any) error {
	f.execs = append(f.execs, tokenExecCall{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (f *tokenFakeRunner) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "FROM striatumd.clients") && strings.Contains(sql, "FOR UPDATE"):
		if f.recordErr != nil {
			return tokenRow{err: f.recordErr}
		}
		return tokenRow{values: []any{
			f.record.ClientID,
			f.record.ClientKind,
			f.record.DisplayName,
			f.record.TokenID,
			f.record.TokenHash,
			f.record.TokenSalt,
			timestamptz(f.record.ExpiresAt),
			timestamptz(f.record.RevokedAt),
		}}
	case strings.Contains(sql, "WITH revoked AS"):
		return tokenRow{values: []any{f.revokeCount}}
	default:
		return tokenRow{err: pgx.ErrNoRows}
	}
}

func (f *tokenFakeRunner) QueryScalar(_ context.Context, sql string, args ...any) (string, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.repositories"):
		repoID, _ := args[0].(string)
		if f.repositories[repoID] {
			return repoID, nil
		}
		return "", nil
	case strings.Contains(sql, "HAVING COUNT(*) > 1"):
		return f.ambiguous, nil
	case strings.Contains(sql, "jsonb_agg"):
		if f.activeJSON == "" {
			return "[]", nil
		}
		return f.activeJSON, nil
	default:
		return "", nil
	}
}

func (f *tokenFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	f.beginCount++
	return f, nil
}

func (f *tokenFakeRunner) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *tokenFakeRunner) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

type tokenRow struct {
	values []any
	err    error
}

func (r tokenRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			value, _ := r.values[i].(string)
			*target = value
		case *int64:
			value, _ := r.values[i].(int64)
			*target = value
		case *pgtype.Timestamptz:
			value, _ := r.values[i].(pgtype.Timestamptz)
			*target = value
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func timestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func TestRegisterInstallsTokenHandlers(t *testing.T) {
	server := rpc.NewServer()
	Service{Runner: &tokenFakeRunner{}}.Register(server)
	for _, method := range []string{"daemon.token.create", "daemon.token.revoke", "daemon.token.rotate"} {
		if server.Handlers[method] == nil {
			t.Fatalf("%s was not registered", method)
		}
	}
}

func TestCreateTokenGeneratesHMACSecretAndCapabilityRows(t *testing.T) {
	runner := &tokenFakeRunner{repositories: map[string]bool{"repo_a": true}}
	result, err := Service{Runner: runner}.CreateToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"display_name":  "repo writer",
		"client_kind":   "mcp",
		"capabilities":  []any{"read", "write", "surgical_recovery"},
		"repository_id": "repo_a",
		"expires_in":    float64(120),
	}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	token, ok := result["token"].(string)
	if !ok || !strings.HasPrefix(token, "dtok_") || !strings.Contains(token, ".") {
		t.Fatalf("token shape = %#v", result["token"])
	}
	if _, leaked := result["token_hash"]; leaked {
		t.Fatal("stored token hash leaked in response")
	}
	if runner.beginCount != 1 || !runner.committed {
		t.Fatalf("transaction state begin=%d committed=%v", runner.beginCount, runner.committed)
	}
	if len(runner.execs) != 4 {
		t.Fatalf("exec count = %d, want client insert + 3 capabilities", len(runner.execs))
	}
	clientInsert := runner.execs[0]
	if !strings.Contains(clientInsert.sql, "INSERT INTO striatumd.clients") {
		t.Fatalf("first SQL did not insert client: %s", clientInsert.sql)
	}
	tokenID, secret, _ := strings.Cut(token, ".")
	if clientInsert.args[3] != tokenID {
		t.Fatalf("inserted token_id = %v, response token_id = %s", clientInsert.args[3], tokenID)
	}
	hash, _ := clientInsert.args[4].(string)
	salt, _ := clientInsert.args[5].(string)
	if hmacHexToken(salt, secret) != hash {
		t.Fatal("stored token hash does not match HMAC-SHA256 parity helper")
	}
	for _, call := range runner.execs[1:] {
		if !strings.Contains(call.sql, "INSERT INTO striatumd.client_capabilities") {
			t.Fatalf("capability SQL did not insert client_capabilities: %s", call.sql)
		}
		if call.args[2] != "repo_a" {
			t.Fatalf("capability repository scope = %v, want repo_a", call.args[2])
		}
	}
}

func TestCreateTokenValidationFailures(t *testing.T) {
	runner := &tokenFakeRunner{}
	_, err := Service{Runner: runner}.CreateToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"capabilities": []any{"memory"},
	}})
	if code(err) != "schema_invalid" {
		t.Fatalf("unsupported capability error = %v", err)
	}
	if runner.beginCount != 0 {
		t.Fatal("validation failure opened a transaction")
	}
	_, err = Service{Runner: runner}.CreateToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"capabilities":  []any{"read"},
		"repository_id": "repo_missing",
	}})
	if code(err) != "not_found" {
		t.Fatalf("missing repository error = %v", err)
	}
}

func TestRevokeTokenAcceptsFullTokenAndRejectsWrongSecret(t *testing.T) {
	runner := &tokenFakeRunner{
		record: tokenRecord{
			ClientID:    "dclient_old",
			ClientKind:  "cli",
			DisplayName: "old",
			TokenID:     "dtok_old",
			TokenHash:   hmacHexToken("salt", "good"),
			TokenSalt:   "salt",
		},
		revokeCount: 2,
	}
	result, err := Service{Runner: runner}.RevokeToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"token": "dtok_old.good",
	}})
	if err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if result["client_id"] != "dclient_old" || result["revoked_capabilities"] != int64(2) {
		t.Fatalf("unexpected revoke result: %#v", result)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "UPDATE striatumd.clients") {
		t.Fatalf("client revoke SQL missing: %#v", runner.execs)
	}

	runner = &tokenFakeRunner{
		record: tokenRecord{ClientID: "dclient_old", TokenID: "dtok_old", TokenHash: hmacHexToken("salt", "good"), TokenSalt: "salt"},
	}
	_, err = Service{Runner: runner}.RevokeToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"token": "dtok_old.bad",
	}})
	if code(err) != "token_invalid" {
		t.Fatalf("wrong-secret revoke error = %v", err)
	}
	if len(runner.execs) != 0 {
		t.Fatal("wrong-secret revoke updated rows")
	}
}

func TestRotateTokenPreservesActiveScopeAndRevokesOldRows(t *testing.T) {
	oldExpires := time.Now().UTC().Add(10 * time.Minute)
	runner := &tokenFakeRunner{
		record: tokenRecord{
			ClientID:    "dclient_old",
			ClientKind:  "mcp",
			DisplayName: "agent token",
			TokenID:     "dtok_old",
			TokenHash:   hmacHexToken("salt", "good"),
			TokenSalt:   "salt",
			ExpiresAt:   &oldExpires,
		},
		revokeCount: 2,
		activeJSON:  `[{"capability":"read","repository_id":null,"expires_at":null},{"capability":"write","repository_id":"repo_a","expires_at":null}]`,
	}
	result, err := Service{Runner: runner}.RotateToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"token_id": "dtok_old",
	}})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if result["rotated_from_token_id"] != "dtok_old" {
		t.Fatalf("rotate metadata missing: %#v", result)
	}
	if _, ok := result["token"].(string); !ok {
		t.Fatalf("replacement token missing: %#v", result)
	}
	if len(runner.execs) != 4 {
		t.Fatalf("exec count = %d, want revoke client + replacement client + 2 cap inserts", len(runner.execs))
	}
	if !strings.Contains(runner.execs[0].sql, "UPDATE striatumd.clients") {
		t.Fatalf("old client revoke SQL missing: %s", runner.execs[0].sql)
	}
	if runner.execs[2].args[2] != nil {
		t.Fatalf("global read grant repository scope = %v, want nil", runner.execs[2].args[2])
	}
	if runner.execs[3].args[2] != "repo_a" {
		t.Fatalf("write grant repository scope = %v, want repo_a", runner.execs[3].args[2])
	}
}

func TestRotateTokenRefusesAmbiguousScope(t *testing.T) {
	runner := &tokenFakeRunner{
		record:    tokenRecord{ClientID: "dclient_old", TokenID: "dtok_old", TokenHash: hmacHexToken("salt", "good"), TokenSalt: "salt"},
		ambiguous: "read:<global>",
	}
	_, err := Service{Runner: runner}.RotateToken(context.Background(), rpc.Envelope{Params: map[string]any{
		"token_id": "dtok_old",
	}})
	if code(err) != "token_scope_ambiguous" {
		t.Fatalf("ambiguous rotate error = %v", err)
	}
	if len(runner.execs) != 0 {
		t.Fatal("ambiguous rotate updated rows")
	}
}

func code(err error) string {
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
