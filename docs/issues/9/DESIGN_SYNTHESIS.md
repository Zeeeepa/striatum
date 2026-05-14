---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["gh-9-spec", "gh-10-spec", "gh-11-spec", "dogfood-056-gemini-review", "roadmap-4.1", "todo", "prior-build-handoff", "prior-codex-review"]
---

author: designer-unknown-model-001

# Design Synthesis: GH #9-#11 Security Hardening

## Scope

This pass covers GH #9, GH #10, and GH #11 only. GH #12 clipboard behavior
and GH #13 graph-editor ghost fields stay out of scope.

The implementation must harden the RFC 0050 V2 web mutation surface without
changing Striatum's product boundary: no hosted service, telemetry, transcript
capture, external persistence, or new live-state authority outside the daemon
and existing service/CLI mutation path.

## Implementation Approach

### 1. Strict JSON media-type gate for `/v1/invoke`

Add or keep a shared helper in `src/striatum/service.py` that parses
`Content-Type` as an HTTP media type and accepts only `application/json` with
valid parameters, such as `application/json; charset=utf-8`.

The helper must reject before body parsing or command dispatch:

- missing `Content-Type`
- browser-simple content types such as `text/plain`, `application/x-www-form-urlencoded`, and `multipart/form-data`
- prefix, suffix, or substring tricks such as `application/jsonx` or `text/application/json`
- comma-joined duplicate header values
- malformed parameters such as `application/json; bogus`

Return `415` for non-JSON media types. Preserve `400` for malformed JSON,
invalid content length, and non-object JSON.

### 2. Fail-closed same-origin enforcement for browser-addressable POSTs

`src/striatum/service.py` should enforce same-origin checks before any
POST route handler or `/v1/invoke` command invocation. The gate must apply to
`/v1/invoke` whether or not `--web` is enabled, and to all web-enabled POST
routes when `web_enabled` is true.

Policy:

- A correctly authenticated Bearer-token request may bypass the Origin/Referer
  check because a cross-site browser page cannot supply the operator's token.
- For unauthenticated requests, the request `Host` must parse to one of the
  service's allowed loopback origins derived from the actual bind host and
  port. Do not accept a Host/Origin pair merely because they match each other;
  that leaves a DNS-rebinding-shaped gap.
- If `Origin` is present, it must parse cleanly and match the allowed origin
  set. `Origin: null`, malformed values, non-HTTP schemes, wrong hosts, and
  wrong ports return `403`.
- If `Origin` is absent, a same-origin `Referer` may satisfy the gate.
- If both `Origin` and `Referer` are absent on an unauthenticated request,
  return `403`. This intentionally tightens the older design language that
  allowed headerless non-browser calls. Such callers should use a Bearer token
  or the CLI instead of the browser-addressable mutation bridge.

### 3. Bind override-verdict to rendered job context

GH #10 is defense-in-depth on top of GH #9. The client must not treat mutable
DOM `data-*` values as the sole proof that the modal targets the page the
operator is viewing.

In `src/striatum/web/static/override_verdict.js`:

- parse `(run_id, job_id)` from `/run/<run_id>/job/<job_id>`
- compare that URL context to the modal host's rendered `data-run-id` and
  `data-job-id`
- disable/refuse the modal before `fetch("/v1/invoke", ...)` when the values
  are missing or mismatched
- include a `web_context` envelope alongside `argv`

In `src/striatum/web/templates/job_detail.html` and the job-detail render path:

- render a process-local HMAC token bound to `(run_id, job_id, session_id)`
- put the token on the override modal host as `data-context-token`

In `/v1/invoke` server handling:

- when `argv[0] == "override-verdict"` and the web UI path is enabled, require
  `web_context.kind == "override_verdict"`
- verify `web_context.job_id` and `web_context.session_id` match the parsed
  `--job-id` and `--session-id` argv values
- verify the process-local token
- return `403` on missing context, malformed fields, mismatched argv, or token
  failure

The token may rotate on service restart. Open pages that survive a restart can
receive `403` and require reload; that is acceptable.

### 4. Make `recovery auto-publish --dry-run` read-only by construction

In `src/striatum/cli/recovery.py`, split candidate discovery from mutation in
`auto_publish_stale_artifacts`.

The dry-run branch must not call mutation helpers such as `expire_leases`,
`ack_work`, `publish_artifact`, `complete_job`, `insert_event`, or
`maybe_complete_run`. To preserve preview parity with the live path, it may
classify active leases whose `expires_at` is already in the past as
`would_expire: true`, but it must not update the lease row.

