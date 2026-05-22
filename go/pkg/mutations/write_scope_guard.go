package mutations

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func enforceWriteScopeClean(ctx context.Context, runner any, repositoryID string, job map[string]any) error {
	scope := asMap(job["write_scope_json"])
	if !isRepoWriteScope(scope) {
		return nil
	}
	allowed := stringListFromAny(scope["allowed_paths"])
	forbidden := stringListFromAny(scope["forbidden_paths"])
	if len(allowed) == 0 && len(forbidden) == 0 {
		return nil
	}
	repo, err := rowByID(ctx, runner, repositoryID, "repositories", "repository_id", repositoryID, false)
	if err != nil {
		return err
	}
	repoRoot := fmt.Sprint(repo["repo_root"])
	paths, err := gitChangedPaths(ctx, repoRoot)
	if err != nil {
		return rpc.NewError("invalid_transition", "write_scope check failed: "+err.Error(), nil)
	}
	violations := writeScopeViolations(paths, allowed, forbidden)
	if len(violations) == 0 {
		return nil
	}
	return rpc.NewError("invalid_transition", "write_scope violation: changed paths outside allowed_paths or inside forbidden_paths", map[string]any{
		"job_id":          job["job_id"],
		"workflow_job_id": job["workflow_job_id"],
		"violations":      violations,
		"allowed_paths":   allowed,
		"forbidden_paths": forbidden,
	})
}

func isRepoWriteScope(scope map[string]any) bool {
	return scope["repo_write"] == true || fmt.Sprint(scope["mode"]) == "repo_write"
}

func stringListFromAny(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func gitChangedPaths(ctx context.Context, repoRoot string) ([]string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("repository root is empty")
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "git", "-C", repoRoot, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	output, err := cmd.Output()
	if cmdCtx.Err() != nil {
		return nil, fmt.Errorf("git status timed out")
	}
	if err != nil {
		return nil, err
	}
	return parseGitPorcelainZ(output), nil
}

func parseGitPorcelainZ(output []byte) []string {
	records := bytes.Split(output, []byte{0})
	paths := make([]string, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}
		status := string(record[:2])
		path := string(record[3:])
		if path != "" {
			paths = append(paths, filepath.ToSlash(path))
		}
		if strings.Contains(status, "R") || strings.Contains(status, "C") {
			if i+1 < len(records) && len(records[i+1]) > 0 {
				i++
				paths = append(paths, filepath.ToSlash(string(records[i])))
			}
		}
	}
	sort.Strings(paths)
	return dedupeStrings(paths)
}

func writeScopeViolations(paths []string, allowed []string, forbidden []string) []string {
	allowedMatchers := normalizedScopeMatchers(allowed)
	forbiddenMatchers := normalizedScopeMatchers(forbidden)
	violations := make([]string, 0)
	for _, path := range paths {
		clean, ok := normalizeScopePath(path)
		if !ok {
			violations = append(violations, path)
			continue
		}
		if pathMatchesAny(clean, forbiddenMatchers) {
			violations = append(violations, clean)
			continue
		}
		if len(allowedMatchers) > 0 && !pathMatchesAny(clean, allowedMatchers) {
			violations = append(violations, clean)
		}
	}
	return dedupeStrings(violations)
}

func normalizedScopeMatchers(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		clean, ok := normalizeScopePath(path)
		if ok {
			out = append(out, clean)
		}
	}
	return dedupeStrings(out)
}

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

func pathMatchesAny(path string, matchers []string) bool {
	for _, matcher := range matchers {
		if matcher == "." || path == matcher || strings.HasPrefix(path, matcher+"/") {
			return true
		}
	}
	return false
}

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	out := items[:0]
	var prev string
	for i, item := range items {
		if i == 0 || item != prev {
			out = append(out, item)
			prev = item
		}
	}
	return out
}
