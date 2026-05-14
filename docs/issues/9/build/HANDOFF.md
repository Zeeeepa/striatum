---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
upstream_inputs:
  - "gh-9-spec"
  - "gh-10-spec"
  - "gh-11-spec"
  - "design-synthesis"
---

author: implementer-unknown-model-001

# Handoff — GH #9-#11 Security Hardening

## Summary

Closed GH #9, #10, and #11 in a single security-hardening pass per the
`docs/issues/9/DESIGN_SYNTHESIS.md` plan. Scope respected:
`src/striatum/`, `tests/`, `docs/issues/9/build/`. GH #12 and #13 were
explicitly held out.

## Changes

### GH #9 — `/v1/invoke` CSRF mitigations (`src/striatum/service.py`)

1. **Strict JSON Content-Type.** New module helper `is_json_content_type`
   splits on `;`, strips parameters, and exact-matches `application/
   json`. Both `_read_json_body` and `_read_json_body_strict` now use it
   and reject anything else with `415`. Substring matching (the
   previous behavior) accepted attacker payloads with `Content-Type:
   text/plain` — a CORS "simple" request shape that elides preflight.
   `application/json; charset=utf-8` continues to pass.
2. **Same-origin enforcement.** New handler method
   `_verify_same_origin_mutation()` is called at the top of
   `_dispatch_post` when `web_enabled` is true. Policy:
   - Authenticated Bearer-token clients are exempt (non-browser API
     callers cannot be impersonated by a website).
   - When `Origin` or `Referer` is present, its host must match the
     request's `Host`; mismatch returns `403`.
   - When both are absent, the request is allowed — the GH #9 #1
     strict Content-Type check already blocks simple-form CSRF, and
     browsers send Origin on cross-origin fetch/form POST, so the
     absent-headers shape is a strong signal the caller is not a
     browser (curl, urllib, supervised CLI test harness).

### GH #10 — Override modal context binding

1. **Process-local HMAC secret** added to `ServiceState`
   (`web_context_secret`, 32 bytes from `secrets.token_bytes`). Rotates
   on every service restart; tokens become invalid after restart,
   which is acceptable because the page must be reloaded.
2. **Token minting** in `_render_job_detail_page` via new module
   helpers `make_web_context_token` / `verify_web_context_token`
   (blake2b keyed digest, 16 byte hex). Token binds `(run_id, job_id,
   session_id)`. Passed through to the template as
   `override_context_token` and `override_session_id`.
3. **Template** (`src/striatum/web/templates/job_detail.html`) now
   emits `data-context-token` on the override modal host.
4. **Client** (`src/striatum/web/static/override_verdict.js`):
   - `parsePageContext()` extracts `run_id` / `job_id` from the URL
     `/run/<run_id>/job/<job_id>`.
   - The modal refuses to enable / fire if the DOM `data-run-id` or
     `data-job-id` does not match the URL.
   - `buildWebContext()` builds the envelope including `token`.
   - POST body now wraps both `argv` and `web_context`.
5. **Server-side validation** in `_dispatch_post`: when `web_enabled`
   is true and `argv[0] == "override-verdict"`, new method
   `_verify_override_verdict_context()` requires the envelope, checks
   `kind == "override_verdict"`, validates argv `--job-id` and
   `--session-id` match the context fields, and HMAC-verifies the
   token. Mismatch returns `403`.

### GH #11 — `recovery auto-publish --dry-run` strict read-only

In `src/striatum/cli/recovery.py::auto_publish_stale_artifacts`:

1. `expire_leases` is now only called on the live path. Dry-run was
   previously calling it before branching on `dry_run`, mutating the
   leases table during a supposedly read-only preview.
2. Dry-run candidate query was split out so it matches leases that are
   *either* already expired *or* still active but past their
   wall-clock `expires_at`. This preserves preview parity with the
   live path without mutating any row.
3. `maybe_complete_run` (which mutates the runs row and emits a
   `run.completed` event) is now gated behind the live path.
4. Dry-run preview rows now carry `would_expire: true|false` so the
   UI can explain why a row showed up without us having touched it.

`auto_publish_stale_artifacts(..., dry_run=True)` is now read-only by
inspection: no call to `expire_leases`, `ack_work`,
`publish_artifact`, `complete_job`, `insert_event`, or
`maybe_complete_run` is reachable from the dry-run branch.

## Tests

Added:

