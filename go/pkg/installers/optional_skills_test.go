package installers

import (
	"os"
	"path/filepath"
	"testing"
)

// #177: the installer embeds Striatum-authored optional skills under
// templates/optional. AvailableOptionalSkills must surface refactoring-campaign
// and must never surface the third-party suggestions (adhd, supabase), which
// are not embedded and stay suggest-only.
func TestAvailableOptionalSkillsExcludesThirdParty(t *testing.T) {
	names, err := AvailableOptionalSkills()
	if err != nil {
		t.Fatalf("AvailableOptionalSkills: %v", err)
	}
	if !containsString(names, "refactoring-campaign") {
		t.Fatalf("refactoring-campaign missing from available optional skills: %v", names)
	}
	for _, banned := range []string{"adhd", "supabase-postgres-best-practices"} {
		if containsString(names, banned) {
			t.Fatalf("third-party skill %q must not be embedded/available: %v", banned, names)
		}
	}
}

func TestInstallOptionalSkillUnknownNameErrors(t *testing.T) {
	_, err := InstallOptionalSkill(OptionalSkillsParams{
		Target: t.TempDir(),
		Name:   "no-such-skill",
		Scope:  "project",
	})
	if err == nil {
		t.Fatal("expected error for unknown optional skill")
	}
}

// The embedded optional-skill bundle must stay byte-identical to its canonical
// source at skills/optional/<name>/, so installs ship the maintained content.
func TestEmbeddedOptionalSkillMatchesCanonicalSource(t *testing.T) {
	repoRoot := repositoryRoot(t)
	files, err := optionalSkillFiles("refactoring-campaign")
	if err != nil {
		t.Fatalf("optionalSkillFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no embedded files for refactoring-campaign")
	}
	for _, rel := range files {
		embeddedBody, err := readOptionalSkillFile(filepath.ToSlash(filepath.Join("refactoring-campaign", rel)))
		if err != nil {
			t.Fatalf("read embedded %s: %v", rel, err)
		}
		canonical := filepath.Join(repoRoot, "skills", "optional", "refactoring-campaign", rel)
		canonicalBody, err := os.ReadFile(canonical)
		if err != nil {
			t.Fatalf("read canonical %s: %v", canonical, err)
		}
		if string(embeddedBody) != string(canonicalBody) {
			t.Errorf("embedded optional skill file %s drifted from canonical source %s; re-copy skills/optional/refactoring-campaign/ into go/pkg/installers/templates/optional/refactoring-campaign/", rel, canonical)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "go")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}
		dir = parent
	}
}
