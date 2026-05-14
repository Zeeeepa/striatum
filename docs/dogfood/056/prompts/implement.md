# Implement — RFC 0050 V2

Blocked until `review_design` returns an accepting verdict.

**Canonical inputs:**
- `docs/design/UI_REWORK.md` V2 scope.
- `docs/dogfood/056/DESIGN_SYNTHESIS.md`.
- `docs/dogfood/054b/build/HANDOFF.md` + `docs/dogfood/055/build/HANDOFF.md` —
  V1 / V1.5 you reuse.

**Write scope:** `src/striatum/web/`, `src/striatum/service.py`
(only if a new shaping point is unavoidable), `tests/`,
`docs/dogfood/056/build/`. No writes to `.striatum/`, `go/`,
prior dogfoods.

## V2 deliverables

1. **`frontend/src/islands/recovery-panel/`** new island.
   - Calls `striatum recovery auto-publish --dry-run` via loopback
     `/v1/invoke` and shows the would-publish list.
   - Copy-on-click for each CLI recipe (use the V2 copy_on_click
     module).
   - States: idle, loading, results, error.
2. **`static/override_verdict.js`** — modal.
   - Focus trap, ARIA dialog roles, escape closes.
   - Payload: only `{verdict, rationale, findings_artifact_id?,
     auto_fresh_session}`. No arbitrary fields.
   - `POST /v1/invoke` to the loopback route (mutation-gated by
     `serve --allow-mutations`).
   - Refuses to send if rationale is empty.
3. **`static/copy_on_click.js`** — identifier affordance.
   - Targets elements with `data-copy="<text>"`.
   - On click / Enter: copy + brief toast "copied".
   - Visible cue on hover and focus (CSS).
4. **`workflow-graph-editor` extension** — per-node
   `require_attested_lane` field.
   - DATA BINDING ONLY: store the field on the node, render it
     in the node body, persist to workflow.json. No
     viewport-locked overlay (GH #6 — reactflow 11.11.4).
5. **`base.js`** — wire copy-on-click globally on page load.
6. **`base.css`** — modal styles, copy-on-click hover cue,
   recovery-panel layout tokens.
7. **Tests** — at minimum:
   - `tests/test_recovery_panel_dry_run.py` (loopback API mock).
   - `tests/test_override_modal_payload.py` (only allowed fields).
   - `tests/test_copy_on_click.py` (UI interaction).
   - Existing graph-editor test for `require_attested_lane`
     field persistence.

## Tools / patterns

- V1 shared components + V1.5 partials are the substrate. Reuse.
- The Go supervisor / process layer is out of scope.
- D028: no transcript capture even in this V2 layer.

## HANDOFF

`docs/dogfood/056/build/HANDOFF.md`. Front matter MUST NOT
include `author:`. Byline on title-block line. One section per
deliverable.
