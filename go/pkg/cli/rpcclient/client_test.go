package rpcclient

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestResolveConfigUsesExplicitAndEnvValues(t *testing.T) {
	config, err := ResolveConfig([]string{
		"STRIATUM_DAEMON_SOCKET=/env/socket",
		"STRIATUM_DAEMON_TOKEN=env-token",
	}, "/explicit/socket", "explicit-token", "", 123)
	if err != nil {
		t.Fatal(err)
	}
	if config.SocketPath != "/explicit/socket" || config.Token != "explicit-token" || config.DeadlineMS != 123 {
		t.Fatalf("config = %#v", config)
	}
}

func TestResolveConfigUsesRuntimeTokenFile(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("STRIATUM_DAEMON_RUNTIME_DIR", runtimeDir)
	config, err := ResolveConfig([]string{"XDG_RUNTIME_DIR=/tmp/runtime"}, "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if config.TokenFile != filepath.Join(runtimeDir, "client-token") {
		t.Fatalf("token file = %q", config.TokenFile)
	}
	if config.DeadlineMS != DefaultDeadlineMS {
		t.Fatalf("deadline = %d", config.DeadlineMS)
	}
}

func TestClientMapsDaemonRPCError(t *testing.T) {
	err := rpcError(rpc.Response{OK: false, Data: map[string]any{"code": "repo_not_registered", "message": "missing repo"}})
	if ExitCode(err) != 12 {
		t.Fatalf("exit code = %d", ExitCode(err))
	}
}

func TestClientInvokeReadsTokenAndUsesEnvelope(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	tokenFile := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenFile, []byte("tok.secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := rpc.NewServer()
	server.Register("status", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if envelope.CapabilityToken != "tok.secret" {
			t.Fatalf("token = %q", envelope.CapabilityToken)
		}
		if envelope.Params["repository_id"] != "repo_1" {
			t.Fatalf("params = %#v", envelope.Params)
		}
		return map[string]any{"state": "ok"}, nil
	})
	listener, err := rpc.ListenUnix(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = server.Serve(ctx, listener)
	}()

	data, err := Client{Config: Config{SocketPath: socket, TokenFile: tokenFile, DeadlineMS: 1000}}.Invoke(context.Background(), "status", map[string]any{"repository_id": "repo_1"})
	if err != nil {
		t.Fatal(err)
	}
	if data["state"] != "ok" {
		t.Fatalf("data = %#v", data)
	}
}

func TestClientInvokeFallsBackToDiscoveryTokenAndRepairsRuntimeToken(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	runtimeDir := t.TempDir()
	tokenFile := filepath.Join(runtimeDir, "client-token")
	if err := os.WriteFile(tokenFile, []byte("stale.secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "discovery.json"), []byte(`{"client_token":"valid.secret"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	authorizer := &recordingTokenAuthorizer{valid: "valid.secret"}
	server := rpc.NewServer()
	server.Authorizer = authorizer
	server.Register("status", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if envelope.CapabilityToken != "valid.secret" {
			t.Fatalf("handler saw token %q, want discovery fallback token", envelope.CapabilityToken)
		}
		return map[string]any{"state": "ok"}, nil
	})
	listener, err := rpc.ListenUnix(socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = server.Serve(ctx, listener)
	}()

	data, err := Client{Config: Config{SocketPath: socket, TokenFile: tokenFile, DeadlineMS: 1000}}.Invoke(context.Background(), "status", map[string]any{"repository_id": "repo_1"})
	if err != nil {
		t.Fatal(err)
	}
	if data["state"] != "ok" {
		t.Fatalf("data = %#v", data)
	}
	if got := authorizer.tokens(); len(got) != 2 || got[0] != "stale.secret" || got[1] != "valid.secret" {
		t.Fatalf("auth tokens = %v, want stale token then discovery token", got)
	}
	body, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "valid.secret\n" {
		t.Fatalf("runtime token file = %q, want repaired discovery token", string(body))
	}
}

type recordingTokenAuthorizer struct {
	mu    sync.Mutex
	valid string
	seen  []string
}

func (a *recordingTokenAuthorizer) Authorize(required *rpc.Capability, repositoryID string, token string) rpc.AuthContext {
	a.mu.Lock()
	a.seen = append(a.seen, token)
	a.mu.Unlock()
	if token == a.valid {
		return rpc.AuthContext{RepositoryID: repositoryID, Capability: *required, Decision: "allowed"}
	}
	return rpc.AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_invalid"}
}

func (a *recordingTokenAuthorizer) tokens() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.seen...)
}
