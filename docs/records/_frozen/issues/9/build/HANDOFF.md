---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
upstream_inputs:
  - "gh-9-spec"
  - "gh-10-spec"
  - "gh-11-spec"
  - "design-synthesis"
  - "codex-build-review"
---

author: implementer-unknown-model-001

# Handoff — GH #9-#11 Security Hardening

## Summary

Closed GH #9, #10, and #11 in a single security-hardening pass per the
`docs/issues/9/DESIGN_SYNTHESIS.md` plan and the codex build review's
required revisions. Scope respected: `src/striatum/`, `tests/`,
`docs/issues/9/build/`. GH #12 and #13 were explicitly held out.

## Codex Review Findings Closed

- **Finding 1 (fail-closed Origin/Referer):** Unauthenticated `/v1/invoke`
  and web-enabled POST routes now reject absent, `null`, malformed, and
  cross-origin Origin/Referer evidence with `403`. The allowed origin set
  is derived from the actual bind host and port via
  `allowed_origins_for_bind`, not from a substring match.
- **Finding 2 (exact Content-Type parse):** `is_json_content_type`
  performs a strict parameterized media-type parse — no substring,
  prefix, or suffix matching. Comma-joined header values, malformed
  parameters, and unbracketed quoted values are rejected. Tests cover
  `text/plain`, form types, `application/jsonx`, `text/application/json`,
  `application/json; bogus`, comma-joined duplicates, and
  `application/json; charset=utf-8` accept.
- **Finding 3 (dry-run vs. audit):** The `recovery auto-publish --dry-run`
  branch is read-only by inspection for workflow-domain state
  (`leases`, `jobs`, `queue_messages`, `events`, `artifacts`, `verdicts`,
  `runs`); request-level audit telemetry is unaffected. The dry-run
  regression test asserts byte-for-byte equivalence of those
  workflow-domain tables before/after, while leaving room for
  metadata-only daemon or request audit records elsewhere.

## Changes

### GH #9 — `/v1/invoke` CSRF mitigations (`src/striatum/service.py`)

1. **Strict JSON Content-Type.** Module helper `is_json_content_type`
   exact-matches `application/json` after splitting on `;` and lowercasing
   the bare media type. Each parameter is validated: name must be an HTTP
   token, value must be a valid token or quoted-string. Comma-joined
   duplicate header values and CRLF injection attempts are refused. Both
   `_read_json_body` and `_read_json_body_strict` use it and reject
   anything else with `415`. `application/json; charset=utf-8` continues
   to pass.
2. **Same-origin enforcement (fail-closed).** Handler method
   `_verify_same_origin_mutation()` runs at the top of `_dispatch_post`
   when `state.origin_check_enabled` is set (TCP listener). Predicate
   `_requires_same_origin` covers `/v1/invoke` whether or not `--web` is
   enabled, plus every web-enabled POST route. Policy:
   - Authenticated Bearer-token clients are exempt (non-browser API
     callers cannot be impersonated by a website).
   - Request `Host` must parse to one of the service's allowed loopback
     origins derived from the actual bound listener (`parse_host_origin`
     + `allowed_origins_for_bind`); this defends against DNS-rebinding
     shapes where Host matches Origin but neither matches the listener.
   - When `Origin` is present it must parse cleanly and match the
     allowlist; `null`, malformed, non-`http`, wrong host, and wrong
     port return `403`.
   - When `Origin` is absent, same-origin `Referer` satisfies the gate.
   - When both `Origin` and `Referer` are absent on an unauthenticated
     request, return `403` (`"Origin or Referer required"`). This is the
     fail-closed tightening the design synthesis required; non-browser
     callers should use a Bearer token or the CLI/Unix-socket path.

### GH #10 — Override modal context binding

1. **Process-local HMAC secret** added to `ServiceState`
   (`web_context_secret`, 32 bytes from `secrets.token_bytes`). Rotates
   on every service restart; tokens become invalid after restart, which
   is acceptable because the page must be reloaded.
