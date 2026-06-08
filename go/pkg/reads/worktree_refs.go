package reads

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	worktreeAnchorNone        = "none"
	worktreeAnchorRunBranch   = "run_branch"
	worktreeAnchorJobPin      = "job_pin"
	worktreeAnchorUnreachable = "unreachable"
)

func addWorktreeAnchorProjection(ctx context.Context, row map[string]any) {
	projection := probeWorktreeAnchor(ctx, row)
	row["head"] = projection["head"]
	row["reachable"] = projection["reachable"]
	row["anchor"] = projection["anchor"]
	row["anchored_ref"] = projection["anchored_ref"]
	row["checked_refs"] = projection["checked_refs"]
	if errText := stringFrom(projection, "probe_error"); errText != "" {
		row["probe_error"] = errText
	}
}

func doctorWorktreeRefSafety(ctx context.Context, runner any, repositoryID string) (map[string]any, []string, []map[string]any) {
	block := map[string]any{
		"checked":   false,
		"worktrees": []map[string]any{},
	}
	if repositoryID == "" {
		return block, nil, nil
	}
	rows, err := collectRows(ctx, runner, `
		SELECT w.worktree_id, w.run_id, w.job_id, w.lease_id,
		       w.base_branch, w.worktree_path, w.state, w.created_at,
		       w.released_at, w.removed_at, j.workflow_job_id,
		       j.state AS job_state, r.repo_root, r.branch_name
		  FROM striatumd.job_worktrees w
		  JOIN striatumd.jobs j
		    ON j.repository_id = w.repository_id
		   AND j.job_id = w.job_id
		  JOIN striatumd.runs r
		    ON r.repository_id = w.repository_id
		   AND r.run_id = w.run_id
		 WHERE w.repository_id = $1
		   AND w.state IN ('active', 'abandoned')
		 ORDER BY w.created_at, w.worktree_id`,
		repositoryID,
	)
	if err != nil {
		block["error"] = err.Error()
		return block, nil, nil
	}
	block["checked"] = true
	problems := []string{}
	records := []map[string]any{}
	for _, row := range rows {
		addWorktreeAnchorProjection(ctx, row)
		if stringFrom(row, "anchor") != worktreeAnchorUnreachable {
			continue
		}
		worktreeID := stringFrom(row, "worktree_id")
		jobID := stringFrom(row, "job_id")
		head := stringFrom(row, "head")
		problems = append(problems, fmt.Sprintf(
			"worktree_head_unreachable.%s: worktree HEAD %s is not reachable from the run branch or refs/striatum pins; re-run work.complete to anchor while the worktree exists",
			worktreeID, head,
		))
		records = append(records, worktreeProblemRecord("worktree_head_unreachable", worktreeID, row))
		if stringFrom(row, "job_state") == "completed" {
			problems = append(problems, fmt.Sprintf(
				"job_completed_without_anchor.%s: completed job worktree HEAD %s is not reachable from a durable ref",
				jobID, head,
			))
			records = append(records, worktreeProblemRecord("job_completed_without_anchor", jobID, row))
		}
	}
	block["worktrees"] = rows
	return block, problems, records
}

func worktreeProblemRecord(check, id string, row map[string]any) map[string]any {
	return map[string]any{
		"check": check,
		"id":    id,
		"context": map[string]any{
			"worktree_id":  row["worktree_id"],
			"run_id":       row["run_id"],
			"job_id":       row["job_id"],
			"head":         row["head"],
			"anchor":       row["anchor"],
			"checked_refs": row["checked_refs"],
			"remediation":  "re-run work.complete to anchor while the worktree exists; use --force only to intentionally discard unanchored work",
		},
	}
}

