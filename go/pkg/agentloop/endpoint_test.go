package agentloop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMCPEndpointFromRuntimeEndpointFile(t *testing.T) {
	runtimeDir := t.TempDir()
	path := filepath.Join(runtimeDir, "mcp-http-endpoint")
	if err := os.WriteFile(path, []byte("127.0.0.1:4321\n"), 0o600); err != nil {
		t.Fatalf("write endpoint: %v", err)
	}

	endpoint, err := ResolveMCPEndpoint("", []string{EnvDaemonRuntimeDir + "=" + runtimeDir})
	if err != nil {
		t.Fatalf("ResolveMCPEndpoint() error = %v", err)
	}
	if endpoint != "http://127.0.0.1:4321/mcp/sse" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestResolveMCPEndpointPrefersExplicitURL(t *testing.T) {
	endpoint, err := ResolveMCPEndpoint("", []string{
		EnvDaemonRuntimeDir + "=/missing",
		EnvMCPURL + "=http://127.0.0.1:9876/custom",
	})
	if err != nil {
		t.Fatalf("ResolveMCPEndpoint() error = %v", err)
	}
	if endpoint != "http://127.0.0.1:9876/custom" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestResolveMCPEndpointFromPort(t *testing.T) {
	endpoint, err := ResolveMCPEndpoint("", []string{EnvMCPPort + "=2468"})
	if err != nil {
		t.Fatalf("ResolveMCPEndpoint() error = %v", err)
	}
	if endpoint != "http://127.0.0.1:2468/mcp/sse" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}
