---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["adapter_boundary", "todo_2", "workflow_validation"]
---

# Adapter Boundary Review
author: todo2-adapter-codex-001

## Verdict

`accept_with_findings`.

The closure preserves the current adapter boundary. It does not claim
enforced network or repository/filesystem isolation for the `process` adapter,
and it explicitly treats per-job worktrees as collaboration isolation rather
than sandboxing.

## Findings

F1 (low): `docs/TODO.md` still points to the older adapter-side matrix
location under `src/striatum/db.py`. Current source centralizes this in
`src/striatum/repo_policy.py`. The shared TODO should be refreshed by the
operator after this closure packet is accepted.

F2 (info): The future enforced-isolation work needs a new RFC, not a hidden
expansion of the `process` adapter. That RFC should define OS containment,
network isolation, filesystem namespace behavior, portability, and recovery.
