# RFC 0008: Worktree Isolation for Parallel Jobs

Status: accepted
Date: 2026-05-07

## Problem

In V1, multiple agents working on the same branch share the same repository root. While the runner enforces disjoint "write scopes" via workflow config, it cannot prevent agents from seeing each other's intermediate uncommitted changes. This can lead to non-deterministic behavior, build failures, or agents accidentally incorporating each other's half-finished work into their own artifacts.

## Goals

- Provide true filesystem isolation for parallel "write" jobs.
- Ensure that agents only see the state of the branch as it was at the start of their job (plus any explicitly provided inputs).
- Leverage native Git features for efficiency.

## Non-Goals

- Do not implement a custom virtual filesystem.
- Do not handle merge conflicts between parallel jobs; synthesis jobs remain responsible for merging outputs.

## Proposal

1.  **Git Worktree Integration:** When a job with a "write" capability is claimed, the runner can optionally create a `git worktree` in a temporary directory (e.g., `.striatum/worktrees/job_<id>`).
2.  **Isolated Execution:** The work packet provided to the agent includes the path to this isolated worktree as the `cwd`.
3.  **Artifact Retrieval:** Upon job completion, `publish-artifact` validates paths against the worktree, and the runner manages the collection of artifacts back to the main repository provenance area.
4.  **Safe Cleanup:** Operators can remove completed or abandoned on-disk
    worktrees after the daemon proves their HEAD is reachable from a durable
    ref.

## Acceptance Criteria

- Parallel agents can run `npm install` or build commands without interfering with each other's `node_modules` or build artifacts.
- The main repository remains clean while agents work in their respective worktrees.
- `publish-artifact` correctly handles paths relative to the worktree root.

## Open Questions

- Should worktree isolation be a per-lane configuration or a global runner setting?
- How do we handle large repositories where `git worktree add` might be slow?
- Should we use ephemeral branches for each worktree to ensure total isolation?

## Implementation Notes

V1 ships opt-in per-lane worktree isolation. The actual behavior:

- **Lane flag.** Lanes accept `worktree_isolation: "off" | "per_job"`; the
  default is `"off"`, which preserves prior single-worktree behavior. Workflow
  validation rejects any other value.
- **Repo-write only.** Per-job worktrees are produced exclusively for jobs
  whose `write_scope.repo_write` is true. Review-only and other read-mostly
  jobs continue to use the main worktree, since they do not generate the kind
  of intermediate state (build outputs, partial trees, dependency directories)
  that motivates isolation.
- **Explicit creation.** The runner does not auto-create worktrees on claim.
  Work packets advertise `worktree_required: true` and include a
  `commands.worktree_create` invocation. The agent runs `striatum worktree
  create` itself, which keeps the contract explicit and makes recovery
  predictable.
- **Detached HEAD checkout.** `striatum worktree create` runs
  `git worktree add --detach .striatum/worktrees/<worktree_id> <base_branch>`.
  Detached HEAD avoids conflict with the branch already checked out in the
  main worktree; the isolated worktree itself does not need a checked-out
  writable branch ref because RFC 0117 anchors completed commits through ref
  plumbing.
- **State table.** The daemon-owned PostgreSQL `job_worktrees` table tracks
  `worktree_id`, `run_id`, `job_id`, `lease_id`, `base_branch`,
  `worktree_path`, and `state` (`active`, `released`, `removed`,
  `abandoned`). A partial unique index enforces "at most one active worktree
  per job"; previous rows in terminal states are preserved as historical
  context.
- **Artifact publication.** When a job has an active worktree, `publish-artifact`
  validates file existence at `<worktree_path>/<logical_path>` but records the
  `repo_path` as the logical (repo-relative) path. Artifacts remain durable
  provenance for the main branch, and the same artifact rows are valid before,
  during, and after worktree-isolated execution.
- **Commit custody (RFC 0117).** When `work.complete` completes a repo-write
  job with an active per-job worktree, the daemon makes the worktree HEAD
  reachable from a durable git ref before completing the job: it
  fast-forwards the run branch when ancestry permits and otherwise pins the
  HEAD at `refs/striatum/<run_id>/<job_id>`. Branch creation is ref-only and
  never moves the operator's primary checkout.
- **Recovery on lease expiry.** Before a worktree row is marked `abandoned`,
  recovery anchors the worktree HEAD through the same fast-forward-or-pin
  helper and records the anchor in `worktree.abandoned`. The directory is left
  on disk so an operator can inspect uncommitted work, mirroring the existing
  repo-write stale-lease policy.
- **Doctor.** `striatum doctor` flags worktrees in `state = 'active'` whose
  lease is no longer active (orphaned), active worktrees whose
  `worktree_path` no longer exists on disk (filesystem drift), and active or
  abandoned worktrees whose HEAD is not reachable from the run branch or a
  `refs/striatum/` pin (`worktree_head_unreachable`; completed jobs also
  surface `job_completed_without_anchor`).
- **Inspection.** `striatum worktree list` returns the rows with the job's
  `workflow_job_id` plus ref-safety fields (`head`, `reachable`, `anchor`,
  `anchored_ref`, `checked_refs`). `striatum worktree release` removes the
  directory via `git worktree remove --force` only after the worktree HEAD is
  reachable from the run branch or a `refs/striatum/` pin; `--force` is the
  explicit audited discard path. Releasing an already-terminal row is a no-op.
- **Garbage collection.** `striatum worktree gc [--run-id <id>]` removes only
  on-disk worktree directories for terminal jobs whose HEAD is reachable from
  the run branch or a `refs/striatum/` pin. It reports skipped rows with
  reasons and emits `worktree.gc_removed` for removals.

The first three open questions in this RFC are now resolved as: per-lane
configuration; large-repository performance is left for an operator-level
benchmark or future RFC; ephemeral branches were rejected in favor of
detached-HEAD checkouts so the runner does not silently create branch refs.
