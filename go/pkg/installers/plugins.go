package installers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const pluginsManifestSchema = "striatum.plugins.manifest.v1"

// PluginsAllProfilesOrder mirrors plugins.install.ALL_PROFILES_ORDER.
var PluginsAllProfilesOrder = []string{"claude_code", "agy", "codex"}

var pluginsAllowedProfiles = map[string]bool{
	"claude_code": true,
	"agy":         true,
	"codex":       true,
}

var pluginProfileSkills = []string{"workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp"}
var pluginProfileCommands = []string{"claim-next", "status", "why", "dashboard", "doctor"}

// PluginsParams configure a plugin install run.
type PluginsParams struct {
	Target          string
	Home            string
	Profile         string
	Scope           string
	Namespace       string
	Force           bool
	DryRun          bool
	WithMarketplace bool
	Version         string
}

// PluginsResult mirrors the dict returned by plugins.install.install.
type PluginsResult struct {
	Profile      string         `json:"profile"`
	Scope        string         `json:"scope"`
	Namespace    string         `json:"namespace"`
	BundleRoot   string         `json:"bundle_root"`
	ManifestPath string         `json:"manifest_path"`
	Files        []fileResult   `json:"files"`
	Marketplace  map[string]any `json:"marketplace"`
	DryRun       bool           `json:"dry_run"`
}

// InstallPlugin renders the plugin bundle for a single profile.
func InstallPlugin(p PluginsParams) (PluginsResult, error) {
	if !pluginsAllowedProfiles[p.Profile] {
		return PluginsResult{}, fmt.Errorf("unknown plugin profile %q; expected one of: claude_code, agy, codex", p.Profile)
	}
	if p.Scope != "project" && p.Scope != "user" {
		return PluginsResult{}, fmt.Errorf("unknown plugin scope %q; expected one of: project, user", p.Scope)
	}
	bundleRoot, err := pluginBundleRoot(p.Target, p.Home, p.Profile, p.Scope, p.Namespace)
	if err != nil {
		return PluginsResult{}, err
	}
	ctx := renderContext{StriatumVersion: p.Version, Namespace: p.Namespace}
	plan, err := pluginPlan(p.Profile, p.Namespace, ctx)
	if err != nil {
		return PluginsResult{}, err
	}
	manifestPath := filepath.Join(bundleRoot, ".manifest.json")
	files, err := runPipeline(pipelineParams{
		BaseDir:       bundleRoot,
		ManifestPath:  manifestPath,
		SchemaVersion: pluginsManifestSchema,
		Version:       p.Version,
		Profile:       p.Profile,
		Namespace:     p.Namespace,
		Scope:         p.Scope,
		Plan:          plan,
		Force:         p.Force,
		DryRun:        p.DryRun,
	})
	if err != nil {
		return PluginsResult{}, err
	}

	var marketplace map[string]any
	if p.WithMarketplace && !p.DryRun && p.Scope == "project" {
		if p.Profile == "agy" {
			marketplace = map[string]any{"skipped": true, "reason": "agy has no marketplace concept"}
		} else {
			target, err := filepath.Abs(p.Target)
			if err != nil {
				return PluginsResult{}, err
			}
			marketplace, err = writeMarketplace(target, p.Profile, p.Namespace)
			if err != nil {
				return PluginsResult{}, err
			}
		}
	}

	return PluginsResult{
		Profile:      p.Profile,
		Scope:        p.Scope,
		Namespace:    p.Namespace,
		BundleRoot:   bundleRoot,
		ManifestPath: manifestPath,
		Files:        files,
		Marketplace:  marketplace,
		DryRun:       p.DryRun,
	}, nil
}

// PluginUninstallResult mirrors the dict returned by plugins.install.uninstall.
type PluginUninstallResult struct {
	Profile    string           `json:"profile"`
	Status     string           `json:"status"`
	BundleRoot string           `json:"bundle_root,omitempty"`
	Removed    []map[string]any `json:"removed"`
	Refused    []map[string]any `json:"refused,omitempty"`
}

// UninstallPlugin removes a previously installed bundle by reading its manifest.
func UninstallPlugin(p PluginsParams) (PluginUninstallResult, error) {
	if !pluginsAllowedProfiles[p.Profile] {
		return PluginUninstallResult{}, fmt.Errorf("unknown plugin profile %q; expected one of: claude_code, agy, codex", p.Profile)
	}
	bundleRoot, err := pluginBundleRoot(p.Target, p.Home, p.Profile, p.Scope, p.Namespace)
	if err != nil {
		return PluginUninstallResult{}, err
	}
	manifestPath := filepath.Join(bundleRoot, ".manifest.json")
	doc := loadManifestAny(manifestPath)
	if doc == nil {
		return PluginUninstallResult{Profile: p.Profile, Status: "not_installed", Removed: []map[string]any{}}, nil
	}
	removed := []map[string]any{}
	refused := []map[string]any{}
	for _, entry := range doc.Files {
		absolute := filepath.Join(bundleRoot, filepath.FromSlash(entry.Path))
		data, err := os.ReadFile(absolute)
		if err != nil {
			continue
		}
		if sha256Hex(data) != entry.SHA256 && !p.Force {
			refused = append(refused, map[string]any{"path": entry.Path, "reason": "modified"})
			continue
		}
		if err := os.Remove(absolute); err != nil {
			return PluginUninstallResult{}, err
		}
		removed = append(removed, map[string]any{"path": entry.Path})
	}
	status := "uninstalled"
	if len(refused) > 0 {
		status = "refused"
	} else {
		_ = os.Remove(manifestPath)
		removeEmptyDirs(bundleRoot)
	}
	return PluginUninstallResult{
		Profile:    p.Profile,
		Status:     status,
		BundleRoot: bundleRoot,
		Removed:    removed,
		Refused:    refused,
	}, nil
}