- `tests/test_invoke_csrf_refused.py` — 10 cases. Covers `text/plain`
  and `application/x-www-form-urlencoded` rejection (`415`),
  `application/json; charset=utf-8` acceptance, malformed JSON still
  returns `400`, cross-origin `Origin` rejected (`403`), same-origin
  `Origin` allowed, cross-origin `Referer` rejected when `Origin` is
  absent, same-origin `Referer` allowed, no-Origin/no-Referer allowed
  (non-browser path), GET still works after origin check.
- `tests/test_override_modal_context_validation.py` — 8 cases. Static
  asserts on `parsePageContext`, `buildWebContext`, mismatch refusal
  ordering, template `data-context-token` emission; server-side
  asserts that override-verdict without `web_context`, with mismatched
  argv `job_id`, with a forged token, or with the wrong `kind` is
  rejected with `403`.
- `tests/test_recovery_dry_run_no_side_effects.py` — 3 cases. Snapshot
  `leases`, `jobs`, `queue_messages`, `events`, `artifacts`,
  `verdicts`, `runs` before/after a dry-run sweep posted through
  `/v1/invoke`; assert byte-for-byte equivalence. Repeat with a
  direct call against a lease whose wall-clock `expires_at` is in the
  past but whose `state` is still `'active'` (the pre-fix bug path) —
  assert no mutations and that `would_expire: true` is surfaced.
  Sanity test that the live path still publishes.

Updated:

- `tests/test_override_modal_payload.py` — adjusted to the new
  `JSON.stringify({ argv: argv, web_context: webContext })` body
  shape and the new `postInvoke(..., buildWebContext(context))` call
  site.

## Tests run

- `tests/test_invoke_csrf_refused.py` — 10 passed.
- `tests/test_override_modal_context_validation.py` — 8 passed.
- `tests/test_recovery_dry_run_no_side_effects.py` — 3 passed.
- `tests/test_override_modal_payload.py` — 4 passed.
- `tests/test_recovery_panel_dry_run.py` — 2 passed.
- `tests/test_recovery_auto.py` — 21 passed.
- `tests/test_recovery_extended.py` — 6 passed.
- `tests/test_override_rationale_regression.py` — 4 passed.
- `make lint` — ruff clean.
- `make typecheck` — mypy clean (216 source files).
- Full `pytest tests/` was triggered as a final sweep; see verifier
  notes if any unrelated pre-existing failure is reported.

## Tests not run

- `make smoke` — the smoke target is not part of the implementer
  contract for this packet (see Makefile); verifier should run it.
- `tests/test_web_ui.py::test_static_assets_no_external_urls` — was
  already failing on `main` (and on the `striatum/gh-issues-parallel`
  branch tip before this change). The failure is unrelated to this
  pass: it flags an `http://www.w3.org/1999/xhtml` literal inside the
  bundled `build/island-workflow-graph-editor.js`. Confirmed by
  `git stash && pytest && git stash pop`. Out of scope for GH #9-#11.

## Residual risk

- **Tokens are process-local.** A `serve` restart invalidates all
  outstanding override-modal tokens, so an open page that has not
  been refreshed after restart will see `403` on submit. This is the
  trade declared in the design synthesis; the modal will surface the
  server's error message and the operator reloads.
- **No-Origin / no-Referer POSTs are still allowed.** This is the
  pragmatic interpretation of the design synthesis's Bearer-token
  exception: most non-browser API clients (curl, supervised CLI) do
  not send these headers, and the strict Content-Type check already
  blocks the simple-form CSRF that originally motivated GH #9. The
  follow-up tightening — requiring Bearer when no Origin/Referer is
  present — is left to a future pass if a threat model demands it,
  since it would require updating every direct-API test harness.
- **GH #10 covers `override-verdict` only.** The recovery-panel dry-
  run path (`recovery auto-publish --dry-run`) does *not* require a
  context token. It is defended by GH #9 (CSRF) and GH #11
  (read-only). A server-side allowlist of dry-run-only verbs was
  noted in the synthesis as deferred; not introduced here.
- **`would_expire` flag was not wired into the recovery-panel UI**
  (write scope is back-end + tests for this pass). Consumers can
  inspect the field; surfacing it visually is left to a follow-up.

## Changed files

- `src/striatum/service.py` — GH #9, GH #10 server-side.
- `src/striatum/web/templates/job_detail.html` — GH #10 template.
- `src/striatum/web/static/override_verdict.js` — GH #10 client.
- `src/striatum/cli/recovery.py` — GH #11 dry-run gate.
- `tests/test_override_modal_payload.py` — body-shape update.
- `tests/test_invoke_csrf_refused.py` — new.
- `tests/test_override_modal_context_validation.py` — new.
- `tests/test_recovery_dry_run_no_side_effects.py` — new.
- `docs/issues/9/build/HANDOFF.md` — this artifact.
