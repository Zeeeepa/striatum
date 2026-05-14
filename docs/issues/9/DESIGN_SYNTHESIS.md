---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["gh-9-spec", "gh-10-spec", "gh-11-spec", "dogfood-056-gemini-review", "roadmap-4.1", "todo"]
---

author: designer-unknown-model-001

# Design Synthesis: GH #9-#11 Security Hardening

## Scope

This pass should close GH #9, #10, and #11 only:

- GH #9: `/v1/invoke` must reject browser-simple CSRF payloads and enforce same-origin browser mutations.
- GH #10: the override-verdict modal must not trust mutable DOM identifiers as the only proof of job/run context.
- GH #11: `recovery auto-publish --dry-run` must be proven read-only, including no event, lease, job, queue, or artifact side effects.

GH #12 and GH #13 are explicitly out of scope for this implementation pass.

## Implementation Approach

### 1. Harden JSON request intake for `/v1/invoke`

Update `src/striatum/service.py` so `/v1/invoke` uses strict JSON body handling before command dispatch.

The current `_read_json_body` accepts any `Content-Type`; that is the core GH #9 bug. Do not use substring matching such as `"application/json" in ctype`, because values like `text/application/json` should not pass. Add a shared media-type helper that lowercases and strips parameters, then accepts only:

```text
application/json
```

Use it from `_read_json_body` or replace the `/v1/invoke` call site with a stricter helper. Keep the existing status shape:

- `415` for missing or non-JSON `Content-Type`
- `400` for invalid `Content-Length`, malformed JSON, or non-object JSON
- existing `405` mutation-gate behavior after the body is accepted

This is the V1 fix that turns an attacker payload into a non-simple CORS request, forcing preflight before the browser can send arbitrary operator commands.

### 2. Add same-origin enforcement for browser mutation routes

Add a service helper in `src/striatum/service.py`, for example `_verify_same_origin_mutation(parsed_path)`, and call it near the top of `_dispatch_post` after `_authenticate()` succeeds and before any route-specific mutation handler runs.

Apply it when `self.state.web_enabled` is true. The helper should:

- Compute the service origin from the request's effective host: `Host` header plus the server scheme (`http` for the current local server).
- Accept same-origin `Origin`.
- If `Origin` is absent, accept same-origin `Referer`.
- Reject cross-origin values with `403`.
- Reject browser-shaped POSTs with neither `Origin` nor `Referer` unless a valid Bearer token was required and supplied.

The Bearer-token exception keeps authenticated non-browser API clients viable. Without a configured token, local web UI POSTs should carry same-origin browser headers and cross-site forms/fetches should fail closed.

The minimum route set is every non-GET route exposed by the web UI, including `/v1/invoke`, job/run action routes, chat mutation routes, workflow edit/run routes, and workflow generation write. The implementation can protect all `_dispatch_post` routes when `web_enabled` is true rather than trying to classify route-by-route.

### 3. Bind override-verdict posts to rendered job context

Treat GH #10 as defense-in-depth beyond the CSRF fix. The browser script in `src/striatum/web/static/override_verdict.js` should validate the current page context before building `argv`:

- Parse the run id from `/run/<run_id>/job/<job_id>`.
- Compare it with the modal host's `data-run-id`.
- Compare the page job id with the modal host's `data-job-id`.
- Refuse to call `fetch("/v1/invoke", ...)` if any field is missing or mismatched.

Also add a server-issued context token so the server is not relying only on client-side checks. On the job detail page, render a token tied to the tuple `(run_id, job_id, session_id)` and include it as `data-context-token` on the override modal host. The token can be an HMAC using a process-local secret in the service state; it does not need durable storage because it only protects the local browser session.

Change the modal POST body from:

```json
{"argv": ["override-verdict", "..."]}
```

to:

```json
{
  "argv": ["override-verdict", "..."],
  "web_context": {
    "kind": "override_verdict",
    "run_id": "<rendered-run-id>",
    "job_id": "<rendered-job-id>",
    "session_id": "<rendered-session-id>",
    "token": "<server-token>"
  }
}
```

In `/v1/invoke`, validate this context before invoking `override-verdict` argv. If `argv[0] == "override-verdict"` and `web_enabled` is true, require the context, verify the token, and verify the argv job/session ids match the context fields. Return `403` on mismatch. This closes the specific cross-context modal attack even if an attacker mutates DOM attributes after render.

### 4. Make `auto-publish --dry-run` strictly read-only

`src/striatum/cli/recovery.py:auto_publish_stale_artifacts` currently calls `expire_leases()` before it branches on `dry_run`. That means dry-run can mutate lease/job state indirectly. Fix the function so `dry_run=True` only reads current rows.

Recommended shape:

