---
title: "RFC 0023 V1 build handoff (dogfood-021)"
date: 2026-05-09
---

# Build handoff: RFC 0023 V1 (web chat + browse)

author: implementer-codex-gpt-5.5-001

## Scope

V1 ships per the synthesis: chat surface (provider-neutral, both flavors), `/view/<path>` minimum file-view endpoint, inline Markdown rendering on `/run/<id>/artifact/<id>`. Full file-tree browser UI deferred to V1.5.

## Findings disposition

| # | Posture | Severity | Disposition |
| --- | --- | --- | --- |
| F1-security | security | acceptance-blocking | **Implemented**: `validate_base_url` refuses non-loopback `http://` URLs. Test: `test_chat_provider_url_scheme_validation_rejects_remote_http`. |
| F2-security | security | note | **Implemented**: `ChatProviderError` messages never include the API key value. |
| F3-security | security | note | **Documented**: image `data:` URIs allowed by `img-src 'self' data:` is acceptable; left as-is. |
| F1-devils | devils_advocate | note | **Implemented**: chat-index empty-state shows a `<pre>` with copy-pasteable env-var setup. |
| Threat boundaries B1-B4 | threat_model | (no findings) | All four boundaries acknowledged; mitigations in implementation. |

## Files

### New

- `src/striatum/web/markdown.py` — `markdown-it-py` wrapper.
- `src/striatum/web/chat_provider.py` — `ChatProviderConfig` + outbound clients (Anthropic + OpenAI flavors). ~250 LoC.
- `src/striatum/web/templates/chat.html`, `chat_index.html`, `view_file.html`.
- `src/striatum/web/static/chat.js` — SSE listener + form submit JS island.
- `tests/test_web_chat.py` — 8 cases.
- `tests/test_web_view.py` — 8 cases.

### Modified

- `pyproject.toml` — adds `markdown-it-py>=4.0`.
- `src/striatum/service.py` — new `_dispatch_get` and `_dispatch_post` branches; new chat + view handlers; module-level helpers (`_is_safe_id`, `_append_jsonl`, etc.). `_render_artifact_view_page` extended for inline Markdown. Stale RFC 0013 startup warning dropped.
- `src/striatum/web/templates/base.html` — adds `Chat` to top nav.
- `src/striatum/web/templates/artifact_view.html` — renders `rendered_md`.
- `src/striatum/web/static/base.css` — appends ~90 LoC: chat layout, Markdown body, file-view code-pre.

### Docs

- `CHANGELOG.md` — `## 1.12.0 — 2026-05-09` section.
- `pyproject.toml` and `__init__.py` — bumped to `1.12.0`.
- `docs/DECISION_LOG.md` — D074.
- `docs/TODO.md` — F21.
- `docs/rfcs/0023-web-chat-and-browse.md` — status `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.

## CLI / API surface

New env vars on `striatum serve`:

```
STRIATUM_CHAT_API_BASE_URL=https://api.anthropic.com
STRIATUM_CHAT_API_KEY=sk-ant-...
STRIATUM_CHAT_MODEL=claude-opus-4-5
STRIATUM_CHAT_API_FLAVOR=anthropic_messages
```

New HTTP routes:

```
GET  /chat
GET  /chat/<id>
GET  /chat/<id>/events
POST /chat/new
POST /chat/<id>/send
POST /chat/<id>/stop
GET  /view/<repo-relative-path>
```

## Smoke

`striatum serve --web --allow-mutations` with the four env vars set; `/chat` shows the index with the new-chat button; `POST /chat/new` returns 303 → `/chat/<id>`; the transcript JSONL appears under `.striatum/scratch/chat-<id>/`.

## Test results

- `tests/test_web_chat.py`: 8 / 8 pass.
- `tests/test_web_view.py`: 8 / 8 pass.
- `make lint`: clean.
- `make typecheck`: clean (68 source files).
- Full `make test`: pending.

## Out of scope (V1.5 candidates)

- Tool use; supervised-CLI backend; full file-tree browser UI; "Add to chat" cross-tab island; syntax highlighting; OS-keyring; chat→artifact promotion; per-session provider override via UI.

## Acceptance summary

All three review postures (security, devils_advocate, threat_model) accepted at design time; the `required_review_postures: ["security", "devils_advocate", "threat_model"]` gate on the implement job satisfied. 16 new tests cover both API flavors, URL scheme validation, path traversal refusal, Markdown rendering, CSP preservation, and mutation gating.