Define the GH #11 invariant narrowly and usefully: dry-run must produce no
workflow-domain side effects. It must not publish artifacts, take or expire
leases, mutate jobs/runs/queue messages/verdicts, or emit workflow-domain
events. This does not prohibit separate metadata-only daemon or request audit
records when the command is invoked through an audited service path.

Do not add a broad `/v1/invoke` argv allowlist in this pass unless the change
stays small. The required closure is strict request validation plus a proven
read-only dry-run implementation.

## Exact Write Scope For Implementation

Expected source edits:

- `src/striatum/service.py`
- `src/striatum/web/templates/job_detail.html`
- `src/striatum/web/static/override_verdict.js`
- `src/striatum/cli/recovery.py`

Expected test edits/additions:

- `tests/test_invoke_csrf_refused.py`
- `tests/test_override_modal_context_validation.py`
- `tests/test_override_modal_payload.py`
- `tests/test_recovery_dry_run_no_side_effects.py`
- existing route tests that POST to the web service and now need same-origin
  headers or Bearer-token authentication

Avoid edits to workflow generation, RFC 0051 auto-finalize, clipboard behavior,
graph-editor field cleanup, or any dogfood #12/#13 surface.

## Tests To Add Or Update

1. `/v1/invoke` content-type tests:
   - reject `text/plain`, form content types, missing header, malformed
     parameters, comma-joined values, and non-exact JSON media types with `415`
   - accept `application/json` and `application/json; charset=utf-8`
   - still return `400` for malformed JSON with a valid content type

2. Origin/Referer tests:
   - reject cross-origin, `null`, malformed, wrong-port, and DNS-rebinding-style
     Host/Origin requests with `403`
   - reject unauthenticated headerless POSTs to `/v1/invoke`, including when
     the service is not running with `--web`
   - accept same-origin `Origin`
   - accept same-origin `Referer` when `Origin` is absent
   - accept authenticated Bearer-token clients without browser origin evidence

3. Override modal tests:
   - static JS checks for URL parsing, mismatch refusal before submit, and
     `web_context` in the POST body
   - template check for `data-context-token`
   - server tests that missing context, wrong kind, argv job/session mismatch,
     and forged token return `403`

4. Recovery dry-run tests:
   - seed a stale publishable artifact and call `recovery auto-publish --dry-run`
     through `/v1/invoke`
   - call the function directly with an active lease whose wall-clock
     `expires_at` is stale
   - snapshot workflow-domain tables before and after, including `leases`,
     `jobs`, `queue_messages`, `events`, `artifacts`, `verdicts`, and `runs`
   - assert snapshots are unchanged and preview output reports
     `dry_run: true`, expected `would_publish` rows, and `would_expire` where
     applicable
   - include one live-path sanity test proving non-dry-run still publishes

## Acceptance Criteria

- Cross-site simple-request payloads cannot execute `/v1/invoke`.
- Cross-origin, malformed-origin, DNS-rebinding-shaped, and unauthenticated
  headerless POSTs are rejected before CLI invocation or web route mutation.
- Same-origin UI POSTs and authenticated API clients continue to work.
- Override-verdict requests are bound to the rendered job page on both client
  and server.
- `recovery auto-publish --dry-run` is read-only for workflow-domain state while
  preserving room for metadata-only audit.
- Tests cover the exact edge cases raised by GH #9-#11 and the prior Codex
  security review.

## Known Security And Regression Risks

- Origin parsing is easy to get wrong for `localhost`, `127.0.0.1`, and IPv6
  loopback aliases. Keep the allowlist derived from the actual bind host and
  add explicit tests for hostile Host/Origin pairs.
- Tightening headerless POST handling may break older tests or scripts that
  used `/v1/invoke` as a local HTTP shell without Origin/Referer. Those callers
  should move to Bearer-token auth or use the CLI directly.
- Process-local context tokens invalidate open pages across service restart.
  This is acceptable, but the modal should surface the server error cleanly.
- Dry-run tests must not use setup helpers inside the assertion window that
  mutate the tables being compared.
- A future `/v1/invoke` allowlist or CSRF-token system may still be warranted,
  but it is not required to close GH #9-#11.

## Reviewer Verification

The downstream reviewer should verify:

- `Content-Type` is parsed exactly, not checked with substring or prefix logic.
- Same-origin enforcement runs before `/v1/invoke` dispatch and before
  web-enabled POST route handlers.
- Missing, malformed, `null`, cross-origin, and DNS-rebinding-shaped origin
  evidence fails closed unless a valid Bearer token is present.
- `override-verdict` cannot be invoked through `/v1/invoke` without a matching
  server-issued context token.
- The dry-run branch in `auto_publish_stale_artifacts` is read-only by
  inspection, not just by happy-path assertions.
- GH #12 and GH #13 were not folded into this security-hardening change.
