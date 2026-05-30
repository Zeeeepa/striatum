package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassifyScopeViolations exercises the path-classification logic directly:
// a path outside allowed_paths is flagged, a path inside forbidden_paths is
// flagged (even when also inside allowed_paths), and a path inside allowed but
// not forbidden is clean.
func TestClassifyScopeViolations(t *testing.T) {
	allowed := []string{"docs/", "src/cli"}
	forbidden := []string{"src/cli/secret", ".striatum"}

	changed := []string{
		"docs/report.md",      // inside allowed, not forbidden -> clean
		"src/cli/main.go",     // inside allowed, not forbidden -> clean
		"pkg/other/thing.go",  // outside allowed -> outside_allowed
		"src/cli/secret/k.go", // inside allowed AND forbidden -> forbidden wins
		".striatum/scratch",   // forbidden (and outside allowed) -> forbidden wins
	}

	violations := classifyScopeViolations(changed, allowed, forbidden)

	got := map[string]string{}
	for _, v := range violations {
		got[v["path"].(string)] = v["reason"].(string)
	}

	if _, flagged := got["docs/report.md"]; flagged {
		t.Errorf("docs/report.md should be clean, got reason %q", got["docs/report.md"])
	}
	if _, flagged := got["src/cli/main.go"]; flagged {
		t.Errorf("src/cli/main.go should be clean, got reason %q", got["src/cli/main.go"])
	}
	if got["pkg/other/thing.go"] != "outside_allowed" {
		t.Errorf("pkg/other/thing.go reason = %q, want outside_allowed", got["pkg/other/thing.go"])
	}
	if got["src/cli/secret/k.go"] != "forbidden" {
		t.Errorf("src/cli/secret/k.go reason = %q, want forbidden (forbidden beats allowed)", got["src/cli/secret/k.go"])
	}
	if got[".striatum/scratch"] != "forbidden" {
		t.Errorf(".striatum/scratch reason = %q, want forbidden", got[".striatum/scratch"])
	}
	if len(violations) != 3 {
		t.Fatalf("expected 3 violations, got %d: %#v", len(violations), violations)
	}
}

// A "." (repo root) allowed prefix means everything is in scope unless it is
// inside a forbidden prefix.
func TestClassifyScopeViolationsRepoRootAllowed(t *testing.T) {
	violations := classifyScopeViolations(
		[]string{"anything/here.go", "deep/nested/file.txt", "node_modules/x"},
		[]string{"."},
		[]string{"node_modules"},
	)
	if len(violations) != 1 || violations[0]["path"] != "node_modules/x" || violations[0]["reason"] != "forbidden" {
		t.Fatalf("repo-root allowed: want only node_modules/x forbidden, got %#v", violations)
	}
}

// Empty allowed_paths with only forbidden_paths flags solely forbidden hits.
func TestClassifyScopeViolationsForbiddenOnly(t *testing.T) {
	violations := classifyScopeViolations(
		[]string{"a/b.go", "secret/key.pem"},
		nil,
		[]string{"secret"},
	)
	if len(violations) != 1 || violations[0]["path"] != "secret/key.pem" {
		t.Fatalf("forbidden-only: want secret/key.pem flagged, got %#v", violations)
	}
}

func TestParseScopeCheckStatusRenameAndUntracked(t *testing.T) {
	out := " M src/cli/main.go\n?? docs/new.md\nR  src/old.go -> pkg/new.go\nA  added.txt\n"
	paths, err := parseScopeCheckStatus(out)
	if err != nil {
		t.Fatalf("parseScopeCheckStatus: %v", err)
	}
	want := map[string]bool{
		"src/cli/main.go": true,
		"docs/new.md":     true,
		"src/old.go":      true, // rename source
		"pkg/new.go":      true, // rename dest
		"added.txt":       true,
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %d entries", paths, len(want))
	}
	for _, p := range paths {
		if !want[p] {
			t.Errorf("unexpected path %q in %v", p, paths)
		}
	}
}

// initScopeCheckRepo creates a throwaway git repo with one committed baseline
// file, so subsequent edits show up in `git status --porcelain`.
func initScopeCheckRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "base.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "base")
	return dir
}

func writeScopeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScopeCheckCleanExitZero(t *testing.T) {
	dir := initScopeCheckRepo(t)
	writeScopeFile(t, dir, "docs/new.md", "hi\n")

	var stdout, stderr bytes.Buffer
	exit := run([]string{"--repo", dir, "scope-check", "--allowed", "docs"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d (want 0); stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestScopeCheckDriftExitNonzeroJSON(t *testing.T) {
	dir := initScopeCheckRepo(t)
	writeScopeFile(t, dir, "docs/new.md", "hi\n")      // in scope
	writeScopeFile(t, dir, "src/rogue.go", "x\n")      // outside allowed
	writeScopeFile(t, dir, "docs/secret/k.pem", "p\n") // forbidden

	var stdout, stderr bytes.Buffer
	exit := run([]string{"--json", "--repo", dir, "scope-check",
		"--allowed", "docs", "--forbidden", "docs/secret"}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("exit = %d (want 1); stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v; out=%s", err, stdout.String())
	}
	data := payload["data"].(map[string]any)
	if data["clean"] != false {
		t.Fatalf("clean = %v, want false", data["clean"])
	}
	reasons := map[string]string{}
	for _, raw := range data["violations"].([]any) {
		v := raw.(map[string]any)
		reasons[v["path"].(string)] = v["reason"].(string)
	}
	if reasons["src/rogue.go"] != "outside_allowed" {
		t.Errorf("src/rogue.go reason = %q, want outside_allowed (%#v)", reasons["src/rogue.go"], reasons)
	}
	if reasons["docs/secret/k.pem"] != "forbidden" {
		t.Errorf("docs/secret/k.pem reason = %q, want forbidden (%#v)", reasons["docs/secret/k.pem"], reasons)
	}
	if _, flagged := reasons["docs/new.md"]; flagged {
		t.Errorf("docs/new.md should be in scope, got %q", reasons["docs/new.md"])
	}
}

func TestScopeCheckRequiresScopeFlags(t *testing.T) {
	dir := initScopeCheckRepo(t)
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--repo", dir, "scope-check"}, &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d (want 2); stderr=%s", exit, stderr.String())
	}
}

func TestScopeCheckChangedPathsReadsWorkingTree(t *testing.T) {
	dir := initScopeCheckRepo(t)
	writeScopeFile(t, dir, "a/b.go", "x\n")
	paths, err := scopeCheckChangedPaths(context.Background(), dir)
	if err != nil {
		t.Fatalf("scopeCheckChangedPaths: %v", err)
	}
	found := false
	for _, p := range paths {
		if p == "a/b.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a/b.go not in changed paths: %v", paths)
	}
}

// #91: scope-check reads write_scope from a work-packet JSON file, daemon-free.
func TestScopeFromPacketExtractsWriteScope(t *testing.T) {
	// top-level write_scope
	a, f, err := scopeFromPacket([]byte(`{"write_scope":{"allowed_paths":["docs/","go/"],"forbidden_paths":[".striatum/"]}}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(a) != 2 || a[0] != "docs/" || len(f) != 1 || f[0] != ".striatum/" {
		t.Fatalf("unexpected scope: allowed=%v forbidden=%v", a, f)
	}
	// wrapped under data.packet (work.await_packet shape)
	a2, f2, err := scopeFromPacket([]byte(`{"data":{"packet":{"write_scope":{"allowed_paths":["src/"]}}}}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(a2) != 1 || a2[0] != "src/" || len(f2) != 0 {
		t.Fatalf("unexpected wrapped scope: allowed=%v forbidden=%v", a2, f2)
	}
	// non-JSON is a clear error
	if _, _, err := scopeFromPacket([]byte("not json")); err == nil {
		t.Fatal("expected parse error for non-JSON packet")
	}
}

func TestScopeCheckPacketFileDriftExitNonzero(t *testing.T) {
	dir := initScopeCheckRepo(t)
	writeScopeFile(t, dir, "docs/new.md", "hi\n") // in scope
	writeScopeFile(t, dir, "src/rogue.go", "x\n") // outside allowed
	packet := filepath.Join(dir, "packet.json")
	if err := os.WriteFile(packet, []byte(`{"write_scope":{"allowed_paths":["docs"],"forbidden_paths":["docs/secret"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--repo", dir, "scope-check", "--packet-file", packet}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("expected drift exit 1, got %d (out=%s err=%s)", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "src/rogue.go") {
		t.Fatalf("expected the out-of-scope path reported, got: %s", stdout.String())
	}
}
