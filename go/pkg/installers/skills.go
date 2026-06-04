package installers

import (
	"fmt"
	"path/filepath"
)

const skillsManifestSchema = "striatum.skills.manifest.v1"

// SkillsAllProfilesOrder mirrors striatum.skills.install.ALL_PROFILES_ORDER:
// the stable fan-out order for `--profile all`.
var SkillsAllProfilesOrder = []string{"claude_code", "agy", "codex", "generic"}

var skillsAllowedProfiles = map[string]bool{
	"claude_code": true,
	"agy":         true,
	"codex":       true,
	"generic":     true,
}

// claudeCodeSkills mirrors CLAUDE_CODE_SKILLS — bare skill names.
var claudeCodeSkills = []string{"workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp"}

// SkillsParams configure a skills install run.
type SkillsParams struct {
	Target    string // target repo root (project scope)
	Home      string // user-scope base (defaults to the user's home)
	Profile   string
	Scope     string
	Namespace string
	Force     bool
	DryRun    bool
	Version   string
}

// SkillsResult mirrors the dict returned by skills.install.install.
type SkillsResult struct {
	Profile      string       `json:"profile"`
	Scope        string       `json:"scope"`
	Namespace    string       `json:"namespace"`
	ManifestPath string       `json:"manifest_path"`
	Files        []fileResult `json:"files"`
	Cleanups     []fileResult `json:"cleanups,omitempty"`
	DryRun       bool         `json:"dry_run"`
}

// SkillsAllResult mirrors the `--profile all` fan-out dict.
type SkillsAllResult struct {
	Profile   string         `json:"profile"`
	Scope     string         `json:"scope"`
	Namespace string         `json:"namespace"`
	Results   []SkillsResult `json:"results"`
	DryRun    bool           `json:"dry_run"`
}

// InstallSkills renders the skill bundle for a single profile.
func InstallSkills(p SkillsParams) (SkillsResult, error) {
	if !skillsAllowedProfiles[p.Profile] {
		return SkillsResult{}, fmt.Errorf("unknown skills profile %q; expected one of: claude_code, agy, codex, generic", p.Profile)
	}
	if p.Scope != "project" && p.Scope != "user" {
		return SkillsResult{}, fmt.Errorf("unknown skills scope %q; expected one of: project, user", p.Scope)
	}
	root, err := scopeBase(p.Target, p.Home, p.Scope)
	if err != nil {
		return SkillsResult{}, err
	}
	plan, err := skillsPlan(p.Profile, p.Namespace, renderContext{StriatumVersion: p.Version, Namespace: p.Namespace})
	if err != nil {
		return SkillsResult{}, err
	}
	cleanups, err := cleanupStalePeerSkillBundles(p)
	if err != nil {
		return SkillsResult{}, err
	}
	manifestPath := skillsManifestPath(root, p.Profile, p.Namespace)
	files, err := runPipeline(pipelineParams{
		BaseDir:       root,
		ManifestPath:  manifestPath,
		SchemaVersion: skillsManifestSchema,
		Version:       p.Version,
		Profile:       p.Profile,
		Namespace:     p.Namespace,
		Scope:         p.Scope,
		Plan:          plan,
		Force:         p.Force,
		DryRun:        p.DryRun,
	})
	if err != nil {
		return SkillsResult{}, err
	}
	return SkillsResult{
		Profile:      p.Profile,
		Scope:        p.Scope,
		Namespace:    p.Namespace,
		ManifestPath: manifestPath,
		Files:        files,
		Cleanups:     cleanups,
		DryRun:       p.DryRun,
	}, nil
}

// InstallSkillsAll fans out across SkillsAllProfilesOrder.
func InstallSkillsAll(p SkillsParams) (SkillsAllResult, error) {
	out := SkillsAllResult{Profile: "all", Scope: p.Scope, Namespace: p.Namespace, DryRun: p.DryRun}
	for _, profile := range SkillsAllProfilesOrder {
		sub := p
		sub.Profile = profile
		result, err := InstallSkills(sub)
		if err != nil {
			return SkillsAllResult{}, err
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

func skillsPlan(profile, namespace string, ctx renderContext) ([]planEntry, error) {
	switch profile {
	case "claude_code":
		return skillsSkillPlan(namespace, ctx, func(skill string) string {
			return fmt.Sprintf(".claude/skills/%s%s/SKILL.md", namespace, skill)
		})
	case "agy":
		// agy reuses the claude_code skill template bodies wholesale (RFC 0088
		// Decision 4 — "agy plugin import claude" works wholesale). Only the
		// destination differs: agy stores skills under .agy/.
		return skillsSkillPlan(namespace, ctx, func(skill string) string {
			return fmt.Sprintf(".agy/skills/%s%s/SKILL.md", namespace, skill)
		})
	case "codex":
		return skillsSkillPlan(namespace, ctx, func(skill string) string {
			return fmt.Sprintf(".codex/agents/%s%s.md", namespace, skill)
		})
	case "generic":
		return skillsSingle("generic/STRIATUM_AGENT_GUIDE.md.tmpl", namespace+"STRIATUM_AGENT_GUIDE.md", ctx)
	}
	return nil, fmt.Errorf("unsupported profile %q", profile)
}

// skillsSkillPlan builds the per-skill plan; claude_code and codex share the
// same template bodies (codex only differs in destination layout).
func skillsSkillPlan(namespace string, ctx renderContext, dest func(skill string) string) ([]planEntry, error) {
	plan := make([]planEntry, 0, len(claudeCodeSkills))
	for _, skill := range claudeCodeSkills {
		templateRel := fmt.Sprintf("claude_code/%s.md.tmpl", skill)
		entry, err := skillsEntry(templateRel, dest(skill), ctx)
		if err != nil {
			return nil, err
		}
		plan = append(plan, entry)
	}
	return plan, nil
}

func skillsSingle(templateRel, outputRel string, ctx renderContext) ([]planEntry, error) {
	entry, err := skillsEntry(templateRel, outputRel, ctx)
	if err != nil {
		return nil, err
	}
	return []planEntry{entry}, nil
}

func skillsEntry(templateRel, outputRel string, ctx renderContext) (planEntry, error) {
	raw, err := readSkillTemplate(templateRel)
	if err != nil {
		return planEntry{}, err
	}
	rendered, err := render(raw, ctx)
	if err != nil {
		return planEntry{}, fmt.Errorf("template %q: %w", templateRel, err)
	}
	return planEntry{
		Path:           outputRel,
		Rendered:       rendered,
		Template:       templateRel,
		TemplateSHA256: sha256Hex([]byte(raw)),
	}, nil
}

func skillsManifestPath(root, profile, namespace string) string {
	switch profile {
	case "claude_code":
		return filepath.Join(root, ".claude", "skills", namespace+"workflow", ".manifest.json")
	case "agy":
		return filepath.Join(root, ".agy", "skills", namespace+"workflow", ".manifest.json")
	case "codex":
		return filepath.Join(root, ".codex", "agents", namespace+"workflow.manifest.json")
	case "generic":
		return filepath.Join(root, namespace+"STRIATUM_AGENT_GUIDE.manifest.json")
	}
	return filepath.Join(root, ".manifest.json")
}
