---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["security", "csrf", "rfc-0050", "v1-invoke", "design-review", "gh-9", "gh-10", "gh-11"]
---

author: reviewer-unknown-model-001

# Design Review — GH #9-#11 Security Hardening

**Logical Name:** design_review
**Posture:** security (fresh-context, document-only)
**Target:** `docs/issues/9/DESIGN_SYNTHESIS.md` (designer-unknown-model-001)

## Executive Summary

The design closes the captured requirements of GH #9, #10, and #11 with concrete file targets, a coherent acceptance-criteria list, and an explicit out-of-scope statement for GH #12 and #13. The mitigation stack (strict media-type check, same-origin enforcement, server-issued context token for the override modal, read-only dry-run split) maps cleanly onto the three SPECs and the gemini findings.

Implementation can proceed in the shape described, but the same-origin helper as currently specified has one **actionable security weakness** (DNS-rebinding via trusted `Host` header) and two smaller ambiguities that the implementer should resolve before landing. None of these are blocking re-design; they are corrections to the implementation guidance.

Verdict: **accept_with_findings**.

## What The Design Gets Right

- **GH #9 root-cause fix is concrete.** The design names the exact pre-condition (`_read_json_body` ignores `Content-Type`), the exact strict matcher shape (lowercase + strip parameters + reject substring matches such as `text/application/json`), and the status code grid (`415`, `400`, existing `405`). Acceptance criteria 1 + Test 1 cover both the rejection and the success path.
- **Layered defense.** Same-origin enforcement is added on top of the media-type check rather than replacing it, so a future regression in either layer does not by itself reopen the CSRF surface.
- **GH #10 is addressed on both sides of the trust boundary.** Client-side URL/data-attribute parity check + server-side HMAC context token bound to `(run_id, job_id, session_id)`. The HMAC is keyed by a process-local secret with no durable storage, which respects the local-first invariant in `AGENTS.md`.
- **GH #11 names the actual existing bug.** `auto_publish_stale_artifacts` calls `expire_leases()` before branching on `dry_run`; the design splits candidate discovery from mutation and prescribes a `would_expire: true` classification rather than mutating the lease row. Test 5 asserts byte-for-byte equivalence on `events`, `artifacts`, `leases`, `jobs`, `queue_messages`, which is the strongest read-only assertion practically available.
- **Scope discipline.** GH #12 (clipboard) and #13 (ghost field) are listed as explicitly out of scope, and the "exact write scope" file list (`service.py`, `job_detail.html`, `override_verdict.js`, `recovery.py` + their tests) does not touch `copy_on_click.js` or `WorkflowGraphEditor.tsx`. This matches ROADMAP §4.1 vs §5.1.
- **Bearer-token escape hatch preserved.** Non-browser authenticated clients keep working, which avoids breaking the documented Service auth path in `docs/SPEC.md` §Local Service.

## Findings

### F1 — Same-origin computation trusts the `Host` header (DNS rebinding bypass)

- **Severity:** Medium
- **Where:** `DESIGN_SYNTHESIS.md` §2 ("Add same-origin enforcement for browser mutation routes"), bullet "Compute the service origin from the request's effective host: `Host` header plus the server scheme."
- **Why it matters:** `striatum serve` binds to loopback per `docs/SPEC.md` §Local Service. DNS rebinding is the canonical attack against loopback HTTP services: an attacker page on `evil.example` (initially resolving to attacker IP, then rebinded to `127.0.0.1`) issues a same-origin `POST http://evil.example:8080/v1/invoke` after rebind. The browser sends `Host: evil.example:8080` and `Origin: http://evil.example:8080`. If the server derives its own "service origin" from the request's `Host` header, both values agree and the same-origin check passes — even though the actual request is hitting the local runner. The Content-Type check from §1 does not save us because the request is browser-same-origin and skips preflight.
- **Suggested correction:** The service origin must be computed from the server's *bound* address (e.g., `BaseHTTPRequestHandler.server.server_address` plus scheme), not the request's `Host` header. Equivalently, the helper can enforce an explicit allowlist of host values matching what `striatum serve --host`/`--port` was started with (loopback literals + the configured port), and refuse requests whose `Host` header does not match that allowlist. Either approach defeats DNS rebinding; deriving from the request `Host` header does not.
- **Acceptance criterion delta:** add a test that POSTs to `/v1/invoke` with `Host: evil.example:<port>` and a matching `Origin: http://evil.example:<port>` over the loopback socket, and asserts `403`.

### F2 — "Browser-shaped POST" classification is under-specified

