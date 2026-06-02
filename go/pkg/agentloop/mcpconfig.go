package agentloop

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// geminiSettingsRelPath is the work-tree-relative path of the agy MCP settings
// file. It carries a rotating bearer token and must never enter git provenance.
const geminiSettingsRelPath = ".gemini/settings.json"

// geminiExcludeMarker tags the line we append to the work tree's local git
// exclude so teardown can remove EXACTLY our line without disturbing any
// operator-authored excludes (#70 / RFC 0096 §3).
const geminiExcludeMarker = " # striatum-agy-lane (RFC 0096 #70): ephemeral MCP bearer, never commit"

// injectLaneMCPConfig gives an agent-loop lane CLI a striatum MCP server
// pointed at the live, env-resolved endpoint + token (RFC 0088 Decision 5).
// The config is generated FRESH at launch into an ephemeral 0600 file under
// the target repo's .striatum scratch and removed when the lane exits — the
// rotating endpoint is never persisted to a repo-tracked or gitignored file,
// which makes the F45 stale-port class structurally impossible.
//
// P1 wires claude (the baseline): `--mcp-config <file> --strict-mcp-config`
// loads ONLY the striatum server and ignores any stale global ~/.claude.json
// entry. P2 extends the same shape to agy (claude-shaped CLI: supports
// `--mcp-config <configs...>` and `agy plugin import claude`). P3 wires codex
// via a TOML override flag (codex has no --mcp-config; it overrides ~/.codex/
// config.toml per-key with `-c key=value`); the bearer is read by codex from
// the STRIATUM_MCP_TOKEN env var supervisedEnv already provides.
func injectLaneMCPConfig(command []string, repoRoot, endpoint string, token TokenMaterial) ([]string, func(), error) {
	noop := func() {}
	if len(command) == 0 || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(token.Token) == "" {
		return command, noop, nil
	}
	switch LaneAdapterName(command[0]) {
	case "claude":
		// claude accepts `--mcp-config <file> --strict-mcp-config` and loads
		// ONLY the striatum server from the ephemeral file, ignoring any stale
		// global config entry.
		path, cleanup, err := writeEphemeralMCPConfig(repoRoot, endpoint, token.Token)
		if err != nil {
			return command, noop, err
		}
		out := append([]string(nil), command...)
		out = append(out, "--mcp-config", path, "--strict-mcp-config")
		return out, cleanup, nil
	case "agy":
		// agy (Antigravity) has NO --mcp-config flag; passing claude-shaped
		// flags makes it print usage and exit. It reads MCP servers from a
		// project-level .gemini/settings.json (gemini `httpUrl` schema). Write
		// that settings file fresh per launch and remove/restore it on teardown
		// so no stale rotating endpoint persists (RFC 0088 Decision 5 / the F45
		// footgun). The command itself is unchanged.
		cleanup, err := writeEphemeralGeminiSettings(repoRoot, endpoint, token.Token)
		if err != nil {
			return command, noop, err
		}
		return command, cleanup, nil
	case "codex":
		// Codex stores the striatum MCP server in ~/.codex/config.toml; the
		// bearer is read from the STRIATUM_MCP_TOKEN env var (supervisedEnv
		// provides it). Only the rotating URL needs overriding at launch —
		// `-c mcp_servers.striatum.url="<endpoint>"` overrides config.toml
		// without persisting (Decision 5: never persist the rotating port).
		out := append([]string(nil), command...)
		out = append(out, "-c", fmt.Sprintf(`mcp_servers.striatum.url=%q`, endpoint))
		return out, noop, nil
	default:
		return command, noop, nil
	}
}

// LaneAdapterName maps a (possibly absolute) lane argv0 to its bare adapter
// name (e.g. "/home/u/.local/bin/claude" → "claude"). It is the single canonical
// argv→adapter mapping the agent-loop wiring uses; the mutations package reuses
// it to scope per-adapter supervised-lane env hardening (see
// supervisedAdapterEnvEntries / #101) so the daemon and the agent-loop agree on
// adapter identity from one source.
func LaneAdapterName(arg0 string) string {
	base := filepath.Base(strings.TrimSpace(arg0))
	base = strings.TrimSuffix(base, ".exe")
	return base
}

