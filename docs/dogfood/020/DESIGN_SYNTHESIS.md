# Design synthesis: RFC 0022 V1 (web UI redesign)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1 ships RFC 0022's three steps. Pragmatic V1 scope (no
feature creep beyond what RFC 0022 promises and the user's
named pain points):

- Step 1: Jinja2 SSR + multi-page routing.
- Step 2: refreshed CSS with custom-property palette + dark
  mode + system fonts + 4px spacing scale.
- Step 3: SVG dependency graph, layered top-down layout,
  click-to-navigate.

Out of V1: hash-route 302 redirect (deferred to V1.5; legacy
hash URLs still work because the browser doesn't send `#`
to the server, but the SPA's hash-router code path is removed).
Inline Markdown rendering on artifact pages → V1.5.

## 1. Templates

`src/striatum/web/templates/` ships 6 files:

- `base.html` — shell with `<head>` (CSP meta, viewport,
  CSS link), `<body>` with header (brand link, nav: Runs,
  Doctor) and `{% block main %}{% endblock %}`.
- `run_list.html` — list of runs with state, branch, last
  event timestamp.
- `run_detail.html` — left rail: jobs list (clickable,
  state-colored). Center: status panel + verdict counts +
  posture chips (RFC 0018 step 3). Right rail: SVG
  dependency graph + next-actions list. Mutation buttons
  (verdict, decision, checkpoint resolve) gated on
  `STRIATUM_WEB_MUTATIONS=1`.
- `job_detail.html` — job state, lease, current verdict,
  artifacts list, latest event payload.
- `artifact_view.html` — artifact metadata, sha256, raw
  link. (V1.5 will add inline Markdown rendering.)
- `doctor.html` — doctor output, formatted as panels per
  category.

Each template extends `base.html`.

## 2. Routes

`service.py:_dispatch_get` adds (before `/static/*`):

```
GET /                                 → run_list.html
GET /run/<run_id>                     → run_detail.html
GET /run/<run_id>/job/<job_id>        → job_detail.html
GET /run/<run_id>/artifact/<art_id>   → artifact_view.html
GET /doctor                           → doctor.html
```

Each handler:
1. Validates the path matches a known shape (regex / split).
2. Calls the existing JSON producers (`status`, `doctor`,
   etc.) to gather data.
3. Renders the template via `_jinja_env().get_template(name).render(...)`.
4. Sends `200 OK`, `Content-Type: text/html; charset=utf-8`,
   plus the existing CSP header.

The Jinja2 environment is constructed once per service via
a module-level lazy factory:

```python
@lru_cache(maxsize=1)
def _jinja_env() -> Environment:
    return Environment(
        loader=PackageLoader("striatum.web", "templates"),
        autoescape=select_autoescape(["html"]),
        keep_trailing_newline=False,
    )
```

## 3. CSS architecture

`src/striatum/web/static/base.css` (new) defines the palette
via CSS custom properties:

```css
:root {
  --bg-base: #f8f9fb;
  --bg-elevated: #ffffff;
  --bg-overlay: #eef0f4;
  --fg-primary: #0d1117;
  --fg-secondary: #57606a;
  --fg-muted: #8b949e;
  --border: #d0d7de;
  --accent: #0969da;
  --status-running: #d29922;
  --status-completed: #1a7f37;
  --status-failed: #d1242f;
  --status-blocked: #cf222e;
  --status-canceled: #8b949e;
  --status-queued: #6f42c1;
  --status-waiting_human: #bf8700;
  --shadow-sm: 0 1px 2px rgba(13,17,23,0.05);
  --shadow-md: 0 4px 8px rgba(13,17,23,0.08);
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
  --radius: 6px;
  --font-sans: -apple-system, BlinkMacSystemFont,
    "Segoe UI", "Roboto", "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, "Cascadia Code",
    "JetBrains Mono", Consolas, "Liberation Mono", monospace;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg-base: #0d1117;
    --bg-elevated: #161b22;
    --bg-overlay: #21262d;
    --fg-primary: #e6edf3;
    --fg-secondary: #8b949e;
    --fg-muted: #6e7681;
    --border: #30363d;
    --accent: #2f81f7;
    --status-running: #d29922;
    --status-completed: #2ea043;
    --status-failed: #f85149;
    --status-blocked: #ff7b72;
    --status-canceled: #6e7681;
    --status-queued: #a371f7;
    --status-waiting_human: #d29922;
    --shadow-sm: 0 1px 2px rgba(0,0,0,0.3);
    --shadow-md: 0 4px 8px rgba(0,0,0,0.5);
  }
}
```

Component CSS uses these properties exclusively; no hex
literals outside the `:root` blocks. Existing badge classes
become `.status-pill` with `background: var(--status-<state>)`
backgrounds.

`app.css` is removed in this commit; the new base.css replaces
it. The mutation-button JS island stays in `app.js` (now
loaded as a non-module `<script defer>` since the SPA shell is
gone). All inline styling moves to base.css.

## 4. SVG dependency graph

`src/striatum/web/graph_svg.py` (new) exports
`render_run_graph(workflow, node_states) -> str` returning an
SVG `<svg>...</svg>` string.

### Layout algorithm

Layered top-down (Sugiyama-lite):

1. Compute topological depth for each node (longest path from
   any root). Nodes with no incoming edges are at depth 0.
2. Group nodes by depth into layers.
3. For each layer, sort nodes by `job_id` (deterministic).
4. Position node `(layer, index)` at:
   `x = padding + index * (node_width + gap_x) + layer_offset(layer)`,
   `y = padding + layer * (node_height + gap_y)`.
5. For each edge `(from, to)`, draw an L-shaped polyline:
   from `(x1 + node_width/2, y1 + node_height)` down to
   midpoint between layers, then horizontal to
   `x2 + node_width/2`, then down to `(x2 + node_width/2, y2)`.

Constants: `node_width=180`, `node_height=44`, `gap_x=24`,
`gap_y=44`, `padding=24`. The viewBox is computed to fit all
nodes; the SVG `width="100%"` so it scales in container.

### Node rendering

Each node:

```svg
<a href="/run/{run_id}/job/{job_id}" class="graph-node-link">
  <g class="graph-node graph-node-{state}" transform="translate({x} {y})">
    <rect width="180" height="44" rx="6" />
    <text x="12" y="18" class="graph-node-title">{job_id}</text>
    <text x="12" y="34" class="graph-node-meta">{role_id} · {state}</text>
    <title>{job_id}: {state} ({role_id}/{lane_id})</title>
  </g>
</a>
```

CSS classes match status custom properties. The `<title>` is
the SVG accessibility tooltip. State coloring is via `fill:
var(--status-<state>)` on `.graph-node-{state} rect`.

### Edge rendering

```svg
<path class="graph-edge" d="M ..." />
```

`stroke: var(--border)`, `fill: none`, `stroke-width: 1.5`.
Arrow markers via `<marker>` defs at the SVG top.

## 5. Hash-route compatibility

The legacy hash-route SPA used `window.location.hash` for
routing. With the SPA gone, browsers visiting `/#/run/<id>`
will hit `/` (the hash isn't sent to the server), see
`run_list.html`, and the user clicks the run. A small JS
island in `base.html` reads `window.location.hash` on load
and, if it matches `^#/run/...`, does
`window.location.replace("/run/...")`. This is best-effort;
a real 302 server-side redirect for legacy hash URLs isn't
possible since the server never sees the hash.

## 6. Mutation-button preservation

The mutation buttons (RFC 0013 step 7) stay client-side. They
move from `app.js`'s old hash-route handlers into per-page JS
islands. Each page's template includes a `<script defer
src="/static/mutations.js">` only when
`STRIATUM_WEB_MUTATIONS=1`. The script attaches click handlers
to button elements rendered with `data-mutation` attributes.
Same gating, same payload shape, same CSP-friendly approach.

## 7. Test plan (`tests/test_web_ui_redesign.py`)

```
1.  test_run_list_page_renders_html
2.  test_run_detail_page_renders_html
3.  test_run_detail_includes_svg_graph
4.  test_job_detail_page_renders_html
5.  test_artifact_view_page_renders_html
6.  test_doctor_page_renders_html
7.  test_csp_header_unchanged
8.  test_dark_mode_palette_present_in_base_css
9.  test_svg_graph_layered_layout_for_three_node_workflow
10. test_svg_node_link_navigates_to_job_detail_url
11. test_legacy_hash_route_javascript_island_present
12. test_jinja2_environment_constructs
13. test_mutation_buttons_respect_gate
```

## 8. Documentation surface

V1 also updates:

- `docs/SPEC.md` § "Web UI" subsection rewritten.
- `docs/UBIQUITOUS_LANGUAGE.md` adds `page route`, `theme palette`.
- `docs/DECISION_LOG.md` adds D073.
- `docs/TODO.md` adds F20.
- `docs/rfcs/0022-web-ui-redesign.md` status →
  `accepted (V1)`.
- `docs/rfcs/0013-local-web-ui.md` adds a note that V1's
  static SPA is superseded by RFC 0022 V1.
- `docs/rfcs/README.md` index updated.
- `CHANGELOG.md` `## 1.11.0 — 2026-05-09` section.
- `pyproject.toml` adds `dependencies = ["jinja2>=3.1"]` and
  bumps to `1.11.0`. `src/striatum/__init__.py` to `1.11.0`.

## 9. Zero-regression contract

The JSON API (`/v1/*`) and SSE feed (`/events`) are unchanged.
Existing tests at `tests/test_web_ui.py` covering API behavior
continue to pass. The CSP header is byte-identical. Mutation
gating logic unchanged.

The static SPA (`app.js`'s hash-router) is *removed*; the
posture chip + verdict badge JS that lives in `app.js` is
factored into `mutations.js`. This is intentional removal,
not regression — the new pages render server-side.

## 10. Out of scope (V1.5 candidates)

- Inline Markdown rendering on `artifact_view.html`.
- SVG graph zoom/pan.
- Dark mode toggle (V1 uses `prefers-color-scheme` only).
- Search/filter on the run list.
- A `/v1.5` API namespace for new fields.
