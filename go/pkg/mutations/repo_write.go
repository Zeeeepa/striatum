package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func HandleRepoWrite(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
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
	pathText, err := requiredStringParam(envelope.Params, "path")
	if err != nil {
		return nil, err
	}
	content, ok := envelope.Params["content"].(string)
	if !ok {
		return nil, rpc.NewError("schema_invalid", "content must be a string", nil)
	}
	if _, err := enforceSessionBinding(ctx, sessionID); err != nil {
		return nil, err
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return repoWrite(ctx, tx, repositoryID, sessionID, jobID, leaseID, pathText, content)
	})
}

func repoWrite(
	ctx context.Context,
	runner any,
	repositoryID string,
	sessionID string,
	jobID string,
	leaseID string,
	pathText string,
	content string,
) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return nil, err
	}
	applyFrozenAttemptWriteScope(ctx, runner, repositoryID, job, leaseID)
	if !isRepoWrite(job) {
		return nil, rpc.NewError("invalid_transition", "repo.write requires a repo_write job write_scope", nil)
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return nil, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	scope := asMap(job["write_scope_json"])
	if !pathAllowed(repoRoot, pathText, scope) {
		return nil, rpc.NewError(
			"path_outside_scope",
			"write_scope violation: repo.write path is outside this job's write_scope",
			map[string]any{
				"path":            pathText,
				"allowed_paths":   scope["allowed_paths"],
				"forbidden_paths": scope["forbidden_paths"],
			},
		)
	}
	activeWorktree, err := requireActiveWorktreeForJob(ctx, runner, repositoryID, job, "repo.write")
	if err != nil {
		return nil, err
	}
	target, err := artifactSourcePath(repoRoot, pathText, activeWorktree)
	if err != nil {
		return nil, rpc.NewError("path_outside_scope", err.Error(), map[string]any{"path": pathText})
	}
	payload := []byte(content)
	if err := writeFileAtomic(target, payload); err != nil {
		return nil, rpc.NewError("invalid_transition", "repo.write failed: "+err.Error(), nil)
	}
	sum := sha256.Sum256(payload)
	return map[string]any{
		"status":   "written",
		"path":     pathText,
		"bytes":    len(payload),
		"sha256":   hex.EncodeToString(sum[:]),
		"job_id":   jobID,
		"lease_id": leaseID,
	}, nil
}

func writeFileAtomic(target string, payload []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return fmt.Errorf("target is a directory")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".striatum-repo-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}
	cleanup = false
	return nil
}
