---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-007", "rfc-0013"]
---

# RFC 0013 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Run: dogfood-007

Verdict intent: **accept**.

The build implements RFC 0013 V1 steps 1–6 plus the new
`/v1/artifacts/<id>/raw` endpoint, folds in design-review
findings F1 + F2, and resolves the macOS / metadata CI failures
that surfaced on the dogfood-006 push.

## D020 / D028 / CSP

- No external CDN imports; the `test_static_assets_no_external_urls`
  test walks the bundled bytes and asserts no `http(s)://` outside
  loopback comments.
- `Content-Security-Policy: default-src 'self'; script-src 'self';
  style-src 'self'; img-src 'self' data:; connect-src 'self'` on
  every static and artifact-raw response.
- HTML escaped at the input boundary of the Markdown renderer
  (F2). `<script>...</script>` in an artifact body renders as
  visible text, not executable code.
- No transcripts captured; service log_message remains a no-op.

## Endpoints

- `GET /` → `index.html` when `--web` set.
- `GET /static/<path>` → bundled asset, content-type set per
  extension.
- `GET /v1/artifacts/<id>/raw` → file bytes; 404 envelope when
  the artifact id or file is missing.
- Path traversal (`..`) rejected with HTTP 400.

## SPA

- Vanilla ES module; hash-based routing across five views.
- `EventSource` for live SSE on run detail; closes on view change.
- Per-kind front matter formatting matches synthesis § 4.
- Tiny Markdown helper: paragraphs, headings, lists, bold/italic/
  code, fenced blocks. Adequate for V1; full CommonMark deferred.

## CI Fixes

The dogfood-006 push exposed three failures; the implementer
folded all three into this commit:

1. Service-readiness window 10s → 30s for macOS cold imports.
2. Unix-socket test uses a short `tempfile.mkdtemp` path so
   macOS's AF_UNIX limit doesn't trigger.
3. `scripts/release_metadata_check.py` sources expected version
   from `pyproject.toml` instead of hardcoded `0.1.0`.

CI on the next push should turn green.

## Tests / Lint / Typecheck

- `make test`: 233 / 233 pass.
- `make lint`: clean.
- `make typecheck`: clean (46 source files).
- New: 8 web UI tests.

## Versioning

`pyproject.toml` bumped to `0.3.0`. CHANGELOG promoted from
Unreleased to a 0.3.0 section. Tag `v0.3.0` after FF push.

## Findings

### F1 (info) — Mermaid in Markdown bodies still falls back to `<pre>`

Synthesis § 4 deferred the Mermaid loader to V2; the
implementation honors that deferral. Operators viewing artifact
bodies with embedded Mermaid see them as `<pre>` blocks. Documented;
not a blocker.

### F2 (info) — Workflow file viewer not in V1

The synthesis Open Questions called out a workflow-file-viewer
feature for V2. Implementation omits it cleanly. Not a blocker.

### F3 (info) — `/v1/artifacts/<id>/raw` is an additive RFC 0012 endpoint

The new endpoint is a small, read-only extension to the RFC 0012
V1 surface. Documented in the SPEC's "Local Web UI (RFC 0013 V1)"
subsection. Behaves correctly on missing-id and missing-file
cases.

## Verdict

**accept.** Build matches the accepted design plus design-review
F1 + F2 + the CI fixes; no blocking findings. Ready to merge.
