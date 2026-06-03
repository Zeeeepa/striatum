package agentloop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func claudeProjects(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	p, _ := cfg["projects"].(map[string]any)
	return p
}

// TestSeedClaudeWorkspaceTrustCreatesAndTrusts: against a missing config, the seed
// creates it and marks the workspace trusted (#163).
func TestSeedClaudeWorkspaceTrustCreatesAndTrusts(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	ws := "/tmp/some-fresh-repo"
	note, err := seedClaudeWorkspaceTrust(cfg, ws)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !strings.Contains(note, "seeded") {
		t.Fatalf("note = %q, want a 'seeded' note", note)
	}
	entry, _ := claudeProjects(t, cfg)[ws].(map[string]any)
	if entry == nil || entry["hasTrustDialogAccepted"] != true || entry["hasCompletedProjectOnboarding"] != true {
		t.Fatalf("workspace not trusted: %#v", entry)
	}
}

// TestSeedClaudeWorkspaceTrustPreservesExistingConfig: the read-modify-write
// preserves every other top-level key and every other project (incl. their
// subkeys) — it must not clobber the operator's live claude config.
func TestSeedClaudeWorkspaceTrustPreservesExistingConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	seed := map[string]any{
		"firstStartTime":         "2026-01-01",
		"hasCompletedOnboarding": true,
		"projects": map[string]any{
			"/home/op/other": map[string]any{"hasTrustDialogAccepted": true, "lastLinesAdded": float64(42)},
		},
	}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cfg, data, 0o600); err != nil {
		t.Fatal(err)
	}
	ws := "/tmp/fresh-repo"
	if _, err := seedClaudeWorkspaceTrust(cfg, ws); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out := map[string]any{}
	d, _ := os.ReadFile(cfg)
	if err := json.Unmarshal(d, &out); err != nil {
		t.Fatal(err)
	}
	if out["firstStartTime"] != "2026-01-01" || out["hasCompletedOnboarding"] != true {
		t.Fatalf("top-level keys lost: %#v", out)
	}
	projects := out["projects"].(map[string]any)
	other, _ := projects["/home/op/other"].(map[string]any)
	if other == nil || other["hasTrustDialogAccepted"] != true || other["lastLinesAdded"] != float64(42) {
		t.Fatalf("existing project clobbered: %#v", other)
	}
	ne, _ := projects[ws].(map[string]any)
	if ne == nil || ne["hasTrustDialogAccepted"] != true {
		t.Fatalf("new workspace not trusted: %#v", ne)
	}
}

// TestSeedClaudeWorkspaceTrustIdempotent: a second seed of an already-trusted
// workspace is a no-op (no rewrite), minimizing the clobber window on the shared
// config.
func TestSeedClaudeWorkspaceTrustIdempotent(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	ws := "/tmp/repo"
	if _, err := seedClaudeWorkspaceTrust(cfg, ws); err != nil {
		t.Fatalf("seed1: %v", err)
	}
	note, err := seedClaudeWorkspaceTrust(cfg, ws)
	if err != nil {
		t.Fatalf("seed2: %v", err)
	}
	if !strings.Contains(note, "already trusted") {
		t.Fatalf("second seed must be an idempotent no-op; note = %q", note)
	}
}

// TestSeedClaudeWorkspaceTrustDoesNotClobberCorruptConfig: a non-JSON config is
// claude's to own — the seed refuses and leaves it untouched.
func TestSeedClaudeWorkspaceTrustDoesNotClobberCorruptConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	const corrupt = "not json {{{"
	if err := os.WriteFile(cfg, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := seedClaudeWorkspaceTrust(cfg, "/tmp/repo"); err == nil {
		t.Fatal("expected an error for a corrupt config")
	}
	d, _ := os.ReadFile(cfg)
	if string(d) != corrupt {
		t.Fatalf("corrupt config was clobbered: %q", d)
	}
}

// TestEnsureClaudeWorkspaceTrustedUsesHomeConfig: the public entrypoint resolves
// HOME/.claude.json and trusts the absolute workspace path.
func TestEnsureClaudeWorkspaceTrustedUsesHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws := t.TempDir()
	if _, err := ensureClaudeWorkspaceTrusted(ws); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	abs, _ := filepath.Abs(ws)
	entry, _ := claudeProjects(t, filepath.Join(home, ".claude.json"))[abs].(map[string]any)
	if entry == nil || entry["hasTrustDialogAccepted"] != true {
		t.Fatalf("HOME-based config not trusted for %q: %#v", abs, entry)
	}
}
