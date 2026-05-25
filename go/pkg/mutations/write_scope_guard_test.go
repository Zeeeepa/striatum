package mutations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWriteScopeViolationsRejectsOutsideAndForbiddenPaths(t *testing.T) {
	got := writeScopeViolations(
		[]string{"docs/rfc-0050/design/codex/DESIGN.md", "go/pkg/mcp/capabilities.go", ".striatum/scratch/pid"},
		[]string{"docs/rfc-0050/design/codex/"},
		[]string{".striatum/"},
	)
	want := []string{".striatum/scratch/pid", "go/pkg/mcp/capabilities.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %#v, want %#v", got, want)
	}
}

func TestWriteScopeViolationsAllowsBroadScopeButStillHonorsForbidden(t *testing.T) {
	got := writeScopeViolations(
		[]string{"src/striatum/workflow.py", ".striatum/state"},
		[]string{"."},
		[]string{".striatum/"},
	)
	want := []string{".striatum/state"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %#v, want %#v", got, want)
	}
}

func TestParseGitPorcelainZIncludesRenameOldAndNewPaths(t *testing.T) {
	got := parseGitPorcelainZ([]byte("R  docs/new.md\x00docs/old.md\x00?? tests/new_test.py\x00"))
	want := []string{"docs/new.md", "docs/old.md", "tests/new_test.py"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestGitTouchedPathsSinceBaselineIgnoresPreExistingUntrackedPaths(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "allowed.txt"), "base\n")
	runGit(t, repo, "add", "allowed.txt")
	runGit(t, repo, "commit", "-m", "base")
	mustWrite(t, filepath.Join(repo, "outside-preexisting.txt"), "already here\n")

	baseline, err := gitChangedPathSnapshots(context.Background(), repo)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	entries := make([]any, 0, len(baseline))
	for _, item := range baseline {
		entries = append(entries, map[string]any{"path": item.Path, "hash": item.Hash})
	}

	mustWrite(t, filepath.Join(repo, "allowed.txt"), "changed\n")
	touched, err := gitTouchedPathsSinceBaseline(context.Background(), repo, map[string]any{
		"write_scope_baseline": map[string]any{"changed_paths": entries},
	})
	if err != nil {
		t.Fatalf("touched: %v", err)
	}
	want := []string{"allowed.txt"}
	if !reflect.DeepEqual(touched, want) {
		t.Fatalf("touched paths = %#v, want %#v", touched, want)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func mustWrite(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
