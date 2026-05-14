---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: implementer-unknown-model-001

# RFC 0050 V2 Implementation Handoff

Status: complete
Date: 2026-05-14

## 1. Recovery Panel Island

Implemented. The run recovery panel now mounts a React island that previews
`striatum recovery auto-publish --dry-run` through `/v1/invoke`, renders
idle/loading/result/error states, and keeps recovery commands copy-first.

- `src/striatum/web/frontend/src/islands/recovery-panel/`
- `src/striatum/web/frontend/vite.config.ts`
- `src/striatum/web/templates/_recovery_panel.html`
- `src/striatum/web/templates/run_detail.html`
- `src/striatum/service.py`
- `src/striatum/web/static/build/island-recovery-panel.js`

## 2. Override Verdict Modal

Implemented. Job detail pages now load `override_verdict.js`; the script
builds an ARIA dialog, traps focus, closes on Escape, requires a non-empty
rationale, and posts only the allowed override fields through `/v1/invoke`
as the existing `override-verdict` argv path.

- `src/striatum/web/static/override_verdict.js`
- `src/striatum/web/templates/job_detail.html`

## 3. Copy On Click

Implemented. `copy_on_click.js` initializes `[data-copy]` targets, copies
on click or Enter, and shows a short `copied` toast. `base.js` wires the
initializer globally on `DOMContentLoaded`, and recovery recipes now expose
`data-copy`.

- `src/striatum/web/static/copy_on_click.js`
- `src/striatum/web/static/base.js`
- `src/striatum/web/templates/_recovery_panel.html`

## 4. Workflow Graph Editor Data Binding

Implemented. The graph editor now preserves, renders, edits, and saves the
per-review-job `require_attested_lane` field. The field appears in node body
labels and the textual summary. No viewport overlay or React Flow v12-only API
was added.

- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`
- `src/striatum/web/static/build/island-workflow-graph-editor.js`

## 5. Base JS And CSS

Implemented. `base.js` loads the copy-on-click helper as a progressive
enhancement. `base.css` adds compact styles for recovery-panel layout, modal
forms, copy cues, and toast/status affordances.

- `src/striatum/web/static/base.js`
- `src/striatum/web/static/base.css`

## 6. Tests

Added and updated focused coverage for the new V2 surfaces.

- `tests/test_recovery_panel_dry_run.py`
- `tests/test_override_modal_payload.py`
- `tests/test_copy_on_click.py`
- `tests/test_run_detail_recovery_panel.py`
- `tests/test_web_ui.py`
- `src/striatum/web/frontend/src/__tests__/recovery-panel.test.tsx`
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`

## Verification

- `PYTHONPATH=src pytest -q tests/test_copy_on_click.py tests/test_override_modal_payload.py tests/test_recovery_panel_dry_run.py tests/test_run_detail_recovery_panel.py`
- `PYTHONPATH=src pytest -q tests/test_web_ui.py::test_static_assets_served_when_web_enabled tests/test_web_ui.py::test_assets_resolvable_via_importlib_resources tests/test_copy_on_click.py tests/test_override_modal_payload.py tests/test_recovery_panel_dry_run.py tests/test_run_detail_recovery_panel.py`
- `npm test -- --run src/__tests__/mount.test.ts src/__tests__/recovery-panel.test.tsx src/__tests__/workflow-graph-editor.test.ts`
- `npm run build`
- `make ui-bundle-hash`
- `ruff check src/striatum/service.py tests/test_web_ui.py tests/test_copy_on_click.py tests/test_override_modal_payload.py tests/test_recovery_panel_dry_run.py tests/test_run_detail_recovery_panel.py`
- `python3 -m py_compile src/striatum/service.py tests/test_copy_on_click.py tests/test_override_modal_payload.py tests/test_recovery_panel_dry_run.py tests/test_run_detail_recovery_panel.py`
- `node --check src/striatum/web/static/copy_on_click.js && node --check src/striatum/web/static/override_verdict.js`
- `git diff --check`

Notes: Vitest prints the existing React `act(...)` environment warning for
the recovery-panel component tests, but the tests pass. `npm run build` prints
the existing large-chunk warning.
