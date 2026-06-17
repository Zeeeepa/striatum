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
	endpoint, err := ResolveMCPEndpoint("", isolatedEndpointEnv(t, EnvMCPPort+"=2468"))
	if err != nil {
		t.Fatalf("ResolveMCPEndpoint() error = %v", err)
	}
	if endpoint != "http://127.0.0.1:2468/mcp/sse" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestResolveMCPEndpointSourcePriority(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "mcp-http-endpoint"), []byte("127.0.0.1:2222\n"), 0o600); err != nil {
		t.Fatalf("write runtime endpoint: %v", err)
	}
	endpointFile := filepath.Join(t.TempDir(), "endpoint")
	if err := os.WriteFile(endpointFile, []byte("127.0.0.1:1111\n"), 0o600); err != nil {
		t.Fatalf("write explicit endpoint file: %v", err)
	}
	repoRoot := t.TempDir()
	repoMetadataDir := filepath.Join(repoRoot, ".striatum")
	if err := os.Mkdir(repoMetadataDir, 0o700); err != nil {
		t.Fatalf("mkdir repo metadata dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoMetadataDir, "daemon-mcp.json"), []byte(`{"sse_url":"127.0.0.1:3333"}`), 0o600); err != nil {
		t.Fatalf("write repo metadata: %v", err)
	}

	tests := []struct {
		name     string
		noRepo   bool
		env      []string
		expected string
	}{
		{
			name: "explicit URL wins over all other sources",
			env: []string{
				EnvMCPURL + "=http://127.0.0.1:9999/custom",
				EnvDaemonMCPHTTPEndpointFile + "=" + endpointFile,
				EnvDaemonRuntimeDir + "=" + runtimeDir,
				EnvDaemonMCPHTTPAddr + "=127.0.0.1:4444",
				EnvMCPPort + "=5555",
			},
			expected: "http://127.0.0.1:9999/custom",
		},
		{
			name: "explicit endpoint file wins over runtime dir",
			env: []string{
				EnvDaemonMCPHTTPEndpointFile + "=" + endpointFile,
				EnvDaemonRuntimeDir + "=" + runtimeDir,
				EnvDaemonMCPHTTPAddr + "=127.0.0.1:4444",
				EnvMCPPort + "=5555",
			},
			expected: "http://127.0.0.1:1111/mcp/sse",
		},
		{
			name: "runtime dir wins over repo metadata",
			env: []string{
				EnvDaemonRuntimeDir + "=" + runtimeDir,
				EnvDaemonMCPHTTPAddr + "=127.0.0.1:4444",
				EnvMCPPort + "=5555",
			},
			expected: "http://127.0.0.1:2222/mcp/sse",
		},
		{
			name: "repo metadata wins over address",
			env: []string{
				EnvDaemonRuntimeDir + "=" + t.TempDir(),
				EnvDaemonMCPHTTPAddr + "=127.0.0.1:4444",
				EnvMCPPort + "=5555",
			},
			expected: "http://127.0.0.1:3333/mcp/sse",
		},
		{
			name:   "address wins over port",
			noRepo: true,
			env: []string{
				EnvDaemonRuntimeDir + "=" + t.TempDir(),
				EnvDaemonMCPHTTPAddr + "=127.0.0.1:4444",
				EnvMCPPort + "=5555",
			},
			expected: "http://127.0.0.1:4444/mcp/sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repoRoot
			if tt.noRepo {
				repo = ""
			}
			endpoint, err := ResolveMCPEndpoint(repo, tt.env)
			if err != nil {
				t.Fatalf("ResolveMCPEndpoint() error = %v", err)
			}
			if endpoint != tt.expected {
				t.Fatalf("endpoint = %q, want %q", endpoint, tt.expected)
			}
		})
	}
}

