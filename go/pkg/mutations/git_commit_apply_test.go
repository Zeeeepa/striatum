package mutations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type gitCommitApplyFakeRunner struct {
	repoRoot string
}

func (r gitCommitApplyFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("git.commit_apply must not write daemon tables")
}

func (r gitCommitApplyFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: errors.New("unexpected row query")}
}

func (r gitCommitApplyFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r gitCommitApplyFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("git.commit_apply must not open a transaction")
}

func (r gitCommitApplyFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "FROM striatumd.repositories") {
		return nil, errors.New("unexpected query: " + sql)
	}
	if len(args) != 1 || args[0] != "repo_1" {
		return runPrepareRowsFromMaps(nil), nil
	}
	return runPrepareRowsFromMaps([]map[string]any{{"repo_root": r.repoRoot}}), nil
}

func TestHandleGitCommitApplyCreatesLocalCommitFromConfirmedRequest(t *testing.T) {
	commitApplyRequireGit(t)
	repo := initCommitApplyGitRepo(t)
	writeCommitApplyFile(t, repo, "README.md", "seed\n")
	runCommitApplyGit(t, repo, "add", "README.md")
	runCommitApplyGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")
	baseHead := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}")

	writeCommitApplyFile(t, repo, "docs/out.md", "result\n")
	writeCommitRequest(t, repo, "docs/commit-request.md", baseHead, "operator_confirmed", []string{
		"docs/out.md",
		"docs/commit-request.md",
	})

	result, err := HandleGitCommitApply(context.Background(), gitCommitApplyFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":       "repo_1",
			"schema_version":      1,
			"commit_request_path": "docs/commit-request.md",
			"confirm":             true,
			"confirm_request_id":  "commit_req_0001",
		},
	})
	if err != nil {
		t.Fatalf("HandleGitCommitApply: %v", err)
	}
	commitSHA := fmt.Sprint(result["commit_sha"])
	if len(commitSHA) != 40 || commitSHA == baseHead {
		t.Fatalf("commit_sha = %#v base=%s", result["commit_sha"], baseHead)
	}
	if result["schema_version"] != gitCommitApplySchemaVersion ||
		result["request_id"] != "commit_req_0001" ||
		result["confirmation_status"] != "operator_confirmed" ||
		result["local_commit_created"] != true ||
		result["pushed"] != false ||
		result["hosted_provider_invoked"] != false ||
		result["provider_credentials_loaded"] != false ||
		result["hooks_disabled"] != true {
		t.Fatalf("unexpected result = %#v", result)
	}
	if subject := commitApplyGitOutput(t, repo, "log", "-1", "--format=%s"); subject != "Apply confirmed local commit" {
		t.Fatalf("commit subject = %q", subject)
	}
	if status := commitApplyGitOutput(t, repo, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("status after commit = %q", status)
	}
}

func TestGitCommitApplyRequiresExplicitConfirmationBeforeCommit(t *testing.T) {
	commitApplyRequireGit(t)
	repo := initCommitApplyGitRepo(t)
	writeCommitApplyFile(t, repo, "README.md", "seed\n")
	runCommitApplyGit(t, repo, "add", "README.md")
	runCommitApplyGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")
	baseHead := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}")
	writeCommitApplyFile(t, repo, "docs/out.md", "result\n")
	writeCommitRequest(t, repo, "docs/commit-request.md", baseHead, "operator_confirmed", []string{"docs/out.md", "docs/commit-request.md"})

	_, err := HandleGitCommitApply(context.Background(), gitCommitApplyFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":       "repo_1",
			"commit_request_path": "docs/commit-request.md",
			"confirm_request_id":  "commit_req_0001",
		},
	})
	assertRPCCode(t, err, "confirmation_required")
	if head := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}"); head != baseHead {
		t.Fatalf("HEAD changed without confirmation: got %s want %s", head, baseHead)
	}
}

func TestGitCommitApplyRequiresConfirmedRequestArtifact(t *testing.T) {
	commitApplyRequireGit(t)
	repo := initCommitApplyGitRepo(t)
	writeCommitApplyFile(t, repo, "README.md", "seed\n")
	runCommitApplyGit(t, repo, "add", "README.md")
	runCommitApplyGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")
	baseHead := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}")
	writeCommitApplyFile(t, repo, "docs/out.md", "result\n")
	writeCommitRequest(t, repo, "docs/commit-request.md", baseHead, "pending", []string{"docs/out.md", "docs/commit-request.md"})

	_, err := HandleGitCommitApply(context.Background(), gitCommitApplyFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":       "repo_1",
			"commit_request_path": "docs/commit-request.md",
			"confirm":             true,
			"confirm_request_id":  "commit_req_0001",
		},
	})
	assertRPCCode(t, err, "confirmation_required")
	if head := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}"); head != baseHead {
		t.Fatalf("HEAD changed from unconfirmed request: got %s want %s", head, baseHead)
	}
}