// writeEphemeralGeminiSettings writes the rotating striatum MCP endpoint into a
// project-level .gemini/settings.json (the only MCP-config surface agy reads,
// since it has no --mcp-config flag), merging into any existing project
// settings and restoring/removing the file on teardown. Fresh-per-launch +
// teardown means no stale rotating port is ever persisted (RFC 0088 Decision 5).
//
// #70 / RFC 0096 §3 (OQ3): the bearer token lives inside the target repo work
// tree here, which we would prefer to relocate outside the write surface. agy
// (Antigravity) only reads <cwd>/.gemini/settings.json or ~/.gemini/ — it has
// no out-of-repo settings-path flag or env var (D150: revisit only if its
// config surface changes). The user-global ~/.gemini is shared across all
// repos/lanes and unsafe to clobber per-launch, so the per-repo file remains
// the only viable agy MCP surface in Phase 1. We therefore keep the repo path
// but GUARANTEE removal on every teardown path: the cleanup is centralized at
// the supervisor terminal-state transition (mutations.updateSupervisorState →
// CleanupGeminiSettings) so graceful exit, supervise stop, and tmux kill/lost
// all restore/remove the file. Codex and claude never persist a repo token
// (codex reads STRIATUM_MCP_TOKEN; claude uses an ephemeral --mcp-config under
// .striatum/scratch), so this repo-tree token is agy-only.
func writeEphemeralGeminiSettings(repoRoot, endpoint, bearer string) (func(), error) {
	noop := func() {}
	if strings.TrimSpace(repoRoot) == "" {
		return noop, fmt.Errorf("repository root is empty")
	}
	dir := filepath.Join(repoRoot, ".gemini")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return noop, fmt.Errorf("create .gemini dir: %w", err)
	}
	path := filepath.Join(dir, "settings.json")
	var backup []byte
	hadExisting := false
	if b, err := os.ReadFile(path); err == nil {
		backup = b
		hadExisting = true
	}
	settings := map[string]any{}
	if hadExisting {
		// Preserve any existing project settings (auth, trust, etc.); only the
		// striatum mcpServers entry is ours.
		_ = json.Unmarshal(backup, &settings)
	}
	settings["mcpServers"] = map[string]any{
		"striatum": map[string]any{
			"httpUrl": endpoint,
			"headers": map[string]any{"Authorization": "Bearer " + bearer},
		},
	}
	// #76: a supervised agy lane must not stall on the gemini-cli usage/feedback
	// survey ("How's the CLI experience? [1] Good ...") inside the PTY while a
	// work packet is active. usageStatisticsEnabled:false is the documented
	// gemini-cli key that disables usage statistics and the periodic feedback
	// prompt; set it unless the project already declares it. Additive and
	// harmless if a given agy build ignores the key.
	if _, declared := settings["usageStatisticsEnabled"]; !declared {
		settings["usageStatisticsEnabled"] = false
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return noop, err
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return noop, fmt.Errorf("write .gemini/settings.json: %w", err)
	}
	// #70 / RFC 0096 §3: keep the bearer-bearing settings file out of git
	// provenance for the lane's lifetime — invisible to `git status`, to
	// work-scope baselines, and to `git add -A` — so a token can never be swept
	// into a commit. Removed again on every teardown path below.
	excludeGeminiSettingsFromGit(repoRoot)

	supervisorID := os.Getenv("STRIATUM_SUPERVISOR_ID")
	var scratchDir string
	if supervisorID != "" {
		scratchDir = filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)
		_ = os.MkdirAll(scratchDir, 0o755)
		if hadExisting {
			_ = os.WriteFile(filepath.Join(scratchDir, "settings.json.backup"), backup, 0o600)
		} else {
			_ = os.WriteFile(filepath.Join(scratchDir, "settings.json.created"), []byte{}, 0o600)
		}
	}

	cleanup := func() {
		unexcludeGeminiSettingsFromGit(repoRoot)
		if supervisorID != "" {
			_ = os.Remove(filepath.Join(scratchDir, "settings.json.backup"))
			_ = os.Remove(filepath.Join(scratchDir, "settings.json.created"))
		}
		if hadExisting {
			_ = os.WriteFile(path, backup, 0o600)
			return
		}
		_ = os.Remove(path)
	}
	return cleanup, nil
}

