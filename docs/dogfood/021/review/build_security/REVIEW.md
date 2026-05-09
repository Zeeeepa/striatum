# Build review (security): RFC 0023 V1

author: reviewer-claude-opus-006
date: 2026-05-09
verdict: accept

Security posture on the V1 build.

## Verdict

**accept** — all three design-review security findings (F1, F2, F3) are addressed. No new acceptance-blocking issues found.

## Verification

### F1 (acceptance-blocking) — URL scheme validation

`validate_base_url` in `chat_provider.py` refuses `http://` URLs unless the host is `localhost` / `127.*` / `::1`. Verified by `test_chat_provider_url_scheme_validation_rejects_remote_http`. ✓

### F2 (note) — API key in error responses

`ChatProviderError` messages cap the upstream response body at 500 bytes. The handler in `service.py` writes a `system` role entry to the transcript with the error message, then returns 502 to the browser; neither path includes the API key value. The key only appears in the outbound HTTP headers. ✓

### F3 (note) — Markdown image src

The CSP policy is `img-src 'self' data:`. `markdown-it-py` with `html: False` doesn't allow raw `<img>` tags from raw HTML in the source, so the only way an image appears is via Markdown's `![](url)` syntax — which `markdown-it-py` URL-normalizes (rejects `javascript:` and other unsafe schemes by default). The CSP layer blocks remote `https://evil.com/track.gif` because `img-src 'self'` is set; only `'self'` and `data:` work. The threat-model surface is acceptable as documented. ✓

## Sweep matrix

| Concern | Mitigation | Verified |
| --- | --- | --- |
| Path traversal on `/view/<path>` | `relative_to(repo_root)` after `resolve()`; null-byte / `..` / leading-slash refused | `test_view_path_traversal_refused` |
| `.git/` and `.striatum/` exposure | First-path-component check returns 404 | `test_view_dotgit_hidden`, `test_view_dotstriatum_hidden` |
| Path traversal on artifact `.md` rendering | `relative_to(repo_root)` after resolving the artifact's `repo_path` | Source review of `_render_artifact_view_page` |
| Binary file masquerading | Extension blacklist + null-byte heuristic in first 1024 bytes | `test_view_binary_metadata_panel` |
| API key in scratch JSONL | Transcript records role + content + created_at only; `_append_jsonl` doesn't see the key | Source review of `_handle_chat_send` |
| API key in HTML responses | Error responses surface the upstream body excerpt; headers stripped | Source review of `_stream_anthropic` / `_stream_openai` exception path |
| Form-field injection | `_read_form_body` with `parse_qs(keep_blank_values=True)` returns lists; no shell interpolation | Source review |
| SSE backpressure | `time.sleep(SSE_POLL_INTERVAL_SECONDS)` between polls; 10-min cap | Source review of `_stream_chat_events` |
| Multipart parsing | `_parse_simple_multipart` is text-only; binary uploads aren't a V1 surface | Source review |
| CSP regression | `_send_html` uses byte-identical CSP from RFC 0022 V1 | `test_chat_csp_unchanged` |

## Counterargument: "What if the provider returns malicious Markdown?"

The chat response goes into the transcript JSONL and is rendered server-side via `striatum.web.markdown.render` with `html: False`. Raw HTML in the response is escaped, not emitted. The CSP layer blocks any inline `<script>` that somehow slipped through (defense in depth). **Accept.**

## Counterargument: "What if the operator misconfigures with the API key in the URL?"

If `STRIATUM_CHAT_API_BASE_URL` contains `?api_key=sk-...`, striatum will preserve the URL and use the env-var key as the auth header. The base URL is logged at startup (visible in the service envelope). An operator who pastes the key into the URL would see it in their shell history; that's an operator-side issue and out of scope for V1. V1.5 may add a startup-time warning if the URL contains a query string with `key|token|secret` substrings. **Acceptable.**

## Counterargument: "Same-host SSRF via STRIATUM_CHAT_API_BASE_URL=http://localhost:8080/admin"

Per F1, `http://` is allowed for loopback hosts. An operator who points striatum at a localhost service that exposes admin endpoints could trick striatum into POSTing chat-completion-shaped JSON to it. The "attack" requires the operator's own configuration; it's not an external attacker vector. The error message would surface the response body excerpt back to the chat — an admin-API leak path, but the operator is the attacker. **Acceptable for V1; document in V1.5 as "your loopback chat endpoint should be a chat endpoint, not your admin panel."**

## Decision

Accept. All three design-review security findings addressed; no new blockers; documentation captures the residual risks.
