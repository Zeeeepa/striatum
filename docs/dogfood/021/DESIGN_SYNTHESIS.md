# Design synthesis: RFC 0023 V1 (web chat + browse)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1 ships in three steps (compact-V1 per research):
1. Chat surface (provider-neutral, both API flavors).
2. Minimum file-view endpoint (`/view/<path>`).
3. Markdown rendering on artifact pages (closes RFC 0022 V1.5).

Deferred to V1.5: full file-tree UI, "add to chat" island,
chat-index UI beyond a basic listing, supervised-CLI backend
option (RFC 0023 § Q8).

## 1. Chat provider configuration

Four env vars at `striatum serve` startup:

```
STRIATUM_CHAT_API_BASE_URL
STRIATUM_CHAT_API_KEY
STRIATUM_CHAT_MODEL
STRIATUM_CHAT_API_FLAVOR  # anthropic_messages | openai_chat
```

Validation at startup:
- All four required to enable chat.
- `STRIATUM_CHAT_API_FLAVOR` must be one of two values; unknown values exit code 8.
- Missing any → chat disabled silently (chat-index page renders empty-state with copy-pasteable env-var setup; doctor reports `chat_configured: false`).

API key never written to SQLite, never logged, never echoed in error messages. Errors say "provider returned 401" not "API key sk-... rejected."

## 2. Chat lifecycle

Routes:

```
POST /chat/new                  → create session, redirect to /chat/<id>
GET  /chat                      → chat_index.html (sessions list)
GET  /chat/<id>                 → chat.html (history + input)
POST /chat/<id>/send            → append message + outbound to provider
GET  /chat/<id>/events          → SSE stream of new transcript events
POST /chat/<id>/stop            → mark session closed
```

Per-session state lives in `.striatum/scratch/chat-<id>/transcript.jsonl` (gitignored under `.striatum/`). One JSON-line per message (role: user/assistant/system, content, created_at). Page reload replays the last 200 lines (full transcript on disk; cap is initial-render only).

Session id format: `chat_<8-hex-digits>` (collision-resistant enough for one operator on one box).

`POST /chat/<id>/send`:
1. Validate the chat-provider config.
2. Append the user message to transcript JSONL.
3. POST to the provider with the full transcript (last N messages, configurable N defaulting to 50 for V1).
4. Stream the response chunks both to the SSE listener AND append assistant message progress to JSONL (so reload mid-stream picks up partial).
5. On completion: append final assistant message; close the SSE event with `event: complete`.

`GET /chat/<id>/events`:
- SSE stream that polls the transcript JSONL for new lines (250ms poll, like existing /events).
- Per-session concurrent listener cap: 1 (additional listeners get 429).

## 3. API flavors

Two flavor implementations in `striatum.web.chat_provider`:

### `AnthropicMessagesClient`

POST `{base_url}/v1/messages`:
```json
{
  "model": "<STRIATUM_CHAT_MODEL>",
  "messages": [{"role": "user|assistant", "content": "..."}],
  "stream": true,
  "max_tokens": 4096
}
```
Headers:
```
x-api-key: <STRIATUM_CHAT_API_KEY>
anthropic-version: 2023-06-01
content-type: application/json
```
Response: SSE events of types `message_start`, `content_block_start`, `content_block_delta` (with `delta.text`), `content_block_stop`, `message_delta`, `message_stop`. Parser collects `delta.text` chunks into the assistant message.

### `OpenAIChatClient`

POST `{base_url}/v1/chat/completions`:
```json
{
  "model": "<STRIATUM_CHAT_MODEL>",
  "messages": [{"role": "user|assistant|system", "content": "..."}],
  "stream": true
}
```
Headers:
```
Authorization: Bearer <STRIATUM_CHAT_API_KEY>
content-type: application/json
```
Response: SSE `data: {...}\n\n` lines with `choices[0].delta.content`. Final line: `data: [DONE]`.

Both clients return an iterator yielding `(text_chunk, is_final)` tuples; the route handler appends each to the transcript JSONL and pushes to the SSE listener.

## 4. File-view endpoint

`GET /view/<path>`:
- Path safety: `..`, leading `/`, null byte → 400.
- `<repo>/<path>` resolved to absolute; refused if outside `<repo>` (symlink check).
- `.git/` and `.striatum/` → 404 (not exposed; V1.5 may add `?show=hidden`).
- If directory: 404 (V1; V1.5 adds full tree).
- If `.md`: render via `striatum.web.markdown.render(body)` and embed in `view_file.html` with breadcrumb header.
- Else if text (first 1024 bytes have no null byte AND extension not in binary blacklist): `<pre><code>...</code></pre>` with HTML escaping.
- Else: `view_file.html` with metadata panel + raw-bytes link.

