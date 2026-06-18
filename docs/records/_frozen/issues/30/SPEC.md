# GH #30 - stale repo-write lease requeue recovery

Source: https://github.com/halbritt/striatum/issues/30

## Summary

Repo-write jobs can land in `stale_lease`, but the operator recovery path used
to refuse them and point back at itself. The recovery surface needs an explicit
force path with an audit rationale after operator worktree inspection.

## Acceptance

1. `recovery requeue-stale` supports a repo-write override after explicit
   operator confirmation.
2. The override records a recovery event with the supplied justification.
3. Refusal messages point at the override or inspection workflow, not at a
   circular command path.
