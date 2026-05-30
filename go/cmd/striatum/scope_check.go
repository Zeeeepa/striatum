package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// scopeCheckTimeout bounds the read-only git invocations so a wedged git can
// never hang the diagnostic.
const scopeCheckTimeout = 3 * time.Second

// runScopeCheck implements `striatum scope-check`: a read-only, pre-`work.complete`
// diagnostic (RFC 0099 Phase-1 seed). It compares the working tree's changed
// paths against an active job's write_scope (allowed_paths / forbidden_paths)
// and reports every path that is outside allowed_paths or inside forbidden_paths
// WHILE work is still in progress, instead of letting the operator discover the
// drift only when `work.complete` refuses the transition.
//
// write_scope source: no daemon READ method the CLI already calls exposes a job's
// full allowed_paths/forbidden_paths lists — `run.graph` surfaces only
// write_scope_mode, and the full lists are delivered in the work packet at claim
// time, not re-exposed by status/run.summary/run.graph. So the command takes the
// scope via repeatable --allowed / --forbidden flags (paste them from the work
// packet's write_scope), keeping this a pure local diagnostic with no mutation.
func runScopeCheck(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	var allowed []string
	var forbidden []string
	repoRoot := repoRootOverride
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" || arg == "help" {
			printScopeCheckUsage(stdout)
			return 0
		}
		key, value, hasValue := strings.Cut(arg, "=")
		next := func() (string, bool) {
			if hasValue {
				return value, true
			}
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		switch key {
		case "--allowed":
			v, ok := next()
			if !ok {
				_, _ = fmt.Fprintln(stderr, "--allowed requires a path")
				return 2
			}
			allowed = append(allowed, v)
		case "--forbidden":
			v, ok := next()
			if !ok {
				_, _ = fmt.Fprintln(stderr, "--forbidden requires a path")
				return 2
			}
			forbidden = append(forbidden, v)
		case "--repo":
			v, ok := next()
			if !ok {
				_, _ = fmt.Fprintln(stderr, "--repo requires a path")
				return 2
			}
			repoRoot = v
		case "--json":
			parsed, err := optionalBool(value, hasValue)
			if err != nil {
				_, _ = fmt.Fprintln(stderr, "--json must be a boolean")
				return 2
			}
			jsonOutput = parsed
		default:
			_, _ = fmt.Fprintf(stderr, "unknown scope-check flag: %s\n", arg)
			return 2
		}
	}

	if len(allowed) == 0 && len(forbidden) == 0 {
		_, _ = fmt.Fprintln(stderr, "scope-check requires at least one --allowed or --forbidden path (paste them from the active work packet's write_scope)")
		printScopeCheckUsage(stderr)
		return 2
	}

	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return scopeCheckError(stdout, stderr, jsonOutput, "cwd_error", err.Error())
		}
		repoRoot = cwd
	}

	changed, err := scopeCheckChangedPaths(context.Background(), repoRoot)
	if err != nil {
		return scopeCheckError(stdout, stderr, jsonOutput, "git_status_failed", err.Error())
	}

	violations := classifyScopeViolations(changed, allowed, forbidden)
	clean := len(violations) == 0

	if jsonOutput {
		payload := map[string]any{
			"ok": true,
			"data": map[string]any{
				"clean":           clean,
				"changed_paths":   changed,
				"allowed_paths":   normalizeScopeList(allowed),
				"forbidden_paths": normalizeScopeList(forbidden),
				"violations":      violations,
			},
		}
		_ = writeJSON(stdout, payload, stderr)
		if clean {
			return 0
		}
		return 1
	}

	if clean {
		if len(changed) == 0 {
			_, _ = fmt.Fprintln(stdout, "scope-check: clean (no changed paths)")
		} else {
			_, _ = fmt.Fprintf(stdout, "scope-check: clean (%d changed path(s), all in scope)\n", len(changed))
		}
		return 0
	}

	_, _ = fmt.Fprintf(stdout, "scope-check: %d path(s) outside write_scope:\n", len(violations))
	for _, v := range violations {
		_, _ = fmt.Fprintf(stdout, "  %-9s %s\n", v["reason"], v["path"])
	}
	_, _ = fmt.Fprintln(stdout, "fix these (or adjust the job's write_scope) before work.complete.")
	return 1
}

func printScopeCheckUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, "usage: striatum scope-check (--allowed <path> ...) (--forbidden <path> ...) [--repo <path>] [--json]")
	_, _ = fmt.Fprintln(out, "read-only pre-work.complete diagnostic: flags any changed path outside allowed_paths or inside forbidden_paths.")
	_, _ = fmt.Fprintln(out, "paste --allowed/--forbidden from the active work packet's write_scope; exits nonzero on drift.")
}

