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
	// branch should fork from). It may be empty for pre-RFC 0117 runs; the
	// ref-ensure path then falls back to HEAD (#183 compatibility).
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
// `git worktree add --detach <branch>` resolves it. Pre-RFC 0117 runs may have
// a confirmed branch name but no actual ref (#183), so the create path repairs
// that state with `git branch <name> <base>` and never `git checkout -b`.
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
	force := boolParam(envelope, "force")
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
	reachability := worktreeReachability{Reachable: true}
	if pathExists(target) {
		reachability, err = worktreeHeadReachability(ctx, repoRoot, target, row)
		if err != nil {
			return nil, err
		}
		if !reachability.Reachable && !force {
			return nil, rpc.NewError("worktree_head_unreachable", fmt.Sprintf(
				"worktree %s HEAD %s is not reachable from the run branch or refs/striatum pins; re-run work.complete to anchor it, or pass --force to discard it explicitly",
				worktreeID, reachability.Head,
			), map[string]any{
				"worktree_id":  worktreeID,
				"head":         reachability.Head,
				"checked_refs": reachability.CheckedRefs,
			})
		}
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
		eventType := "worktree.released"
		status := "released"
		if force && !reachability.Reachable {
			eventType = "worktree.force_released"
			status = "force_released"
		}
		if _, err := appendEvent(ctx, tx, repositoryID, loaded["run_id"], eventType, nil, loaded["job_id"], nil, nil, loaded["lease_id"], map[string]any{
			"worktree_id":   worktreeID,
			"worktree_path": fmt.Sprint(loaded["worktree_path"]),
			"head":          nullableString(reachability.Head),
			"reachable":     reachability.Reachable,
			"checked_refs":  reachability.CheckedRefs,
		}); err != nil {
			return nil, err
		}
		releaseResult = map[string]any{
			"status":      status,
			"worktree_id": worktreeID,
			"state":       "removed",
		}
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}
	return releaseResult, nil
}

type worktreeGCCheck struct {
	Remove       bool
	Reason       string
	Target       string
	Reachability worktreeReachability
}

func HandleWorktreeGC(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(stringParam(envelope, "run_id"))

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := lockRepo(ctx, tx, repositoryID); err != nil {
			return nil, err
		}
		args := []any{repositoryID}
		where := "WHERE w.repository_id = $1 AND w.state IN ('active', 'abandoned')"
		if runID != "" {
			args = append(args, runID)
			where += " AND w.run_id = $2"
		}
		rows, err := queryRows(ctx, tx, `
			SELECT w.worktree_id, w.run_id, w.job_id, w.lease_id,
			       w.base_branch, w.worktree_path, w.state,
			       j.state AS job_state, j.workflow_job_id,
			       r.repo_root, r.branch_name
			  FROM striatumd.job_worktrees w
			  JOIN striatumd.jobs j
			    ON j.repository_id = w.repository_id
			   AND j.job_id = w.job_id
			  JOIN striatumd.runs r
			    ON r.repository_id = w.repository_id
			   AND r.run_id = w.run_id
			  `+where+`
			 ORDER BY w.created_at, w.worktree_id
			 FOR UPDATE OF w`, args...)
		if err != nil {
			return nil, err
		}

		removed := []map[string]any{}
		skipped := []map[string]any{}
		for _, row := range rows {
			repoRoot := strings.TrimSpace(fmt.Sprint(row["repo_root"]))
			check, err := worktreeGCDecision(ctx, repoRoot, row)
			base := map[string]any{
				"worktree_id":     row["worktree_id"],
				"worktree_path":   row["worktree_path"],
				"run_id":          row["run_id"],
				"job_id":          row["job_id"],
				"workflow_job_id": row["workflow_job_id"],
				"state":           row["state"],
				"job_state":       row["job_state"],
			}
			if err != nil {
				base["reason"] = check.Reason
				if check.Reason == "" {
					base["reason"] = "probe_failed"
				}
				base["error"] = err.Error()
				skipped = append(skipped, base)
				continue
			}
			if !check.Remove {
				base["reason"] = check.Reason
				base["head"] = nullableString(check.Reachability.Head)
				base["reachable"] = check.Reachability.Reachable
				base["checked_refs"] = check.Reachability.CheckedRefs
				skipped = append(skipped, base)
				continue
			}

			result, err := runGitWorktreeCommand(ctx, repoRoot, "worktree", "remove", "--force", check.Target)
			if err != nil {
				return nil, err
			}
			if result.ExitCode != 0 && pathExists(check.Target) {
				return nil, rpc.NewError("invalid_transition", gitWorktreeErrorMessage("git worktree remove failed", result), nil)
			}
			now := nowString()
			if err := tx.Exec(ctx, `
				UPDATE striatumd.job_worktrees
				   SET state = 'removed',
				       released_at = COALESCE(released_at, $1),
				       removed_at = $2
				 WHERE repository_id = $3
				   AND worktree_id = $4
				   AND state IN ('active', 'abandoned')`,
				now, now, repositoryID, row["worktree_id"]); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, row["run_id"], "worktree.gc_removed", nil, row["job_id"], nil, nil, row["lease_id"], map[string]any{
				"worktree_id":   row["worktree_id"],
				"worktree_path": row["worktree_path"],
				"head":          nullableString(check.Reachability.Head),
				"reachable":     check.Reachability.Reachable,
				"checked_refs":  check.Reachability.CheckedRefs,
				"job_state":     row["job_state"],
			}); err != nil {
				return nil, err
			}
			base["status"] = "removed"
			base["head"] = nullableString(check.Reachability.Head)
			base["reachable"] = check.Reachability.Reachable
			base["checked_refs"] = check.Reachability.CheckedRefs
			removed = append(removed, base)
		}
		return map[string]any{
			"status":        "ok",
			"removed_count": len(removed),
			"skipped_count": len(skipped),
			"removed":       removed,
			"skipped":       skipped,
		}, nil
	})
}