- **Severity:** Low
- **Where:** §2 last bullet: "Reject browser-shaped POSTs with neither `Origin` nor `Referer` unless a valid Bearer token was required and supplied."
- **Why it matters:** The phrase "browser-shaped" is not defined. A naive read is "any POST," in which case any non-browser client (curl, the test harness, the daemon RPC bridge) that omits `Origin` and `Referer` must either set a Bearer token or be rejected. If the service is started without `--token`, the design does not say whether the absence of `Origin`/`Referer` is allowed or refused. Both interpretations are reasonable, with different security/UX trade-offs:
  - **Strict (recommended):** when `web_enabled` is true, *all* non-GET requests must carry an `Origin` or `Referer` matching the service's bound origin, OR a valid Bearer token. This closes the surface unambiguously.
  - **Permissive:** if no `--token` is configured, missing `Origin`/`Referer` is allowed. This silently re-opens CSRF for the default `striatum serve --web --allow-mutations` configuration (which is the *exact* scenario GH #9 calls out).
- **Suggested correction:** rewrite the bullet to: "When `web_enabled` is true, any non-GET request to `/v1/invoke` and other mutation routes must satisfy *one* of (a) `Origin` matches the bound service origin, (b) `Referer` matches the bound service origin and `Origin` is absent, (c) a valid Bearer token is presented. Otherwise return `403`." Make it explicit that absence-of-token does not waive the Origin/Referer requirement — that is what GH #9's HIGH severity hinges on.
- **Acceptance criterion delta:** add a test "no `Origin`, no `Referer`, no Bearer token → `403`" against the default `striatum serve --web --allow-mutations` configuration.

### F3 — Context token has no replay or expiry semantics

- **Severity:** Low
- **Where:** §3 ("Bind override-verdict posts to rendered job context"), description of the HMAC token.
- **Why it matters:** The token is keyed to `(run_id, job_id, session_id)` with no nonce, timestamp, or single-use marker. In the local-first single-operator model this is acceptable, but it is worth recording the limitation: a token that leaks (e.g., via a logged `Referer`, a screenshot, or a copy-paste of network panel output) can be replayed against `/v1/invoke` as long as the session id is still valid. The design acknowledges that tokens become invalid on service restart, which is the only natural expiry.
- **Suggested correction:** either (a) accept the limitation and document it in the design's "Known Risks" section, or (b) include a short-lived `iat` timestamp inside the HMAC payload and enforce a max age (e.g., 8h) at validation time. Implementation effort for (b) is negligible if (a) is rejected by the implementer.
- **Acceptance criterion delta:** none required for the V1 scope; this is defense-in-depth.

## Non-Findings (Examined And Cleared)

- **Substring-matching media types.** Design §1 explicitly forbids `"application/json" in ctype` and prescribes a lowercase + strip-parameters matcher. This is the correct anti-pattern call-out for the existing bug.
- **HMAC secret durability.** Process-local secret regeneration on restart is the right call for a local-first single-operator service; no external secret store is needed and no telemetry surface is introduced (consistent with `AGENTS.md` "Product Boundary").
- **Scope creep into GH #12/#13.** The write-scope file list is disjoint from `copy_on_click.js` and `WorkflowGraphEditor.tsx`. The tests-to-add list is similarly disjoint. Verified.
- **Dry-run read-only split.** Design §4 names the offending `expire_leases()` call site explicitly, lists the exact mutation helpers the dry-run branch must not invoke (`expire_leases`, `ack_work`, `publish_artifact`, `complete_job`, `insert_event`, `maybe_complete_run`), and proposes the `would_expire: true` classification as the read-only substitute. This is implementable as-stated.
- **Override-verdict-only validation.** Design §3 scopes server-side context validation to `argv[0] == "override-verdict"`. The design notes the trade-off ("Do not add a special `/v1/invoke` allowlist for this pass unless the implementation remains small"). Acceptable for V1 of this hardening pass; future mutation verbs reaching the modal pattern can extend the check.
- **Local-first invariants.** No hosted services, telemetry, transcript capture, or external persistence are introduced. All mitigations are server-local or browser-side. Consistent with `AGENTS.md` Product Boundary.

## Reviewer Verification (Forward To Build Reviewers)

When the implementation lands, the build reviewers should confirm:

1. The same-origin helper compares Origin/Referer against the server's *bound* address (or an allowlist matching the bound address), not the request `Host` header. (F1)
2. The default `striatum serve --web --allow-mutations` configuration (no Bearer token) refuses POSTs that lack both `Origin` and `Referer`. (F2)
3. `_read_json_body` (or its `/v1/invoke` replacement) refuses substring matches like `text/application/json`.
4. `auto_publish_stale_artifacts(..., dry_run=True)` cannot reach `expire_leases`, `ack_work`, `publish_artifact`, `complete_job`, `insert_event`, or `maybe_complete_run` by static inspection.
5. The override modal's server-side validator returns `403` for any of: missing `web_context`, invalid HMAC token, mismatched `run_id`/`job_id`/`session_id` between argv and context.
6. The five test files listed in the design exist and fail against the pre-fix code paths (golden-negative).

## Verdict

`accept_with_findings`. Severity `medium` for F1; F2 and F3 are `low`. Implementation should proceed and address F1 as part of the same-origin helper rather than as a follow-up issue, because F1 directly affects the GH #9 acceptance criterion.
