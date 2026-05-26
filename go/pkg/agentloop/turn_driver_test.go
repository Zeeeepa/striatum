package agentloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentOnlyEnvStripsAllStriatumVariables(t *testing.T) {
	env := ContentOnlyEnv([]string{
		"PATH=/bin",
		"HOME=/home/operator",
		"STRIATUM_MCP_URL=http://127.0.0.1:1234/mcp",
		"STRIATUM_MCP_TOKEN=secret",
		"STRIATUM_MCP_TOKEN_FILE=/runtime/client-token",
		"STRIATUM_DAEMON_SOCKET=/tmp/daemon.sock",
		"STRIATUM_REPOSITORY_ID=repo_1",
		"STRIATUM_SESSION_ID=sess_1",
		"STRIATUM_RUN_ID=run_1",
		"STRIATUM_OTHER=blocked-by-default",
	})
	for _, entry := range env {
		if strings.HasPrefix(entry, "STRIATUM_") {
			t.Fatalf("content-only child env leaked Striatum variable %q", entry)
		}
	}
	assertEnvValue(t, env, "PATH", "/bin")
	assertEnvValue(t, env, "HOME", "/home/operator")
}

func TestTurnDriverTokenProviderRereadsTokenFile(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenPath, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	client := &mcpToolClient{token: TokenMaterial{Source: tokenPath}}
	got, err := client.currentToken()
	if err != nil {
		t.Fatalf("currentToken first: %v", err)
	}
	if got != "first" {
		t.Fatalf("token = %q, want first", got)
	}
	if err := os.WriteFile(tokenPath, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("rewrite token: %v", err)
	}
	got, err = client.currentToken()
	if err != nil {
		t.Fatalf("currentToken second: %v", err)
	}
	if got != "second" {
		t.Fatalf("token = %q, want second", got)
	}
}
