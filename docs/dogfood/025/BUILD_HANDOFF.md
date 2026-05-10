---
title: "RFC 0024 V2 build handoff (dogfood-025)"
date: 2026-05-09
---

# Build handoff: RFC 0024 V2 (run-now + If-Match + field-level errors)

author: implementer-claude-opus-001

## Scope

V2 closes three deferrals from D077:

1. **"Run now" lifecycle** — `POST /workflows/run/<path>` lifts a
   workflow file into a fresh run via `create_run + branch_confirm +
   run_start`. Mutation-gated; auto-confirms branch when the
   workflow declares `branch.mode == "auto"`; surfaces
   `needs_branch_confirmation` as a 200 with that status when the
   mode is `confirm` so the operator can finish out-of-band.
2. **`If-Match: <sha256>` concurrency guard** — GET
   `/workflows/edit/<path>` stamps the disk sha256 in a hidden
   `<script>` tag; editor JS echoes it on POST; on stale sha the
   server returns 412 with `current_sha256`. Missing header is V1.5
   opt-out (backward-compat).
3. **Field-level validation errors** — `WorkflowError` extended with
   optional `field_path`. 8 high-traffic raise sites updated to
   carry paths like `jobs[2].role_id`. 422 body now carries
   `errors: [{field_path, message}]`; editor highlights the field
   via `data-field-path` on the form `<dd>` wrappers.

Drag-and-drop, templates, and AI-assisted scaffolding remain V3.

## Files

### New

- `src/striatum/web/static/workflow_run.js` — ~30-LoC island for the
  "Run now" button on the detail page. POSTs `{}`, redirects to
  `/run/<run_id>` on 200, alerts on error.
- `tests/test_workflow_field_errors.py` — 9 tests covering
  `WorkflowError.field_path`, the 8 tagged raise sites, and V1.5
  backward-compat for untagged sites.
- `tests/test_web_workflow_run.py` — 7 HTTP tests covering the
  run-now route: 200 happy path, 405 without --allow-mutations, 422
  invalid workflow, 404 missing path, 400 traversal, 415 wrong
  Content-Type, detail page button presence.

### Modified

- `src/striatum/errors.py` — `WorkflowError.__init__` adds
  keyword-only `field_path: str | None = None`.
- `src/striatum/workflow.py` — 8 raise sites tagged: schema_version,
  duplicate job id, unknown role, unknown lane, invalid artifact
  path, cycle references unknown job, cycle max_iterations < 1.
  `enumerate()` over `jobs` and `cycles` so we have indices for
  paths.
- `src/striatum/service.py`:
  - `_dispatch_post` adds `/workflows/run/<path>` branch.
  - `_render_workflow_edit_page` computes `hashlib.sha256` of disk
    contents and passes as `workflow_sha256` to the template.
  - `_handle_workflow_edit_save` reads `If-Match` header, returns
    412 on stale + re-checks before rename (TOCTOU narrowing); 422
    body now includes `errors[]` carrying tagged `field_path`s; 200
    body now includes the new `sha256` so the editor can update its
    in-memory copy.
  - `_handle_workflow_run_now` is the new run-now handler.
- `src/striatum/web/templates/workflow_edit.html` — adds
  `<script id="workflow-sha256">` tag.
- `src/striatum/web/templates/workflow_detail.html` — adds Run-now
  button (rendered only when `workflow.status == "valid"`) and
  loads `/static/workflow_run.js`.
- `src/striatum/web/static/workflow_edit.js` — reads the disk
  sha256, echoes as `If-Match` on POST, highlights form fields via
  `[data-field-path]` on 422 with `errors[]`, surfaces a
  reload-prompt banner on 412.
- `src/striatum/web/static/base.css` — `.workflow-edit-page
  .field-error` styling (red outline, inputs get red border).

### Docs