// TestResolveMCPEndpointFreshAfterRotation is the #323 rotation reader fixture:
// a mid-run daemon restart rewrites the runtime endpoint file to a new dynamic
// port; ResolveMCPEndpointFresh must return the NEW endpoint by re-reading the
// file, ignoring any cached launch literal.
func TestResolveMCPEndpointFreshAfterRotation(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv(EnvDaemonRuntimeDir, runtimeDir)
	path := filepath.Join(runtimeDir, "mcp-http-endpoint")

	if err := os.WriteFile(path, []byte("127.0.0.1:37637\n"), 0o600); err != nil {
		t.Fatalf("write initial endpoint: %v", err)
	}
	first, err := ResolveMCPEndpointFresh("")
	if err != nil {
		t.Fatalf("ResolveMCPEndpointFresh() initial error = %v", err)
	}
	if first != "http://127.0.0.1:37637/mcp/sse" {
		t.Fatalf("initial endpoint = %q", first)
	}

	// Daemon restart rotates the dynamic port and rewrites the runtime file.
	if err := os.WriteFile(path, []byte("127.0.0.1:41283\n"), 0o600); err != nil {
		t.Fatalf("rotate endpoint: %v", err)
	}
	second, err := ResolveMCPEndpointFresh("")
	if err != nil {
		t.Fatalf("ResolveMCPEndpointFresh() after rotation error = %v", err)
	}
	if second != "http://127.0.0.1:41283/mcp/sse" {
		t.Fatalf("rotated endpoint = %q, want the NEW port", second)
	}
}

// TestResolveMCPEndpointFreshIgnoresLaunchEnvLiteral proves the fresh reader
// bypasses the env-baked launch literal (STRIATUM_MCP_URL) that the lane was
// launched with — the whole point of #323 is to NOT trust that stale value.
func TestResolveMCPEndpointFreshIgnoresLaunchEnvLiteral(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv(EnvDaemonRuntimeDir, runtimeDir)
	// The lane's environment still carries the stale launch-time URL.
	t.Setenv(EnvMCPURL, "http://127.0.0.1:11111/mcp/sse")
	if err := os.WriteFile(filepath.Join(runtimeDir, "mcp-http-endpoint"), []byte("127.0.0.1:22222\n"), 0o600); err != nil {
		t.Fatalf("write runtime endpoint: %v", err)
	}
	fresh, err := ResolveMCPEndpointFresh("")
	if err != nil {
		t.Fatalf("ResolveMCPEndpointFresh() error = %v", err)
	}
	if fresh != "http://127.0.0.1:22222/mcp/sse" {
		t.Fatalf("fresh endpoint = %q, want the on-disk runtime value, not the stale env literal", fresh)
	}
}

// TestResolveTokenMaterialFreshReadsRuntimeToken proves the sibling token reader
// re-reads the runtime client-token file (the same session-bound bearer the lane
// already uses, #135/#296).
func TestResolveTokenMaterialFreshReadsRuntimeToken(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv(EnvDaemonRuntimeDir, runtimeDir)
	// Ensure no env file pointer shadows the runtime-dir client-token. t.Setenv
	// registers cleanup to restore the prior value after the test.
	t.Setenv(EnvMCPTokenFile, "")
	if err := os.Unsetenv(EnvMCPTokenFile); err != nil {
		t.Fatalf("unset token file env: %v", err)
	}
	tokenPath := filepath.Join(runtimeDir, "client-token")
	if err := os.WriteFile(tokenPath, []byte("dtok_runtime_secret\n"), 0o600); err != nil {
		t.Fatalf("write runtime token: %v", err)
	}
	tok, err := ResolveTokenMaterialFresh("")
	if err != nil {
		t.Fatalf("ResolveTokenMaterialFresh() error = %v", err)
	}
	if tok.Token != "dtok_runtime_secret" {
		t.Fatalf("token = %q, want runtime secret", tok.Token)
	}
	if tok.Source != tokenPath {
		t.Fatalf("token source = %q, want %q", tok.Source, tokenPath)
	}
}

func isolatedEndpointEnv(t *testing.T, env ...string) []string {
	t.Helper()
	return append([]string{EnvDaemonRuntimeDir + "=" + t.TempDir()}, env...)
}
