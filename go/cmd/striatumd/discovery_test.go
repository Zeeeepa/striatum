package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/admin"
)

func TestWriteDaemonDiscoveryFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(admin.EnvRuntimeDir, tmpDir)

	globalSocketPath = "/tmp/fake-socket.sock"

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	webOpts := webServiceOptions{
		ServiceToken: "secret_token_123",
	}

	discoveryPath, err := writeDaemonDiscoveryFile("http://127.0.0.1:1234/mcp", webOpts, listener)
	if err != nil {
		t.Fatalf("writeDaemonDiscoveryFile: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "discovery.json")
	if discoveryPath != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, discoveryPath)
	}

	info, err := os.Stat(discoveryPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Fatalf("expected discovery permissions 0600, got %#o", mode)
	}

	content, err := os.ReadFile(discoveryPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["pid"] == nil {
		t.Fatal("expected pid to be present")
	}
	if parsed["socket_path"] != "/tmp/fake-socket.sock" {
		t.Fatalf("expected socket_path /tmp/fake-socket.sock, got %v", parsed["socket_path"])
	}
	if parsed["mcp_http_url"] != "http://127.0.0.1:1234/mcp" {
		t.Fatalf("expected mcp_http_url http://127.0.0.1:1234/mcp, got %v", parsed["mcp_http_url"])
	}
	if parsed["client_token"] != "secret_token_123" {
		t.Fatalf("expected client_token secret_token_123, got %v", parsed["client_token"])
	}
}
