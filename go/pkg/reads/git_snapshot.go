package reads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	gitSnapshotSchemaVersion = "striatum.git_snapshot.v1"
	maxGitAncestryLimit      = 50
	defaultGitAncestryLimit  = 10
	gitCommandTimeout        = 3 * time.Second
)

var gitExecutable = "git"

func HandleGitSnapshot(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if err := validateGitSnapshotSchema(envelope); err != nil {
		return nil, err
	}
	includeAncestry, err := boolParamDefault(envelope, "include_ancestry", true)
	if err != nil {
		return nil, err
	}
	ancestryLimit, err := intParamDefault(envelope, "ancestry_limit", defaultGitAncestryLimit)
	if err != nil {
		return nil, err
	}
	if ancestryLimit < 0 {
		return nil, rpc.NewError("schema_invalid", "git.snapshot ancestry_limit must be non-negative", nil)
	}
	if ancestryLimit > maxGitAncestryLimit {
		ancestryLimit = maxGitAncestryLimit
	}
	repoRoot, err := archiveRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	observedAt := time.Now().UTC().Format(time.RFC3339)
	result := baseGitSnapshot(repositoryID, repoRoot, observedAt, includeAncestry, ancestryLimit)

	gitPath, err := exec.LookPath(gitExecutable)
	if err != nil {
		result["git_available"] = false
		return result, nil
	}
	git := localGit{path: gitPath}
	isRepo, err := gitIsRepository(ctx, git, repoRoot)
	if err != nil {
		return nil, gitSnapshotError(err)
	}
	result["git_available"] = true
	result["is_git_repository"] = isRepo
	if !isRepo {
		return result, nil
	}

	status, err := git.output(ctx, repoRoot, "status", "--porcelain=v1", "--branch")
	if err != nil {
		return nil, gitSnapshotError(err)
	}
	branch, changedPaths, dirty, err := parseGitStatus(status)
	if err != nil {
		return nil, gitSnapshotError(err)
	}
	result["branch"] = branch
	result["dirty"] = dirty
	result["changed_paths"] = changedPaths

	if head, err := gitHead(ctx, git, repoRoot); err == nil {
		result["head"] = head
	} else if !isNoHeadError(err) {
		return nil, gitSnapshotError(err)
	}
	if includeAncestry {
		commits := []map[string]any{}
		if ancestry, err := gitAncestry(ctx, git, repoRoot, ancestryLimit); err == nil {
			commits = ancestry
		} else if !isNoHeadError(err) {
			return nil, gitSnapshotError(err)
		}
		result["ancestry"] = map[string]any{
			"limit":   ancestryLimit,
			"commits": commits,
		}
	}
	return result, nil
}

func baseGitSnapshot(repositoryID string, repoRoot string, observedAt string, includeAncestry bool, ancestryLimit int) map[string]any {
	result := map[string]any{
		"schema_version":    gitSnapshotSchemaVersion,
		"repository_id":     repositoryID,
		"repo_path":         repoRoot,
		"observed_at":       observedAt,
		"git_available":     true,
		"is_git_repository": false,
		"branch":            nil,
		"head":              nil,
		"dirty": map[string]any{
			"is_dirty":         false,
			"tracked_modified": 0,
			"staged":           0,
			"untracked":        0,
			"conflicted":       0,
			"renamed":          0,
			"deleted":          0,
		},
		"changed_paths": []map[string]any{},
	}
	if includeAncestry {
		result["ancestry"] = map[string]any{"limit": ancestryLimit, "commits": []map[string]any{}}
	}
	return result
}

func validateGitSnapshotSchema(envelope rpc.Envelope) error {
	raw, ok := envelope.Params["schema_version"]
	if !ok || raw == nil {
		return nil
	}
	value, ok := numberParam(raw)
	if !ok || value != 1 {
		return rpc.NewError("schema_invalid", "git.snapshot schema_version must be 1 when supplied", nil)
	}
	return nil
}

func boolParamDefault(envelope rpc.Envelope, key string, fallback bool) (bool, error) {
	raw, ok := envelope.Params[key]
	if !ok || raw == nil {
		return fallback, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, rpc.NewError("schema_invalid", "git.snapshot "+key+" must be boolean", nil)
	}
	return value, nil
}

func intParamDefault(envelope rpc.Envelope, key string, fallback int) (int, error) {
	raw, ok := envelope.Params[key]
	if !ok || raw == nil {
		return fallback, nil
	}
	value, ok := numberParam(raw)
	if !ok {
		return 0, rpc.NewError("schema_invalid", "git.snapshot "+key+" must be an integer", nil)
	}
	return value, nil
}

