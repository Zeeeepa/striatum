package agentloop

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/mcp"
)

// gitInit creates a minimal git work tree in dir for the #70 provenance tests.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitStatusPorcelain returns `git status --porcelain` output for dir.
func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return string(out)
}

// TestAgyGeminiSettingsNeverEntersGitProvenance is the #230 worktree-no-config
// fixture: while an agy lane is live, generated MCP settings must live in the
// lane user's Antigravity settings file, not in target-repo .gemini/.
func TestAgyGeminiSettingsNeverEntersGitProvenance(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	gitInit(t, repo)
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_prov")

	cleanup, err := writeEphemeralGeminiSettings(repo, "http://127.0.0.1:9/mcp", "dtok_bearer_not_real")
	if err != nil {
		t.Fatalf("write gemini settings: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("project .gemini/settings.json must not be created, stat err = %v", err)
	}
	if status := gitStatusPorcelain(t, repo); strings.Contains(status, ".gemini/settings.json") {
		t.Fatalf("generated settings appeared in git status:\n%s", status)
	}
	userSettingsPath := filepath.Join(home, agyUserSettingsRelPath)
	if _, err := os.Stat(userSettingsPath); err != nil {
		t.Fatalf("user-scoped agy settings should exist while lane is live: %v", err)
	}

	cleanup()
	if _, err := os.Stat(userSettingsPath); !os.IsNotExist(err) {
		t.Fatalf("created user settings should be removed on teardown, stat err = %v", err)
	}
}

// TestAgyGeminiSettingsExcludePreservesOperatorExcludes proves agy config
// isolation no longer edits the target repo's local git exclude.
func TestAgyGeminiSettingsExcludePreservesOperatorExcludes(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	gitInit(t, repo)
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_prov2")

	excludePath := gitInfoExcludePath(repo)
	if err := os.WriteFile(excludePath, []byte("# operator excludes\n*.log\nbuild/\n"), 0o644); err != nil {
		t.Fatalf("seed operator excludes: %v", err)
	}

	cleanup, err := writeEphemeralGeminiSettings(repo, "http://127.0.0.1:9/mcp", "dtok_bearer_not_real")
	if err != nil {
		t.Fatalf("write gemini settings: %v", err)
	}
	cleanup()

	b, _ := os.ReadFile(excludePath)
	got := string(b)
	for _, want := range []string{"# operator excludes", "*.log", "build/"} {
		if !strings.Contains(got, want) {
			t.Fatalf("teardown dropped operator exclude %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, geminiExcludeMarker) {
		t.Fatalf("agy settings isolation should not add target git exclude marker:\n%s", got)
	}
}

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

// TestRewriteEphemeralMCPConfigSwapsEndpointAndToken is the #323 rotation
// rewriter fixture: an ephemeral claude config written at endpoint A must, after
// RewriteEphemeralMCPConfig with endpoint B + a new token, report B in place —
// same path, 0600 preserved — so a mid-run port rotation reaches the lane CLI
// without persisting the rotating endpoint anywhere but the ephemeral scratch.
func TestRewriteEphemeralMCPConfigSwapsEndpointAndToken(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := writeEphemeralMCPConfig(repo, "http://127.0.0.1:37637/mcp/sse", "dtok_launch")
	if err != nil {
		t.Fatalf("write ephemeral config: %v", err)
	}
	defer cleanup()

	parse := func() (url, auth string) {
		t.Helper()
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("read config: %v", rerr)
		}
		var parsed struct {
			MCPServers map[string]struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			} `json:"mcpServers"`
		}
		if jerr := json.Unmarshal(body, &parsed); jerr != nil {
			t.Fatalf("config json: %v", jerr)
		}
		s := parsed.MCPServers["striatum"]
		return s.URL, s.Headers["Authorization"]
	}

	if url, auth := parse(); url != "http://127.0.0.1:37637/mcp/sse" || auth != "Bearer dtok_launch" {
		t.Fatalf("pre-rewrite content wrong: url=%q auth=%q", url, auth)
	}

	if err := RewriteEphemeralMCPConfig(path, "http://127.0.0.1:41283/mcp/sse", "dtok_rotated"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	if url, auth := parse(); url != "http://127.0.0.1:41283/mcp/sse" || auth != "Bearer dtok_rotated" {
		t.Fatalf("post-rewrite content wrong: url=%q auth=%q", url, auth)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat rewritten config: %v", err)
	}
	if info.Mode()&0o077 != 0 {
		t.Fatalf("rewritten config not 0600: %v", info.Mode())
	}
}

func TestInjectLaneMCPConfigAgyWritesEphemeralGeminiSettings(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_inject_agy")
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"/home/x/.local/bin/agy", "--dangerously-skip-permissions"},
		repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	defer cleanup()

	// agy has no --mcp-config flag, so the command is unchanged.
	if len(cmd) != 2 || cmd[1] != "--dangerously-skip-permissions" {
		t.Fatalf("agy command should be unchanged (no claude-shaped flags): %#v", cmd)
	}
	for _, a := range cmd {
		if a == "--mcp-config" || a == "--strict-mcp-config" {
			t.Fatalf("agy must not receive claude-shaped MCP flags: %#v", cmd)
		}
	}

	if _, err := os.Stat(filepath.Join(repo, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("agy MCP config must not be written to target-repo .gemini, stat err = %v", err)
	}
	// MCP config is written to user-level Antigravity settings (gemini schema).
	settingsPath := filepath.Join(home, agyUserSettingsRelPath)
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat user agy settings: %v", err)
	}
	if info.Mode()&0o077 != 0 {
		t.Fatalf("settings not 0600: %v", info.Mode())
	}
	body, _ := os.ReadFile(settingsPath)
	var parsed struct {
		MCPServers map[string]struct {
			HTTPURL string            `json:"httpUrl"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("settings json: %v", err)
	}
	s, ok := parsed.MCPServers["striatum"]
	if !ok || s.HTTPURL != "http://127.0.0.1:34135/mcp" || s.Headers["Authorization"] != "Bearer dtok_secret" {
		t.Fatalf("gemini settings content wrong: %s", body)
	}

	// Teardown removes the user settings file we created (no pre-existing settings here).
	cleanup()
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("ephemeral agy user settings not removed on teardown")
	}
}

func TestAgyGeminiSettingsUsesUserSettingsByDefault(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_isolated")

	cleanup, err := writeEphemeralGeminiSettings(repo, "http://127.0.0.1:34135/mcp", "dtok_secret")
	if err != nil {
		t.Fatalf("write gemini settings: %v", err)
	}

	projectPath := filepath.Join(repo, ".gemini", "settings.json")
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		t.Fatalf("project .gemini/settings.json must not be created, stat err = %v", err)
	}
	userSettingsPath := filepath.Join(home, agyUserSettingsRelPath)
	body, err := os.ReadFile(userSettingsPath)
	if err != nil {
		t.Fatalf("read user-scoped agy settings: %v", err)
	}
	if !strings.Contains(string(body), `"usageStatisticsEnabled":false`) ||
		!strings.Contains(string(body), `"httpUrl":"http://127.0.0.1:34135/mcp"`) {
		t.Fatalf("user settings missing striatum MCP/survey suppression: %s", body)
	}

	cleanup()
	if _, err := os.Stat(userSettingsPath); !os.IsNotExist(err) {
		t.Fatalf("created user settings should be removed on cleanup, stat err = %v", err)
	}
}

func TestInjectLaneMCPConfigAgyPreservesExistingGeminiSettings(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_preserve_agy")
	dir := filepath.Join(home, filepath.Dir(agyUserSettingsRelPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, agyUserSettingsRelPath)
	original := []byte(`{"security":{"auth":{"selectedType":"oauth-personal"}}}`)
	if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, cleanup, err := injectLaneMCPConfig(
		[]string{"agy"}, repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	// While active, both the preserved auth block and our mcpServers are present.
	body, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(body), "oauth-personal") || !strings.Contains(string(body), "httpUrl") {
		t.Fatalf("settings should merge existing auth with striatum mcpServers: %s", body)
	}
	// #76: the usage/feedback survey is disabled so a supervised lane does not
	// stall on it inside the PTY.
	if !strings.Contains(string(body), `"usageStatisticsEnabled":false`) {
		t.Fatalf("settings should disable usage statistics/survey (#76): %s", body)
	}
	// Teardown restores the original file verbatim.
	cleanup()
	restored, _ := os.ReadFile(settingsPath)
	if string(restored) != string(original) {
		t.Fatalf("teardown should restore original settings, got: %s", restored)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("project .gemini/settings.json must not be created, stat err = %v", err)
	}
}

func TestAgyGeminiSettingsUsesLaneUserHomeWhenRepoUnwritable(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup/user:1")

	userSettingsPath := filepath.Join(home, agyUserSettingsRelPath)
	if err := os.MkdirAll(filepath.Dir(userSettingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"theme":"dark","mcpServers":{"operator":{"httpUrl":"http://existing.invalid/mcp"}}}`)
	if err := os.WriteFile(userSettingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(repo, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })

	cleanup, err := writeEphemeralGeminiSettings(repo, "http://127.0.0.1:34135/mcp", "dtok_secret")
	if err != nil {
		t.Fatalf("write gemini settings: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(filepath.Join(repo, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("project settings should not be created in unwritable repo, stat err = %v", err)
	}
	body, err := os.ReadFile(userSettingsPath)
	if err != nil {
		t.Fatalf("read user settings: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("user settings json: %v", err)
	}
	if parsed["theme"] != "dark" {
		t.Fatalf("fallback should preserve existing settings: %s", body)
	}
	servers, _ := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["operator"]; !ok {
		t.Fatalf("fallback should preserve existing mcpServers: %s", body)
	}
	striatumServer, _ := servers["striatum"].(map[string]any)
	if striatumServer["httpUrl"] != "http://127.0.0.1:34135/mcp" {
		t.Fatalf("fallback striatum MCP server missing/wrong: %s", body)
	}
	backupPath := agyUserSettingsBackupPath(userSettingsPath, "sup/user:1")
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected user settings backup marker: %v", err)
	}

	if err := os.WriteFile(userSettingsPath, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	CleanupAgyUserSettings(home, "sup/user:1")
	restored, err := os.ReadFile(userSettingsPath)
	if err != nil {
		t.Fatalf("read restored user settings: %v", err)
	}
	if string(restored) != string(original) {
		t.Fatalf("CleanupAgyUserSettings should restore original user settings, got: %s", restored)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected backup marker to be removed, stat err = %v", err)
	}
}

func TestCleanupAgyUserSettingsRemovesCreatedSettings(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("STRIATUM_SUPERVISOR_ID", "sup_created")

	if err := os.Chmod(repo, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })

	cleanup, err := writeEphemeralGeminiSettings(repo, "http://127.0.0.1:34135/mcp", "dtok_secret")
	if err != nil {
		t.Fatalf("write gemini settings: %v", err)
	}
	defer cleanup()

	userSettingsPath := filepath.Join(home, agyUserSettingsRelPath)
	createdPath := agyUserSettingsCreatedPath(userSettingsPath, "sup_created")
	if _, err := os.Stat(userSettingsPath); err != nil {
		t.Fatalf("expected user settings to be created: %v", err)
	}
	if _, err := os.Stat(createdPath); err != nil {
		t.Fatalf("expected created marker: %v", err)
	}

	CleanupAgyUserSettings(home, "sup_created")
	if _, err := os.Stat(userSettingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected user settings to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Fatalf("expected created marker to be removed, stat err = %v", err)
	}
}

func TestInjectLaneMCPConfigCodexAppendsTomlUrlOverride(t *testing.T) {
	repo := t.TempDir()
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"/home/x/.local/bin/codex"},
		repo, "http://127.0.0.1:42727/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject codex: %v", err)
	}
	defer cleanup()
	want := []string{
		"/home/x/.local/bin/codex",
		"-c", `mcp_servers.striatum.url="http://127.0.0.1:42727/mcp"`,
		"-c", `mcp_servers.striatum.bearer_token_env_var="STRIATUM_MCP_TOKEN"`,
		"-c", CodexProjectTrustOverrideArg(repo),
	}
	if strings.Join(cmd, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("codex injection = %#v, want %#v", cmd, want)
	}
}

