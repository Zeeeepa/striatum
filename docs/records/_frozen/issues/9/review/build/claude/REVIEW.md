---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics", "dx", "rfc-0050", "v2", "csrf", "override-modal", "recovery-panel", "gh-9", "gh-10", "gh-11"]
---

author: reviewer-unknown-model-002

# Build Review — GH #9-#11 Security Hardening (Ergonomics/DX)

**Logical Name:** build_review_claude
**Posture:** ergonomics_dx (fresh-context, first-time operator surface)
**Implementation under review:**

- `src/striatum/service.py` (GH #9, GH #10 server-side)
- `src/striatum/web/templates/job_detail.html` (GH #10 template)
- `src/striatum/web/static/override_verdict.js` (GH #10 client)
- `src/striatum/web/frontend/src/islands/recovery-panel/RecoveryPanel.tsx`
  (GH #11 surface)
- `src/striatum/cli/recovery.py` (GH #11 dry-run gate)

## Executive Summary

The implementation closes the security contract from
`DESIGN_SYNTHESIS.md` cleanly: media-type gate, fail-closed
same-origin enforcement bound to the actual listener, HMAC context
token bound to `(run_id, job_id, session_id)`, and a dry-run branch
that is read-only by inspection. The codex prior-round findings are
addressed in `service.py:1022-1094` (strict media-type parse, host
allowlist derived from the bind) and `recovery.py:766-907` (dry-run
discovery/mutation split with `would_expire` annotation).

From a first-time-operator perspective, however, the new safeguards
trade discoverability for security in several places where a cheap
ergonomic affordance would preserve both. The override-verdict
modal collapses several distinct failure shapes into a silent-disabled
button or a system `alert()`; the recovery-panel UI receives the new
`would_expire` annotation but drops it on the floor; and the 403
remediation hint that the design synthesis names in prose
("non-browser callers should use a Bearer token or the CLI") never
reaches the wire.

Verdict: **accept_with_findings**. None of the findings block landing.

## Per-Issue Acceptance Checklist

### GH #9 — CSRF on `/v1/invoke`

- [x] **Strict Content-Type parse before body parsing.**
  `service.py:1022-1049` rejects substrings, prefixes, suffixes,
  `application/jsonx`, `text/application/json`, comma-joined header
  values, and malformed parameters; accepts `application/json` and
  `application/json; charset=utf-8`. Returns `415`.
- [x] **Fail-closed Origin/Referer on every browser-addressable POST.**
  `service.py:3392-3447` runs at the top of `_dispatch_post` via
  `_requires_same_origin` covering `/v1/invoke` and every web-enabled
  POST. Bearer-token clients exempted, host origin derived from the
  actual bind, `Origin: null` / malformed / wrong-port / wrong-host
  rejected, headerless unauthenticated POSTs return `403`.
- [x] **Gate fires whether `--web` is on or off.**
  `service.py:3392-3395` — predicate does not gate on `web_enabled`
  for `/v1/invoke`. Verified by
  `tests/test_invoke_csrf_refused.py::test_invoke_refuses_missing_origin_referer_without_web_ui`.
- [x] **Tests cover the attack-shape grid.**
  17 cases in `tests/test_invoke_csrf_refused.py` map 1:1 to the
  design synthesis test list.
- [ ] **Operator-facing 403 messages name a remediation.** See
  Finding 4.

### GH #10 — Override modal trust-boundary

- [x] **URL → DOM context check on the client.**
  `override_verdict.js:56-60, 171-174` parse `(run_id, job_id)` from
  `/run/.../job/...` and refuse the modal on mismatch.
- [x] **`web_context` envelope on every POST.**
  `override_verdict.js:62-76` builds and includes
  `{kind, run_id, job_id, session_id, token}`.
- [x] **Process-local HMAC binding to `(run_id, job_id, session_id)`.**
  `service.py:1126-1154` mints/verifies tokens via keyed `blake2b`,
  16-byte hex, secret in `ServiceState.web_context_secret`.
- [x] **Server-side validation on `argv[0] == "override-verdict"`.**
  `service.py:1388-1394, 3449-3509` rejects missing envelope, wrong
  `kind`, argv/context mismatch, and forged tokens with `403`.
- [x] **Template carries `data-context-token`.**
  `job_detail.html:67`.
- [ ] **Disabled-trigger reason is discoverable.** See Finding 1.
- [ ] **Failure UX is consistent with the in-modal status pattern.**
  See Finding 2.
- [ ] **Service-restart token rotation surfaces a recovery hint.**
  See Finding 3.

### GH #11 — Recovery `auto-publish --dry-run` read-only

- [x] **Discovery / mutation split.**
  `recovery.py:766-805` runs `expire_leases` only on the live path;
  dry-run candidates instead match leases that are *either* expired
  *or* still active but past wall-clock `expires_at`.
- [x] **No reachable mutation helpers from dry-run branch.**
  `ack_work`, `publish_artifact`, `complete_job`, `insert_event`,
  `maybe_complete_run`, `expire_leases` all gated behind
  `if not dry_run:` (`recovery.py:766, 884, 909-976`).
- [x] **`would_expire: true|false` annotation on dry-run rows.**
  `recovery.py:884-908`.
- [x] **Workflow-domain table snapshot regression test.**
  `tests/test_recovery_dry_run_no_side_effects.py` covers `leases`,
  `jobs`, `queue_messages`, `events`, `artifacts`, `verdicts`,
  `runs`, plus live-path sanity.
- [ ] **`would_expire` flag is surfaced in the recovery panel UI.**
  See Finding 5.
- [ ] **Recovery panel has a documented follow-up path from preview
  to publish.** See Finding 6 (low; deliberate V1 scope).

## Findings

### Finding 1: Disabled "Override verdict" button gives no operator-visible reason

- **Severity:** Medium
- **Files:**
  `src/striatum/web/static/override_verdict.js:210`,
  `src/striatum/web/templates/job_detail.html:62-73`.
- **Symptom:** The trigger button is disabled whenever any of
  `jobId`, `sessionId`, `runId`, `contextToken`, or page-context
  match is missing/false. All five failure modes collapse to the
  same grayed-out state with no hover hint, no helper text near the
  button, and no live-region update. The accompanying
  `<span class="muted">Recorded override rationales remain visible
  above.</span>` (`job_detail.html:72`) is unrelated to the disabled
  state and tells the operator nothing about why the affordance is
  unavailable.
- **First-time-operator impact:** A new operator landing on a job
  detail page where the override affordance is disabled cannot
  distinguish "the workflow hasn't produced a verdict yet" (the
  expected steady state on a fresh run) from "the page token has
  rotated" (a recoverable error) from "the page's `data-job-id` is
  mismatched against the URL" (a security event the operator wants
  to know about). All three deserve different responses.
- **Remediation:**
  1. Set `trigger.title` from the failure mode the disable check
     fired on (e.g., `"No prior verdict to override yet"`,
     `"Page token unavailable — reload to refresh"`,
     `"Page context mismatch — refusing to enable"`).
  2. Optionally write the same message into the `<span class="muted">`
     slot or a sibling `<p>` with `role="status"` so it is
     discoverable without hover.
- **Why non-blocking:** the security gate is firmly closed; this is
  a discoverability fix that can land in the next ergonomics pass.

### Finding 2: `window.alert()` for modal failure paths breaks the modal UX idiom

- **Severity:** Low
- **Files:** `src/striatum/web/static/override_verdict.js:192-199`.
- **Symptom:** `openDialog` uses `window.alert(...)` for the
  "missing identifiers" and "context mismatch" branches:

  ```js
  if (!context.jobId || ...) {
    window.alert("Override verdict requires server-rendered job and session identifiers.");
    return;
  }
  if (contextMismatch) {
    window.alert("Override verdict refused: DOM job/run identifiers do not match the page URL.");
    return;
  }
  ```

  The rest of the file already establishes an in-dialog status
  affordance (`setStatus(status, message, true)`, lines 87-91) with
  `role="alert"` and CSS error styling
  (`base.css:1126-1134`). These two alert paths are the only places
  in the file that route through the browser's system modal.
- **First-time-operator impact:** In practice the trigger is
  disabled before `openDialog` can fire on these paths, so the
  alerts are a deep-defense backstop rather than an everyday surface.
  But if a script or extension flips `trigger.disabled` between
  render and click (which is exactly the threat model GH #10 names),
  the operator gets a browser-level alert instead of the in-dialog
  error styling the page already provides. The inconsistency also
  matters when reviewing the code: `alert()` is the visual marker
  for "this branch is unreachable" in modern UIs, so its presence in
  a defense-in-depth path is easy to read as dead code.
- **Remediation:** Either render the dialog and call
  `setStatus(status, "...", true)`, or render a sibling `<p
  role="alert" class="island-error">` near the trigger and route
  both branches through it. Drop the `window.alert` calls.
- **Why non-blocking:** the affected paths are gated by the
  `disabled` attribute and the server-side token check, so the
  inconsistency is cosmetic rather than functional.

### Finding 3: Token rotation on `serve` restart surfaces as opaque "invalid web_context token"

- **Severity:** Medium
- **Files:**
  `src/striatum/service.py:3504-3508` (error envelope),
  `src/striatum/web/static/override_verdict.js:240-247` (client
  status rendering).
- **Symptom:** `web_context_secret` rotates each time the service
  starts (`secrets.token_bytes`, `service.py:ServiceState.__init__`),
  by design. An operator who opens a job detail page, restarts
  `striatum serve` (any operational reason), and then clicks
  "Override verdict" sees the modal status:
  > invalid web_context token
  The handoff's residual-risk section names this case and the
  remediation ("the modal surfaces the server's error message and
  the operator reloads"), but the message itself contains no
  reload hint. The same operator who has not read the residual-risk
  doc will reasonably assume the token is *invalid as a security
  matter* rather than *stale because the page predates the current
  process*.
- **First-time-operator impact:** Worst-case behavior is the
  operator reports a perceived bug; best-case behavior is they
  refresh the page, but only after a frustration cycle.
- **Remediation:** Either:
  1. Have the server emit a stable error code
     (`"web_context_token_expired"` vs
     `"web_context_token_invalid"`) and have the client surface a
     "Reload to refresh page token" hint when the former is seen; or
  2. Bake the hint into the server message:
     `"invalid web_context token — reload the page to refresh"`.
  The minimal version is option (2): change one string in
  `service.py:3506` and ship.
- **Why non-blocking:** the security boundary is unaffected; this is
  a recoverability hint, not a fix.

### Finding 4: Same-origin error envelopes do not name a remediation

- **Severity:** Low
- **Files:** `src/striatum/service.py:3417-3447`.
- **Symptom:** The three 403 envelopes from
  `_verify_same_origin_mutation` are:
  - `"Host header origin refused"`
  - `"cross-origin request refused"`
  - `"Origin or Referer required"`

  The first is operator-hostile in vocabulary (the average operator
  has no mental model for "host header origin"). The third
  intentionally tightens earlier behavior — non-browser callers that
  used to POST to `/v1/invoke` without `Origin`/`Referer` now fail —
  but the message gives no hint that the design-prescribed
  remediation is "use a Bearer token (`--token`) or the CLI."
- **First-time-operator impact:** An operator running a local script
  that POSTs to `/v1/invoke` and starts seeing 403s post-upgrade
  will not, from the error envelope alone, know that the
  same-origin tightening is the cause or that Bearer-token auth is
  the supported path forward. The `docs/POSTGRES_TRANSITION.md`-
  style operator runbook for the v1.48.x security hardening doesn't
  exist yet, so the error envelope is the de facto first signal.
- **Remediation:** Append a remediation hint to the `"Origin or
  Referer required"` message (`"... — pass a Bearer token via
  --token or use the CLI"`). Optionally rename `"Host header origin
  refused"` to something more operator-readable
  (`"request Host does not match the service bind address"`). The
  cross-origin path is the rarest and least worth touching.
- **Why non-blocking:** the codes (`403`) and category messages are
  the security contract; the remediation is documentation that can
  live in either prose or the envelope.

### Finding 5: Recovery panel drops the new `would_expire` annotation

- **Severity:** Medium
- **Files:**
  `src/striatum/web/frontend/src/islands/recovery-panel/RecoveryPanel.tsx:82-102, 240-269`.
- **Symptom:** The dry-run server response carries a new per-row
  `would_expire: true|false` field that the implementer explicitly
  added so operators can tell why a row is eligible without us
  touching the lease (`recovery.py:884-908`, `HANDOFF.md` GH #11
  bullet 5). `normalizeRows` in the React component drops the field
  on the floor — the `WouldPublishRow` interface does not include it
  and the table (`RecoveryPanel.tsx:244-269`) has no column for it.
- **First-time-operator impact:** The point of `would_expire` is
  preview-parity with the live path's expire-then-evaluate
  behavior. Without surfacing the field, an operator looking at the
  "Would publish" table cannot tell whether the row is (a) genuinely
  stale (lease state already expired) or (b) wall-clock past
  `expires_at` with `state='active'` — the exact case that motivated
  the GH #11 fix. The implementer's residual-risk note correctly
  flags this: "would_expire flag was not wired into the
  recovery-panel UI."
- **Remediation:** Add `wouldExpire: boolean` to `WouldPublishRow`,
  hoist it from the candidate object during `normalizeRows`, and
  render either a column (`<th scope="col">Would expire</th>`) or a
  badge (`<span className="status-pill ...">stale wall-clock</span>`)
  on rows with `would_expire: true`. The CSS already has a
  `status-stale-lease` token (`base.css:21`) the badge can reuse.
- **Why non-blocking:** the server contract is correct; this is the
  frontend follow-up the implementer explicitly deferred. Acceptable
  to defer further, but worth landing before the recovery-panel
  ergonomics pass.

### Finding 6: Recovery panel has no live-publish path; the dry-run sub-state is the entire affordance

- **Severity:** Low (deliberate V1 scope)
- **Files:**
  `src/striatum/web/frontend/src/islands/recovery-panel/RecoveryPanel.tsx:158-277`.
- **Symptom:** The panel renders a "Dry run" button and a
  copy-this-command list. There is no "Publish now" button and the
  panel itself tells the operator
  `This panel only runs <code>striatum recovery auto-publish ...
  --dry-run</code>` (line 270-272). The next step after seeing the
  preview ("now actually publish") drops back to the terminal.
- **First-time-operator impact:** The recovery-panel surface is a
  preview widget, not an end-to-end recovery flow. This is
  consistent with the design synthesis (it does not require a
  publish button) and is honestly labeled, but for a first-time
  operator it can read as a UI that surfaces a problem and walks
  away. The copy-on-click affordance partially closes the gap
  (line 217-222 emits the publish command without `--dry-run`
  *would* be derivable, but the explicit recipe in the recipes
  list is only the dry-run one).
- **Remediation:** Either:
  1. Add a second recipe entry without `--dry-run` so the operator
     can copy the live command; or
  2. Add a "Publish previewed rows" button gated behind a
     confirmation that POSTs the same argv without `--dry-run` (only
     when the recovery-panel is allowed to mutate).
  Both are out of the GH #9-#11 scope as written; flagging for the
  recovery-panel follow-up.
- **Why non-blocking:** the design synthesis explicitly held the
  recovery-panel publish path out of this pass and named the V2
  argv-allowlist work as separate future scope.

## Verification Assessment

- **Tests run by the implementer.** The handoff lists 17 + 8 + 3 +
  4 + 2 + 21 + 6 + 4 + 22 + 10 + 20 + 7 + 3 = 127 passing tests across
  the focused and adjacent suites, plus `make lint` and `make
  typecheck` clean. The named regression suites
  (`tests/test_invoke_csrf_refused.py`,
  `tests/test_override_modal_context_validation.py`,
  `tests/test_recovery_dry_run_no_side_effects.py`) exist and exercise
  the attack-shape grid named in `DESIGN_SYNTHESIS.md`.
- **Tests deliberately not re-run.** `make smoke` was not run per the
  implementer-contract scope; the verifier should run it if the gate
  requires.
- **Pre-existing failure declared.**
  `tests/test_web_ui.py::test_static_assets_no_external_urls` already
  failed on `main` before this branch (per HANDOFF.md "Tests not
  run"). Not caused by this implementation. The failure points at a
  bundled `island-workflow-graph-editor.js` literal — out of scope
  for GH #9-#11.

## Scope Discipline (Verified)

- Source edits limited to `service.py`, `job_detail.html`,
  `override_verdict.js`, `recovery.py`.
- Test edits limited to the four named in `DESIGN_SYNTHESIS.md`
  "Tests To Add Or Update".
- No edits in `copy_on_click.js` (GH #12) or
  `WorkflowGraphEditor.tsx` (GH #13).
- No new external dependencies, hosted services, telemetry surfaces,
  or persistence stores.

## Non-Findings (Examined And Cleared)

- **`is_json_content_type` is byte-exact.** No prefix/suffix
  matching; tokens validated via the HTTP token charset; quoted
  strings require closing quotes and forbid CR/LF.
  `service.py:1022-1064`.
- **Allowed origin set is derived from the bind, not the request.**
  `service.py:3665` populates `state.allowed_origins` from
  `allowed_origins_for_bind(bound_host, bound_port)`. DNS-rebinding
  attempt covered by
  `tests/test_invoke_csrf_refused.py::test_invoke_refuses_dns_rebinding_host_origin_pair`.
- **Bearer-token bypass requires `tokens_match` constant-time
  compare.** `service.py:3379-3390` reuses the existing
  authentication helper.
- **`auto_publish_stale_artifacts` dry-run branch is read-only by
  inspection.** `recovery.py:766-907` — `if not dry_run:` gates each
  mutation helper; the dry-run query is a self-contained `SELECT`.
- **HMAC token compare uses `hmac.compare_digest`.**
  `service.py:1140-1154`.
- **Modal trap-focus / aria still wired.**
  `override_verdict.js:100-116, 122-125` — no regression from the
  GH #10 changes.

## Verdict

**Accept with findings.** The security contract from
`DESIGN_SYNTHESIS.md` lands cleanly and the prior codex
needs-revision items are closed by construction. The findings are
ergonomic discoverability gaps on the operator surface that should
land in a follow-up ergonomics pass — none block the GH #9-#11
hardening from shipping. Medium-severity items are #1 (disabled
trigger gives no reason), #3 (token rotation produces opaque error),
and #5 (`would_expire` is emitted but not displayed). Low-severity
items are #2 (`alert()` vs in-modal status pattern), #4 (403 error
envelopes name no remediation), and #6 (recovery panel has no
live-publish affordance).
