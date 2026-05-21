package admin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapRuntimeTokenCreatesAdminClientAndPrivateFile(t *testing.T) {
	runner := &tokenFakeRunner{}
	tokenPath := filepath.Join(t.TempDir(), "runtime", "client-token")

	result, err := BootstrapRuntimeTokenIfNeeded(context.Background(), runner, tokenPath)
	if err != nil {
		t.Fatalf("bootstrap runtime token: %v", err)
	}
	token, ok := result["token"].(string)
	if !ok || !strings.HasPrefix(token, "dtok_") || !strings.Contains(token, ".") {
		t.Fatalf("unexpected bootstrap token result: %#v", result)
	}
	tokenID, secret, _ := strings.Cut(token, ".")

	body, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("runtime token file was not written: %v", err)
	}
	if string(body) != token+"\n" {
		t.Fatalf("runtime token file content = %q, want token plus newline", string(body))
	}
	if mode := fileMode(t, tokenPath); mode != 0o600 {
		t.Fatalf("runtime token mode = %#o, want 0600", mode)
	}
	if mode := fileMode(t, filepath.Dir(tokenPath)); mode != 0o700 {
		t.Fatalf("runtime token directory mode = %#o, want 0700", mode)
	}
	if runner.beginCount != 1 || !runner.committed {
		t.Fatalf("transaction state begin=%d committed=%v", runner.beginCount, runner.committed)
	}
	if len(runner.execs) != 10 {
		t.Fatalf("exec count = %d, want table lock + client insert + 8 capabilities", len(runner.execs))
	}
	if !strings.Contains(runner.execs[0].sql, "LOCK TABLE striatumd.clients") {
		t.Fatalf("first SQL did not lock clients table: %s", runner.execs[0].sql)
	}
	clientInsert := runner.execs[1]
	if !strings.Contains(clientInsert.sql, "INSERT INTO striatumd.clients") {
		t.Fatalf("second SQL did not insert client: %s", clientInsert.sql)
	}
	if clientInsert.args[1] != "cli" || clientInsert.args[2] != "bootstrap-admin" {
		t.Fatalf("client insert identity = %#v", clientInsert.args[:3])
	}
	if clientInsert.args[3] != tokenID {
		t.Fatalf("inserted token_id = %v, result token_id = %s", clientInsert.args[3], tokenID)
	}
	hash, _ := clientInsert.args[4].(string)
	salt, _ := clientInsert.args[5].(string)
	if hmacHexToken(salt, secret) != hash {
		t.Fatal("stored token hash does not match runtime token secret")
	}
	capabilities := map[string]bool{}
	for _, call := range runner.execs[2:] {
		if !strings.Contains(call.sql, "INSERT INTO striatumd.client_capabilities") {
			t.Fatalf("capability SQL did not insert client_capabilities: %s", call.sql)
		}
		if call.args[2] != nil {
			t.Fatalf("bootstrap capability repository scope = %v, want global nil", call.args[2])
		}
		capability, _ := call.args[3].(string)
		capabilities[capability] = true
	}
	for _, capability := range []string{"admin", "read", "write", "claim", "review", "apply", "recovery", "surgical_recovery"} {
		if !capabilities[capability] {
			t.Fatalf("missing bootstrap capability %q in %#v", capability, capabilities)
		}
	}
}

func TestBootstrapRuntimeTokenSkipsWhenClientsExist(t *testing.T) {
	runner := &tokenFakeRunner{clientCount: 1}
	tokenPath := filepath.Join(t.TempDir(), "runtime", "client-token")

	result, err := BootstrapRuntimeTokenIfNeeded(context.Background(), runner, tokenPath)
	if err != nil {
		t.Fatalf("bootstrap runtime token: %v", err)
	}
	if result != nil {
		t.Fatalf("bootstrap result = %#v, want nil", result)
	}
	if runner.beginCount != 0 || len(runner.execs) != 0 {
		t.Fatalf("existing client path wrote database rows: begin=%d execs=%#v", runner.beginCount, runner.execs)
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("existing client path wrote runtime token file: %v", err)
	}
}

func TestRuntimeTokenPathMatchesPythonRuntimeOverride(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv(EnvRuntimeDir, runtimeDir)

	tokenPath, err := RuntimeTokenPath()
	if err != nil {
		t.Fatalf("runtime token path: %v", err)
	}
	if tokenPath != filepath.Join(runtimeDir, "client-token") {
		t.Fatalf("runtime token path = %s", tokenPath)
	}
}

func TestRuntimeMCPEndpointPathMatchesPythonRuntimeOverride(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv(EnvRuntimeDir, runtimeDir)

	endpointPath, err := RuntimeMCPEndpointPath()
	if err != nil {
		t.Fatalf("runtime MCP endpoint path: %v", err)
	}
	if endpointPath != filepath.Join(runtimeDir, "mcp-http-endpoint") {
		t.Fatalf("runtime MCP endpoint path = %s", endpointPath)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode() & 0o777
}
