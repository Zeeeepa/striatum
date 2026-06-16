# RFC 0084: Interrogable agent-loop attestation + interrogation-log chat UI

Status: accepted (D141)
Date: 2026-05-26
Author: proposer-claude-opus-4-7-001
Context: builds on [`RFC 0082`](0082-interrogation-sessions.md) (interrogation
sessions, D138), [`RFC 0083`](0083-iterated-panel-review-with-interrogation.md)
(iterated interrogating panel, D139/D140), and D026/D080 (lane attestation).
Discovered and validated by the live run under
`docs/operator/workflows/interrogating-panel-2026-05-25/`.

## Summary

Two coupled changes, both surfaced by running the RFC 0083 pattern for real:

1. **Interrogable agent-loop attestation.** RFC 0082 requires an interrogation
   target to be `active` **and** attested (D026). Attestation (D080) is bound to
   the `supervise start` `--print` wrapper — but the wrapper spawns a fresh
   process per packet with **no preserved context**, while the headless MCP
   **agent-loop** lanes that *do* preserve the context interrogation exists to
   query are **never attested**. Result: the only sessions worth interrogating
   could not be interrogated. This widens `requireLiveTarget` to also accept a
   live session that has entered the RFC 0082 §5 `awaiting_interrogation`
   window, leaving artifact byline attestation (D080) untouched.

2. **Interrogation-log chat UI.** The Go web service renders a run's
   interrogation Q&A thread as a server-rendered chat transcript, backed by the
   existing interrogation read path. Designed by the 3-model design loop +
   interrogation; the interrogation directly shaped the implementation.

## 1. Interrogable agent-loop attestation

### Problem

`requireLiveTarget` (`go/pkg/mutations/interrogation.go`) failed
`target_unavailable` for any non-wrapper-attested target. Empirically, a live
`interrogable` agent-loop synthesizer in `awaiting_interrogation` returned
*"target session is not attested (no attached supervisor)"*. Attestation is a
`process_supervisors` row written by `supervise start`. Since RFC 0083 / D140
deprecate `--print` for interrogable runs, no attestation path remained for the
agent-loop targets interrogation is meant to query — a direct contradiction.

### Decision

`requireLiveTarget` accepts a target that is `active` **and** either:
- wrapper-attested (unchanged D026 path, drives byline provenance), **or**
- in the `awaiting_interrogation` window (it completed an `interrogable` job,
  evidenced by a `session.awaiting_interrogation` event).

Byline attestation (D080) is deliberately **not** granted by the second path:
`sessionLaneAttestation` is unchanged, so an agent-loop session still publishes
artifacts as `author: operator`. This widens only interrogation *liveness*, not
provenance — the forgery surface (GH #2/#5) is not enlarged.

### Why this is safe

Interrogation's attestation requirement exists to ensure the target is a real,
live, identified session — not to establish artifact authorship. The
`awaiting_interrogation` window already proves the target genuinely completed an
interrogable job and is held live for review; combined with `state = active`
this is an equivalent liveness guarantee. Regression test:
`TestInterrogationOpenAcceptsAwaitingInterrogationTarget` (and the existing
`TestInterrogationOpenRequiresLiveTarget` still rejects a bare active,
non-awaiting, unattested target).

### Validation

Proven end-to-end on the live run: a codex reviewer interrogated a claude
synthesizer (headless agent-loop, unattested) over 3 threat-model rounds; the
6-turn thread persisted and is exportable via
`striatum trajectory export --profile dialogue`. The interrogation caught a real
defect — a design proposal claimed `text/template` auto-escapes (it does not);
the synthesis correctly used `html/template`.

## 2. Interrogation-log chat UI

### Design (from the 3-model synthesis + interrogation)

- **Server-side `html/template`**, not client JS and not `text/template`:
  context-aware auto-escaping neutralizes stored XSS in attacker-influenced turn
  bodies; Markdown rendering is deferred so links/code blocks render inert.
- **Reuse the existing read path** (`reads.HandleInterrogationShow` /
  `interrogation.list`) — no new migration, no data-model change, only curated
  turn fields exposed (D028).
- **New GET routes** on the web service: `/v1/runs/{runID}/interrogations`
  (list) and `/v1/runs/{runID}/interrogations/{id}` (chat page), the latter
  enforcing **run-ownership** (404 when `interrogation.run_id != runID`) to
  close the IDOR path the interrogation flagged.

### Implementation

`go/pkg/webassets/templates/interrogation.html` + `RenderInterrogation`
(`html/template`); the routes + ownership check in
`go/pkg/webservice/service.go`; escaping tests in `go/pkg/webassets/` and
`go/pkg/webservice/` pinning that `<script>` / `<img onerror=…>` bodies render
escaped.

## Drawbacks / follow-ups

- The agent-loop still has no first-class *byline* attestation; a future RFC
  could let an agent-loop self-attest its session via its owned PTY/pid to also
  earn lane/model bylines (out of scope here).
- The chat UI ships the minimal server-rendered slice; Markdown, SSE live
  updates, export, and stricter multi-session tenancy are deferred (named in the
  build review).
- The MCP `review.verdict` / `artifact.publish` tools returned opaque errors in
  the live run (lanes fell back to CLI); worth a separate hardening pass.
- **The Go web service is not mounted in the running daemon.**
  `newWebServiceHandler` has no non-test caller — `striatumd`'s HTTP listener
  serves only the MCP handler, so the entire `/v1/...` surface (including this
  chat route) is built and httptest-verified but not reachable from the live
  daemon. The chat HTML was verified by rendering the real persisted thread
  through the production `RenderInterrogation` path
  (`artifacts/interrogation-chat-rendered.html`). Mounting the web handler on
  the daemon listener (multiplexed with `/mcp`, with the web service's own auth)
  is a prerequisite follow-up to view interrogations in a live browser; it is
  broader than F36's bundle-embedding gap and deserves its own change.

## Decisions

- **D141** — accept the interrogable agent-loop attestation widening.
- **D142** — accept the interrogation-log chat UI (server-rendered, html/template).
