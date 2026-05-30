package mutations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
	current, err := gitChangedPathSnapshots(ctx, repoRoot)
	if err != nil {
		return rpc.NewError("invalid_transition", "write_scope check failed: "+err.Error(), nil)
	}
	currentPaths := make([]string, 0, len(current))
	for _, item := range current {
		currentPaths = append(currentPaths, item.Path)
	}
	// Sibling lanes in a shared run may publish their own in-scope artifacts that
	// land outside this job's allowed_paths; when the dirty path matches a
	// sibling-published artifact's content digest, it is that lane's write, not a
	// violation of this job's scope.
	ignoredPaths, err := publishedRunArtifactIgnoredPaths(ctx, runner, repositoryID, repoRoot, job, currentPaths)
	if err != nil {
		return rpc.NewError("invalid_transition", "write_scope check failed: "+err.Error(), nil)
	}
	violations := writeScopeViolationsSinceBaseline(current, gitBaselineFromJob(job), allowed, forbidden, ignoredPaths)
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
	snapshots, err := gitChangedPathSnapshots(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(snapshots))
	for _, item := range snapshots {
		paths = append(paths, item.Path)
	}
	return paths, nil
}

type gitPathSnapshot struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	// Untracked is true when the path is untracked by git (porcelain `??`).
	// Untracked paths are operator/lane scratch: an out-of-scope untracked file
	// that was already present at claim time is never attributed to the attempt,
	// even if its content later changes. It is omitted from the persisted baseline
	// JSON (the baseline only needs path+hash) and is derived fresh from the
	// completion-time `git status`.
	Untracked bool `json:"-"`
}

// writeScopeViolationsSinceBaseline applies the RFC 0095 §6 write-scope rule.
//
// `current` is the working tree's dirty/untracked snapshot at completion time;
// `baseline` maps each path that was already dirty/untracked at claim time to
// its claim-time content hash. The guard keys on *what this attempt did*, not on
// total worktree cleanliness, so a path outside `allowed_paths` is a violation
// only when it is attributable to the attempt:
//
//  1. it was **created during the attempt** — dirty now but NOT present in the
//     claim-time baseline (the attempt created the file, or mutated a previously
//     clean/committed file away from its tracked content); or
//  2. it is an **in-scope-or-tracked file mutated away from its baseline** — a
//     tracked (committed) file that was already dirty at claim and whose content
//     the attempt moved further away from the claim-time hash.
//
// Conversely the rule ignores:
//   - a baseline-dirty/untracked path that transitioned to clean/committed (it is
//     absent from `current`, so it never enters the loop — the dirty→clean case);
//   - a pre-existing out-of-scope file the attempt left exactly as it was at
//     claim (same hash) — operator state the attempt never touched; and
//   - a pre-existing out-of-scope **untracked** operator file (e.g. an operator
//     report written incrementally during the run) even when its content changes,
//     because an untracked file already present at claim is not the attempt's
//     write — only its first-appearance (rule 1) or a tracked mutation (rule 2)
//     is attributable.
//
// `forbidden_paths` are absolute for paths the attempt actually authored: a
// current dirty path inside a forbidden prefix is a violation regardless of
// baseline. A sibling-published artifact (a dirty path whose content digest
// matches another lane's already-published run artifact, per
// `publishedRunArtifactIgnoredPaths`) is provably *that* lane's write, not this
// attempt's, so it is skipped *before* the forbidden check — otherwise every
// multi-lane gate job (#93) deadlocks at `work.complete` in a shared worktree
// when a sibling's published artifact lands inside this job's forbidden_paths
// (e.g. synthesis forbids artifacts/design/, implement forbids
// artifacts/review/).
func writeScopeViolationsSinceBaseline(current []gitPathSnapshot, baseline map[string]string, allowed, forbidden []string, ignored map[string]bool) []string {
	allowedMatchers := normalizedScopeMatchers(allowed)
	forbiddenMatchers := normalizedScopeMatchers(forbidden)
	violations := make([]string, 0)
	for _, item := range current {
		clean, ok := normalizeScopePath(item.Path)
		if !ok {
			violations = append(violations, item.Path)
			continue
		}
		// A sibling lane's published artifact is not this attempt's write. Skip it
		// before any forbidden/allowed/baseline attribution: forbidden_paths stay
		// absolute only for paths this attempt actually authored.
		if ignored[clean] {
			continue
		}
		// Forbidden is absolute for the attempt's own writes.
		if pathMatchesAny(clean, forbiddenMatchers) {
			violations = append(violations, clean)
			continue
		}
		if len(allowedMatchers) == 0 || pathMatchesAny(clean, allowedMatchers) {
			continue
		}
		// Outside allowed_paths: attribute to the attempt only via rule 1 or 2.
		baselineHash, inBaseline := baseline[item.Path]
		if !inBaseline {
			// Rule 1: appeared during the attempt.
			violations = append(violations, clean)
			continue
		}
		// Pre-existing at claim: a violation only when it is a tracked file the
		// attempt mutated away from its baseline content (rule 2). A pre-existing
		// untracked operator file is never attributed to the attempt, regardless of
		// later content changes.
		if !item.Untracked && baselineHash != item.Hash {
			violations = append(violations, clean)
		}
	}
	return dedupeStrings(violations)
}

