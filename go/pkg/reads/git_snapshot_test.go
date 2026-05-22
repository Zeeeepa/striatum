package reads

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type gitSnapshotFakeRunner struct {
	repoRoot string
}

func (r gitSnapshotFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("git.snapshot must be read-only")
}

func (r gitSnapshotFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return workflowAuthoringFakeRow{}
}

func (r gitSnapshotFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r gitSnapshotFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("git.snapshot must not open a transaction")
}

func (r gitSnapshotFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "FROM striatumd.repositories") {
		return nil, errors.New("unexpected query: " + sql)
	}
	if len(args) != 1 || args[0] != "repo_1" {
		return dashboardAllRowsFromMaps(nil), nil
	}
	return dashboardAllRowsFromMaps([]map[string]any{{"repo_root": r.repoRoot}}), nil
}

func TestHandleGitSnapshotReportsDirtyAndAncestry(t *testing.T) {
	requireGit(t)
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "seed\n")
	writeFile(t, filepath.Join(repo, "gone.txt"), "remove me\n")
	runGit(t, repo, "add", "README.md", "gone.txt")
	runGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")

	writeFile(t, filepath.Join(repo, "README.md"), "changed\n")
	writeFile(t, filepath.Join(repo, "staged.txt"), "staged\n")
	runGit(t, repo, "add", "staged.txt")
	if err := os.Remove(filepath.Join(repo, "gone.txt")); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}
	writeFile(t, filepath.Join(repo, "untracked.txt"), "new\n")

	result, err := HandleGitSnapshot(context.Background(), gitSnapshotFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":  "repo_1",
			"ancestry_limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("HandleGitSnapshot: %v", err)
	}
	if result["schema_version"] != gitSnapshotSchemaVersion || result["repository_id"] != "repo_1" {
		t.Fatalf("snapshot identity = %#v", result)
	}
	if result["git_available"] != true || result["is_git_repository"] != true {
		t.Fatalf("git availability = %#v", result)
	}
	branch := result["branch"].(map[string]any)
	if branch["name"] != "main" || branch["detached"] != false {
		t.Fatalf("branch = %#v", branch)
	}
	dirty := result["dirty"].(map[string]any)
	if dirty["is_dirty"] != true || dirty["staged"] != 1 || dirty["untracked"] != 1 || dirty["deleted"] != 1 {
		t.Fatalf("dirty = %#v", dirty)
	}
	paths := changedPathNames(result["changed_paths"].([]map[string]any))
	for _, expected := range []string{"README.md", "gone.txt", "staged.txt", "untracked.txt"} {
		if !containsString(paths, expected) {
			t.Fatalf("changed paths missing %s in %#v", expected, paths)
		}
	}
	head := result["head"].(map[string]any)
	if len(head["sha"].(string)) != 40 || head["subject"] != "seed" {
		t.Fatalf("head = %#v", head)
	}
	ancestry := result["ancestry"].(map[string]any)
	commits := ancestry["commits"].([]map[string]any)
	if ancestry["limit"] != 5 || len(commits) != 1 {
		t.Fatalf("ancestry = %#v", ancestry)
	}
}

func TestGitSnapshotCapsAncestryLimit(t *testing.T) {
	requireGit(t)
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "seed\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")

	result, err := HandleGitSnapshot(context.Background(), gitSnapshotFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":  "repo_1",
			"ancestry_limit": 999,
		},
	})
	if err != nil {
		t.Fatalf("HandleGitSnapshot: %v", err)
	}
	ancestry := result["ancestry"].(map[string]any)
	if ancestry["limit"] != maxGitAncestryLimit {
		t.Fatalf("ancestry limit = %#v", ancestry["limit"])
	}
}

func TestGitSnapshotRejectsNonGitDirectoryAsData(t *testing.T) {
	requireGit(t)
	result, err := HandleGitSnapshot(context.Background(), gitSnapshotFakeRunner{repoRoot: t.TempDir()}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	if err != nil {
		t.Fatalf("HandleGitSnapshot: %v", err)
	}
	if result["git_available"] != true || result["is_git_repository"] != false {
		t.Fatalf("non-git result = %#v", result)
	}
}

func TestParseGitStatusCoversRenameConflictAndDetachedHead(t *testing.T) {
	branch, changed, dirty, err := parseGitStatus("## HEAD (no branch)\nR  old.txt -> new.txt\nUU conflicted.txt\n")
	if err != nil {
		t.Fatalf("parseGitStatus: %v", err)
	}
	if branch["name"] != "HEAD" || branch["detached"] != true {
		t.Fatalf("branch = %#v", branch)
	}
	if len(changed) != 2 {
		t.Fatalf("changed = %#v", changed)
	}
	if changed[0]["kind"] != "renamed" || changed[0]["old_path"] != "old.txt" || changed[0]["path"] != "new.txt" {
		t.Fatalf("rename entry = %#v", changed[0])
	}
	if changed[1]["kind"] != "conflicted" || dirty["conflicted"] != 1 || dirty["renamed"] != 1 {
		t.Fatalf("conflict/dirty = changed %#v dirty %#v", changed, dirty)
	}
}

func TestGitSnapshotUsesOnlyReadOnlyGitCommands(t *testing.T) {
	repo := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-argv.log")
	fakeGit := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(fakeGit, []byte(`#!/bin/sh
printf '%s\n' "$*" >> "$GIT_ARGV_LOG"
case "$3" in
  rev-parse)
    printf 'true\n'
    ;;
  status)
    printf '## main\n M tracked.txt\n?? untracked.txt\n'
    ;;
  log)
    case "$6" in
      *%x1e)
        printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\037aaaaaaaaaaaa\037\0372026-05-22T00:00:00Z\0372026-05-22T00:00:00Z\037seed\036\n'
        ;;
      *)
        printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\037aaaaaaaaaaaa\0372026-05-22T00:00:00Z\0372026-05-22T00:00:00Z\037seed\n'
        ;;
    esac
    ;;
  *)
    exit 7
    ;;
esac
`), 0o700); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	oldGit := gitExecutable
	gitExecutable = fakeGit
	t.Setenv("GIT_ARGV_LOG", logPath)
	t.Cleanup(func() {
		gitExecutable = oldGit
	})

	_, err := HandleGitSnapshot(context.Background(), gitSnapshotFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	if err != nil {
		t.Fatalf("HandleGitSnapshot: %v", err)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git log: %v", err)
	}
	commands := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(commands) != 4 {
		t.Fatalf("commands = %#v", commands)
	}
	for _, command := range commands {
		if !strings.Contains(command, " rev-parse ") &&
			!strings.Contains(command, " status ") &&
			!strings.Contains(command, " log ") {
			t.Fatalf("unexpected git command: %q", command)
		}
		for _, forbidden := range []string{" fetch", " pull", " push", " remote", " ls-remote", " commit", " checkout", " switch", " merge", " rebase", " reset", " add", " restore", " tag", " gh", " glab", " curl", " ssh"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("mutating/network command used: %q", command)
			}
		}
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable not available")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "checkout", "-qb", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	return repo
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func changedPathNames(items []map[string]any) []string {
	paths := make([]string, 0, len(items))
	for _, item := range items {
		if path, ok := item["path"].(string); ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}
