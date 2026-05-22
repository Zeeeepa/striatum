---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0074-workflow-shape-catalog-phase-a"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md"
state: "open"
opened_at: "2026-05-22"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "high"
---

# RFC 0074 Workflow Shape Catalog Phase A Plan
author: coordinator-codex-gpt-5-001

## Outcome

Run a bounded RFC 0074 Phase A catalog pass: metadata-first catalog entries,
role/adversary pack discovery, one implementation-panel example validation,
read-only discovery review, and closure. The workflow must keep generator
shape implementation, role/adversary generation flags, cost estimation, RFC
0052 committee artifacts, and web chooser pack selection deferred to Phase B or
later.

## Inputs

- [`RFC 0074`](../../rfcs/0074-workflow-shape-and-adversary-pack-catalog.md)
- [`RFC 0034`](../../rfcs/0034-workflow-generator-and-template-catalog.md)
- [`RFC 0076`](../../rfcs/0076-three-lane-code-and-doc-audit-workflow.md)
- [`Phase 4 synthesis`](../artifacts/active-runway-1-5/phase4/SYNTHESIS.md)
- [`RFC 0076 catalog follow-up`](../artifacts/rfc-0076-audit-remediation/catalog-followup/PLAN.md)
- [`docs/operator/workflows/rfc-0074-phase-a-catalog/workflow.json`](../workflows/rfc-0074-phase-a-catalog/workflow.json)

## Workstreams

| Workstream | State |
|---|---|
| Discover role/adversary pack names, overlaps, and RFC 0076 fit | scaffolded |
| Add metadata-first catalog entries and read-only discovery surfaces | scaffolded |
| Validate one hand-authored implementation-panel example | scaffolded |
| Review discovery surfaces for Phase A/Phase B boundary leaks | scaffolded |
| Publish closure with validation evidence and remaining deferred work | scaffolded |

## Workflow Scaffold

Validate the workflow:

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0074-phase-a-catalog/workflow.json
```

Prepare and start it through the daemon-backed runner when Phase A
implementation is ready:

```bash
PYTHONPATH=src python3 -m striatum.cli run prepare --workflow docs/operator/workflows/rfc-0074-phase-a-catalog/workflow.json --json
PYTHONPATH=src python3 -m striatum.cli run start --run-id <run_id> --json
```

## Guardrails

- Phase A is local package-data/catalog metadata plus one validating example.
- Do not make role packs or adversary packs runtime state, daemon state, model
  identity, or workflow-schema requirements.
- Do not add `workflow generate --shape implementation_panel`, `--role-pack`,
  `--adversary-pack`, or chooser pack-selection behavior in this phase.
- Do not add new artifact kinds or RFC 0052 debate/panel schemas.
- Keep catalog discovery read-only: listing, showing, rendering, and service
  read responses may expose packs, but write/generation flows must not pretend
  to honor pack choices yet.
