---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["security", "csrf", "rfc-0050", "v1-invoke", "design-review", "gh-9", "gh-10", "gh-11"]
---

author: reviewer-unknown-model-001

# Design Review — GH #9-#11 Security Hardening

**Logical Name:** design_review
**Posture:** security (fresh-context, document-only)
**Target:** `docs/issues/9/DESIGN_SYNTHESIS.md` (designer-unknown-model-001)

## Executive Summary

The design closes GH #9, GH #10, and GH #11 with sufficient specificity for
implementation, and explicitly holds GH #12 (clipboard) and GH #13 (graph-
editor ghost fields) out of scope. The mitigation stack — strict media-type
gate, same-origin enforcement derived from the bind address, server-issued
HMAC context token bound to `(run_id, job_id, session_id)`, and a read-only
dry-run split that names the existing offending call sites — maps cleanly
onto the three SPECs, the gemini adversarial findings (dogfood-056 build
review), and the prior Codex needs-revision contract.

The two actionable issues raised by the prior Codex review — DNS-rebinding
via a trusted `Host` header and an underspecified headerless-POST policy —
are both now closed in the design text (§2 second and last bullets).
Remaining observations are low-severity polish for the implementer rather
than gaps that block implementation.

Verdict: **accept**.

## What The Design Gets Right (Security-Specific)

- **DNS-rebinding gap explicitly named and closed.** §2 reads:
  "Do not accept a Host/Origin pair merely because they match each other;
  that leaves a DNS-rebinding-shaped gap." The allowed origin set is
  derived from the actual bind host and port, not echoed from the request
  `Host` header. This is the correct fix for the canonical loopback-CSRF
  bypass.
- **Fail-closed for unauthenticated headerless POSTs.** §2 last bullet
  resolves the "absent Origin and Referer" ambiguity in the right
  direction: return `403`. The design explicitly notes this tightens
  earlier language and directs non-browser callers to Bearer tokens or
  the CLI. This removes the silent re-opening of the GH #9 surface for
  the default `striatum serve --web --allow-mutations` configuration.
- **Gate runs regardless of `--web`.** §2 first bullet: "must apply to
  `/v1/invoke` whether or not `--web` is enabled." This closes the
  variant where a future runner exposes `/v1/invoke` headlessly and
  forgets to add the gate.
- **Layered defense, not replacement.** Media-type check + same-origin
  check + (for override-verdict) HMAC context token are independent
  layers. A regression in any one does not by itself reopen the CSRF
  surface from GH #9's POC (`{"argv":["run","cancel","--run-id","all"]}`
  via `enctype="text/plain"`).
- **GH #10 trust-boundary fix is two-sided.** Client refuses to submit
  on URL/DOM mismatch; server requires a process-local HMAC bound to
  `(run_id, job_id, session_id)`. Process-local secret rotation is
  acceptable because it requires no external secret store, consistent
  with `AGENTS.md` Product Boundary.
- **GH #11 read-only contract is constructive, not assertion-only.** §4
  splits candidate discovery from mutation rather than relying on
  happy-path assertions, and enumerates the forbidden mutation helpers
  the dry-run branch must not reach (`expire_leases`, `ack_work`,
  `publish_artifact`, `complete_job`, `insert_event`,
  `maybe_complete_run`). The invariant is scoped to workflow-domain
  side effects, preserving room for metadata-only audit — this is the
  exact distinction the prior Codex review required.
- **Test enumeration is comprehensive.** §"Tests To Add Or Update" lists
  the malicious-shape cases (text/plain, form-encoded, `application/
  jsonx`, comma-joined duplicates, `Origin: null`, malformed,
  wrong-port, DNS-rebinding-shaped Host/Origin), plus a live-path
  sanity test guarding against the dry-run branch silently becoming
  unreachable. Each acceptance criterion has a matching test case.

## Scope Discipline (Verified)