2. **Token minting** in `_render_job_detail_page` via module helpers
   `make_web_context_token` / `verify_web_context_token` (blake2b keyed
   digest, 16-byte hex). The token binds `(run_id, job_id, session_id)`
   and is passed through to the template as `override_context_token`
   and `override_session_id`.
3. **Template** (`src/striatum/web/templates/job_detail.html`) emits
   `data-context-token` on the override modal host.
4. **Client** (`src/striatum/web/static/override_verdict.js`):
   - `parsePageContext()` extracts `run_id` / `job_id` from the URL
     `/run/<run_id>/job/<job_id>`.
   - The modal refuses to enable / fire when the DOM `data-run-id` or
     `data-job-id` does not match the URL (`contextMismatch` guard is
     checked before `submitButton.disabled = true;`).
   - `buildWebContext()` builds the envelope including `token`.
   - POST body wraps both `argv` and `web_context`.
5. **Server-side validation** in `_dispatch_post`: when `web_enabled` is
   true and `argv[0] == "override-verdict"`, new method
   `_verify_override_verdict_context()` requires the envelope, checks
   `kind == "override_verdict"`, validates argv `--job-id` and
   `--session-id` match the context fields, and HMAC-verifies the token.
   Any mismatch returns `403`.

### GH #11 — `recovery auto-publish --dry-run` strict read-only

In `src/striatum/cli/recovery.py::auto_publish_stale_artifacts`:

1. `expire_leases` is only called on the live path. Dry-run was
   previously calling it before branching on `dry_run`, mutating the
   leases table during a supposedly read-only preview.
2. The dry-run candidate query matches leases that are *either* already
   expired *or* still active but past their wall-clock `expires_at`,
   preserving preview parity with the live path without mutating any row.
3. `maybe_complete_run` (which mutates the runs row and emits a
   `run.completed` event) is gated behind the live path.
4. The dry-run branch hits `continue` before any of `ack_work`,
   `publish_artifact`, `complete_job`, or `insert_event` are reachable.
5. Dry-run preview rows carry `would_expire: true|false` so the UI can
   explain why a row showed up without us having touched it.

