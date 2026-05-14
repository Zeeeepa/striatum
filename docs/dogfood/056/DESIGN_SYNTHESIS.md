---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/design/UI_REWORK.md", "docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md", "docs/dogfood/054/build/HANDOFF.md", "docs/dogfood/054b/build/HANDOFF.md", "docs/dogfood/055/build/HANDOFF.md", "docs/dogfood/055b/build/HANDOFF.md"]
---

author: designer-unknown-model-001

# RFC 0050 V2 Design Synthesis

Status: ready for implementation
Date: 2026-05-14

## Scope Boundary

V2 is the interactive layer on top of the shipped V1 and V1.5 surfaces. Dogfood 054 shipped the shared chip vocabulary, byline/provenance primitives, dashboard parity, and semantic CSS tokens. Dogfood 054b closed the byline, override, attestation-drift, and dashboard-rationale honesty gaps. Dogfood 055 shipped the server-rendered run recovery panel, job expected-artifacts table, process evidence, artifact provenance trail, posture verdict provenance, doctor recipes, and view-file breadcrumb. Dogfood 055b tightened artifact attestation and override evidence handling.

This synthesis therefore excludes V1.5 template extension work. It also excludes target-repository template catalog extensions and React Flow viewport overlays. RFC 0050 names Reactflow v12 features as a non-goal, including "`ViewportPortal` viewport-locked overlays"; the graph-editor work here is data binding only.

## Decisions Per Deliverable

1. `src/striatum/web/frontend/src/islands/recovery-panel/` is the only new React island in V2. It enhances the already-rendered recovery panel and must preserve the no-JS fallback. UI_REWORK.md §8.3 is explicit: "The island is optional -- the page renders correctly without JS". The island should call `recovery auto-publish --run-id <r> --dry-run` through the loopback invoke path only when the operator asks for a preview, then render the would-publish rows and gate reasons. It must not publish artifacts.

2. Recovery recipes remain copy-first. UI_REWORK.md §5.7 says, "Copy-on-click recipes are always available." The island should improve copying and previewing, not turn every recovery path into a mutation button. The exception is the dry-run preview, which is read-shaped by intent.

3. `src/striatum/web/static/override_verdict.js` owns the override modal behavior on `job_detail.html`. UI_REWORK.md §8.6 requires a script that "posts to `/v1/invoke` with the literal argv" for `override-verdict`. The modal should collect only `verdict`, `rationale`, optional `findings_artifact_id`, and `auto_fresh_session`; session/job identifiers come from server-rendered data attributes, not user-editable fields. It needs `<dialog>` behavior, Escape/close handling, initial focus, focus return, and a focus trap.

4. `src/striatum/web/static/copy_on_click.js` is the shared identifier affordance. UI_REWORK.md §7.7 says: "Every identifier rendered in `<code>` carries a click handler that copies the text to the clipboard". The matcher is also specified there: `^(run|job|sess|art|proc|super|lease)_[0-9a-f]+$`. The implementation should add hover/focus cues and a short confirmation without requiring per-template wiring.

5. `base.js` should initialize the copy affordance globally, preserving existing UTC/local time behavior. The file-map summary in UI_REWORK.md names `base.js` as "extend: copy-on-click for identifiers; existing UTC/Local toggle stays". Keep the initializer idempotent so pages with island hydration and ordinary Jinja2 pages behave the same.

6. `workflow-graph-editor` gets a per-node `require_attested_lane` field only. UI_REWORK.md §8.8 says the inspector should expose "lane attestation requirements" as `require_attested_lane: bool` for review jobs and reject it for non-review jobs. V2 should store the field in the graph editor state, render it in the node body, include it in serialized workflow JSON where the editor already serializes node fields, and add tests for load/edit/save. Do not implement the viewport-positioned attestation overlay; that waits for React Flow v12 per GH #6.

7. `static/base.css` may gain small modal, toast, and copy-token styles if existing tokens are insufficient. Keep the visual system constraints from UI_REWORK.md §7: semantic tokens, visible focus, compact table/panel density, and no purple-dominant theme beyond the override semantic marker.

## Implementation Order

Start with `copy_on_click.js` plus `base.js` wiring, because the recovery panel and doctor/job recipes all rely on the same behavior. Add a focused DOM test around identifier matching, hover/focus classing, clipboard success, and non-matching `<code>` nodes.

Next implement `override_verdict.js`. It is self-contained, uses the existing `/v1/invoke` route, and has high provenance risk, so land keyboard and parameter-filter tests with the script. Assert it never sends extra form fields.

Then add the `recovery-panel` island. Reuse the V1.5 server payload instead of inventing a second recovery model. The island should enhance pre-rendered recipes, run the dry-run preview, and render empty/error/loading states without changing the server-rendered panel contract.

Finish with the workflow graph editor extension. Its blast radius is in the TypeScript editor state and serializer, and it should be kept deliberately boring: field control, node-body render, and round-trip persistence only.

## Acceptance

Use UI_REWORK.md §9 rows that apply to V2 interactive behavior:

- §9.1 browser smoke: keep `run_detail.html`, `job_detail.html`, and `workflow_edit.html` rendering against fixtures, now asserting the recovery island mount, override modal script hooks, and graph editor node field rendering.
- §9.2 responsive screenshots: `/run/<id>` with populated recovery panel and `/run/<id>/job/<wfjob>` must remain stable at 1440, 1024, 768, and 375 widths after the new scripts and modal CSS.
- §9.4 override-verdict rendering: retain the rule that the override rationale is visible and the original verdict row remains above the override.
- §9.6 keyboard/accessibility: apply it directly to the override modal and copy affordance; tab order, focus rings, labels, Escape close, and focus restoration are required.
- §9.7 unsupported-claim regressions: the recovery panel must not promise nonexistent recovery paths, and the graph editor must not imply viewport attestation overlays exist.
- §9.8 bundle refusal: run the UI build and preserve committed bundle-hash discipline.
- §9.10 browser/CLI parity for V1.41 next actions: the dry-run auto-publish recipe shown in the browser must match the CLI next-action recipe exactly.
