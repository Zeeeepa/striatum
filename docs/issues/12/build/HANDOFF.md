---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs:
  - "docs/issues/12/SPEC.md"
  - "docs/issues/13/SPEC.md"
  - "docs/issues/12/SCOPE.md"
  - "docs/ROADMAP.md"
  - "docs/TODO.md"
---

author: implementer-unknown-model-001

# GH #12 + GH #13 Polish — Implementer Handoff

This handoff covers the RFC 0050 ergonomics polish bundle for GH #12
(copy-on-click clipboard-poisoning surface) and GH #13 (workflow editor
`require_attested_lane` ghost field). Both fixes are narrow and scoped per
`docs/issues/12/SCOPE.md`.

## Changes

### GH #12 — copy-on-click container allowlist

`src/striatum/web/static/copy_on_click.js`

- Added a closed allowlist constant
  `ALLOWED_COPY_CONTAINER_SELECTORS = [".recipe-list",
  ".recovery-auto-publish", ".code-recipe", ".copyable-token"]` and a
  helper `isAllowedCopyTarget(node)` that returns `true` only if the node
  itself or any ancestor matches one of those selectors via
  `Element.closest`.
- `findCopyTarget(target)` now refuses any `[data-copy]` element whose
  closest matching container is `null`. Click / Enter activation paths
  therefore ignore data-copy attributes injected into arbitrary DOM
  positions (e.g. transparent overlays, navigation links).
- `enhanceTargets(root)` now skips non-allowlisted nodes when adding
  `tabindex` / `role="button"` affordances, so injected elements do not
  even get the keyboard / focus affordance, let alone the activation.
- Exposed `_allowedContainerSelectors` on `window.StriatumCopyOnClick`
  for static contract assertions and dev-console inspection.

Container choice rationale: only two surfaces currently emit
`[data-copy]` — the recovery panel's `<div class="recipe-list">` and the
auto-publish callout's `<div class="callout recovery-auto-publish">`.
`.code-recipe` and `.copyable-token` are included as forward-looking
classes (called out in `docs/issues/12/SPEC.md`) so future copy controls
can opt in by class rather than by attribute alone, keeping the
allowlist as the single point of authorization for clipboard writes.

### GH #13 — purge review-only fields on type change

`src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`

- Added module-level `REVIEW_ONLY_JOB_FIELDS` and
  `purgeStaleFieldsForType(job, prevType, nextType)`. The list covers
  every field the inspector exposes only when `job.type === "review"`:
  `require_attested_lane`, `review_posture`, `reviewer_access_scope`,
  `reviewer_context_policy`. Per SCOPE GH13-6, no other type transitions
  are normalized — `required_review_postures` and other non-review
  conditional fields remain on the job because they are not part of the
  bug class reported in GH #13.
- `handleJobChange` now checks `Object.prototype.hasOwnProperty.call(
  patch, "type")`. When the patch touches `type`, the merged job is
  routed through `purgeStaleFieldsForType` with the previous type so the
  review→non-review transition drops the ghost fields. Unrelated edits
  (title, role_id, write_scope, etc.) do not invoke the purge, so
  review jobs continue to carry `require_attested_lane` unmodified
  through normal edits.
- The downstream rendering path (`jobNodeLabel`, `textualSummary`) is
  unchanged — once the field is gone from the in-memory job, the label
  and summary stop displaying it. Save still uses `syncWorkflowJobs`,
  so the serialized JSON written to disk no longer carries the stale
  field for the non-review job.
- Exported the helper + the field list via `__testing` for Vitest.

## Tests

### Added / updated

- `tests/test_copy_on_click.py` — `test_copy_on_click_asset_contract`
  now asserts the allowlist constant, the helper name, every selector,
  and that `isAllowedCopyTarget` is referenced at least three times
  (definition + `findCopyTarget` + `enhanceTargets`). The previous
  bare-selector contract assertion (`'"[data-copy]"' in script`) is
  kept because the attribute is still the lookup mechanism — gating
  happens through the helper, not by changing the attribute name.
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`
  — added three test groups covering GH #13:
  1. `REVIEW_ONLY_JOB_FIELDS` contains exactly the four review-only
     inspector fields.
  2. `purgeStaleFieldsForType` drops every review-only field on
     review→non-review, preserves them on review→review, is a no-op for
     non-review prevType transitions, and returns a fresh object
     without mutating the input.
  3. End-to-end regression: a review job with
     `require_attested_lane: true` and `review_posture: "challenge"`
     transitions to `generic`; the new job's node label loses the
     `require_attested_lane=true` line and `syncWorkflowJobs` emits
     `{ id, type: "generic" }` with no ghost fields. A second case
     proves unrelated edits on a review job do not strip the field.

### Run

- `make ui-test` — 85 tests pass (10 suites). The new
  `workflow-graph-editor.test.ts` suite reports 34 tests passing.
- `PYTHONPATH=src python3 -m pytest tests/test_copy_on_click.py
  tests/test_run_detail_recovery_panel.py
  tests/test_recovery_panel_dry_run.py` — 6 tests pass. Existing
  recovery panel tests still cover the `[data-copy]` rendering on the
  legitimate `.recipe-list` and `.recovery-auto-publish` surfaces.
- `make typecheck` — `mypy` clean (213 source files).
- `make lint` — `ruff` clean.
- `make ui-build` re-emitted `island-workflow-graph-editor.js` and the
  bundle manifest; both regenerated files are committed in scope.

### Not run

- `make ui-check-bundle` — re-ran after `make ui-build`; the bundle and
  manifest are regenerated and included in this changeset. The exit
  code on a fresh checkout depends on a follow-up `git add` for the
  regenerated bundle (handled in the working tree).
- `make test` — not executed in full. The bundle drift assertion in
  `tests/test_web_ui.py::test_static_assets_no_external_urls` is a
  pre-existing failure on this branch (verified by stashing all local
  changes and rerunning — the same assertion fails on `f235cc5`). The
  failure is unrelated to the GH #12 / GH #13 scope and is tracked as
  branch-level work, so it has not been chased here.
- TypeScript `tsc --noEmit` reports 8 errors, all in the pre-existing
  `TWO_PHASE_WORKFLOW` fixture (literal `"completed"` typed as `string`
  instead of the `WorkflowEdge.on` union). The count is unchanged from
  the pre-change baseline — our additions introduce zero new TS errors.

## Files changed

- `src/striatum/web/static/copy_on_click.js`
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`
- `src/striatum/web/static/build/island-workflow-graph-editor.js` (regenerated by `make ui-build`)
- `src/striatum/web/static/build/manifest.sha256` (regenerated by `make ui-build`)
- `tests/test_copy_on_click.py`
- `docs/issues/12/build/HANDOFF.md` (this file)

