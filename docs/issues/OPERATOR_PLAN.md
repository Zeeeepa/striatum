author: operator

# Open GH Issues Operator Plan

Date: 2026-05-14
Branch: `striatum/gh-issues-parallel`

## Objective

Drive the scaffolded open-GH-issue work to completion with maximum parallelism
while keeping role work in role-owned implementation and review paths. The
operator records plan, routing, friction, commits, pushes, and checkpoint
handling; the operator does not author role artifacts or invent role verdicts.

## Current Runs

- GH #9/#10/#11 security hardening:
  `run_ba9f16af26204248b7f7d0a8e30ffa33`.
  Current blocker: `blk_82bb6b6033ef4abcab4393fe782171f6`.
- GH #12/#13 ergonomics polish:
  `run_1b89c643a3554bbaa86192e57bc5e791`.
  Current blocker: `blk_9df968ca407f4378b81936671634c739`.
- GH #14:
  `run_f02421257abd4463bbff7dd7fcae1df8`, completed.
- GH #15:
  `run_354cbdba2bbf4b449d90e5a456482d29`, completed.
- GH #17:
  `run_1238b55a412e42749901d43b901e50e5`, completed after one revision
  cycle.

## Execution Plan

1. Preserve the operator/control-plane decisions first.
   Commit the D103 daemon-MCP decision, operator initialization updates, and
   this operator plan/friction log as a small scoped commit.
2. Maximize parallelism in independent slices.
   Run native sub-agents for daemon test failures, static-asset URL scanning,
   GH #9 security gap implementation, and checkpoint-resolution runbook
   research.
3. Keep workflow role work in role scope.
   If a checkpoint can be resolved by requeueing or rerouting a role job, use
   the Striatum control plane. If a new role artifact is required, dispatch it
   to a role session rather than writing it as operator.
4. Treat actual product decisions as interview points only.
   If the control plane requires a human policy choice that cannot be encoded as
   workflow routing, ask a focused question. Otherwise proceed.
5. Verify before declaring progress.
   Run narrow tests from each parallel slice, then `make lint`,
   `make typecheck`, `make ui-test`, and `make test` when the slices converge.
6. Commit and push frequently.
   Use small coherent commits, preserve unrelated user changes, fetch and
   fast-forward from `main` when the worktree allows, and push after each
   coherent checkpoint.

## Non-Goals

- Do not edit `.striatum/` directly.
- Do not falsify bylines or convert operator notes into role artifacts.
- Do not stage unrelated dirty worktree changes into scoped commits.
- Do not resolve a human checkpoint by claiming a role accepted work that did
  not accept.
