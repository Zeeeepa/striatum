---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/UBIQUITOUS_LANGUAGE.md", "src/striatum/repo_policy.py", "src/striatum/workflow.py", "src/striatum/daemon_pg/handlers/context.py", "tests/test_cli_mvp.py"]
---

# TODO 2 Adapter Constraint Map
author: todo2-adapter-codex-001

## Current Behavior

TODO 2's accepted implementation model is present in source:

- `src/striatum/workflow.py` validates known constraint names and values,
  validates `required_enforcement`, rejects undeclared enforcement requests,
  and refuses workflows that require a stronger level than the adapter can
  provide.
- `src/striatum/repo_policy.py` centralizes the current enforcement matrix:
  the `process` adapter reports `transcripts=off` as `enforced`, and reports
  `network=forbidden` plus `repo_scope=local_only` as `advisory_strict`.
- `src/striatum/daemon_pg/handlers/context.py` projects requested
  constraints, required levels, actual enforcement, and satisfaction into work
  packets.
- Existing tests in `tests/test_cli_mvp.py` cover validation refusal,
  work-packet projection, and proxy-environment scrubbing/sentinel env vars
  for `network=forbidden` and `repo_scope=local_only`.

## Remaining Gap

The remaining gap is not a safe process-adapter patch. The process adapter
can scrub proxy environment variables and set policy sentinels, but it cannot
guarantee OS-level network denial or filesystem containment. Per-job git
worktrees isolate checkout state for collaboration, but they are not a
filesystem sandbox and do not make network access impossible.

Mechanically promoting `network` or `repo_scope` from `advisory_strict` to
`enforced` needs a new adapter/RFC that defines local containment primitives,
platform support, failure behavior, recovery semantics, and operator UX.

## Closure Position

TODO 2 should close for the current process-adapter scope. The residual
enforced network/filesystem isolation work should move to a separate future
RFC or TODO item rather than keeping the current item open as if a small
source patch remains.
