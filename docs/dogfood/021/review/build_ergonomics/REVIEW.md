# Build review (ergonomics_dx): RFC 0023 V1

author: reviewer-claude-opus-005
date: 2026-05-09
verdict: accept

Ergonomics-DX posture: evaluate first-time-operator experience.

## Verdict

**accept** — the build's first-time-operator UX is honest about what works without configuration.

## Sweep

### Chat-unconfigured state

`/chat` with no env vars renders the empty-state with the four env-var lines as a copy-pasteable `<pre>`. The text mentions both flavors (`anthropic_messages`, `openai_chat`) and the loopback-http-only constraint. Good first-time-operator affordance.

### Chat-configured state

Once env vars are set, `/chat` renders the index with a "+ New Chat" button (gated on `--allow-mutations`). Sessions list with started-at timestamp + message count. `/chat/<id>` page has the message stream + textarea + send button. Enter-to-send + Shift-Enter-for-newline matches operator expectations from terminal tools.

### Dark mode

Inherits from the existing RFC 0022 V1 CSS custom properties. Chat messages use `--bg-elevated` and `--bg-overlay`; readable in both modes. No new palette work needed.

### Send button affordance

The button is a clear blue `primary-button` style with hover/disabled states. Disabled while a response is streaming (set via JS island). Operators see immediate feedback.

### SSE reconnection

The chat.js island opens `EventSource` on page load; on disconnect, the status span shows "Disconnected; reload the page to retry." V1.5 may add automatic reconnection; V1 just tells the operator to refresh.

### File view

`/view/<path>` is a one-shot render — no breadcrumb navigation across sibling files. V1 by design (full file-tree UI is V1.5). Operators paste paths in the URL bar; for a more polished workflow, V1.5 will add the file-tree.

### Errors

Provider errors (4xx/5xx from upstream) become `system` role entries in the transcript with the upstream body excerpt. Network errors visible. API key never echoed.

### Mobile

The CSS uses `flex` layouts with viewport-meta `width=device-width`. Not specifically tested on mobile, but no fixed-pixel constraints that would break.

## Findings

None blocking. Two notes for V1.5:

- **Note 1**: The chat index empty-state explains *what* env vars to set but not *which providers* are likely to give the operator an API key. V1.5 could add a small "Where do I get a key?" section linking out to known providers.
- **Note 2**: The file-view page is reachable only by typing the URL. V1.5's file-tree UI (`/browse/`) closes this gap.

## Decision

Accept. First-time UX is honest; affordances match the V1 scope.
