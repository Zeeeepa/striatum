package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSkillsInstallJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "install", "--profile", "claude_code", "--scope", "user", "--force", "--json"}, &stdout, &stderr, "", "7.7.7")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not JSON: %v (%s)", err, stdout.String())
	}
	if payload["ok"] != true {
		t.Fatalf("ok = %#v", payload["ok"])
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "striatum-workflow", "SKILL.md")); err != nil {
		t.Fatalf("skill not installed: %v", err)
	}
}

func TestRunSkillsInstallAllJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "install", "--profile", "all", "--scope", "user", "--force", "--json"}, &stdout, &stderr, "", "1.0.0")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].(map[string]any)
	if data["profile"] != "all" {
		t.Fatalf("profile = %v", data["profile"])
	}
}

func TestRunPluginInstallProjectJSON(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plugin", "install", "--profile", "claude_code", "--scope", "project", "--no-marketplace", "--json"}, &stdout, &stderr, target, "5.0.0")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(target, ".striatum", "plugins", "claude_code", ".claude-plugin", "plugin.json")); err != nil {
		t.Fatalf("plugin bundle not written: %v", err)
	}
}

func TestRunUnknownProfileErrorsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "install", "--profile", "bogus", "--scope", "user", "--json"}, &stdout, &stderr, "", "1")
	if code == 0 {
		t.Fatal("expected nonzero exit")
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != false {
		t.Fatalf("ok = %#v", payload["ok"])
	}
}
