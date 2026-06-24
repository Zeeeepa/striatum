# Original User Request

## Initial Request — 2026-06-10T21:41:13Z

Load and execute the GIT_HYGIENE cleaning prompt to verify and remove safe-to-delete git debris from the target repository, leaving a recovery map.

Working directory: ~/teamwork_projects/git_hygiene
Integrity mode: development

## Requirements

### R1. Target Repository and Preflight
- The target repository is located at `~/git/striatum`.
- Resolve the default branch (`git symbolic-ref refs/remotes/origin/HEAD` or fallback).
- Run preflight checks: fetch all and prune, check for other contributors, note `git status`, and snapshot the full ref state.
- Authority for remote branch deletions (`git push --delete`) is **explicitly granted** for verified merged or gone branches on remotes the maintainer owns.

### R2. Clean Safe-to-Delete git Refs and Debris
Work category by category and delete verified safe-to-delete debris from the target repository:
- Merged local/remote branches (fully reachable from the default branch).
- Patch-equivalent local/remote branches (content already merged via squash/rebase).
- Gone remote-tracking refs.
- Prunable worktrees (clean tree, checked-out branch merged).
- Landed stashes (stash content already in HEAD or empty).
- Scratch tags (reachable from kept refs, not release-shaped).
- Dead remotes (duplicated URLs or unreachable).
Every deletion must cite exactly one evidence class with command outputs.

### R3. Safe Deferrals and Stop Conditions
Do not delete refs with unique work (commits not reachable from a kept ref), worktrees with uncommitted changes, stashes with unlanded content, release tags, or branches with open PRs. Defer these to the maintainer with a log summary and recommended verdict.

### R4. Report and Recovery Map
Create a markdown report named `<PROJECT_NAME>_GIT_HYGIENE_<MODEL_NAME>_YYYY-MM-DD.md` in the target repository root.
It must include:
- Category inventory counts (before and after).
- Executive summary of the cleanup.
- Recovery map listing every deleted ref, its tip SHA, last-commit date, subject, evidence class, and the exact recovery command.
- Detailed sections for done branches, other done items, deferred items, verified clean categories, and follow-ups.

### R5. Verification Script
Provide an automated verification script (e.g. bash or python) that can be run on the target repository to:
1. Confirm all branches/tags listed in the recovery map are indeed deleted.
2. Confirm the tip SHAs of deleted refs still exist in the repository object database (revertible).
3. Confirm that no protected refs (default branch, currently checked-out branch) were modified or deleted.

## Acceptance Criteria

### Execution Safety
- [ ] No history rewriting (no rebase, no force-push, no history modifications).
- [ ] No `git gc`, object pruning, or reflog expiration.
- [ ] Protected refs (default branch, currently checked-out branches, release tags, explicitly named kept refs) are untouched.
- [ ] Tip SHAs are recorded in the recovery map before deletion.

### Report and Artifacts
- [ ] The report file is created in `~/git/striatum` matching the naming convention: `STRIATUM_GIT_HYGIENE_<MODEL_NAME>_YYYY-MM-DD.md`.
- [ ] Every deleted ref listed in the recovery map has a valid recovery command.
- [ ] The verification script is created in the working directory and runs successfully.