- §1 names #9/#10/#11 only; #12/#13 explicitly out.
- §"Exact Write Scope" lists `service.py`, `job_detail.html`,
  `override_verdict.js`, `recovery.py` and corresponding tests. Disjoint
  from `copy_on_click.js` (#12) and `WorkflowGraphEditor.tsx` (#13).
- "Avoid edits to … clipboard behavior, graph-editor field cleanup, or
  any dogfood #12/#13 surface" is explicit.
- §"Known Security And Regression Risks" calls out the
  Bearer-token-test-harness regression risk honestly rather than
  papering over it.

## Observations (Non-Blocking)

### O1 — Origin canonicalization algorithm is left to the implementer

- **Severity:** Low
- **Where:** §2 ("If `Origin` is present, it must parse cleanly and match
  the allowed origin set.")
- **Note:** "Parse cleanly" is not defined. The design lists hostile
  Host/Origin variants to reject in the tests, which gives the
  implementer enough to anchor against, but does not specify a canonical
  parser (e.g., `urllib.parse.urlsplit` with explicit scheme/host/port
  comparison, rejecting userinfo and path components). The known-risks
  section already flags loopback-alias parsing as fragile. Adding one
  sentence — "Compare `(scheme, host, port)` tuples; reject Origins
  containing userinfo, path, fragment, or query components" — would
  remove an implementation-time judgment call.
- **Why non-blocking:** the test enumeration covers the attack-shape
  cases, so a wrong parser will be caught by tests before landing.

### O2 — `web_context.run_id` is not in the explicit comparison list

- **Severity:** Low
- **Where:** §3 ("verify `web_context.job_id` and `web_context.session_id`
  match the parsed `--job-id` and `--session-id` argv values").
- **Note:** The HMAC token is bound to `(run_id, job_id, session_id)`,
  so token verification transitively checks `run_id` *if* the
  implementer recomputes the HMAC from `web_context.run_id` rather than
  from a URL-parsed run_id. A safer reading is to make the comparison
  list match the binding tuple: argv `--run-id` (or the URL run_id, if
  argv lacks it) versus `web_context.run_id`, in addition to job_id and
  session_id. Otherwise a stale token from a same-session different-run
  page (if such a thing could be constructed) could be replayed against
  argv that targets a different run.
- **Why non-blocking:** in practice `override-verdict` argv always
  carries `--job-id` and `--session-id`, both of which are in the
  binding tuple; a run-id mismatch with a valid `(job_id, session_id)`
  is unlikely to be constructible against the current page render.
  Worth tightening for defense in depth, but not a re-design.

### O3 — Token has no `iat`/`exp` inside the signed payload

- **Severity:** Low (defense-in-depth)
- **Where:** §3 ("The token may rotate on service restart.")
- **Note:** Service-restart rotation is the only natural expiry. In the
  local-first single-operator model this is acceptable. A leaked token
  (logged Referer, screenshot, devtools copy) is replayable until the
  service restarts. The design's "Known Risks" already implicitly
  accepts this. Adding a short-lived `iat` (e.g., 8h) inside the HMAC
  payload is trivial future work; not required for this pass.
- **Why non-blocking:** local-first threat model + single operator;
  no external secret store is justified by this risk alone.

### O4 — Other mutation verbs on `/v1/invoke` rely on media-type + same-origin only

- **Severity:** Low (scope trade-off, explicitly chosen)
- **Where:** §3 / §4 (design defers a broad `/v1/invoke` argv allowlist).
- **Note:** Per-verb context-token validation is scoped to `argv[0] ==
  "override-verdict"`. Other write-shaped verbs reachable through the
  bridge (`run cancel`, `verdict`, `complete`, `block`, `ack`,
  non-dry-run `recovery auto-publish`, etc.) are defended only by the
  media-type gate and the same-origin gate. For the gemini-captured
  attack shape (cross-origin `text/plain` form POST) this is
  sufficient. A future write verb added to the recovery panel without
  a corresponding token would not be covered.
- **Why non-blocking:** the design's "Known Risks" notes "A future
  `/v1/invoke` allowlist or CSRF-token system may still be warranted,
  but it is not required to close GH #9-#11." This is a deliberate
  choice and the captured issues do not require closing it.

## Non-Findings (Examined And Cleared)

- **Substring-match media types.** §1 forbids prefix/suffix/substring
  tricks (`application/jsonx`, `text/application/json`) and prescribes
  exact-media-type matching after parameter stripping. Both
  `application/json` and `application/json; charset=utf-8` accept;
  everything else rejects with `415`.
- **Status code grid.** `415` for non-JSON media types, `400` preserved
  for malformed JSON / non-object JSON / invalid content length, `403`
  for origin/context-token failures. Distinguishable failure modes for
  client diagnostics.
- **HMAC secret durability.** Process-local + regenerate on restart is
  the right call. No external secret store, no telemetry surface, no
  hosted dependency. Consistent with `AGENTS.md` Product Boundary and
  `docs/SPEC.md` Local Service invariants.
- **Dry-run audit preservation.** §4 explicitly carves out metadata-only
  daemon or request audit records from the no-side-effects invariant.
  This closes the prior Codex Finding 3 (no-events-row was too broad
  for an auditable mutation bridge).
- **Acceptance criteria coverage.** Each criterion in the
  "Acceptance Criteria" section has a corresponding test in the
  "Tests To Add Or Update" section. The "Reviewer Verification" handoff
  is concrete and inspectable.
- **GH #12 / GH #13 scope creep.** Write-scope file list excludes
  `copy_on_click.js` and `WorkflowGraphEditor.tsx`; tests-to-add list
  is similarly disjoint. Verified.
- **Local-first invariants.** No hosted services, telemetry, transcript
  capture, or external persistence are introduced. All mitigations are
  server-local (Python service, process-local HMAC, dry-run code split)
  or browser-side (override modal JS, template data-attribute).

## Reviewer Verification (Forward To Build Reviewers)

When the implementation lands, the build reviewer should confirm:

1. The same-origin helper computes the allowed origin set from the
   server's bound address (or an explicit allowlist anchored to the
   `--host`/`--port` values), not from the request `Host` header.
2. `Origin: null`, malformed Origin, wrong-port, wrong-host, and a
   DNS-rebinding-shaped Host/Origin pair (`Host: evil.example:<port>`
   + matching `Origin`) all return `403`.
3. An unauthenticated POST to `/v1/invoke` with neither `Origin` nor
   `Referer` returns `403` against the default `striatum serve --web
   --allow-mutations` configuration (no Bearer token).
4. The media-type matcher refuses prefix/suffix variants such as
   `application/jsonx`, `text/application/json`, and comma-joined
   duplicate Content-Type headers, with `415`.
5. `auto_publish_stale_artifacts(..., dry_run=True)` is read-only by
   static inspection — no reachable call to `expire_leases`, `ack_work`,
   `publish_artifact`, `complete_job`, `insert_event`, or
   `maybe_complete_run` from the dry-run branch.
6. The override-modal server validator returns `403` for any of:
   missing `web_context`, wrong `kind`, mismatched argv `--job-id` or
   `--session-id` against `web_context`, forged or stale HMAC token.
7. The test files named in §"Tests To Add Or Update" exist and each
   asserts the rejection path *before* CLI dispatch (not just at the
   handler return), so a regression in argument routing cannot bypass
   the gate.
8. No edits land in `src/striatum/web/static/copy_on_click.js` or
   `src/striatum/web/frontend/src/islands/workflow-graph-editor/`
   (GH #12 / GH #13 hold-out).

## Verdict

`accept`. The design synthesis is sufficient for implementation of
GH #9, GH #10, and GH #11 without ambiguity. The prior medium-severity
findings (DNS rebinding via trusted Host, headerless-POST fail-open) are
both closed in the current design text. Remaining observations
(canonicalization wording, `web_context.run_id` in the comparison list,
token-replay defense in depth, and bridge-wide allowlist deferral) are
low-severity polish that the implementer can address or defer without
re-design. Scope discipline against GH #12 and GH #13 is verified.
