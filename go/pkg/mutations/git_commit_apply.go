package mutations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	gitCommitApplySchemaVersion = "striatum.git_commit_apply.v1"
	gitCommitCommandTimeout     = 10 * time.Second
)

var gitCommitExecutable = "git"

type commitRequest struct {
	RequestID          string
	BaseHead           string
	Branch             string
	GitSnapshotHash    string
	IncludedPaths      []string
	ReviewedArtifacts  []string
	CommitMessage      string
	ConfirmationStatus string
	ConfirmedBy        string
	ConfirmedAt        string
}

func HandleGitCommitApply(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if err := validateGitCommitApplySchema(envelope); err != nil {
		return nil, err
	}
	if envelope.Params["confirm"] != true {
		return nil, rpc.NewError("confirmation_required", "git.commit_apply requires confirm=true", map[string]any{"field_path": "confirm"})
	}
	commitRequestPath := strings.TrimSpace(stringParam(envelope, "commit_request_path"))
	if commitRequestPath == "" {
		return nil, rpc.NewError("schema_invalid", "git.commit_apply requires commit_request_path", map[string]any{"field_path": "commit_request_path"})
	}

	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	requestAbs, err := repoRelativePath(repoRoot, commitRequestPath, false)
	if err != nil {
		return nil, rpc.NewError("schema_invalid", "commit_request_path must be repo-relative and outside .striatum", map[string]any{"field_path": "commit_request_path"})
	}
	requestBytes, err := os.ReadFile(requestAbs)
	if err != nil {
		return nil, rpc.NewError("commit_request_not_found", "commit_request artifact is not readable", map[string]any{"path": commitRequestPath})
	}
	request, err := parseCommitRequestArtifact(requestBytes)
	if err != nil {
		return nil, err
	}
	confirmRequestID := strings.TrimSpace(stringParam(envelope, "confirm_request_id"))
	if confirmRequestID == "" || confirmRequestID != request.RequestID {
		return nil, rpc.NewError("confirmation_required", "confirm_request_id must match the commit_request artifact request_id", map[string]any{
			"field_path": "confirm_request_id",
			"request_id": request.RequestID,
		})
	}
	if err := validateCommitRequestConfirmation(request); err != nil {
		return nil, err
	}
	includedPaths, err := normalizeCommitIncludedPaths(repoRoot, request.IncludedPaths)
	if err != nil {
		return nil, err
	}

	gitPath, err := exec.LookPath(gitCommitExecutable)
	if err != nil {
		return nil, rpc.NewError("git_unavailable", "git executable is not available", nil)
	}
	hooksDir, err := os.MkdirTemp("", "striatum-git-hooks-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(hooksDir)
	git := localCommitGit{path: gitPath, hooksPath: hooksDir}

	isRepo, err := gitCommitIsRepository(ctx, git, repoRoot)
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	if !isRepo {
		return nil, rpc.NewError("invalid_transition", "registered target is not a Git work tree", nil)
	}
	currentHead, err := gitCommitHead(ctx, git, repoRoot)
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	if !strings.EqualFold(currentHead, request.BaseHead) {
		return nil, rpc.NewError("base_head_mismatch", "current HEAD does not match commit_request base_head", map[string]any{
			"base_head":    request.BaseHead,
			"current_head": currentHead,
		})
	}
	currentBranch, err := gitCommitBranch(ctx, git, repoRoot)
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	if currentBranch == "HEAD" || currentBranch == "" {
		return nil, rpc.NewError("invalid_transition", "git.commit_apply refuses detached HEAD", nil)
	}
	if currentBranch != request.Branch {
		return nil, rpc.NewError("branch_mismatch", "current branch does not match commit_request branch", map[string]any{
			"branch":         request.Branch,
			"current_branch": currentBranch,
		})
	}
	changedPaths, err := gitCommitChangedPaths(ctx, git, repoRoot)
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	violations := writeScopeViolations(changedPaths, includedPaths, nil)
	if len(violations) > 0 {
		return nil, rpc.NewError("dirty_tree_outside_commit_request", "working tree has changes outside commit_request included_paths", map[string]any{
			"violations":     violations,
			"included_paths": includedPaths,
		})
	}

	if _, err := git.output(ctx, repoRoot, append([]string{"add", "--"}, includedPaths...)...); err != nil {
		return nil, gitCommitApplyError(err)
	}
	commitArgs := append([]string{"commit", "--no-gpg-sign", "-m", request.CommitMessage, "--"}, includedPaths...)
	if _, err := git.output(ctx, repoRoot, commitArgs...); err != nil {
		return nil, gitCommitApplyError(err)
	}
	commitSHA, err := gitCommitHead(ctx, git, repoRoot)
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	if commitSHA == currentHead {
		return nil, rpc.NewError("invalid_transition", "git.commit_apply did not create a new commit", nil)
	}
	shortSHA, err := git.output(ctx, repoRoot, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return nil, gitCommitApplyError(err)
	}
	artifactSum := sha256.Sum256(requestBytes)

	return map[string]any{
		"schema_version":              gitCommitApplySchemaVersion,
		"repository_id":               repositoryID,
		"repo_path":                   repoRoot,
		"commit_request_path":         filepath.ToSlash(commitRequestPath),
		"commit_request_sha256":       "sha256:" + hex.EncodeToString(artifactSum[:]),
		"request_id":                  request.RequestID,
		"base_head":                   request.BaseHead,
		"branch":                      request.Branch,
		"included_paths":              includedPaths,
		"reviewed_artifacts":          request.ReviewedArtifacts,
		"git_snapshot_hash":           request.GitSnapshotHash,
		"confirmation_status":         request.ConfirmationStatus,
		"confirmed_by":                request.ConfirmedBy,
		"confirmed_at":                nullableString(request.ConfirmedAt),
		"commit_sha":                  commitSHA,
		"short_sha":                   strings.TrimSpace(shortSHA),
		"commit_subject":              commitSubject(request.CommitMessage),
		"local_commit_created":        true,
		"pushed":                      false,
		"hosted_provider_invoked":     false,
		"provider_credentials_loaded": false,
		"hooks_disabled":              true,
	}, nil
}

func validateGitCommitApplySchema(envelope rpc.Envelope) error {
	raw, ok := envelope.Params["schema_version"]
	if !ok || raw == nil {
		return nil
	}
	value, ok := integerParam(raw)
	if !ok || value != 1 {
		return rpc.NewError("schema_invalid", "git.commit_apply schema_version must be 1 when supplied", nil)
	}
	return nil
}

func parseCommitRequestArtifact(payload []byte) (commitRequest, error) {
	fields, err := artifactcontracts.ParseAndValidateFrontMatter("commit_request", "COMMIT_REQUEST.md", payload)
	if err != nil {
		return commitRequest{}, rpc.NewError("schema_invalid", err.Error(), nil)
	}
	request := commitRequest{
		RequestID:          scalarField(fields, "request_id"),
		BaseHead:           scalarField(fields, "base_head"),
		Branch:             scalarField(fields, "branch"),
		GitSnapshotHash:    scalarField(fields, "git_snapshot_hash"),
		IncludedPaths:      listField(fields, "included_paths"),
		ReviewedArtifacts:  listField(fields, "reviewed_artifacts"),
		CommitMessage:      scalarField(fields, "commit_message"),
		ConfirmationStatus: scalarField(fields, "confirmation_status"),
		ConfirmedBy:        scalarField(fields, "confirmed_by"),
		ConfirmedAt:        scalarField(fields, "confirmed_at"),
	}
	required := map[string]string{
		"request_id":          request.RequestID,
		"base_head":           request.BaseHead,
		"branch":              request.Branch,
		"git_snapshot_hash":   request.GitSnapshotHash,
		"commit_message":      request.CommitMessage,
		"confirmation_status": request.ConfirmationStatus,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return commitRequest{}, rpc.NewError("schema_invalid", "commit_request "+field+" is required", map[string]any{"field_path": field})
		}
	}
	if !isFullGitSHA(request.BaseHead) {
		return commitRequest{}, rpc.NewError("schema_invalid", "commit_request base_head must be a full 40-hex commit SHA", map[string]any{"field_path": "base_head"})
	}
	if len(request.IncludedPaths) == 0 {
		return commitRequest{}, rpc.NewError("schema_invalid", "commit_request included_paths must be non-empty", map[string]any{"field_path": "included_paths"})
	}
	if strings.ContainsRune(request.CommitMessage, '\x00') {
		return commitRequest{}, rpc.NewError("schema_invalid", "commit_request commit_message must not contain NUL", map[string]any{"field_path": "commit_message"})
	}
	if len(request.CommitMessage) > 1000 {
		return commitRequest{}, rpc.NewError("schema_invalid", "commit_request commit_message is too long for git.commit_apply", map[string]any{"field_path": "commit_message"})
	}
	return request, nil
}

