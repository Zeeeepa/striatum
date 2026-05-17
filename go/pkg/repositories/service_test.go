package repositories

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasParentTraversalRejectsBeforeCleaning(t *testing.T) {
	if !hasParentTraversal("repo/../other") {
		t.Fatal("expected parent traversal to be rejected before filepath.Clean")
	}
	if hasParentTraversal("repo/subdir") {
		t.Fatal("ordinary repo path should not be rejected")
	}
}

func TestOperationalScratchInitCreatesScratchAndGitignore(t *testing.T) {
	repo := t.TempDir()

	stateDir, err := operationalScratch(repo, true)
	if err != nil {
		t.Fatalf("operationalScratch: %v", err)
	}

	if stateDir != filepath.Join(repo, ".striatum") {
		t.Fatalf("stateDir = %q", stateDir)
	}
	info, err := os.Stat(filepath.Join(stateDir, "scratch"))
	if err != nil {
		t.Fatalf("scratch missing: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("scratch is not a directory")
	}
	body, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(body), ".striatum/\n") {
		t.Fatalf(".gitignore missing .striatum entry: %q", string(body))
	}
}
