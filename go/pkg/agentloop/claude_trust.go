package agentloop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ensureClaudeWorkspaceTrusted pre-accepts claude's per-folder workspace-trust
// dialog for repoRoot in the operator's ~/.claude.json (#163).
//
// claude 2.1.x prompts "Is this a project you trust?" the first time it runs in a
// directory and PARKS an interactive (PTY) lane there waiting for Enter/Esc.
// `--dangerously-skip-permissions` does NOT bypass it (that gates tool permissions,
// not workspace trust), and claude only skips the trust dialog for already-trusted
// directories or non-interactive (`-p`) mode — neither of which applies to a
// supervised agent-loop lane (it runs claude interactively in a PTY). So without
// this, a claude lane on a fresh target repo silently wedges before it can claim
// its first packet (the #163 / #148-class park).
//
// Trust is durable and per-folder, keyed by the absolute workspace path under
// `projects` in ~/.claude.json — exactly the operator action of trusting the repo
// once. This seed is idempotent: it only rewrites the config when the entry is not
// already trusted, so the shared config is written at most once per fresh repo
// (minimizing any last-writer-wins clobber window against claude's own writes). It
// is best-effort: the returned error/note is for the caller to log; the lane launch
// proceeds regardless (a failed seed degrades to the pre-fix park, never worse).
func ensureClaudeWorkspaceTrusted(repoRoot string) (string, error) {
	if repoRoot == "" {
		return "", nil
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		abs = repoRoot
	}
	return seedClaudeWorkspaceTrust(claudeConfigPath(), abs)
}

// claudeConfigPath resolves the operator's claude config file (HOME/.claude.json).
func claudeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude.json")
}

// seedClaudeWorkspaceTrust performs the read-modify-write against an explicit
// config path (separated from path resolution so it is hermetically testable).
func seedClaudeWorkspaceTrust(configPath, workspace string) (string, error) {
	if configPath == "" {
		return "", fmt.Errorf("cannot resolve claude config path (no home dir)")
	}
	config := map[string]any{}
	if data, rerr := os.ReadFile(configPath); rerr == nil {
		if jerr := json.Unmarshal(data, &config); jerr != nil {
			// A corrupt / non-JSON config is claude's to own; do not clobber it.
			return "", fmt.Errorf("claude config is not valid JSON, not seeding trust: %w", jerr)
		}
	} else if !os.IsNotExist(rerr) {
		return "", rerr
	}

	projects, _ := config["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	entry, _ := projects[workspace].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	if entry["hasTrustDialogAccepted"] == true && entry["hasCompletedProjectOnboarding"] == true {
		return "already trusted: " + workspace, nil // idempotent: no write
	}
	entry["hasTrustDialogAccepted"] = true
	entry["hasCompletedProjectOnboarding"] = true
	projects[workspace] = entry
	config["projects"] = projects

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	tmp := fmt.Sprintf("%s.striatum-trust.%d.tmp", configPath, os.Getpid())
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, configPath); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return "seeded workspace trust: " + workspace, nil
}
