# Design synthesis: RFC 0024 V1 (workflow browser, browse-only)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1 ships read-only browse. Editor is V1.5; deferred from this release.

## 1. Discovery (`striatum.web.workflows.discover`)

Walk `repo.rglob("workflow.json")`; skip any path containing one of:

```
.git .striatum .venv __pycache__ node_modules build dist .mypy_cache .pytest_cache
```

Per match, JSON-parse, run `validate_workflow`, capture status. Return list of dicts sorted by path.

Per-entry shape:

```python
{
    "path": "docs/dogfood/022/workflow.json",  # repo-relative, forward-slash
    "workflow_id": "dogfood-022-...",
    "workflow_version": "2026-05-09",
    "status": "valid" | "parse_error" | "workflow_error",
    "message": "..." | None,
    "job_count": 5,
    "lane_count": 2,
    "role_count": 5,
    "data": {...},  # the parsed JSON; detail page consumes this; index strips it
}
```

Errors during file read produce `status: "parse_error"` with the OS/JSON error truncated to 200 chars; never raises out of `discover`.

## 2. Routes

```
GET /workflows                  → workflows_index.html
GET /workflows/<repo-path>      → workflow_detail.html
```

Path safety: identical to `/view/<path>` — refuse `..`, leading `/`, null bytes, paths escaping the repo, paths starting with hidden dirs.

## 3. Templates

### workflows_index.html

Header: "Workflows" + count.

Table with columns: **Path** (link), **Workflow ID**, **Version**, **Status** (pill: green=valid / yellow=workflow_error / red=parse_error), **Jobs**, **Lanes**, **Roles**.

A small inline SVG thumbnail in a "Graph" column at the right (rendered at 200×150px via CSS scaling of the full SVG output).

Empty-state message when no workflows are found: "No workflow.json files in this repo. See `examples/` for templates."

### workflow_detail.html

Header: workflow_id (h1) + version + status pill.

Sections in order:
1. **Path** — repo-relative path as `<code>`.
2. **Validation** — status pill + (when `workflow_error` or `parse_error`) the message as `<pre><code>`.
3. **Graph** — inline full-size SVG via `render_run_graph(data, node_states={}, run_id=None)`.
4. **Jobs** — table: id, type, role, lane, posture, required_review_postures, expected_artifacts (count).
5. **Lanes** — table: id, adapter, display_model, capabilities, transcripts/repo_scope/network constraints.
6. **Roles** — table: id, definition_path.
7. **Edges** — table: from, to, on, requires_verdict.
8. **Cycles** (if any) — table: from, to, on_verdict, max_iterations.

Path safety as above. If the workflow doesn't exist or the path is outside the repo: 404. If it's invalid JSON or `WorkflowError`: 200 with the error rendered (don't 500).

## 4. Chat tool (RFC 0023 V1.5 closed set extension)

Add `list_workflows` (no inputs):

```python
{
    "name": "list_workflows",
    "description": "List every workflow.json in the operator's repo with validation status, job count, and workflow_id. Capped at 100 entries.",
    "parameters": {"type": "object", "properties": {}, "additionalProperties": False},
}
```

`execute_tool` dispatches to `_tool_list_workflows(repo)` which calls `discover()` and formats:

```
status         path                                                         jobs  workflow_id
valid          docs/dogfood/022/workflow.json                                5     dogfood-022-...
valid          docs/dogfood/021/workflow.json                                9     dogfood-021-...
valid          examples/code-change-flow/workflow.json                       3     code-change-flow
...
```

Truncates at 100 entries with a `[truncated; N total]` marker.

## 5. Nav

`base.html` top-nav adds `<a href="/workflows">Workflows</a>` between "Runs" and "Chat".

## 6. Test plan (`tests/test_web_workflows.py`)

```
1. test_workflows_index_lists_repo_workflows
   - tmp_path repo with two workflow.json files (one valid, one parse_error)
   - GET /workflows → 200; both paths visible; status pills correct
2. test_workflows_index_skips_hidden_dirs
   - workflow.json under .git/ and .striatum/ → not in list
3. test_workflow_detail_renders_valid
   - GET /workflows/<valid-path> → 200; SVG present; tables present
4. test_workflow_detail_renders_invalid
   - GET /workflows/<invalid-path> → 200 with error block; no 500
5. test_workflow_detail_path_traversal_refused
   - GET /workflows/../../etc/passwd → 400
6. test_workflow_detail_dotgit_hidden
   - GET /workflows/.git/HEAD → 404
7. test_workflow_detail_404_for_missing
   - GET /workflows/nope.json → 404
```

Plus `tests/test_chat_tools.py` extension:

```
8. test_list_workflows_tool
   - tmp_path with 2 workflow.json files
   - execute_tool("list_workflows", {}, repo=tmp_path) returns formatted list
9. test_list_workflows_tool_empty
   - empty repo → "[no workflow.json files found]"
```

## 7. Documentation surface

- `docs/SPEC.md` § "Local Web UI" — extend with workflows surface.
- `docs/UBIQUITOUS_LANGUAGE.md` — `workflow file`, `validation status`.
- `docs/DECISION_LOG.md` — D076.
- `docs/TODO.md` — F23.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.14.0 — 2026-05-09`.
- `pyproject.toml` + `__init__.py` — bump to `1.14.0`.

## 8. Out of scope

V1.5 (separate dogfood):
- Edit page form (`/workflows/edit/<path>`).
- Save action with server-side validation.
- Per-job posture + required_review_postures widgets.
- Flash banner + redirect-after-save.

V2:
- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button.

## 9. Zero-regression contract

- Without visiting `/workflows`: behavior unchanged from v1.13.0.
- JSON API (`/v1/*`), SSE, chat surface, run pages all unchanged.
- CSP byte-identical.
- Existing tests continue to pass.
