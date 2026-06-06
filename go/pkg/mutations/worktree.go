package mutations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

const (
	worktreeStateDir = ".striatum"
	worktreeSubdir   = "worktrees"
)

type worktreeCreateInputs struct {
	Job        map[string]any
	RunID      string
	BaseBranch string
	// BranchBase is the run's recorded branch_base (the commit/ref the confirmed
	// branch should fork from). It may be empty when the branch was confirmed in
	// records_only mode; the ref-ensure path then falls back to HEAD (#183).
	BranchBase string
}

type gitWorktreeResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

var runGitWorktreeCommand = defaultRunGitWorktreeCommand

func HandleWorktreeCreate(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
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
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}

	var inputs worktreeCreateInputs
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		validated, err := validatedWorktreeCreateInputs(ctx, tx, repositoryID, sessionID, jobID, leaseID)
		if err != nil {
			return nil, err
		}
		inputs = validated
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}

	worktreeID, err := newID("wt")
	if err != nil {
		return nil, err
	}
	relative := filepath.ToSlash(filepath.Join(worktreeStateDir, worktreeSubdir, worktreeID))
	target, err := worktreeTarget(repoRoot, relative)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}

	if err := ensureWorktreeBaseBranchRef(ctx, repoRoot, inputs.BaseBranch, inputs.BranchBase); err != nil {
		return nil, err
	}

	result, err := runGitWorktreeCommand(ctx, repoRoot, "worktree", "add", "--detach", target, inputs.BaseBranch)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, rpc.NewError("invalid_transition", gitWorktreeErrorMessage("git worktree add failed", result), nil)
	}

	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		validated, err := validatedWorktreeCreateInputs(ctx, tx, repositoryID, sessionID, jobID, leaseID)
		if err != nil {
			return nil, err
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.job_worktrees (
			  repository_id, worktree_id, run_id, job_id, lease_id,
			  base_branch, worktree_path, state, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)`,
			repositoryID,
			worktreeID,
			validated.RunID,
			jobID,
			leaseID,
			validated.BaseBranch,
			relative,
			now,
		); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, validated.RunID, "worktree.created", sessionID, jobID, nil, nil, leaseID, map[string]any{
			"worktree_id":   worktreeID,
			"worktree_path": relative,
			"base_branch":   validated.BaseBranch,
		}); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}

	return map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": relative,
		"base_branch":   inputs.BaseBranch,
	}, nil
}

// ensureWorktreeBaseBranchRef makes the confirmed branch ref exist before
// `git worktree add --detach <branch>` resolves it. `branch confirm` in its
// default records_only mode never creates the git ref (run.go HandleBranchConfirm),
// so a per-job worktree create against a confirmed-but-refless branch failed with
// "invalid reference: <branch>" (#183). When the ref is missing, create it with
// `git branch <name> <base>` at the run's recorded branch_base (or HEAD when
// branch_base was not recorded) — NEVER `git checkout -b`: the daemon must never
// move the operator's primary checkout HEAD. A later RFC (0117, per-job worktree
// & branch ref-safety) will supersede the broader branch-confirmation lifecycle;
// this is an independently-safe standalone fix that only adds the missing ref and
// leaves HEAD untouched.
func ensureWorktreeBaseBranchRef(ctx context.Context, repoRoot, branch, branchBase string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	// Already a resolvable ref (branch, tag, or commit)? Nothing to do.
	verify, err := runGitWorktreeCommand(ctx, repoRoot, "rev-parse", "--verify", "--quiet", branch+"^{commit}")
	if err != nil {
		return err
	}
	if verify.ExitCode == 0 {
		return nil
	}
	base := strings.TrimSpace(branchBase)
	if base == "" {
		base = "HEAD"
	}
	created, err := runGitWorktreeCommand(ctx, repoRoot, "branch", branch, base)
	if err != nil {
		return err
	}
	if created.ExitCode != 0 {
		// A concurrent worktree create for the same run may have created the
		// branch between our probe and this create; if the ref resolves now,
		// the goal (ref exists) is met — losing that race is not a failure.
		verify, verr := runGitWorktreeCommand(ctx, repoRoot, "rev-parse", "--verify", "--quiet", branch+"^{commit}")
		if verr == nil && verify.ExitCode == 0 {
			return nil
		}
		return rpc.NewError("invalid_transition", gitWorktreeErrorMessage("git branch create for worktree base failed", created), nil)
	}
	return nil
}

func HandleWorktreeRelease(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	worktreeID, err := requiredStringParam(envelope.Params, "worktree_id")
	if err != nil {
		return nil, err
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}

	var row map[string]any
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		loaded, err := worktreeRow(ctx, tx, repositoryID, worktreeID, false)
		if err != nil {
			return nil, err
		}
		row = loaded
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}
	if fmt.Sprint(row["state"]) != "active" {
		return map[string]any{
			"status":      "already_released",
			"worktree_id": worktreeID,
			"state":       fmt.Sprint(row["state"]),
		}, nil
	}

	target, err := worktreeTarget(repoRoot, fmt.Sprint(row["worktree_path"]))
	if err != nil {
		return nil, err
	}
	result, err := runGitWorktreeCommand(ctx, repoRoot, "worktree", "remove", "--force", target)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 && pathExists(target) {
		return nil, rpc.NewError("invalid_transition", gitWorktreeErrorMessage("git worktree remove failed", result), nil)
	}

	var releaseResult map[string]any
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		loaded, err := worktreeRow(ctx, tx, repositoryID, worktreeID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(loaded["state"]) != "active" {
			releaseResult = map[string]any{
				"status":      "already_released",
				"worktree_id": worktreeID,
				"state":       fmt.Sprint(loaded["state"]),
			}
			return map[string]any{}, nil
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.job_worktrees
			   SET state = 'removed', released_at = $1, removed_at = $2
			 WHERE repository_id = $3 AND worktree_id = $4`,
			now,
			now,
			repositoryID,
			worktreeID,
		); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, loaded["run_id"], "worktree.released", nil, loaded["job_id"], nil, nil, loaded["lease_id"], map[string]any{
			"worktree_id":   worktreeID,
			"worktree_path": fmt.Sprint(loaded["worktree_path"]),
		}); err != nil {
			return nil, err
		}
		releaseResult = map[string]any{
			"status":      "released",
			"worktree_id": worktreeID,
			"state":       "removed",
		}
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}
	return releaseResult, nil
}