func pluginBundleRoot(target, home, profile, scope, namespace string) (string, error) {
	if scope == "user" {
		base := home
		if base == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = h
		}
		abs, err := filepath.Abs(base)
		if err != nil {
			return "", err
		}
		switch profile {
		case "claude_code":
			return filepath.Join(abs, ".claude", "plugins", namespace), nil
		case "agy":
			return filepath.Join(abs, ".agy", "plugins", namespace), nil
		case "codex":
			return filepath.Join(abs, ".codex", "plugins", namespace), nil
		}
		return "", fmt.Errorf("user-scope path undefined for profile %q", profile)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".striatum", "plugins", profile), nil
}

func pluginPlan(profile, namespace string, ctx renderContext) ([]planEntry, error) {
	var specs []struct{ tmpl, out string }
	switch profile {
	case "claude_code", "agy":
		// RFC 0088 Decision 4: agy is claude-shaped (`agy plugin import claude`
		// works wholesale), so the agy installer reuses the claude_code template
		// tree rather than authoring a parallel one. The bundle root differs
		// (see pluginBundleRoot) but the in-bundle layout is identical.
		specs = append(specs, entrySpec("claude_code/plugin.json.tmpl", ".claude-plugin/plugin.json"))
		for _, skill := range pluginProfileSkills {
			specs = append(specs, entrySpec(fmt.Sprintf("claude_code/skills/%s.md.tmpl", skill), fmt.Sprintf("skills/%s-%s/SKILL.md", namespace, skill)))
		}
		for _, cmd := range pluginProfileCommands {
			specs = append(specs, entrySpec(fmt.Sprintf("claude_code/commands/%s.md.tmpl", cmd), fmt.Sprintf("commands/%s.md", cmd)))
		}
		specs = append(specs, entrySpec("claude_code/hooks/hooks.json.tmpl", "hooks/hooks.json"))
		specs = append(specs, entrySpec("claude_code/mcp.json.tmpl", ".mcp.json"))
		specs = append(specs, entrySpec("claude_code/README.md.tmpl", "README.md"))
	case "codex":
		specs = append(specs, entrySpec("codex/plugin.json.tmpl", ".codex-plugin/plugin.json"))
		for _, skill := range pluginProfileSkills {
			specs = append(specs, entrySpec(fmt.Sprintf("codex/skills/%s.md.tmpl", skill), fmt.Sprintf("skills/%s-%s/SKILL.md", namespace, skill)))
		}
		for _, cmd := range pluginProfileCommands {
			specs = append(specs, entrySpec(fmt.Sprintf("codex/commands/%s.md.tmpl", cmd), fmt.Sprintf("commands/%s.md", cmd)))
		}
		specs = append(specs, entrySpec("codex/hooks/hooks.json.tmpl", "hooks/hooks.json"))
		specs = append(specs, entrySpec("codex/mcp.json.tmpl", ".mcp.json"))
		specs = append(specs, entrySpec("codex/README.md.tmpl", "README.md"))
	default:
		return nil, fmt.Errorf("unsupported profile %q", profile)
	}

	plan := make([]planEntry, 0, len(specs))
	for _, spec := range specs {
		raw, err := readPluginTemplate(spec.tmpl)
		if err != nil {
			return nil, err
		}
		rendered, err := render(raw, ctx)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", spec.tmpl, err)
		}
		plan = append(plan, planEntry{
			Path:           spec.out,
			Rendered:       rendered,
			Template:       spec.tmpl,
			TemplateSHA256: sha256Hex([]byte(raw)),
		})
	}
	return plan, nil
}

func entrySpec(tmpl, out string) struct{ tmpl, out string } {
	return struct{ tmpl, out string }{tmpl, out}
}

// writeMarketplace mirrors plugins.install._write_marketplace.
func writeMarketplace(target, profile, namespace string) (map[string]any, error) {
	path := filepath.Join(target, ".striatum", "plugins", "marketplace.json")
	existing := map[string]any{
		"name":      "local-striatum",
		"interface": map[string]any{"displayName": "Local Striatum"},
		"plugins":   []any{},
	}
	if data, err := os.ReadFile(path); err == nil {
		var loaded map[string]any
		if json.Unmarshal(data, &loaded) == nil {
			existing = loaded
		}
	}
	plugins, _ := existing["plugins"].([]any)
	newEntry := map[string]any{
		"name":     namespace,
		"source":   map[string]any{"source": "local", "path": "./" + profile},
		"policy":   map[string]any{"installation": "AVAILABLE"},
		"category": "Developer Tools",
	}
	matched := false
	for i, raw := range plugins {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		src, _ := p["source"].(map[string]any)
		if p["name"] == namespace && src != nil && src["path"] == "./"+profile {
			plugins[i] = newEntry
			matched = true
			break
		}
	}
	if !matched {
		plugins = append(plugins, newEntry)
	}
	existing["plugins"] = plugins
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return nil, err
	}
	result := map[string]any{"path": path}
	if matched {
		result["updated"] = true
	} else {
		result["appended"] = true
	}
	return result, nil
}

func loadManifestAny(path string) *manifestDoc {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc manifestDoc
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	return &doc
}

func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepathWalkDirs(root, &dirs)
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d)
	}
	_ = os.Remove(root)
}

func filepathWalkDirs(root string, dirs *[]string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(root, e.Name())
			*dirs = append(*dirs, sub)
			_ = filepathWalkDirs(sub, dirs)
		}
	}
	return nil
}
