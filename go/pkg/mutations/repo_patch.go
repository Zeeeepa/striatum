package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

type repoPatchConfig struct {
	RepositoryID string
	SessionID    string
	JobID        string
	LeaseID      string
	RunID        string
	RepoRoot     string
	Cwd          string
	WriteScope   map[string]any
	PatchSHA     string
}

func HandleRepoPatchPreview(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	return handleRepoPatch(ctx, runner, envelope, false)
}

func HandleRepoPatchApply(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	return handleRepoPatch(ctx, runner, envelope, true)
}

func handleRepoPatch(ctx context.Context, runner db.Runner, envelope rpc.Envelope, apply bool) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredStringParam(envelope.Params, "session_id")
	if err != nil {
		return nil, err
	}
	jobID, err := requiredStringParam(envelope.Params, "job_id")
	if err != nil {
		return nil, err
	}
	leaseID, err := requiredStringParam(envelope.Params, "lease_id")
	if err != nil {
		return nil, err
	}
	patch, err := requiredStringParam(envelope.Params, "patch")
	if err != nil {
		return nil, err
	}
	if _, err := enforceSessionBinding(ctx, sessionID); err != nil {
		return nil, err
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		cfg, err := repoPatchConfigFor(ctx, tx, repositoryID, sessionID, jobID, leaseID, patch)
		if err != nil {
			return nil, err
		}
		paths, err := repoPatchChangedPaths(ctx, cfg.Cwd, patch)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, rpc.NewError("schema_invalid", "repo patch did not name any changed paths", nil)
		}
		for _, path := range paths {
			if !pathAllowed(cfg.RepoRoot, path, cfg.WriteScope) {
				return nil, rpc.NewError("path_outside_scope", "write_scope violation: repo patch path is outside this job's write_scope", map[string]any{
					"path":            path,
					"allowed_paths":   cfg.WriteScope["allowed_paths"],
					"forbidden_paths": cfg.WriteScope["forbidden_paths"],
				})
			}
		}
		status := "previewed"
		eventType := "repo.patch_previewed"
		if apply {
			if err := gitApplyPatch(ctx, cfg.Cwd, patch); err != nil {
				return nil, rpc.NewError("invalid_transition", "repo.patch-apply failed: "+err.Error(), nil)
			}
			status = "applied"
			eventType = "repo.patch_applied"
		}
		payload := map[string]any{
			"job_id":       cfg.JobID,
			"lease_id":     cfg.LeaseID,
			"paths":        paths,
			"patch_sha256": cfg.PatchSHA,
		}
		if _, err := appendEvent(ctx, tx, cfg.RepositoryID, cfg.RunID, eventType, cfg.SessionID, cfg.JobID, nil, nil, cfg.LeaseID, payload); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":       status,
			"job_id":       cfg.JobID,
			"lease_id":     cfg.LeaseID,
			"paths":        paths,
			"patch_sha256": cfg.PatchSHA,
		}, nil
	})
}

func repoPatchConfigFor(ctx context.Context, runner any, repositoryID, sessionID, jobID, leaseID, patch string) (repoPatchConfig, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return repoPatchConfig{}, err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return repoPatchConfig{}, err
	}
	if !isRepoWrite(job) {
		return repoPatchConfig{}, rpc.NewError("invalid_transition", "repo.patch requires a repo_write job write_scope", nil)
	}
	runID := fmt.Sprint(job["run_id"])
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
	if err != nil {
		return repoPatchConfig{}, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	cwd, err := processRunCwd(ctx, runner, repositoryID, job, repoRoot, "repo.patch")
	if err != nil {
		return repoPatchConfig{}, err
	}
	sum := sha256.Sum256([]byte(patch))
	return repoPatchConfig{
		RepositoryID: repositoryID,
		SessionID:    sessionID,
		JobID:        jobID,
		LeaseID:      leaseID,
		RunID:        runID,
		RepoRoot:     repoRoot,
		Cwd:          cwd,
		WriteScope:   asMap(job["write_scope_json"]),
		PatchSHA:     hex.EncodeToString(sum[:]),
	}, nil
}

func repoPatchChangedPaths(ctx context.Context, cwd, patch string) ([]string, error) {
	if err := gitApplyCheck(ctx, cwd, patch); err != nil {
		return nil, rpc.NewError("invalid_transition", "repo patch check failed: "+err.Error(), nil)
	}
	output, err := runGitApply(ctx, cwd, []string{"apply", "--numstat", "-"}, patch)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "repo patch numstat failed: "+err.Error(), nil)
	}
	seen := map[string]struct{}{}
	paths := []string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			return nil, rpc.NewError("schema_invalid", "repo patch numstat output was not understood", map[string]any{"line": line})
		}
		path := strings.TrimSpace(parts[2])
		if path == "" {
			return nil, rpc.NewError("schema_invalid", "repo patch numstat output included an empty path", nil)
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths, nil
}

func gitApplyCheck(ctx context.Context, cwd, patch string) error {
	_, err := runGitApply(ctx, cwd, []string{"apply", "--check", "-"}, patch)
	return err
}

func gitApplyPatch(ctx context.Context, cwd, patch string) error {
	_, err := runGitApply(ctx, cwd, []string{"apply", "--whitespace=nowarn", "-"}, patch)
	return err
}

func runGitApply(ctx context.Context, cwd string, args []string, patch string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(patch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text != "" {
			return "", fmt.Errorf("%w: %s", err, text)
		}
		return "", err
	}
	return string(output), nil
}