func numberParam(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		if value != float64(int(value)) {
			return 0, false
		}
		return int(value), true
	case json.Number:
		parsed, err := strconv.Atoi(value.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

type localGit struct {
	path string
}

func (g localGit) output(ctx context.Context, repoRoot string, args ...string) (string, error) {
	if err := validateGitArgs(args); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()
	cmdArgs := append([]string{"-C", repoRoot}, args...)
	cmd := exec.CommandContext(ctx, g.path, cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git command timed out")
		}
		return "", gitCommandFailure{args: args, message: boundedGitMessage(stderr.String())}
	}
	return stdout.String(), nil
}

type gitCommandFailure struct {
	args    []string
	message string
}

func (e gitCommandFailure) Error() string {
	if e.message == "" {
		return "git " + strings.Join(e.args, " ") + " failed"
	}
	return "git " + strings.Join(e.args, " ") + " failed: " + e.message
}

func validateGitArgs(args []string) error {
	if len(args) == 0 {
		return errors.New("git command missing subcommand")
	}
	switch args[0] {
	case "rev-parse", "status", "log":
	default:
		return fmt.Errorf("git subcommand is not allowed: %s", args[0])
	}
	for _, arg := range args {
		switch arg {
		case "fetch", "pull", "push", "remote", "ls-remote", "commit", "checkout", "switch", "merge", "rebase", "reset", "add", "restore", "tag":
			return fmt.Errorf("git argument is not allowed: %s", arg)
		}
	}
	return nil
}

func gitIsRepository(ctx context.Context, git localGit, repoRoot string) (bool, error) {
	out, err := git.output(ctx, repoRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		var gitErr gitCommandFailure
		if errors.As(err, &gitErr) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func gitHead(ctx context.Context, git localGit, repoRoot string) (map[string]any, error) {
	out, err := git.output(ctx, repoRoot, "log", "-n", "1", "--format=%H%x1f%h%x1f%aI%x1f%cI%x1f%s")
	if err != nil {
		return nil, err
	}
	fields := strings.SplitN(strings.TrimRight(out, "\n"), "\x1f", 5)
	if len(fields) < 5 || fields[0] == "" {
		return nil, gitCommandFailure{args: []string{"log", "-n", "1"}, message: "no HEAD commit"}
	}
	return map[string]any{
		"sha":            fields[0],
		"short_sha":      fields[1],
		"author_date":    fields[2],
		"committer_date": fields[3],
		"subject":        sanitizeGitSubject(fields[4]),
	}, nil
}

func gitAncestry(ctx context.Context, git localGit, repoRoot string, limit int) ([]map[string]any, error) {
	if limit == 0 {
		return []map[string]any{}, nil
	}
	out, err := git.output(ctx, repoRoot, "log", "-n", strconv.Itoa(limit), "--format=%H%x1f%h%x1f%P%x1f%aI%x1f%cI%x1f%s%x1e")
	if err != nil {
		return nil, err
	}
	return parseGitLogRecords(out), nil
}

func parseGitLogRecords(out string) []map[string]any {
	commits := []map[string]any{}
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x1f", 6)
		if len(fields) < 6 || fields[0] == "" {
			continue
		}
		parents := []string{}
		for _, parent := range strings.Fields(fields[2]) {
			parents = append(parents, parent)
		}
		commits = append(commits, map[string]any{
			"sha":            fields[0],
			"short_sha":      fields[1],
			"parents":        parents,
			"author_date":    fields[3],
			"committer_date": fields[4],
			"subject":        sanitizeGitSubject(fields[5]),
		})
	}
	return commits
}

func parseGitStatus(out string) (map[string]any, []map[string]any, map[string]any, error) {
	branch := map[string]any{
		"name":     nil,
		"detached": false,
		"upstream": nil,
		"ahead":    0,
		"behind":   0,
	}
	changed := []map[string]any{}
	dirty := map[string]any{
		"is_dirty":         false,
		"tracked_modified": 0,
		"staged":           0,
		"untracked":        0,
		"conflicted":       0,
		"renamed":          0,
		"deleted":          0,
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseGitBranchLine(branch, strings.TrimPrefix(line, "## "))
			continue
		}
		entry, err := parseGitStatusEntry(line)
		if err != nil {
			return nil, nil, nil, err
		}
		if entry == nil {
			continue
		}
		changed = append(changed, entry)
		updateDirtySummary(dirty, entry)
	}
	dirty["is_dirty"] = len(changed) > 0
	return branch, changed, dirty, nil
}

func parseGitBranchLine(branch map[string]any, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	status := ""
	if head, tail, ok := strings.Cut(line, " ["); ok {
		line = head
		status = strings.TrimSuffix(tail, "]")
	}
	if line == "HEAD (no branch)" {
		branch["name"] = "HEAD"
		branch["detached"] = true
	} else if name, upstream, ok := strings.Cut(line, "..."); ok {
		branch["name"] = name
		branch["upstream"] = upstream
	} else {
		branch["name"] = line
	}
	for _, part := range strings.Split(status, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "ahead ") {
			branch["ahead"] = parseCountSuffix(part, "ahead ")
		}
		if strings.HasPrefix(part, "behind ") {
			branch["behind"] = parseCountSuffix(part, "behind ")
		}
	}
}

