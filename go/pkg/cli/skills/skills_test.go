package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// #177: `skills install --optional <name>` renders a Striatum-authored
// optional skill, manifest-tracked like the core bundle.
func TestRunOptionalSkillInstallRendersAndTracks(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "install", "--optional", "refactoring-campaign", "--scope", "project", "--json"}, &stdout, &stderr, target, "9.9.9")
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
	skillDir := filepath.Join(target, ".claude", "skills", "striatum-refactoring-campaign")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "scripts", "instantiate.sh")); err != nil {
		t.Fatalf("scripts/instantiate.sh not installed: %v", err)
	}
	// Manifest records the install so re-install/upgrade work.
	manifest := filepath.Join(skillDir, ".manifest.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	if !strings.Contains(string(body), "striatum.optional_skill.manifest.v1") {
		t.Fatalf("manifest missing schema version: %s", string(body))
	}
}

// #177: an unknown optional skill name errors legibly.
func TestRunOptionalSkillInstallUnknownNameErrors(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "install", "--optional", "does-not-exist", "--scope", "project", "--json"}, &stdout, &stderr, target, "1")
	if code == 0 {
		t.Fatal("expected nonzero exit for unknown optional skill")
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not JSON: %v (%s)", err, stdout.String())
	}
	if payload["ok"] != false {
		t.Fatalf("ok = %#v", payload["ok"])
	}
	msg, _ := payload["error"].(map[string]any)["message"].(string)
	if !strings.Contains(msg, "unknown optional skill") {
		t.Fatalf("error message not legible: %q", msg)
	}
}

// #177: third-party suggestions (adhd, supabase) are NOT rendered by the
// installer — they stay suggest-only per skills/optional/README.md.
func TestRunOptionalSkillInstallRefusesThirdParty(t *testing.T) {
	target := t.TempDir()
	for _, thirdParty := range []string{"adhd", "supabase-postgres-best-practices"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"skills", "install", "--optional", thirdParty, "--scope", "project", "--json"}, &stdout, &stderr, target, "1")
		if code == 0 {
			t.Fatalf("%s: expected refusal, got success", thirdParty)
		}
		if _, err := os.Stat(filepath.Join(target, ".claude", "skills", "striatum-"+thirdParty)); !os.IsNotExist(err) {
			t.Fatalf("%s: third-party skill was rendered", thirdParty)
		}
	}
}

// #177: `skills list` reports available optional skills and installed state.
func TestRunOptionalSkillsList(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skills", "list", "--scope", "project", "--json"}, &stdout, &stderr, target, "1")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not JSON: %v (%s)", err, stdout.String())
	}
	data := payload["data"].(map[string]any)
	list, _ := data["optional_skills"].([]any)
	foundCampaign := false
	for _, item := range list {
		entry := item.(map[string]any)
		if entry["name"] == "refactoring-campaign" {
			foundCampaign = true
			if entry["installed"] != false {
				t.Fatalf("refactoring-campaign should not be installed yet: %#v", entry)
			}
		}
		// Third-party skills must never appear as installable optional skills.
		if entry["name"] == "adhd" || entry["name"] == "supabase-postgres-best-practices" {
			t.Fatalf("third-party skill listed as installable: %#v", entry)
		}
	}
	if !foundCampaign {
		t.Fatalf("refactoring-campaign missing from optional skills list: %s", stdout.String())
	}

	// After install, list reports it installed.
	var instOut, instErr bytes.Buffer
	if c := Run([]string{"skills", "install", "--optional", "refactoring-campaign", "--scope", "project", "--json"}, &instOut, &instErr, target, "1"); c != 0 {
		t.Fatalf("install exit = %d, stderr = %s", c, instErr.String())
	}
	var listOut, listErr bytes.Buffer
	if c := Run([]string{"skills", "list", "--scope", "project", "--json"}, &listOut, &listErr, target, "1"); c != 0 {
		t.Fatalf("list exit = %d, stderr = %s", c, listErr.String())
	}
	var listPayload map[string]any
	if err := json.Unmarshal(listOut.Bytes(), &listPayload); err != nil {
		t.Fatal(err)
	}
	entries := listPayload["data"].(map[string]any)["optional_skills"].([]any)
	installed := false
	for _, item := range entries {
		entry := item.(map[string]any)
		if entry["name"] == "refactoring-campaign" && entry["installed"] == true {
			installed = true
		}
	}
	if !installed {
		t.Fatalf("refactoring-campaign not reported installed after install: %s", listOut.String())
	}
}

func TestBareSkillsAndPluginShowUsage(t *testing.T) {
	tests := []struct {
		args     []string
		exitCode int
		stream   string
		want     string
	}{
		{args: []string{"skills"}, exitCode: 2, stream: "stderr", want: "usage: striatum skills install"},
		{args: []string{"skills", "--help"}, exitCode: 0, stream: "stdout", want: "usage: striatum skills install"},
		{args: []string{"plugin"}, exitCode: 2, stream: "stderr", want: "usage: striatum plugin {install|uninstall}"},
		{args: []string{"plugin", "--help"}, exitCode: 0, stream: "stdout", want: "usage: striatum plugin {install|uninstall}"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := Run(tt.args, &stdout, &stderr, "", "1")
		if code != tt.exitCode {
			t.Fatalf("%v exit = %d, stderr = %s", tt.args, code, stderr.String())
		}
		got := stdout.String()
		if tt.stream == "stderr" {
			got = stderr.String()
		}
		if !strings.Contains(got, tt.want) {
			t.Fatalf("%v output missing %q; stdout=%q stderr=%q", tt.args, tt.want, stdout.String(), stderr.String())
		}
	}
}
