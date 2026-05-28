package agentloop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectLaneMCPConfigClaudeWritesEphemeralStrictConfig(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"/home/x/.local/bin/claude", "--model", "opus"},
		repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	defer cleanup()

	// Flags appended after the original command.
	if len(cmd) < 5 || cmd[len(cmd)-1] != "--strict-mcp-config" || cmd[len(cmd)-3] != "--mcp-config" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
	cfgPath := cmd[len(cmd)-2]
	if !strings.HasPrefix(cfgPath, filepath.Join(repo, ".striatum", "scratch")) {
		t.Fatalf("config not under .striatum/scratch: %q", cfgPath)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode()&0o077 != 0 {
		t.Fatalf("config not 0600: %v", info.Mode())
	}
	body, _ := os.ReadFile(cfgPath)
	var parsed struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("config json: %v", err)
	}
	s, ok := parsed.MCPServers["striatum"]
	if !ok || s.URL != "http://127.0.0.1:34135/mcp" || s.Headers["Authorization"] != "Bearer dtok_secret" {
		t.Fatalf("config content wrong: %s", body)
	}

	// Cleanup removes the file.
	cleanup()
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("ephemeral config not removed")
	}
}

func TestInjectLaneMCPConfigAgyWritesEphemeralStrictConfig(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"/home/x/.local/bin/agy", "--model", "opus"},
		repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	defer cleanup()

	// Flags appended after the original command.
	if len(cmd) < 5 || cmd[len(cmd)-1] != "--strict-mcp-config" || cmd[len(cmd)-3] != "--mcp-config" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
	cfgPath := cmd[len(cmd)-2]
	if !strings.HasPrefix(cfgPath, filepath.Join(repo, ".striatum", "scratch")) {
		t.Fatalf("config not under .striatum/scratch: %q", cfgPath)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode()&0o077 != 0 {
		t.Fatalf("config not 0600: %v", info.Mode())
	}
	body, _ := os.ReadFile(cfgPath)
	var parsed struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("config json: %v", err)
	}
	s, ok := parsed.MCPServers["striatum"]
	if !ok || s.URL != "http://127.0.0.1:34135/mcp" || s.Headers["Authorization"] != "Bearer dtok_secret" {
		t.Fatalf("config content wrong: %s", body)
	}

	// Cleanup removes the file.
	cleanup()
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("ephemeral config not removed")
	}
}

func TestInjectLaneMCPConfigCodexAppendsTomlUrlOverride(t *testing.T) {
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"/home/x/.local/bin/codex"},
		t.TempDir(), "http://127.0.0.1:42727/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject codex: %v", err)
	}
	defer cleanup()
	want := []string{"/home/x/.local/bin/codex", "-c", `mcp_servers.striatum.url="http://127.0.0.1:42727/mcp"`}
	if strings.Join(cmd, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("codex injection = %#v, want %#v", cmd, want)
	}
}

func TestInjectLaneMCPConfigPassthrough(t *testing.T) {
	// No token: unchanged even for injected adapters.
	for _, adapter := range []string{"claude", "agy", "codex"} {
		cmd, _, err := injectLaneMCPConfig([]string{adapter}, t.TempDir(), "http://x/mcp", TokenMaterial{})
		if err != nil || strings.Join(cmd, " ") != adapter {
			t.Fatalf("no-token %s passthrough failed: %#v %v", adapter, cmd, err)
		}
	}
	// Unknown adapter: unchanged.
	cmd, _, err := injectLaneMCPConfig([]string{"some-other-cli"}, t.TempDir(), "http://x/mcp", TokenMaterial{Token: "t"})
	if err != nil || strings.Join(cmd, " ") != "some-other-cli" {
		t.Fatalf("unknown-adapter passthrough failed: %#v %v", cmd, err)
	}
}
