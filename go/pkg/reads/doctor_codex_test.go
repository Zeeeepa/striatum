package reads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// writeCodexConfig writes a fixture ~/.codex/config.toml under a fake HOME and
// returns the HOME dir.
func writeCodexConfig(t *testing.T, contents string) string {
	t.Helper()
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// setCodexDoctorEnv points HOME at a fixture, sets the live endpoint, and
// controls token presence for the duration of a test.
func setCodexDoctorEnv(t *testing.T, home, liveEndpoint, token string) {
	t.Helper()
	t.Setenv("HOME", home)
	// The agentloop resolvers treat an empty (after-trim) env value as absent,
	// so t.Setenv(..., "") cleanly neutralizes a var and is auto-restored.
	t.Setenv(agentloop.EnvMCPURL, liveEndpoint)
	// Avoid any ambient runtime endpoint/token bleed-through.
	t.Setenv(agentloop.EnvDaemonRuntimeDir, t.TempDir())
	t.Setenv(agentloop.EnvMCPToken, token)
	t.Setenv(agentloop.EnvMCPTokenFile, "")
	// Stub the runtime client-token path to a nonexistent file so the only
	// source of a token is the env var the test controls.
	prev := runtimeClientTokenPath
	missing := filepath.Join(t.TempDir(), "client-token")
	runtimeClientTokenPath = func() (string, error) { return missing, nil }
	t.Cleanup(func() { runtimeClientTokenPath = prev })
}

func TestCodexStriatumURLSectionForm(t *testing.T) {
	body := `
model = "o3"

[mcp_servers.striatum]
url = "http://127.0.0.1:7777/mcp/sse"
`
	url, referenced := codexStriatumURL(body)
	if !referenced {
		t.Fatal("expected striatum server to be referenced")
	}
	if url != "http://127.0.0.1:7777/mcp/sse" {
		t.Fatalf("url = %q", url)
	}
}

func TestCodexStriatumURLDottedForm(t *testing.T) {
	body := `mcp_servers.striatum.url = "http://127.0.0.1:8888/mcp/sse"` + "\n"
	url, referenced := codexStriatumURL(body)
	if !referenced || url != "http://127.0.0.1:8888/mcp/sse" {
		t.Fatalf("dotted form: referenced=%v url=%q", referenced, url)
	}
}

func TestCodexStriatumURLAbsent(t *testing.T) {
	body := "[mcp_servers.other]\nurl = \"http://x/y\"\n"
	_, referenced := codexStriatumURL(body)
	if referenced {
		t.Fatal("no striatum server should be referenced")
	}
}

func TestCodexDoctorWarnsOnStaleEndpoint(t *testing.T) {
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:1111/mcp/sse\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:2222/mcp/sse", "bearer")

	block, warnings := codexDoctorBlock()
	if block["stale"] != true {
		t.Fatalf("expected stale=true, block=%#v", block)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "codex_config_stale") {
		t.Fatalf("expected stale warning, got: %v", warnings)
	}
	if strings.Contains(joined, "bearer") {
		t.Fatalf("token value leaked into warnings: %v", warnings)
	}
}

func TestCodexDoctorCleanWhenEndpointMatches(t *testing.T) {
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:3333/mcp/sse\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:3333/mcp/sse", "bearer")

	block, warnings := codexDoctorBlock()
	if block["stale"] != false {
		t.Fatalf("expected stale=false, block=%#v", block)
	}
	for _, w := range warnings {
		if strings.Contains(w, "codex_config_stale") {
			t.Fatalf("unexpected stale warning: %v", warnings)
		}
	}
}

// A base URL with no /mcp/sse path that matches host:port is NOT stale.
func TestCodexDoctorAuthorityOnlyMatchIsClean(t *testing.T) {
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:4444\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:4444/mcp/sse", "bearer")

	block, _ := codexDoctorBlock()
	if block["stale"] == true {
		t.Fatalf("authority-only match should not be stale, block=%#v", block)
	}
}

func TestCodexDoctorWarnsOnAbsentToken(t *testing.T) {
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:3333/mcp/sse\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:3333/mcp/sse", "") // no token

	block, warnings := codexDoctorBlock()
	if block["token_present"] != false {
		t.Fatalf("expected token_present=false, block=%#v", block)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "codex_token_absent") {
		t.Fatalf("expected token-absent warning, got: %v", warnings)
	}
}

func TestCodexDoctorSilentWhenNoCodexConfig(t *testing.T) {
	home := t.TempDir() // no ~/.codex/config.toml
	setCodexDoctorEnv(t, home, "http://127.0.0.1:3333/mcp/sse", "bearer")

	block, warnings := codexDoctorBlock()
	if block["config_present"] != false {
		t.Fatalf("expected config_present=false, block=%#v", block)
	}
	if len(warnings) != 0 {
		t.Fatalf("no codex config should yield no warnings, got: %v", warnings)
	}
}

func TestCodexDoctorNeverReadsTokenValue(t *testing.T) {
	// Even when a token file exists, the block must report only presence/source,
	// never the token string.
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:3333/mcp/sse\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:3333/mcp/sse", "")
	tokenFile := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenFile, []byte("SENSITIVE-VALUE"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeClientTokenPath = func() (string, error) { return tokenFile, nil }

	block, warnings := codexDoctorBlock()
	if block["token_present"] != true {
		t.Fatalf("expected token_present=true with token file, block=%#v", block)
	}
	for k, v := range block {
		if s, ok := v.(string); ok && strings.Contains(s, "SENSITIVE-VALUE") {
			t.Fatalf("token value leaked into block[%q]=%q", k, s)
		}
	}
	if strings.Contains(strings.Join(warnings, "\n"), "SENSITIVE-VALUE") {
		t.Fatalf("token value leaked into warnings: %v", warnings)
	}
}