func scopeCheckError(stdout io.Writer, stderr io.Writer, jsonOutput bool, code string, message string) int {
	if jsonOutput {
		_ = writeJSON(stdout, map[string]any{
			"ok":    false,
			"error": map[string]any{"code": code, "message": message},
		}, stderr)
		return 1
	}
	_, _ = fmt.Fprintln(stderr, message)
	return 1
}

// scopeCheckChangedPaths returns the repo-relative changed paths in the working
// tree (staged, unstaged, and untracked), mirroring the dirty-path enumeration
// the write-scope guard performs at work.complete. It is a minimal, read-only
// reimplementation (git status --porcelain) so the diagnostic stays inside the
// cmd package without reaching into pkg/mutations.
func scopeCheckChangedPaths(ctx context.Context, repoRoot string) ([]string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found on PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, scopeCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gitPath, "-C", repoRoot, "status", "--porcelain", "--untracked-files=all")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git status timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git status failed: %s", msg)
	}
	return parseScopeCheckStatus(stdout.String())
}

// parseScopeCheckStatus extracts repo-relative paths from `git status --porcelain`
// output. Renames (`R  old -> new`) contribute BOTH the old and new path so a
// rename that moves a file out of allowed_paths is caught. Quoted paths
// (core.quotepath) are unquoted.
func parseScopeCheckStatus(out string) ([]string, error) {
	seen := map[string]bool{}
	paths := []string{}
	add := func(raw string) error {
		clean, ok := normalizeScopePath(raw)
		if !ok {
			return fmt.Errorf("git status path escapes repository: %q", raw)
		}
		if clean == "." || seen[clean] {
			return nil
		}
		seen[clean] = true
		paths = append(paths, clean)
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		rest := line[3:]
		if before, after, ok := strings.Cut(rest, " -> "); ok && strings.ContainsRune(xy, 'R') {
			if err := add(unquoteScopePath(before)); err != nil {
				return nil, err
			}
			if err := add(unquoteScopePath(after)); err != nil {
				return nil, err
			}
			continue
		}
		if err := add(unquoteScopePath(rest)); err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func unquoteScopePath(raw string) string {
	path := strings.TrimSpace(raw)
	if strings.HasPrefix(path, `"`) {
		if unquoted, err := strconv.Unquote(path); err == nil {
			return unquoted
		}
	}
	return path
}

// classifyScopeViolations applies the same precedence the daemon's write-scope
// guard uses: a path inside forbidden_paths is a violation even if it is also
// inside allowed_paths; otherwise, when allowed_paths is non-empty, a path
// outside every allowed prefix is a violation. Each violation records its path
// and the reason ("forbidden" or "outside_allowed").
func classifyScopeViolations(changed []string, allowed []string, forbidden []string) []map[string]any {
	allowedMatchers := normalizeScopeList(allowed)
	forbiddenMatchers := normalizeScopeList(forbidden)
	violations := []map[string]any{}
	for _, path := range changed {
		clean, ok := normalizeScopePath(path)
		if !ok {
			violations = append(violations, map[string]any{"path": path, "reason": "invalid_path"})
			continue
		}
		if scopePathMatchesAny(clean, forbiddenMatchers) {
			violations = append(violations, map[string]any{"path": clean, "reason": "forbidden"})
			continue
		}
		if len(allowedMatchers) > 0 && !scopePathMatchesAny(clean, allowedMatchers) {
			violations = append(violations, map[string]any{"path": clean, "reason": "outside_allowed"})
		}
	}
	return violations
}

// normalizeScopeList cleans and de-dupes a list of scope prefixes, dropping any
// that escape the repository (absolute or `..`-bearing).
func normalizeScopeList(paths []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range paths {
		clean, ok := normalizeScopePath(p)
		if !ok || seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

// normalizeScopePath mirrors pkg/mutations' scope-path normalization: forward
// slashes, cleaned, repo-relative; "." means the repository root (matches
// everything); a path that escapes the repository returns ok=false.
func normalizeScopePath(path string) (string, bool) {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" || path == "." || path == "./" {
		return ".", true
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "../") || path == ".." {
		return "", false
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." || clean == "" {
		return ".", true
	}
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
		return "", false
	}
	return strings.TrimSuffix(clean, "/"), true
}

// scopePathMatchesAny reports whether path is at or under any matcher prefix.
// A "." matcher (repo root) matches everything.
func scopePathMatchesAny(path string, matchers []string) bool {
	for _, matcher := range matchers {
		if matcher == "." || path == matcher || strings.HasPrefix(path, matcher+"/") {
			return true
		}
	}
	return false
}
