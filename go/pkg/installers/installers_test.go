package installers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillsClaudeCode(t *testing.T) {
	home := t.TempDir()
	params := SkillsParams{Home: home, Profile: "claude_code", Scope: "user", Namespace: "striatum-", Version: "9.9.9"}
	result, err := InstallSkills(params)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != len(claudeCodeSkills) {
		t.Fatalf("expected %d files, got %d", len(claudeCodeSkills), len(result.Files))
	}
	for _, f := range result.Files {
		if f.Status != "written" {
			t.Fatalf("fresh install file %s status = %s", f.Path, f.Status)
		}
	}
	// Every skill file exists and carries the rendered version stamp.
	workflowPath := filepath.Join(home, ".claude", "skills", "striatum-workflow", "SKILL.md")
	body, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "9.9.9") {
		t.Fatalf("rendered skill missing version stamp")
	}
	if strings.Contains(string(body), "{striatum_version}") || strings.Contains(string(body), "{verbs_") {
		t.Fatalf("rendered skill has unexpanded placeholders")
	}
	claimLoopPath := filepath.Join(home, ".claude", "skills", "striatum-claim-loop", "SKILL.md")
	claimLoop, err := os.ReadFile(claimLoopPath)
	if err != nil {
		t.Fatal(err)
	}
	claimLoopText := string(claimLoop)
	for _, want := range []string{"striatum.finding.v1", "verdict_intent:", "striatum.collaboration_ledger.v1", "expected_artifacts[].author_line", "lease.heartbeat_after_seconds"} {
		if !strings.Contains(claimLoopText, want) {
			t.Fatalf("rendered claim-loop skill missing %q:\n%s", want, claimLoopText)
		}
	}
	if strings.Contains(claimLoopText, "{front_matter_schema_skeletons}") {
		t.Fatalf("rendered claim-loop skill has unexpanded schema placeholder")
	}

	// Manifest carries the version stamp doctor reads.
	doc := loadManifest(result.ManifestPath, skillsManifestSchema)
	if doc == nil {
		t.Fatalf("manifest not written/parseable at %s", result.ManifestPath)
	}
	if doc.StriatumVersion != "9.9.9" {
		t.Fatalf("manifest version = %s", doc.StriatumVersion)
	}
	if doc.Profile != "claude_code" || doc.Scope != "user" {
		t.Fatalf("manifest meta mismatch: %+v", doc)
	}
}

func TestInstallSkillsIdempotent(t *testing.T) {
	home := t.TempDir()
	params := SkillsParams{Home: home, Profile: "claude_code", Scope: "user", Namespace: "striatum-", Version: "1.2.3"}
	if _, err := InstallSkills(params); err != nil {
		t.Fatal(err)
	}
	result, err := InstallSkills(params)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Files {
		if f.Status != "skipped_unchanged" {
			t.Fatalf("reinstall file %s status = %s, want skipped_unchanged", f.Path, f.Status)
		}
	}
}

