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

// #316: the daemon mints a fresh per-process boot epoch and publishes it to the
// runtime-dir sibling file (0600) so a client can read the expected epoch.
func TestWriteBootEpochFileRoundTrips(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("STRIATUM_DAEMON_RUNTIME_DIR", runtimeDir)

	epoch := daemonBootEpoch()
	if epoch == "" || epoch == "epoch-unknown" {
		t.Fatalf("daemonBootEpoch() = %q, want a real per-process epoch", epoch)
	}
	// Stable within a process run.
	if again := daemonBootEpoch(); again != epoch {
		t.Fatalf("daemonBootEpoch() not stable: %q then %q", epoch, again)
	}

	path, err := writeBootEpochFile(epoch)
	if err != nil {
		t.Fatalf("writeBootEpochFile() error = %v", err)
	}
	if path != filepath.Join(runtimeDir, bootEpochFileName) {
		t.Fatalf("path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read boot epoch file: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != epoch {
		t.Fatalf("boot epoch file body = %q, want %q", got, epoch)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat boot epoch file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("boot epoch file mode = %o, want 0600", mode)
	}
}

// #316: the per-process boot epoch is NOT the (deliberately restart-stable)
// daemon instance id; they must be independently generated so a restart that
// recycles the MCP port still flips the epoch.
func TestBootEpochDistinctFromInstanceID(t *testing.T) {
	if daemonBootEpoch() == daemonInstanceID() {
		t.Fatal("boot epoch must not equal the restart-stable instance id")
	}
	if !strings.HasPrefix(daemonBootEpoch(), "epoch-") {
		t.Fatalf("boot epoch = %q, want epoch- prefix", daemonBootEpoch())
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
