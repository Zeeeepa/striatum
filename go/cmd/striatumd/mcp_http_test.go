package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListenMCPHTTPRefusesNonLoopback(t *testing.T) {
	for _, addr := range []string{":0", "0.0.0.0:0", "[::]:0"} {
		t.Run(addr, func(t *testing.T) {
			listener, err := listenMCPHTTP(addr)
			if err == nil {
				_ = listener.Close()
				t.Fatalf("listenMCPHTTP(%q) succeeded, want loopback refusal", addr)
			}
			if !strings.Contains(err.Error(), "loopback") {
				t.Fatalf("listenMCPHTTP(%q) error = %v, want loopback refusal", addr, err)
			}
		})
	}
}

func TestListenMCPHTTPAcceptsLoopback(t *testing.T) {
	listener, err := listenMCPHTTP("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenMCPHTTP() error = %v", err)
	}
	defer func() { _ = listener.Close() }()
	endpoint := mcpEndpointURL(listener.Addr())
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") || !strings.HasSuffix(endpoint, "/mcp") {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestWriteMCPEndpointFileIsOwnerOnly(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("STRIATUM_DAEMON_RUNTIME_DIR", runtimeDir)

	path, err := writeMCPEndpointFile("http://127.0.0.1:12345/mcp")
	if err != nil {
		t.Fatalf("writeMCPEndpointFile() error = %v", err)
	}
	if path != filepath.Join(runtimeDir, "mcp-http-endpoint") {
		t.Fatalf("path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read endpoint file: %v", err)
	}
	if string(body) != "http://127.0.0.1:12345/mcp\n" {
		t.Fatalf("endpoint file body = %q", string(body))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat endpoint file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("endpoint file mode = %o, want 0600", mode)
	}
	dirInfo, err := os.Stat(runtimeDir)
	if err != nil {
		t.Fatalf("stat runtime dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("runtime dir mode = %o, want 0700", mode)
	}
}
