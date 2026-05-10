# Research: web chat + browse touchpoints

author: researcher-claude-opus-001
date: 2026-05-09

## Existing surfaces

### `service.py` route table

`_dispatch_get` matches paths in order:
1. `/v1/health`, `/v1/runs`, `/v1/doctor`, `/v1/runs/...`,
   `/v1/artifacts/.../raw` — JSON API.
2. `/`, `/run/<id>...`, `/doctor` — RFC 0022 V1 SSR pages.
3. `/static/*` — bundled CSS / JS.
4. 404 fallback.

V1 inserts new branches for `/chat`, `/chat/<id>`,
`/chat/<id>/events` (SSE), `/view/<path>`. POSTs to
`/chat/new`, `/chat/<id>/send`, `/chat/<id>/stop` in
`_dispatch_post`.

### Outbound HTTP for chat

stdlib `http.client` is sufficient (HTTP/1.1, manual SSE
parsing ~50 LoC per flavor). No new HTTP runtime dep needed.

### Markdown library

`markdown-it-py` 4.0+ — ~150 KB, well-maintained, CommonMark
+ GFM. Disable raw HTML at parse time (`md.disable("html_inline").disable("html_block")`)
so no separate sanitizer is required.

### Cross-tab messaging

`BroadcastChannel` is CSP-`script-src 'self'`-compatible.
Deferred to V1.5 anyway (no full file-tree UI in V1).

### V1 scope (compact for autonomous run)

- Step 1: chat lifecycle (both API flavors).
- Step 2: `/view/<path>` minimum file-view endpoint.
- Step 3: Markdown rendering on artifact_view.html
  (closes RFC 0022 V1.5).

Deferred to V1.5: full file-tree UI, "add to chat" island,
multi-tab BroadcastChannel.

## Summary table

| V1 surface | File | Action |
| --- | --- | --- |
| Outbound HTTP | `src/striatum/web/chat_provider.py` | new |
| Markdown | `src/striatum/web/markdown.py` | new |
| Routes | `src/striatum/service.py` | `/chat/*`, `/view/<path>` |
| Templates | `src/striatum/web/templates/` | `chat.html`, `chat_index.html`, `view_file.html`; update `artifact_view.html` + `base.html` |
| CSS | `src/striatum/web/static/base.css` | chat layout |
| JS island | `src/striatum/web/static/chat.js` | SSE + form submit |
| Tests | `tests/test_web_chat.py`, `tests/test_web_view.py` | per synthesis |
| Deps | `pyproject.toml` | add `markdown-it-py>=4.0` |