func TestInjectLaneMCPConfigCodexAppendsBearerEnvOverride(t *testing.T) {
	repo := t.TempDir()
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"codex"},
		repo, "http://127.0.0.1:42727/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject codex: %v", err)
	}
	defer cleanup()
	joined := strings.Join(cmd, "\x00")
	if !strings.Contains(joined, `mcp_servers.striatum.bearer_token_env_var="STRIATUM_MCP_TOKEN"`) {
		t.Fatalf("codex injection missing bearer token env override: %#v", cmd)
	}
}

func TestInjectCodexMCPConfigArgsPrecedesExecSubcommand(t *testing.T) {
	repo := t.TempDir()
	cmd := InjectCodexMCPConfigArgs(
		[]string{"codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "-"},
		repo,
		"http://127.0.0.1:42727/mcp",
	)
	wantPrefix := []string{
		"codex",
		"-c", `mcp_servers.striatum.url="http://127.0.0.1:42727/mcp"`,
		"-c", `mcp_servers.striatum.bearer_token_env_var="STRIATUM_MCP_TOKEN"`,
		"-c", CodexProjectTrustOverrideArg(repo),
		"exec",
	}
	if len(cmd) < len(wantPrefix) || strings.Join(cmd[:len(wantPrefix)], "\x00") != strings.Join(wantPrefix, "\x00") {
		t.Fatalf("codex exec injection = %#v, want prefix %#v", cmd, wantPrefix)
	}
	if cmd[len(cmd)-1] != "-" {
		t.Fatalf("codex exec stdin marker moved: %#v", cmd)
	}
}

// #316: when STRIATUM_MCP_BOOT_EPOCH is set, the claude ephemeral --mcp-config
// carries the boot epoch as the mcp.HeaderBootEpoch request header alongside the
// bearer; when unset, no epoch header is added (backward-compatible).
func TestInjectLaneMCPConfigClaudeCarriesBootEpochHeader(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvMCPBootEpoch, "epoch-claude-test")
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"claude"}, repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	defer cleanup()
	cfgPath := cmd[len(cmd)-2]
	body, _ := os.ReadFile(cfgPath)
	var parsed struct {
		MCPServers map[string]struct {
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("config json: %v", err)
	}
	if got := parsed.MCPServers["striatum"].Headers[mcp.HeaderBootEpoch]; got != "epoch-claude-test" {
		t.Fatalf("%s header = %q, want epoch-claude-test; body=%s", mcp.HeaderBootEpoch, got, body)
	}
}

