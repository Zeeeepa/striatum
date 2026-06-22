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
	setCodexDoctorEnv(t, home, "http://127.0.0.1:2222/mcp/sse", "SENSITIVE-VALUE")

	block, warnings := codexDoctorBlock()
	if block["stale"] != true {
		t.Fatalf("expected stale=true, block=%#v", block)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "codex_config_stale") {
		t.Fatalf("expected stale warning, got: %v", warnings)
	}
	if strings.Contains(joined, "SENSITIVE-VALUE") {
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

func TestCodexDoctorWarnsWhenEnvMissingEvenIfRuntimeTokenExists(t *testing.T) {
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \"http://127.0.0.1:3333/mcp/sse\"\nbearer_token_env_var = \"STRIATUM_MCP_TOKEN\"\n")
	setCodexDoctorEnv(t, home, "http://127.0.0.1:3333/mcp/sse", "")
	tokenFile := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenFile, []byte("SENSITIVE-VALUE"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeClientTokenPath = func() (string, error) { return tokenFile, nil }

	block, warnings := codexDoctorBlock()
	if block["token_env_present"] != false {
		t.Fatalf("expected token_env_present=false, block=%#v", block)
	}
	if block["runtime_token_present"] != true {
		t.Fatalf("expected runtime_token_present=true, block=%#v", block)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "codex_token_env_absent") {
		t.Fatalf("expected env-absent warning, got: %v", warnings)
	}
	if strings.Contains(joined, "SENSITIVE-VALUE") {
		t.Fatalf("token value leaked into warnings: %v", warnings)
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
	if block["token_present"] != false || block["token_env_present"] != false {
		t.Fatalf("expected direct codex token env to be absent, block=%#v", block)
	}
	if block["runtime_token_present"] != true {
		t.Fatalf("expected runtime_token_present=true with token file, block=%#v", block)
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

// stubLaneCodexConfig points STRIATUM_LANE_OS_USER at a distinct fake lane user
// whose home contains the given ~/.codex/config.toml, stubbing the OS-user home
// lookup and the daemon user so codexLaneConfigBlock inspects the fixture. It
// returns the lane home dir. With contents=="" the lane home has no codex config.
func stubLaneCodexConfig(t *testing.T, laneUser, contents string) string {
	t.Helper()
	laneHome := t.TempDir()
	if contents != "" {
		codexDir := filepath.Join(laneHome, ".codex")
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(laneOSUserEnv, laneUser)
	origUser := currentUsername
	origLookup := lookupOSUserHome
	currentUsername = func() string { return "striatum-daemon" }
	lookupOSUserHome = func(name string) (string, bool) {
		if name == laneUser {
			return laneHome, true
		}
		return "", false
	}
	t.Cleanup(func() { currentUsername = origUser; lookupOSUserHome = origLookup })
	return laneHome
}

// #568: a surviving codex lane reads the LANE OS user's ~/.codex/config.toml,
// which goes stale across a daemon restart. doctor must flag the lane-side stale
// endpoint, not only the daemon user's config.
func TestCodexDoctorFlagsStaleLaneConfig(t *testing.T) {
	const live = "http://127.0.0.1:55555/mcp/sse"
	// Daemon user's own config is FRESH (no daemon-side warning).
	daemonHome := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \""+live+"\"\nbearer_token_env_var = \""+agentloop.EnvMCPToken+"\"\n")
	setCodexDoctorEnv(t, daemonHome, live, "bearer")
	// Lane user's config points at a STALE (different) port.
	const staleLane = "http://127.0.0.1:11111/mcp/sse"
	stubLaneCodexConfig(t, "striatum-lane", "[mcp_servers.striatum]\nurl = \""+staleLane+"\"\n")

	block, warnings := codexDoctorBlock()
	laneBlock, ok := block["lane_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected lane_config sub-block, block=%#v", block)
	}
	if laneBlock["stale"] != true {
		t.Fatalf("expected lane_config.stale=true, lane_block=%#v", laneBlock)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "codex_lane_config_stale") || !strings.Contains(joined, "striatum-lane") || !strings.Contains(joined, staleLane) {
		t.Fatalf("expected stale lane-config warning naming the lane user + stale url, got: %v", warnings)
	}
	// The daemon-user config is fresh, so there must be NO daemon-side stale warn.
	if strings.Contains(joined, "codex_config_stale:") {
		t.Fatalf("daemon-user config is fresh; should not warn codex_config_stale: %v", warnings)
	}
}

// A fresh lane config produces a lane_config block but no stale warning.
func TestCodexDoctorLaneConfigFreshNoWarning(t *testing.T) {
	const live = "http://127.0.0.1:55555/mcp/sse"
	daemonHome := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \""+live+"\"\n")
	setCodexDoctorEnv(t, daemonHome, live, "bearer")
	stubLaneCodexConfig(t, "striatum-lane", "[mcp_servers.striatum]\nurl = \""+live+"\"\n")

	block, warnings := codexDoctorBlock()
	laneBlock, ok := block["lane_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected lane_config sub-block, block=%#v", block)
	}
	if laneBlock["stale"] != false {
		t.Fatalf("expected lane_config.stale=false for matching endpoint, lane_block=%#v", laneBlock)
	}
	if strings.Contains(strings.Join(warnings, "\n"), "codex_lane_config_stale") {
		t.Fatalf("fresh lane config must not warn: %v", warnings)
	}
}

// No lane_config block when STRIATUM_LANE_OS_USER is unset (lane == daemon user).
func TestCodexDoctorNoLaneBlockWhenLaneUserUnset(t *testing.T) {
	const live = "http://127.0.0.1:55555/mcp/sse"
	home := writeCodexConfig(t, "[mcp_servers.striatum]\nurl = \""+live+"\"\n")
	setCodexDoctorEnv(t, home, live, "bearer")
	t.Setenv(laneOSUserEnv, "")

	block, _ := codexDoctorBlock()
	if _, present := block["lane_config"]; present {
		t.Fatalf("no distinct lane user => no lane_config block, block=%#v", block)
	}
}