func gitBaselineFromJob(job map[string]any) map[string]string {
	raw := asMap(job["write_scope_baseline"])
	entries := asList(raw["changed_paths"])
	out := map[string]string{}
	for _, entry := range entries {
		item := asMap(entry)
		path, _ := item["path"].(string)
		hash, _ := item["hash"].(string)
		if path != "" {
			out[path] = hash
		}
	}
	return out
}

func gitChangedPathSnapshots(ctx context.Context, repoRoot string) ([]gitPathSnapshot, error) {
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
	entries := parseGitPorcelainStatusZ(output)
	snapshots := make([]gitPathSnapshot, 0, len(entries))
	for _, entry := range entries {
		hash, err := hashRepoPath(repoRoot, entry.Path)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, gitPathSnapshot{Path: entry.Path, Hash: hash, Untracked: entry.Untracked})
	}
	return snapshots, nil
}

func hashRepoPath(repoRoot, path string) (string, error) {
	full := filepath.Join(repoRoot, filepath.FromSlash(path))
	info, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if info.IsDir() {
		return "dir", nil
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type gitPorcelainEntry struct {
	Path      string
	Untracked bool
}

func parseGitPorcelainStatusZ(output []byte) []gitPorcelainEntry {
	records := bytes.Split(output, []byte{0})
	entries := make([]gitPorcelainEntry, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}
		status := string(record[:2])
		path := string(record[3:])
		untracked := status == "??"
		if path != "" {
			entries = append(entries, gitPorcelainEntry{Path: filepath.ToSlash(path), Untracked: untracked})
		}
		if strings.Contains(status, "R") || strings.Contains(status, "C") {
			if i+1 < len(records) && len(records[i+1]) > 0 {
				i++
				// The rename/copy source is a tracked path.
				entries = append(entries, gitPorcelainEntry{Path: filepath.ToSlash(string(records[i]))})
			}
		}
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Path < entries[b].Path })
	return dedupeGitPorcelainEntries(entries)
}

func dedupeGitPorcelainEntries(entries []gitPorcelainEntry) []gitPorcelainEntry {
	if len(entries) == 0 {
		return nil
	}
	out := entries[:0]
	var prev string
	for i, entry := range entries {
		if i == 0 || entry.Path != prev {
			out = append(out, entry)
			prev = entry.Path
		}
	}
	return out
}

func parseGitPorcelainZ(output []byte) []string {
	entries := parseGitPorcelainStatusZ(output)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func writeScopeViolations(paths []string, allowed []string, forbidden []string) []string {
	return writeScopeViolationsWithIgnored(paths, allowed, forbidden, nil)
}

func writeScopeViolationsWithIgnored(paths []string, allowed []string, forbidden []string, ignored map[string]bool) []string {
	allowedMatchers := normalizedScopeMatchers(allowed)
	forbiddenMatchers := normalizedScopeMatchers(forbidden)
	violations := make([]string, 0)
	for _, path := range paths {
		clean, ok := normalizeScopePath(path)
		if !ok {
			violations = append(violations, path)
			continue
		}
		// A sibling lane's published artifact (matching digest) is not this
		// attempt's write — skip it before the forbidden check (#93).
		if ignored[clean] {
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

func publishedRunArtifactIgnoredPaths(ctx context.Context, runner any, repositoryID, repoRoot string, job map[string]any, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	runID := fmt.Sprint(job["run_id"])
	jobID := fmt.Sprint(job["job_id"])
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(jobID) == "" {
		return nil, nil
	}
	touched := map[string]bool{}
	for _, path := range paths {
		clean, ok := normalizeScopePath(path)
		if ok {
			touched[clean] = true
		}
	}
	if len(touched) == 0 {
		return nil, nil
	}
	rows, err := queryRows(ctx, runner, `
		SELECT repo_path, content_sha256
		  FROM striatumd.artifacts
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND job_id != $3`, repositoryID, runID, jobID)
	if err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	for _, row := range rows {
		clean, ok := normalizeScopePath(fmt.Sprint(row["repo_path"]))
		if !ok || !touched[clean] {
			continue
		}
		currentHash, err := hashRepoPath(repoRoot, clean)
		if err != nil {
			return nil, err
		}
		if currentHash != "" && currentHash == fmt.Sprint(row["content_sha256"]) {
			ignored[clean] = true
		}
	}
	if len(ignored) == 0 {
		return nil, nil
	}
	return ignored, nil
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
