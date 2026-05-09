---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0013 step 7 — web UI mutation buttons research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## Existing mutation gate (RFC 0012 V1)

`src/striatum/service.py` already enforces the mutation contract:

- `is_read_command(argv: list[str]) -> bool` — whitelist of read
  verbs (status, why, doctor, list, evidence, dashboard +
  subcommand-aware reads under workflow / supervise / worktree /
  run / recovery).
- `POST /v1/invoke` — accepts JSON `{argv: [...]}`. If the
  command is a mutation and `allow_mutations` is False, returns
  exit code 8 with a clear error envelope.
- Token auth via constant-time compare for HTTP loopback;
  Unix sockets bind 0o600 with no token.

**The SPA's job is to call `POST /v1/invoke`** with the right
argv. The runner already enforces the mutation gate; the SPA
just needs to (a) detect the gate state, (b) render buttons
that match the gate, and (c) render the error envelope cleanly
when mutations are off.

## Smallest viable mutation set

Per RFC 0013 § 7, the deferred mutations are *verdict / decision /
claim / block*. Concretely the SPA should expose:

| Button | View | argv shape | Notes |
|---|---|---|---|
| **Continue blocker** | Job detail (when blocker is open) | `["checkpoint", "resolve", "--blocker-id", "<id>", "--action", "continue"]` | Re-queues the affected job. Shows on every open `human_checkpoint` blocker. |
| **Cancel blocker** | Job detail (when blocker is open) | `["checkpoint", "resolve", "--blocker-id", "<id>", "--action", "cancel"]` | Cancels the affected job. Same view as Continue; presented in a confirm modal (cancellation is destructive). |
| **Record verdict** | Review job detail (when state=running) | `["verdict", "--session-id", ..., "--job-id", ..., "--lease-id", ..., "--verdict", "<v>", "--rationale", "<text>"]` | Verdict drop-down (`accept | accept_with_findings | needs_revision | reject`); rationale textarea. Step 7 V1 only emits `verdict`, not `submit-review` (publish + verdict in one call) — operators upload artifacts from disk via the existing `publish-artifact` flow before recording verdict. |
| **Record decision** | Run detail (always available) | `["decision", "record", "--run-id", ..., "--path", ..., "--outcome", "<o>", "--title", "<t>"]` | Outcome drop-down (`accepted | rejected | accepted_with_follow_up`); path / title text inputs. Decisions don't need a lease. |
| **Requeue stale review-only** | Job detail (when state=stale_lease, review-only job) | `["recovery", "requeue-stale", "--run-id", ..., "--job-id", ...]` | One-click trigger of the existing operator verb. Shown only on stale review-only jobs. |

Step 7 explicitly **does not** ship a "claim" button — claiming
work means becoming a session holder, which the SPA cannot
plausibly represent (no session id; no lease ownership). RFC
0013 § "Open Questions" left this to a future surface.

## SPA flow per button

1. User clicks the button → confirmation modal opens with the
   exact argv that would be sent.
2. User confirms → SPA POSTs `/v1/invoke` with body
   `{argv: [...]}`.
3. On 200 + `{ok: true}` → show success banner with the result
   envelope inline; refresh the underlying view (run detail,
   job detail) by re-fetching.
4. On 200 + `{ok: false, error: {...}}` → show the structured
   error envelope inline (exit code, message). View does not
   refresh.
5. On 4xx (e.g., the `--allow-mutations` gate refusing) →
   the SPA's gate-detection (see below) should already have
   hidden the button; if it still fires, render the same error
   envelope shape.

## Gate detection

The SPA must know whether the service was started with
`--allow-mutations` so it can hide the buttons (or grey them
with a tooltip) when mutations are off.

Two options:

- **Add `allow_mutations` to `/v1/health`.** The health endpoint
  already returns `{ok: true, data: {started_at, version, mode}}`;
  adding `allow_mutations: bool` is a one-line schema extension.
  Document as a permitted addition.
- **Probe a known mutation.** Cheap, but spammy: a full POST
  per page load.

V1 picks option 1: extend `/v1/health` with `allow_mutations`.

## Confirmation modal

Vanilla CSS modal; no library. Click outside to dismiss.
Confirm button hits the API. The destructive ones (Cancel
blocker, `reject` verdict) get a red confirm button; the rest
get a neutral primary-color confirm button.

## CSP preservation

The existing `src/striatum/web/static/app.js` already loads
under a strict CSP (`default-src 'self'`). Mutation buttons
require:

- `fetch('/v1/invoke', {method: 'POST', body: ...})` — already
  permitted, no CSP change.
- No external dependencies. No `eval`, no inline event handlers
  (we use `addEventListener` everywhere). No `innerHTML` for
  user-supplied text.

CSP is unchanged.

## Test plan

`tests/test_web_ui.py` additions:

- `test_health_includes_allow_mutations_flag` — `/v1/health`
  returns `allow_mutations: bool`.
- `test_invoke_verdict_succeeds_when_mutations_allowed` — full
  round trip via the existing test workflow.
- `test_invoke_verdict_refused_without_allow_mutations` — exit
  code 8 + structured error envelope.
- `test_invoke_decision_record_succeeds_when_mutations_allowed` —
  decision record runs without a lease.
- `test_invoke_checkpoint_resolve_continue` — re-queues the
  affected job.
- `test_invoke_checkpoint_resolve_cancel` — cancels the affected
  job.
- `test_invoke_recovery_requeue_stale_review_only` — happy path
  on a review-only stale lease.
- `test_invoke_rejects_unknown_verb` — non-existent verb returns
  exit code 2 with parser error envelope.
- `test_spa_app_js_has_mutation_button_handlers` — string-grep
  for `'/v1/invoke'`, `'verdict'`, `'decision'`,
  `'checkpoint'` in the bundled SPA.
- `test_no_external_url_invariant_after_mutations` — bundled
  SPA bytes contain no `http://` / `https://`.

## Friction anticipated

- **Lease lookup for verdict**. The verdict button needs the
  current lease id and session id for the review job. Today the
  SPA fetches `/v1/runs/<id>` which includes the run + job
  list; it does NOT include lease ownership. The SPA needs to
  fetch `/v1/runs/<id>/jobs/<job-id>` (existing read endpoint)
  for the active lease. Two-fetch flow per click; acceptable.
- **Decision path**. The decision-record path is a free-form
  text input; the publisher refuses paths outside the repo and
  inside `.striatum/`. The SPA validates client-side (no `..`,
  no leading `/`, no `.striatum/`), but the canonical refusal
  is server-side.
- **Mutation gate state changes**. The gate state is fixed at
  service start; the SPA caches it from `/v1/health` once per
  page load. If the operator restarts with a different flag,
  the page must be reloaded — same as today's read-only paths.
- **No CSRF token**. Loopback service with no cross-origin
  exposure makes CSRF a non-issue today; a tailscale bridge
  (or equivalent) does change that. V1 leaves CSRF out of
  scope; a future RFC could add an `X-Striatum-CSRF` header
  pattern.

## Recommended order

1. Extend `/v1/health` with `allow_mutations`.
2. Add `mutationsAllowed()` helper in `app.js`; cache from
   health probe.
3. Per-button: `app.js` button + handler + modal.
4. Tests.
5. README / HOW_TO_HUMAN doc updates ("now operators can
   resolve human checkpoints from the web UI").