func validatedWorktreeCreateInputs(ctx context.Context, runner any, repositoryID, sessionID, jobID, leaseID string) (worktreeCreateInputs, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return worktreeCreateInputs{}, err
	}
	lease, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID)
	if err != nil {
		return worktreeCreateInputs{}, err
	}
	session, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", sessionID, true)
	if err != nil {
		return worktreeCreateInputs{}, err
	}
	if fmt.Sprint(session["state"]) != "active" {
		return worktreeCreateInputs{}, rpc.NewError("lease_error", "lease owner session is not active", nil)
	}
	if fmt.Sprint(lease["resource_type"]) != "job" {
		return worktreeCreateInputs{}, rpc.NewError("lease_error", "lease does not belong to a job", nil)
	}
	if fmt.Sprint(lease["run_id"]) != fmt.Sprint(job["run_id"]) {
		return worktreeCreateInputs{}, rpc.NewError("lease_error", "lease does not belong to the job run", nil)
	}
	if !isRepoWrite(job) {
		return worktreeCreateInputs{}, rpc.NewError("invalid_transition", "worktree create requires a repo-write job", nil)
	}
	if active, err := activeWorktreeForJob(ctx, runner, repositoryID, jobID); err != nil {
		return worktreeCreateInputs{}, err
	} else if active != nil {
		return worktreeCreateInputs{}, rpc.NewError("invalid_transition", "job already has an active worktree", nil)
	}

	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), true)
	if err != nil {
		return worktreeCreateInputs{}, err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return worktreeCreateInputs{}, err
	}
	workflow := asMap(snapshot["workflow_json"])
	laneID := jobLaneID(job)
	if laneWorktreeIsolation(workflow, laneID) != "per_job" {
		return worktreeCreateInputs{}, rpc.NewError("invalid_transition", "lane is not configured for worktree_isolation: per_job", nil)
	}
	baseBranch := fmt.Sprint(run["branch_name"])
	if baseBranch == "" || baseBranch == "<nil>" {
		return worktreeCreateInputs{}, rpc.NewError("invalid_transition", "run has no confirmed branch for worktree base", nil)
	}
	if nullable(run["branch_confirmed_at"]) == nil {
		return worktreeCreateInputs{}, rpc.NewError("invalid_transition", "run branch must be confirmed before worktree create", nil)
	}
	branchBase := strings.TrimSpace(fmt.Sprint(run["branch_base"]))
	if branchBase == "<nil>" {
		branchBase = ""
	}
	return worktreeCreateInputs{
		Job:        job,
		RunID:      fmt.Sprint(job["run_id"]),
		BaseBranch: baseBranch,
		BranchBase: branchBase,
	}, nil
}