// gitInfoExcludePath resolves the active info/exclude path for repoRoot,
// correctly handling linked worktrees (where .git is a file and info/exclude
// lives in the common git dir). Returns "" when repoRoot is not a git work tree
// or git is unavailable.
func gitInfoExcludePath(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--git-path", "info/exclude")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	return p
}

// excludeGeminiSettingsFromGit appends .gemini/settings.json to the work tree's
// local git exclude so the ephemeral bearer file never appears in `git status`,
// never dirties a write-scope baseline, and is never picked up by `git add -A`
// (#70 / RFC 0096 §3). Best-effort + idempotent: a non-git target, an existing
// exclusion, or an unwritable exclude is a no-op (teardown removal is the
// backstop).
func excludeGeminiSettingsFromGit(repoRoot string) {
	path := gitInfoExcludePath(repoRoot)
	if path == "" {
		return
	}
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			t := strings.TrimSpace(line)
			if t == geminiSettingsRelPath || strings.HasPrefix(t, geminiSettingsRelPath+" ") {
				return // already excluded (ours or the operator's)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(geminiSettingsRelPath + geminiExcludeMarker + "\n")
}

// unexcludeGeminiSettingsFromGit removes ONLY the marker-tagged line that
// excludeGeminiSettingsFromGit added, restoring the operator's prior exclude
// exactly. A pre-existing operator exclusion (no marker) is left untouched.
// Best-effort + idempotent.
func unexcludeGeminiSettingsFromGit(repoRoot string) {
	path := gitInfoExcludePath(repoRoot)
	if path == "" {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	kept := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if strings.Contains(line, geminiExcludeMarker) {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return
	}
	_ = os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}

// CleanupGeminiSettings locates the scratch directory and restores the pre-existing settings or deletes the created settings.
func CleanupGeminiSettings(repoRoot, supervisorID string) {
	if supervisorID == "" || repoRoot == "" {
		return
	}
	// #70 / RFC 0096 §3: drop the local git exclusion we added at launch (no-op
	// if absent), regardless of whether the settings file itself is restored or
	// removed below.
	unexcludeGeminiSettingsFromGit(repoRoot)
	scratchDir := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)
	backupPath := filepath.Join(scratchDir, "settings.json.backup")
	createdPath := filepath.Join(scratchDir, "settings.json.created")
	settingsPath := filepath.Join(repoRoot, ".gemini", "settings.json")

	if b, err := os.ReadFile(backupPath); err == nil {
		_ = os.WriteFile(settingsPath, b, 0o600)
		_ = os.Remove(backupPath)
	} else if _, err := os.Stat(createdPath); err == nil {
		_ = os.Remove(settingsPath)
		_ = os.Remove(createdPath)
	}
}

// CleanupClaudeScheduledTasksLock removes the ephemeral
// .claude/scheduled_tasks.lock that a supervised Claude lane writes into the
// target work tree (#129, mirroring the #62 .gemini/settings.json cleanup). Lane
// teardown must not leave this operational lock file dirtying the work tree.
//
// Conservative by design: it removes ONLY that one lock file — never broader
// .claude/ contents, which may be the operator's own Claude config. Best-effort
// and idempotent (a missing repo root or absent file is a no-op), so it is safe
// to call on every terminal supervisor transition, and from any teardown path.
func CleanupClaudeScheduledTasksLock(repoRoot string) {
	if repoRoot == "" {
		return
	}
	lockPath := filepath.Join(repoRoot, ".claude", "scheduled_tasks.lock")
	_ = os.Remove(lockPath)
}

func writeEphemeralMCPConfig(repoRoot, endpoint, bearer string) (string, func(), error) {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"striatum": map[string]any{
				"type":    "http",
				"url":     endpoint,
				"headers": map[string]any{"Authorization": "Bearer " + bearer},
			},
		},
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return "", func() {}, err
	}
	dir := filepath.Join(repoRoot, ".striatum", "scratch")
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "lane-mcp-config-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("create ephemeral mcp config: %w", err)
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}
