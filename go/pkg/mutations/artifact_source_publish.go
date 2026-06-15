package mutations

import (
	"context"
	"fmt"
	"sort"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// publishWorktreeSourceChanges implements the opt-in #287 source-change publish.
// When a repo-write job sets write_scope.publish_source_changes=true, the daemon
// (operator) commits the lane's IN-SCOPE source edits in the per-job worktree to
// the run branch alongside its declared artifacts, so a code_change dogfood's
// actual edits land on the run branch instead of being stranded in the per-job
// worktree (and lost to worktree gc once the run finalizes). It runs at
// work.complete AFTER the declared-artifact porter and BEFORE
// anchorActiveWorktreeForJob advances the run ref, so the anchor captures both
// commits. enforceWriteScopeClean has already proven every changed path is
// in-scope, so this only commits paths the attempt was authorized to write.
//
// This is the git-worktree form of RFC 0127 P1's daemon-owned change-set commit:
// it reuses the RFC 0125 porter plumbing (force-add past .gitignore + commit as
// the operator on the detached worktree HEAD). It is scoped to per-job-isolated
// jobs on purpose — a shared-checkout repo-write job has no per-job worktree to
// publish from, and the isolated worktree's fresh detached checkout means every
// changed path is unambiguously this attempt's write. Returns the published repo
// paths (for the provenance event), or nil when there is nothing to publish.
func publishWorktreeSourceChanges(ctx context.Context, runner any, repositoryID string, job map[string]any) ([]string, error) {
	scope := asMap(job["write_scope_json"])
	if !isRepoWriteScope(scope) || scope["publish_source_changes"] != true {
		return nil, nil
	}
	allowed := stringListFromAny(scope["allowed_paths"])
	if len(allowed) == 0 {
		// An unbounded write scope would publish an arbitrary diff; require an
		// explicit allowed_paths before committing source changes to the run branch.
		return nil, nil
	}
	forbidden := stringListFromAny(scope["forbidden_paths"])

	required, worktree, err := worktreeRequirementForJob(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if !required || worktree == nil {
		return nil, nil
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	target, err := worktreeTarget(repoRoot, fmt.Sprint(worktree["worktree_path"]))
	if err != nil {
		return nil, err
	}

	current, err := gitChangedPathSnapshots(ctx, target)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "source-change publish failed: "+err.Error(), nil)
	}
	if len(current) == 0 {
		return nil, nil
	}
	currentPaths := make([]string, 0, len(current))
	for _, item := range current {
		currentPaths = append(currentPaths, item.Path)
	}
	// A sibling lane's published artifact in a shared run is not this attempt's
	// write; exclude it exactly as the write-scope guard does.
	ignored, err := publishedRunArtifactIgnoredPaths(ctx, runner, repositoryID, target, job, currentPaths)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "source-change publish failed: "+err.Error(), nil)
	}
	// The per-job worktree is a fresh detached checkout (nil baseline), so every
	// changed path is this attempt's write; collect the in-scope, non-sibling set.
	paths := collectInScopeAuthoredPaths(current, nil, allowed, forbidden, ignored)
	if len(paths) == 0 {
		return nil, nil
	}

	// Stage only the paths that actually add cleanly; the returned list (and the
	// provenance event built from it) must reflect what was committed, not the
	// pre-staging candidate set.
	staged := make([]string, 0, len(paths))
	for _, p := range paths {
		repoPath, ok := cleanDurableArtifactPath(p)
		if !ok {
			continue
		}
		added, err := runGitWorktreeCommand(ctx, target, "add", "-f", "--", repoPath)
		if err != nil {
			return nil, err
		}
		if added.ExitCode == 0 {
			staged = append(staged, repoPath)
		}
	}
	if len(staged) == 0 {
		return nil, nil
	}
	// Everything staged may already match HEAD (e.g. the declared-artifact porter
	// committed it just before); don't author an empty commit.
	if diff, err := runGitWorktreeCommand(ctx, target, "diff", "--cached", "--quiet"); err != nil {
		return nil, err
	} else if diff.ExitCode == 0 {
		return nil, nil
	}
	committed, err := runGitWorktreeCommand(ctx, target,
		"-c", "user.name=striatum-porter", "-c", "user.email=porter@striatum.local",
		"commit", "-m", fmt.Sprintf("striatum: lane source changes (job %s)", fmt.Sprint(job["job_id"])))
	if err != nil {
		return nil, err
	}
	if committed.ExitCode != 0 {
		return nil, rpc.NewError("invalid_transition",
			gitWorktreeErrorMessage("daemon source-change publish commit failed", committed), nil)
	}
	return dedupeStrings(staged), nil
}

