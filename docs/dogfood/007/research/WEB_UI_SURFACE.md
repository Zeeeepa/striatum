---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0013 V1 — Web UI Surface Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

Concise survey: RFC 0012 just shipped, so the API surface is in
hand. Confirm what RFC 0013 needs, fix the UI stack, list artifact
kinds for per-kind rendering.

## RFC 0012 Endpoints The UI Consumes

- `GET /v1/runs` — list (run list view).
- `GET /v1/runs/<id>` — run detail (jobs, blockers, verdicts, claimables).
- `GET /v1/runs/<id>/why?id=<entity>` — used by job detail to fetch
  events / artifacts for a given job_id.
- `GET /v1/runs/<id>/dashboard` — TUI's JSON shape (already includes
  jobs + state; usable for run detail).
- `GET /v1/runs/<id>/events` — SSE for live updates.
- `GET /v1/doctor` — doctor view.

All read endpoints work without `--allow-mutations`; the UI is
read-only for V1 (mutation buttons are step 7 / out of scope).

## Artifact Kinds + Rendering

From `src/striatum/artifacts.py:FRONT_MATTER_SCHEMAS`:

- `prompt`, `marker` — minimal front matter; render plain.
- `handoff` — `schema_version`, `artifact_kind` only; render body.
- `finding` — `verdict_intent` (badge: accept/accept_with_findings/
  needs_revision/reject), `severity` (chip: info/low/medium/high/
  critical), `tags` (chips).
- `findings_ledger` — `summary_count` (int), `entries_path` (link).
- `synthesis` — `inputs` (list of repo paths or artifact ids;
  resolve when possible).
- `support_ledger` — `audited_artifact` (link), `claim_count`.
- `action_item_ledger` — `source_review_artifact` (link),
  `revision_round`, `total_items`.
- `harness_improvement_proposal` — `target` chip, `expected_benefit`,
  `risk`, `rollback`.
- `decision` — `outcome` badge (accepted=green / rejected=red /
  accepted_with_follow_up=amber), `follow_up_required` (bool),
  `created_at` (relative).

Body renderers needed:
- Markdown (without HTML passthrough; XSS posture).
- JSON pretty-print with collapsible nodes.
- Plain text fallback.
- Binary placeholder + download link.

Mermaid embedded in Markdown bodies: deferred. V1 falls back to
showing fenced ```mermaid blocks as `<pre>`. Mermaid loader lands
in V2 once the bundle size budget is settled.

## Static Asset Stack

**Vanilla JS, no Preact for V1.** Five views are simple enough to
hand-roll without a framework. Total budget: ~30KB JS, ~10KB CSS.

- `src/striatum/web/static/index.html` — `<div id="app">` mount + a
  `<script type="module" src="app.js">`.
- `src/striatum/web/static/app.js` — hash-based routing; one render
  function per view; `EventSource` for SSE; `fetch` for everything
  else.
- `src/striatum/web/static/app.css` — minimal monospace UI; state
  badges; flex layouts.

No Markdown library; render `**bold**` / `_italic_` / `\` `code` `\``
/ `## headings` / lists / paragraphs via a tiny in-house regex
pass. Code-fences render as `<pre>`. Acceptable V1 surface; full
CommonMark is a follow-up.

No Mermaid. No Tailwind. No Preact. No build step.

## Service Wiring

`service.py` already accepts `--web`. Currently a no-op that returns
a 404 with an explanatory message. RFC 0013 V1 needs:

- A new handler branch for `GET /` and `GET /static/<path>` when
  `web_enabled` is True.
- The handler reads from `importlib.resources.files("striatum.web.static")`
  to find the file, sets the correct Content-Type, returns it.
- Asset paths resolved against the package's installed location; no
  filesystem-relative reads.

## MANIFEST.in / pyproject.toml

The static assets need to ship inside the wheel. Add to
`pyproject.toml`'s `[tool.setuptools.package-data]` or use
`MANIFEST.in`. Verify with `python -m build` (out of scope; the
test asserts importlib.resources can read the assets).

## Recommended Implementation Order

1. Add static assets (index.html, app.js, app.css) under
   `src/striatum/web/static/`.
2. Update `service.py` to serve them when `--web` is set.
3. Implement run list view (vanilla JS table).
4. Implement run detail view (jobs, blockers, SSE event log).
5. Implement job detail view (work packet + artifacts list).
6. Implement artifact viewer with per-kind front matter + body.
7. Implement doctor view.
8. Tests: HTTP smoke for each route, SSE integration via the
   existing service test patterns, no-external-URL invariant on
   shipped assets.

## Risks / Friction Anticipated

- **Asset packaging.** If MANIFEST.in is missing, `pip install`
  drops the assets. Test enforces `importlib.resources.files`
  resolves them.
- **Browser auto-test in CI.** No CI Selenium budget. Tests assert
  the HTML+CSS+JS payload bytes match expectations; manual
  browser verification noted in BUILD_HANDOFF.
- **CSP headers.** V1 ships a permissive Content-Security-Policy
  (no third-party origins, no inline-script except the bootstrap)
  to reinforce the no-external-URL invariant.

## V1 Scope Note

Steps 1-6 of RFC 0013 land. Step 7 (mutation buttons) explicitly
deferred per the RFC's own acceptance gate.
