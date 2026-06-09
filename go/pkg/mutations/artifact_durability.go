package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

type publishedArtifactDurabilityProblem struct {
	ArtifactID    string
	LogicalName   string
	RepoPath      string
	ContentSHA256 string
	Reason        string
}

type artifactDurabilityCheck struct {
	RepositoryID string
	RepoRoot     string
	Job          map[string]any
	Worktree     map[string]any
}

func ensurePerJobPublishedArtifactsDurable(ctx context.Context, runner any, repositoryID string, job map[string]any, surface string) error {
	if !isRepoWrite(job) {
		return nil
	}
	required, worktree, err := worktreeRequirementForJob(ctx, runner, repositoryID, job)
	if err != nil {
		return err
	}
	if !required {
		return nil
	}
	if worktree == nil {
		return worktreeRequiredError(job, surface)
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return err
	}
	problems, err := publishedArtifactDurabilityProblems(ctx, runner, artifactDurabilityCheck{
		RepositoryID: repositoryID,
		RepoRoot:     repoRoot,
		Job:          job,
		Worktree:     worktree,
	})
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		return nil
	}
	return publishedArtifactsNotDurableError(surface, job, problems)
}

func publishedArtifactDurabilityProblems(ctx context.Context, runner any, check artifactDurabilityCheck) ([]publishedArtifactDurabilityProblem, error) {
	target, err := worktreeTarget(check.RepoRoot, fmt.Sprint(check.Worktree["worktree_path"]))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(ctx, runner, `
		SELECT a.artifact_id, a.logical_name, a.repo_path, a.content_sha256, a.blob_key
		  FROM striatumd.artifacts a
		 WHERE a.repository_id = $1
		   AND a.run_id = $2
		   AND a.job_id = $3
		   AND a.attempt = $4
		 ORDER BY a.created_at, a.artifact_id`,
		check.RepositoryID, check.Job["run_id"], check.Job["job_id"], check.Job["attempt"])
	if err != nil {
		return nil, err
	}
	problems := []publishedArtifactDurabilityProblem{}
	for _, row := range rows {
		blobKey := strings.TrimSpace(fmt.Sprint(nullable(row["blob_key"])))
		if blobKey != "" && blobKey != "<nil>" {
			continue
		}
		problem := publishedArtifactDurabilityProblem{
			ArtifactID:    fmt.Sprint(row["artifact_id"]),
			LogicalName:   fmt.Sprint(row["logical_name"]),
			RepoPath:      fmt.Sprint(row["repo_path"]),
			ContentSHA256: fmt.Sprint(row["content_sha256"]),
		}
		repoPath, ok := cleanDurableArtifactPath(problem.RepoPath)
		if !ok {
			problem.Reason = "invalid_repo_path"
			problems = append(problems, problem)
			continue
		}
		committedSHA, found, err := gitCommittedFileSHA256(ctx, target, repoPath)
		if err != nil {
			return nil, err
		}
		if !found {
			problem.Reason = "missing_from_head"
			problems = append(problems, problem)
			continue
		}
		if committedSHA != problem.ContentSHA256 {
			problem.Reason = "head_content_mismatch"
			problems = append(problems, problem)
		}
	}
	return problems, nil
}

func cleanDurableArtifactPath(pathText string) (string, bool) {
	pathText = strings.TrimSpace(filepath.ToSlash(pathText))
	if pathText == "" || strings.HasPrefix(pathText, "/") {
		return "", false
	}
	clean := filepath.ToSlash(filepath.Clean(pathText))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func gitCommittedFileSHA256(ctx context.Context, worktreeRoot, repoPath string) (string, bool, error) {
	result, err := runGitWorktreeCommand(ctx, worktreeRoot, "show", "HEAD:"+repoPath)
	if err != nil {
		return "", false, err
	}
	if result.ExitCode != 0 {
		return "", false, nil
	}
	sum := sha256.Sum256([]byte(result.Stdout))
	return hex.EncodeToString(sum[:]), true, nil
}

func publishedArtifactsNotDurableError(surface string, job map[string]any, problems []publishedArtifactDurabilityProblem) error {
	return rpc.NewError("invalid_transition", fmt.Sprintf(
		"%s refused: published artifact content is not durable outside the per-job worktree; commit the artifact file or publish it through blob-backed storage before retrying",
		surface,
	), map[string]any{
		"run_id":    job["run_id"],
		"job_id":    job["job_id"],
		"artifacts": publishedArtifactDurabilityProblemMaps(problems),
	})
}

func publishedArtifactDurabilityProblemMaps(problems []publishedArtifactDurabilityProblem) []map[string]any {
	items := make([]map[string]any, 0, len(problems))
	for _, p := range problems {
		items = append(items, map[string]any{
			"artifact_id":    p.ArtifactID,
			"logical_name":   p.LogicalName,
			"repo_path":      p.RepoPath,
			"content_sha256": p.ContentSHA256,
			"reason":         p.Reason,
		})
	}
	return items
}
