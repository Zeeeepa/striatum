---
title: "RFC 0022 V1 build handoff (dogfood-020)"
date: 2026-05-09
---

# Build handoff: RFC 0022 V1 (web UI redesign)

author: implementer-claude-opus-001

## Scope

V1 ships RFC 0022's three steps. Both design-review findings
addressed in implementation/tests:

- **Finding 1** (note): cycles render as chip annotations, not
  graph edges. The SVG renderer reads `graph.edges` from
  `workflow_graph_data` (forward DAG only); `graph.cycles` are
  not touched.
- **Finding 2** (note): legacy `/static/index.html` request now
  returns 404 (the file is gone); the `test_static_assets_served_when_web_enabled`
  test was updated to assert the new shape (`/static/base.css`
  exists; `/static/legacy_hash_redirect.js` exists).

## Files

### New

- `src/striatum/web/templates/base.html` — common shell with
  CSP meta, header, nav, `{% block main %}`, deferred-load
  hash-redirect island.
- `src/striatum/web/templates/run_list.html` — table of runs
  with state pills + branch + empty-state copy.
- `src/striatum/web/templates/run_detail.html` — three-column
  layout: jobs rail / center status panel + verdicts-by-posture
  + next-actions / SVG graph rail.
- `src/striatum/web/templates/job_detail.html` — job state +
  metadata + artifacts list + verdict + posture chip.
- `src/striatum/web/templates/artifact_view.html` — artifact
  metadata + sha256 + raw API pointer (V1.5 will inline the
  Markdown body).
- `src/striatum/web/templates/doctor.html` — doctor output
  with check pills.
- `src/striatum/web/static/base.css` — refreshed palette via
  CSS custom properties, `prefers-color-scheme: dark` overrides,
  4px-grid spacing scale, system font stack, status pill +
  posture chip + table styles + run-grid 3-col layout.
- `src/striatum/web/static/legacy_hash_redirect.js` — the
  hash-redirect island (~10 LoC).
- `src/striatum/web/graph_svg.py` — `render_run_graph(workflow,
  node_states, *, run_id)` exports a layered SVG; layout uses
  longest-path topological depth; `compute_node_states_from_jobs`
  helper projects job rows to the renderer's input shape.
- `tests/test_web_ui_redesign.py` — 8 cases covering each new
  page, CSP preservation, dark-mode palette, hash-redirect
  island, SVG node rendering, click-navigate href.

### Modified

- `pyproject.toml`:
  - `dependencies = ["jinja2>=3.1"]` (was `[]`).
  - `[tool.setuptools.package-data]` adds
    `"striatum.web.templates" = ["*.html"]`.
- `src/striatum/service.py`:
  - New module-level `_jinja_env()` factory (lru_cached).
  - `_dispatch_get` adds `/run/<run_id>` + `/run/<run_id>/job/<id>`
    + `/run/<run_id>/artifact/<id>` + `/doctor` branches before
    `/static/*`. The `/` branch now renders `run_list.html`.
  - New `_render_run_list_page`, `_render_run_detail_page`,
    `_render_job_detail_page`, `_render_artifact_view_page`,
    `_render_doctor_page`, `_render_run_subpath` methods.
  - New `_send_html` helper (mirrors `_send_json` but with
    `text/html` Content-Type + the same CSP header).
- `tests/test_web_ui.py` — `test_static_assets_served_when_web_enabled`
  updated to assert the new templates / asset names (per
  Finding 2). All 16 pre-existing web-UI tests still pass.

### Removed (effectively)

- The hash-routed SPA mount in `app.js` is no longer used by
  any served page. `app.js` itself is still present in the
  repo (operators with old SPA bookmarks may still load it via
  direct URL until v1.12 cleanup); the new pages don't
  reference it. RFC 0022 V1.5 will retire `app.js` entirely
  after the deprecation period.
- `src/striatum/web/static/app.css` is no longer referenced;
  the new pages link `base.css`. `app.css` is left in place
  for one release.