func TestInstallSkillsRefusesOperatorEditWithoutForce(t *testing.T) {
	home := t.TempDir()
	params := SkillsParams{Home: home, Profile: "claude_code", Scope: "user", Namespace: "striatum-", Version: "1.0.0"}
	if _, err := InstallSkills(params); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(home, ".claude", "skills", "striatum-workflow", "SKILL.md")
	if err := os.WriteFile(edited, []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := InstallSkills(params)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Files {
		if strings.HasSuffix(f.Path, "striatum-workflow/SKILL.md") {
			found = true
			if f.Status != "refused_modified" {
				t.Fatalf("edited file status = %s, want refused_modified", f.Status)
			}
		}
	}
	if !found {
		t.Fatal("workflow skill not in results")
	}
	if body, _ := os.ReadFile(edited); string(body) != "operator edit\n" {
		t.Fatal("refused file was overwritten")
	}

	// With force, the operator edit is overwritten.
	forced := params
	forced.Force = true
	if _, err := InstallSkills(forced); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(edited); string(body) == "operator edit\n" {
		t.Fatal("force did not overwrite operator edit")
	}
}

func TestInstallSkillsAllFansOut(t *testing.T) {
	home := t.TempDir()
	result, err := InstallSkillsAll(SkillsParams{Home: home, Scope: "user", Namespace: "striatum-", Version: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != len(SkillsAllProfilesOrder) {
		t.Fatalf("expected %d profile results, got %d", len(SkillsAllProfilesOrder), len(result.Results))
	}
	for i, prof := range SkillsAllProfilesOrder {
		if result.Results[i].Profile != prof {
			t.Fatalf("result %d profile = %s, want %s", i, result.Results[i].Profile, prof)
		}
	}
}

func TestInstallSkillsCleansStalePeerScopeManifest(t *testing.T) {
	home := t.TempDir()
	target := t.TempDir()
	if _, err := InstallSkills(SkillsParams{
		Target: target, Home: home, Profile: "claude_code", Scope: "project", Namespace: "striatum-", Version: "1.48.2",
	}); err != nil {
		t.Fatal(err)
	}
	staleSkill := filepath.Join(target, ".claude", "skills", "striatum-workflow", "SKILL.md")
	staleManifest := filepath.Join(target, ".claude", "skills", "striatum-workflow", ".manifest.json")
	if _, err := os.Stat(staleSkill); err != nil {
		t.Fatalf("stale project skill missing before cleanup: %v", err)
	}

	result, err := InstallSkills(SkillsParams{
		Target: target, Home: home, Profile: "claude_code", Scope: "user", Namespace: "striatum-", Version: "2.24.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cleanups) == 0 {
		t.Fatal("expected stale peer-scope cleanup results")
	}
	if _, err := os.Stat(staleSkill); !os.IsNotExist(err) {
		t.Fatalf("stale project skill still present: %v", err)
	}
	if _, err := os.Stat(staleManifest); !os.IsNotExist(err) {
		t.Fatalf("stale project manifest still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "striatum-workflow", "SKILL.md")); err != nil {
		t.Fatalf("user skill not installed: %v", err)
	}
}

func TestCheckSkillsHealthFlagsManifestAndLooseHeaderSkew(t *testing.T) {
	home := t.TempDir()
	target := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := InstallSkills(SkillsParams{
		Target: target, Home: home, Profile: "claude_code", Scope: "project", Namespace: "striatum-", Version: "2.8.0",
	}); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(home, ".claude", "skills", "striatum-legacy", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(loose), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loose, []byte("<!-- Generated by striatum 1.48.2. Regenerate with `striatum skills install`. -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	health, err := CheckSkillsHealth(SkillsHealthParams{Target: target, Home: home, CurrentVersion: "2.24.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(health.Items) != 2 {
		t.Fatalf("health items = %#v", health.Items)
	}
	warnings := strings.Join(SkillsHealthWarnings(health), "\n")
	for _, want := range []string{"skills_outdated", "2.8.0", "1.48.2", "skills install"} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q:\n%s", want, warnings)
		}
	}
}

func TestInstallSkillsUnknownProfile(t *testing.T) {
	_, err := InstallSkills(SkillsParams{Home: t.TempDir(), Profile: "nope", Scope: "user", Version: "1"})
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestInstallPluginClaudeCode(t *testing.T) {
	target := t.TempDir()
	result, err := InstallPlugin(PluginsParams{
		Target: target, Profile: "claude_code", Scope: "project",
		Namespace: "striatum", WithMarketplace: true, Version: "3.1.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	// plugin.json + 6 skills + 5 commands + hooks + mcp + README = 15
	if len(result.Files) != 15 {
		t.Fatalf("expected 15 plugin files, got %d", len(result.Files))
	}
	pluginJSON := filepath.Join(result.BundleRoot, ".claude-plugin", "plugin.json")
	body, err := os.ReadFile(pluginJSON)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("plugin.json not valid JSON (brace escaping?): %v", err)
	}
	if parsed["version"] != "3.1.4" {
		t.Fatalf("plugin.json version = %v", parsed["version"])
	}
	if parsed["name"] != "striatum" {
		t.Fatalf("plugin.json name = %v", parsed["name"])
	}
	// Marketplace written under project scope.
	if result.Marketplace == nil {
		t.Fatal("expected marketplace result")
	}
	mkt := filepath.Join(target, ".striatum", "plugins", "marketplace.json")
	marketplaceBody, err := os.ReadFile(mkt)
	if err != nil {
		t.Fatalf("marketplace.json missing: %v", err)
	}
	var marketplace map[string]any
	if err := json.Unmarshal(marketplaceBody, &marketplace); err != nil {
		t.Fatalf("marketplace.json not valid JSON: %v", err)
	}
	owner, ok := marketplace["owner"].(map[string]any)
	if !ok || owner["name"] != "Striatum" {
		t.Fatalf("marketplace owner = %#v", marketplace["owner"])
	}
	readme, err := os.ReadFile(filepath.Join(result.BundleRoot, "README.md"))
	if err != nil {
		t.Fatalf("README.md missing: %v", err)
	}
	if strings.Contains(string(readme), "/plugin install ./.striatum/plugins/claude_code") {
		t.Fatalf("README still contains invalid direct path install:\n%s", string(readme))
	}
	if !strings.Contains(string(readme), "/plugin marketplace add ./.striatum/plugins/marketplace.json") {
		t.Fatalf("README missing marketplace file install path:\n%s", string(readme))
	}
}

func TestPluginUninstallRoundTrip(t *testing.T) {
	target := t.TempDir()
	params := PluginsParams{Target: target, Profile: "claude_code", Scope: "project", Namespace: "striatum", WithMarketplace: false, Version: "1.0.0"}
	if _, err := InstallPlugin(params); err != nil {
		t.Fatal(err)
	}
	result, err := UninstallPlugin(params)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "uninstalled" {
		t.Fatalf("status = %s", result.Status)
	}
	if _, err := os.Stat(result.BundleRoot); !os.IsNotExist(err) {
		t.Fatalf("bundle root still present: %v", err)
	}
}

func TestPluginAgySkipsMarketplace(t *testing.T) {
	target := t.TempDir()
	result, err := InstallPlugin(PluginsParams{Target: target, Profile: "agy", Scope: "project", Namespace: "striatum", WithMarketplace: true, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Marketplace["skipped"] != true {
		t.Fatalf("agy marketplace not skipped: %+v", result.Marketplace)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	home := t.TempDir()
	result, err := InstallSkills(SkillsParams{Home: home, Profile: "claude_code", Scope: "user", Namespace: "striatum-", DryRun: true, Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Files {
		if f.Status != "dry_run" {
			t.Fatalf("dry-run file status = %s", f.Status)
		}
	}
	if _, err := os.Stat(result.ManifestPath); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote a manifest")
	}
}

func TestFormatMapBraceEscaping(t *testing.T) {
	out, err := formatMap("{{\"v\": \"{striatum_version}\"}}", map[string]string{"striatum_version": "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"v": "1.0"}` {
		t.Fatalf("got %q", out)
	}
	if _, err := formatMap("{unknown_key}", map[string]string{}); err == nil {
		t.Fatal("expected error for unknown key")
	}
}
