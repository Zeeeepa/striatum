# RFC 0022: Web UI Redesign

Status: accepted (V1)
Date: 2026-05-09
Context:
RFC 0013 (local web UI, accepted V1+step 7) — the existing SPA,
RFC 0007 (workflow visualization, accepted) — the workflow graph
data model the redesign reuses,
RFC 0016 (dashboard dependency graph, accepted V1+step 3) — the
ASCII graph renderer whose semantics the SVG renderer mirrors,
`src/striatum/web/static/` (current 950-LoC dependency-free SPA),
`src/striatum/service.py` (`BaseHTTPRequestHandler` + static-asset
serving)

## Problem

The web UI shipped by RFC 0013 V1 + step 7 is functional but
scrappy:

1. **Visual debt.** The CSS palette is from a developer's quick
   pass; typography is the system default; spacing is inconsistent
   between panels; there's no dark mode. Operators reading evidence
   on a CRT-bright laptop in a dim room have a bad time.
2. **The dependency graph is text.** RFC 0016 ships an ASCII /
   Unicode graph in the terminal dashboard. The web UI has no
   visual graph at all — operators inspecting a multi-job run have
   to read flat lists. RFC 0007 produces graph data the renderer
   can consume; the gap is purely a renderer.
3. **Hash routing limits sharability.** URLs like `#/run/<id>` are
   awkward to copy/paste into chat or issue trackers; the hash
   never reaches server logs for triage.
4. **Per-run dogfood Markdown is invisible.** Operators reading a
   run's evidence have to leave the UI and open
   `docs/dogfood/NNN/BUILD_HANDOFF.md` in a file viewer. The
   provenance the runner records is durably-on-disk Markdown; the
   UI should render it inline.

This RFC redesigns the UI to address (1) and (2) in V1 (the
operator-validated primary pain points), with (3) as a natural
side effect of moving to server-side routing and (4) deferred to
V1.5.

## Goals

- **Server-side rendered multi-page UI.** Each route is a real
  HTML page served from `/run/<id>` etc. (no hash, history-API
  URLs work via direct request). Pages render server-side via
  Jinja2; small interactive islands (verdict modal, SSE feed,
  refresh poll) are vanilla JS components mounted into named
  slots.
- **Refreshed visual design.** Modern typography, consistent
  spacing, semantic color palette, dark mode via
  `prefers-color-scheme`. No framework dependency for the look —
  the cost is paid in CSS, not in JS.
- **SVG dependency graph.** Render the workflow graph as a real
  SVG with state-colored nodes, directional edges, and (V1.5)
  interactive selection. Reuses `workflow_graph_data` and the
  `workflow_graph_dot` orchestrator from RFC 0007/0016. No
  client-side graph library — V1 emits SVG directly from the
  server-side renderer (the layout is small, ≤30 nodes typical;
  manual layout is tractable).
- **Preserve the local-first invariants.** No hosted services, no
  telemetry, no remote assets. CSP stays locked-down. The wheel
  ships pre-rendered templates as package-data; no Node toolchain
  added; `pip install` is still the only build step.
- **Preserve mutations.** RFC 0013 step 7's mutation buttons
  (verdict, decision, checkpoint resolve, requeue stale review-
  only) move to the new pages with the same gating
  (`STRIATUM_WEB_MUTATIONS=1`).

## Non-Goals

- **Node toolchain.** No npm, no pnpm, no Vite, no SvelteKit. The
  UI ships as Python + Jinja2 + plain CSS + small JS islands. A
  contributor can edit `src/striatum/web/templates/run_detail.html`
  in any text editor and see the change without a build step.
- **Heavy framework adoption.** No React/Vue/Svelte/HTMX. The JS
  islands are <100 LoC each and stay vanilla. RFC 0013's
  zero-supply-chain ethos wins.
- **Authentication / multi-tenancy / hosted operation.** Out of
  scope per `AGENTS.md` boundary. The redesign keeps the
  loopback / localhost binding model.
- **Real-time graph layout.** V1 ships a simple top-down or
  layered SVG layout (server-rendered). Force-directed
  / interactive zoom / pan is V1.6+ if operators want it.
- **WebSocket replacement of SSE.** The existing SSE event stream
  is fine. Don't replace working plumbing.
- **Authoring / writing artifacts from the UI.** The UI remains
  read + curated-mutation only. The CLI is the only legal
  *write* surface (D006/D009 boundary).

## Proposal

V1 ships in three landable steps. Each can be its own PR; together
they constitute V1.

### Step 1. Jinja2 templating + multi-page routing

