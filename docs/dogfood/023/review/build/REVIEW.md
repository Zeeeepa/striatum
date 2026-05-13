# Build review (devils_advocate): RFC 0024 V1

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

V1 ships per synthesis. Smoke against the live tailnet bridge confirms 33 workflows discovered including dogfood-023's own (self-evident dogfood).

## Sweep matrix

| Concern | Mitigation | Verified |
| --- | --- | --- |
| Hidden dirs excluded | `_SKIP_DIRS` frozenset checked per path part | `test_discover_skips_hidden_dirs` |
| Path traversal on detail page | `load_workflow_at` refuses; handler refuses before that | `test_workflow_detail_path_traversal_400`, `test_load_workflow_at_traversal_refused` |
| Invalid JSON not 500 | try/except in `discover` + `load_workflow_at` | `test_discover_invalid_json_reports_parse_error` |
| Invalid workflow not 500 | try/except for `WorkflowError` | `test_workflow_detail_renders_invalid_no_500` |
| Missing path 404 | `load_workflow_at` returns None; handler 404s | `test_workflow_detail_missing_404` |
| SVG renders | `render_run_graph` reused; `node_states={}` for static | `test_workflow_detail_renders_valid` (asserts `<svg`) |
| Empty repo state | Empty-state message in template | `test_workflows_index_empty_state` |
| Nav link | `base.html` updated | `test_workflows_nav_link_present` |
| Chat tool | Closed set + format + cap | `test_list_workflows_chat_tool` |
| 33-workflow live discovery | Live smoke against the running service | tailnet curl |

## Counterargument sweep

### "Why not also list workflows under `examples/`?"

`discover` walks the whole tree including `examples/`. Already covered. The smoke output shows `docs/dogfood/*` entries; if `examples/*/workflow.json` existed in this repo they'd show up too. **Accept.**

### "Sorting alphabetically — should newer dogfood appear first?"

V1 sorts by path, which puts dogfood-001-v2 first and dogfood-023 last. Operators looking for a recent run might be surprised. **Acceptable for V1**; V1.5 could add a "sort by mtime descending" toggle.

### "Detail page renders all jobs in a flat table — what about 50-job workflows?"

The largest workflow in this repo (dogfood-021) has 9 jobs. Tables scale fine for that range. If a user authors a 50-job workflow, the table grows but stays readable. V2 could add column sorting / filtering. **Accept.**

### "Validation runs *every time* the index loads — perf concern?"

For 33 workflows × ~10ms validate each = ~330ms response time. Noticeable but not bad. V1.5 could cache by file mtime. **Accept.**

### "list_workflows tool — what if it returns a thousand entries?"

Capped at 100 with truncation marker. Same pattern as `list_dir`. **Accept.**

## Decision

Accept clean. Land v1.14.0.
