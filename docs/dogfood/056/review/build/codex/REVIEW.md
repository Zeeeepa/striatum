---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "ui_rework", "workflow_graph_editor"]
---

author: reviewer-unknown-model-001

# RFC 0050 V2 Build Review

Status: complete
Date: 2026-05-14
Verdict: accept_with_findings

## Trust Boundaries And Attack Surfaces

- Browser JavaScript to `/v1/invoke`: operator actions cross from local UI into runner CLI invocation. The override modal and recovery dry-run are the sensitive paths.
- DOM copy affordances: rendered command and identifier text crosses into the operator clipboard.
- Workflow editor serialization: untrusted browser state is saved back into durable workflow JSON.
- Provenance display: UI labels can overclaim attestation, recovery, or override semantics if they render fields outside their supported scope.

## Required Checks

- Override modal payload is constrained to the allowed form fields. `collectPayload` reads only `verdict`, `rationale`, `findings_artifact_id`, and `auto_fresh_session`, then `buildArgv` appends only the corresponding `override-verdict` argv fields plus server-rendered `sessionId` and `jobId` context. The POST body is only `{ argv }`. See `src/striatum/web/static/override_verdict.js:18`, `src/striatum/web/static/override_verdict.js:34`, and `src/striatum/web/static/override_verdict.js:53`.
- Recovery-panel dry-run uses the dry-run argv and does not expose a publish call from the island. `autoPublishArgv` always includes `--dry-run`, `invokeDryRun` posts that argv to `/v1/invoke`, and the server-rendered auto-publish recipe also includes `--dry-run`. See `src/striatum/web/frontend/src/islands/recovery-panel/RecoveryPanel.tsx:65`, `src/striatum/web/frontend/src/islands/recovery-panel/RecoveryPanel.tsx:104`, and `src/striatum/service.py:564`.
- Copy-on-click copies only the `data-copy` attribute from the activated element. The delegated click/key handlers resolve the nearest `[data-copy]` element and `activate` reads `target.getAttribute("data-copy")`; it does not copy selected text, child text, URL fragments, or arbitrary sibling content. See `src/striatum/web/static/copy_on_click.js:36`, `src/striatum/web/static/copy_on_click.js:84`, and `src/striatum/web/static/copy_on_click.js:99`.
- `workflow-graph-editor::require_attested_lane` has no viewport overlay implementation. The graph node label is data binding only, and `PhaseBands` still returns `null` with the React Flow v12 overlay deferred. See `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:160` and `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:375`.
- V1 and V1.5 provenance invariants are preserved in the reviewed V2 paths: override rationale remains rendered beside override rows in job detail, artifact attestation is derived from recorded artifact data and override rationale, lane evidence remains explicit `override` or muted rather than falsely green, and no reviewed code adds transcript capture. See `src/striatum/web/templates/job_detail.html:49`, `src/striatum/service.py:753`, and `src/striatum/service.py:343`.

## Findings

### F1. `require_attested_lane` can persist on non-review jobs after type changes

Severity: medium

The V2 synthesis says the workflow graph editor should expose `require_attested_lane` for review jobs and reject it for non-review jobs. The implementation only hides the checkbox unless `job.type === "review"`, but it does not remove the field when a user changes an existing review job to a non-review type. `handleJobChange` merges patches into the existing job object, `syncWorkflowJobs` persists the resulting jobs unchanged, and `handleSave` submits that workflow body. As a result, a review job with `require_attested_lane: true` can be changed to `build` and still serialize the review-only field.

Evidence:

- The field is rendered in node labels whenever `job.require_attested_lane === true`, independent of job type: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:160`.
- The inspector control is merely conditional on `job.type === "review"`; it is not a validation or cleanup boundary: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:645`.
- Job edits merge patches over existing objects, so changing `type` does not clear unrelated fields: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:1091`.
- Save writes the workflow with the current jobs list unchanged: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:280` and `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:1154`.

Threat impact: this is not a direct privilege escalation, but it weakens provenance honesty. The editor can produce workflow JSON that visually suggests an attested-lane requirement on a non-review job even though the feature is scoped to review jobs. That creates a misleading operator signal and leaves downstream validation/runtime behavior ambiguous.

Recommended fix: when `type` changes away from `review`, clear `require_attested_lane`; also normalize or reject non-review jobs carrying the field before save. Add a regression test that starts with `{type: "review", require_attested_lane: true}`, changes the type to `build`, saves, and asserts the field is absent.

## Residual Risk

I did not find a path where the override modal posts extra operator-controlled fields, where the recovery panel omits `--dry-run`, where copy-on-click reads content outside `data-copy`, or where viewport overlay code was introduced. The remaining issue is the graph-editor field-scope leak above.