// detectStrandedInScopePaths computes the #297 stranded set at work.complete: the
// in-scope, attempt-authored changed paths in the per-job worktree that are
// NEITHER declared as expected_artifacts NOR published via the source-publish path
// (publishWorktreeSourceChanges). These are files the lane genuinely wrote inside
// its write_scope but that the publication step silently dropped — multi-file code
// slices stranding their tests/migrations untracked in the worktree.
//
// It is the loud, non-silent half of the #297 fix (D201). It is read-only: it
// commits nothing and never refuses; it only surfaces the paths so the caller can
// warn. The write-scope guard already proved every reported path is in-scope; the
// source-publish step (when opted in) already committed its own set; so the
// residual here is exactly what would otherwise vanish on worktree gc.
//
// Returns nil (no warning) for non-repo-write jobs, shared-checkout jobs (no
// per-job worktree to inspect), jobs with no allowed_paths, and jobs where every
// authored in-scope path is covered by a declared artifact or the published
// source set.
func detectStrandedInScopePaths(ctx context.Context, runner any, repositoryID string, job map[string]any, publishedSourcePaths []string) ([]string, error) {
	scope := asMap(job["write_scope_json"])
	if !isRepoWriteScope(scope) {
		return nil, nil
	}
	allowed := stringListFromAny(scope["allowed_paths"])
	if len(allowed) == 0 {
		// An unbounded write scope cannot define an "in-scope authored" set without
		// claiming the whole worktree; mirror the source-publish guard and stay quiet.
		return nil, nil
	}
	forbidden := stringListFromAny(scope["forbidden_paths"])

	required, worktree, err := worktreeRequirementForJob(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if !required || worktree == nil {
		// A shared-checkout job has no per-job worktree whose dirt is unambiguously
		// this attempt's write, so the #297 detection (which depends on the fresh
		// detached worktree's nil baseline) does not apply here.
		return nil, nil
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	target, err := worktreeTarget(repoRoot, fmt.Sprint(worktree["worktree_path"]))
	if err != nil {
		return nil, err
	}

	current, err := gitChangedPathSnapshots(ctx, target)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "stranded-path detection failed: "+err.Error(), nil)
	}
	if len(current) == 0 {
		return nil, nil
	}
	currentPaths := make([]string, 0, len(current))
	for _, item := range current {
		currentPaths = append(currentPaths, item.Path)
	}
	// A sibling lane's published artifact in a shared run is not this attempt's
	// write; exclude it exactly as the write-scope guard and source-publish do.
	ignored, err := publishedRunArtifactIgnoredPaths(ctx, runner, repositoryID, target, job, currentPaths)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "stranded-path detection failed: "+err.Error(), nil)
	}
	// The per-job worktree is a fresh detached checkout (nil baseline), so every
	// in-scope changed path is this attempt's write.
	authored := collectInScopeAuthoredPaths(current, nil, allowed, forbidden, ignored)
	if len(authored) == 0 {
		return nil, nil
	}

	// Covered = declared expected_artifacts (attempt-resolved) ∪ source-published
	// paths. The complement of authored is the stranded set.
	covered := publishedAndDeclaredPathSet(job, publishedSourcePaths)
	return strandedInScopeAuthoredPaths(authored, covered), nil
}

// publishedAndDeclaredPathSet collects the repo paths that DO reach the run branch
// at work.complete: the job's declared expected_artifacts (resolved against the
// current attempt's revision-cycle placement) plus whatever the source-publish step
// actually committed. Paths are normalized to the same key space as
// collectInScopeAuthoredPaths so the set difference is exact.
func publishedAndDeclaredPathSet(job map[string]any, publishedSourcePaths []string) map[string]bool {
	covered := map[string]bool{}
	attempt := jobAttemptValue(job["attempt"])
	for _, item := range resolveExpectedArtifactCycles(asList(job["expected_artifacts_json"]), attempt) {
		entry := asMap(item)
		if clean, ok := normalizeScopePath(fmt.Sprint(entry["path"])); ok {
			covered[clean] = true
		}
	}
	for _, p := range publishedSourcePaths {
		if clean, ok := normalizeScopePath(p); ok {
			covered[clean] = true
		}
	}
	return covered
}

// strandedInScopeAuthoredPaths returns the authored in-scope paths that are not in
// the covered (declared-or-published) set, sorted for stable reporting. This is the
// pure core of the #297 detection, factored out so it is directly testable without
// a worktree or database.
func strandedInScopeAuthoredPaths(authored []string, covered map[string]bool) []string {
	out := make([]string, 0)
	for _, p := range authored {
		clean, ok := normalizeScopePath(p)
		if !ok {
			clean = p
		}
		if covered[clean] {
			continue
		}
		out = append(out, clean)
	}
	out = dedupeStrings(out)
	sort.Strings(out)
	return out
}

// collectInScopeAuthoredPaths returns the changed paths attributable to the
// attempt that are within allowed_paths (and outside forbidden / not a sibling
// lane's published artifact) — the complement of writeScopeViolationsSinceBaseline.
// The attribution rules mirror that guard exactly: a path created during the
// attempt (absent from the claim-time baseline) or a tracked file mutated away
// from its baseline content is the attempt's; a pre-existing untracked file is
// not. With a nil baseline (a fresh per-job worktree) every in-scope changed path
// is the attempt's write.
func collectInScopeAuthoredPaths(current []gitPathSnapshot, baseline map[string]string, allowed, forbidden []string, ignored map[string]bool) []string {
	allowedMatchers := normalizedScopeMatchers(allowed)
	forbiddenMatchers := normalizedScopeMatchers(forbidden)
	out := make([]string, 0)
	for _, item := range current {
		clean, ok := normalizeScopePath(item.Path)
		if !ok {
			continue
		}
		if ignored[clean] {
			continue
		}
		if pathMatchesAny(clean, forbiddenMatchers) {
			continue
		}
		if len(allowedMatchers) == 0 || !pathMatchesAny(clean, allowedMatchers) {
			continue
		}
		baselineHash, inBaseline := baseline[item.Path]
		if !inBaseline {
			out = append(out, clean)
			continue
		}
		if !item.Untracked && baselineHash != item.Hash {
			out = append(out, clean)
		}
	}
	return dedupeStrings(out)
}
