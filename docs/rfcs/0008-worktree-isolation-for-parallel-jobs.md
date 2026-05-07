# RFC 0008: Worktree Isolation for Parallel Jobs

Status: proposed
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
4.  **Automatic Cleanup:** Worktrees are removed after job completion or cancellation.

## Acceptance Criteria

- Parallel agents can run `npm install` or build commands without interfering with each other's `node_modules` or build artifacts.
- The main repository remains clean while agents work in their respective worktrees.
- `publish-artifact` correctly handles paths relative to the worktree root.

## Open Questions

- Should worktree isolation be a per-lane configuration or a global runner setting?
- How do we handle large repositories where `git worktree add` might be slow?
- Should we use ephemeral branches for each worktree to ensure total isolation?
