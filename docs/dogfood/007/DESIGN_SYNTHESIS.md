---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0013-local-web-ui.md", "docs/rfcs/0012-local-service-api.md", "docs/dogfood/007/research/WEB_UI_SURFACE.md", "src/striatum/service.py", "src/striatum/artifacts.py"]
---

# RFC 0013 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0013 (steps 1–6 of its
implementation path). Read-only UI; mutation buttons deferred to
step 7 / future RFC.

## 1. File Layout

```text
src/striatum/web/
  __init__.py
  static/
    index.html
    app.js
    app.css
```

Asset reads via `importlib.resources.files("striatum.web.static")`.

## 2. service.py Changes

- New constant `_STATIC_DIR_PACKAGE = "striatum.web.static"`.
- New helper `_serve_static(handler, relative_path)` that resolves
  the asset, sets Content-Type, returns bytes.
- `_dispatch_get` checks `path` against the static-asset routes
  **after** the API endpoint table:
  - When `state.web_enabled` is True and `path == "/"` → serve
    `index.html`.
  - When `path.startswith("/static/")` → serve the matching asset.
  - Falls through to the existing 404 otherwise.
- Content-Security-Policy header on every static response:
  `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:`.
- The existing `web_warning` startup field stays (keeps RFC 0012's
  contract intact); operators who run `--web` now see the UI
  instead of the 404 documented for V1.

## 3. SPA Shape (vanilla JS)

`index.html`:

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:">
  <title>Striatum</title>
  <link rel="stylesheet" href="/static/app.css">
</head>
<body>
  <div id="app">Loading...</div>
  <script type="module" src="/static/app.js"></script>
</body>
</html>
```

`app.js`:

- Hash-based routing: `#/` (run list), `#/runs/<id>` (run detail),
  `#/runs/<id>/jobs/<job_id>` (job detail),
  `#/runs/<id>/artifacts/<artifact_id>` (artifact viewer),
  `#/doctor` (doctor view).
- `fetch` against `/v1/*` endpoints; treat the response envelope as
  the source of truth.
- `EventSource` against `/v1/runs/<id>/events` on the run detail
  view; close on view change.
- One render function per view; rerender on hashchange.
- Tiny Markdown helper: paragraphs, headings (`#`, `##`, `###`),
  lists (`- ` / `* `), `**bold**`, `_italic_`, `` `code` ``,
  fenced ```lang blocks → `<pre>`. No HTML passthrough.

`app.css`:

- Monospace; state badges (running=blue, completed=green,
  failed=red, blocked=amber, canceled=grey, queued=neutral).
- Two-column layout for run detail (jobs left, event log right).
- Cards for artifacts and blockers.

## 4. Per-Kind Artifact Rendering

Per-kind renderers in `app.js` keyed off `artifact_kind`:

| Kind | Front-matter panel formatting |
|---|---|
| `decision` | `outcome` colored badge; `follow_up_required` glyph; `created_at` relative timestamp |
| `finding` | `verdict_intent` badge; `severity` colored chip; `tags` chips |
| `findings_ledger` | `summary_count` highlighted; `entries_path` linked |
| `synthesis` | `inputs` linked list (heuristic: paths get plain links, `art_*` ids resolve via `/v1/runs/<id>/why?id=<art>`) |
| `support_ledger` | `audited_artifact` linked; `claim_count` highlighted |
| `action_item_ledger` | `source_review_artifact` linked; `revision_round`, `total_items` shown |
| `harness_improvement_proposal` | `target` chip; `expected_benefit`, `risk`, `rollback` labelled rows |
| `prompt`, `marker`, `handoff` | minimal panel; just kind + sha + path |

The body section uses the Markdown renderer; bodies whose path
ends in `.json` use a JSON pretty-printer; others fall back to
`<pre>`. Mermaid blocks render as `<pre>` (deferred).

The viewer is read-only: bytes never modified. A "view raw"
fetches the artifact bytes via a new endpoint…

## 5. Service Endpoint Addition

To support raw artifact viewing the SPA needs the artifact body.
Two options:

- **(a)** Reuse `striatum why` JSON which surfaces artifacts, and
  have the SPA render bodies it has already received.
- **(b)** Add a new `/v1/artifacts/<artifact_id>/body` endpoint
  that streams the raw file bytes.

V1: take **(a)**. `striatum why <artifact_id> --verbose --json`
already returns the artifact row including `repo_path`. The SPA
performs `fetch("/static/__artifact__/" + path)` — wait, that
doesn't work because `repo_path` is repo-relative, not asset-
relative.

Final V1 design: add a small `/v1/artifacts/<id>/raw` endpoint to
`service.py`. It looks up the artifact row, opens the file from
disk, streams bytes with the appropriate Content-Type. Read-only;
no mutation gate needed. This is a tiny additive change to RFC
0012's V1 surface; recorded as an extension.

## 6. Test Plan

`tests/test_web_ui.py` (new):

| Test | Asserts |
|---|---|
| `test_static_assets_served_when_web_enabled` | GET `/` returns index.html, /static/app.js returns the JS, /static/app.css returns the CSS. All have `Content-Type` set correctly. |
| `test_static_assets_404_when_web_disabled` | Without `--web`, GET `/` returns 404. |
| `test_csp_header_on_static_responses` | CSP header is present and forbids external sources. |
| `test_artifact_raw_endpoint` | `/v1/artifacts/<id>/raw` returns the file bytes for a published artifact. |
| `test_static_assets_no_external_urls` | Walk index.html / app.js / app.css; assert no `http://`, `https://`, `cdn.`, `googleapis.` outside loopback comments. |
| `test_assets_resolvable_via_importlib_resources` | The bundled assets are reachable via `importlib.resources.files("striatum.web.static")` (wheel-install regression guard). |

Mutation-button tests deferred (step 7 out of V1 scope).

## 7. pyproject.toml

Add `[tool.setuptools.package-data] "striatum.web" = ["static/*"]`
so `pip install striatum` includes the assets.

## 8. Doc Updates

- `docs/SPEC.md` — add a "Local Web UI (RFC 0013 V1)" subsection
  under "Local Service".
- `docs/UBIQUITOUS_LANGUAGE.md` — add "local web UI".
- `docs/rfcs/0013-local-web-ui.md` — status from `proposed` to
  `accepted (V1)` with an Implementation Slice subsection.
- `docs/rfcs/README.md` — index status flip.
- `docs/DECISION_LOG.md` — D-row.
- `docs/TODO.md` — F-row.
- `README.md` — paragraph under "Local Service".
- `CHANGELOG.md` — Unreleased → 0.3.0 promotion.
- `pyproject.toml` — version bump to 0.3.0.

## 9. Deferred (Out Of V1)

- Mermaid loader for embedded diagrams.
- Mutation buttons (step 7).
- Workflow file viewer with syntax highlighting.
- Browser notifications on terminal-state transitions.
- localStorage operator preferences beyond defaults.
- A11y polish (keyboard shortcuts, ARIA labels).

## 10. Versioning

Per the operator's "add releases as you land these" preference,
this RFC bumps `0.2.0 → 0.3.0`. The CHANGELOG promotes the new
Unreleased section accordingly. After FF-to-main, tag `v0.3.0`.

## Acceptance Gate

The implementation job blocks until a human acceptance decision is
recorded under `docs/dogfood/007/decisions/`.