func TestInjectLaneMCPConfigClaudeOmitsBootEpochHeaderWhenUnset(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvMCPBootEpoch, "")
	cmd, cleanup, err := injectLaneMCPConfig(
		[]string{"claude"}, repo, "http://127.0.0.1:34135/mcp", TokenMaterial{Token: "dtok_secret"},
	)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	defer cleanup()
	body, _ := os.ReadFile(cmd[len(cmd)-2])
	if strings.Contains(string(body), mcp.HeaderBootEpoch) {
		t.Fatalf("boot epoch header present though env unset: %s", body)
	}
}

// #316: codex carries the boot epoch as a per-server http_headers override.
func TestInjectCodexMCPConfigArgsCarriesBootEpochHeader(t *testing.T) {
	repo := t.TempDir()
	t.Setenv(EnvMCPBootEpoch, "epoch-codex-test")
	cmd := InjectCodexMCPConfigArgs([]string{"codex"}, repo, "http://127.0.0.1:42727/mcp")
	want := fmt.Sprintf(`mcp_servers.striatum.http_headers.%q="epoch-codex-test"`, mcp.HeaderBootEpoch)
	if !strings.Contains(strings.Join(cmd, "\x00"), want) {
		t.Fatalf("codex injection missing boot-epoch header override %q: %#v", want, cmd)
	}
}