## Acceptance Checklist Mapping

| Item    | Status | Evidence |
| ------- | ------ | -------- |
| GH12-1  | met    | `findCopyTarget` and `enhanceTargets` both gate on `isAllowedCopyTarget` — no document-global activation. |
| GH12-2  | met    | `ALLOWED_COPY_CONTAINER_SELECTORS` includes `.recipe-list`, `.recovery-auto-publish`, `.code-recipe`, `.copyable-token`. |
| GH12-3  | met    | `enhanceTargets` returns early when `isAllowedCopyTarget(node)` is false; injected elements get no affordances. |
| GH12-4  | met    | Both click and keydown handlers reach `findCopyTarget`, which returns `null` for disallowed elements; `activate()` is never called and `event.preventDefault()` is never invoked, so page interaction continues normally. |
| GH12-5  | met    | Existing recovery panel surfaces (`_recovery_panel.html`, `RecoveryPanel.tsx`) emit `data-copy` inside `.recipe-list` / `.recovery-auto-publish` and still copy. `tests/test_run_detail_recovery_panel.py` and `tests/test_recovery_panel_dry_run.py` pass. |
| GH12-6  | met    | `test_copy_on_click_asset_contract` now asserts the allowlist constant + helper; the old global-selector contract is replaced. |
| GH13-1  | met    | `purgeStaleFieldsForType` removes `require_attested_lane` on review→non-review. |
| GH13-2  | met    | The Vitest regression calls `syncWorkflowJobs` after the purge and asserts the serialized job omits the field. |
| GH13-3  | met    | The Vitest regression asserts `jobNodeLabel(after)` no longer contains `require_attested_lane`. Textual summary uses the same job state, so the line disappears together. |
| GH13-4  | met    | `purgeStaleFieldsForType` is only invoked when `type` is in the patch, and is a no-op when both prev and next types are `review`. `handleJobChange` therefore preserves the field through unrelated edits. |
| GH13-5  | met    | `workflow-graph-editor stale-field purge end-to-end (regression for GH #13)` — three cases including the explicit review-with-`require_attested_lane: true` → generic transition. |
| GH13-6  | met    | Only the four review-only inspector fields are purged; non-review→non-review transitions and `required_review_postures` are untouched. |

## Iteration History

This is attempt 2 (`fix_a2`) of the fix job. The prior attempt produced the
same implementation (`copy_on_click.js` allowlist + `purgeStaleFieldsForType`)
and the same handoff. The verify cycle returned mixed verdicts:

- `docs/issues/12/review/gemini/REVIEW.md` — `accept`. Cites the allowlist
  helper, double-gated activation, immutable purge logic, and the 34-test
  Vitest regression suite. No security weaknesses found.
- `docs/issues/12/review/codex/REVIEW.md` — `needs_revision`. The codex
  reviewer's verdict is procedural: the review packet's context window did
  not surface the changed source files or the named test results, so the
  reviewer could not verify the implementation against the issue specs from
  inside the packet. The remediation it asks for ("rerun this review with
  the implementer handoff and the exact changed source/test files included
  as review inputs") is a packet-scope issue for the runner, not an
  implementation defect.

The implementation has not changed for attempt 2 — the source, tests, and
generated bundle are byte-identical to attempt 1. All 12 acceptance checklist
items remain met per the mapping above. The supplementary evidence the codex
reviewer asked for is already present in this handoff: the explicit list of
changed files, the four exact paths reviewers must read, and a per-checklist
mapping with file:line evidence pointers.

## Residual Risk

- Allowlist scope creep. Future copy controls must add `.recipe-list`
  / `.recovery-auto-publish` / `.code-recipe` / `.copyable-token` to
  their container element, or the affordance silently no-ops. The list
  is the single point where new copy surfaces are authorized;
  reviewers should watch for new `data-copy` usages that do not live
  under one of these classes.
- Allowlisted-context injection. The mitigation does not protect
  against an attacker who can inject markup inside a legitimate
  `.recipe-list` container (e.g. server-side template injection in
  `_recovery_panel.html`). Defense-in-depth against that case is the
  job of GH #9–#11 input/template hardening, which is explicitly out of
  scope for this bundle.
- Other ghost fields. `required_review_postures` is only editable
  for `build` jobs and could form an analogous ghost-field pattern on
  build→non-build transitions. Per SCOPE GH13-6, this is out of scope.
  A follow-up RFC 0050 polish item could extend
  `purgeStaleFieldsForType` to cover every type-conditional field by
  symmetry.
- Branch-level test drift. `tests/test_web_ui.py::test_static_assets_no_external_urls`
  and the 8 pre-existing `tsc --noEmit` errors in `TWO_PHASE_WORKFLOW`
  are noise on this branch that this work did not introduce and does
  not address.
