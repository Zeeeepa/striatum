package rpcclient

import (
	"context"
	"os"
	"path/filepath"
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