func TestGitCommitApplyRefusesDirtyPathsOutsideIncludedPaths(t *testing.T) {
	commitApplyRequireGit(t)
	repo := initCommitApplyGitRepo(t)
	writeCommitApplyFile(t, repo, "README.md", "seed\n")
	runCommitApplyGit(t, repo, "add", "README.md")
	runCommitApplyGit(t, repo, "commit", "-m", "seed", "--no-gpg-sign")
	baseHead := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}")
	writeCommitApplyFile(t, repo, "docs/out.md", "result\n")
	writeCommitApplyFile(t, repo, "extra.txt", "outside\n")
	writeCommitRequest(t, repo, "docs/commit-request.md", baseHead, "operator_confirmed", []string{"docs/out.md", "docs/commit-request.md"})

	_, err := HandleGitCommitApply(context.Background(), gitCommitApplyFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":       "repo_1",
			"commit_request_path": "docs/commit-request.md",
			"confirm":             true,
			"confirm_request_id":  "commit_req_0001",
		},
	})
	assertRPCCode(t, err, "dirty_tree_outside_commit_request")
	if head := commitApplyGitOutput(t, repo, "rev-parse", "--verify", "HEAD^{commit}"); head != baseHead {
		t.Fatalf("HEAD changed despite dirty-path refusal: got %s want %s", head, baseHead)
	}
}

func TestGitCommitApplyUsesNoPushProviderOrHooks(t *testing.T) {
	repo := t.TempDir()
	baseHead := strings.Repeat("a", 40)
	writeCommitRequest(t, repo, "docs/commit-request.md", baseHead, "operator_confirmed", []string{"docs/out.md", "docs/commit-request.md"})
	logPath := filepath.Join(t.TempDir(), "git-argv.log")
	statePath := filepath.Join(t.TempDir(), "committed")
	fakeGit := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(fakeGit, []byte(`#!/bin/sh
printf '%s\n' "$*" >> "$GIT_ARGV_LOG"
cmd=""
for arg in "$@"; do
  case "$arg" in
    rev-parse|status|add|commit|fetch|pull|push|remote|ls-remote|gh|glab|hub|curl|ssh)
      cmd="$arg"
      break
      ;;
  esac
done
case "$cmd" in
  rev-parse)
    case "$*" in
      *"--is-inside-work-tree"*) printf 'true\n' ;;
      *"--abbrev-ref"*) printf 'main\n' ;;
      *"--short=12"*) printf 'bbbbbbbbbbbb\n' ;;
      *)
        if [ -f "$FAKE_GIT_COMMITTED" ]; then
          printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n'
        else
          printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'
        fi
        ;;
    esac
    ;;
  status)
    printf ' M docs/out.md\000?? docs/commit-request.md\000'
    ;;
  add)
    ;;
  commit)
    touch "$FAKE_GIT_COMMITTED"
    ;;
  *)
    exit 9
    ;;
esac
`), 0o700); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	oldGit := gitCommitExecutable
	gitCommitExecutable = fakeGit
	t.Setenv("GIT_ARGV_LOG", logPath)
	t.Setenv("FAKE_GIT_COMMITTED", statePath)
	t.Cleanup(func() {
		gitCommitExecutable = oldGit
	})

	result, err := HandleGitCommitApply(context.Background(), gitCommitApplyFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{
			"repository_id":       "repo_1",
			"commit_request_path": "docs/commit-request.md",
			"confirm":             true,
			"confirm_request_id":  "commit_req_0001",
		},
	})
	if err != nil {
		t.Fatalf("HandleGitCommitApply: %v", err)
	}
	if result["commit_sha"] != strings.Repeat("b", 40) || result["hooks_disabled"] != true {
		t.Fatalf("fake commit result = %#v", result)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git log: %v", err)
	}
	commands := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(commands) == 0 {
		t.Fatal("fake git was not called")
	}
	sawCommitWithHooksDisabled := false
	for _, command := range commands {
		for _, forbidden := range []string{" fetch", " pull", " push", " remote", " ls-remote", " gh", " glab", " hub", " curl", " ssh"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("network/provider command used: %q", command)
			}
		}
		if strings.Contains(command, " commit ") && strings.Contains(command, " core.hooksPath=") {
			sawCommitWithHooksDisabled = true
		}
	}
	if !sawCommitWithHooksDisabled {
		t.Fatalf("commit did not set core.hooksPath in commands: %#v", commands)
	}
}

func commitApplyRequireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable not available")
	}
}

func initCommitApplyGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runCommitApplyGit(t, repo, "init", "-q")
	runCommitApplyGit(t, repo, "checkout", "-qb", "main")
	runCommitApplyGit(t, repo, "config", "user.email", "test@example.com")
	runCommitApplyGit(t, repo, "config", "user.name", "Test User")
	return repo
}

func runCommitApplyGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func commitApplyGitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func writeCommitApplyFile(t *testing.T, repo string, rel string, body string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func writeCommitRequest(t *testing.T, repo string, rel string, baseHead string, status string, includedPaths []string) {
	t.Helper()
	quoted := make([]string, 0, len(includedPaths))
	for _, path := range includedPaths {
		quoted = append(quoted, fmt.Sprintf("%q", path))
	}
	body := fmt.Sprintf(`---
schema_version: "striatum.commit_request.v1"
artifact_kind: "commit_request"
request_id: "commit_req_0001"
base_head: %q
branch: "main"
git_snapshot_hash: "sha256:test"
included_paths: [%s]
reviewed_artifacts: ["docs/review.md"]
commit_message: "Apply confirmed local commit"
rationale: "Test confirmed local commit apply."
confirmation_status: %q
confirmed_by: "operator"
confirmed_at: "2026-05-23T00:00:00Z"
---

# Commit Request
`, baseHead, strings.Join(quoted, ", "), status)
	writeCommitApplyFile(t, repo, rel, body)
}

func assertRPCCode(t *testing.T, err error, code string) {
	t.Helper()
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error %s", err, code)
	}
	if rpcErr.Code != code {
		t.Fatalf("rpc code = %s, want %s (%v)", rpcErr.Code, code, err)
	}
}