Binary blacklist: `.png .jpg .jpeg .gif .pdf .zip .tar .gz .ico .woff .woff2 .ttf .eot .mp3 .mp4 .mov`.

## 5. Markdown rendering

`striatum.web.markdown.render(source: str) -> str`:
```python
from markdown_it import MarkdownIt

_md = (
    MarkdownIt("commonmark", {"breaks": False, "html": False})
    .enable("table")
    .enable("strikethrough")
)

def render(source: str) -> str:
    return _md.render(source)
```

Raw HTML disabled at parser level (`html: False`); no separate sanitizer needed for V1. The renderer wraps tables in `<div class="md-table-wrap">` for horizontal-scroll on small screens (CSS-side).

Updated `artifact_view.html` calls `striatum.web.markdown.render` for `.md` artifacts (extension check on the artifact's `repo_path`); other artifacts unchanged.

## 6. CSP impact

Unchanged: `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'`.

The chat client's outbound HTTPS is server-to-server (striatum's `service.py` to the provider); from the browser's perspective everything is same-origin. `connect-src 'self'` covers the SSE feed.

## 7. Test plan

`tests/test_web_chat.py`:
1. `test_chat_disabled_without_env_vars` — service starts but `/chat` index renders the empty-state.
2. `test_post_chat_new_creates_session` — with env vars set, POST creates a JSONL file under scratch.
3. `test_chat_send_anthropic_flavor_request_shape` — POST `/chat/<id>/send` against a mock HTTP server; assert request body + headers.
4. `test_chat_send_openai_flavor_request_shape` — same against `openai_chat` flavor.
5. `test_chat_send_streams_response_via_sse` — mock server streams SSE; client receives events.
6. `test_chat_transcript_jsonl_round_trip` — send/receive cycle persists to JSONL.
7. `test_chat_provider_error_surfaces_in_transcript` — 5xx from provider becomes a `system` role entry.
8. `test_chat_unknown_flavor_exits_8` — `STRIATUM_CHAT_API_FLAVOR=foo` refuses.
9. `test_csp_unchanged_on_chat_pages` — chat pages have byte-identical CSP.
10. `test_api_key_never_in_response_body` — error responses don't echo the key.

`tests/test_web_view.py`:
1. `test_view_md_renders_html` — `.md` file renders via Markdown.
2. `test_view_text_renders_pre` — `.py` file renders as `<pre>`.
3. `test_view_path_traversal_400` — `/view/../../etc/passwd` → 400.
4. `test_view_dotgit_hidden` — `/view/.git/HEAD` → 404.
5. `test_view_dotstriatum_hidden` — `/view/.striatum/state.sqlite3` → 404.
6. `test_view_binary_metadata_panel` — `.png` file renders metadata, not bytes.
7. `test_view_directory_404` — directory paths → 404 in V1.

`tests/test_web_ui_redesign.py` extension:
1. `test_artifact_view_renders_md_inline` — `.md` artifact body rendered inline.
2. `test_artifact_view_non_md_unchanged` — `.json` artifact still shows metadata + raw link.

## 8. Documentation surface

- `docs/SPEC.md` § "Local Web UI" — add chat + view subsections.
- `docs/UBIQUITOUS_LANGUAGE.md` — `chat session`, `chat provider config`, `markdown render policy`, `file view` (browse path is V1.5).
- `docs/DECISION_LOG.md` — D074 (RFC 0023 V1 acceptance + outbound HTTP carve-out).
- `docs/TODO.md` — F21.
- `docs/rfcs/0023-web-chat-and-browse.md` — status `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.12.0 — 2026-05-09`.
- `pyproject.toml` + `__init__.py` — bump to 1.12.0; add `markdown-it-py>=4.0` dep.

## 9. Zero-regression contract

Without chat env vars: no behavior change. Without visiting `/chat` or `/view`: no behavior change. JSON API (`/v1/*`), SSE feed (`/events`), existing pages unchanged. CSP byte-identical.

## 10. Out of scope (V1.5)

Per RFC 0023 § "Implementation Path" V1.5 candidates plus this synthesis's deferrals:
- Full file-tree browser UI.
- "Add to chat" cross-tab island.
- Chat-index list rendering (V1 ships a basic empty-state-or-list).
- Tool use (chat calls striatum verbs).
- Supervised-CLI backend option.
- Per-session provider override via UI.
- Chat→artifact promotion mutation.
- File-browser pagination.
- Syntax highlighting.
- OS-keyring API-key storage.