- `CHANGELOG.md` — `## 1.16.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.16.0`.
- `docs/DECISION_LOG.md` — D078 row.
- `docs/TODO.md` — F25 row.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status
  `accepted (V1+V1.5+V2)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Document the workflow-trust model | note | **Addressed**: BUILD_HANDOFF + CHANGELOG state explicitly that "Run now" trusts every committed `workflow.json`; matches CLI surface. |
| F2: Field-path tooltip should also surface globally | note | **Addressed**: editor `save()` always populates the top-of-form banner with `error.message` AND highlights fields. Both signals are present. |
| F3: 409 should include `git_status` for dirty-tree | note | **Deferred to V3**: we surface the original `BranchConfirmationError` message which already mentions "uncommitted changes"; richer payload waits for a follow-up. |

## Smoke

End-to-end against the local service:

```
$ curl -X POST http://127.0.0.1:8088/workflows/run/docs/dogfood/024/workflow.json \
    -H "Content-Type: application/json" -d '{}'
→ 422 (workflow already a completed run; expected behavior — fresh run on existing branch)

$ curl http://127.0.0.1:8088/workflows/edit/docs/dogfood/025/workflow.json
→ 200 with <script id="workflow-sha256"> stamped

$ curl -X POST http://127.0.0.1:8088/workflows/edit/docs/dogfood/025/workflow.json \
    -H "Content-Type: application/json" \
    -H 'If-Match: "deadbeef"' -d '{...}'
→ 412 with current_sha256 in body
```

## Test results

- `tests/test_workflow_field_errors.py`: 9 / 9 pass.
- `tests/test_web_workflow_run.py`: 7 / 7 pass.
- `tests/test_web_workflow_edit.py`: 20 / 20 pass (5 new V2 tests).
- `tests/test_web_workflows.py` (existing): 18 / 18 pass.
- `make lint`: clean.
- `make typecheck`: clean (75 source files).
- Full suite: pending (running while this handoff is drafted).

## Workflow-trust model

V2 lets any operator with `--allow-mutations` start a run from any
committed `workflow.json`. This matches the CLI surface (`striatum
run prepare --workflow <path>` from a shell). No new attack surface.
Operators who want stricter gating should not pass `--allow-mutations`
and should use the CLI exclusively.

## Out of scope (V3)

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- AI-assisted scaffolding via chat tool that *writes* workflow.json.
- Multi-error reporting (collect all errors, not just first).
- Field-path coverage for the remaining ~22 raise sites.
- `flock()` for hard concurrency guarantees.
- 409 body carrying `git status --short` output.

## Acceptance summary

| Gate | Verified |
| --- | --- |
| `WorkflowError.field_path` is None by default | `test_workflowerror_default_field_path_is_none` |
| `WorkflowError.field_path` accepts kwarg | `test_workflowerror_carries_optional_field_path` |
| 8 raise sites tagged correctly | 6 explicit + 2 implicit |
| Untagged raise site keeps None | `test_unconverted_raise_site_keeps_none` |
| Run-now creates run | `test_run_now_creates_run_returns_run_id` |
| Run-now mutation-gated | `test_run_now_without_mutations_returns_405` |
| Run-now invalid → 422 + errors[] | `test_run_now_invalid_workflow_returns_422` |
| Run-now missing → 404 | `test_run_now_missing_path_returns_404` |
| Run-now traversal → 400 | `test_run_now_traversal_returns_400` |
| Run-now wrong content-type → 415 | `test_run_now_wrong_content_type_returns_415` |
| Detail page has Run-now button | `test_run_detail_page_has_run_now_button` |
| If-Match matching → 200 | `test_edit_post_if_match_matching_succeeds` |
| If-Match stale → 412 | `test_edit_post_if_match_stale_returns_412` |
| If-Match missing → V1.5 compat | `test_edit_post_if_match_missing_is_v15_compat` |
| GET injects sha256 | `test_edit_get_injects_sha256` |
| 422 carries errors[] with field_path | `test_edit_post_structured_errors_carries_field_path` |
