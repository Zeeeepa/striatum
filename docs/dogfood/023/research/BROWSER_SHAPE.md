# Research: workflow browser touchpoints

author: researcher-claude-opus-001
date: 2026-05-09

## Existing surfaces

- `striatum.workflow.workflow_graph_data(workflow) -> {workflow_id, graph: {nodes, edges, cycles}}` — generates the graph topology used by the SVG renderer.
- `striatum.workflow.validate_workflow(workflow, ...) -> None | raise WorkflowError` — V1 calls this in a try/except per discovered workflow.
- `striatum.web.graph_svg.render_run_graph(workflow, node_states, *, run_id) -> str` — SVG renderer. Pass `node_states={}` and `run_id=None` for a static-no-state graph (file view, not a run).
- `service.py:_dispatch_get` — V1 inserts `/workflows` and `/workflows/<path>` branches before `/static/*`.
- `chat_tools.py:TOOL_NAMES` — closed set of six. V1 adds `list_workflows` as the seventh.

## Discovery walk

```python
def discover(repo: Path) -> list[dict]:
    skip = {".git", ".striatum", ".venv", "__pycache__", "node_modules", "build", "dist"}
    found = []
    for path in repo.rglob("workflow.json"):
        # Skip paths inside any of the skip dirs.
        rel_parts = path.relative_to(repo).parts
        if any(part in skip for part in rel_parts):
            continue
        try:
            raw = path.read_text(encoding="utf-8")
            data = json.loads(raw)
        except (OSError, json.JSONDecodeError) as exc:
            found.append({"path": str(path.relative_to(repo)),
                          "status": "parse_error",
                          "message": str(exc)[:200]})
            continue
        try:
            validate_workflow(data)
            status = "valid"
            message = None
        except WorkflowError as exc:
            status = "workflow_error"
            message = str(exc)[:200]
        found.append({
            "path": str(path.relative_to(repo)),
            "workflow_id": data.get("workflow_id"),
            "workflow_version": data.get("workflow_version"),
            "status": status,
            "message": message,
            "job_count": len(data.get("jobs") or []),
            "lane_count": len(data.get("lanes") or {}),
            "role_count": len(data.get("roles") or {}),
            "data": data,  # for the detail page; index page omits
        })
    found.sort(key=lambda e: e["path"])
    return found
```

The index page caller drops `data` to keep the response small.

## Route shape

- `GET /workflows` — list page; renders `workflows_index.html`.
- `GET /workflows/<repo-relative-path>` — detail page; renders `workflow_detail.html`.

Path safety: same as `/view/<path>` — `..`, leading `/`, null bytes, symlink-escape, hidden-dir refusal.

## Templates

- `workflows_index.html` — table with columns: path, workflow_id, status pill, jobs/lanes/roles, SVG thumbnail (small inline).
- `workflow_detail.html` — header + status pill + full SVG + tabular jobs/lanes/roles/edges/cycles + validation error block (when invalid).

## Chat tool

```python
def _tool_list_workflows(repo: Path) -> str:
    from striatum.web.workflows import discover
    items = discover(repo)
    if not items:
        return "[no workflow.json files found]"
    lines = []
    for item in items[:100]:
        lines.append(f"{item['status']:<14} {item['path']:<60} jobs={item['job_count']:<3} {item.get('workflow_id') or ''}")
    if len(items) > 100:
        lines.append(f"[truncated at 100; {len(items)} total]")
    return "\n".join(lines)
```

Schema added to TOOL_SCHEMAS with empty params (no inputs).

## Test precedent

- `tests/test_web_workflows.py` (new): use `tmp_path` + write a `workflow.json` + spawn service with `--web` + assert routes return 200.
- Reuse `tests/test_web_chat.py`'s server-spawn helper.

## V1 scope summary

| Surface | Action |
| --- | --- |
| `src/striatum/web/workflows.py` (new) | `discover(repo) -> list[dict]` |
| `src/striatum/web/templates/workflows_index.html` (new) | List page |
| `src/striatum/web/templates/workflow_detail.html` (new) | Detail page |
| `src/striatum/service.py` | Routes + handlers |
| `src/striatum/web/templates/base.html` | Add Workflows nav |
| `src/striatum/web/chat_tools.py` | Add list_workflows |
| `tests/test_web_workflows.py` (new) | Test routes + chat tool |