`auto_publish_stale_artifacts(..., dry_run=True)` is now read-only by
inspection for workflow-domain state. Request/audit telemetry written
outside this function (e.g. the service's own request log) is
intentionally left untouched.

## Tests

Added:

- `tests/test_invoke_csrf_refused.py` — 17 cases covering
  `text/plain`, form, `application/jsonx`, `text/application/json`,
  `application/json; bogus`, comma-joined duplicates, missing
  Content-Type → `415`; `application/json` and
  `application/json; charset=utf-8` → accept; malformed JSON with valid
  Content-Type → `400`; cross-origin Origin → `403`; same-origin
  Origin → accept; cross-origin Referer → `403`; same-origin Referer →
  accept; `Origin: null` → `403`; missing Origin and Referer → `403`
  (fail-closed); Bearer-token client bypasses Origin check; DNS-rebinding
  Host/Origin mismatch → `403`; GET still works.
- `tests/test_override_modal_context_validation.py` — 8 cases. Static
  asserts on `parsePageContext`, `buildWebContext`, mismatch refusal
  ordering, template `data-context-token` emission; server-side asserts
  that override-verdict without `web_context`, with mismatched argv
  `job_id`, with a forged token, or with the wrong `kind` returns
  `403`. Tests now supply a same-origin `Origin` header so the fail-
  closed CSRF gate doesn't pre-empt the context-validation assertion.
- `tests/test_recovery_dry_run_no_side_effects.py` — 3 cases.
  Snapshot `leases`, `jobs`, `queue_messages`, `events`, `artifacts`,
  `verdicts`, `runs` before/after a dry-run sweep posted through
  `/v1/invoke`; assert byte-for-byte equivalence. Repeat with a direct
  call against a lease whose wall-clock `expires_at` is in the past but
  whose `state` is still `'active'` (the pre-fix bug path) — assert no
  mutations and that `would_expire: true` is surfaced. Sanity test that
  the live (non-dry-run) path still publishes.

Updated:

- `tests/test_override_modal_payload.py` — adjusted to the new
  `JSON.stringify({ argv: argv, web_context: webContext })` body shape
  and the new `postInvoke(..., buildWebContext(context))` call site.
- `tests/test_override_modal_context_validation.py` — now passes a
  same-origin `Origin` header on each server test so it exercises the
  override-verdict context gate rather than the Origin/Referer gate.

## Tests run

- `tests/test_invoke_csrf_refused.py` — 17 passed.
- `tests/test_override_modal_context_validation.py` — 8 passed.
- `tests/test_recovery_dry_run_no_side_effects.py` — 3 passed.
- `tests/test_override_modal_payload.py` — 4 passed.
- `tests/test_recovery_panel_dry_run.py` — 2 passed.
- `tests/test_recovery_auto.py` — 21 passed.
- `tests/test_recovery_extended.py` — 6 passed.
- `tests/test_override_rationale_regression.py` — 4 passed.
- `tests/test_service.py` — 22 passed.
- `tests/test_web_chat.py` — 10 passed.
- `tests/test_web_workflow_edit.py` — 20 passed.
- `tests/test_web_workflow_run.py` — 7 passed.
- `tests/test_copy_on_click.py` — 3 passed.
- `make lint` — ruff clean.
- `make typecheck` — mypy clean (216 source files).

## Tests not run

- `make smoke` — the smoke target is not part of the implementer
  contract for this packet (see Makefile); verifier should run it if
  the gate requires it.
- `tests/test_web_ui.py::test_static_assets_no_external_urls` — was
  already failing on `main` before this branch. The failure flags an
  `http://www.w3.org/1999/xhtml` literal inside the bundled
  `build/island-workflow-graph-editor.js`. Out of scope for GH #9-#11.
- Full `pytest tests/` was not re-run in this revision pass; the focused
  suites above and the broader related suites (95 tests) all pass with
  the change in place.

## Residual risk

- **Tokens are process-local.** A `serve` restart invalidates all
  outstanding override-modal tokens, so an open page that has not been
  refreshed after restart will see `403` on submit. This is the trade
  declared in the design synthesis; the modal surfaces the server's
  error message and the operator reloads.
- **Fail-closed CSRF gate may surprise direct-HTTP clients.** Any
  unauthenticated POST that previously relied on `/v1/invoke` from a
  non-browser script without `Origin`/`Referer` will now return `403`.
  Callers should move to Bearer-token auth or use the CLI / Unix-socket
  path. Unix-socket clients are unaffected because
  `origin_check_enabled` is set only on the TCP listener.
- **GH #10 covers `override-verdict` only.** The recovery-panel dry-run
  path (`recovery auto-publish --dry-run`) does not require a context
  token. It is defended by GH #9 (CSRF) and GH #11 (read-only). A
  server-side allowlist of dry-run-only verbs was noted in the synthesis
  as deferred; not introduced here.
- **`would_expire` flag was not wired into the recovery-panel UI**
  (write scope is back-end + tests for this pass). Consumers can
  inspect the field; surfacing it visually is left to a follow-up.

## Changed files

- `src/striatum/service.py` — GH #9 (Content-Type, Origin/Referer
  fail-closed), GH #10 server-side.
- `src/striatum/web/templates/job_detail.html` — GH #10 template.
- `src/striatum/web/static/override_verdict.js` — GH #10 client.
- `src/striatum/cli/recovery.py` — GH #11 dry-run gate.
- `tests/test_override_modal_payload.py` — body-shape update.
- `tests/test_invoke_csrf_refused.py` — new (17 cases).
- `tests/test_override_modal_context_validation.py` — new + same-origin
  Origin header threaded through server-side cases.
- `tests/test_recovery_dry_run_no_side_effects.py` — new.
- `docs/issues/9/build/HANDOFF.md` — this artifact.
