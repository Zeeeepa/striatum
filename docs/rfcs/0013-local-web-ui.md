# RFC 0013: Local Web UI

Status: accepted (V1+step 7)
Date: 2026-05-08

## V1 Implementation Slice

Implemented under dogfood-007 (v0.3.0). Steps 1–6 of the RFC's
implementation path landed; step 7 (mutation buttons) deferred.

- `src/striatum/web/static/{index.html,app.js,app.css}` (new) —
  vanilla ES module SPA with hash-based routing for run list, run
  detail, job detail, artifact viewer, doctor view. Tiny in-house
  Markdown renderer (no external dependencies; HTML escaped at the
  input boundary per design-review F2).
- `src/striatum/service.py` — `--web` flag now serves the bundled
  assets; `/` and `/static/<path>` resolve via
  `importlib.resources.files("striatum.web.static")`.
  `Content-Security-Policy` headers reinforce the no-third-party
  origin invariant.
- New endpoint `GET /v1/artifacts/<id>/raw` streams artifact file
  bytes for the viewer (design-review F1).
- `pyproject.toml` ships static assets via
  `[tool.setuptools.package-data]`.
- 8 tests in `tests/test_web_ui.py` covering: assets served when
  `--web` is on, 404 when off, CSP header, path-traversal rejection,
  artifact-raw endpoint, no-external-URL invariant, importlib
  resolution, /v1/* endpoints alongside /static.

Per-kind front-matter rendering (decision badge, finding verdict
+ severity chip, harness-improvement-proposal target chip,
synthesis input list) lands as documented in synthesis § 4.
Cross-artifact link resolution is best-effort: `art_<hex>` ids
resolve via `/v1/runs/<id>/why?id=<art>` when present.

Deferred per the synthesis: Mermaid loader for embedded diagrams,
mutation buttons (step 7), workflow-file viewer, browser
notifications, full CommonMark renderer, a11y polish.


Context:
RFC 0012 (Local Service API),
`docs/DECISION_LOG.md` (D006, D007, D020, D028),
`src/striatum/dashboard.py`,
`docs/SPEC.md` § "Local API And MCP Wrapper Boundary"

## Problem

D006 promised that "Slack, TUI, and web dashboards can later attach
through the same state store via CLI or a local API." The TUI dashboard
landed (`striatum dashboard --run-id <id>`). The local API plumbing
landed in pieces (`striatum.api.invoke`, the MCP stdio wrapper, and now
RFC 0012's `striatum serve`). The remaining piece is the web UI.

Operator pain points the web UI addresses:

- **No quick visual run overview without a terminal session.** The TUI
  dashboard works, but it requires keeping a terminal open and
  redrawing every 2 seconds. Editor users want a tab.
- **Job graph inspection.** The Mermaid output from `workflow graph`
  is a static snapshot; a live render that shades job state and
  highlights the current claimable would be more useful for operators
  babysitting a run.
- **Artifact and event browsing.** Today this requires `striatum
  why <id> --verbose --json` plus jq. A web view can show artifact
  bodies, event timelines, and verdict history without command
  composition.
- **Doctor surfacing.** `striatum doctor` is informative but easy to
  forget to run. A persistent "Doctor: 1 warning" badge in the
  browser keeps it visible.

The web UI is operator-facing, read-only by default, and does not
become Striatum's product identity. The CLI is still primary; this is
a third adapter on top of D006's coordination layer.

## Goals

- Provide a browser-rendered view of runs, jobs, artifacts, evidence,
  and doctor output.
- Read-only by default; write actions (verdict, decision record,
  claim-next) gated behind `--allow-mutations` from RFC 0012.
- Bundle the static SPA inside the runner — no external build step at
  install time, no `npm install`, no node_modules in the published
  wheel.
- Localhost-only, served by `striatum serve --web`. No standalone
  hosting, no production deployment story.
- Live updates via SSE from RFC 0012. No polling.

## Non-Goals

- Multi-user authentication or shared sessions. Single-operator local
  tool.
- Workflow authoring in the browser. Workflows stay file-based; the
  UI is read + verdict + decision, not author.
- Replacing the CLI or the TUI. Both stay primary.
- A theming system, plugin marketplace, or extensible widget API.
- Mobile / responsive design beyond "doesn't break at narrow
  widths". This is a developer-laptop tool.
- Hosted SaaS. D020's no-hosted-services boundary is preserved.

## Proposal

Ship a static single-page application bundled with the runner under
`src/striatum/web/static/`. Serve it via the RFC 0012 service when
`--web` is passed. Build with no external dependencies at install
time; if a build step is needed, run it during package publish and
commit the build output.

### Directory layout

```text
src/striatum/web/
  __init__.py
  static/
    index.html
    app.js
    app.css
    favicon.svg
  templates/                 # if any HTML is server-rendered (none in V1)
```

The runner imports `striatum.web` only when `striatum serve --web` is
invoked; the static assets are not loaded by other code paths.

### Pages / views

V1 ships four views, each backed by RFC 0012 read endpoints plus SSE:

1. **Run list** (`/`)
   - All runs, newest first; state badges (`running`, `completed`,
     `failed`, `canceled`).
   - Click-through to run detail.
   - Backed by `GET /v1/runs`.

2. **Run detail** (`/runs/<run_id>`)
   - Header: workflow id, branch, state, started_at, latest verdict
     summary.
   - Job graph rendered as SVG (Mermaid → SVG client-side, or
     direct DOT → SVG via the existing `workflow graph --format dot`
     output). Nodes shade by state; current claimable jobs glow.
   - Job list with state, lane, role, latest verdict.
   - Live event log on the right pane, scrolling, oldest-first or
     newest-first toggle.
   - Backed by `GET /v1/runs/{run_id}/dashboard` and
     `GET /v1/runs/{run_id}/events` (SSE).

3. **Job detail** (`/runs/<run_id>/jobs/<job_id>`)
   - Work-packet view (the JSON the agent saw).
   - Artifact list with sha256, kind, logical_name, path. Click an
     artifact to open the artifact viewer described under "Artifact
     rendering" below.
   - Event timeline filtered to this job.
   - Verdict history if it's a review job.
   - Blocker history if it has blockers.
   - Backed by `POST /v1/invoke {"argv": ["why", "--id", ...]}`
     and the convenience read endpoints.

4. **Doctor / health** (`/doctor`)
   - The output of `striatum doctor` rendered with severity colors.
   - Reload-on-click; SSE stream for `striatum.doctor` events when
     they exist.
   - Backed by `GET /v1/doctor`.

### Artifact rendering

Striatum's artifact corpus is small and structured: every committed
artifact has a registered `kind` (per RFC 0003 / 0004 / 0005, plus
the existing `prompt` / `finding` / `findings_ledger` / `synthesis`
/ `decision` / `marker` / `handoff`), a sha256, and a logical name
the workflow author declared. Most kinds carry validated YAML front
matter. The viewer treats that structure as a first-class signal,
not as text to render and forget.

The artifact viewer lives at
`/runs/<run_id>/jobs/<job_id>/artifacts/<artifact_id>` and renders
in three stacked sections:

1. **Header.** The standard metadata Striatum already records:
   `kind`, `logical_name`, `path`, `sha256` (truncated, click to
   expand), `published_at`, publishing session id, and a "view raw"
   link that returns the file as it was committed (no rendering).

2. **Front-matter panel.** When the artifact's `kind` has a
   registered front-matter schema (per `striatum.artifacts.
   FRONT_MATTER_SCHEMAS`), the validated fields render as a
   structured key/value list above the body. Specific kinds get
   per-kind formatting:

   - `decision`: `outcome` rendered as a colored badge
     (`accepted` green, `rejected` red, `accepted_with_follow_up`
     amber). `follow_up_required` shown as a checkbox-like glyph.
     `created_at` as a relative timestamp.
   - `finding`: `verdict_intent` rendered as a badge
     (`accept` / `accept_with_findings` / `needs_revision` /
     `reject`); `severity` as a colored chip
     (`info` / `low` / `medium` / `high` / `critical`).
   - `findings_ledger`: `summary_count` rendered as a count;
     `entries_path` linked if present.
   - `support_ledger`: `audited_artifact` resolved as a link
     (see "cross-artifact links" below); `claim_count` shown
     prominently.
   - `action_item_ledger`: `source_review_artifact` linked;
     `revision_round` shown as "round N of M" when the workflow
     declares a max-iterations cycle.
   - `harness_improvement_proposal`: `target` as a chip
     (`prompt` / `workflow` / `spec` / `defaults` /
     `documentation`); `expected_benefit`, `risk`, `rollback`
     as labelled rows.
   - `synthesis`: `inputs` rendered as a list of clickable links
     (each input is either a repo path or another artifact id;
     resolve when possible).

   Front matter that fails schema validation surfaces as a warning
   banner in this panel; the rendering does not silently drop it.

3. **Body.** The artifact's text content below the front-matter
   block.

   - **Markdown.** Default renderer for `.md` artifacts. Standard
     CommonMark plus GFM tables. No HTML passthrough (XSS
     posture).
   - **Embedded Mermaid.** Fenced ```` ```mermaid ```` blocks in
     the body render as SVG inline using the same Mermaid loader
     the run-graph view uses. The Mermaid library is shared, not
     re-imported per artifact.
   - **JSON.** Default renderer for `.json` artifacts: pretty-
     printed with collapsible objects/arrays. The header's "view
     raw" link still serves the original bytes.
   - **Other text.** Rendered as a `<pre>` block with syntax
     highlighting when the file extension matches a known
     language (Python, Bash, TOML, YAML); otherwise plain
     monospace.
   - **Unknown / binary.** The body is **not** rendered inline;
     the viewer shows a placeholder and the size, plus a "view
     raw" link that triggers a download. V1 does not embed image
     viewers, log tails, or binary diff tools.

#### Cross-artifact links

When the front matter or body references another artifact (by id
or by `audited_artifact` / `findings_artifact_id` /
`source_review_artifact` / synthesis `inputs`), the viewer
resolves the reference. Resolution rules:

- A pure artifact id (`art_<hex>`): looked up via
  `POST /v1/invoke {"argv": ["why", "--id", "<id>", "--json"]}`
  and rendered as a hover-card with kind, logical_name, and a
  click-through to the artifact viewer.
- A repo-relative path (`docs/...`): linked to the run-relative
  artifact viewer when an artifact at that path exists in the
  current run; otherwise rendered as a plain text path with a
  tooltip explaining no artifact was found.
- A workflow id or run id: linked to the corresponding run /
  workflow page when present.

Unresolved references render as plain text — never as broken
links. The hover-card and click-through are progressive
enhancements that fail closed.

#### Bodies the viewer does not change

The viewer never modifies the artifact bytes. Markdown rendering
is for display only; "view raw" always serves the committed
content. The viewer also does not re-validate the front-matter
schema (the publish boundary already does that and stored the
result in the artifact row); it only re-reads the validated
shape.

### Optional write actions (`--allow-mutations`)

When the service is started with `--allow-mutations`, the UI exposes
explicit, narrowly-scoped buttons:

- **Record verdict.** On a review job's detail page, with verdict
  intent prefilled from the artifact's front matter.
- **Record decision.** On a run detail, opens a small form for
  outcome / title / follow-up text.
- **Claim next.** From the run detail page; spins until a packet
  arrives or a timeout fires.
- **Block.** From the job detail page; opens a small form for
  severity / reason.

Each button sends a `POST /v1/invoke` with the matching argv. Other
mutation commands (e.g., `recovery`, `migrate`) remain CLI-only in
V1; the UI does not surface them.

### Live updates (no polling)

The run detail and job detail pages subscribe to
`GET /v1/runs/{run_id}/events` via the browser's `EventSource`. New
events appear without page reload. The SVG job graph re-shades on
relevant events (`job.state_changed`, `verdict.recorded`,
`run.terminal`).

### Static asset stack

V1 chooses a tiny stack to keep the publish-time build pipeline
minimal:

- **HTML**: hand-written `index.html` with one `<div id="app">`
  mount point.
- **JavaScript**: vanilla ES modules. Optional: Preact (≈ 4KB) for
  templating if the hand-written DOM grows. No React, no Next.js.
- **CSS**: hand-written `app.css`. No Tailwind / Bootstrap. Aim for
  ~5KB.
- **Job graph rendering**: prefer Mermaid via `mermaid.min.js` (≈
  500KB minified, vendored) bundled in `static/vendor/mermaid.js`.
  If that is too heavy at install time, fall back to rendering the
  DOT output via `viz.js` or to SVG generated server-side from DOT.

The runner's published wheel includes the static assets verbatim.
There is no `package.json`, no `npm install`, no Node dependency to
run the runner. If a build is needed, it runs at PR-merge time, not
install time.

### What the user sees

```text
striatum serve --web --unix /tmp/striatum.sock
[striatum] service ready on /tmp/striatum.sock
[striatum] open http://localhost/   (use socat to expose to a browser)
```

For Unix-socket users, document a one-liner to expose it to
`localhost:8080` for the browser:

```bash
socat TCP-LISTEN:8080,fork,reuseaddr UNIX-CONNECT:/tmp/striatum.sock
```

For HTTP users:

```text
striatum serve --web --host 127.0.0.1 --port 8080
[striatum] service ready on http://127.0.0.1:8080/
```

## Acceptance Criteria

- `striatum serve --web --port 8080` boots and `GET /` returns
  `index.html`.
- `GET /` (without `--web`) returns 404; `/v1/*` still works.
- The run list at `/` renders all known runs with correct state
  badges.
- The run detail page at `/runs/<run_id>` renders the job graph,
  shades nodes by state, and updates live via SSE when an event is
  inserted.
- The job detail page lists artifacts and links to the artifact
  viewer; the event timeline filters to the job.
- The artifact viewer renders the per-kind front-matter panel
  (decision badge, finding verdict + severity chip, ledger
  counts, harness-proposal target chip), the body (Markdown with
  embedded Mermaid SVG, JSON pretty-printed with collapsible
  nodes, syntax-highlighted text for known languages, placeholder
  + raw download for binary), and resolves cross-artifact
  references (`art_<hex>`, `audited_artifact`,
  `findings_artifact_id`, `source_review_artifact`, synthesis
  `inputs`) as hover-cards with click-through.
- Without `--allow-mutations`, no UI button issues a mutation; the
  buttons are absent or disabled with a tooltip explaining the gate.
- With `--allow-mutations`, "Record verdict" successfully calls
  `POST /v1/invoke` and the page reflects the new verdict via SSE.
- The static asset bundle is < 1MB (Mermaid included) or < 100KB
  (without Mermaid).
- `tests/test_web_ui.py` covers: route smoke (`/`, `/v1/runs`,
  `/runs/<id>` returning the SPA shell), 404 when `--web` is off,
  and end-to-end against a fixture run that the API exposes.

## Open Questions

- **Mermaid vs viz.js vs server-side SVG.** Mermaid is heavy but
  client-side rendering is the easiest path. Server-side SVG (DOT
  → SVG via Graphviz at request time) avoids the JS payload but
  requires Graphviz on the operator's machine. V1 leans
  Mermaid-client-side; revisit if payload size complaints arrive.
- **Preact vs vanilla.** Vanilla is fine for the four V1 views.
  Preact keeps complexity bounded as views grow. Choose at
  implementation time; not load-bearing for the RFC.
- **Mutation surface scope.** Verdict, decision, claim, block are
  the four V1 buttons. Should `recovery requeue-stale`,
  `session close`, `worktree release` join? Probably not in V1; the
  CLI remains the recovery surface. Operator buttons in the UI
  imply the operator has decided to act; recovery commands often
  benefit from the contextual review the CLI's `--json` output
  encourages.
- **Notifications.** Browser notifications when a run transitions
  to `failed` or `completed`? Out of scope for V1; revisit after
  operator usage data exists.
- **Auth on mutation buttons.** When `--allow-mutations` is set,
  should each mutation button require a re-confirm dialog? V1
  recommendation: yes for `verdict reject` / `block` /
  `recovery cancel`; no for `verdict accept` / `decision record`
  (the form is already a confirmation).
- **Workflow authoring view.** Browser-side editing of workflow
  JSON files is in the non-goals list, but reading the file with
  syntax highlighting (and a Mermaid render of the workflow graph)
  is reasonable for "I want to inspect what this workflow does
  before I prepare a run." Borderline V1; lean defer to V2.
- **Persistence of operator preferences.** Newest-first vs
  oldest-first event log, default refresh cadence, etc. V1 keeps
  preferences in `localStorage`; nothing persisted server-side.
- **Open Telemetry / metrics.** D020 forbids hosted telemetry.
  Local-only metrics (`/metrics` endpoint with Prometheus shape)
  are conceivable but out of scope for V1.
- **Artifact diffing across revision rounds.** When `cycles`
  produce v1 / v2 / v3 of the same logical artifact, side-by-side
  diff would be informative for reviewers tracking what changed.
  The artifact viewer already renders one version at a time; a
  diff view is additive on top. Out of scope for V1; revisit in a
  follow-up RFC once the V1 viewer ships and operators ask.
- **Image and trace artifacts.** V1 treats unknown / binary kinds
  as "show metadata + raw download." If a future kind adds
  embedded images (e.g., `striatum.benchmark.v1` with a chart),
  the viewer will need an image render path. Deferred until a
  concrete kind motivates it.
- **Front-matter editing.** The viewer is read-only. A reviewer
  who wants to tweak `verdict_intent` from a finding's front
  matter does it via the CLI today and via the
  "Record verdict" mutation button in V1. Direct front-matter
  editing in the browser stays out of scope; workflows continue
  to treat artifacts as immutable once published.
- **Artifact viewer for `decision` artifacts that did not pass
  through `publish-artifact`.** `striatum decision record` writes
  the artifact directly; the viewer should still resolve it via
  the standard artifact id since `decision record` registers the
  row. No special-casing needed.

## Relationship To Other RFCs

- **RFC 0012 (proposed, prerequisite)** — provides every endpoint
  the UI consumes. The web UI does not query SQLite directly; it
  goes through `/v1/*` and SSE.
- **RFC 0007** — workflow visualization. The job graph view in the
  run detail page renders RFC 0007's graph output; this RFC does
  not change the graph contract.
- **RFC 0009** — long-lived supervision. Independent. The web UI
  observes supervised runs; it does not start or stop supervisors.
- **D028 (no transcripts)** — the UI does not capture or persist
  agent stdout/stderr. The browser shows what's already in
  SQLite.
- **D020 (no hosted services)** — preserved. The UI is bundled,
  served locally, never phones home, has no third-party CDN
  imports.

## Implementation Path

V1 builds in this order, each step landable independently:

1. **Static-asset scaffolding.** `src/striatum/web/static/` with
   placeholder `index.html`. `striatum serve --web` returns it. No
   real UI yet. (Smallest possible PR.)
2. **Run list view.** Basic table backed by `GET /v1/runs`.
3. **Run detail view + SSE.** Job graph, event log live.
4. **Job detail view.** Artifact list, event timeline filtered to
   the job, verdict history. Artifacts are listed but click-through
   in this step opens the raw view; the structured viewer lands
   next.
5. **Artifact viewer.** Per-kind front-matter panel (decision /
   finding / ledger / harness-proposal / synthesis / etc.), body
   rendering for Markdown (with embedded Mermaid), JSON, syntax-
   highlighted text, and the binary-placeholder fallback.
   Cross-artifact link resolution. Read-only; bytes never modified.
6. **Doctor view.**
7. **Mutation buttons** (verdict, decision, claim, block) gated by
   `--allow-mutations`.

Each step has its own acceptance test in `tests/test_web_ui.py`.
RFC 0013 is "accepted" once steps 1–6 land; step 7 may land in a
follow-up RFC if mutation policy needs more deliberation.
