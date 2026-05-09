# Design review (security posture): RFC 0023 V1

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept_with_findings

Security posture: read the artifact looking for security weaknesses; verdict acceptance means we actively looked and found nothing acceptance-blocking.

## Verdict

**accept_with_findings** — the V1 design is implementable safely; three findings to fold into the build (one acceptance-blocking, two notes).

## Sweep

### F1 (acceptance-blocking) — SSRF surface via STRIATUM_CHAT_API_BASE_URL

The synthesis says "operator picks the endpoint" but doesn't address: an operator who pastes a URL like `http://127.0.0.1:8080/admin` could trick the chat endpoint into making requests to internal services on the same host. The risk is low because (a) the operator is the attacker (b) the request is a POST with an OpenAI/Anthropic JSON body, not a generic SSRF probe — but a malicious co-tenant on a shared dev box could potentially trick striatum into echoing internal-service responses into the chat stream.

**Recommendation**: V1 logs the resolved base URL at startup so operators see what's actually configured. V1 also validates the URL scheme is `https` OR `http://localhost*` / `http://127.*` (allow loopback for local model servers like Ollama; deny other `http://` for non-loopback).

### F2 (note) — API key in URL params

If the operator misconfigures and puts the API key in `STRIATUM_CHAT_API_BASE_URL` as a query string instead of using `STRIATUM_CHAT_API_KEY`, striatum would send it but also potentially log it on errors. Defense: when an error response includes the request URL, redact `?api_key=...` patterns. Cheap.

### F3 (note) — Markdown HTML disabled but image src is still attacker-controlled

`html: False` blocks `<script>` etc. but Markdown's image syntax (`![](url)`) still produces `<img src="...">`. With CSP `img-src 'self' data:`, an attacker-supplied `https://evil.com/track.gif` in a model response would be blocked at the browser layer — good. But `data:` is allowed; a model could embed a tracking pixel via `data:image/png;base64,...` (no exfil though, since data: doesn't reach the network). Acceptable; leave as-is.

## Decision

Accept with F1 (URL scheme validation) folded into implementation.