### Docs

- `CHANGELOG.md` — `## 1.11.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to 1.11.0.
- `docs/DECISION_LOG.md` — D073.
- `docs/TODO.md` — F20.
- `docs/rfcs/0022-web-ui-redesign.md` — status →
  `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.

## Smoke

```
$ striatum --repo /tmp/some-target init
$ striatum --repo /tmp/some-target serve --web --port 8123
$ curl -s http://127.0.0.1:8123/ | grep -i "<h1"
<h1>Runs</h1>

$ curl -s http://127.0.0.1:8123/doctor | grep -i "<h1"
<h1>Doctor</h1>

$ curl -s -I http://127.0.0.1:8123/static/base.css \
  | grep -i "content-type\|content-security"
Content-Type: text/css; charset=utf-8
Content-Security-Policy: default-src 'self'; script-src 'self'; …
```

## Test results

- `tests/test_web_ui_redesign.py`: 8 / 8 pass.
- `tests/test_web_ui.py`: 16 / 16 pass (one test updated for
  the new asset names per Finding 2).
- `make lint`: clean.
- `make typecheck`: 64 source files, no issues.
- Full `make test`: pending — running while this handoff is
  drafted.

## Acceptance summary

| V1 acceptance gate | How it's satisfied |
| --- | --- |
| Multi-page routing with history-API URLs | Five new routes wired into `_dispatch_get` before `/static/*`. |
| Pages render server-side via Jinja2 | Six templates loaded via `PackageLoader("striatum.web", "templates")`. |
| CSP unchanged | `_send_html` uses the byte-identical CSP header from `_serve_static_asset`. Verified by `test_csp_header_unchanged`. |
| Refreshed CSS palette + dark mode | `base.css` defines `:root` palette + `@media (prefers-color-scheme: dark)` overrides. Verified by `test_dark_mode_palette_present_in_base_css`. |
| Layered SVG dependency graph with click-navigate | `graph_svg.render_run_graph` emits SVG with `<a href="/run/<id>/job/<id>">` wrappers. Verified by `test_run_detail_page_renders_html_and_svg` + `test_svg_node_link_targets_job_detail`. |
| Cycles not rendered as edges (Finding 1) | The renderer reads `graph.edges` only; `graph.cycles` is left to other surfaces. Source review of `render_run_graph`. |
| Legacy hash-route compatibility | `legacy_hash_redirect.js` ships as `/static/legacy_hash_redirect.js` and is `<script defer>`-loaded on every page. Verified by `test_legacy_hash_redirect_island_ships`. |
| JSON API + SSE unchanged | `/v1/*` routes are matched before the new HTML routes; no existing handler was modified. All `test_web_ui.py` API tests pass. |
| Mutation gating preserved | `STRIATUM_WEB_MUTATIONS` flag still controls whether mutation buttons render; the gating wrapper in `service.py` is unchanged. |
| Zero regression for `striatum serve --web` boot path | `_spawn_service` + ports test still pass. |
| Jinja2 added as runtime dep | `pyproject.toml` shows `dependencies = ["jinja2>=3.1"]`; the wheel resolves cleanly via `pip install -e .` adding markupsafe + jinja2. |

## Out of scope (V1.5 candidates)

- **Inline Markdown rendering on `/run/<id>/artifact/<id>`** —
  the artifact page currently shows metadata + a pointer to the
  raw API. V1.5 will load the file body and render it via a
  small Markdown renderer (likely `markdown-it-py` or hand-rolled
  for the limited subset striatum artifacts use).
- **SVG graph zoom/pan**.
- **Posture chip on graph nodes** for review jobs.
- **Search/filter on the run list** when run counts grow.
- **Retire `app.js` and `app.css`** after one release of
  deprecation.

V1 closes RFC 0022's primary scope: SSR + visual polish + SVG
graph. The deferred items can be sequenced as separate dogfoods
when operator evidence shows they're wanted.
