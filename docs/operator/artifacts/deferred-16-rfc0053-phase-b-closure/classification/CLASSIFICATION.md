---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/deferred-16-rfc0053-phase-b-closure/surface/SURFACE_MAP.md", "docs/rfcs/0053-human-principal-and-terminology-truing.md", "docs/TODO.md", "docs/ROADMAP.md", "src/striatum/workflow.py", "go/pkg/workflowauthoring/workflow.go", "go/pkg/mutations/run.go"]
---

# RFC 0053 Phase B Classification
author: deferred16-classifier-codex-gpt-5-001

## Classification

Status: blocked pending a scheduled schema/runtime migration.

RFC 0053 Phase B should not be treated as a generic terminology sweep. The
rename affects durable workflow JSON, generated templates, daemon database
constraints, live job/blocker transition code, read-model field names, and
Python/Go test fixtures. The first implementation step must be a versioned
compatibility plan, not search-and-replace.

## Exact Blockers

| Blocker | Why It Blocks |
|---|---|
| Workflow schema version not chosen | Current validators accept only v1 and v1.1. Phase B needs a new accepted version or a documented compatibility strategy for v1/v1.1 snapshots. |
| Upgrade rule missing | `workflow upgrade` has no rule to rewrite `root_review_needs_revision: "human_checkpoint"` or generated `human_checkpoint` jobs/shapes to the new vocabulary. |
| Database enum constraints still old | PostgreSQL migrations constrain job type, job state, queue message kind, and blocker severity to old values. Runtime writes of new values would fail without a migration. |
| Production Go daemon still writes old state | Go mutations still open `human_checkpoint` blockers and set jobs to `waiting_human`. Python handlers mirror the same vocabulary for tests and compatibility. |
| Read-model compatibility unresolved | Status/dashboard/escalation reads expose `human_checkpoints` and `resolve_human_checkpoint`. Renaming these response fields is an API compatibility choice beyond workflow JSON alone. |
| Generator/catalog compatibility unresolved | Built-in shape and block vocabulary still includes `human_checkpoint`. New workflows need a new shape/block name or aliasing policy. |
| Historical artifacts and docs contain valid old terms | Historical docs, old workflows, and evidence should not be bulk rewritten. Only current product surfaces should change after runtime compatibility lands. |

## Unblock Sequence

1. Record the implementation decision: choose the exact schema version and
   compatibility posture. A conservative shape is: keep v1/v1.1 accepted for
   existing workflows, add a new workflow schema version for new terminology,
   and make `workflow upgrade` rewrite old workflow files on demand.
2. Add failing tests first in Python and Go:
   `test_workflow_field_errors.py`, `test_workflow_upgrade.py`,
   `test_workflow_generator.py`, `go/pkg/workflowauthoring/workflow_test.go`,
   and `go/pkg/workflowgenerate/generate_test.go`.
3. Add a daemon PostgreSQL migration that updates CHECK constraints and either
   migrates open live rows to the new values or explicitly supports dual-read
   old/new rows for one compatibility window.
4. Update production Go mutations and read handlers, then mirror required
   Python handler/test compatibility. Writes should use the new values after
   migration; reads should be deliberate about old-row aliases.
5. Update workflow authoring and generation: schema constants, validation
   rules, generator shape/block vocabulary, template catalog entries, docs
   catalog rendering, and `workflow upgrade` dry-run/apply output.
6. Update UI/dashboard/status naming and tests. If old response keys remain as
   compatibility aliases, document them as aliases instead of canonical terms.
7. Sweep current docs and skill templates only after the migration and tests
   land. Leave historical/reference fixtures unchanged unless they are active
   examples that validate current behavior.
8. Only then update shared status docs (`TODO.md`, `ROADMAP.md`,
   `operator/BRIEF.md`) to mark Phase B implemented or partially implemented.

## Non-Changes In This Pass

- No daemon database migration was added.
- No runtime state was changed.
- No shared TODO, roadmap, or brief status was edited.
- No compatibility alias was chosen by code. This artifact classifies the work
  needed to choose and implement it.
