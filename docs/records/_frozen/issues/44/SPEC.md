# GH #44 - workflow details page write-isolation warning

Source: https://github.com/halbritt/striatum/issues/44

## Summary

The workflow details page surfaced `repo_write_without_worktree_isolation` for
serial issue-workflow `triage` and `fix` jobs even when there is no concurrent
repo-write execution to collide with the main worktree.

## Acceptance

1. Serial workflows do not report the parallel-worktree collision warning.
2. Workflows that can run repo-write jobs concurrently still report the warning
   unless the lane declares per-job worktree isolation.
3. Tests cover both serial and concurrent cases.
