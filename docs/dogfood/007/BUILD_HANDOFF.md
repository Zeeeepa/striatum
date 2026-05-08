---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0013 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-007 / RFC 0013 (Local Web UI)
Decision: `accepted_with_follow_up` (autonomous)
Version: `0.3.0`

V1 build slice (RFC 0013 steps 1–6) ships in one commit. Two
design-review findings (F1, F2) folded in.

## Files Changed

- **`src/striatum/web/__init__.py`** (new) — package marker.
- **`src/striatum/web/static/__init__.py`** (new) — marker so
  `importlib.resources.files()` resolves the dir.
- **`src/striatum/web/static/index.html`** (new) — SPA shell with
  `<div id="app">`, CSP meta, app.js module loader.
- **`src/striatum/web/static/app.js`** (new, ~280 lines) — vanilla
  ES module: hash routing, five views (run list, run detail with
  live SSE, job detail, artifact viewer, doctor), tiny Markdown
  helper that escapes HTML at the input boundary (F2),
  EventSource for live updates.
- **`src/striatum/web/static/app.css`** (new, ~130 lines) —
  monospace UI; state badges; verdict badges; severity chips;
  two-column layout; event log panel.
- **`src/striatum/service.py`** — new `_serve_static_asset` and
  `_handle_artifact_raw` handlers; routing extended to serve
  `/`, `/static/<path>`, `/v1/artifacts/<id>/raw` when `--web`
  is set.
- **`pyproject.toml`** — version `0.2.0` → `0.3.0`; new
  `[tool.setuptools.package-data]` to ship the static assets.
- **`scripts/release_metadata_check.py`** — sources expected
  version from `pyproject.toml` (no longer hardcoded `0.1.0`).
- **`tests/test_web_ui.py`** (new, 8 cases) — assets served when
  `--web` is on, 404 when off, CSP header, path-traversal
  rejection, artifact-raw endpoint via real workflow run,
  no-external-URL invariant on shipped bytes, importlib
  resolution, /v1/* alongside /static.
- **`tests/test_service.py`** — service-readiness window bumped
  to 30s; Unix-socket test uses a short `tempfile.mkdtemp` path
  so macOS's AF_UNIX path-too-long limit doesn't trigger.

## Documentation

- `docs/SPEC.md` — new "Local Web UI (RFC 0013 V1)" subsection
  under "Local Service".
- `docs/UBIQUITOUS_LANGUAGE.md` — "local web UI" entry.
- `docs/rfcs/0013-local-web-ui.md` — status from `proposed` to
  `accepted (V1)` with implementation slice subsection.
- `docs/rfcs/README.md` — index status flip.
- `docs/DECISION_LOG.md` — D059 row.
- `docs/TODO.md` — F7 done.
- `README.md` — new "Local Web UI (RFC 0013 V1)" subsection.
- `CHANGELOG.md` — `## 0.3.0 — 2026-05-08` section above the
  fresh `## Unreleased` placeholder.

## Tests / Lint / Typecheck

- `make test`: **233 passed** (was 225; +8 new web UI tests).
- `make lint`: clean.
- `make typecheck`: clean (46 source files).
- `tests/test_web_ui.py` + `tests/test_service.py`: 24/24 in ~21s.

## Validation Against Design-Review Findings

| Finding | Status | Notes |
|---|---|---|
| F1 (document `/v1/artifacts/<id>/raw`) | done | Added to SPEC's "Local Web UI" subsection and to RFC 0013's V1 Implementation Slice. Test `test_artifact_raw_endpoint` verifies via a real workflow run; missing-id 404s return the standard envelope shape. |
| F2 (HTML-escape Markdown at input boundary) | done | `escapeHTML` applied as the first step of `renderMarkdown`; only a small set of patterns is then restored. The CSP `script-src 'self'` and the escape-first approach defend in depth against XSS. |

## CI Fixes Folded In

The dogfood-006 push exposed three CI failures on macOS:

1. **Service tests "did not respond on port within 10s"** — macOS
   GitHub runners have slow cold-import startup. Fix: bump the
   service-readiness window to 30s. Local runs still resolve in
   under 1s.
2. **Unix-socket "AF_UNIX path too long"** — pytest's `tmp_path`
   on `/Users/runner/work/...` exceeds macOS's ~104-byte AF_UNIX
   limit. Fix: use `tempfile.mkdtemp(prefix="strs-")` for the
   socket path.
3. **Release-metadata check failed: Version** — the script
   hardcoded `EXPECTED_VERSION = "0.1.0"` and didn't track
   pyproject bumps. Fix: source the expected version from
   `pyproject.toml` with an `STRIATUM_RELEASE_VERSION` env
   override for CI matrices.

These are recorded in `docs/dogfood/FRICTION_LOG.md`.

## Versioning

`pyproject.toml`: `0.2.0 → 0.3.0`. CHANGELOG promoted from
`Unreleased` to `0.3.0`. Tag `v0.3.0` after FF-to-main push.

## How To Verify

```bash
.venv/bin/striatum --repo . serve --web --port 8080 &
sleep 1
curl -s http://127.0.0.1:8080/v1/health | python3 -m json.tool
curl -s http://127.0.0.1:8080/ | head -3   # index.html
curl -s http://127.0.0.1:8080/static/app.js | head -1  # app.js
kill $!
```

In a browser: navigate to <http://127.0.0.1:8080/> and confirm
the run list renders. Navigate to a run; the live event log
updates as new events fire.
