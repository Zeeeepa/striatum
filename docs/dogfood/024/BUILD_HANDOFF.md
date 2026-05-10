---
title: "RFC 0024 V1.5 build handoff (dogfood-024)"
date: 2026-05-09
---

# Build handoff: RFC 0024 V1.5 (workflow visual builder)

author: implementer-claude-opus-001

## Scope

V1.5 ships the form-driven visual builder per the design synthesis.
V1 (D076) shipped read-only browsing; V1.5 closes the editor deferral.
Drag-and-drop, templates, "run-now", and field-level error
highlighting remain V2 deferrals.

## Files

### New

- `src/striatum/web/templates/workflow_edit.html` — container template;
  embeds the parsed workflow JSON in a `<script id="workflow-data"
  type="application/json">` tag for the JS island to read; renders
  empty section divs (`#section-header`, `#section-roles`,
  `#section-lanes`, `#section-jobs`, `#section-edges`,
  `#section-cycles`); save / cancel buttons + an error banner; pulls
  in `/static/workflow_edit.js` deferred.
- `src/striatum/web/static/workflow_edit.js` — ~400-LoC JS island.
  Reads workflow from script tag, renders forms per section, with
  add/remove buttons that mutate state. Job-row UI conditionally
  surfaces a posture `<select>` for `job_type=review` and a
  `required_review_postures` input for `job_type=build`. `save()`
  POSTs JSON to `/workflows/edit/<path>`; on 200 redirects to
  `/workflows/<path>`; on 422 surfaces the WorkflowError message
  in the error banner. localStorage draft is written on every
  state mutation and offered for recovery on page load if it
  differs from disk.
- `tests/test_web_workflow_edit.py` — 14 tests covering GET (existing
  / invalid / nonexistent / traversal / hidden-dir refusal) and POST
  (valid-writes-file, invalid-422, no-mutations-405, traversal-400,
  wrong-content-type-415, invalid-json-400, creates-intermediate-dirs,
  atomic-unchanged-on-failure) plus the detail-page edit-link.

### Modified

- `src/striatum/service.py` — `_dispatch_get` adds
  `/workflows/edit/<path>` branch; `_dispatch_post` adds the matching
  POST handler. New methods `_render_workflow_edit_page` (loads
  existing or scaffolds empty) and `_handle_workflow_edit_save`
  (mutation gate → path safety → content-type → body cap → JSON
  parse → `validate_workflow` → atomic write).
- `src/striatum/web/templates/workflow_detail.html` — adds an "Edit"
  link below the header.
- `src/striatum/web/static/base.css` — appends ~80 LoC of
  `.workflow-editor *` styling: form rows, add/remove buttons,
  error banner, draft-recovery dialog.

### Docs

- `CHANGELOG.md` — `## 1.15.0 — 2026-05-09` section.
- `pyproject.toml` + `src/striatum/__init__.py` — bumped to `1.15.0`.
- `docs/DECISION_LOG.md` — D077 row.
- `docs/TODO.md` — F24 row.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status
  `accepted (V1+V1.5)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Missing 1 MB body cap + Content-Type validation | acceptance-blocking | **Addressed**: handler enforces `application/json`, returns 415 on mismatch; reads `Content-Length` and aborts with 413 if > 1 MB; `read()` uses the cap as ceiling. |
| F2: New-vs-existing affordance ambiguous | note | **Addressed**: handler passes `is_new` flag; template header reads "New workflow" or "Editing <path>". |
| Last-writer-wins concurrency | accepted (V1.5 single-operator) | **Deferred to V2**: documented in CHANGELOG deferral list; If-Match precondition is the V2 plan. |
| Field-level error highlighting | accepted | **Deferred to V2**: V1.5 surfaces the WorkflowError message in a top-of-form banner; field-level highlighting requires restructuring `validate_workflow` to yield typed errors. |

## Smoke

End-to-end against the live tailnet bridge (`https://proximal.tail0ecc2e.ts.net/`):

```
$ curl https://proximal.tail0ecc2e.ts.net/workflows/edit/docs/dogfood/024/workflow.json
→ 200 with form filled from disk

$ curl https://proximal.tail0ecc2e.ts.net/workflows/edit/docs/dogfood/024/workflow.json -X POST \
    -H "Content-Type: application/json" -d @bad.json
→ 422 {"ok": false, "error": {"code": 422, "message": "WorkflowError: ..."}}

$ curl https://proximal.tail0ecc2e.ts.net/workflows/edit/new-workflow.json
→ 200 with empty scaffold + workflow_id derived from path stem

$ curl https://proximal.tail0ecc2e.ts.net/workflows/edit/../etc/passwd
→ 400 (path safety refuses)

$ curl https://proximal.tail0ecc2e.ts.net/workflows/edit/x.json -X POST \
    -H "Content-Type: text/plain" -d '{}'
→ 415 (content-type refuses)
```

## Test results

- `tests/test_web_workflow_edit.py`: 14 / 14 pass.
- `tests/test_web_workflows.py` (existing): 18 / 18 pass.
- `tests/test_chat_tools.py` (existing): 17 / 17 pass.
- Full suite: **452 / 452 pass** (278.62s).
- `make lint`: clean.
- `make typecheck`: clean (72 source files).

## Out of scope (V2)

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button.
- Field-level error highlighting (requires `validate_workflow` API change).
- `If-Match: <sha256>` precondition for safe concurrent edits.
- AI-assisted scaffolding via chat tool that *writes* workflow.json
  (would require per-tool gating).

## Acceptance summary

| Gate | Verified |
| --- | --- |
| GET existing workflow renders form | `test_get_existing_workflow_renders_form` |
| GET invalid workflow opens editor anyway | `test_get_invalid_workflow_opens_editor` |
| GET nonexistent path scaffolds empty | `test_get_nonexistent_scaffolds_empty` |
| GET traversal refused | `test_get_traversal_refused_400` |
| GET hidden dir refused | `test_get_hidden_refused_404` |
| POST valid writes file atomically | `test_post_valid_writes_file_atomically` |
| POST invalid → 422, file unchanged | `test_post_invalid_422_unchanged` |
| POST without --allow-mutations → 405 | `test_post_no_mutations_405` |
| POST traversal → 400 | `test_post_traversal_400` |
| POST wrong content-type → 415 (F1) | `test_post_wrong_content_type_415` |
| POST invalid JSON → 400 | `test_post_invalid_json_400` |
| POST creates intermediate dirs | `test_post_creates_intermediate_dirs` |
| Atomic write: file unchanged on validation failure | `test_post_atomic_file_unchanged_on_failure` |
| Detail page has edit link | `test_detail_page_has_edit_link` |
| Smoke against live tailnet bridge | All 5 cases pass |
