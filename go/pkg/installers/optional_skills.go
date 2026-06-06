package installers

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
)

const optionalSkillsManifestSchema = "striatum.optional_skill.manifest.v1"

// OptionalSkillsParams configure an optional-skill install run (#177).
type OptionalSkillsParams struct {
	Target    string // target repo root (project scope)
	Home      string // user-scope base (defaults to the user's home)
	Name      string // optional skill bundle name (e.g. refactoring-campaign)
	Scope     string // project | user
	Namespace string
	Force     bool
	DryRun    bool
	Version   string
}

// OptionalSkillResult mirrors the per-install result for an optional skill.
type OptionalSkillResult struct {
	Name         string       `json:"name"`
	Scope        string       `json:"scope"`
	Namespace    string       `json:"namespace"`
	SkillDir     string       `json:"skill_dir"`
	ManifestPath string       `json:"manifest_path"`
	Files        []fileResult `json:"files"`
	DryRun       bool         `json:"dry_run"`
}

// OptionalSkillListEntry describes one available optional skill and whether it
// is currently installed in the requested scope.
type OptionalSkillListEntry struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	SkillDir  string `json:"skill_dir"`
}

// AvailableOptionalSkills returns the Striatum-authored optional skill names
// that `skills install --optional <name>` can render. Third-party suggestions
// (e.g. adhd, supabase) are intentionally not embedded and never appear here —
// they remain suggest-only per skills/optional/README.md.
func AvailableOptionalSkills() ([]string, error) {
	names, err := optionalSkillNames()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// ListOptionalSkills reports the available optional skills and their installed
// state in the given scope.
func ListOptionalSkills(p OptionalSkillsParams) ([]OptionalSkillListEntry, error) {
	if p.Scope != "project" && p.Scope != "user" {
		return nil, fmt.Errorf("unknown skills scope %q; expected one of: project, user", p.Scope)
	}
	root, err := scopeBase(p.Target, p.Home, p.Scope)
	if err != nil {
		return nil, err
	}
	names, err := AvailableOptionalSkills()
	if err != nil {
		return nil, err
	}
	entries := make([]OptionalSkillListEntry, 0, len(names))
	for _, name := range names {
		dir := optionalSkillDir(root, p.Namespace, name)
		manifestPath := filepath.Join(dir, ".manifest.json")
		installed := loadManifest(manifestPath, optionalSkillsManifestSchema) != nil
		entries = append(entries, OptionalSkillListEntry{
			Name:      name,
			Installed: installed,
			SkillDir:  dir,
		})
	}
	return entries, nil
}

// InstallOptionalSkill renders one Striatum-authored optional skill bundle into
// the user/project skills directory, manifest-tracked exactly like the core
// bundle so re-install/upgrade and edit-detection work. Bundle files are copied
// verbatim (they are authored content with literal braces), version-stamped via
// the manifest.
func InstallOptionalSkill(p OptionalSkillsParams) (OptionalSkillResult, error) {
	if p.Scope != "project" && p.Scope != "user" {
		return OptionalSkillResult{}, fmt.Errorf("unknown skills scope %q; expected one of: project, user", p.Scope)
	}
	available, err := AvailableOptionalSkills()
	if err != nil {
		return OptionalSkillResult{}, err
	}
	if !containsString(available, p.Name) {
		return OptionalSkillResult{}, fmt.Errorf("unknown optional skill %q; available: %s (third-party suggestions stay suggest-only — see skills/optional/README.md)", p.Name, joinOrNone(available))
	}
	root, err := scopeBase(p.Target, p.Home, p.Scope)
	if err != nil {
		return OptionalSkillResult{}, err
	}
	files, err := optionalSkillFiles(p.Name)
	if err != nil {
		return OptionalSkillResult{}, err
	}
	if len(files) == 0 {
		return OptionalSkillResult{}, fmt.Errorf("optional skill %q has no embedded files", p.Name)
	}
	sort.Strings(files)

	skillDirName := p.Namespace + p.Name
	plan := make([]planEntry, 0, len(files))
	for _, rel := range files {
		raw, err := readOptionalSkillFile(path.Join(p.Name, rel))
		if err != nil {
			return OptionalSkillResult{}, err
		}
		// Bundle files are copied verbatim (no format_map render).
		dest := path.Join(".claude", "skills", skillDirName, rel)
		plan = append(plan, planEntry{
			Path:           dest,
			Rendered:       string(raw),
			Template:       path.Join("optional", p.Name, rel),
			TemplateSHA256: sha256Hex(raw),
		})
	}

	manifestPath := filepath.Join(optionalSkillDir(root, p.Namespace, p.Name), ".manifest.json")
	results, err := runPipeline(pipelineParams{
		BaseDir:       root,
		ManifestPath:  manifestPath,
		SchemaVersion: optionalSkillsManifestSchema,
		Version:       p.Version,
		Profile:       "optional:" + p.Name,
		Namespace:     p.Namespace,
		Scope:         p.Scope,
		Plan:          plan,
		Force:         p.Force,
		DryRun:        p.DryRun,
	})
	if err != nil {
		return OptionalSkillResult{}, err
	}
	return OptionalSkillResult{
		Name:         p.Name,
		Scope:        p.Scope,
		Namespace:    p.Namespace,
		SkillDir:     optionalSkillDir(root, p.Namespace, p.Name),
		ManifestPath: manifestPath,
		Files:        results,
		DryRun:       p.DryRun,
	}, nil
}

func optionalSkillDir(root, namespace, name string) string {
	return filepath.Join(root, ".claude", "skills", namespace+name)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += value
	}
	return out
}