Add **Jinja2** as a runtime dependency (the project's first).
Concretely, in `pyproject.toml`:

```toml
dependencies = ["jinja2>=3.1"]
```

Jinja2 is the canonical Python templating engine, ~250 KB,
well-maintained, no further transitive dependencies (markupsafe
which it pulls in is also stdlib-grade). The cost is one runtime
dep for substantial readability + correctness gains over
`str.format` / f-strings for HTML.

`src/striatum/web/templates/` ships:

- `base.html` — common shell (header, nav, footer, CSP meta tag)
- `run_list.html` — replaces hash-route `#/`
- `run_detail.html` — replaces `#/run/<id>`
- `job_detail.html` — replaces `#/run/<id>/job/<id>`
- `artifact_view.html` — replaces `#/artifact/<id>`
- `doctor.html` — replaces `#/doctor`

`src/striatum/service.py` route table extends:

```
GET /                         → run_list.html
GET /run/<run_id>             → run_detail.html
GET /run/<run_id>/job/<id>    → job_detail.html
GET /run/<run_id>/artifact/<id> → artifact_view.html
GET /doctor                   → doctor.html
GET /static/*                 → (unchanged) bundled assets
GET /api/*                    → (unchanged) JSON API
GET /events                   → (unchanged) SSE stream
```

Hash routes redirect to history-API URLs for one release for
backwards compatibility, then drop.

`importlib.resources` resolves templates from
`striatum.web.templates`; Jinja2 reads bytes via a
`PackageLoader` adapter. CSP unchanged
(`default-src 'self'`).

### Step 2. Visual polish

Rewrite `app.css` (260 LoC) into `base.css` + per-page CSS as
needed. Concretely:

- **Typography.** System font stack with explicit weights:
  ```
  font-family: -apple-system, BlinkMacSystemFont,
    "Segoe UI", "Roboto", "Helvetica Neue", Arial, sans-serif;
  ```
  Monospace for IDs and code:
  ```
  font-family: ui-monospace, SFMono-Regular, "Cascadia Code",
    "JetBrains Mono", Consolas, "Liberation Mono", monospace;
  ```
- **Palette.** Semantic CSS custom properties: `--bg-base`,
  `--bg-elevated`, `--fg-primary`, `--fg-secondary`,
  `--accent`, plus state-keyed colors `--status-running`,
  `--status-completed`, `--status-failed`, `--status-blocked`,
  `--status-canceled`, `--status-queued`. Dark mode via
  `@media (prefers-color-scheme: dark)` overrides only the
  custom-property values, so component CSS doesn't branch.
- **Spacing scale.** 4px-grid: `--space-1: 0.25rem` through
  `--space-8: 2rem`. All component padding/margin uses the
  scale.
- **Component refresh.** Badges become pill-shaped with
  consistent height; tables get zebra striping at low contrast;
  panel headers get a uniform shape; the next-actions list gets
  promoted to a sticky right-rail on `run_detail.html`.

No JS framework, no CSS framework. The CSS grows from ~260 LoC
to roughly ~500 LoC, all hand-written and readable.

### Step 3. SVG dependency graph

A new server-side renderer `striatum.web.graph_svg.render_run_graph(workflow, node_states)`
emits SVG markup that the `run_detail.html` template embeds
inline (no `<img src="...">` round-trip needed, since the SVG is
rendered in the same template pass).

The renderer:

- Reuses `workflow_graph_data(workflow)` for the graph topology.
- Uses a layered (Sugiyama-ish) layout: assign each node a layer
  by topological depth; spread nodes within a layer evenly;
  draw orthogonal edges between layers. This is RFC 0016's
  ASCII layout in pixels — same algorithm, different output.
- Colors nodes by current state (the same palette as the
  visual-polish CSS custom properties).
- Renders verdict + posture chips on review nodes (V1.5's
  introspection data already lives in `node_states`).
- Outputs a `<svg viewBox="0 0 W H" preserveAspectRatio="...">`
  that scales to its container.

Interactivity (V1):

- Click a node → navigate to that job's detail page.
- Hover a node → show a small floating tooltip with state +
  posture (when applicable).

V1.5 extensions: zoom/pan, force-directed alternative layout,
inline verdict chip rendering when the graph is the primary
view. Out of V1.

## Acceptance Criteria

- A fresh `pip install striatum-orchestrator` + `striatum serve
  --web` produces working pages at `/`, `/run/<id>`,
  `/run/<id>/job/<id>`, `/run/<id>/artifact/<id>`, `/doctor`
  with no hash in the URL bar.
- The CSP header on each page is `default-src 'self'; …` —
  identical to v1.10.0; no `unsafe-inline` or `unsafe-eval`
  added.
- Dark mode renders correctly when the OS sets
  `prefers-color-scheme: dark` (no toggle button in V1; the
  OS preference is the source of truth).
- The dependency graph SVG on a multi-job run renders all jobs
  as colored nodes with edges; click navigates; hover tooltip
  shows state.
- Hash-route URLs (`/#/run/<id>`) redirect to `/run/<id>`
  during V1 (for one release); v1.12+ may drop the redirect.
- Existing JSON API (`/api/*`) and SSE stream (`/events`)
  unchanged.
- Existing mutation buttons (verdict, decision, checkpoint
  resolve, requeue) work on the new pages.
- `tests/test_web_ui.py` cases continue to pass after route
  refactor (status codes, response bodies, mutation gating).
- New tests at `tests/test_web_ui_redesign.py` cover Jinja2
  template rendering, multi-page routing, SVG graph rendering,
  CSS variable presence, and the hash-route redirect.
- `make lint`, `make typecheck`, full `make test` pass.

## Open Questions

- **Jinja2 as the project's first runtime dep.** Is the wheel-
  size + supply-chain impact (one well-maintained package, one
  transitive `markupsafe`) worth the readability gain over
  string-formatted HTML? V1 commits to yes; the alternative
  is hand-written string concatenation, which past experience
  shows decays into XSS hazards quickly.
- **Hash-route deprecation timing.** V1 keeps the redirect
  for one release. v1.12+ drops it. Operators with bookmarks
  see a 302 once. Acceptable.
- **Template caching.** Jinja2 environment is constructed once
  per `service.py` server lifecycle; templates compile lazily
  and cache. No filesystem watcher for template hot-reload in
  V1 (operators don't iterate on templates from a running
  server in this tool's use case).
- **SVG accessibility.** `<title>` and `aria-label` on each
  node so screen readers can navigate. Pinned in V1.
- **Browser support.** No IE / legacy Edge. Chromium /
  Firefox / Safari latest two majors only. CSS custom
  properties + `prefers-color-scheme` require this.

## Implementation Path

V1 ships in the three steps above, in order.

1. **Step 1 (Jinja2 + multi-page routing):**
   `pyproject.toml` adds Jinja2; new `src/striatum/web/templates/`
   directory; `service.py` route table; tests for each page;
   hash-route redirect. Bumps to v1.11.0 on landing.
2. **Step 2 (visual polish):** rewrite `app.css` into
   `base.css` + per-page CSS; CSS custom properties for theme
   + dark mode; component refresh. Bumps to v1.11.0 (lands in
   the same release as step 1; the visual style is the
   payoff for the multi-page move).
3. **Step 3 (SVG dependency graph):** `striatum.web.graph_svg`
   module; embed in `run_detail.html`; click-navigate +
   hover tooltip; tests. Also lands in v1.11.0; the graph
   is the operator-validated primary pain point alongside
   visual polish, so V1 must include it.

V1.5 (separate dogfood) ships:

- Inline dogfood-Markdown rendering on `artifact_view.html`
  (the deferred priority from the user's scoping).
- SVG graph zoom/pan interactivity.
- Per-page metadata chips (posture, verdict, attempt) wherever
  they aren't already.

## Domain Modeling

This RFC adds two value objects to the model:

- **page route** — a value object naming a server-rendered URL
  pattern (e.g., `run_detail`, `job_detail`). Identity is the
  route name; equality is by-name. Constructed at template-
  registration time; never mutated in flight.
- **theme palette** — a value object naming the set of CSS
  custom properties for one rendering mode (light / dark).
  Closed set in V1; V1.5 may extend.

The runner's bounded context is *unchanged*. The redesign is
purely on the introspection surface; no new state stored, no
new mutations, no new validators. Per `docs/DDD.md § "Adding to
the model"`:

1. **Glossary** — `docs/UBIQUITOUS_LANGUAGE.md` adds entries for
   `page route` and `theme palette`.
2. **Pattern** — both are value objects.
3. **Validator** — N/A (not part of the SQL state).
4. **Surface** — visible in the rendered pages themselves.
5. **Citation** — DECISION_LOG D073 cites the new entries when
   the RFC lands.

## Relationship To Other RFCs

- **RFC 0013 V1 + step 7** (local web UI) — this RFC supersedes
  RFC 0013's static-SPA approach. RFC 0013 stays accepted (the
  `serve --web` verb and the JSON API are still load-bearing);
  the SPA implementation is replaced.
- **RFC 0007 V1** (workflow visualization) — the SVG graph
  reuses `workflow_graph_data`. No change to RFC 0007.
- **RFC 0016 V1 + step 3** (dashboard dependency graph) — same
  layout algorithm, different output medium. No change to
  RFC 0016.
- **RFC 0018 V1 + step 3** (review postures) — V1 of this RFC
  surfaces posture chips on review nodes in the SVG graph.
- **RFC 0021 V1 + V1.5** (DDD layout scaffold) — unrelated.
