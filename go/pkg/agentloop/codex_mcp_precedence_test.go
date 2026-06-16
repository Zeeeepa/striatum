package agentloop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexConfigOverridePrecedenceAgainstSection is the #296 "table-stakes" live
// repro, captured as a hermetic, gated regression test.
//
// The daemon injects the live Striatum MCP endpoint into a codex lane as a
// per-key TOML override (`-c mcp_servers.striatum.url=<live>`, see
// InjectCodexMCPConfigArgs). The whole approach rests on an assertion that was
// previously *asserted but never tested*: that this launch-time `-c` override
// WINS over a pre-existing section-form `[mcp_servers.striatum]` block in the
// user's config.toml (which is exactly what goes stale across daemon restarts —
// the #296 bug). If codex ever changed its layering so a section beat a `-c`
// override, the injection would silently no-op against the stale port and the
// fix would regress invisibly.
//
// This test pins that behavior against the real codex CLI: it writes a config
// with a STALE section URL, asks codex to resolve the effective server while
// injecting the override (via the exact CodexMCPURLOverrideArg the daemon uses),
// and confirms the override URL — not the section URL — is what codex resolves.
//
// Gated on a codex binary being installed (CI without codex skips); hermetic via
// CODEX_HOME so it never reads or mutates the operator's ~/.codex/config.toml.
func TestCodexConfigOverridePrecedenceAgainstSection(t *testing.T) {
	codexBin, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("codex CLI not installed; skipping live -c precedence repro (#296)")
	}

	const staleURL = "http://127.0.0.1:11111/mcp"
	const liveURL = "http://127.0.0.1:22222/mcp"

	codexHome := t.TempDir()
	configBody := "[mcp_servers.striatum]\nurl = \"" + staleURL + "\"\nbearer_token_env_var = \"" + EnvMCPToken + "\"\n"
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(configBody), 0o600); err != nil {
		t.Fatalf("write hermetic codex config: %v", err)
	}

	// Sanity: with no override, codex resolves the (stale) section URL — proves the
	// section is actually loaded and the override below is doing real work.
	baseline := runCodexMCPGet(t, codexBin, codexHome /* no override */)
	if !strings.Contains(baseline, staleURL) {
		t.Fatalf("baseline codex mcp get did not resolve the section URL %q; got:\n%s", staleURL, baseline)
	}

	// With the daemon's exact override arg, the LIVE URL must win.
	override := runCodexMCPGet(t, codexBin, codexHome, "-c", CodexMCPURLOverrideArg(liveURL))
	if !strings.Contains(override, liveURL) {
		t.Fatalf("codex -c override did NOT win over the pre-existing [mcp_servers.striatum] section (#296 regression): want %q, got:\n%s", liveURL, override)
	}
	if strings.Contains(override, staleURL) {
		t.Fatalf("codex still resolved the stale section URL %q despite the -c override; got:\n%s", staleURL, override)
	}
}

func runCodexMCPGet(t *testing.T, codexBin, codexHome string, extraArgs ...string) string {
	t.Helper()
	args := append([]string{"mcp", "get", "striatum"}, extraArgs...)
	cmd := exec.Command(codexBin, args...)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+codexHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codex %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