func TestInjectCodexMCPConfigArgsOmitsBootEpochHeaderWhenUnset(t *testing.T) {
	repo := t.TempDir()
	t.Setenv(EnvMCPBootEpoch, "")
	cmd := InjectCodexMCPConfigArgs([]string{"codex"}, repo, "http://127.0.0.1:42727/mcp")
	if strings.Contains(strings.Join(cmd, "\x00"), "http_headers") {
		t.Fatalf("codex injection added http_headers though epoch unset: %#v", cmd)
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

func TestCleanupGeminiSettings(t *testing.T) {
	t.Run("restores backup when legacy project settings exist", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, ".gemini")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		settingsPath := filepath.Join(dir, "settings.json")
		original := []byte(`{"original":true}`)
		if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
			t.Fatal(err)
		}

		backupPath := filepath.Join(repo, ".striatum", "scratch", "sup1", "settings.json.backup")
		if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(backupPath, original, 0o600); err != nil {
			t.Fatal(err)
		}

		// Modify settings to simulate crash/no cleanup called
		_ = os.WriteFile(settingsPath, []byte("garbage"), 0o600)

		// Call CleanupGeminiSettings directly
		CleanupGeminiSettings(repo, "sup1")

		// Verify settings restored
		restored, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(restored) != string(original) {
			t.Fatalf("expected restored = %s, got %s", original, restored)
		}

		// Verify backup file removed
		if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
			t.Fatalf("expected backup to be removed")
		}

		// Running cleanup after is safe and noop
		CleanupGeminiSettings(repo, "sup1")
	})

	t.Run("removes settings when legacy created marker exists", func(t *testing.T) {
		repo := t.TempDir()
		settingsPath := filepath.Join(repo, ".gemini", "settings.json")

		createdPath := filepath.Join(repo, ".striatum", "scratch", "sup2", "settings.json.created")
		if err := os.MkdirAll(filepath.Dir(createdPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(settingsPath, []byte(`{"created":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(createdPath, []byte{}, 0o600); err != nil {
			t.Fatal(err)
		}

		// Call CleanupGeminiSettings directly
		CleanupGeminiSettings(repo, "sup2")

		// Verify settings removed
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Fatalf("expected settings file to be removed")
		}

		// Verify marker file removed
		if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
			t.Fatalf("expected created marker to be removed")
		}

		// Running cleanup after is safe and noop
		CleanupGeminiSettings(repo, "sup2")
	})
}

// TestCleanupClaudeScheduledTasksLock is the #129 regression: a supervised
// Claude lane writes .claude/scheduled_tasks.lock into the target work tree, and
// lane teardown must remove that one file — but never other .claude/ contents,
// which may be operator config.
func TestCleanupClaudeScheduledTasksLock(t *testing.T) {
	repo := t.TempDir()
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(claudeDir, "scheduled_tasks.lock")
	if err := os.WriteFile(lockPath, []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	// An unrelated operator-owned .claude file that must be preserved.
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"operator":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	CleanupClaudeScheduledTasksLock(repo)

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("scheduled_tasks.lock was not removed: stat err = %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("teardown must not remove unrelated .claude contents: %v", err)
	}
	// Idempotent: a second call (or a missing file / empty root) is a safe no-op.
	CleanupClaudeScheduledTasksLock(repo)
	CleanupClaudeScheduledTasksLock("")
}
