---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/deferred-19-rfc0056-layout-closure/source/MAP.md", "docs/rfcs/0056-consumer-repo-directory-structure-opinions.md", "docs/CONSUMER_REPO_LAYOUT.md", "tests/test_scaffold_ddd_layout.py"]
---

# RFC 0056 Optional Follow-Up Classification
author: rfc0056-layout-classifier-codex-gpt-5-001

## Classification

Deferred item 19 is closed as an explicit non-change for the RFC 0056 layout
scaffold.

The optional behavior should not be implemented inside
`init --with-striatum-layout`:

- workflow-file generation remains the job of `striatum workflow generate`,
  where the operator provides the output path and `--artifact-root`;
- artifact-root `.gitignore` policy remains operator-owned because Striatum
  supports both committed provenance artifacts and ignored ephemeral outputs.

## Decision Matrix

| Question | Classification | Rationale |
|---|---|---|
| Should `init --with-striatum-layout` create a workflow file? | No | The scaffold is directory-only. Workflow generation already has an explicit authoring command with validation and overwrite refusal. Creating a workflow from `init` would blur bootstrap layout with workflow selection. |
| Should `init --with-striatum-layout` add `striatum/<workflow-slug>/` to `.gitignore`? | No | Artifact roots may be durable provenance or ephemeral output. The runner cannot infer the commit policy safely from a layout flag. |
| Should `.striatum/` stay auto-ignored? | Yes | `.striatum/` is operational scratch only and `striatum init` already owns that ignore entry. This is separate from artifact-root policy. |
| Should future UI/generator defaults point at RFC 0056 paths? | Optional separate ergonomics work | Suggestions are safe when the operator still confirms an explicit path. They should not become hidden writes through the layout scaffold. |

## Closure Rule

Treat future requests for workflow-file generation or artifact-root ignore
edits as new workflow-authoring or adoption-UX proposals, not as unfinished RFC
0056 Phase B work. Any such proposal should preserve:

- explicit operator-selected workflow path;
- explicit operator-selected artifact root;
- validation before writing generated workflows;
- no artifact-root `.gitignore` edits without a dedicated operator flag and a
  clear preview/dry-run result.