func worktreeGCDecision(ctx context.Context, repoRoot string, row map[string]any) (worktreeGCCheck, error) {
	if !terminalJobStates[fmt.Sprint(row["job_state"])] {
		return worktreeGCCheck{Reason: "job_not_terminal"}, nil
	}
	target, err := worktreeTarget(repoRoot, fmt.Sprint(row["worktree_path"]))
	if err != nil {
		return worktreeGCCheck{Reason: "invalid_path"}, err
	}
	if !pathExists(target) {
		return worktreeGCCheck{Reason: "missing_on_disk", Target: target}, nil
	}
	reachability, err := worktreeHeadReachability(ctx, repoRoot, target, row)
	if err != nil {
		return worktreeGCCheck{Reason: "probe_failed", Target: target}, err
	}
	if !reachability.Reachable {
		return worktreeGCCheck{Reason: "head_unreachable", Target: target, Reachability: reachability}, nil
	}
	return worktreeGCCheck{Remove: true, Target: target, Reachability: reachability}, nil
}

func anchorActiveWorktreeForJob(ctx context.Context, runner any, repositoryID string, job map[string]any) (map[string]any, error) {
	if !isRepoWrite(job) {
		return nil, nil
	}
	required, worktree, err := worktreeRequirementForJob(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if required && worktree == nil {
		return nil, worktreeRequiredError(job, "work.complete")
	}
	if worktree == nil {
		return nil, err
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return nil, err
	}
	runBranch := strings.TrimSpace(fmt.Sprint(run["branch_name"]))
	if runBranch == "" || runBranch == "<nil>" || nullable(run["branch_confirmed_at"]) == nil {
		return nil, rpc.NewError("invalid_transition", "repo-write worktree commits require a confirmed run branch before completion", nil)
	}
	payload, err := anchorWorktreeCommitStack(ctx, repoRoot, fmt.Sprint(job["run_id"]), fmt.Sprint(job["job_id"]), runBranch, worktree)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func requireActiveWorktreeForJob(ctx context.Context, runner any, repositoryID string, job map[string]any, surface string) (map[string]any, error) {
	required, worktree, err := worktreeRequirementForJob(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if required && worktree == nil {
		return nil, worktreeRequiredError(job, surface)
	}
	return worktree, nil
}

func worktreeRequirementForJob(ctx context.Context, runner any, repositoryID string, job map[string]any) (bool, map[string]any, error) {
	if !isRepoWrite(job) {
		return false, nil, nil
	}
	worktree, err := activeWorktreeForJob(ctx, runner, repositoryID, fmt.Sprint(job["job_id"]))
	if err != nil {
		return false, nil, err
	}
	laneID := jobLaneID(job)
	if strings.TrimSpace(laneID) == "" {
		return false, worktree, nil
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return false, nil, err
	}
	snapshotID := strings.TrimSpace(fmt.Sprint(run["workflow_snapshot_id"]))
	if snapshotID == "" || snapshotID == "<nil>" {
		return false, worktree, nil
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", snapshotID, false)
	if err != nil {
		return false, nil, err
	}
	workflow := asMap(snapshot["workflow_json"])
	required := laneWorktreeIsolation(workflow, laneID) == "per_job"
	return required, worktree, nil
}

func worktreeRequiredError(job map[string]any, surface string) error {
	jobID := strings.TrimSpace(fmt.Sprint(job["job_id"]))
	workflowJobID := strings.TrimSpace(fmt.Sprint(job["workflow_job_id"]))
	laneID := strings.TrimSpace(jobLaneID(job))
	message := fmt.Sprintf("%s requires an active per-job worktree for repo-write job %s before touching repository files", surface, jobID)
	if workflowJobID != "" && workflowJobID != "<nil>" {
		message += fmt.Sprintf(" (%s)", workflowJobID)
	}
	message += "; run worktree.create with the active session/job/lease from the work packet, then retry"
	return rpc.NewError("worktree_required", message, map[string]any{
		"job_id":          jobID,
		"workflow_job_id": nullableString(workflowJobID),
		"lane_id":         nullableString(laneID),
		"surface":         surface,
		"required_action": "worktree.create",
	})
}

func anchorWorktreeCommitStack(ctx context.Context, repoRoot, runID, jobID, runBranch string, worktree map[string]any) (map[string]any, error) {
	target, err := worktreeTarget(repoRoot, fmt.Sprint(worktree["worktree_path"]))
	if err != nil {
		return nil, err
	}
	head, err := gitRevParseCommit(ctx, target, "HEAD")
	if err != nil {
		return nil, err
	}
	runRef := "refs/heads/" + runBranch
	payload := map[string]any{
		"anchor":        "none",
		"head":          head,
		"worktree_id":   worktree["worktree_id"],
		"worktree_path": worktree["worktree_path"],
		"run_branch":    runBranch,
		"run_ref":       runRef,
	}
	runTip, err := gitRevParseCommit(ctx, repoRoot, runRef)
	if err != nil {
		payload["run_branch_missing"] = true
		return pinWorktreeCommitStack(ctx, repoRoot, runID, jobID, head, payload)
	}
	if head == runTip {
		payload["reason"] = "head_already_at_run_branch"
		return payload, nil
	}
	if ok, err := gitIsAncestor(ctx, repoRoot, runTip, head); err != nil {
		return nil, err
	} else if ok {
		out, exit, err := integrateGit(ctx, repoRoot, "update-ref", runRef, head, runTip)
		if err != nil {
			return nil, err
		}
		if exit == 0 {
			payload["anchor"] = "run_branch_ff"
			payload["from"] = runTip
			payload["to"] = head
			return payload, nil
		}
		payload["ff_failed"] = strings.TrimSpace(out)
	}
	return pinWorktreeCommitStack(ctx, repoRoot, runID, jobID, head, payload)
}

func pinWorktreeCommitStack(ctx context.Context, repoRoot, runID, jobID, head string, payload map[string]any) (map[string]any, error) {
	pinRef := "refs/striatum/" + runID + "/" + jobID
	out, exit, err := integrateGit(ctx, repoRoot, "update-ref", pinRef, head)
	if err != nil {
		return nil, err
	}
	if exit != 0 {
		return nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf("update-ref of %q failed while anchoring worktree commits: %s", pinRef, strings.TrimSpace(out)), nil)
	}
	payload["anchor"] = "job_pin"
	payload["pin_ref"] = pinRef
	return payload, nil
}

type worktreeReachability struct {
	Head        string
	Reachable   bool
	CheckedRefs []string
}

func worktreeHeadReachability(ctx context.Context, repoRoot, worktreeRoot string, row map[string]any) (worktreeReachability, error) {
	head, err := gitRevParseCommit(ctx, worktreeRoot, "HEAD")
	if err != nil {
		return worktreeReachability{}, err
	}
	refs := durableWorktreeRefs(ctx, repoRoot, row)
	for _, ref := range refs {
		ok, err := gitIsAncestor(ctx, repoRoot, head, ref)
		if err != nil {
			return worktreeReachability{}, err
		}
		if ok {
			return worktreeReachability{Head: head, Reachable: true, CheckedRefs: refs}, nil
		}
	}
	return worktreeReachability{Head: head, Reachable: false, CheckedRefs: refs}, nil
}

func durableWorktreeRefs(ctx context.Context, repoRoot string, row map[string]any) []string {
	seen := map[string]bool{}
	refs := []string{}
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			return
		}
		if _, err := gitRevParseCommit(ctx, repoRoot, ref); err == nil {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	if branch := strings.TrimSpace(fmt.Sprint(row["base_branch"])); branch != "" && branch != "<nil>" {
		add("refs/heads/" + branch)
	}
	runID := strings.TrimSpace(fmt.Sprint(row["run_id"]))
	jobID := strings.TrimSpace(fmt.Sprint(row["job_id"]))
	if runID != "" && runID != "<nil>" && jobID != "" && jobID != "<nil>" {
		add("refs/striatum/" + runID + "/" + jobID)
	}
	if runID != "" && runID != "<nil>" {
		out, exit, err := integrateGit(ctx, repoRoot, "for-each-ref", "--format=%(refname)", "refs/striatum/"+runID+"/")
		if err == nil && exit == 0 {
			for _, line := range strings.Split(out, "\n") {
				add(line)
			}
		}
	}
	return refs
}

func gitRevParseCommit(ctx context.Context, repoRoot, rev string) (string, error) {
	out, exit, err := integrateGit(ctx, repoRoot, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	if exit != 0 {
		return "", rpc.NewError("invalid_transition", fmt.Sprintf("git commit %q does not resolve", rev), nil)
	}
	sha := firstLine(out)
	if !isFullGitSHA(sha) {
		return "", rpc.NewError("invalid_transition", fmt.Sprintf("git commit %q resolved to invalid sha %q", rev, sha), nil)
	}
	return sha, nil
}

func gitIsAncestor(ctx context.Context, repoRoot, ancestor, descendant string) (bool, error) {
	_, exit, err := integrateGit(ctx, repoRoot, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		return false, err
	}
	switch exit {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, rpc.NewError("git_commit_apply_failed", fmt.Sprintf("merge-base --is-ancestor failed for %s -> %s", ancestor, descendant), nil)
	}
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
