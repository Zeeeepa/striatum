---
title: "RFC 0024 V1 build handoff (dogfood-023)"
date: 2026-05-09
---

# Build handoff: RFC 0024 V1 (workflow browser, browse-only)

author: implementer-codex-gpt-5.5-001

## Scope

V1 ships read-only browse per the synthesis. Editor is V1.5; deferred.

## Files

### New

- `src/striatum/web/workflows.py` — `discover(repo)` walks `**/workflow.json` (skipping hidden + build dirs), runs `validate_workflow` per file, returns list of dicts with `{path, workflow_id, status, message, job_count, lane_count, role_count, data}`. `load_workflow_at(repo, rel)` is the path-safe per-file loader for the detail page.
- `src/striatum/web/templates/workflows_index.html` — table view; columns path / workflow_id / status / jobs / lanes / roles.
- `src/striatum/web/templates/workflow_detail.html` — full SVG graph + tables for jobs, lanes, roles, edges, cycles.
- `tests/test_web_workflows.py` — 18 cases covering `discover` unit tests, `load_workflow_at` path-safety, route rendering for valid/invalid/missing/traversal cases, nav presence, and the `list_workflows` chat tool.

### Modified

- `src/striatum/service.py` — `_dispatch_get` adds `/workflows` and `/workflows/<path>` branches; new methods `_render_workflows_index_page` and `_render_workflow_detail_page`.
- `src/striatum/web/templates/base.html` — adds Workflows link to the top nav.
- `src/striatum/web/static/base.css` — appends `.workflows-table` + `.data-table` styling.
- `src/striatum/web/chat_tools.py` — `_TOOLS` array adds `list_workflows`; `execute_tool` dispatches to new `_tool_list_workflows`.

### Docs

- `CHANGELOG.md` — `## 1.14.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.14.0`.
- `docs/DECISION_LOG.md` — D076 row.
- `docs/TODO.md` — F23 row.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.

## Smoke

End-to-end against the live service on this repo:

```
$ curl https://proximal.tail0ecc2e.ts.net/workflows
33 workflow.json files discovered (every dogfood + every example)

$ curl https://proximal.tail0ecc2e.ts.net/workflows/docs/dogfood/022/workflow.json
→ 200 with SVG graph + jobs/lanes/roles tables

$ curl https://proximal.tail0ecc2e.ts.net/workflows/../../etc/passwd
→ 404 (path safety refuses)
```

## Test results

- `tests/test_web_workflows.py`: 18 / 18 pass.
- `tests/test_chat_tools.py` (existing): 17 / 17 pass.
- `make lint`: clean.
- `make typecheck`: clean (71 source files).
- Full suite: pending — running while this handoff is drafted.

## Out of scope (V1.5 / V2)

V1.5 (separate dogfood, future):
- `/workflows/edit/<path>` form-driven visual builder.
- Save action with server-side `validate_workflow`.
- Per-job posture + required_review_postures widgets.
- Flash banner + redirect-after-save.

V2:
- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- Full lifecycle "run this workflow now" button.

## Acceptance summary

| Gate | Verified |
| --- | --- |
| Discovery walks repo, skips hidden dirs | `test_discover_skips_hidden_dirs` |
| Invalid JSON / WorkflowError → status reported, no exception | `test_discover_invalid_json_reports_parse_error`, `test_discover_invalid_workflow_reports_workflow_error` |
| Sorted output | `test_discover_sorts_paths` |
| Path safety on detail page | `test_workflow_detail_path_traversal_400`, `test_load_workflow_at_traversal_refused`, `test_load_workflow_at_hidden_refused` |
| Invalid workflow renders inline (no 500) | `test_workflow_detail_renders_invalid_no_500` |
| Missing path → 404 | `test_workflow_detail_missing_404` |
| SVG graph in detail | `test_workflow_detail_renders_valid` (asserts `<svg`) |
| Workflows nav link | `test_workflows_nav_link_present` |
| Empty-state on index | `test_workflows_index_empty_state` |
| `list_workflows` chat tool | `test_list_workflows_chat_tool`, `test_list_workflows_chat_tool_empty` |
| Smoke against live tailnet bridge | 33 workflows discovered; detail + traversal verified |