func probeWorktreeAnchor(ctx context.Context, row map[string]any) map[string]any {
	result := map[string]any{
		"head":         nil,
		"reachable":    false,
		"anchor":       worktreeAnchorNone,
		"anchored_ref": nil,
		"checked_refs": []string{},
	}
	state := strings.TrimSpace(stringFrom(row, "state"))
	if state != "active" && state != "abandoned" {
		return result
	}
	repoRoot := strings.TrimSpace(stringFrom(row, "repo_root"))
	if repoRoot == "" {
		result["probe_error"] = "repo_root_missing"
		return result
	}
	worktreeRoot, err := readWorktreeTarget(repoRoot, stringFrom(row, "worktree_path"))
	if err != nil {
		result["probe_error"] = err.Error()
		return result
	}
	if _, err := os.Stat(worktreeRoot); err != nil {
		result["probe_error"] = "worktree_path_missing"
		return result
	}

	refs := durableWorktreeProbeRefs(ctx, repoRoot, row)
	result["checked_refs"] = refs
	head, err := readGitCommit(ctx, worktreeRoot, "HEAD")
	if err != nil {
		result["probe_error"] = "worktree_head_unavailable"
		return result
	}
	result["head"] = head
	if len(refs) == 0 {
		result["anchor"] = worktreeAnchorUnreachable
		return result
	}
	for _, ref := range refs {
		if readGitAncestor(ctx, repoRoot, head, ref) {
			result["reachable"] = true
			result["anchored_ref"] = ref
			if strings.HasPrefix(ref, "refs/heads/") {
				result["anchor"] = worktreeAnchorRunBranch
			} else {
				result["anchor"] = worktreeAnchorJobPin
			}
			return result
		}
	}
	result["anchor"] = worktreeAnchorUnreachable
	return result
}

func readWorktreeTarget(repoRoot, pathText string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("repo_root_invalid")
	}
	pathText = strings.TrimSpace(pathText)
	if pathText == "" {
		return "", fmt.Errorf("worktree_path_missing")
	}
	var target string
	if filepath.IsAbs(pathText) {
		target = filepath.Clean(pathText)
	} else {
		target = filepath.Clean(filepath.Join(root, filepath.FromSlash(pathText)))
	}
	worktreesRoot := filepath.Join(root, ".striatum", "worktrees")
	if !readPathWithin(worktreesRoot, target) {
		return "", fmt.Errorf("worktree_path_outside_scratch")
	}
	return target, nil
}

func readPathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func durableWorktreeProbeRefs(ctx context.Context, repoRoot string, row map[string]any) []string {
	refs := []string{}
	if branch := strings.TrimSpace(stringFrom(row, "branch_name")); branch != "" {
		refs = appendUniqueString(refs, branchToRef(branch))
	} else if branch := strings.TrimSpace(stringFrom(row, "base_branch")); branch != "" {
		refs = appendUniqueString(refs, branchToRef(branch))
	}
	runID := strings.TrimSpace(stringFrom(row, "run_id"))
	jobID := strings.TrimSpace(stringFrom(row, "job_id"))
	if runID != "" && jobID != "" {
		refs = appendUniqueString(refs, "refs/striatum/"+runID+"/"+jobID)
	}
	if runID != "" {
		out, err := readGitOutput(ctx, repoRoot, "for-each-ref", "--format=%(refname)", "refs/striatum/"+runID+"/")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					refs = appendUniqueString(refs, line)
				}
			}
		}
	}
	return refs
}

func branchToRef(branch string) string {
	if strings.HasPrefix(branch, "refs/") {
		return branch
	}
	return "refs/heads/" + branch
}

func appendUniqueString(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func readGitCommit(ctx context.Context, repoRoot, ref string) (string, error) {
	return readGitOutput(ctx, repoRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
}

func readGitAncestor(ctx context.Context, repoRoot, ancestor, descendantRef string) bool {
	if _, err := readGitCommit(ctx, repoRoot, descendantRef); err != nil {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "merge-base", "--is-ancestor", ancestor, descendantRef)
	return cmd.Run() == nil
}

func readGitOutput(ctx context.Context, repoRoot string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoRoot}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