func parseCountSuffix(value string, prefix string) int {
	parsed, _ := strconv.Atoi(strings.TrimPrefix(value, prefix))
	return parsed
}

func parseGitStatusEntry(line string) (map[string]any, error) {
	if len(line) < 4 {
		return nil, nil
	}
	indexStatus := string(line[0])
	worktreeStatus := string(line[1])
	rawPath := line[3:]
	oldPath := any(nil)
	if strings.ContainsAny(indexStatus+worktreeStatus, "R") {
		before, after, ok := strings.Cut(rawPath, " -> ")
		if ok {
			parsedOld, err := safeGitPath(before)
			if err != nil {
				return nil, err
			}
			oldPath = parsedOld
			rawPath = after
		}
	}
	path, err := safeGitPath(rawPath)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"path":            path,
		"index_status":    indexStatus,
		"worktree_status": worktreeStatus,
		"kind":            gitChangeKind(indexStatus, worktreeStatus),
		"old_path":        oldPath,
	}, nil
}

func safeGitPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if strings.HasPrefix(path, `"`) {
		if unquoted, err := strconv.Unquote(path); err == nil {
			path = unquoted
		}
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || path == "" || strings.HasPrefix(path, "/") || path == ".." || strings.HasPrefix(path, "../") {
		return "", fmt.Errorf("git status path escapes repository: %q", raw)
	}
	return path, nil
}

func gitChangeKind(indexStatus string, worktreeStatus string) string {
	if isGitConflict(indexStatus, worktreeStatus) {
		return "conflicted"
	}
	if indexStatus == "?" && worktreeStatus == "?" {
		return "untracked"
	}
	if indexStatus == "R" || worktreeStatus == "R" {
		return "renamed"
	}
	if indexStatus == "D" || worktreeStatus == "D" {
		return "deleted"
	}
	if indexStatus == "A" || worktreeStatus == "A" {
		return "added"
	}
	return "modified"
}

func isGitConflict(indexStatus string, worktreeStatus string) bool {
	pair := indexStatus + worktreeStatus
	switch pair {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return indexStatus == "U" || worktreeStatus == "U"
	}
}

func updateDirtySummary(dirty map[string]any, entry map[string]any) {
	indexStatus, _ := entry["index_status"].(string)
	worktreeStatus, _ := entry["worktree_status"].(string)
	if isGitConflict(indexStatus, worktreeStatus) {
		incrementDirty(dirty, "conflicted")
		return
	}
	if indexStatus == "?" && worktreeStatus == "?" {
		incrementDirty(dirty, "untracked")
		return
	}
	if indexStatus != " " && indexStatus != "?" {
		incrementDirty(dirty, "staged")
	}
	if worktreeStatus != " " && worktreeStatus != "?" {
		incrementDirty(dirty, "tracked_modified")
	}
	if indexStatus == "R" || worktreeStatus == "R" {
		incrementDirty(dirty, "renamed")
	}
	if indexStatus == "D" || worktreeStatus == "D" {
		incrementDirty(dirty, "deleted")
	}
}

func incrementDirty(dirty map[string]any, key string) {
	current, _ := dirty[key].(int)
	dirty[key] = current + 1
}

func gitSnapshotError(err error) error {
	return rpc.NewError("git_snapshot_failed", err.Error(), nil)
}

func isNoHeadError(err error) bool {
	var gitErr gitCommandFailure
	if !errors.As(err, &gitErr) {
		return false
	}
	message := strings.ToLower(gitErr.message)
	return strings.Contains(message, "does not have any commits yet") ||
		strings.Contains(message, "your current branch") ||
		strings.Contains(message, "no head commit")
}

func boundedGitMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func sanitizeGitSubject(subject string) string {
	subject = strings.TrimSpace(strings.ReplaceAll(subject, "\n", " "))
	if len(subject) > 240 {
		return subject[:240]
	}
	return subject
}
