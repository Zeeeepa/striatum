# Research: web redesign touchpoints

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Existing surfaces

### `service.py` routing

- `do_GET` (line 159) → `_dispatch_get` (line 175). `_dispatch_get`
  matches by path against a fixed set of prefixes:
  - `/v1/health`, `/v1/runs`, `/v1/runs/...`, `/v1/doctor`,
    `/v1/artifacts/.../raw` — JSON API.
  - `/`, `/static/*` — static SPA assets (gated by
    `state.web_enabled`).
- 404 fallback for everything else.
- V1 inserts new branches before the `/static/*` catch-all:
  - `/run/<run_id>` → render `run_detail.html`.
  - `/run/<run_id>/job/<job_id>` → render `job_detail.html`.
  - `/run/<run_id>/artifact/<artifact_id>` → render
    `artifact_view.html`.
  - `/doctor` → render `doctor.html`.
  - `/` → render `run_list.html` (replaces serving
    `index.html`).
  - `/#/...` legacy hash routes resolve naturally because the
    server only sees the path before `#` (browser-side
    redirect via small JS island instead).

### Workflow graph data

`workflow.py:workflow_graph_data(workflow)` returns
`{"workflow_id": ..., "graph": {"nodes": [...], "edges": [...],
"cycles": [...]}}` (line 242). Each node has `job_id`, `type`,
`role_id`, `lane_id`, `parallel_group`, etc. Edges have
`from`, `to`, optional `requires_verdict`. Same shape the SVG
renderer consumes.

### CSS today

`web/static/app.css` (260 LoC). Has `.badge-*` classes per
state + verdict; minimal layout primitives; no CSS custom
properties; no dark mode. V1 rewrites this around custom
properties + dark mode media query.

### Test precedent

`tests/test_web_ui.py` patterns: spawn the service via
`subprocess`, hit an HTTP port, assert response content +
status code. New tests follow the same pattern with checks
against rendered HTML (look for known template strings) and
SVG structure (`<svg>` tag presence).

### Jinja2 PackageLoader pattern

```python
from jinja2 import Environment, PackageLoader, select_autoescape
env = Environment(
    loader=PackageLoader("striatum.web", "templates"),
    autoescape=select_autoescape(["html"]),
)
template = env.get_template("run_detail.html")
html = template.render(run=run, jobs=jobs, ...)
```

`PackageLoader` reads templates from
`src/striatum/web/templates/` shipped via package-data. Same
mechanism as the skill-bundle templates and the scaffold
templates.

## Summary table

| V1 surface | File:line | Action |
| --- | --- | --- |
| Runtime dep | `pyproject.toml:30` | Add `jinja2>=3.1` |
| Templates | `src/striatum/web/templates/*.html` (new) | 6 files: base + 5 pages |
| CSS | `src/striatum/web/static/base.css` (new), `app.css` retired | Custom properties + dark mode + spacing scale |
| SVG renderer | `src/striatum/web/graph_svg.py` (new) | Layered layout + click nav |
| Routing | `src/striatum/service.py:175` | New branches before `/static/*` |
| Tests | `tests/test_web_ui_redesign.py` (new) | Per synthesis |
| Docs | various | RFC status, CHANGELOG, version bump |
