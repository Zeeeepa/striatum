---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0013 step 7 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-09
Verdict: `accept`

## Pinned contracts (verified)

- **`/v1/health` extension**: `allow_mutations: bool` present on
  every health response. Verified both flag states by spawning
  the service with and without `--allow-mutations` (tests).
- **Mutation buttons**: all five render in their declared
  views. Per-blocker continue/cancel buttons appear inline next
  to open blockers. Verdict button appears on review jobs in
  `running` state. Decision button appears on the run-detail
  view. Requeue-stale appears on stale-lease review-only jobs.
- **`POST /v1/invoke` with the documented argv shapes**: the
  SPA POSTs the argv literally; the runner parses it through
  the same dispatch path as the CLI. No client-side argv
  rewriting.
- **Confirmation modal**: shows the literal argv via
  `<pre class="modal-argv">`, escaped through `escapeHTML`
  before insertion (no `innerHTML` for user-supplied
  strings).
- **Destructive variant**: cancel-job and reject-verdict get a
  red confirm button (CSS `.modal-confirm.destructive`).
- **CSP unchanged**: no inline event handlers, no `eval`, no
  external URLs. Verified by `test_spa_app_js_no_external_urls`
  + the `addEventListener`-only handler attachment.
- **Result envelope rendering**: success and error paths both
  render an inline banner (`.result-banner.success` /
  `.result-banner.error`); error includes the exit code and
  message verbatim.
- **View refresh on success**: each `runMutation` invocation
  declares an `onSuccess` callback that re-fetches the
  underlying view (run detail or job detail). The fresh data
  shows the new state immediately.
- **Tests**: 13 web UI tests pass (8 baseline + 5 new). Full
  suite 285 passing (was 280; +5).
- **Lint + typecheck**: clean.

## Notes

- The HTTP 405 status for mutation refusal (vs. exit code 8 in
  the CLI) is correct for HTTP semantics — the gate refusal
  *is* a method-not-allowed condition for that argv. Test
  asserts both the status and the message substring.
- Verdict-button UX uses `prompt()` for session/lease ids
  because the why-endpoint doesn't yet expose them on the
  job-detail payload. Implementer flagged this in the handoff
  as a follow-up; not a blocker for V1+step 7. The button
  works end-to-end with operator-pasted lease/session.
- Decision-path validation is server-side via the publisher
  (refuses inside `.striatum/`, outside repo). Client-side
  prompt collects the path; server returns the canonical
  refusal. Defence-in-depth.
- The mutation buttons reuse the existing CLI verbs verbatim;
  no new server logic was added. Matches the design
  synthesis's "thin SPA shell" framing.
- Live service at `proximal:8088` was restarted with
  `--allow-mutations` so the buttons render for the operator
  testing this PR.

## Decision

`accept`. Step 7 closes RFC 0013's deferred mutation slice
cleanly. The runner-side gate stays the authority; the SPA
adds click-driven access to the same mutations the CLI
already supports. No regressions on the 8 V1 read paths.
