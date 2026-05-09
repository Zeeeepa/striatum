# Design review (threat_model): RFC 0023 V1

author: reviewer-claude-opus-003
date: 2026-05-09
verdict: accept

Threat-modeling posture: enumerate trust boundaries the artifact introduces; verdict acceptance means each is acknowledged or mitigated.

## Verdict

**accept** — boundaries are explicitly enumerated in the RFC + synthesis; no acceptance-blocking gaps. Two notes for completeness in BUILD_HANDOFF.

## Trust boundaries

### B1: striatum → provider (HTTPS + API key)

- **In scope**: outbound POST to operator-configured base URL. API key in header.
- **Out of scope**: striatum doesn't validate the provider's certificate beyond what stdlib `http.client` does (which uses system trust store). Acceptable.
- **What gets through**: the operator's full conversation history (last N messages, default 50). The provider sees what the operator types AND any prior assistant responses. Standard for any chat client; not a striatum-specific concern.

### B2: browser → striatum (loopback)

- **In scope**: HTTP to 127.0.0.1:<port>. Same-origin. CSP-locked.
- **Out of scope**: anyone with localhost access on the same machine can hit striatum's HTTP port. This is the existing RFC 0013 model; chat doesn't make it worse. A multi-user dev box should rely on OS-level user separation (different `--repo` per user; different ports).

### B3: model output → DOM (Markdown sanitization)

- **In scope**: model-produced Markdown rendered as HTML in the chat page.
- **Mitigation**: `html: False` on markdown-it-py prevents raw HTML. CSP `script-src 'self'` blocks any inline JS that slipped through (defense in depth).
- **Edge case**: Markdown link with `javascript:` URL. markdown-it-py 4.x normalizes URL schemes by default (rejects `javascript:`). Build review should verify this on the installed version.

### B4: scratch JSONL → git tree

- **In scope**: chat transcripts live in `.striatum/scratch/chat-<id>/transcript.jsonl`.
- **Mitigation**: `.striatum/` is gitignored on `striatum init`. Operators who don't init won't have transcripts in git anyway (the directory won't exist).
- **Risk**: an operator who manually `git add .striatum/scratch/...` overrides gitignore. That's an operator footgun, not a striatum design issue. AGENTS.md already documents `.striatum/` as not-for-commit.

### Note 1 — `connect-src 'self'` and the SSE stream

The SSE stream is same-origin (`/chat/<id>/events` on striatum's own host); CSP `connect-src 'self'` covers it. The outbound HTTP to the provider is server-to-server, not browser-to-provider, so no CSP relaxation needed. Confirm in build review.

### Note 2 — Concurrent chat sessions

Multiple chat sessions racing to the same scratch directory: each has a unique `chat_<id>`, so directory contention is impossible. Multiple SSE listeners on one session: synthesis caps at 1 (additional get 429). Acceptable.

## Decision

Accept. Boundaries are well-enumerated; mitigations match the threat model.