func activeRepositoryRoot(ctx context.Context, runner any, repositoryID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT repo_root
		  FROM striatumd.repositories
		 WHERE repository_id = $1 AND state = 'active'`,
		repositoryID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", rpc.NewError("repo_not_registered", "daemon RPC repository is not registered", nil)
	}
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(fmt.Sprint(row["repo_root"]))
	if root == "" || root == "<nil>" {
		return "", rpc.NewError("invalid_transition", "registered repository has no repo_root", nil)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func activeWorktreeForJob(ctx context.Context, runner any, repositoryID, jobID string) (map[string]any, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT *
		  FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND job_id = $2 AND state = 'active'
		 LIMIT 1
		 FOR UPDATE`,
		repositoryID,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func worktreeRow(ctx context.Context, runner any, repositoryID, worktreeID string, forUpdate bool) (map[string]any, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row, err := oneRow(ctx, runner, `
		SELECT *
		  FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND worktree_id = $2`+suffix,
		repositoryID,
		worktreeID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rpc.NewError("not_found", fmt.Sprintf("could not find job_worktrees row for %q", worktreeID), nil)
	}
	return row, err
}

func jobLaneID(job map[string]any) string {
	lane, _ := asMap(job["lane_selector_json"])["lane_id"].(string)
	return lane
}

func worktreeTarget(repoRoot string, pathText string) (string, error) {
	if strings.TrimSpace(pathText) == "" {
		return "", rpc.NewError("invalid_transition", "worktree path must stay under .striatum/worktrees", nil)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	raw := filepath.FromSlash(pathText)
	target := raw
	if !filepath.IsAbs(raw) {
		target = filepath.Join(root, raw)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	target = filepath.Clean(target)
	if !pathWithin(root, target) {
		return "", rpc.NewError("invalid_transition", "worktree path must stay inside the repository", nil)
	}
	worktreesRoot := filepath.Join(root, worktreeStateDir, worktreeSubdir)
	if worktreePathHasSymlinkComponent(root, worktreesRoot) || worktreePathHasSymlinkComponent(root, target) {
		return "", rpc.NewError("invalid_transition", "worktree path must not traverse symlinks", nil)
	}
	if !pathWithin(worktreesRoot, target) {
		return "", rpc.NewError("invalid_transition", "worktree path must stay under .striatum/worktrees", nil)
	}
	return target, nil
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func worktreePathHasSymlinkComponent(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return true
	}
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return !errors.Is(err, os.ErrNotExist)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func defaultRunGitWorktreeCommand(ctx context.Context, repoRoot string, args ...string) (gitWorktreeResult, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := gitWorktreeResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func gitWorktreeErrorMessage(prefix string, result gitWorktreeResult) string {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if len(message) > 200 {
		message = message[:200] + "..."
	}
	if message == "" {
		return prefix
	}
	return prefix + ": " + message
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func requiredStringParam(params map[string]any, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", rpc.NewError("schema_invalid", key+" must be a non-empty string", nil)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", rpc.NewError("schema_invalid", key+" must be a non-empty string", nil)
	}
	return text, nil
}