func validateCommitRequestConfirmation(request commitRequest) error {
	switch request.ConfirmationStatus {
	case "operator_confirmed", "human_confirmed":
	default:
		return rpc.NewError("confirmation_required", "commit_request confirmation_status must be operator_confirmed or human_confirmed", map[string]any{
			"field_path":          "confirmation_status",
			"confirmation_status": request.ConfirmationStatus,
		})
	}
	if strings.TrimSpace(request.ConfirmedBy) == "" {
		return rpc.NewError("confirmation_required", "confirmed commit_request requires confirmed_by", map[string]any{"field_path": "confirmed_by"})
	}
	return nil
}

func scalarField(fields map[string]any, key string) string {
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func listField(fields map[string]any, key string) []string {
	value, ok := fields[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeCommitIncludedPaths(repoRoot string, paths []string) ([]string, error) {
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, pathText := range paths {
		pathText = strings.TrimSpace(pathText)
		if pathText == "" {
			return nil, rpc.NewError("schema_invalid", "commit_request included_paths must not contain empty paths", map[string]any{"field_path": "included_paths"})
		}
		abs, err := repoRelativePath(repoRoot, pathText, false)
		if err != nil {
			return nil, rpc.NewError("schema_invalid", "commit_request included_paths must be repo-relative and outside .striatum", map[string]any{
				"field_path": "included_paths",
				"path":       pathText,
			})
		}
		rel, err := filepath.Rel(repoAbs, abs)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil, rpc.NewError("schema_invalid", "git.commit_apply refuses repository-root included_paths", map[string]any{"field_path": "included_paths"})
		}
		if !seen[rel] {
			normalized = append(normalized, rel)
			seen[rel] = true
		}
	}
	return normalized, nil
}

type localCommitGit struct {
	path      string
	hooksPath string
}

func (g localCommitGit) output(ctx context.Context, repoRoot string, args ...string) (string, error) {
	if err := validateGitCommitArgs(args); err != nil {
		return "", err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, gitCommitCommandTimeout)
	defer cancel()
	cmdArgs := []string{"-C", repoRoot}
	if len(args) > 0 && args[0] == "commit" {
		cmdArgs = append(cmdArgs, "-c", "core.hooksPath="+g.hooksPath)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(cmdCtx, g.path, cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if cmdCtx.Err() != nil {
			return "", fmt.Errorf("git command timed out")
		}
		return "", gitCommitCommandFailure{args: args, message: boundedGitCommitMessage(stderr.String())}
	}
	return stdout.String(), nil
}

type gitCommitCommandFailure struct {
	args    []string
	message string
}

func (e gitCommitCommandFailure) Error() string {
	if e.message == "" {
		return "git " + e.displayArgs() + " failed"
	}
	return "git " + e.displayArgs() + " failed: " + e.message
}

func (e gitCommitCommandFailure) displayArgs() string {
	if len(e.args) >= 4 && e.args[0] == "commit" && e.args[2] == "-m" {
		redacted := append([]string(nil), e.args...)
		redacted[3] = "<commit-message>"
		return strings.Join(redacted, " ")
	}
	return strings.Join(e.args, " ")
}

func validateGitCommitArgs(args []string) error {
	if len(args) == 0 {
		return errors.New("git command missing subcommand")
	}
	switch args[0] {
	case "rev-parse", "status", "add", "commit":
	default:
		return fmt.Errorf("git subcommand is not allowed: %s", args[0])
	}
	return nil
}

func gitCommitIsRepository(ctx context.Context, git localCommitGit, repoRoot string) (bool, error) {
	out, err := git.output(ctx, repoRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		var gitErr gitCommitCommandFailure
		if errors.As(err, &gitErr) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func gitCommitHead(ctx context.Context, git localCommitGit, repoRoot string) (string, error) {
	out, err := git.output(ctx, repoRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitCommitBranch(ctx context.Context, git localCommitGit, repoRoot string) (string, error) {
	out, err := git.output(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitCommitChangedPaths(ctx context.Context, git localCommitGit, repoRoot string) ([]string, error) {
	out, err := git.output(ctx, repoRoot, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parseGitPorcelainZ([]byte(out)), nil
}

func gitCommitApplyError(err error) error {
	return rpc.NewError("git_commit_apply_failed", err.Error(), nil)
}

func integerParam(raw any) (int, bool) {
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

func isFullGitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func commitSubject(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\r\n", "\n"))
	if index := strings.IndexByte(message, '\n'); index >= 0 {
		message = message[:index]
	}
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func boundedGitCommitMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}