- Split candidate discovery from mutation.
- For dry-run, query only already-expired/stale candidates and compute `would_publish` rows without calling `expire_leases`, `ack_work`, `publish_artifact`, `complete_job`, `insert_event`, or `maybe_complete_run`.
- For non-dry-run, keep the existing mutation path, including lazy lease expiry and final run completion checks.
- If the UI needs dry-run to preview leases that are expired by wall clock but not yet marked expired, return a read-only classification such as `would_expire: true` instead of mutating the lease row.

Do not add a special `/v1/invoke` allowlist for this pass unless the implementation remains small. The primary fix is the strict read-only guarantee plus the GH #9 request hardening.

## Exact Write Scope For Implementation

Expected source edits:

- `src/striatum/service.py`
- `src/striatum/web/templates/job_detail.html`
- `src/striatum/web/static/override_verdict.js`
- `src/striatum/cli/recovery.py`

Expected test edits/additions:

- `tests/test_service.py` or new `tests/test_invoke_csrf_refused.py`
- `tests/test_service.py` or new `tests/test_origin_enforcement.py`
- `tests/test_override_modal_payload.py` or new `tests/test_override_modal_context_validation.py`
- `tests/test_recovery_panel_dry_run.py` or new `tests/test_recovery_dry_run_no_side_effects.py`

Avoid edits to workflow generation, RFC 0051 auto-finalize, clipboard behavior, or graph-editor fields in this pass.

## Tests To Add Or Update

1. `/v1/invoke` rejects simple-request content types.
   - Spawn service with `--web --allow-mutations`.
   - POST a valid JSON body with `Content-Type: text/plain`.
   - Assert `415` and no command side effects.
   - Also assert `application/json; charset=utf-8` succeeds and malformed JSON still returns `400`.

2. Same-origin enforcement rejects cross-site mutation posts.
   - POST to `/v1/invoke` with `Origin: http://evil.example`.
   - Assert `403`.
   - POST with `Origin: http://127.0.0.1:<port>` and assert the same read command succeeds.
   - Add a Referer-only case.
   - Cover at least one direct web route such as run cancel or job retry if a lightweight fixture exists.

3. Override modal refuses mismatched context before fetch.
   - Extend the static JS tests to assert the script parses `/run/<run_id>/job/<job_id>`.
   - Assert mismatched `data-run-id` or `data-job-id` returns/refuses before `postInvoke`.
   - Assert the POST body includes `web_context` and the context token.

4. `/v1/invoke` validates override context server-side.
   - Render or synthesize a valid context token for `(run_id, job_id, session_id)`.
   - Assert matching token/context reaches normal CLI validation.
   - Assert mismatched argv job id, mismatched session id, or invalid token returns `403`.

5. Recovery dry-run has no side effects.
   - Seed a stale publishable artifact.
   - Snapshot counts and key rows from `events`, `artifacts`, `leases`, `jobs`, and `queue_messages`.
   - Run `striatum recovery auto-publish --run-id <id> --dry-run --json`, both directly and through `/v1/invoke`.
   - Assert the result reports `dry_run: true` and expected `would_publish` rows.
   - Assert the database snapshots are byte-for-byte equivalent for the covered rows.

## Acceptance Criteria

- Cross-site form or fetch requests using `text/plain`, form content types, or missing JSON content type cannot execute `/v1/invoke`.
- Cross-origin browser POSTs to the web UI mutation surface are rejected before route handlers run.
- Same-origin UI operations continue to work without requiring a durable CSRF token or external persistence.
- Override-verdict requests are bound to the rendered job page context on both client and server.
- `recovery auto-publish --dry-run` performs no database writes and records no events or artifacts.
- The implementation keeps Striatum local-first: no hosted service, telemetry, transcript capture, or external persistence is introduced.

## Known Risks

- Strict origin enforcement can break existing tests or scripts that POST to web-enabled service routes without browser headers. Keep the Bearer-token exception and update tests to model either same-origin browser calls or authenticated API calls.
- A process-local context-token secret means tokens become invalid after service restart. That is acceptable for modal actions because the page should be reloaded after a restart.
- Host normalization is easy to get subtly wrong for IPv6 literals and `localhost` versus `127.0.0.1`. Tests should cover the service's actual bound host and at least one clearly cross-origin host.
- Tightening `_read_json_body_strict` media-type matching may affect existing workflow-generation tests that relied on permissive substring behavior. This is desirable, but failures should be fixed by sending the correct header.
- Dry-run no-side-effect tests must avoid using helper setup that mutates state during the assertion window.

## Reviewer Verification

The downstream reviewer should verify:

- `src/striatum/service.py` has a single, exact JSON media-type check and `/v1/invoke` uses it.
- Origin/Referer enforcement happens before command invocation and before web mutation handlers.
- The override modal has both client-side page context checks and server-side token validation for `override-verdict`.
- `auto_publish_stale_artifacts(..., dry_run=True)` cannot call mutation helpers by inspection, not only by happy-path tests.
- The new tests fail against the pre-fix code paths described in GH #9-#11 and pass after the implementation.

